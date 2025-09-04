# Etcd Operator Size状态修复验证工作流

## 概述

基于现有测试工作流，验证3种Size状态修复方案的效果。

## 验证目标

- 修复 `status.size` 与实际Pod数量不一致问题
- 对比3种方案的修复效果和性能影响
- 选择最优方案用于生产环境

## 前置条件

参考 `ETCD_OPERATOR_TESTING_WORKFLOW.md` 的环境准备部分。

## 方案概览

参考 `SIZE_STATUS_SYNC_ISSUE_ANALYSIS.md` 的3种解决方案：

- **方案1**：统一状态更新逻辑（分支：`fix/size-sync-v1`）
- **方案2**：原子性状态更新（分支：`fix/size-sync-v2`）
- **方案3**：标准Kubebuilder模式（分支：`fix/size-sync-v3`）

## 验证流程

### 准备工作
```bash
# 1. 创建基准分支
git checkout dev-1
git checkout -b baseline/size-fix

# 2. 环境清理
kubectl delete etcdcluster --all --all-namespaces
make undeploy
make uninstall
```

### 方案1验证（快速修复）
```bash
# 1. 创建分支
git checkout baseline/size-fix
git checkout -b fix/size-sync-v1

# 2. 实施修改（参考SIZE_STATUS_SYNC_ISSUE_ANALYSIS.md方案1）
# 修改pkg/cluster/cluster.go的updateMemberStatus方法

# 3. 构建部署
make docker-build IMG=etcd-operator:size-fix-v1
make deploy IMG=etcd-operator:size-fix-v1

# 4. 功能测试（参考ETCD_OPERATOR_TESTING_WORKFLOW.md）
kubectl apply -f config/samples/etcd_v1alpha1_etcdcluster.yaml
# 监控状态同步，扩缩容测试，故障恢复测试

# 5. 记录结果
echo "## 方案1测试结果" > docs/results/solution-v1.md
# 记录测试数据和观察

# 6. 提交
git add .
git commit -m "fix: 方案1 - 统一状态更新逻辑"
git push origin fix/size-sync-v1
```

### 方案2验证（原子性更新）
```bash
# 1. 创建分支
git checkout baseline/size-fix
git checkout -b fix/size-sync-v2

# 2. 实施修改（参考SIZE_STATUS_SYNC_ISSUE_ANALYSIS.md方案2）
# 重构状态更新机制

# 3. 构建部署
make docker-build IMG=etcd-operator:size-fix-v2
make deploy IMG=etcd-operator:size-fix-v2

# 4. 功能测试
# 同样的测试流程

# 5. 记录结果
echo "## 方案2测试结果" > docs/results/solution-v2.md

# 6. 提交
git add .
git commit -m "refactor: 方案2 - 原子性状态更新"
git push origin fix/size-sync-v2
```

### 方案3验证（标准Kubebuilder）
```bash
# 1. 创建分支
git checkout baseline/size-fix
git checkout -b fix/size-sync-v3

# 2. 实施修改（参考SIZE_STATUS_SYNC_ISSUE_ANALYSIS.md方案3）
# 重构控制器

# 3. 构建部署
make docker-build IMG=etcd-operator:size-fix-v3
make deploy IMG=etcd-operator:size-fix-v3

# 4. 功能测试
# 同样的测试流程

# 5. 记录结果
echo "## 方案3测试结果" > docs/results/solution-v3.md

# 6. 提交
git add .
git commit -m "refactor: 方案3 - 标准Kubebuilder模式"
git push origin fix/size-sync-v3
```

## 结果对比

### 测试结果记录
```bash
# 创建结果对比文档
cat > docs/results/size-fix-comparison.md << EOF
# Size状态修复方案对比

## 测试环境
- 测试时间: $(date)
- Kubernetes版本: $(kubectl version --short | grep 'Server Version')
- Docker版本: $(docker --version | cut -d' ' -f3 | cut -d',' -f1)

## 方案对比

| 方案 | 修复效果 | 构建复杂度 | 性能影响 | 推荐度 |
|------|----------|------------|----------|--------|
| 方案1 | [待测试] | 低 | [待测试] | [待评估] |
| 方案2 | [待测试] | 中 | [待测试] | [待评估] |
| 方案3 | [待测试] | 高 | [待测试] | [待评估] |

## 详细结果

### 方案1: 统一状态更新逻辑
#### 修复效果
- [测试结果]

#### 性能影响
- [测试数据]

#### 观察到的问题
- [问题描述]

### 方案2: 原子性状态更新
#### 修复效果
- [测试结果]

#### 性能影响
- [测试数据]

#### 观察到的问题
- [问题描述]

### 方案3: 标准Kubebuilder模式
#### 修复效果
- [测试结果]

#### 性能影响
- [测试数据]

#### 观察到的问题
- [问题描述]

## 最终推荐
[基于测试结果的推荐]
EOF
```

## 验证检查清单

### 每个方案的验证要点
- [ ] Size状态与Pod数量一致
- [ ] 扩缩容后状态正确更新
- [ ] 故障恢复后状态正确
- [ ] 并发更新无冲突
- [ ] 性能影响在可接受范围
- [ ] 日志输出正常
- [ ] 无功能回归

## 快速验证命令

```bash
# 状态一致性验证
echo "Spec: $(kubectl get etcdcluster etcdcluster-sample -o jsonpath='{.spec.size}')"
echo "Status: $(kubectl get etcdcluster etcdcluster-sample -o jsonpath='{.status.size}')"
echo "Pods: $(kubectl get pods -l app=etcd --no-headers | wc -l)"

# 扩缩容测试
kubectl patch etcdcluster etcdcluster-sample -p '{"spec":{"size":5}}' --type=merge
sleep 60
kubectl patch etcdcluster etcdcluster-sample -p '{"spec":{"size":3}}' --type=merge

# 故障恢复测试
kubectl delete pod $(kubectl get pods -l app=etcd -o jsonpath='{.items[0].metadata.name}')
```

---

**文档版本**: v1.0  
**创建时间**: 2025-09-04  
**维护者**: Etcd Operator Team