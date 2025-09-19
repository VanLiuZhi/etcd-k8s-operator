# Etcd Operator 测试框架

这个测试框架允许在不依赖真实Kubernetes集群的情况下测试和调试etcd operator的调谐过程。

## 架构设计

### 核心组件

1. **MockK8sClient** - 模拟Kubernetes API Server
   - 内存中管理Pod、Service、PVC等资源
   - 模拟Pod状态转换（Pending → Running）
   - 记录所有K8s操作和事件

2. **MockEtcdClient** - 模拟etcd集群
   - 内存中管理etcd成员
   - 模拟成员添加/删除操作
   - 支持法定人数检查和故障模拟

3. **TestCluster** - 测试集群封装
   - 整合Mock组件和真实调谐逻辑
   - 提供便捷的测试API
   - 自动收集测试结果

4. **测试用例** - 预定义测试场景
   - 基础调谐测试
   - 故障恢复测试
   - 集群扩缩容测试
   - 并发操作测试

## 使用方法

### 1. 运行测试

```bash
# 运行所有测试
go test ./pkg/cluster/...

# 运行特定测试
go test ./pkg/cluster/ -run TestClusterBasicReconciliation

# 运行测试并显示详细日志
go test ./pkg/cluster/ -run TestClusterFailureRecovery -v

# 运行性能测试
go test ./pkg/cluster/ -bench=BenchmarkClusterReconciliation
```

### 2. 使用调试工具

```bash
# 构建调试工具
go build ./cmd/debug-reconciler/

# 交互模式运行
./debug-reconciler -interactive=true

# 非交互模式运行预设场景
./debug-reconciler -interactive=false -debug=true

# 自定义参数
./debug-reconciler -cluster-name=my-cluster -size=5 -timeout=120s
```

### 3. 在测试中使用

```go
func TestMyFeature(t *testing.T) {
    // 创建测试配置
    config := &testutil.TestConfig{
        ClusterName:      "my-test-cluster",
        Namespace:        "default",
        InitialSize:      3,
        ReconcileTimeout: 30 * time.Second,
        EnableDebugLog:   true,
    }

    // 创建测试集群
    testCluster := testutil.NewTestCluster(t, config)
    testCluster.Start()
    defer testCluster.Stop()

    // 运行测试逻辑
    result := testCluster.RunTest(func(tc *testutil.TestCluster) error {
        // 等待集群就绪
        err := tc.WaitForClusterReady(30 * time.Second)
        if err != nil {
            return err
        }

        // 添加成员
        _, err := tc.MockEtcd.AddMember([]string{"http://localhost:2380"}, nil, []string{"http://new-member:2380"})
        if err != nil {
            return err
        }

        // 等待调谐完成
        return tc.WaitForMemberCount(4, 30 * time.Second)
    })

    // 验证结果
    if !result.Success {
        t.Fatalf("Test failed: %v", result.Error)
    }
}
```

## 主要功能

### 状态管理

```go
// 获取集群状态
state := testCluster.GetClusterState()
fmt.Printf("Pods: %d, Members: %d, Quorum: %v\n",
    state.PodCount, state.MemberCount, state.HasQuorum)

// 等待条件满足
err := testCluster.WaitForCondition(func() bool {
    return testCluster.MockEtcd.GetMemberCount() == 5
}, 30*time.Second, "member count = 5")
```

### 故障模拟

```go
// 模拟Pod故障
testCluster.SimulatePodFailure("etcd-0")

// 模拟etcd成员故障
testCluster.MockEtcd.SimulateMemberFailure(memberID)

// 模拟法定人数丢失
testCluster.MockEtcd.SetQuorumLost(true)

// 启用故障模式（所有操作都失败）
testCluster.EnableFailureMode()
```

### 操作验证

```go
// 验证集群状态
expectedState := testutil.ClusterState{
    PodCount:    3,
    RunningPods: 3,
    PendingPods: 0,
    MemberCount: 3,
    HasQuorum:   true,
}

err := testCluster.AssertClusterState(expectedState)
if err != nil {
    t.Fatalf("State assertion failed: %v", err)
}
```

## 调试功能

### 详细日志

```go
// 启用详细日志
config := &testutil.TestConfig{
    EnableDebugLog: true,
}

// 打印集群状态
testCluster.PrintClusterState()
```

### 事件跟踪

```go
// 获取K8s事件
events := testCluster.MockK8s.GetEventRecords()
for _, event := range events {
    fmt.Printf("Event: %s/%s - %s\n", event.EventType, event.Reason, event.Message)
}

// 获取etcd操作记录
addCount, removeCount, listCount := testCluster.MockEtcd.GetCallStats()
fmt.Printf("Etcd operations - Add: %d, Remove: %d, List: %d\n",
    addCount, removeCount, listCount)
```

## 测试场景

### 1. 基础调谐测试
验证集群创建、成员管理和状态同步的基本功能。

### 2. 故障恢复测试
模拟Pod故障和恢复，验证系统的自愈能力。

### 3. 法定人数测试
验证法定人数丢失时的行为和恢复机制。

### 4. 扩缩容测试
测试集群成员的动态添加和删除。

### 5. 并发操作测试
验证并发操作的正确性和一致性。

### 6. 错误处理测试
验证各种异常情况下的错误处理机制。

## 最佳实践

### 1. 测试设计
- 每个测试用例专注于一个特定功能
- 使用WaitFor*方法等待异步操作完成
- 验证最终状态，而不仅仅是操作成功

### 2. 调试技巧
- 使用PrintClusterState()查看当前状态
- 启用详细日志跟踪调谐过程
- 使用交互式调试工具进行手动测试

### 3. 性能考虑
- 在性能测试中禁用调试日志
- 合理设置超时时间
- 使用基准测试评估性能

## 扩展框架

### 添加新的Mock功能

```go
// 扩展MockK8sClient
func (m *MockK8sClient) SimulateNetworkPartition(podName string) {
    // 实现网络分区模拟
}

// 扩展MockEtcdClient
func (m *MockEtcdClient) SimulateSlowResponse(delay time.Duration) {
    // 实现延迟响应模拟
}
```

### 添加新的测试工具

```go
// 自定义测试辅助函数
func WaitForCustomCondition(tc *TestCluster, condition func() bool, timeout time.Duration) error {
    // 实现自定义条件等待
}
```

这个测试框架为etcd operator的调谐过程提供了完整的测试和调试支持，可以大大提高开发效率和代码质量。