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

package integration

import (
	"context"
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

var _ = Describe("EtcdCluster Controller", func() {
	Context("When creating an EtcdCluster", func() {
		It("Should create the necessary resources", func() {
			ctx := context.Background()

			// Create a test EtcdCluster
			cluster := &etcdv1alpha1.EtcdCluster{
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
					Security: etcdv1alpha1.EtcdSecuritySpec{
						TLS: etcdv1alpha1.EtcdTLSSpec{
							Enabled: true,
							AutoTLS: true,
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())

			// Wait for the cluster to be created
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				return err == nil
			}, time.Minute, time.Second).Should(BeTrue())

			// Check if StatefulSet is created
			sts := &appsv1.StatefulSet{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
				return err == nil
			}, time.Minute, time.Second).Should(BeTrue())

			// Verify StatefulSet configuration
			Expect(*sts.Spec.Replicas).Should(Equal(cluster.Spec.Size))
			Expect(sts.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(sts.Spec.Template.Spec.Containers[0].Image).Should(Equal("quay.io/coreos/etcd:3.5.9"))

			// Check if Services are created
			clientService := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name + "-client",
					Namespace: cluster.Namespace,
				}, clientService)
				return err == nil
			}, time.Minute, time.Second).Should(BeTrue())

			peerService := &corev1.Service{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name + "-peer",
					Namespace: cluster.Namespace,
				}, peerService)
				return err == nil
			}, time.Minute, time.Second).Should(BeTrue())

			// Verify service configuration
			Expect(clientService.Spec.Ports).Should(HaveLen(1))
			Expect(clientService.Spec.Ports[0].Port).Should(Equal(int32(utils.EtcdClientPort)))

			Expect(peerService.Spec.ClusterIP).Should(Equal(corev1.ClusterIPNone)) // Headless service
			Expect(peerService.Spec.Ports).Should(HaveLen(1))
			Expect(peerService.Spec.Ports[0].Port).Should(Equal(int32(utils.EtcdPeerPort)))

			// Check if ConfigMap is created
			configMap := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name + "-config",
					Namespace: cluster.Namespace,
				}, configMap)
				return err == nil
			}, time.Minute, time.Second).Should(BeTrue())

			// Verify ConfigMap has etcd configuration
			Expect(configMap.Data).Should(HaveKey("etcd.conf"))
			Expect(configMap.Data["etcd.conf"]).Should(ContainSubstring("name: $(ETCD_NAME)"))

			// Check cluster status
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				if err != nil {
					return ""
				}
				return cluster.Status.Phase
			}, time.Minute, time.Second).Should(Equal(etcdv1alpha1.EtcdClusterPhaseCreating))

			// Cleanup
			Expect(k8sClient.Delete(ctx, cluster)).Should(Succeed())
		})

		It("Should validate cluster specification", func() {
			ctx := context.Background()

			// Test with even number of replicas (should fail validation)
			invalidCluster := &etcdv1alpha1.EtcdCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-cluster",
					Namespace: "default",
				},
				Spec: etcdv1alpha1.EtcdClusterSpec{
					Size:    2, // Even number - should be invalid
					Version: "3.5.9",
				},
			}

			Expect(k8sClient.Create(ctx, invalidCluster)).Should(Succeed())

			// Wait for the cluster to be processed and check if it fails
			Eventually(func() etcdv1alpha1.EtcdClusterPhase {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      invalidCluster.Name,
					Namespace: invalidCluster.Namespace,
				}, invalidCluster)
				if err != nil {
					return ""
				}
				return invalidCluster.Status.Phase
			}, time.Minute, time.Second).Should(Equal(etcdv1alpha1.EtcdClusterPhaseFailed))

			// Check if there's a condition indicating the failure
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      invalidCluster.Name,
					Namespace: invalidCluster.Namespace,
				}, invalidCluster)
				if err != nil {
					return false
				}

				for _, condition := range invalidCluster.Status.Conditions {
					if condition.Type == utils.ConditionTypeReady &&
						condition.Status == metav1.ConditionFalse &&
						condition.Reason == utils.ReasonFailed {
						return true
					}
				}
				return false
			}, time.Minute, time.Second).Should(BeTrue())

			// Cleanup
			Expect(k8sClient.Delete(ctx, invalidCluster)).Should(Succeed())
		})
	})

	Context("When updating an EtcdCluster", func() {
		It("Should handle scaling operations", func() {
			ctx := context.Background()

			// Create a test EtcdCluster
			cluster := &etcdv1alpha1.EtcdCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "scale-test-cluster",
					Namespace: "default",
				},
				Spec: etcdv1alpha1.EtcdClusterSpec{
					Size:       3,
					Version:    "3.5.9",
					Repository: "quay.io/coreos/etcd",
				},
			}

			Expect(k8sClient.Create(ctx, cluster)).Should(Succeed())

			// Wait for initial creation
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, cluster)
				return err == nil && cluster.Status.Phase != ""
			}, time.Minute, time.Second).Should(BeTrue())

			// Scale up to 5 replicas
			cluster.Spec.Size = 5
			Expect(k8sClient.Update(ctx, cluster)).Should(Succeed())

			// Check if StatefulSet is updated
			sts := &appsv1.StatefulSet{}
			Eventually(func() int32 {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      cluster.Name,
					Namespace: cluster.Namespace,
				}, sts)
				if err != nil {
					return 0
				}
				return *sts.Spec.Replicas
			}, time.Minute, time.Second).Should(Equal(int32(5)))

			// Cleanup
			Expect(k8sClient.Delete(ctx, cluster)).Should(Succeed())
		})
	})
})
