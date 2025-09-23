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

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SimpleTestCluster 简化的测试集群，用于测试etcd调谐逻辑
type SimpleTestCluster struct {
	T        *testing.T
	Config   *TestConfig
	MockEtcd *MockEtcdClient
	MockK8s  *SimpleK8sClient
}

// NewSimpleTestCluster 创建简化的测试集群
func NewSimpleTestCluster(t *testing.T, config *TestConfig) *SimpleTestCluster {
	if config == nil {
		config = &TestConfig{
			ClusterName:      "test-etcd-cluster",
			Namespace:        "default",
			InitialSize:      3,
			ReconcileTimeout: 30 * time.Second,
			EnableDebugLog:   true,
		}
	}

	cluster := &SimpleTestCluster{
		T:        t,
		Config:   config,
		MockEtcd: NewMockEtcdClient(),
		MockK8s:  NewSimpleK8sClient(),
	}

	// 初始化Pod
	cluster.initializePods()

	return cluster
}

// initializePods 初始化Pod
func (c *SimpleTestCluster) initializePods() {
	ctx := context.Background()

	for i := 0; i < int(c.Config.InitialSize); i++ {
		podName := fmt.Sprintf("%s-etcd-%d", c.Config.ClusterName, i)
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: c.Config.Namespace,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "etcd",
						Image: "quay.io/coreos/etcd:v3.5.21",
					},
				},
			},
		}
		c.MockK8s.CreatePod(ctx, c.Config.Namespace, pod)

		// 初始化对应的etcd成员
		peerURLs := []string{fmt.Sprintf("http://%s:2380", podName)}
		c.MockEtcd.AddMember([]string{"http://localhost:2380"}, nil, peerURLs)
	}
}

// AddMember 添加成员到etcd集群
func (c *SimpleTestCluster) AddMember(peerURLs []string) error {
	_, err := c.MockEtcd.AddMember([]string{"http://localhost:2380"}, nil, peerURLs)
	return err
}

// RemoveMember 从etcd集群移除成员
func (c *SimpleTestCluster) RemoveMember(memberID uint64) error {
	return c.MockEtcd.RemoveMember([]string{"http://localhost:2380"}, nil, memberID)
}

// ListMembers 列出etcd集群成员
func (c *SimpleTestCluster) ListMembers() ([]*etcdserverpb.Member, error) {
	resp, err := c.MockEtcd.ListMembers([]string{"http://localhost:2380"}, nil)
	if err != nil {
		return nil, err
	}
	return resp.Members, nil
}

// GetPodCount 获取Pod数量
func (c *SimpleTestCluster) GetPodCount() int {
	pods, _ := c.MockK8s.ListPods(context.Background(), c.Config.Namespace)
	return len(pods)
}

// GetMemberCount 获取成员数量
func (c *SimpleTestCluster) GetMemberCount() int {
	return c.MockEtcd.GetMemberCount()
}

// SimulatePodFailure 模拟Pod故障
func (c *SimpleTestCluster) SimulatePodFailure(podName string) {
	c.MockK8s.SetPodState(c.Config.Namespace, podName, corev1.PodFailed)

	// 找到对应的etcd成员并设置为故障状态
	for _, member := range c.MockEtcd.Members {
		if member.Name == podName {
			c.MockEtcd.SimulateMemberFailure(member.ID)
			break
		}
	}
}

// SimulatePodRecovery 模拟Pod恢复
func (c *SimpleTestCluster) SimulatePodRecovery(podName string) {
	c.MockK8s.SetPodState(c.Config.Namespace, podName, corev1.PodRunning)

	// 找到对应的etcd成员并设置为恢复状态
	for _, member := range c.MockEtcd.Members {
		if member.Name == podName {
			c.MockEtcd.SimulateMemberRecovery(member.ID)
			break
		}
	}
}

// WaitForCondition 等待条件满足
func (c *SimpleTestCluster) WaitForCondition(condition func() bool, timeout time.Duration, description string) error {
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
				c.T.Logf("Condition '%s' met after %v", description, time.Since(start))
				return nil
			}
		}
	}
}

// WaitForClusterReady 等待集群就绪
func (c *SimpleTestCluster) WaitForClusterReady(timeout time.Duration) error {
	return c.WaitForCondition(func() bool {
		return c.IsClusterReady()
	}, timeout, "cluster ready")
}

// WaitForMemberCount 等待成员数量达到预期
func (c *SimpleTestCluster) WaitForMemberCount(expected int, timeout time.Duration) error {
	return c.WaitForCondition(func() bool {
		return c.GetMemberCount() == expected
	}, timeout, fmt.Sprintf("member count = %d", expected))
}

// IsClusterReady 检查集群是否就绪
func (c *SimpleTestCluster) IsClusterReady() bool {
	// 检查Pod状态
	running, _ := c.MockK8s.GetPodState()
	if running < int(c.Config.InitialSize) {
		return false
	}

	// 检查etcd成员状态
	if !c.MockEtcd.HasQuorum() {
		return false
	}

	return true
}

// GetClusterState 获取集群状态
func (c *SimpleTestCluster) GetClusterState() ClusterState {
	running, pending := c.MockK8s.GetPodState()

	return ClusterState{
		PodCount:    running + pending,
		RunningPods: running,
		MemberCount: c.MockEtcd.GetMemberCount(),
		HasQuorum:   c.MockEtcd.HasQuorum(),
	}
}

// RunTest 运行测试
func (c *SimpleTestCluster) RunTest(testFunc func(*SimpleTestCluster) error) error {
	return testFunc(c)
}

// PrintClusterState 打印集群状态
func (c *SimpleTestCluster) PrintClusterState() {
	state := c.GetClusterState()
	c.T.Logf("=== Cluster State ===")
	c.T.Logf("Pod Count: %d (Running: %d)", state.PodCount, state.RunningPods)
	c.T.Logf("Member Count: %d, Has Quorum: %v", state.MemberCount, state.HasQuorum)
}

// AssertClusterState 断言集群状态
func (c *SimpleTestCluster) AssertClusterState(expected ClusterState) error {
	actual := c.GetClusterState()

	if expected.PodCount != actual.PodCount {
		return fmt.Errorf("pod count mismatch: expected %d, got %d", expected.PodCount, actual.PodCount)
	}

	if expected.RunningPods != actual.RunningPods {
		return fmt.Errorf("running pods mismatch: expected %d, got %d", expected.RunningPods, actual.RunningPods)
	}

	if expected.MemberCount != actual.MemberCount {
		return fmt.Errorf("member count mismatch: expected %d, got %d", expected.MemberCount, actual.MemberCount)
	}

	if expected.HasQuorum != actual.HasQuorum {
		return fmt.Errorf("quorum status mismatch: expected %v, got %v", expected.HasQuorum, actual.HasQuorum)
	}

	return nil
}

// TestClusterBasicReconciliation 基础调谐测试
func TestClusterBasicReconciliation(tc *SimpleTestCluster) error {
	tc.T.Log("Starting basic reconciliation test")

	// 等待集群就绪
	err := tc.WaitForClusterReady(30 * time.Second)
	if err != nil {
		return fmt.Errorf("cluster not ready: %v", err)
	}

	// 验证初始状态
	state := tc.GetClusterState()
	if state.PodCount != int(tc.Config.InitialSize) {
		return fmt.Errorf("expected %d pods, got %d", tc.Config.InitialSize, state.PodCount)
	}

	if !state.HasQuorum {
		return fmt.Errorf("cluster should have quorum")
	}

	tc.T.Logf("Basic reconciliation test completed successfully")
	return nil
}

// TestClusterScaling 集群扩缩容测试
func TestClusterScaling(tc *SimpleTestCluster) error {
	tc.T.Log("Starting cluster scaling test")

	// 等待初始集群就绪
	err := tc.WaitForClusterReady(30 * time.Second)
	if err != nil {
		return fmt.Errorf("initial cluster not ready: %v", err)
	}

	// 扩容到5个节点
	tc.T.Log("Scaling up to 5 nodes")
	for i := 3; i < 5; i++ {
		podName := fmt.Sprintf("%s-etcd-%d", tc.Config.ClusterName, i)
		peerURLs := []string{fmt.Sprintf("http://%s:2380", podName)}

		err := tc.AddMember(peerURLs)
		if err != nil {
			return fmt.Errorf("failed to add member %d: %v", i, err)
		}

		// 添加对应的Pod
		ctx := context.Background()
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: tc.Config.Namespace,
			},
		}
		tc.MockK8s.CreatePod(ctx, tc.Config.Namespace, pod)
	}

	// 等待扩容完成
	err = tc.WaitForMemberCount(5, 30*time.Second)
	if err != nil {
		return fmt.Errorf("scale up failed: %v", err)
	}

	tc.T.Logf("Cluster scaling test completed successfully")
	return nil
}
