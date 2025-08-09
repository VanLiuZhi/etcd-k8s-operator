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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	"github.com/your-org/etcd-k8s-operator/pkg/client"
	"github.com/your-org/etcd-k8s-operator/pkg/k8s"
)

// configMapManager ConfigMap 管理器实现
type configMapManager struct {
	k8sClient client.KubernetesClient
}

// NewConfigMapManager 创建 ConfigMap 管理器
func NewConfigMapManager(k8sClient client.KubernetesClient) ConfigMapManager {
	return &configMapManager{
		k8sClient: k8sClient,
	}
}

// Ensure 确保 ConfigMap 存在
func (cm *configMapManager) Ensure(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	desired := k8s.BuildConfigMap(cluster)

	existing := &corev1.ConfigMap{}
	err := cm.k8sClient.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, existing)

	if errors.IsNotFound(err) {
		// 不存在，创建新的
		// 设置ControllerReference（如果客户端支持）
		if client := cm.k8sClient.GetClient(); client != nil {
			if err := ctrl.SetControllerReference(cluster, desired, client.Scheme()); err != nil {
				return err
			}
		}
		return cm.k8sClient.Create(ctx, desired)
	} else if err != nil {
		return err
	}

	// 已存在，检查是否需要更新
	if cm.needsUpdate(existing, desired) {
		return cm.Update(ctx, existing, desired)
	}

	return nil
}

// Get 获取 ConfigMap
func (cm *configMapManager) Get(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	err := cm.k8sClient.Get(ctx, types.NamespacedName{
		Name:      cluster.Name + "-config",
		Namespace: cluster.Namespace,
	}, configMap)
	return configMap, err
}

// Update 更新 ConfigMap
func (cm *configMapManager) Update(ctx context.Context, existing *corev1.ConfigMap, desired *corev1.ConfigMap) error {
	existing.Data = desired.Data
	existing.Labels = desired.Labels
	existing.Annotations = desired.Annotations

	return cm.k8sClient.Update(ctx, existing)
}

// Delete 删除 ConfigMap
func (cm *configMapManager) Delete(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	configMap, err := cm.Get(ctx, cluster)
	if errors.IsNotFound(err) {
		return nil // 已经不存在
	} else if err != nil {
		return err
	}

	return cm.k8sClient.Delete(ctx, configMap)
}

// GenerateEtcdConfig 生成 etcd 配置
func (cm *configMapManager) GenerateEtcdConfig(cluster *etcdv1alpha1.EtcdCluster) (map[string]string, error) {
	// TODO: 实现配置生成逻辑
	return map[string]string{
		"etcd.conf": "# etcd configuration",
	}, nil
}

// needsUpdate 检查是否需要更新
func (cm *configMapManager) needsUpdate(existing, desired *corev1.ConfigMap) bool {
	// 检查数据
	if len(existing.Data) != len(desired.Data) {
		return true
	}

	for key, existingValue := range existing.Data {
		if desiredValue, exists := desired.Data[key]; !exists || existingValue != desiredValue {
			return true
		}
	}

	return false
}
