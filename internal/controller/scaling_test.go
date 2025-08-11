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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
)

var _ = Describe("EtcdCluster Scaling", func() {
	Context("When scaling etcd clusters", func() {
		var (
			ctx       context.Context
			namespace *corev1.Namespace
			cluster   *etcdv1alpha1.EtcdCluster
		)

		BeforeEach(func() {
			ctx = context.Background()

			// 创建测试命名空间
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-scaling-",
				},
			}
			Expect(k8sClient.Create(ctx, namespace)).To(Succeed())

			// 创建初始的3节点集群
			cluster = &etcdv1alpha1.EtcdCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-scaling-cluster",
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
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			// 等待集群就绪
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
		})

		AfterEach(func() {
			// 清理测试命名空间
			if namespace != nil {
				Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
			}
		})

		It("Should scale up from 3 to 5 nodes", func() {
			// 更新集群大小
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

			// 验证集群进入扩容状态
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

			// 验证最终达到5个就绪副本
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return 0
				}
				return cluster.Status.ReadyReplicas
			}, time.Minute*8, time.Second*10).Should(Equal(int32(5)))

			// 验证集群回到运行状态
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

			// 验证成员数量
			Eventually(func() int {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return 0
				}
				return len(cluster.Status.Members)
			}, time.Minute*2, time.Second*5).Should(Equal(5))
		})

		It("Should scale down from 3 to 1 node", func() {
			// 更新集群大小
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

			// 验证集群进入扩容状态
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

			// 验证最终达到1个就绪副本
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return 0
				}
				return cluster.Status.ReadyReplicas
			}, time.Minute*5, time.Second*10).Should(Equal(int32(1)))

			// 验证集群回到运行状态
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

			// 验证成员数量
			Eventually(func() int {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return 0
				}
				return len(cluster.Status.Members)
			}, time.Minute*2, time.Second*5).Should(Equal(1))
		})

		It("Should handle rapid scaling changes", func() {
			// 快速扩容到7节点
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return err
				}
				cluster.Spec.Size = 7
				return k8sClient.Update(ctx, cluster)
			}, time.Second*30, time.Second*2).Should(Succeed())

			// 等待一段时间后再缩容到5节点
			time.Sleep(time.Second * 30)

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

			// 验证最终达到5个就绪副本
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return 0
				}
				return cluster.Status.ReadyReplicas
			}, time.Minute*10, time.Second*15).Should(Equal(int32(5)))

			// 验证集群最终回到运行状态
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute*3, time.Second*10).Should(Equal(etcdv1alpha1.EtcdClusterPhaseRunning))
		})
	})
})
