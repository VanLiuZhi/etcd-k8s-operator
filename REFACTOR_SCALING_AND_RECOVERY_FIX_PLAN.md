# 扩缩容及故障恢复功能修复方案

## 1. 问题根源分析

当前分支在修复 `Status` 并发更新问题后，丧失了自动扩缩容和单节点故障恢复的能力。根本原因在于职责划分混乱：

1.  **`Cluster` 实例失去决策权**：为了解决 `Status` 更新冲突，我们将状态计算和决策逻辑移到了主控制器 `EtcdClusterReconciler` 中。这导致 `Cluster` 实例（`pkg/cluster/cluster.go`）本身不知道当前集群的期望状态（`spec.size`），因此无法判断应该扩容还是缩容。
2.  **主控制器错误地承担了执行任务**：在我之前的修复尝试中，错误地让 `EtcdClusterReconciler` 直接创建Pod (`createMissingPods`)。这违反了单一职责原则，`Reconciler` 应该只负责协调和管理 `Cluster` 实例的生命周期，而不是执行具体的etcd成员管理和Pod创建。

正确的模式应该是：`EtcdClusterReconciler` 作为**决策者**，`Cluster` 实例作为**执行者**。

## 2. 解决方案：建立“指令传递”机制

我们将重构代码，建立清晰的指令传递机制，以恢复 `Cluster` 实例的扩缩容能力。

### 2.1. 核心思路

1.  **决策**：`EtcdClusterReconciler` 在其 `Reconcile` 循环中，通过对比 `spec.size` 和当前实际的Pod数量，计算出需要扩容或缩容的节点数量（`delta`）。
2.  **指令传递**：`EtcdClusterReconciler` 不直接操作Pod，而是调用 `Cluster` 实例对应的方法（例如 `Reconcile(delta)`），将计算出的 `delta` 作为“指令”传递给 `Cluster` 实例。
3.  **执行**：`Cluster` 实例接收到指令后，负责执行所有底层操作，包括：
    *   **扩容 (`delta > 0`)**: 调用 `etcdctl member add`，然后创建新的Pod。
    *   **缩容 (`delta < 0`)**: 调用 `etcdctl member remove`，然后删除Pod。

### 2.2. 代码修改计划

#### 2.2.1. `internal/controller/etcdcluster_controller.go`

- **移除错误代码**: 彻底删除 `createMissingPods` 及其相关的所有Pod直接创建逻辑。
- **实现决策和指令传递**:
    - 在 `EtcdClusterReconciler.Reconcile` 方法中：
        1. 获取当前健康的Pod列表。
        2. 计算 `delta = etcdCluster.Spec.Size - len(healthyPods)`。
        3. 如果 `delta != 0`，则获取对应的 `Cluster` 实例，并调用其 `Reconcile(delta)` 方法。

#### 2.2.2. `pkg/cluster/cluster.go`

- **修改 `Cluster.Reconcile` 方法签名**: 将 `func (c *Cluster) Reconcile() error` 修改为 `func (c *Cluster) Reconcile(delta int) error`。
- **实现扩缩容逻辑**:
    - 在 `Reconcile(delta int)` 方法中：
        - 如果 `delta > 0`，循环 `delta` 次，调用 `addMember()` 方法。
        - 如果 `delta < 0`，循环 `abs(delta)` 次，调用 `removeMember()` 方法。

#### 2.2.3. `pkg/cluster/reconcile.go`

- **重构 `reconcile` 函数**: 此文件中的 `reconcile` 函数可能需要重构或废弃，因为其逻辑将被移至 `cluster.go` 中的 `Reconcile(delta int)` 方法。核心的 `addMember` 和 `removeMember` 逻辑将在这里实现或被调用。
- **实现 `addMember`**: 确保此函数正确地通过etcd API添加新成员，并创建一个新的Pod。
- **实现 `removeMember`**: 确保此函数选择一个合适的Pod，通过etcd API移除，然后再删除该Pod。

通过以上修改，我们可以恢复强大的扩缩容和故障恢复能力，同时保持主控制器管理 `Status` 的稳定性，彻底解决问题。
