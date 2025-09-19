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

package testutil

// MockK8sClient Mock K8s客户端类型别名，用于向后兼容
type MockK8sClient = SimpleK8sClient

// NewMockK8sClient 创建新的Mock K8s客户端
func NewMockK8sClient() *MockK8sClient {
	return NewSimpleK8sClient()
}