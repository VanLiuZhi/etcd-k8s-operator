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

// TestServiceManager_EnsureServices 测试EnsureServices方法
func TestServiceManager_EnsureServices(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *etcdv1alpha1.EtcdCluster
		mockSetup   func(*mocks.MockKubernetesClient)
		expectError bool
		description string
	}{
		{
			name:    "成功创建客户端和对等服务",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock客户端服务不存在，需要创建
				mockClient.On("Get", mock.Anything, types.NamespacedName{
					Name:      "test-cluster-client",
					Namespace: "default",
				}, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "services"}, "test-cluster-client"))

				// Mock对等服务不存在，需要创建
				mockClient.On("Get", mock.Anything, types.NamespacedName{
					Name:      "test-cluster-peer",
					Namespace: "default",
				}, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "services"}, "test-cluster-peer"))

				// Mock GetClient - 返回nil（跳过ControllerReference设置）
				mockClient.On("GetClient").Return(nil)

				// Mock创建客户端服务
				mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					svc, ok := obj.(*corev1.Service)
					return ok && svc.Name == "test-cluster-client"
				})).Return(nil)

				// Mock创建对等服务
				mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					svc, ok := obj.(*corev1.Service)
					return ok && svc.Name == "test-cluster-peer"
				})).Return(nil)
			},
			expectError: false,
			description: "应该成功创建客户端和对等服务",
		},
		{
			name:    "服务已存在，无需创建",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				clientSvc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-client",
						Namespace: "default",
					},
				}
				peerSvc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-peer",
						Namespace: "default",
					},
				}

				// Mock客户端服务已存在
				mockClient.On("Get", mock.Anything, types.NamespacedName{
					Name:      "test-cluster-client",
					Namespace: "default",
				}, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*corev1.Service)
					*obj = *clientSvc
				}).Return(nil)

				// Mock对等服务已存在
				mockClient.On("Get", mock.Anything, types.NamespacedName{
					Name:      "test-cluster-peer",
					Namespace: "default",
				}, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*corev1.Service)
					*obj = *peerSvc
				}).Return(nil)

				// 可能需要更新服务，Mock Update方法
				mockClient.On("Update", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Return(nil).Maybe() // Maybe()表示可能调用也可能不调用

				// 不应该调用Create，因为服务已存在
			},
			expectError: false,
			description: "服务已存在时不应该重复创建",
		},
		{
			name:    "创建客户端服务失败",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock客户端服务不存在
				mockClient.On("Get", mock.Anything, types.NamespacedName{
					Name:      "test-cluster-client",
					Namespace: "default",
				}, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "services"}, "test-cluster-client"))

				// Mock GetClient
				mockClient.On("GetClient").Return(nil)

				// Mock创建客户端服务失败
				mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					svc, ok := obj.(*corev1.Service)
					return ok && svc.Name == "test-cluster-client"
				})).Return(apierrors.NewInternalError(fmt.Errorf("internal server error")))
			},
			expectError: true,
			description: "创建客户端服务失败时应该返回错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockClient := &mocks.MockKubernetesClient{}
			tt.mockSetup(mockClient)

			manager := resourcepkg.NewServiceManager(mockClient)

			// Act: 执行测试
			err := manager.EnsureServices(context.Background(), tt.cluster)

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

// TestServiceManager_EnsureClientService 测试EnsureClientService方法
func TestServiceManager_EnsureClientService(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *etcdv1alpha1.EtcdCluster
		mockSetup   func(*mocks.MockKubernetesClient)
		expectError bool
		description string
	}{
		{
			name:    "成功创建客户端服务",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock服务不存在
				mockClient.On("Get", mock.Anything, types.NamespacedName{
					Name:      "test-cluster-client",
					Namespace: "default",
				}, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "services"}, "test-cluster-client"))

				// Mock GetClient
				mockClient.On("GetClient").Return(nil)

				// Mock创建服务成功
				mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					svc, ok := obj.(*corev1.Service)
					if !ok {
						return false
					}
					// 验证服务的关键属性
					return svc.Name == "test-cluster-client" &&
						svc.Namespace == "default" &&
						len(svc.Spec.Ports) > 0 &&
						svc.Spec.Ports[0].Port == 2379 // etcd客户端端口
				})).Return(nil)
			},
			expectError: false,
			description: "应该成功创建客户端服务",
		},
		{
			name:    "客户端服务已存在",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				existingSvc := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-cluster-client",
						Namespace: "default",
					},
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name: "client",
								Port: 2379,
							},
						},
					},
				}

				// Mock服务已存在
				mockClient.On("Get", mock.Anything, types.NamespacedName{
					Name:      "test-cluster-client",
					Namespace: "default",
				}, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Run(func(args mock.Arguments) {
					obj := args.Get(2).(*corev1.Service)
					*obj = *existingSvc
				}).Return(nil)

				// 可能需要更新服务，Mock Update方法
				mockClient.On("Update", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Return(nil).Maybe()

				// 不应该调用Create
			},
			expectError: false,
			description: "客户端服务已存在时不应该重复创建",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockClient := &mocks.MockKubernetesClient{}
			tt.mockSetup(mockClient)

			manager := resourcepkg.NewServiceManager(mockClient)

			// Act: 执行测试
			err := manager.EnsureClientService(context.Background(), tt.cluster)

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

// TestServiceManager_EnsurePeerService 测试EnsurePeerService方法
func TestServiceManager_EnsurePeerService(t *testing.T) {
	tests := []struct {
		name        string
		cluster     *etcdv1alpha1.EtcdCluster
		mockSetup   func(*mocks.MockKubernetesClient)
		expectError bool
		description string
	}{
		{
			name:    "成功创建对等服务",
			cluster: createTestCluster("test-cluster", 3),
			mockSetup: func(mockClient *mocks.MockKubernetesClient) {
				// Mock服务不存在
				mockClient.On("Get", mock.Anything, types.NamespacedName{
					Name:      "test-cluster-peer",
					Namespace: "default",
				}, mock.MatchedBy(func(obj interface{}) bool {
					_, ok := obj.(*corev1.Service)
					return ok
				})).Return(apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "services"}, "test-cluster-peer"))

				// Mock GetClient
				mockClient.On("GetClient").Return(nil)

				// Mock创建服务成功
				mockClient.On("Create", mock.Anything, mock.MatchedBy(func(obj interface{}) bool {
					svc, ok := obj.(*corev1.Service)
					if !ok {
						return false
					}
					// 验证对等服务的关键属性
					return svc.Name == "test-cluster-peer" &&
						svc.Namespace == "default" &&
						len(svc.Spec.Ports) > 0 &&
						svc.Spec.Ports[0].Port == 2380 && // etcd对等端口
						svc.Spec.ClusterIP == "None" // Headless服务
				})).Return(nil)
			},
			expectError: false,
			description: "应该成功创建对等服务（Headless）",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 准备测试环境
			mockClient := &mocks.MockKubernetesClient{}
			tt.mockSetup(mockClient)

			manager := resourcepkg.NewServiceManager(mockClient)

			// Act: 执行测试
			err := manager.EnsurePeerService(context.Background(), tt.cluster)

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
