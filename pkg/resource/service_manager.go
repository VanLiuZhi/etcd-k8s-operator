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

package resource

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	"github.com/your-org/etcd-k8s-operator/pkg/client"
	"github.com/your-org/etcd-k8s-operator/pkg/k8s"
)

// serviceManager Service 管理器实现
type serviceManager struct {
	k8sClient client.KubernetesClient
}

// NewServiceManager 创建 Service 管理器
func NewServiceManager(k8sClient client.KubernetesClient) ServiceManager {
	return &serviceManager{
		k8sClient: k8sClient,
	}
}

// EnsureServices 确保所有服务存在
func (sm *serviceManager) EnsureServices(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	// 确保客户端服务
	if err := sm.EnsureClientService(ctx, cluster); err != nil {
		return fmt.Errorf("failed to ensure client service: %w", err)
	}

	// 确保对等服务
	if err := sm.EnsurePeerService(ctx, cluster); err != nil {
		return fmt.Errorf("failed to ensure peer service: %w", err)
	}

	return nil
}

// EnsureClientService 确保客户端服务存在
func (sm *serviceManager) EnsureClientService(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	return sm.ensureService(ctx, cluster, "client", k8s.BuildClientService(cluster))
}

// EnsurePeerService 确保对等服务存在
func (sm *serviceManager) EnsurePeerService(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	return sm.ensureService(ctx, cluster, "peer", k8s.BuildPeerService(cluster))
}

// ensureService 确保服务存在的通用方法
func (sm *serviceManager) ensureService(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, serviceType string, desired *corev1.Service) error {
	existing := &corev1.Service{}
	err := sm.k8sClient.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, existing)

	if errors.IsNotFound(err) {
		// 不存在，创建新的
		// 设置ControllerReference（如果客户端支持）
		if client := sm.k8sClient.GetClient(); client != nil {
			if err := ctrl.SetControllerReference(cluster, desired, client.Scheme()); err != nil {
				return err
			}
		}
		return sm.k8sClient.Create(ctx, desired)
	} else if err != nil {
		return err
	}

	// 已存在，检查是否需要更新
	if sm.needsServiceUpdate(existing, desired) {
		existing.Spec.Ports = desired.Spec.Ports
		existing.Spec.Selector = desired.Spec.Selector
		existing.Labels = desired.Labels
		existing.Annotations = desired.Annotations

		return sm.k8sClient.Update(ctx, existing)
	}

	return nil
}

// Get 获取服务
func (sm *serviceManager) Get(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, serviceType string) (*corev1.Service, error) {
	var serviceName string
	switch serviceType {
	case "client":
		serviceName = cluster.Name + "-client"
	case "peer":
		serviceName = cluster.Name
	default:
		return nil, fmt.Errorf("unknown service type: %s", serviceType)
	}

	svc := &corev1.Service{}
	err := sm.k8sClient.Get(ctx, types.NamespacedName{
		Name:      serviceName,
		Namespace: cluster.Namespace,
	}, svc)
	return svc, err
}

// Delete 删除服务
func (sm *serviceManager) Delete(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, serviceType string) error {
	svc, err := sm.Get(ctx, cluster, serviceType)
	if errors.IsNotFound(err) {
		return nil // 已经不存在
	} else if err != nil {
		return err
	}

	return sm.k8sClient.Delete(ctx, svc)
}

// GetServiceEndpoints 获取服务端点
func (sm *serviceManager) GetServiceEndpoints(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) ([]string, error) {
	// TODO: 实现获取服务端点的逻辑
	endpoints := make([]string, 0, cluster.Spec.Size)
	for i := int32(0); i < cluster.Spec.Size; i++ {
		endpoint := fmt.Sprintf("%s-%d.%s.%s.svc.cluster.local:2379",
			cluster.Name, i, cluster.Name, cluster.Namespace)
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, nil
}

// needsServiceUpdate 检查服务是否需要更新
func (sm *serviceManager) needsServiceUpdate(existing, desired *corev1.Service) bool {
	// 检查端口
	if len(existing.Spec.Ports) != len(desired.Spec.Ports) {
		return true
	}

	for i, existingPort := range existing.Spec.Ports {
		if i >= len(desired.Spec.Ports) {
			return true
		}
		desiredPort := desired.Spec.Ports[i]
		if existingPort.Port != desiredPort.Port ||
			existingPort.TargetPort != desiredPort.TargetPort ||
			existingPort.Protocol != desiredPort.Protocol {
			return true
		}
	}

	// 检查选择器
	if len(existing.Spec.Selector) != len(desired.Spec.Selector) {
		return true
	}

	for key, existingValue := range existing.Spec.Selector {
		if desiredValue, exists := desired.Spec.Selector[key]; !exists || existingValue != desiredValue {
			return true
		}
	}

	return false
}
