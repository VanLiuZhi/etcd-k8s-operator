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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
)

var _ = Describe("Resource Creation Controller", func() {
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
			Client: k8sClient,
			Scheme: scheme.Scheme,
		}
	})

	AfterEach(func() {
		cancel()
		err := testEnv.Stop()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("When ensuring resources", func() {
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
			}

			// 创建 EtcdCluster 资源
			err := k8sClient.Create(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should create ConfigMap successfully", func() {
			// 调用 ensureConfigMap
			err := reconciler.ensureConfigMap(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())

			// 验证 ConfigMap 是否创建
			configMap := &corev1.ConfigMap{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name + "-config",
				Namespace: cluster.Namespace,
			}, configMap)
			Expect(err).NotTo(HaveOccurred())

			// 验证 ConfigMap 内容
			Expect(configMap.Data).To(HaveKey("etcd.conf"))
			Expect(configMap.Data["etcd.conf"]).To(ContainSubstring("name: $(ETCD_NAME)"))
		})

		It("Should create Services successfully", func() {
			// 调用 ensureServices
			err := reconciler.ensureServices(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())

			// 验证 Client Service 是否创建
			clientService := &corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name + "-client",
				Namespace: cluster.Namespace,
			}, clientService)
			Expect(err).NotTo(HaveOccurred())

			// 验证 Peer Service 是否创建
			peerService := &corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name + "-peer",
				Namespace: cluster.Namespace,
			}, peerService)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should create StatefulSet successfully", func() {
			// 调用 ensureStatefulSet
			err := reconciler.ensureStatefulSet(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())

			// 验证 StatefulSet 是否创建
			statefulSet := &appsv1.StatefulSet{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			}, statefulSet)
			Expect(err).NotTo(HaveOccurred())

			// 验证 StatefulSet 配置
			Expect(*statefulSet.Spec.Replicas).To(Equal(cluster.Spec.Size))
			Expect(statefulSet.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Image).To(Equal("quay.io/coreos/etcd:3.5.9"))
		})

		It("Should create all resources through ensureResources", func() {
			// 调用 ensureResources
			err := reconciler.ensureResources(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())

			// 验证所有资源都已创建
			// 1. ConfigMap
			configMap := &corev1.ConfigMap{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name + "-config",
				Namespace: cluster.Namespace,
			}, configMap)
			Expect(err).NotTo(HaveOccurred())

			// 2. Client Service
			clientService := &corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name + "-client",
				Namespace: cluster.Namespace,
			}, clientService)
			Expect(err).NotTo(HaveOccurred())

			// 3. Peer Service
			peerService := &corev1.Service{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name + "-peer",
				Namespace: cluster.Namespace,
			}, peerService)
			Expect(err).NotTo(HaveOccurred())

			// 4. StatefulSet
			statefulSet := &appsv1.StatefulSet{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			}, statefulSet)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should handle resource updates correctly", func() {
			// 首次创建资源
			err := reconciler.ensureResources(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())

			// 再次调用应该不报错（幂等性）
			err = reconciler.ensureResources(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())

			// 修改集群规格
			cluster.Spec.Size = 5
			err = k8sClient.Update(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())

			// 再次确保资源，应该更新 StatefulSet
			err = reconciler.ensureResources(ctx, cluster)
			Expect(err).NotTo(HaveOccurred())

			// 验证 StatefulSet 已更新
			statefulSet := &appsv1.StatefulSet{}
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			}, statefulSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(*statefulSet.Spec.Replicas).To(Equal(int32(5)))
		})
	})
})

func TestResourceCreation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource Creation Controller Suite")
}
