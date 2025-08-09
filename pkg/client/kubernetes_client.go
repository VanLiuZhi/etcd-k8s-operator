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

// kubernetesClient Kubernetes 客户端实现
type kubernetesClient struct {
	client   client.Client
	recorder record.EventRecorder
}

// NewKubernetesClient 创建 Kubernetes 客户端
func NewKubernetesClient(client client.Client, recorder record.EventRecorder) KubernetesClient {
	return &kubernetesClient{
		client:   client,
		recorder: recorder,
	}
}

// Create 创建资源
func (kc *kubernetesClient) Create(ctx context.Context, obj client.Object) error {
	return kc.client.Create(ctx, obj)
}

// Update 更新资源
func (kc *kubernetesClient) Update(ctx context.Context, obj client.Object) error {
	return kc.client.Update(ctx, obj)
}

// Delete 删除资源
func (kc *kubernetesClient) Delete(ctx context.Context, obj client.Object) error {
	return kc.client.Delete(ctx, obj)
}

// Get 获取资源
func (kc *kubernetesClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	return kc.client.Get(ctx, key, obj)
}

// List 列出资源
func (kc *kubernetesClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return kc.client.List(ctx, list, opts...)
}

// UpdateStatus 更新资源状态
func (kc *kubernetesClient) UpdateStatus(ctx context.Context, obj client.Object) error {
	return kc.client.Status().Update(ctx, obj)
}

// RecordEvent 记录事件
func (kc *kubernetesClient) RecordEvent(obj runtime.Object, eventType, reason, message string) {
	kc.recorder.Event(obj, eventType, reason, message)
}

// GetClient 获取原始客户端
func (kc *kubernetesClient) GetClient() client.Client {
	return kc.client
}

// GetRecorder 获取事件记录器
func (kc *kubernetesClient) GetRecorder() record.EventRecorder {
	return kc.recorder
}
