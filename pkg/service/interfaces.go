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

package service

import (
	"context"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ClusterService 集群管理服务接口
type ClusterService interface {
	// 集群生命周期管理
	InitializeCluster(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error)
	CreateCluster(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error)
	UpdateClusterStatus(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error
	DeleteCluster(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error)

	// 集群状态管理
	GetClusterStatus(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (*ClusterStatus, error)
	IsClusterReady(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (bool, error)

	// 集群验证
	ValidateClusterSpec(cluster *etcdv1alpha1.EtcdCluster) error
	SetDefaults(cluster *etcdv1alpha1.EtcdCluster)
}

// ScalingService 扩缩容服务接口
type ScalingService interface {
	// 扩缩容操作
	HandleRunning(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error)
	HandleScaling(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error)
	HandleStopped(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error)

	// 扩缩容检查
	NeedsScaling(cluster *etcdv1alpha1.EtcdCluster) bool

	// 扩缩容验证
	ValidateScaling(cluster *etcdv1alpha1.EtcdCluster, targetSize int32) error
}

// HealthService 健康检查服务接口
type HealthService interface {
	// 健康检查
	CheckClusterHealth(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (*HealthStatus, error)
	PerformHealthCheck(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error

	// 故障处理
	HandleFailed(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error)
}

// ClusterStatus 集群状态信息
type ClusterStatus struct {
	Phase           string
	ReadyReplicas   int32
	Members         []MemberInfo
	LeaderID        string
	ClusterID       string
	ClientEndpoints []string
	Conditions      []Condition
}

// MemberInfo etcd 成员信息
type MemberInfo struct {
	Name      string
	ID        string
	PeerURL   string
	ClientURL string
	Ready     bool
	Role      string
}

// HealthStatus 健康状态
type HealthStatus struct {
	Overall   string
	Members   []MemberHealth
	LastCheck int64
	Issues    []HealthIssue
}

// MemberHealth 成员健康状态
type MemberHealth struct {
	Name     string
	Status   string
	LastSeen int64
	Errors   []string
}

// HealthIssue 健康问题
type HealthIssue struct {
	Type        string
	Description string
	Severity    string
}

// Condition 条件信息
type Condition struct {
	Type    string
	Status  string
	Reason  string
	Message string
}
