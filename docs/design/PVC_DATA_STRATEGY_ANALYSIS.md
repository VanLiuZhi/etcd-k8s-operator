# PVC数据策略分析和测试方案

## 📊 问题分析

### 🔍 当前问题
在etcd集群扩缩容过程中，PVC数据保留策略导致状态污染：

1. **缩容到1节点后**：
   - ✅ etcd成员被正确移除
   - ✅ StatefulSet副本数正确更新
   - ❌ **PVC数据仍然保留旧的集群配置**
   - ❌ **单节点etcd仍认为自己是多节点集群的一部分**

2. **再次扩容时**：
   - 新Pod使用旧的PVC数据目录
   - 旧数据包含之前的集群成员信息
   - etcd启动时发现状态不一致
   - 导致集群无法正常启动

### 🎯 根本原因
**StatefulSet的PVC保留策略**：
- StatefulSet删除Pod时，PVC默认保留
- etcd数据目录包含集群状态信息
- 缩容后的单节点仍保留多节点集群的配置
- 扩容时新节点无法加入已损坏的集群状态

## 🧪 测试方案设计

### 📋 测试场景

#### 场景1: 当前实现 - PVC数据保留
**测试步骤**：
1. 创建1节点集群 → 验证正常运行
2. 扩容到3节点 → 验证扩容成功
3. 缩容到1节点 → 验证缩容成功
4. **再次扩容到3节点** → **预期失败** ❌

**预期结果**：
- 第4步失败，因为PVC数据污染
- 验证当前问题的存在

#### 场景2: PVC清理策略
**实现方案**：
- 缩容时删除多余的PVC
- 确保数据完全清理

**测试步骤**：
1. 创建1节点集群
2. 扩容到3节点
3. 缩容到1节点 + **删除多余PVC**
4. 再次扩容到3节点 → **预期成功** ✅

#### 场景3: 数据目录重置策略
**实现方案**：
- 保留PVC但清理etcd数据目录
- 重置单节点集群状态

**测试步骤**：
1. 创建1节点集群
2. 扩容到3节点
3. 缩容到1节点 + **重置数据目录**
4. 再次扩容到3节点 → **预期成功** ✅

### 🔧 技术实现方案

#### 方案A: PVC删除策略
```go
// 在handleScaleDown中添加PVC清理
func (r *EtcdClusterReconciler) handleScaleDown(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
    // 1. 移除etcd成员
    // 2. 更新StatefulSet副本数
    // 3. 删除多余的PVC (新增)
    return r.cleanupExtraPVCs(ctx, cluster, desiredReplicas)
}
```

#### 方案B: 数据目录重置策略
```go
// 在缩容到单节点时重置集群状态
func (r *EtcdClusterReconciler) resetSingleNodeCluster(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
    // 1. 停止etcd进程
    // 2. 清理数据目录中的集群状态
    // 3. 重新初始化为单节点集群
    // 4. 重启etcd进程
}
```

### 📊 测试验证指标

#### 功能验证
- [ ] 1→3→1→3 扩缩容循环成功
- [ ] 所有Pod状态为 `2/2 Running`
- [ ] etcd集群健康检查通过
- [ ] 数据读写功能正常

#### 性能验证
- [ ] 扩容时间 < 2分钟
- [ ] 缩容时间 < 1分钟
- [ ] 数据清理时间 < 30秒
- [ ] 无数据丢失

#### 稳定性验证
- [ ] 多次扩缩容循环稳定
- [ ] 异常情况下的恢复能力
- [ ] 资源清理完整性

## 🎯 推荐方案

### 优先级1: 数据目录重置策略
**优势**：
- 保留PVC，避免数据丢失风险
- 只清理集群状态，保留用户数据
- 实现相对简单

**实现**：
1. 在缩容到单节点时，重置etcd数据目录
2. 清理 `member/` 目录下的集群状态
3. 保留 `wal/` 和 `snap/` 中的数据

### 优先级2: PVC删除策略
**优势**：
- 彻底清理，避免状态污染
- 实现简单直接

**风险**：
- 可能导致数据丢失
- 需要谨慎处理

## ✅ 已实现的修复方案

### 🔧 核心修复内容

#### 1. PVC自动清理 (利用StorageClass Delete策略)
```go
func (r *EtcdClusterReconciler) cleanupExtraPVCs(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, desiredReplicas, currentReplicas int32) error {
    // 删除多余的PVC，让StorageClass的Delete策略自动删除PV
    for i := desiredReplicas; i < currentReplicas; i++ {
        pvcName := fmt.Sprintf("data-%s-%d", cluster.Name, i)
        // 删除PVC，StorageClass ReclaimPolicy=Delete 会自动清理PV
    }
}
```

#### 2. 单节点集群状态重置
```go
func (r *EtcdClusterReconciler) resetSingleNodeCluster(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
    // 删除单节点Pod，让StatefulSet重新创建
    // 新Pod启动时会以干净的单节点模式初始化
}
```

#### 3. 状态同步修复
```go
// 修复updateMemberStatus - 基于实际Pod而不是期望Size
// 避免显示不存在的成员状态
```

### 🎯 StorageClass策略分析

**当前环境**:
- StorageClass: `standard` (默认)
- ReclaimPolicy: `Delete` ✅
- Provisioner: `rancher.io/local-path`

**工作原理**:
1. StatefulSet缩容时不会自动删除PVC (Kubernetes设计)
2. 我们手动删除多余的PVC
3. StorageClass的Delete策略自动删除对应的PV
4. 彻底清理存储，避免数据污染

## 📝 测试验证计划

### 🧪 完整测试场景

#### 测试1: 基础扩缩容循环
```bash
# 1. 创建单节点集群
kubectl apply -f test/testdata/test-scaling-scenarios.yaml

# 2. 验证单节点正常
kubectl get pods -n test-scaling
kubectl get etcdcluster -n test-scaling

# 3. 扩容到3节点
kubectl patch etcdcluster test-scaling-cluster -n test-scaling --type='merge' -p='{"spec":{"size":3}}'

# 4. 验证3节点集群
kubectl get pods -n test-scaling
kubectl get pvc -n test-scaling

# 5. 缩容到1节点 (关键测试)
kubectl patch etcdcluster test-scaling-cluster -n test-scaling --type='merge' -p='{"spec":{"size":1}}'

# 6. 验证PVC清理
kubectl get pvc -n test-scaling  # 应该只有1个PVC

# 7. 再次扩容到3节点 (关键测试)
kubectl patch etcdcluster test-scaling-cluster -n test-scaling --type='merge' -p='{"spec":{"size":3}}'

# 8. 验证成功
kubectl get pods -n test-scaling  # 应该全部2/2 Running
```

#### 测试2: 多轮循环稳定性
```bash
# 连续多次扩缩容测试
for i in {1..3}; do
  echo "=== Round $i ==="
  kubectl patch etcdcluster test-scaling-cluster -n test-scaling --type='merge' -p='{"spec":{"size":3}}'
  sleep 60
  kubectl patch etcdcluster test-scaling-cluster -n test-scaling --type='merge' -p='{"spec":{"size":1}}'
  sleep 60
done
```

### 📊 验证指标

#### 功能验证
- [ ] 1→3→1→3 扩缩容循环成功
- [ ] PVC数量与集群大小一致
- [ ] 所有Pod状态为 `2/2 Running`
- [ ] etcd集群健康检查通过

#### 资源清理验证
- [ ] 缩容后多余PVC被删除
- [ ] StorageClass Delete策略生效
- [ ] 无残留PV资源
- [ ] 单节点状态正确重置

#### 稳定性验证
- [ ] 多轮扩缩容循环稳定
- [ ] 无状态污染问题
- [ ] 错误恢复能力正常
