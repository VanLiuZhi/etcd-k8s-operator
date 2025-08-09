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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	clientpkg "github.com/your-org/etcd-k8s-operator/pkg/client"
	"github.com/your-org/etcd-k8s-operator/pkg/utils"
)

// pvcManager PVC 管理器实现
type pvcManager struct {
	k8sClient clientpkg.KubernetesClient
}

// NewPVCManager 创建 PVC 管理器
func NewPVCManager(k8sClient clientpkg.KubernetesClient) PVCManager {
	return &pvcManager{
		k8sClient: k8sClient,
	}
}

// List 列出 PVC
func (pm *pvcManager) List(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) ([]corev1.PersistentVolumeClaim, error) {
	pvcList := &corev1.PersistentVolumeClaimList{}

	// 使用标签选择器查找相关的 PVC
	labelSelector := client.MatchingLabels{
		utils.LabelAppName:     "etcd",
		utils.LabelAppInstance: cluster.Name,
	}

	err := pm.k8sClient.List(ctx, pvcList,
		client.InNamespace(cluster.Namespace),
		labelSelector,
	)

	if err != nil {
		return nil, err
	}

	return pvcList.Items, nil
}

// CleanupExtra 清理多余的 PVC
func (pm *pvcManager) CleanupExtra(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, currentSize int32) error {
	pvcs, err := pm.List(ctx, cluster)
	if err != nil {
		return err
	}

	// 删除超出当前大小的 PVC
	for _, pvc := range pvcs {
		// 从 PVC 名称中提取索引 (格式: data-clustername-N)
		// TODO: 实现 PVC 索引提取和清理逻辑
		_ = pvc
	}

	return nil
}

// CleanupAll 清理所有 PVC
func (pm *pvcManager) CleanupAll(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	pvcs, err := pm.List(ctx, cluster)
	if err != nil {
		return err
	}

	// 删除所有相关的 PVC
	for _, pvc := range pvcs {
		if err := pm.k8sClient.Delete(ctx, &pvc); err != nil {
			return err
		}
	}

	return nil
}
