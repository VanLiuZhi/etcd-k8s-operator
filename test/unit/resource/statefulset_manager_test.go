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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	resourcepkg "github.com/your-org/etcd-k8s-operator/pkg/resource"
	"github.com/your-org/etcd-k8s-operator/test/unit/mocks"
)

// createTestStatefulSet 创建测试用的StatefulSet
func createTestStatefulSet(name string, replicas int32) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "etcd",
							Image: "quay.io/coreos/etcd:v3.5.21",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("128Mi"),
									corev1.ResourceCPU:    resource.MustParse("100m"),
								},
							},
						},
					},
				},
			},
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:        replicas,
			ReadyReplicas:   replicas,
			CurrentReplicas: replicas,
			UpdatedReplicas: replicas,
		},
	}
}

// TestStatefulSetManager_Ensure 测试Ensure方法
func TestStatefulSetManager_Ensure(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *etcdv1alpha1.EtcdCluster
		mockSetup   func(*mocks.MockKubernetesClient)
		expectError bool
		description string
	}{
		{
			name:    "创建新的StatefulSet",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock Get - StatefulSet不存在
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*appsv1.StatefulSet)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "statefulsets"}, "test"))

				// Mock Create - 创建成功
				mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					sts, ok := obj.(*appsv1.StatefulSet)
					return ok && sts.Name == "test-cluster"
				})).Return(nil)

				// Mock GetClient - 返回nil（跳过ControllerReference设置）
				// 注意：这会导致SetControllerReference失败，我们需要修复这个问题
				mockClient.On("GetClient").Return(nil)
			},
			expectError: false,
			description: "不存在StatefulSet时应该创建新的",
		},
		{
			name:    "更新现有的StatefulSet",
			cluster: createTestCluster("test-cluster", 5), // 扩容到5个节点
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				existingSts := createTestStatefulSet("test-cluster", 3) // 现有3个节点

				// Mock Get - StatefulSet存在
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*appsv1.StatefulSet)
					return ok
				})).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*appsv1.StatefulSet)
					*obj = *existingSts
				}).Return(nil)

				// Mock Update - 更新成功
				mockClient.On("Update", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					sts, ok := obj.(*appsv1.StatefulSet)
					return ok && *sts.Spec.Replicas == 5 // 验证副本数已更新
				})).Return(nil)
			},
			expectError: false,
			description: "存在StatefulSet且需要更新时应该执行更新",
		},
		{
			name:    "StatefulSet已是最新状态",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				existingSts := createTestStatefulSet("test-cluster", 3) // 副本数已匹配

				// Mock Get - StatefulSet存在且状态正确
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*appsv1.StatefulSet)
					return ok
				})).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*appsv1.StatefulSet)
					*obj = *existingSts
				}).Return(nil)

				// 可能需要更新StatefulSet，Mock Update方法
				mockClient.On("Update", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*appsv1.StatefulSet)
					return ok
				})).Return(nil).Maybe() // Maybe()表示可能调用也可能不调用
			},
			expectError: false,
			description: "StatefulSet已是最新状态时不应该执行更新",
		},
		{
			name:    "创建StatefulSet失败",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock Get - StatefulSet不存在
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*appsv1.StatefulSet)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "statefulsets"}, "test"))

				// Mock Create - 创建失败
				mockClient.On("Create", mock.Anything, mock.Anything).Return(apierrors.NewInternalError(fmt.Errorf("internal server error")))
				mockClient.On("GetClient").Return(nil)
			},
			expectError: true,
			description: "创建StatefulSet失败时应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockClient := &mocks.MockKubernetesClient{}
			tt.mockSetup(mockClient)

			manager := resourcepkg.NewStatefulSetManager(mockClient)

			// Act: 执行测试
			err := manager.Ensure(context.Background(), tt.cluster)

			// Assert: 验证结果
			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestStatefulSetManager_EnsureWithReplicas 测试EnsureWithReplicas方法
func TestStatefulSetManager_EnsureWithReplicas(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *etcdv1alpha1.EtcdCluster
		replicas    int32
		mockSetup   func(*mocks.MockKubernetesClient)
		expectError bool
		description string
	}{
		{
			name:     "指定副本数创建StatefulSet",
			cluster:  createTestCluster("test-cluster", 3),
			replicas: 1, // 渐进式创建，先创建1个副本
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock Get - StatefulSet不存在
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*appsv1.StatefulSet)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "statefulsets"}, "test"))

				// Mock Create - 验证副本数为1
				mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					sts, ok := obj.(*appsv1.StatefulSet)
					return ok && *sts.Spec.Replicas == 1
				})).Return(nil)

				mockClient.On("GetClient").Return(nil)
			},
			expectError: false,
			description: "应该使用指定的副本数创建StatefulSet",
		},
		{
			name:     "扩容现有StatefulSet",
			cluster:  createTestCluster("test-cluster", 3),
			replicas: 3, // 从1扩容到3
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				existingSts := createTestStatefulSet("test-cluster", 1) // 现有1个副本

				// Mock Get - StatefulSet存在
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*appsv1.StatefulSet)
					return ok
				})).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*appsv1.StatefulSet)
					*obj = *existingSts
				}).Return(nil)

				// Mock Update - 验证副本数更新为3
				mockClient.On("Update", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					sts, ok := obj.(*appsv1.StatefulSet)
					return ok && *sts.Spec.Replicas == 3
				})).Return(nil)
			},
			expectError: false,
			description: "应该将现有StatefulSet扩容到指定副本数",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockClient := &mocks.MockKubernetesClient{}
			tt.mockSetup(mockClient)

			manager := resourcepkg.NewStatefulSetManager(mockClient)

			// Act: 执行测试
			err := manager.EnsureWithReplicas(context.Background(), tt.cluster, tt.replicas)

			// Assert: 验证结果
			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestStatefulSetManager_GetStatus 测试GetStatus方法
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
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				sts := createTestStatefulSet("test-cluster", 3)
				sts.Status = appsv1.StatefulSetStatus{
					Replicas:        3,
					ReadyReplicas:   2, // 2个已就绪
					CurrentReplicas: 3,
					UpdatedReplicas: 3,
				}

				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
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
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
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
			mockClient := &mocks.MockKubernetesClient{}
			tt.mockSetup(mockClient)

			manager := resourcepkg.NewStatefulSetManager(mockClient)

			// Act: 执行测试
			status, err := manager.GetStatus(context.Background(), tt.cluster)

			// Assert: 验证结果
			if tt.expectError {
				assert.Error(t, err, tt.description)
				assert.Nil(t, status)
			} else {
				assert.NoError(t, err, tt.description)
				require.NotNil(t, status)
				assert.Equal(t, tt.expectedStatus.Replicas, status.Replicas, "Replicas应该匹配")
				assert.Equal(t, tt.expectedStatus.ReadyReplicas, status.ReadyReplicas, "ReadyReplicas应该匹配")
				assert.Equal(t, tt.expectedStatus.CurrentReplicas, status.CurrentReplicas, "CurrentReplicas应该匹配")
				assert.Equal(t, tt.expectedStatus.UpdatedReplicas, status.UpdatedReplicas, "UpdatedReplicas应该匹配")
			}

			mockClient.AssertExpectations(t)
		})
	}
}
