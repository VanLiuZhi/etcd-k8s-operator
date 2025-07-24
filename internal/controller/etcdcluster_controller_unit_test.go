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

package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	"github.com/your-org/etcd-k8s-operator/pkg/utils"
)

// MockClient Mock Kubernetes 客户端
type MockClient struct {
	mock.Mock
}

func (m *MockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	args := m.Called(ctx, key, obj)
	return args.Error(0)
}

func (m *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	args := m.Called(ctx, list, opts)
	return args.Error(0)
}

func (m *MockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

func (m *MockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

func (m *MockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

func (m *MockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	args := m.Called(ctx, obj, patch)
	return args.Error(0)
}

func (m *MockClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

func (m *MockClient) Status() client.StatusWriter {
	args := m.Called()
	return args.Get(0).(client.StatusWriter)
}

func (m *MockClient) Scheme() *runtime.Scheme {
	args := m.Called()
	return args.Get(0).(*runtime.Scheme)
}

func (m *MockClient) RESTMapper() meta.RESTMapper {
	args := m.Called()
	return args.Get(0).(meta.RESTMapper)
}

func (m *MockClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	args := m.Called(obj)
	return args.Get(0).(schema.GroupVersionKind), args.Error(1)
}

func (m *MockClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	args := m.Called(obj)
	return args.Bool(0), args.Error(1)
}

func (m *MockClient) SubResource(subResource string) client.SubResourceClient {
	args := m.Called(subResource)
	return args.Get(0).(client.SubResourceClient)
}

// MockStatusWriter Mock 状态写入器
type MockStatusWriter struct {
	mock.Mock
}

func (m *MockStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}

func (m *MockStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	args := m.Called(ctx, obj, patch)
	return args.Error(0)
}

func (m *MockStatusWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	args := m.Called(ctx, obj, subResource)
	return args.Error(0)
}

// MockEventRecorder Mock 事件记录器
type MockEventRecorder struct {
	mock.Mock
}

func (m *MockEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	m.Called(object, eventtype, reason, message)
}

func (m *MockEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	m.Called(object, eventtype, reason, messageFmt, args)
}

func (m *MockEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	m.Called(object, annotations, eventtype, reason, messageFmt, args)
}

// EtcdClusterControllerTestSuite 控制器测试套件
type EtcdClusterControllerTestSuite struct {
	suite.Suite
	reconciler   *EtcdClusterReconciler
	mockClient   *MockClient
	mockStatus   *MockStatusWriter
	mockRecorder *MockEventRecorder
	cluster      *etcdv1alpha1.EtcdCluster
	ctx          context.Context
}

// SetupTest 设置测试环境
func (suite *EtcdClusterControllerTestSuite) SetupTest() {
	suite.mockClient = new(MockClient)
	suite.mockStatus = new(MockStatusWriter)
	suite.mockRecorder = new(MockEventRecorder)
	suite.ctx = context.Background()

	// 设置 Mock 客户端返回状态写入器
	suite.mockClient.On("Status").Return(suite.mockStatus)
	suite.mockClient.On("Scheme").Return(runtime.NewScheme())

	suite.reconciler = &EtcdClusterReconciler{
		Client:   suite.mockClient,
		Scheme:   runtime.NewScheme(),
		Recorder: suite.mockRecorder,
	}

	suite.cluster = &etcdv1alpha1.EtcdCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: etcdv1alpha1.EtcdClusterSpec{
			Size:       3,
			Version:    "3.5.9",
			Repository: "quay.io/coreos/etcd",
			Storage: etcdv1alpha1.EtcdStorageSpec{
				Size: resource.MustParse("10Gi"),
			},
		},
	}
}

// TearDownTest 清理测试环境
func (suite *EtcdClusterControllerTestSuite) TearDownTest() {
	suite.mockClient.AssertExpectations(suite.T())
	suite.mockStatus.AssertExpectations(suite.T())
	suite.mockRecorder.AssertExpectations(suite.T())
}

// TestReconcileClusterNotFound 测试集群不存在的情况
func (suite *EtcdClusterControllerTestSuite) TestReconcileClusterNotFound() {
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "non-existent-cluster",
			Namespace: "default",
		},
	}

	// Mock 客户端返回 NotFound 错误
	notFoundError := errors.NewNotFound(schema.GroupResource{
		Group:    "etcd.etcd.io",
		Resource: "etcdclusters",
	}, "non-existent-cluster")

	suite.mockClient.On("Get", suite.ctx, req.NamespacedName, mock.AnythingOfType("*v1alpha1.EtcdCluster")).Return(notFoundError)

	result, err := suite.reconciler.Reconcile(suite.ctx, req)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), ctrl.Result{}, result)
}

// TestValidateClusterSpec 测试集群规范验证
func (suite *EtcdClusterControllerTestSuite) TestValidateClusterSpec() {
	// 测试有效的集群规范
	err := suite.reconciler.validateClusterSpec(suite.cluster)
	assert.NoError(suite.T(), err)

	// 测试无效的集群大小（偶数）
	suite.cluster.Spec.Size = 2
	err = suite.reconciler.validateClusterSpec(suite.cluster)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "cluster size must be odd number")

	// 测试无效的集群大小（超出范围）
	suite.cluster.Spec.Size = 11
	err = suite.reconciler.validateClusterSpec(suite.cluster)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "cluster size must be between 1 and 9")
}

// TestSetDefaults 测试默认值设置
func (suite *EtcdClusterControllerTestSuite) TestSetDefaults() {
	// 创建一个空的集群规范
	emptyCluster := &etcdv1alpha1.EtcdCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "empty-cluster",
			Namespace: "default",
		},
		Spec: etcdv1alpha1.EtcdClusterSpec{},
	}

	suite.reconciler.setDefaults(emptyCluster)

	// 验证默认值
	assert.Equal(suite.T(), int32(utils.DefaultClusterSize), emptyCluster.Spec.Size)
	assert.Equal(suite.T(), utils.DefaultEtcdVersion, emptyCluster.Spec.Version)
	assert.Equal(suite.T(), utils.DefaultEtcdRepository, emptyCluster.Spec.Repository)
	assert.Equal(suite.T(), resource.MustParse(utils.DefaultStorageSize), emptyCluster.Spec.Storage.Size)
}

// TestSetCondition 测试条件设置
func (suite *EtcdClusterControllerTestSuite) TestSetCondition() {
	// 设置新条件
	suite.reconciler.setCondition(suite.cluster, utils.ConditionTypeReady, metav1.ConditionTrue, utils.ReasonRunning, "Cluster is running")

	assert.Len(suite.T(), suite.cluster.Status.Conditions, 1)
	condition := suite.cluster.Status.Conditions[0]
	assert.Equal(suite.T(), utils.ConditionTypeReady, condition.Type)
	assert.Equal(suite.T(), metav1.ConditionTrue, condition.Status)
	assert.Equal(suite.T(), utils.ReasonRunning, condition.Reason)
	assert.Equal(suite.T(), "Cluster is running", condition.Message)

	// 更新现有条件
	suite.reconciler.setCondition(suite.cluster, utils.ConditionTypeReady, metav1.ConditionFalse, utils.ReasonFailed, "Cluster failed")

	assert.Len(suite.T(), suite.cluster.Status.Conditions, 1)
	updatedCondition := suite.cluster.Status.Conditions[0]
	assert.Equal(suite.T(), metav1.ConditionFalse, updatedCondition.Status)
	assert.Equal(suite.T(), utils.ReasonFailed, updatedCondition.Reason)
	assert.Equal(suite.T(), "Cluster failed", updatedCondition.Message)
}

// TestNeedsScaling 测试扩缩容检测
func (suite *EtcdClusterControllerTestSuite) TestNeedsScaling() {
	// 测试不需要扩缩容
	suite.cluster.Spec.Size = 3
	suite.cluster.Status.ReadyReplicas = 3
	assert.False(suite.T(), suite.reconciler.needsScaling(suite.cluster))

	// 测试需要扩容
	suite.cluster.Spec.Size = 5
	suite.cluster.Status.ReadyReplicas = 3
	assert.True(suite.T(), suite.reconciler.needsScaling(suite.cluster))

	// 测试需要缩容
	suite.cluster.Spec.Size = 1
	suite.cluster.Status.ReadyReplicas = 3
	assert.True(suite.T(), suite.reconciler.needsScaling(suite.cluster))
}

// TestCheckClusterReady 测试集群就绪检查
func (suite *EtcdClusterControllerTestSuite) TestCheckClusterReady() {
	// Mock StatefulSet 获取成功
	sts := &appsv1.StatefulSet{
		Status: appsv1.StatefulSetStatus{
			Replicas:      3,
			ReadyReplicas: 3,
		},
	}

	suite.mockClient.On("Get", suite.ctx, types.NamespacedName{
		Name:      suite.cluster.Name,
		Namespace: suite.cluster.Namespace,
	}, mock.AnythingOfType("*v1.StatefulSet")).Run(func(args mock.Arguments) {
		statefulSet := args.Get(2).(*appsv1.StatefulSet)
		*statefulSet = *sts
	}).Return(nil)

	ready, err := suite.reconciler.checkClusterReady(suite.ctx, suite.cluster)

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), ready)
}

// TestEtcdClusterControllerTestSuite 运行测试套件
func TestEtcdClusterControllerTestSuite(t *testing.T) {
	suite.Run(t, new(EtcdClusterControllerTestSuite))
}
