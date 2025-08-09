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

package client

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EtcdClient etcd 客户端接口
type EtcdClient interface {
	// 连接管理
	Connect(ctx context.Context, endpoints []string) error
	Disconnect() error
	IsConnected() bool

	// 集群管理
	AddMember(ctx context.Context, peerURL string) (*MemberAddResponse, error)
	RemoveMember(ctx context.Context, memberID uint64) (*MemberRemoveResponse, error)
	ListMembers(ctx context.Context) (*MemberListResponse, error)

	// 健康检查
	HealthCheck(ctx context.Context, endpoint string) (*HealthCheckResponse, error)

	// 集群状态
	GetClusterStatus(ctx context.Context) (*EtcdClusterStatus, error)
}

// KubernetesClient Kubernetes 客户端接口
type KubernetesClient interface {
	// 基础操作
	Create(ctx context.Context, obj client.Object) error
	Update(ctx context.Context, obj client.Object) error
	Delete(ctx context.Context, obj client.Object) error
	Get(ctx context.Context, key client.ObjectKey, obj client.Object) error
	List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error

	// 状态更新
	UpdateStatus(ctx context.Context, obj client.Object) error

	// 事件记录
	RecordEvent(obj runtime.Object, eventType, reason, message string)

	// 获取原始客户端
	GetClient() client.Client
	GetRecorder() record.EventRecorder
}

// MemberAddResponse 添加成员响应
type MemberAddResponse struct {
	Member *EtcdMember
	Error  error
}

// MemberRemoveResponse 移除成员响应
type MemberRemoveResponse struct {
	Error error
}

// MemberListResponse 成员列表响应
type MemberListResponse struct {
	Members []*EtcdMember
	Error   error
}

// HealthCheckResponse 健康检查响应
type HealthCheckResponse struct {
	Healthy bool
	Error   error
}

// EtcdClusterStatus etcd 集群状态
type EtcdClusterStatus struct {
	Members   []*EtcdMember
	Leader    *EtcdMember
	ClusterID string
	Version   string
	IsHealthy bool
}

// EtcdMember etcd 成员信息
type EtcdMember struct {
	ID         uint64
	Name       string
	PeerURLs   []string
	ClientURLs []string
	IsLeader   bool
	IsHealthy  bool
}
