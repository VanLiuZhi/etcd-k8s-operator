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

package cluster

import (
	"fmt"
	"testing"
	"time"

	"github.com/etcd-lz/etcd-k8s-operator/pkg/testutil"
)

func TestSimpleClusterBasicReconciliation(t *testing.T) {
	// 创建测试配置
	config := &testutil.TestConfig{
		ClusterName:      "test-simple-reconcile",
		Namespace:        "default",
		InitialSize:      3,
		ReconcileTimeout: 30 * time.Second,
		EnableDebugLog:   true,
	}

	// 创建简化的测试集群
	testCluster := testutil.NewSimpleTestCluster(t, config)

	// 运行基础调谐测试
	err := testCluster.RunTest(testutil.TestClusterBasicReconciliation)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	// 验证最终状态
	expectedState := testutil.ClusterState{
		PodCount:    3,
		RunningPods: 3,
		MemberCount: 3,
		HasQuorum:   true,
	}

	if err := testCluster.AssertClusterState(expectedState); err != nil {
		t.Fatalf("Cluster state assertion failed: %v", err)
	}

	// 打印调试信息
	testCluster.PrintClusterState()

	t.Logf("Simple cluster basic reconciliation test completed successfully")
}

func TestSimpleClusterScaling(t *testing.T) {
	// 创建测试配置
	config := &testutil.TestConfig{
		ClusterName:      "test-simple-scaling",
		Namespace:        "default",
		InitialSize:      3,
		ReconcileTimeout: 60 * time.Second,
		EnableDebugLog:   true,
	}

	// 创建简化的测试集群
	testCluster := testutil.NewSimpleTestCluster(t, config)

	// 运行扩缩容测试
	err := testCluster.RunTest(testutil.TestClusterScaling)
	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	// 验证最终状态
	expectedState := testutil.ClusterState{
		PodCount:    5,
		RunningPods: 5,
		MemberCount: 5,
		HasQuorum:   true,
	}

	if err := testCluster.AssertClusterState(expectedState); err != nil {
		t.Fatalf("Cluster state assertion failed: %v", err)
	}

	// 打印调试信息
	testCluster.PrintClusterState()

	t.Logf("Simple cluster scaling test completed successfully")
}

func TestSimpleClusterFailureRecovery(t *testing.T) {
	// 创建测试配置
	config := &testutil.TestConfig{
		ClusterName:      "test-simple-failure-recovery",
		Namespace:        "default",
		InitialSize:      3,
		ReconcileTimeout: 60 * time.Second,
		EnableDebugLog:   true,
	}

	// 创建简化的测试集群
	testCluster := testutil.NewSimpleTestCluster(t, config)

	// 自定义测试函数
	err := testCluster.RunTest(func(tc *testutil.SimpleTestCluster) error {
		tc.T.Log("Starting failure recovery test")

		// 1. 等待初始集群就绪
		err := tc.WaitForClusterReady(30 * time.Second)
		if err != nil {
			return fmt.Errorf("initial cluster not ready: %v", err)
		}

		// 2. 模拟一个Pod故障
		tc.T.Log("Simulating pod failure")
		failedPodName := "test-simple-failure-recovery-etcd-0"
		tc.SimulatePodFailure(failedPodName)

		// 3. 等待一段时间观察状态
		time.Sleep(1 * time.Second)

		// 4. 模拟Pod恢复
		tc.T.Log("Simulating pod recovery")
		tc.SimulatePodRecovery(failedPodName)

		// 5. 等待集群恢复
		err = tc.WaitForClusterReady(30 * time.Second)
		if err != nil {
			return fmt.Errorf("cluster recovery failed: %v", err)
		}

		tc.T.Log("Failure recovery test completed successfully")
		return nil
	})

	if err != nil {
		t.Fatalf("Test failed: %v", err)
	}

	// 验证最终状态
	expectedState := testutil.ClusterState{
		PodCount:    3,
		RunningPods: 3,
		MemberCount: 3,
		HasQuorum:   true,
	}

	if err := testCluster.AssertClusterState(expectedState); err != nil {
		t.Fatalf("Cluster state assertion failed: %v", err)
	}

	testCluster.PrintClusterState()
	t.Logf("Simple cluster failure recovery test completed successfully")
}