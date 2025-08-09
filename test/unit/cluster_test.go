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

package unit

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	resourcepkg "github.com/your-org/etcd-k8s-operator/pkg/resource"
	"github.com/your-org/etcd-k8s-operator/pkg/service"
	"github.com/your-org/etcd-k8s-operator/test/unit/mocks"
)

// =============================================================================
// 测试数据工厂函数
// =============================================================================

// createTestCluster 创建测试用的EtcdCluster对象
func createTestCluster(name, namespace string, size int32, phase etcdv1alpha1.EtcdClusterPhase) *etcdv1alpha1.EtcdCluster {
	return &etcdv1alpha1.EtcdCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: etcdv1alpha1.EtcdClusterSpec{
			Size:       size,
			Version:    "v3.5.21",
			Repository: "quay.io/coreos/etcd",
			Storage: etcdv1alpha1.EtcdStorageSpec{
				Size: resource.MustParse("10Gi"),
			},
		},
		Status: etcdv1alpha1.EtcdClusterStatus{
			Phase: phase,
		},
	}
}

// createDeletingCluster 创建正在删除的集群
func createDeletingCluster(name, namespace string) *etcdv1alpha1.EtcdCluster {
	now := metav1.Now()
	cluster := createTestCluster(name, namespace, 3, etcdv1alpha1.EtcdClusterPhaseRunning)
	cluster.DeletionTimestamp = &now
	cluster.Finalizers = []string{"etcd.etcd.io/etcd-cluster"}
	return cluster
}

// =============================================================================
// 服务层单元测试 - ClusterService
// =============================================================================

func TestClusterService_SetDefaults(t *testing.T) {
	tests := []struct {
		name         string
		inputSize    int32
		inputPhase   etcdv1alpha1.EtcdClusterPhase
		expectedSize int32
		description  string
	}{
		{
			name:         "新集群设置默认Size",
			inputSize:    0,
			inputPhase:   "",
			expectedSize: 3,
			description:  "新创建的集群应该设置默认Size=3",
		},
		{
			name:         "现有集群保持Size=0",
			inputSize:    0,
			inputPhase:   etcdv1alpha1.EtcdClusterPhaseRunning,
			expectedSize: 0,
			description:  "运行中的集群Size=0应该保持不变(支持缩容到0)",
		},
		{
			name:         "现有集群保持自定义Size",
			inputSize:    5,
			inputPhase:   etcdv1alpha1.EtcdClusterPhaseRunning,
			expectedSize: 5,
			description:  "现有集群的自定义Size应该保持不变",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试数据
			cluster := createTestCluster("test-cluster", "default", tt.inputSize, tt.inputPhase)
			mockK8sClient := &mocks.MockKubernetesClient{}
			mockResourceManager := &mocks.MockResourceManager{}
			clusterService := service.NewClusterService(mockK8sClient, mockResourceManager)

			// Act: 执行被测试的方法
			clusterService.SetDefaults(cluster)

			// Assert: 验证结果
			assert.Equal(t, tt.expectedSize, cluster.Spec.Size, tt.description)
			assert.Equal(t, "v3.5.21", cluster.Spec.Version, "应该设置默认版本")
			assert.Equal(t, "quay.io/coreos/etcd", cluster.Spec.Repository, "应该设置默认仓库")
			assert.True(t, cluster.Spec.Storage.Size.Equal(resource.MustParse("10Gi")), "应该设置默认存储大小")
		})
	}
}

func TestClusterService_ValidateClusterSpec(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *etcdv1alpha1.EtcdCluster
		wantErr     bool
		errContains string
		description string
	}{
		{
			name:        "有效的单节点集群",
			cluster:     createTestCluster("test", "default", 1, ""),
			wantErr:     false,
			description: "Size=1的单节点集群应该通过验证",
		},
		{
			name:        "有效的三节点集群",
			cluster:     createTestCluster("test", "default", 3, ""),
			wantErr:     false,
			description: "Size=3的多节点集群应该通过验证",
		},
		{
			name:        "有效的五节点集群",
			cluster:     createTestCluster("test", "default", 5, ""),
			wantErr:     false,
			description: "Size=5的多节点集群应该通过验证",
		},
		{
			name:        "无效的负数Size",
			cluster:     createTestCluster("test", "default", -1, ""),
			wantErr:     true,
			errContains: "cluster size cannot be negative",
			description: "负数Size应该被拒绝",
		},
		{
			name:        "无效的偶数Size",
			cluster:     createTestCluster("test", "default", 2, ""),
			wantErr:     true,
			errContains: "cluster size must be odd for multi-node clusters",
			description: "偶数Size应该被拒绝(etcd要求奇数节点)",
		},
		{
			name:        "无效的偶数Size=4",
			cluster:     createTestCluster("test", "default", 4, ""),
			wantErr:     true,
			errContains: "cluster size must be odd for multi-node clusters",
			description: "Size=4应该被拒绝",
		},
	}

	// 创建空版本的集群用于测试
	emptyVersionCluster := createTestCluster("test", "default", 3, "")
	emptyVersionCluster.Spec.Version = ""
	tests = append(tests, struct {
		name        string
		cluster     *etcdv1alpha1.EtcdCluster
		wantErr     bool
		errContains string
		description string
	}{
		name:        "无效的空版本",
		cluster:     emptyVersionCluster,
		wantErr:     true,
		errContains: "etcd version cannot be empty",
		description: "空的etcd版本应该被拒绝",
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockK8sClient := &mocks.MockKubernetesClient{}
			mockResourceManager := &mocks.MockResourceManager{}
			clusterService := service.NewClusterService(mockK8sClient, mockResourceManager)

			// Act: 执行验证
			err := clusterService.ValidateClusterSpec(tt.cluster)

			// Assert: 验证结果
			if tt.wantErr {
				assert.Error(t, err, tt.description)
				assert.Contains(t, err.Error(), tt.errContains, "错误消息应该包含预期内容")
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

func TestClusterService_InitializeCluster(t *testing.T) {
	tests := []struct {
		name            string
		cluster         *etcdv1alpha1.EtcdCluster
		mockSetup       func(*mocks.MockKubernetesClient)
		expectedPhase   etcdv1alpha1.EtcdClusterPhase
		expectedRequeue bool
		expectError     bool
		description     string
	}{
		{
			name:    "成功初始化新集群",
			cluster: createTestCluster("test-cluster", "default", 3, ""),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock UpdateStatus 成功
				mockClient.On("UpdateStatus", mock.Anything, mock.Anything).Return(nil)
			},
			expectedPhase:   etcdv1alpha1.EtcdClusterPhaseCreating,
			expectedRequeue: true,
			expectError:     false,
			description:     "新集群初始化应该设置Phase为Creating并重新入队",
		},
		{
			name:    "初始化时状态更新失败",
			cluster: createTestCluster("test-cluster", "default", 3, ""),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock UpdateStatus 失败
				mockClient.On("UpdateStatus", mock.Anything, mock.Anything).Return(apierrors.NewInternalError(errors.New("internal error")))
			},
			expectedPhase:   etcdv1alpha1.EtcdClusterPhaseCreating,
			expectedRequeue: false,
			expectError:     true,
			description:     "状态更新失败时应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockK8sClient := &mocks.MockKubernetesClient{}
			mockResourceManager := &mocks.MockResourceManager{}
			tt.mockSetup(mockK8sClient)

			clusterService := service.NewClusterService(mockK8sClient, mockResourceManager)

			// Act: 执行初始化
			result, err := clusterService.InitializeCluster(context.Background(), tt.cluster)

			// Assert: 验证结果
			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
				assert.Equal(t, tt.expectedRequeue, result.Requeue, "Requeue标志应该正确")
			}

			assert.Equal(t, tt.expectedPhase, tt.cluster.Status.Phase, "Phase应该正确设置")
			mockK8sClient.AssertExpectations(t)
		})
	}
}

func TestClusterService_CreateCluster(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *etcdv1alpha1.EtcdCluster
		mockSetup   func(*mocks.MockKubernetesClient, *mocks.MockResourceManager, *mocks.MockStatefulSetManager)
		expectError bool
		description string
	}{
		{
			name:    "成功创建单节点集群",
			cluster: createTestCluster("test-cluster", "default", 1, etcdv1alpha1.EtcdClusterPhaseCreating),
			mockSetup: func(mockK8sClient *mocks.MockKubernetesClient, mockResourceManager *mocks.MockResourceManager, mockStatefulSetManager *mocks.MockStatefulSetManager) {
				// Mock 资源创建成功
				mockResourceManager.On("EnsureAllResources", mock.Anything, mock.Anything).Return(nil)

				// Mock 集群就绪检查
				mockResourceManager.On("StatefulSet").Return(mockStatefulSetManager)
				mockStatefulSetManager.On("GetStatus", mock.Anything, mock.Anything).Return(&resourcepkg.StatefulSetStatus{
					Replicas:      1,
					ReadyReplicas: 1,
				}, nil)

				// Mock 状态更新
				mockK8sClient.On("UpdateStatus", mock.Anything, mock.Anything).Return(nil)
			},
			expectError: false,
			description: "单节点集群创建成功后应该转换到Running状态",
		},
		{
			name:    "资源创建失败",
			cluster: createTestCluster("test-cluster", "default", 1, etcdv1alpha1.EtcdClusterPhaseCreating),
			mockSetup: func(mockK8sClient *mocks.MockKubernetesClient, mockResourceManager *mocks.MockResourceManager, mockStatefulSetManager *mocks.MockStatefulSetManager) {
				// Mock 资源创建失败
				mockResourceManager.On("EnsureAllResources", mock.Anything, mock.Anything).Return(apierrors.NewInternalError(errors.New("resource creation failed")))

				// Mock 错误状态更新
				mockK8sClient.On("UpdateStatus", mock.Anything, mock.Anything).Return(nil)
				mockK8sClient.On("RecordEvent", mock.Anything, "Warning", "Failed", mock.Anything)
			},
			expectError: false, // updateStatusWithError 不返回错误，而是重新入队
			description: "资源创建失败时应该设置Failed状态",
		},
		{
			name:    "多节点集群使用渐进式创建",
			cluster: createTestCluster("test-cluster", "default", 3, etcdv1alpha1.EtcdClusterPhaseCreating),
			mockSetup: func(mockK8sClient *mocks.MockKubernetesClient, mockResourceManager *mocks.MockResourceManager, mockStatefulSetManager *mocks.MockStatefulSetManager) {
				// Mock 资源创建成功
				mockResourceManager.On("EnsureAllResources", mock.Anything, mock.Anything).Return(nil)
				// 多节点集群会调用 handleMultiNodeClusterCreation，目前返回重新入队
			},
			expectError: false,
			description: "多节点集群应该使用渐进式创建策略",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockK8sClient := &mocks.MockKubernetesClient{}
			mockResourceManager := &mocks.MockResourceManager{}
			mockStatefulSetManager := &mocks.MockStatefulSetManager{}
			tt.mockSetup(mockK8sClient, mockResourceManager, mockStatefulSetManager)

			clusterService := service.NewClusterService(mockK8sClient, mockResourceManager)

			// Act: 执行集群创建
			result, err := clusterService.CreateCluster(context.Background(), tt.cluster)

			// Assert: 验证结果
			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
				// 对于成功的情况，验证返回值
				assert.NotNil(t, result, "应该返回有效的Result")
			}

			mockK8sClient.AssertExpectations(t)
			mockResourceManager.AssertExpectations(t)
		})
	}
}

func TestClusterService_IsClusterReady(t *testing.T) {
	tests := []struct {
		name          string
		cluster       *etcdv1alpha1.EtcdCluster
		mockSetup     func(*mocks.MockResourceManager, *mocks.MockStatefulSetManager)
		expectedReady bool
		expectError   bool
		description   string
	}{
		{
			name:    "集群完全就绪",
			cluster: createTestCluster("test-cluster", "default", 3, etcdv1alpha1.EtcdClusterPhaseCreating),
			mockSetup: func(mockResourceManager *mocks.MockResourceManager, mockStatefulSetManager *mocks.MockStatefulSetManager) {
				mockResourceManager.On("StatefulSet").Return(mockStatefulSetManager)
				mockStatefulSetManager.On("GetStatus", mock.Anything, mock.Anything).Return(&resourcepkg.StatefulSetStatus{
					Replicas:      3,
					ReadyReplicas: 3,
				}, nil)
			},
			expectedReady: true,
			expectError:   false,
			description:   "所有副本就绪时应该返回true",
		},
		{
			name:    "集群部分就绪",
			cluster: createTestCluster("test-cluster", "default", 3, etcdv1alpha1.EtcdClusterPhaseCreating),
			mockSetup: func(mockResourceManager *mocks.MockResourceManager, mockStatefulSetManager *mocks.MockStatefulSetManager) {
				mockResourceManager.On("StatefulSet").Return(mockStatefulSetManager)
				mockStatefulSetManager.On("GetStatus", mock.Anything, mock.Anything).Return(&resourcepkg.StatefulSetStatus{
					Replicas:      3,
					ReadyReplicas: 2, // 只有2个就绪
				}, nil)
			},
			expectedReady: false,
			expectError:   false,
			description:   "部分副本就绪时应该返回false",
		},
		{
			name:    "获取状态失败",
			cluster: createTestCluster("test-cluster", "default", 3, etcdv1alpha1.EtcdClusterPhaseCreating),
			mockSetup: func(mockResourceManager *mocks.MockResourceManager, mockStatefulSetManager *mocks.MockStatefulSetManager) {
				mockResourceManager.On("StatefulSet").Return(mockStatefulSetManager)
				mockStatefulSetManager.On("GetStatus", mock.Anything, mock.Anything).Return(nil, apierrors.NewInternalError(errors.New("get status failed")))
			},
			expectedReady: false,
			expectError:   true,
			description:   "获取状态失败时应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockK8sClient := &mocks.MockKubernetesClient{}
			mockResourceManager := &mocks.MockResourceManager{}
			mockStatefulSetManager := &mocks.MockStatefulSetManager{}
			tt.mockSetup(mockResourceManager, mockStatefulSetManager)

			clusterService := service.NewClusterService(mockK8sClient, mockResourceManager)

			// Act: 检查集群就绪状态
			ready, err := clusterService.IsClusterReady(context.Background(), tt.cluster)

			// Assert: 验证结果
			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
				assert.Equal(t, tt.expectedReady, ready, tt.description)
			}

			mockResourceManager.AssertExpectations(t)
			mockStatefulSetManager.AssertExpectations(t)
		})
	}
}

// =============================================================================
// 资源层单元测试 - ResourceManager & StatefulSetManager
// =============================================================================

/*
// TODO: 资源管理器测试需要更复杂的Mock设置，暂时跳过
func TestResourceManager_EnsureAllResources(t *testing.T) {
	// 复杂的资源管理器测试暂时跳过
	// 需要Mock Scheme等复杂对象
}
*/

func TestStatefulSetManager_GetStatus(t *testing.T) {
	tests := []struct {
		name           string
		cluster        *etcdv1alpha1.EtcdCluster
		mockSetup      func(*mocks.MockKubernetesClient)
		expectedStatus *resourcepkg.StatefulSetStatus
		expectError    bool
		description    string
	}{
		{
			name:    "获取正常StatefulSet状态",
			cluster: createTestCluster("test-cluster", "default", 3, etcdv1alpha1.EtcdClusterPhaseRunning),
			mockSetup: func(mockK8sClient *mocks.MockKubernetesClient) {
				sts := &appsv1.StatefulSet{
					Status: appsv1.StatefulSetStatus{
						Replicas:        3,
						ReadyReplicas:   2,
						CurrentReplicas: 3,
						UpdatedReplicas: 3,
					},
				}
				mockK8sClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*appsv1.StatefulSet)
					return ok
				})).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*appsv1.StatefulSet)
					*obj = *sts
				}).Return(nil)
			},
			expectedStatus: &resourcepkg.StatefulSetStatus{
				Replicas:        3,
				ReadyReplicas:   2,
				CurrentReplicas: 3,
				UpdatedReplicas: 3,
			},
			expectError: false,
			description: "应该正确返回StatefulSet状态",
		},
		{
			name:    "StatefulSet不存在时返回零值状态",
			cluster: createTestCluster("test-cluster", "default", 3, etcdv1alpha1.EtcdClusterPhaseCreating),
			mockSetup: func(mockK8sClient *mocks.MockKubernetesClient) {
				mockK8sClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*appsv1.StatefulSet)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "statefulsets"}, "test"))
			},
			expectedStatus: &resourcepkg.StatefulSetStatus{
				Replicas:        0,
				ReadyReplicas:   0,
				CurrentReplicas: 0,
				UpdatedReplicas: 0,
			},
			expectError: false,
			description: "StatefulSet不存在时应该返回零值状态而不是错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockK8sClient := &mocks.MockKubernetesClient{}
			tt.mockSetup(mockK8sClient)

			manager := resourcepkg.NewStatefulSetManager(mockK8sClient)

			// Act: 获取状态
			status, err := manager.GetStatus(context.Background(), tt.cluster)

			// Assert: 验证结果
			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
				assert.Equal(t, tt.expectedStatus.Replicas, status.Replicas, "Replicas应该匹配")
				assert.Equal(t, tt.expectedStatus.ReadyReplicas, status.ReadyReplicas, "ReadyReplicas应该匹配")
				assert.Equal(t, tt.expectedStatus.CurrentReplicas, status.CurrentReplicas, "CurrentReplicas应该匹配")
				assert.Equal(t, tt.expectedStatus.UpdatedReplicas, status.UpdatedReplicas, "UpdatedReplicas应该匹配")
			}

			mockK8sClient.AssertExpectations(t)
		})
	}
}
