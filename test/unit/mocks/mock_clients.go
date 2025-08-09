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

package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	clientpkg "github.com/your-org/etcd-k8s-operator/pkg/client"
	"github.com/your-org/etcd-k8s-operator/pkg/resource"
	"github.com/your-org/etcd-k8s-operator/pkg/service"
)

// MockKubernetesClient Kubernetes 客户端 Mock
type MockKubernetesClient struct {
	mock.Mock
	realClient client.Client // 用于集成测试的真实客户端
}

func (m *MockKubernetesClient) Create(ctx context.Context, obj client.Object) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

func (m *MockKubernetesClient) Update(ctx context.Context, obj client.Object) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

func (m *MockKubernetesClient) Delete(ctx context.Context, obj client.Object) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

func (m *MockKubernetesClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	args := m.Called(ctx, key, obj)
	return args.Error(0)
}

func (m *MockKubernetesClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := m.Called(ctx, list, opts)
	return args.Error(0)
}

func (m *MockKubernetesClient) UpdateStatus(ctx context.Context, obj client.Object) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

func (m *MockKubernetesClient) RecordEvent(obj runtime.Object, eventType, reason, message string) {
	m.Called(obj, eventType, reason, message)
}

func (m *MockKubernetesClient) GetClient() client.Client {
	// 优先返回真实客户端（用于集成测试）
	if m.realClient != nil {
		return m.realClient
	}

	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(client.Client)
}

// SetClient 设置真实的客户端（用于集成测试）
func (m *MockKubernetesClient) SetClient(c client.Client) {
	m.realClient = c
}

// GetRealClient 获取真实的客户端
func (m *MockKubernetesClient) GetRealClient() client.Client {
	return m.realClient
}

func (m *MockKubernetesClient) GetRecorder() record.EventRecorder {
	args := m.Called()
	return args.Get(0).(record.EventRecorder)
}

// MockResourceManager 资源管理器 Mock
type MockResourceManager struct {
	mock.Mock
}

func (m *MockResourceManager) EnsureAllResources(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	args := m.Called(ctx, cluster)
	return args.Error(0)
}

func (m *MockResourceManager) CleanupResources(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	args := m.Called(ctx, cluster)
	return args.Error(0)
}

func (m *MockResourceManager) StatefulSet() resource.StatefulSetManager {
	args := m.Called()
	return args.Get(0).(resource.StatefulSetManager)
}

func (m *MockResourceManager) Service() resource.ServiceManager {
	args := m.Called()
	return args.Get(0).(resource.ServiceManager)
}

func (m *MockResourceManager) ConfigMap() resource.ConfigMapManager {
	args := m.Called()
	return args.Get(0).(resource.ConfigMapManager)
}

func (m *MockResourceManager) PVC() resource.PVCManager {
	args := m.Called()
	return args.Get(0).(resource.PVCManager)
}

// MockStatefulSetManager StatefulSet 管理器 Mock
type MockStatefulSetManager struct {
	mock.Mock
}

func (m *MockStatefulSetManager) Ensure(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	args := m.Called(ctx, cluster)
	return args.Error(0)
}

func (m *MockStatefulSetManager) EnsureWithReplicas(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, replicas int32) error {
	args := m.Called(ctx, cluster, replicas)
	return args.Error(0)
}

func (m *MockStatefulSetManager) GetStatus(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (*resource.StatefulSetStatus, error) {
	args := m.Called(ctx, cluster)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*resource.StatefulSetStatus), args.Error(1)
}

func (m *MockStatefulSetManager) Get(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (*appsv1.StatefulSet, error) {
	args := m.Called(ctx, cluster)
	return args.Get(0).(*appsv1.StatefulSet), args.Error(1)
}

func (m *MockStatefulSetManager) Update(ctx context.Context, existing *appsv1.StatefulSet, desired *appsv1.StatefulSet) error {
	args := m.Called(ctx, existing, desired)
	return args.Error(0)
}

func (m *MockStatefulSetManager) Delete(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	args := m.Called(ctx, cluster)
	return args.Error(0)
}

func (m *MockStatefulSetManager) NeedsUpdate(existing, desired *appsv1.StatefulSet) bool {
	args := m.Called(existing, desired)
	return args.Bool(0)
}

// MockEtcdClient etcd 客户端 Mock
type MockEtcdClient struct {
	mock.Mock
}

func (m *MockEtcdClient) Connect(ctx context.Context, endpoints []string) error {
	args := m.Called(ctx, endpoints)
	return args.Error(0)
}

func (m *MockEtcdClient) Disconnect() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockEtcdClient) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockEtcdClient) AddMember(ctx context.Context, peerURL string) (*clientpkg.MemberAddResponse, error) {
	args := m.Called(ctx, peerURL)
	return args.Get(0).(*clientpkg.MemberAddResponse), args.Error(1)
}

func (m *MockEtcdClient) RemoveMember(ctx context.Context, memberID uint64) (*clientpkg.MemberRemoveResponse, error) {
	args := m.Called(ctx, memberID)
	return args.Get(0).(*clientpkg.MemberRemoveResponse), args.Error(1)
}

func (m *MockEtcdClient) ListMembers(ctx context.Context) (*clientpkg.MemberListResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(*clientpkg.MemberListResponse), args.Error(1)
}

func (m *MockEtcdClient) HealthCheck(ctx context.Context, endpoint string) (*clientpkg.HealthCheckResponse, error) {
	args := m.Called(ctx, endpoint)
	return args.Get(0).(*clientpkg.HealthCheckResponse), args.Error(1)
}

func (m *MockEtcdClient) GetClusterStatus(ctx context.Context) (*clientpkg.EtcdClusterStatus, error) {
	args := m.Called(ctx)
	return args.Get(0).(*clientpkg.EtcdClusterStatus), args.Error(1)
}

// MockClusterService 集群服务 Mock
type MockClusterService struct {
	mock.Mock
}

func (m *MockClusterService) InitializeCluster(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	args := m.Called(ctx, cluster)
	return args.Get(0).(ctrl.Result), args.Error(1)
}

func (m *MockClusterService) CreateCluster(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	args := m.Called(ctx, cluster)
	return args.Get(0).(ctrl.Result), args.Error(1)
}

func (m *MockClusterService) UpdateClusterStatus(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	args := m.Called(ctx, cluster)
	return args.Error(0)
}

func (m *MockClusterService) DeleteCluster(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	args := m.Called(ctx, cluster)
	return args.Get(0).(ctrl.Result), args.Error(1)
}

func (m *MockClusterService) GetClusterStatus(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (*service.ClusterStatus, error) {
	args := m.Called(ctx, cluster)
	return args.Get(0).(*service.ClusterStatus), args.Error(1)
}

func (m *MockClusterService) IsClusterReady(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (bool, error) {
	args := m.Called(ctx, cluster)
	return args.Bool(0), args.Error(1)
}

func (m *MockClusterService) ValidateClusterSpec(cluster *etcdv1alpha1.EtcdCluster) error {
	args := m.Called(cluster)
	return args.Error(0)
}

func (m *MockClusterService) SetDefaults(cluster *etcdv1alpha1.EtcdCluster) {
	m.Called(cluster)
}
