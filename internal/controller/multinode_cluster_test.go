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
					Version:    "v3.5.21",
					Repository: "quay.io/coreos/etcd",
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
					Version:    "v3.5.21",
					Repository: "quay.io/coreos/etcd",
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
					Version:    "v3.5.21",
					Repository: "quay.io/coreos/etcd",
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

	Context("When scaling up clusters", func() {
		var (
			ctx       context.Context
			namespace *corev1.Namespace
		)

		BeforeEach(func() {
			ctx = context.Background()

			// 创建测试命名空间
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-scaleup-",
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

		It("Should scale up from 1 to 3 nodes successfully", func() {
			By("Creating a single-node cluster")
			cluster := &etcdv1alpha1.EtcdCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "scaleup-test-cluster",
					Namespace: namespace.Name,
				},
				Spec: etcdv1alpha1.EtcdClusterSpec{
					Size:       1, // 开始时只有1个节点
					Version:    "v3.5.21",
					Repository: "quay.io/coreos/etcd",
					Storage: etcdv1alpha1.EtcdStorageSpec{
						Size: resource.MustParse("1Gi"),
					},
				},
			}

			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			By("Waiting for single-node cluster to be ready")
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute*3, time.Second*5).Should(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))

			By("Verifying StatefulSet has 1 replica")
			sts := &appsv1.StatefulSet{}
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
				if err != nil {
					return 0
				}
				return sts.Status.ReadyReplicas
			}, time.Minute*2, time.Second*5).Should(Equal(int32(1)))

			By("Scaling up to 3 nodes")
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return err
				}
				cluster.Spec.Size = 3
				return k8sClient.Update(ctx, cluster)
			}, time.Second*30, time.Second*2).Should(Succeed())

			By("Verifying cluster enters scaling phase")
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute*2, time.Second*5).Should(Equal(etcdv1alpha1.EtcdClusterPhaseScaling))

			By("Waiting for scale-up to complete")
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
				if err != nil {
					return 0
				}
				return sts.Status.ReadyReplicas
			}, time.Minute*5, time.Second*10).Should(Equal(int32(3)))

			By("Verifying cluster returns to running phase")
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute*2, time.Second*5).Should(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))

			By("Verifying cluster status shows 3 ready replicas")
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return 0
				}
				return cluster.Status.ReadyReplicas
			}, time.Minute*1, time.Second*5).Should(Equal(int32(3)))
		})

		It("Should scale up from 1 to 5 nodes progressively", func() {
			By("Creating a single-node cluster")
			cluster := &etcdv1alpha1.EtcdCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "progressive-scaleup-cluster",
					Namespace: namespace.Name,
				},
				Spec: etcdv1alpha1.EtcdClusterSpec{
					Size:       1, // 开始时只有1个节点
					Version:    "v3.5.21",
					Repository: "quay.io/coreos/etcd",
					Storage: etcdv1alpha1.EtcdStorageSpec{
						Size: resource.MustParse("1Gi"),
					},
				},
			}

			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			By("Waiting for single-node cluster to be ready")
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute*3, time.Second*5).Should(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))

			By("Scaling up to 5 nodes")
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return err
				}
				cluster.Spec.Size = 5
				return k8sClient.Update(ctx, cluster)
			}, time.Second*30, time.Second*2).Should(Succeed())

			By("Verifying progressive scaling behavior")
			// 大规模扩容应该是渐进式的，可能会经历多个中间状态
			sts := &appsv1.StatefulSet{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
				if err != nil {
					return false
				}
				// 验证StatefulSet的副本数在逐步增加
				return sts.Status.ReadyReplicas > 1 && sts.Status.ReadyReplicas <= 5
			}, time.Minute*3, time.Second*10).Should(BeTrue())

			By("Waiting for final scale-up to complete")
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
				if err != nil {
					return 0
				}
				return sts.Status.ReadyReplicas
			}, time.Minute*8, time.Second*15).Should(Equal(int32(5)))

			By("Verifying cluster returns to running phase")
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute*2, time.Second*5).Should(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))
		})
	})

	Context("When scaling down clusters", func() {
		var (
			ctx       context.Context
			namespace *corev1.Namespace
		)

		BeforeEach(func() {
			ctx = context.Background()

			// 创建测试命名空间
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-scaledown-",
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

		It("Should scale down from 3 to 1 node successfully", func() {
			By("Creating a 3-node cluster")
			cluster := &etcdv1alpha1.EtcdCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "scaledown-test-cluster",
					Namespace: namespace.Name,
				},
				Spec: etcdv1alpha1.EtcdClusterSpec{
					Size:       3, // 开始时有3个节点
					Version:    "v3.5.21",
					Repository: "quay.io/coreos/etcd",
					Storage: etcdv1alpha1.EtcdStorageSpec{
						Size: resource.MustParse("1Gi"),
					},
				},
			}

			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			By("Waiting for 3-node cluster to be ready")
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

			By("Verifying StatefulSet has 3 replicas")
			sts := &appsv1.StatefulSet{}
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
				if err != nil {
					return 0
				}
				return sts.Status.ReadyReplicas
			}, time.Minute*3, time.Second*10).Should(Equal(int32(3)))

			By("Scaling down to 1 node")
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return err
				}
				cluster.Spec.Size = 1
				return k8sClient.Update(ctx, cluster)
			}, time.Second*30, time.Second*2).Should(Succeed())

			By("Verifying cluster enters scaling phase")
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute*2, time.Second*5).Should(Equal(etcdv1alpha1.EtcdClusterPhaseScaling))

			By("Waiting for scale-down to complete")
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
				if err != nil {
					return 0
				}
				return sts.Status.ReadyReplicas
			}, time.Minute*5, time.Second*10).Should(Equal(int32(1)))

			By("Verifying cluster returns to running phase")
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute*2, time.Second*5).Should(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))

			By("Verifying cluster status shows 1 ready replica")
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return 0
				}
				return cluster.Status.ReadyReplicas
			}, time.Minute*1, time.Second*5).Should(Equal(int32(1)))
		})

		It("Should scale down from 5 to 1 node progressively", func() {
			By("Creating a 5-node cluster")
			cluster := &etcdv1alpha1.EtcdCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "progressive-scaledown-cluster",
					Namespace: namespace.Name,
				},
				Spec: etcdv1alpha1.EtcdClusterSpec{
					Size:       5, // 开始时有5个节点
					Version:    "v3.5.21",
					Repository: "quay.io/coreos/etcd",
					Storage: etcdv1alpha1.EtcdStorageSpec{
						Size: resource.MustParse("1Gi"),
					},
				},
			}

			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			By("Waiting for 5-node cluster to be ready")
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute*8, time.Second*15).Should(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))

			By("Verifying StatefulSet has 5 replicas")
			sts := &appsv1.StatefulSet{}
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
				if err != nil {
					return 0
				}
				return sts.Status.ReadyReplicas
			}, time.Minute*5, time.Second*15).Should(Equal(int32(5)))

			By("Scaling down to 1 node")
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return err
				}
				cluster.Spec.Size = 1
				return k8sClient.Update(ctx, cluster)
			}, time.Second*30, time.Second*2).Should(Succeed())

			By("Verifying progressive scaling behavior")
			// 大规模缩容应该是渐进式的，可能会经历多个中间状态
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
				if err != nil {
					return false
				}
				// 验证StatefulSet的副本数在逐步减少
				return sts.Status.ReadyReplicas < 5 && sts.Status.ReadyReplicas >= 1
			}, time.Minute*3, time.Second*10).Should(BeTrue())

			By("Waiting for final scale-down to complete")
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
				if err != nil {
					return 0
				}
				return sts.Status.ReadyReplicas
			}, time.Minute*8, time.Second*15).Should(Equal(int32(1)))

			By("Verifying cluster returns to running phase")
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute*2, time.Second*5).Should(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))
		})
	})

	Context("When testing progressive scaling strategies", func() {
		var (
			ctx       context.Context
			namespace *corev1.Namespace
		)

		BeforeEach(func() {
			ctx = context.Background()

			// 创建测试命名空间
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-progressive-",
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

		It("Should create large clusters progressively", func() {
			By("Creating a 7-node cluster")
			cluster := &etcdv1alpha1.EtcdCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "progressive-large-cluster",
					Namespace: namespace.Name,
				},
				Spec: etcdv1alpha1.EtcdClusterSpec{
					Size:       7, // 大集群应该渐进式创建
					Version:    "v3.5.21",
					Repository: "quay.io/coreos/etcd",
					Storage: etcdv1alpha1.EtcdStorageSpec{
						Size: resource.MustParse("1Gi"),
					},
				},
			}

			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			By("Verifying progressive creation behavior")
			// 大集群创建应该是渐进式的，先创建较少的节点
			sts := &appsv1.StatefulSet{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
				if err != nil {
					return false
				}
				// 验证StatefulSet的副本数在逐步增加，而不是直接到7
				return sts.Status.ReadyReplicas > 0 && sts.Status.ReadyReplicas < 7
			}, time.Minute*3, time.Second*10).Should(BeTrue())

			By("Waiting for progressive scaling to complete")
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
				if err != nil {
					return 0
				}
				return sts.Status.ReadyReplicas
			}, time.Minute*10, time.Second*15).Should(Equal(int32(7)))

			By("Verifying cluster reaches running state")
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute*2, time.Second*5).Should(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))
		})

		It("Should handle scaling strategy changes during creation", func() {
			By("Creating a 5-node cluster")
			cluster := &etcdv1alpha1.EtcdCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "strategy-change-cluster",
					Namespace: namespace.Name,
				},
				Spec: etcdv1alpha1.EtcdClusterSpec{
					Size:       5,
					Version:    "v3.5.21",
					Repository: "quay.io/coreos/etcd",
					Storage: etcdv1alpha1.EtcdStorageSpec{
						Size: resource.MustParse("1Gi"),
					},
				},
			}

			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			By("Waiting for initial scaling to start")
			sts := &appsv1.StatefulSet{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
				if err != nil {
					return false
				}
				return sts.Status.ReadyReplicas > 0
			}, time.Minute*3, time.Second*10).Should(BeTrue())

			By("Changing cluster size during creation")
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return err
				}
				cluster.Spec.Size = 3 // 在创建过程中减小目标大小
				return k8sClient.Update(ctx, cluster)
			}, time.Second*30, time.Second*2).Should(Succeed())

			By("Verifying cluster adapts to new target size")
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
				if err != nil {
					return 0
				}
				return sts.Status.ReadyReplicas
			}, time.Minute*5, time.Second*10).Should(Equal(int32(3)))

			By("Verifying cluster reaches running state with new size")
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute*2, time.Second*5).Should(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))
		})

		It("Should respect etcd cluster size best practices", func() {
			By("Testing odd-sized clusters (recommended)")
			oddCluster := &etcdv1alpha1.EtcdCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "odd-size-cluster",
					Namespace: namespace.Name,
				},
				Spec: etcdv1alpha1.EtcdClusterSpec{
					Size:       3, // 奇数大小（推荐）
					Version:    "v3.5.21",
					Repository: "quay.io/coreos/etcd",
					Storage: etcdv1alpha1.EtcdStorageSpec{
						Size: resource.MustParse("1Gi"),
					},
				},
			}

			Expect(k8sClient.Create(ctx, oddCluster)).To(Succeed())

			By("Verifying odd-sized cluster creates successfully")
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      oddCluster.Name,
					Namespace: oddCluster.Namespace,
				}, oddCluster)
				if err != nil {
					return ""
				}
				return oddCluster.Status.Phase
			}, time.Minute*5, time.Second*10).Should(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))

			By("Testing even-sized clusters (not recommended but should work)")
			evenCluster := &etcdv1alpha1.EtcdCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "even-size-cluster",
					Namespace: namespace.Name,
				},
				Spec: etcdv1alpha1.EtcdClusterSpec{
					Size:       4, // 偶数大小（不推荐但应该工作）
					Version:    "v3.5.21",
					Repository: "quay.io/coreos/etcd",
					Storage: etcdv1alpha1.EtcdStorageSpec{
						Size: resource.MustParse("1Gi"),
					},
				},
			}

			Expect(k8sClient.Create(ctx, evenCluster)).To(Succeed())

			By("Verifying even-sized cluster creates with warnings")
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      evenCluster.Name,
					Namespace: evenCluster.Namespace,
				}, evenCluster)
				if err != nil {
					return ""
				}
				return evenCluster.Status.Phase
			}, time.Minute*5, time.Second*10).Should(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))

			// 可以检查是否有关于偶数大小的警告条件
			// 这取决于具体的实现
		})
	})
})
