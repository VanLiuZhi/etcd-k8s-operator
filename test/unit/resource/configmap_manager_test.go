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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	resourcepkg "github.com/your-org/etcd-k8s-operator/pkg/resource"
	"github.com/your-org/etcd-k8s-operator/test/unit/mocks"
)

// TestConfigMapManager_Ensure 测试Ensure方法
func TestConfigMapManager_Ensure(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *etcdv1alpha1.EtcdCluster
		mockSetup   func(*mocks.MockKubernetesClient)
		expectError bool
		description string
	}{
		{
			name:    "成功创建新的ConfigMap",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock ConfigMap不存在，需要创建
				mockClient.On("Get", mock.Anything, types.NamespacedName{
					Name:      "test-cluster-config",
					Namespace: "default",
				}, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "configmaps"}, "test-cluster-config"))

				// Mock GetClient
				mockClient.On("GetClient").Return(nil)

				// Mock创建ConfigMap成功
				mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					cm, ok := obj.(*corev1.ConfigMap)
					if !ok {
						return false
					}
					// 验证ConfigMap的关键属性
					return cm.Name == "test-cluster-config" &&
						cm.Namespace == "default" &&
						len(cm.Data) > 0 &&
						cm.Data["etcd.conf"] != "" // 应该包含etcd配置
				})).Return(nil)
			},
			expectError: false,
			description: "应该成功创建新的ConfigMap",
		},
		{
			name:    "ConfigMap已存在，检查更新",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				existingCM := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-config",
						Namespace: "default",
					},
					Data: map[string]string{
						"etcd.conf": "# old config",
					},
				}

				// Mock ConfigMap已存在
				mockClient.On("Get", mock.Anything, types.NamespacedName{
					Name:      "test-cluster-config",
					Namespace: "default",
				}, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*corev1.ConfigMap)
					*obj = *existingCM
				}).Return(nil)

				// 可能需要更新ConfigMap，Mock Update方法
				mockClient.On("Update", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					cm, ok := obj.(*corev1.ConfigMap)
					return ok && cm.Name == "test-cluster-config"
				})).Return(nil).Maybe()
			},
			expectError: false,
			description: "ConfigMap已存在时应该检查是否需要更新",
		},
		{
			name:    "创建ConfigMap失败",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock ConfigMap不存在
				mockClient.On("Get", mock.Anything, types.NamespacedName{
					Name:      "test-cluster-config",
					Namespace: "default",
				}, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "configmaps"}, "test-cluster-config"))

				// Mock GetClient
				mockClient.On("GetClient").Return(nil)

				// Mock创建ConfigMap失败
				mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Return(apierrors.NewInternalError(fmt.Errorf("internal server error")))
			},
			expectError: true,
			description: "创建ConfigMap失败时应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockClient := &mocks.MockKubernetesClient{}
			tt.mockSetup(mockClient)

			manager := resourcepkg.NewConfigMapManager(mockClient)

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

// TestConfigMapManager_Get 测试Get方法
func TestConfigMapManager_Get(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *etcdv1alpha1.EtcdCluster
		mockSetup   func(*mocks.MockKubernetesClient)
		expectError bool
		description string
	}{
		{
			name:    "成功获取ConfigMap",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-config",
						Namespace: "default",
					},
					Data: map[string]string{
						"etcd.conf": "# etcd config",
					},
				}

				mockClient.On("Get", mock.Anything, types.NamespacedName{
					Name:      "test-cluster-config",
					Namespace: "default",
				}, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*corev1.ConfigMap)
					*obj = *cm
				}).Return(nil)
			},
			expectError: false,
			description: "应该成功获取ConfigMap",
		},
		{
			name:    "ConfigMap不存在",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				mockClient.On("Get", mock.Anything, types.NamespacedName{
					Name:      "test-cluster-config",
					Namespace: "default",
				}, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "configmaps"}, "test-cluster-config"))
			},
			expectError: true,
			description: "ConfigMap不存在时应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockClient := &mocks.MockKubernetesClient{}
			tt.mockSetup(mockClient)

			manager := resourcepkg.NewConfigMapManager(mockClient)

			// Act: 执行测试
			cm, err := manager.Get(context.Background(), tt.cluster)

			// Assert: 验证结果
			if tt.expectError {
				assert.Error(t, err, tt.description)
				// 注意：Get方法即使出错也会返回ConfigMap对象，这是正确的行为
				assert.NotNil(t, cm)
			} else {
				assert.NoError(t, err, tt.description)
				assert.NotNil(t, cm)
				assert.Equal(t, "test-cluster-config", cm.Name, "ConfigMap名称应该匹配")
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestConfigMapManager_Update 测试Update方法
func TestConfigMapManager_Update(t *testing.T) {
	tests := []struct {
		name        string
		existing    *corev1.ConfigMap
		desired     *corev1.ConfigMap
		mockSetup   func(*mocks.MockKubernetesClient)
		expectError bool
		description string
	}{
		{
			name: "成功更新ConfigMap",
			existing: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster-config",
					Namespace: "default",
				},
				Data: map[string]string{
					"etcd.conf": "# old config",
				},
			},
			desired: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster-config",
					Namespace: "default",
				},
				Data: map[string]string{
					"etcd.conf": "# updated config",
				},
			},
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				mockClient.On("Update", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					cm, ok := obj.(*corev1.ConfigMap)
					return ok && cm.Name == "test-cluster-config" && cm.Data["etcd.conf"] == "# updated config"
				})).Return(nil)
			},
			expectError: false,
			description: "应该成功更新ConfigMap",
		},
		{
			name: "更新ConfigMap失败",
			existing: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster-config",
					Namespace: "default",
				},
			},
			desired: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster-config",
					Namespace: "default",
				},
			},
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				mockClient.On("Update", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Return(apierrors.NewInternalError(fmt.Errorf("update failed")))
			},
			expectError: true,
			description: "更新ConfigMap失败时应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockClient := &mocks.MockKubernetesClient{}
			tt.mockSetup(mockClient)

			manager := resourcepkg.NewConfigMapManager(mockClient)

			// Act: 执行测试
			err := manager.Update(context.Background(), tt.existing, tt.desired)

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

// TestConfigMapManager_Delete 测试Delete方法
func TestConfigMapManager_Delete(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *etcdv1alpha1.EtcdCluster
		mockSetup   func(*mocks.MockKubernetesClient)
		expectError bool
		description string
	}{
		{
			name:    "成功删除ConfigMap",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock Get方法 - ConfigMap存在
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-config",
						Namespace: "default",
					},
				}
				mockClient.On("Get", mock.Anything, types.NamespacedName{
					Name:      "test-cluster-config",
					Namespace: "default",
				}, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*corev1.ConfigMap)
					*obj = *cm
				}).Return(nil)

				// Mock Delete方法
				mockClient.On("Delete", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					cm, ok := obj.(*corev1.ConfigMap)
					return ok && cm.Name == "test-cluster-config" && cm.Namespace == "default"
				})).Return(nil)
			},
			expectError: false,
			description: "应该成功删除ConfigMap",
		},
		{
			name:    "删除不存在的ConfigMap",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock Get方法 - ConfigMap不存在
				mockClient.On("Get", mock.Anything, types.NamespacedName{
					Name:      "test-cluster-config",
					Namespace: "default",
				}, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.ConfigMap)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "configmaps"}, "test-cluster-config"))

				// 不应该调用Delete，因为Get已经返回NotFound
			},
			expectError: false, // 删除不存在的资源通常不算错误（幂等操作）
			description: "删除不存在的ConfigMap应该成功（幂等操作）",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockClient := &mocks.MockKubernetesClient{}
			tt.mockSetup(mockClient)

			manager := resourcepkg.NewConfigMapManager(mockClient)

			// Act: 执行测试
			err := manager.Delete(context.Background(), tt.cluster)

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
