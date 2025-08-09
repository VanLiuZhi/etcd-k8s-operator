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

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	"github.com/your-org/etcd-k8s-operator/pkg/client"
)

// resourceManager 资源管理器实现
type resourceManager struct {
	k8sClient      client.KubernetesClient
	statefulSetMgr StatefulSetManager
	serviceMgr     ServiceManager
	configMapMgr   ConfigMapManager
	pvcMgr         PVCManager
}

// NewResourceManager 创建资源管理器实例
func NewResourceManager(k8sClient client.KubernetesClient) ResourceManager {
	return &resourceManager{
		k8sClient:      k8sClient,
		statefulSetMgr: NewStatefulSetManager(k8sClient),
		serviceMgr:     NewServiceManager(k8sClient),
		configMapMgr:   NewConfigMapManager(k8sClient),
		pvcMgr:         NewPVCManager(k8sClient),
	}
}

// EnsureAllResources 确保所有资源存在
func (rm *resourceManager) EnsureAllResources(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	// 1. 确保 ConfigMap
	if err := rm.configMapMgr.Ensure(ctx, cluster); err != nil {
		return fmt.Errorf("failed to ensure ConfigMap: %w", err)
	}

	// 2. 确保 Services
	if err := rm.serviceMgr.EnsureServices(ctx, cluster); err != nil {
		return fmt.Errorf("failed to ensure Services: %w", err)
	}

	// 3. 确保 StatefulSet
	if err := rm.statefulSetMgr.Ensure(ctx, cluster); err != nil {
		return fmt.Errorf("failed to ensure StatefulSet: %w", err)
	}

	return nil
}

// CleanupResources 清理资源
func (rm *resourceManager) CleanupResources(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	// 1. 删除 StatefulSet
	if err := rm.statefulSetMgr.Delete(ctx, cluster); err != nil {
		return fmt.Errorf("failed to delete StatefulSet: %w", err)
	}

	// 2. 删除 Services
	if err := rm.serviceMgr.Delete(ctx, cluster, "client"); err != nil {
		return fmt.Errorf("failed to delete client service: %w", err)
	}
	if err := rm.serviceMgr.Delete(ctx, cluster, "peer"); err != nil {
		return fmt.Errorf("failed to delete peer service: %w", err)
	}

	// 3. 删除 ConfigMap
	if err := rm.configMapMgr.Delete(ctx, cluster); err != nil {
		return fmt.Errorf("failed to delete ConfigMap: %w", err)
	}

	// 4. 清理 PVCs (可选，根据策略决定)
	if err := rm.pvcMgr.CleanupAll(ctx, cluster); err != nil {
		return fmt.Errorf("failed to cleanup PVCs: %w", err)
	}

	return nil
}

// StatefulSet 获取 StatefulSet 管理器
func (rm *resourceManager) StatefulSet() StatefulSetManager {
	return rm.statefulSetMgr
}

// Service 获取 Service 管理器
func (rm *resourceManager) Service() ServiceManager {
	return rm.serviceMgr
}

// ConfigMap 获取 ConfigMap 管理器
func (rm *resourceManager) ConfigMap() ConfigMapManager {
	return rm.configMapMgr
}

// PVC 获取 PVC 管理器
func (rm *resourceManager) PVC() PVCManager {
	return rm.pvcMgr
}
