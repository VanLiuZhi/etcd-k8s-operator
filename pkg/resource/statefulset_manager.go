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

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	"github.com/your-org/etcd-k8s-operator/pkg/client"
	"github.com/your-org/etcd-k8s-operator/pkg/k8s"
)

// statefulSetManager StatefulSet 管理器实现
type statefulSetManager struct {
	k8sClient client.KubernetesClient
}

// NewStatefulSetManager 创建 StatefulSet 管理器
func NewStatefulSetManager(k8sClient client.KubernetesClient) StatefulSetManager {
	return &statefulSetManager{
		k8sClient: k8sClient,
	}
}

// Ensure 确保 StatefulSet 存在
func (sm *statefulSetManager) Ensure(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	return sm.EnsureWithReplicas(ctx, cluster, cluster.Spec.Size)
}

// EnsureWithReplicas 确保 StatefulSet 存在并设置副本数
func (sm *statefulSetManager) EnsureWithReplicas(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, replicas int32) error {
	// 构建期望的 StatefulSet
	var desired *appsv1.StatefulSet
	if replicas == cluster.Spec.Size {
		desired = k8s.BuildStatefulSet(cluster)
	} else {
		desired = k8s.BuildStatefulSetWithReplicas(cluster, replicas)
	}

	// 检查是否已存在
	existing := &appsv1.StatefulSet{}
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
		createErr := sm.k8sClient.Create(ctx, desired)
		if createErr != nil && errors.IsAlreadyExists(createErr) {
			// 竞态条件：在Get和Create之间，资源被其他goroutine创建了
			// 重新获取资源并继续更新检查逻辑
			if getErr := sm.k8sClient.Get(ctx, types.NamespacedName{
				Name:      desired.Name,
				Namespace: desired.Namespace,
			}, existing); getErr != nil {
				return getErr
			}
			// 继续执行更新检查逻辑
		} else if createErr != nil {
			return createErr
		} else {
			// 创建成功，直接返回
			return nil
		}
	} else if err != nil {
		return err
	}

	// 已存在，检查是否需要更新
	if sm.NeedsUpdate(existing, desired) {
		return sm.Update(ctx, existing, desired)
	}

	return nil
}

// Get 获取 StatefulSet
func (sm *statefulSetManager) Get(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (*appsv1.StatefulSet, error) {
	sts := &appsv1.StatefulSet{}
	err := sm.k8sClient.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, sts)
	return sts, err
}

// Update 更新 StatefulSet
func (sm *statefulSetManager) Update(ctx context.Context, existing *appsv1.StatefulSet, desired *appsv1.StatefulSet) error {
	// 保留一些不应该更新的字段
	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	existing.Annotations = desired.Annotations

	return sm.k8sClient.Update(ctx, existing)
}

// Delete 删除 StatefulSet
func (sm *statefulSetManager) Delete(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	sts := &appsv1.StatefulSet{}
	err := sm.k8sClient.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, sts)

	if errors.IsNotFound(err) {
		return nil // 已经不存在
	} else if err != nil {
		return err
	}

	return sm.k8sClient.Delete(ctx, sts)
}

// GetStatus 获取 StatefulSet 状态
func (sm *statefulSetManager) GetStatus(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (*StatefulSetStatus, error) {
	sts, err := sm.Get(ctx, cluster)
	if err != nil {
		if errors.IsNotFound(err) {
			// StatefulSet 还没有创建，返回零值状态
			return &StatefulSetStatus{
				Replicas:        0,
				ReadyReplicas:   0,
				CurrentReplicas: 0,
				UpdatedReplicas: 0,
			}, nil
		}
		return nil, err
	}

	return &StatefulSetStatus{
		Replicas:        sts.Status.Replicas,
		ReadyReplicas:   sts.Status.ReadyReplicas,
		CurrentReplicas: sts.Status.CurrentReplicas,
		UpdatedReplicas: sts.Status.UpdatedReplicas,
	}, nil
}

// NeedsUpdate 检查是否需要更新
func (sm *statefulSetManager) NeedsUpdate(existing, desired *appsv1.StatefulSet) bool {
	// 检查副本数
	if *existing.Spec.Replicas != *desired.Spec.Replicas {
		return true
	}

	// 检查 Pod 模板注解（用于触发滚动更新）
	if len(existing.Spec.Template.Annotations) != len(desired.Spec.Template.Annotations) {
		return true
	}
	for k, v := range desired.Spec.Template.Annotations {
		if existing.Spec.Template.Annotations[k] != v {
			return true
		}
	}

	// 检查容器镜像版本
	if len(existing.Spec.Template.Spec.Containers) > 0 && len(desired.Spec.Template.Spec.Containers) > 0 {
		if existing.Spec.Template.Spec.Containers[0].Image != desired.Spec.Template.Spec.Containers[0].Image {
			return true
		}
	}

	// 检查 Init Containers（脚本/命令变更）
	existingInits := existing.Spec.Template.Spec.InitContainers
	desiredInits := desired.Spec.Template.Spec.InitContainers
	if len(existingInits) != len(desiredInits) {
		return true
	}
	for i := range desiredInits {
		if desiredInits[i].Name != existingInits[i].Name ||
			desiredInits[i].Image != existingInits[i].Image ||
			len(desiredInits[i].Command) != len(existingInits[i].Command) ||
			len(desiredInits[i].Args) != len(existingInits[i].Args) {
			return true
		}
		for j := range desiredInits[i].Command {
			if desiredInits[i].Command[j] != existingInits[i].Command[j] {
				return true
			}
		}
		for j := range desiredInits[i].Args {
			if desiredInits[i].Args[j] != existingInits[i].Args[j] {
				return true
			}
		}
	}

	// 检查资源限制
	existingResources := existing.Spec.Template.Spec.Containers[0].Resources
	desiredResources := desired.Spec.Template.Spec.Containers[0].Resources

	if !existingResources.Requests.Memory().Equal(*desiredResources.Requests.Memory()) ||
		!existingResources.Requests.Cpu().Equal(*desiredResources.Requests.Cpu()) {
		return true
	}

	// TODO: 添加更多需要检查的字段

	return false
}
