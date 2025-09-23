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

import (
	"context"
	"fmt"
	"testing"
	"time"

	etcdv1alpha1 "github.com/etcd-lz/etcd-k8s-operator/api/v1alpha1"
	"github.com/go-logr/logr/testr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestCluster 测试集群封装
type TestCluster struct {
	// Mock组件
	MockK8s    *MockK8sClient
	MockEtcd   *MockEtcdClient
	MockClient client.Client

	// 集群组件 - 使用 interface{} 避免循环导入
	Cluster interface{}

	// 测试配置
	Config *TestConfig

	// 测试状态
	t *testing.T
}

// TestConfig 测试配置
type TestConfig struct {
	ClusterName      string
	Namespace        string
	InitialSize      int32
	ReconcileTimeout time.Duration
	EnableDebugLog   bool
}

// TestResult 测试结果
type TestResult struct {
	Success   bool
	Duration  time.Duration
	K8sEvents []EventRecord
	EtcdCalls struct {
		AddMembers    int
		RemoveMembers int
		ListMembers   int
	}
	FinalClusterState ClusterState
	Error             error
}

// ClusterState 集群状态快照
type ClusterState struct {
	PodCount    int
	RunningPods int
	PendingPods int
	MemberCount int
	HasQuorum   bool
	Members     []MemberInfo
}

// MemberInfo 成员信息
type MemberInfo struct {
	ID        uint64
	Name      string
	Started   bool
	IsLearner bool
}

// NewTestCluster 创建新的测试集群
func NewTestCluster(t *testing.T, config *TestConfig) *TestCluster {
	if config == nil {
		config = &TestConfig{
			ClusterName:      "test-etcd-cluster",
			Namespace:        "default",
			InitialSize:      3,
			ReconcileTimeout: 30 * time.Second,
			EnableDebugLog:   true,
		}
	}

	// 创建Mock组件
	mockK8s := NewMockK8sClient()
	mockEtcd := NewMockEtcdClient()
	mockClient := NewMockRuntimeClient()

	// 创建EtcdCluster CR
	_ = &etcdv1alpha1.EtcdCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      config.ClusterName,
			Namespace: config.Namespace,
		},
		Spec: etcdv1alpha1.ClusterSpec{
			Size: int(config.InitialSize),
		},
		Status: etcdv1alpha1.ClusterStatus{
			Phase: etcdv1alpha1.ClusterPhaseNone,
		},
	}

	// 创建日志记录器
	logger := testr.New(t)
	if config.EnableDebugLog {
		logger = logger.WithName("test-cluster")
	}

	// 注意：这里不再创建真实的cluster实例，避免循环导入
	// 测试代码需要自己处理cluster的创建

	return &TestCluster{
		MockK8s:    mockK8s,
		MockEtcd:   mockEtcd,
		MockClient: mockClient,
		Cluster:    nil, // 由测试代码设置
		Config:     config,
		t:          t,
	}
}

// Start 启动测试集群
func (tc *TestCluster) Start() {
	// 设置状态变化延迟，减少延迟提高测试速度
	tc.MockK8s.StateChangeDelay = 10 * time.Millisecond
	tc.MockEtcd.ClientTimeout = 5 * time.Millisecond

	tc.t.Logf("Test cluster '%s/%s' started with initial size %d",
		tc.Config.Namespace, tc.Config.ClusterName, tc.Config.InitialSize)
}

// Stop 停止测试集群
func (tc *TestCluster) Stop() {
	// 注意：这里不再直接调用cluster方法，避免循环导入
	// 测试代码需要自己处理cluster的清理

	tc.t.Logf("Test cluster '%s/%s' stopped",
		tc.Config.Namespace, tc.Config.ClusterName)
}

// WaitForCondition 等待条件满足
func (tc *TestCluster) WaitForCondition(condition func() bool, timeout time.Duration, description string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for condition '%s' after %v", description, timeout)
		case <-ticker.C:
			if condition() {
				tc.t.Logf("Condition '%s' met after %v", description, time.Since(start))
				return nil
			}
		}
	}
}

// WaitForClusterReady 等待集群就绪
func (tc *TestCluster) WaitForClusterReady(timeout time.Duration) error {
	return tc.WaitForCondition(func() bool {
		return tc.IsClusterReady()
	}, timeout, "cluster ready")
}

// WaitForPodCount 等待Pod数量达到预期
func (tc *TestCluster) WaitForPodCount(expected int, timeout time.Duration) error {
	return tc.WaitForCondition(func() bool {
		return tc.GetPodCount() == expected
	}, timeout, fmt.Sprintf("pod count = %d", expected))
}

// WaitForMemberCount 等待成员数量达到预期
func (tc *TestCluster) WaitForMemberCount(expected int, timeout time.Duration) error {
	return tc.WaitForCondition(func() bool {
		return tc.MockEtcd.GetMemberCount() == expected
	}, timeout, fmt.Sprintf("member count = %d", expected))
}

// IsClusterReady 检查集群是否就绪
func (tc *TestCluster) IsClusterReady() bool {
	// 检查Pod状态
	running, pending := tc.MockK8s.GetPodState()
	if running < int(tc.Config.InitialSize) || pending > 0 {
		return false
	}

	// 检查etcd成员状态
	if !tc.MockEtcd.HasQuorum() {
		return false
	}

	return true
}

// GetPodCount 获取Pod总数
func (tc *TestCluster) GetPodCount() int {
	running, pending := tc.MockK8s.GetPodState()
	return running + pending
}

// GetClusterState 获取集群状态快照
func (tc *TestCluster) GetClusterState() ClusterState {
	running, pending := tc.MockK8s.GetPodState()

	members := make([]MemberInfo, 0, len(tc.MockEtcd.Members))
	for _, member := range tc.MockEtcd.Members {
		if state, exists := tc.MockEtcd.MemberStates[member.ID]; exists {
			members = append(members, MemberInfo{
				ID:        member.ID,
				Name:      member.Name,
				Started:   state.Started,
				IsLearner: state.IsLearner,
			})
		}
	}

	return ClusterState{
		PodCount:    running + pending,
		RunningPods: running,
		PendingPods: pending,
		MemberCount: tc.MockEtcd.GetMemberCount(),
		HasQuorum:   tc.MockEtcd.HasQuorum(),
		Members:     members,
	}
}

// RunTest 运行测试
func (tc *TestCluster) RunTest(testFunc func(*TestCluster) error) *TestResult {
	start := time.Now()

	// 清除之前的记录
	tc.MockK8s.ClearEventRecords()
	tc.MockEtcd.ClearCallRecords()

	// 运行测试函数
	err := testFunc(tc)

	// 收集结果
	result := &TestResult{
		Success:  err == nil,
		Duration: time.Since(start),
		Error:    err,
	}

	// 收集事件记录
	result.K8sEvents = tc.MockK8s.GetEventRecords()

	// 收集调用统计
	result.EtcdCalls.AddMembers, result.EtcdCalls.RemoveMembers, result.EtcdCalls.ListMembers =
		tc.MockEtcd.GetCallStats()

	// 收集最终状态
	result.FinalClusterState = tc.GetClusterState()

	return result
}

// AssertClusterState 断言集群状态
func (tc *TestCluster) AssertClusterState(expected ClusterState) error {
	actual := tc.GetClusterState()

	if expected.PodCount != actual.PodCount {
		return fmt.Errorf("pod count mismatch: expected %d, got %d", expected.PodCount, actual.PodCount)
	}

	if expected.RunningPods != actual.RunningPods {
		return fmt.Errorf("running pods mismatch: expected %d, got %d", expected.RunningPods, actual.RunningPods)
	}

	if expected.PendingPods != actual.PendingPods {
		return fmt.Errorf("pending pods mismatch: expected %d, got %d", expected.PendingPods, actual.PendingPods)
	}

	if expected.MemberCount != actual.MemberCount {
		return fmt.Errorf("member count mismatch: expected %d, got %d", expected.MemberCount, actual.MemberCount)
	}

	if expected.HasQuorum != actual.HasQuorum {
		return fmt.Errorf("quorum status mismatch: expected %v, got %v", expected.HasQuorum, actual.HasQuorum)
	}

	return nil
}

// PrintClusterState 打印集群状态（用于调试）
func (tc *TestCluster) PrintClusterState() {
	state := tc.GetClusterState()

	tc.t.Logf("=== Cluster State ===")
	tc.t.Logf("Pod Count: %d (Running: %d, Pending: %d)",
		state.PodCount, state.RunningPods, state.PendingPods)
	tc.t.Logf("Member Count: %d, Has Quorum: %v", state.MemberCount, state.HasQuorum)
	tc.t.Logf("Members:")
	for _, member := range state.Members {
		status := "Started"
		if !member.Started {
			status = "Stopped"
		}
		if member.IsLearner {
			status += " (Learner)"
		}
		tc.t.Logf("  - %s (ID: %d): %s", member.Name, member.ID, status)
	}

	// 打印Mock状态
	tc.t.Logf("=== Mock K8s Events ===")
	for _, event := range tc.MockK8s.GetEventRecords() {
		tc.t.Logf("  Event: %s/%s - %s", event.EventType, event.Reason, event.Message)
	}

	tc.t.Logf("=== Mock Etcd State ===")
	tc.t.Log(tc.MockEtcd.PrintClusterState())
}

// SimulatePodFailure 模拟Pod故障
func (tc *TestCluster) SimulatePodFailure(podName string) {
	tc.MockK8s.SetPodState(tc.Config.Namespace, podName, corev1.PodFailed)

	// 找到对应的etcd成员并设置为故障状态
	for _, member := range tc.MockEtcd.Members {
		if member.Name == podName {
			tc.MockEtcd.SimulateMemberFailure(member.ID)
			break
		}
	}
}

// SimulatePodRecovery 模拟Pod恢复
func (tc *TestCluster) SimulatePodRecovery(podName string) {
	tc.MockK8s.SetPodState(tc.Config.Namespace, podName, corev1.PodRunning)

	// 找到对应的etcd成员并设置为恢复状态
	for _, member := range tc.MockEtcd.Members {
		if member.Name == podName {
			tc.MockEtcd.SimulateMemberRecovery(member.ID)
			break
		}
	}
}

// EnableFailureMode 启用故障模式
func (tc *TestCluster) EnableFailureMode() {
	tc.MockK8s.CreatePodFail = true
	tc.MockEtcd.SetAddMemberFail(true)
	tc.MockEtcd.SetRemoveMemberFail(true)
}

// DisableFailureMode 禁用故障模式
func (tc *TestCluster) DisableFailureMode() {
	tc.MockK8s.CreatePodFail = false
	tc.MockEtcd.SetAddMemberFail(false)
	tc.MockEtcd.SetRemoveMemberFail(false)
}

// MockEventRecorder 模拟事件记录器
type MockEventRecorder struct {
	MockK8s *MockK8sClient
}

func (m *MockEventRecorder) Event(object runtime.Object, eventtype, reason, message string) {
	m.MockK8s.RecordEvent(eventtype, reason, message)
}

func (m *MockEventRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	message := fmt.Sprintf(messageFmt, args...)
	m.MockK8s.RecordEvent(eventtype, reason, message)
}

func (m *MockEventRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
	message := fmt.Sprintf(messageFmt, args...)
	m.MockK8s.RecordEvent(eventtype, reason, message)
}

// NewMockRuntimeClient 创建新的MockRuntimeClient
func NewMockRuntimeClient() *MockRuntimeClient {
	return &MockRuntimeClient{}
}

// MockRuntimeClient 模拟runtime.Client
type MockRuntimeClient struct {
	// 简化实现，可以根据需要扩展
}

func (m *MockRuntimeClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return fmt.Errorf("not implemented")
}

func (m *MockRuntimeClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return fmt.Errorf("not implemented")
}

func (m *MockRuntimeClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return fmt.Errorf("not implemented")
}

func (m *MockRuntimeClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return fmt.Errorf("not implemented")
}

func (m *MockRuntimeClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return fmt.Errorf("not implemented")
}

func (m *MockRuntimeClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return fmt.Errorf("not implemented")
}

func (m *MockRuntimeClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return fmt.Errorf("not implemented")
}

func (m *MockRuntimeClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, fmt.Errorf("not implemented")
}

func (m *MockRuntimeClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return false, fmt.Errorf("not implemented")
}

func (m *MockRuntimeClient) RESTMapper() meta.RESTMapper {
	return nil
}

func (m *MockRuntimeClient) Scheme() *runtime.Scheme {
	return nil
}

func (m *MockRuntimeClient) SubResource(subResource string) client.SubResourceClient {
	return nil
}

func (m *MockRuntimeClient) Status() client.StatusWriter {
	return &MockStatusWriter{}
}

// MockStatusWriter 模拟状态写入器
type MockStatusWriter struct{}

func (m *MockStatusWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	return fmt.Errorf("not implemented")
}

func (m *MockStatusWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	return fmt.Errorf("not implemented")
}

func (m *MockStatusWriter) Create(ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption) error {
	return fmt.Errorf("not implemented")
}
