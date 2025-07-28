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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	"github.com/your-org/etcd-k8s-operator/pkg/utils"
)

var _ = Describe("Multi-node EtcdCluster Controller", func() {
	Context("When creating multi-node clusters", func() {
		var (
			ctx       context.Context
			namespace *corev1.Namespace
		)

		BeforeEach(func() {
			ctx = context.Background()

			// 创建测试命名空间
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-multinode-",
				},
			}
			Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
		})

		AfterEach(func() {
			// 清理测试命名空间
			if namespace != nil {
				Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
			}
		})

		It("Should create a 3-node etcd cluster successfully", func() {
			cluster := &etcdv1alpha1.EtcdCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-3-node-cluster",
					Namespace: namespace.Name,
				},
				Spec: etcdv1alpha1.EtcdClusterSpec{
					Size:       3,
					Version:    "3.5.9",
					Repository: "bitnami/etcd",
					Storage: etcdv1alpha1.EtcdStorageSpec{
						Size: resource.MustParse("1Gi"),
					},
				},
			}

			// 创建集群
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			// 验证集群最终达到运行状态
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute*5, time.Second*10).Should(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))

			// 验证副本数
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return 0
				}
				return cluster.Status.ReadyReplicas
			}, time.Minute*3, time.Second*5).Should(Equal(int32(3)))

			// 验证 StatefulSet
			sts := &appsv1.StatefulSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
			}, time.Minute*2, time.Second*5).Should(Succeed())

			Expect(*sts.Spec.Replicas).To(Equal(int32(3)))
			Expect(sts.Status.ReadyReplicas).To(Equal(int32(3)))

			// 验证成员状态
			Eventually(func() int {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return 0
				}
				readyCount := 0
				for _, member := range cluster.Status.Members {
					if member.Ready {
						readyCount++
					}
				}
				return readyCount
			}, time.Minute*3, time.Second*5).Should(Equal(3))

			// 验证客户端端点
			Expect(cluster.Status.ClientEndpoints).To(HaveLen(3))
			for i := 0; i < 3; i++ {
				expectedEndpoint := fmt.Sprintf("http://%s-%d.%s-peer.%s.svc.cluster.local:%d",
					cluster.Name, i, cluster.Name, cluster.Namespace, utils.EtcdClientPort)
				Expect(cluster.Status.ClientEndpoints).To(ContainElement(expectedEndpoint))
			}
		})

		It("Should create a 5-node etcd cluster successfully", func() {
			cluster := &etcdv1alpha1.EtcdCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-5-node-cluster",
					Namespace: namespace.Name,
				},
				Spec: etcdv1alpha1.EtcdClusterSpec{
					Size:       5,
					Version:    "3.5.9",
					Repository: "bitnami/etcd",
					Storage: etcdv1alpha1.EtcdStorageSpec{
						Size: resource.MustParse("1Gi"),
					},
				},
			}

			// 创建集群
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			// 验证集群最终达到运行状态
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute*8, time.Second*10).Should(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))

			// 验证副本数
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return 0
				}
				return cluster.Status.ReadyReplicas
			}, time.Minute*5, time.Second*5).Should(Equal(int32(5)))

			// 验证成员状态
			Eventually(func() int {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return 0
				}
				return len(cluster.Status.Members)
			}, time.Minute*3, time.Second*5).Should(Equal(5))

			// 验证所有成员都就绪
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return false
				}
				for _, member := range cluster.Status.Members {
					if !member.Ready {
						return false
					}
				}
				return len(cluster.Status.Members) == 5
			}, time.Minute*5, time.Second*5).Should(BeTrue())
		})

		It("Should create a 7-node etcd cluster successfully", func() {
			cluster := &etcdv1alpha1.EtcdCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-7-node-cluster",
					Namespace: namespace.Name,
				},
				Spec: etcdv1alpha1.EtcdClusterSpec{
					Size:       7,
					Version:    "3.5.9",
					Repository: "bitnami/etcd",
					Storage: etcdv1alpha1.EtcdStorageSpec{
						Size: resource.MustParse("1Gi"),
					},
				},
			}

			// 创建集群
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			// 验证集群最终达到运行状态（7节点需要更长时间）
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute*10, time.Second*15).Should(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))

			// 验证副本数
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return 0
				}
				return cluster.Status.ReadyReplicas
			}, time.Minute*8, time.Second*10).Should(Equal(int32(7)))

			// 验证客户端端点数量
			Eventually(func() int {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return 0
				}
				return len(cluster.Status.ClientEndpoints)
			}, time.Minute*3, time.Second*5).Should(Equal(7))
		})
	})
})
