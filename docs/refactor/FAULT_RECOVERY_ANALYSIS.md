# 故障恢复场景分析报告

本文档通过一个具体的故障场景，深入分析 `_Reference/etcd-operator` 的代码执行流程，并以此为基准，验证当前重构项目的故障恢复能力。

---

## 1. 场景设定

1.  **初始状态**: 一个健康的3节点 etcd 集群。
    *   `EtcdCluster` CRD: `spec.size: 3`
    *   Operator 内存中的成员列表 `c.members`: `[member-1, member-2, member-3]`
    *   Kubernetes 中运行的 Pods: `[pod-1, pod-2, pod-3]` (全部 Running 和 Ready)

2.  **故障事件**: `pod-2` 意外宕机。例如，其所在的 Kubernetes Node 节点失联。

---

## 2. `_Reference/etcd-operator` 执行流程追踪

我们将以 `pkg/cluster/reconcile.go` 中的 `reconcile()` 函数为入口，追踪 Operator 的反应。

### **协调周期 1: 检测到故障**

1.  **`pollPods()` 被调用**
    *   **输入**: `cluster.Name`
    *   **输出 (running)**: `[pod-1, pod-3]` (因为 `pod-2` 已经无法被 kubelet 报告为 Running)
    *   **输出 (pending)**: `[]`

2.  **`reconcile(pods)` 被调用**
    *   **输入**: `pods = [pod-1, pod-3]`

3.  **`reconcileMembers(running)` 被调用**
    *   `running` 集合: `[member-1, member-3]`
    *   `c.members` 集合: `[member-1, member-2, member-3]`
    *   `!running.IsEqual(c.members)` 条件成立 (2个成员 vs 3个成员)，进入 `reconcileMembers`。

4.  **`reconcileMembers` 内部逻辑**
    *   `unknownMembers := running.Diff(c.members)` -> `[]` (没有未知的 Pod)
    *   `L := running.Diff(unknownMembers)` -> `[member-1, member-3]`
    *   `L.Size() == c.members.Size()` -> `2 == 3` -> `false`。
    *   `L.Size() < c.members.Size()/2 + 1` -> `2 < 3/2 + 1` -> `2 < 2` -> `false`。**结论：集群未丢失法定人数 (quorum)**。
    *   `deadMember := c.members.Diff(L).PickOne()` -> `member-2` 被识别为“死亡成员”。
    *   **`removeDeadMember(deadMember)` 被调用**。

5.  **`removeMember(member-2)` 被调用**
    *   **数据操作 1**: 通过 etcd client 向 etcd 集群发送 `MemberRemove` 请求，移除 `member-2` 的 ID。
    *   **数据操作 2**: Operator 更新其内存中的 `c.members` 列表，移除 `member-2`。`c.members` 现在是 `[member-1, member-3]`。
    *   **数据操作 3**: Operator 向 Kubernetes API Server 发送请求，删除 `pod-2` 这个对象。

**周期 1 结束**: Operator 成功检测到故障，并将故障节点从集群中清理出去。此刻，集群规模为2，小于期望的3。

### **协调周期 2: 执行恢复**

1.  **`pollPods()` 被调用**
    *   **输出 (running)**: `[pod-1, pod-3]`

2.  **`reconcile(pods)` 被调用**
    *   **输入**: `pods = [pod-1, pod-3]`

3.  **`reconcileMembers(running)` 被调用**
    *   `running` 集合: `[member-1, member-3]`
    *   `c.members` 集合: `[member-1, member-3]` (已在上一周期更新)
    *   `!running.IsEqual(c.members)` -> `false`。
    *   `c.members.Size() != sp.Size` -> `2 != 3` -> `true`。**结论：集群规模不一致**。
    *   进入 `reconcileMembers`。

4.  **`reconcileMembers` 内部逻辑**
    *   `L.Size() == c.members.Size()` -> `2 == 2` -> `true`。
    *   返回 `c.resize()`。

5.  **`resize()` 被调用**
    *   `c.members.Size() < c.cluster.Spec.Size` -> `2 < 3` -> `true`。
    *   **`addOneMember()` 被调用**。

6.  **`addOneMember()` 被调用**
    *   **数据操作 1**: 创建一个新的成员对象，比如 `member-4`。
    *   **数据操作 2**: 通过 etcd client 向 etcd 集群发送 `MemberAdd` 请求，添加 `member-4`。
    *   **数据操作 3**: Operator 更新其内存中的 `c.members` 列表，添加 `member-4`。`c.members` 现在是 `[member-1, member-3, member-4]`。
    *   **数据操作 4**: Operator 向 Kubernetes API Server 发送请求，创建 `pod-4`。

**周期 2 结束**: Operator 已成功启动恢复流程，新的 Pod 正在创建中。

### **协调周期 3: 状态稳定**

1.  **`pollPods()` 被调用**
    *   `pod-4` 成功启动并 Ready。
    *   **输出 (running)**: `[pod-1, pod-3, pod-4]`

2.  **`reconcile(pods)` 被调用**
    *   `running` 集合: `[member-1, member-3, member-4]`
    *   `c.members` 集合: `[member-1, member-3, member-4]`
    *   `!running.IsEqual(c.members)` -> `false`。
    *   `c.members.Size() != sp.Size` -> `3 != 3` -> `false`。
    *   所有条件均满足，协调结束。集群恢复到健康的3节点状态。

---

## 3. 结论

通过上述数据驱动的流程追踪，我们可以清晰地看到 `_Reference/etcd-operator` 是如何通过 **“检测 -> 清理 -> 恢复”** 的闭环来保证集群高可用的。

这个分析为我们提供了一个明确的、可验证的基准模型。
