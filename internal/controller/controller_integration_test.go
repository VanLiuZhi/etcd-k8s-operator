/*
Copyright 2025 ETCD Operator Team.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
)

var _ = Describe("Controller Integration", func() {
	var (
		ctx        context.Context
		cancel     context.CancelFunc
		k8sClient  client.Client
		testEnv    *envtest.Environment
		reconciler *EtcdClusterReconciler
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.TODO())

		// 设置日志
		logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

		// 创建测试环境，指定 CRD 路径
		testEnv = &envtest.Environment{
			CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
		}

		cfg, err := testEnv.Start()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).NotTo(BeNil())

		// 注册 scheme
		err = etcdv1alpha1.AddToScheme(scheme.Scheme)
		Expect(err).NotTo(HaveOccurred())

		// 创建客户端
		k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
		Expect(err).NotTo(HaveOccurred())

		// 创建 reconciler
		reconciler = &EtcdClusterReconciler{
			Client:   k8sClient,
			Scheme:   scheme.Scheme,
			Recorder: record.NewFakeRecorder(100),
		}
	})

	AfterEach(func() {
		cancel()
		err := testEnv.Stop()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("When handling creating phase", func() {
		var cluster *etcdv1alpha1.EtcdCluster

		BeforeEach(func() {
			cluster = &etcdv1alpha1.EtcdCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: etcdv1alpha1.EtcdClusterSpec{
					Size:       3,
					Version:    "3.5.9",
					Repository: "quay.io/coreos/etcd",
					Storage: etcdv1alpha1.EtcdStorageSpec{
						Size: resource.MustParse("1Gi"),
					},
				},
				// Status 应该为空，让控制器自己初始化
			}

			// 创建 EtcdCluster 资源
			err := k8sClient.Create(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should handle creating phase correctly", func() {
			// 调用 handleCreating
			result, err := reconciler.handleCreating(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())

			// 应该返回 requeue，因为集群还没有就绪
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))

			// 验证资源已创建
			// 1. StatefulSet
			sts := &appsv1.StatefulSet{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			}, sts)
			Expect(err).NotTo(HaveOccurred())

			// 2. Services
			clientService := &corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name + "-client",
				Namespace: cluster.Namespace,
			}, clientService)
			Expect(err).NotTo(HaveOccurred())

			// 3. ConfigMap
			configMap := &corev1.ConfigMap{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name + "-config",
				Namespace: cluster.Namespace,
			}, configMap)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should check cluster readiness correctly", func() {
			// 首先确保资源存在
			err := reconciler.ensureResources(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())

			// 检查集群就绪性（应该返回 false，因为 Pod 还没有启动）
			ready, err := reconciler.checkClusterReady(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(ready).To(BeFalse())

			// 模拟 StatefulSet 就绪
			sts := &appsv1.StatefulSet{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			}, sts)
			Expect(err).NotTo(HaveOccurred())

			// 更新 StatefulSet 状态为就绪
			sts.Status.Replicas = cluster.Spec.Size
			sts.Status.ReadyReplicas = cluster.Spec.Size
			err = k8sClient.Status().Update(ctx, sts)
			Expect(err).NotTo(HaveOccurred())

			// 再次检查就绪性（现在应该返回 true）
			ready, err = reconciler.checkClusterReady(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(ready).To(BeTrue())
		})

		It("Should complete full reconcile cycle", func() {
			// 第一次 reconcile - 应该添加 finalizer 并重新排队
			result, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue || result.RequeueAfter > 0).To(BeTrue())

			// 第二次 reconcile - 应该初始化并转换到 Creating 状态
			result, err = reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// 验证集群状态转换到 Creating
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			}, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(cluster.Status.Phase).To(Equal(etcdv1alpha1.EtcdClusterPhaseCreating))

			// 第三次 reconcile - 应该创建资源并等待
			result, err = reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0))

			// 模拟 StatefulSet 就绪
			sts := &appsv1.StatefulSet{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			}, sts)
			Expect(err).NotTo(HaveOccurred())

			sts.Status.Replicas = cluster.Spec.Size
			sts.Status.ReadyReplicas = cluster.Spec.Size
			err = k8sClient.Status().Update(ctx, sts)
			Expect(err).NotTo(HaveOccurred())

			// 第四次 reconcile - 应该转换到 Running 状态
			result, err = reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// 验证集群状态转换到 Running
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			}, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(cluster.Status.Phase).To(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))
		})
	})
})

func TestControllerIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controller Integration Suite")
}
