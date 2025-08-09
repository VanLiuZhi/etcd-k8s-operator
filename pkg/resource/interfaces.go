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

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// StatefulSetManager StatefulSet 管理器接口
type StatefulSetManager interface {
	// 基础 CRUD 操作
	Ensure(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error
	EnsureWithReplicas(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, replicas int32) error
	Get(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (*appsv1.StatefulSet, error)
	Update(ctx context.Context, existing *appsv1.StatefulSet, desired *appsv1.StatefulSet) error
	Delete(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error

	// 状态查询
	GetStatus(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (*StatefulSetStatus, error)
	NeedsUpdate(existing, desired *appsv1.StatefulSet) bool
}

// ServiceManager Service 管理器接口
type ServiceManager interface {
	// 基础 CRUD 操作
	EnsureServices(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error
	EnsureClientService(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error
	EnsurePeerService(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error
	Get(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, serviceType string) (*corev1.Service, error)
	Delete(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, serviceType string) error

	// 服务发现
	GetServiceEndpoints(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) ([]string, error)
}

// ConfigMapManager ConfigMap 管理器接口
type ConfigMapManager interface {
	// 基础 CRUD 操作
	Ensure(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error
	Get(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (*corev1.ConfigMap, error)
	Update(ctx context.Context, existing *corev1.ConfigMap, desired *corev1.ConfigMap) error
	Delete(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error

	// 配置生成
	GenerateEtcdConfig(cluster *etcdv1alpha1.EtcdCluster) (map[string]string, error)
}

// PVCManager PVC 管理器接口
type PVCManager interface {
	// PVC 操作
	List(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) ([]corev1.PersistentVolumeClaim, error)
	CleanupExtra(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, currentSize int32) error
	CleanupAll(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error
}

// ResourceManager 资源管理器聚合接口
type ResourceManager interface {
	// 聚合操作
	EnsureAllResources(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error
	CleanupResources(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error

	// 获取各个管理器
	StatefulSet() StatefulSetManager
	Service() ServiceManager
	ConfigMap() ConfigMapManager
	PVC() PVCManager
}

// StatefulSetStatus StatefulSet 状态
type StatefulSetStatus struct {
	Replicas        int32
	ReadyReplicas   int32
	CurrentReplicas int32
	UpdatedReplicas int32
	Conditions      []Condition
}

// Condition 条件信息
type Condition struct {
	Type    string
	Status  string
	Reason  string
	Message string
}
