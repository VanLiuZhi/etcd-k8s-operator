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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	resourcepkg "github.com/your-org/etcd-k8s-operator/pkg/resource"
	"github.com/your-org/etcd-k8s-operator/test/unit/mocks"
)

// TestResourceManager_EnsureAllResources 测试EnsureAllResources方法
func TestResourceManager_EnsureAllResources(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *etcdv1alpha1.EtcdCluster
		mockSetup   func(*mocks.MockKubernetesClient)
		expectError bool
		description string
	}{
		{
			name:    "成功确保所有资源",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock ConfigMap操作
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "configmaps"}, "test"))
				mockClient.On("GetClient").Return(nil)
				mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Return(nil)

				// Mock Service操作 - 客户端服务
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "services"}, "test")).Times(2) // 客户端和对等服务
				mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Return(nil).Times(2)

				// Mock StatefulSet操作
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*appsv1.StatefulSet)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "statefulsets"}, "test"))
				mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*appsv1.StatefulSet)
					return ok
				})).Return(nil)
			},
			expectError: false,
			description: "应该成功确保所有资源（ConfigMap、Services、StatefulSet）",
		},
		{
			name:    "ConfigMap创建失败",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock ConfigMap操作失败
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "configmaps"}, "test"))
				mockClient.On("GetClient").Return(nil)
				mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Return(apierrors.NewInternalError(fmt.Errorf("create failed")))
			},
			expectError: true,
			description: "ConfigMap创建失败时应该返回错误",
		},
		{
			name:    "Service创建失败",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock ConfigMap操作成功
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "configmaps"}, "test"))
				mockClient.On("GetClient").Return(nil)
				mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Return(nil)

				// Mock Service操作失败
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "services"}, "test"))
				mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Return(apierrors.NewInternalError(fmt.Errorf("service create failed")))
			},
			expectError: true,
			description: "Service创建失败时应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockClient := &mocks.MockKubernetesClient{}
			tt.mockSetup(mockClient)

			manager := resourcepkg.NewResourceManager(mockClient)

			// Act: 执行测试
			err := manager.EnsureAllResources(context.Background(), tt.cluster)

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

// TestResourceManager_CleanupResources 测试CleanupResources方法
func TestResourceManager_CleanupResources(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *etcdv1alpha1.EtcdCluster
		mockSetup   func(*mocks.MockKubernetesClient)
		expectError bool
		description string
	}{
		{
			name:    "成功清理所有资源",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock StatefulSet删除 - 需要先Get
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*appsv1.StatefulSet)
					return ok
				})).Return(nil) // StatefulSet存在
				mockClient.On("Delete", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*appsv1.StatefulSet)
					return ok
				})).Return(nil)

				// Mock Service删除 - 需要先Get，然后Delete
				// 客户端服务和对等服务各一次Get和Delete
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Return(nil).Times(2) // 两个服务都存在
				mockClient.On("Delete", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Return(nil).Times(2)

				// Mock ConfigMap删除 - 需要先Get
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Return(nil) // ConfigMap存在
				mockClient.On("Delete", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Return(nil)

				// Mock PVC清理 - List操作
				mockClient.On("List", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.PersistentVolumeClaimList)
					return ok
				}), mock.Anything).Return(nil) // 返回空列表
			},
			expectError: false,
			description: "应该成功清理所有资源",
		},
		{
			name:    "资源不存在时的清理",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock StatefulSet删除 - 不存在
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*appsv1.StatefulSet)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "statefulsets"}, "test"))

				// Mock Service删除 - 不存在
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "services"}, "test")).Times(2)

				// Mock ConfigMap删除 - 不存在
				mockClient.On("Get", mock.Anything, mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "configmaps"}, "test"))

				// Mock PVC清理 - List操作
				mockClient.On("List", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.PersistentVolumeClaimList)
					return ok
				}), mock.Anything).Return(nil) // 返回空列表
			},
			expectError: false,
			description: "资源不存在时清理应该成功（幂等操作）",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockClient := &mocks.MockKubernetesClient{}
			tt.mockSetup(mockClient)

			manager := resourcepkg.NewResourceManager(mockClient)

			// Act: 执行测试
			err := manager.CleanupResources(context.Background(), tt.cluster)

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
