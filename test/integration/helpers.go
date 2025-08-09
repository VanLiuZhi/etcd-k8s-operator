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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
)

// TestHelper 集成测试辅助工具
type TestHelper struct {
	client    client.Client
	namespace string
}

// NewTestHelper 创建测试辅助工具
func NewTestHelper(client client.Client, namespace string) *TestHelper {
	return &TestHelper{
		client:    client,
		namespace: namespace,
	}
}

// WaitForClusterPhase 等待集群达到指定状态
// 这是集成测试中常用的等待机制，确保异步操作完成
func (h *TestHelper) WaitForClusterPhase(ctx context.Context, clusterName string, expectedPhase etcdv1alpha1.EtcdClusterPhase, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 1*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		cluster := &etcdv1alpha1.EtcdCluster{}
		err := h.client.Get(ctx, types.NamespacedName{
			Name:      clusterName,
			Namespace: h.namespace,
		}, cluster)

		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil // 继续等待
			}
			return false, err // 其他错误
		}

		return cluster.Status.Phase == expectedPhase, nil
	})
}

// WaitForClusterDeletion 等待集群被删除
func (h *TestHelper) WaitForClusterDeletion(ctx context.Context, clusterName string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(ctx, 1*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		cluster := &etcdv1alpha1.EtcdCluster{}
		err := h.client.Get(ctx, types.NamespacedName{
			Name:      clusterName,
			Namespace: h.namespace,
		}, cluster)

		if errors.IsNotFound(err) {
			return true, nil // 已删除
		}

		if err != nil {
			return false, err // 其他错误
		}

		return false, nil // 仍然存在，继续等待
	})
}

// GetClusterStatus 获取集群当前状态
func (h *TestHelper) GetClusterStatus(ctx context.Context, clusterName string) (*etcdv1alpha1.EtcdClusterStatus, error) {
	cluster := &etcdv1alpha1.EtcdCluster{}
	err := h.client.Get(ctx, types.NamespacedName{
		Name:      clusterName,
		Namespace: h.namespace,
	}, cluster)

	if err != nil {
		return nil, err
	}

	return &cluster.Status, nil
}

// ValidateClusterResources 验证集群相关的Kubernetes资源
func (h *TestHelper) ValidateClusterResources(ctx context.Context, clusterName string) error {
	// 这里可以添加更复杂的资源验证逻辑
	// 例如检查StatefulSet、Service、ConfigMap等资源的状态

	// 简单示例：检查集群是否存在
	cluster := &etcdv1alpha1.EtcdCluster{}
	err := h.client.Get(ctx, types.NamespacedName{
		Name:      clusterName,
		Namespace: h.namespace,
	}, cluster)

	if err != nil {
		return fmt.Errorf("集群资源验证失败: %w", err)
	}

	return nil
}

// CleanupCluster 清理测试集群
func (h *TestHelper) CleanupCluster(ctx context.Context, clusterName string) error {
	cluster := &etcdv1alpha1.EtcdCluster{}
	err := h.client.Get(ctx, types.NamespacedName{
		Name:      clusterName,
		Namespace: h.namespace,
	}, cluster)

	if errors.IsNotFound(err) {
		return nil // 已经不存在
	}

	if err != nil {
		return err
	}

	// 删除集群
	err = h.client.Delete(ctx, cluster)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	// 等待删除完成
	return h.WaitForClusterDeletion(ctx, clusterName, 30*time.Second)
}

// LogClusterStatus 记录集群状态（用于调试）
func (h *TestHelper) LogClusterStatus(ctx context.Context, clusterName string) {
	status, err := h.GetClusterStatus(ctx, clusterName)
	if err != nil {
		fmt.Printf("❌ 无法获取集群 %s 状态: %v\n", clusterName, err)
		return
	}

	fmt.Printf("📊 集群 %s 状态: Phase=%s, ReadyReplicas=%d\n",
		clusterName, status.Phase, status.ReadyReplicas)
}
