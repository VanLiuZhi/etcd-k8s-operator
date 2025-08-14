# etcd-k8s-operator 扩缩容功能测试流程

## 概述

本文档提供了完整的etcd-k8s-operator扩缩容功能测试流程，用于验证扩缩容功能是否达成目标。

## 测试目标

验证以下功能是否正常工作：
- ✅ 单节点集群创建
- ✅ 扩容功能（1→3节点）
- ✅ 缩容功能（3→2节点）
- ✅ 状态同步正确性
- ✅ 调试日志完整性

## 前置条件

### 1. 环境准备

**必需组件**：
- Kubernetes集群（本地或远程）
- kubectl命令行工具
- Docker（用于构建镜像）
- Go 1.21+（用于编译）

**验证环境**：
```bash
# 检查Kubernetes集群状态
kubectl cluster-info

# 检查节点状态
kubectl get nodes

# 检查命名空间
kubectl get namespaces
```

### 2. 代码准备

**获取最新代码**：
```bash
# 确保在项目根目录
cd etcd-k8s-operator

# 检查当前分支和状态
git status
git log --oneline -5
```

## 测试流程

### 阶段1：环境清理和准备

#### 1.1 清理现有资源

```bash
# 删除现有的EtcdCluster资源
kubectl delete etcdcluster --all

# 删除相关的StatefulSet
kubectl delete statefulset --all

# 删除相关的Service
kubectl delete service -l app.kubernetes.io/name=etcd

# 删除相关的ConfigMap
kubectl delete configmap -l app.kubernetes.io/name=etcd

# 删除相关的PVC
kubectl delete pvc --all

# 等待资源完全删除
kubectl get pods
```

#### 1.2 构建和部署Operator

```bash
# 构建镜像
make docker-build

# 部署CRD
make install

# 部署Operator
make deploy

# 验证Operator部署状态
kubectl get pods -n etcd-operator-system
kubectl logs -n etcd-operator-system deployment/etcd-operator-controller-manager -f
```

**预期结果**：
- Operator Pod状态为Running
- 日志显示"Starting manager"和"Starting workers"

### 阶段2：单节点集群创建测试

#### 2.1 创建单节点集群

```bash
# 创建测试集群
cat <<EOF | kubectl apply -f -
apiVersion: etcd.etcd.io/v1alpha1
kind: EtcdCluster
metadata:
  name: test-scaling
  namespace: default
spec:
  size: 1
  version: "v3.5.21"
  repository: "quay.io/coreos/etcd"
EOF
```

#### 2.2 监控集群创建过程

**终端1 - 监控Pod状态**：
```bash
kubectl get pods -w
```

**终端2 - 监控EtcdCluster状态**：
```bash
watch -n 2 'kubectl get etcdcluster test-scaling'
```

**终端3 - 监控Operator日志**：
```bash
kubectl logs -n etcd-operator-system deployment/etcd-operator-controller-manager -f
```

#### 2.3 验证单节点集群

**检查最终状态**：
```bash
# 检查EtcdCluster状态
kubectl get etcdcluster test-scaling

# 检查Pod状态
kubectl get pods

# 检查StatefulSet状态
kubectl get statefulset test-scaling

# 检查Service状态
kubectl get service -l app.kubernetes.io/instance=test-scaling
```

**预期结果**：
- EtcdCluster: `PHASE: Running, SIZE: 1, READY: 1`
- Pod: `test-scaling-0` 状态为 `2/2 Running`
- StatefulSet: `READY: 1/1`

### 阶段3：扩容测试（1→3节点）

#### 3.1 执行扩容操作

```bash
# 修改集群大小为3
kubectl patch etcdcluster test-scaling --type='merge' -p='{"spec":{"size":3}}'
```

#### 3.2 监控扩容过程

**继续监控（使用阶段2的监控命令）**：
- 观察Pod创建过程
- 观察EtcdCluster状态变化
- 观察Operator日志中的调试信息

#### 3.3 验证扩容结果

**等待扩容完成后检查**：
```bash
# 检查最终状态
kubectl get etcdcluster test-scaling
kubectl get pods
kubectl get statefulset test-scaling

# 检查etcd集群成员
kubectl exec test-scaling-0 -c etcd -- etcdctl member list
```

**预期结果**：
- EtcdCluster: `PHASE: Running, SIZE: 3, READY: 3`
- Pod: `test-scaling-0`, `test-scaling-1`, `test-scaling-2` 都是 `2/2 Running`
- StatefulSet: `READY: 3/3`
- etcd成员列表显示3个成员

#### 3.4 验证调试日志

**在Operator日志中查找关键信息**：
```bash
# 查找扩容相关的调试日志
kubectl logs -n etcd-operator-system deployment/etcd-operator-controller-manager | grep -A 5 -B 5 "SCALING-DEBUG"

# 查找扩容完成日志
kubectl logs -n etcd-operator-system deployment/etcd-operator-controller-manager | grep "Scaling completed"
```

**预期日志内容**：
```
SCALING-DEBUG: Raw cluster spec cluster.Spec.Size: 3
SCALING-DEBUG: StatefulSet info sts.Spec.Replicas: 3
Cluster needs scaling current: 1, desired: 3
Scaling completed finalSize: 3
```

### 阶段4：缩容测试（3→2节点）

#### 4.1 执行缩容操作

```bash
# 修改集群大小为2
kubectl patch etcdcluster test-scaling --type='merge' -p='{"spec":{"size":2}}'
```

#### 4.2 监控缩容过程

**继续使用相同的监控命令**，特别关注：
- Pod删除过程（应该删除`test-scaling-2`）
- EtcdCluster状态变化
- 调试日志输出

#### 4.3 验证缩容结果

**等待缩容完成后检查**：
```bash
# 检查最终状态
kubectl get etcdcluster test-scaling
kubectl get pods
kubectl get statefulset test-scaling

# 检查etcd集群成员
kubectl exec test-scaling-0 -c etcd -- etcdctl member list

# 确认PVC清理
kubectl get pvc
```

**预期结果**：
- EtcdCluster: `PHASE: Running, SIZE: 2, READY: 2`
- Pod: 只有`test-scaling-0`, `test-scaling-1`，都是 `2/2 Running`
- StatefulSet: `READY: 2/2`
- etcd成员列表显示2个成员
- `test-scaling-2`相关的PVC被删除

### 阶段5：状态一致性验证

#### 5.1 状态对比检查

**检查各层状态一致性**：
```bash
# EtcdCluster状态
kubectl get etcdcluster test-scaling -o jsonpath='{.status.readyReplicas}'

# StatefulSet状态
kubectl get statefulset test-scaling -o jsonpath='{.status.readyReplicas}'

# 实际Pod数量
kubectl get pods -l app.kubernetes.io/instance=test-scaling --no-headers | wc -l

# etcd集群成员数量
kubectl exec test-scaling-0 -c etcd -- etcdctl member list | wc -l
```

#### 5.2 状态更新时效性测试

**测试状态更新延迟**：
```bash
# 强制触发状态更新
kubectl patch etcdcluster test-scaling --type='merge' -p='{"metadata":{"annotations":{"test-update":"'$(date)'"}}}'

# 立即检查状态
kubectl get etcdcluster test-scaling

# 等待5秒后再次检查
sleep 5
kubectl get etcdcluster test-scaling
```

**预期结果**：
- 所有状态数值应该一致
- 状态更新应该在合理时间内完成（<30秒）

### 阶段6：错误场景测试

#### 6.1 快速连续扩缩容测试

```bash
# 快速执行多次扩缩容
kubectl patch etcdcluster test-scaling --type='merge' -p='{"spec":{"size":3}}'
sleep 10
kubectl patch etcdcluster test-scaling --type='merge' -p='{"spec":{"size":1}}'
sleep 10
kubectl patch etcdcluster test-scaling --type='merge' -p='{"spec":{"size":2}}'

# 观察系统行为
kubectl get etcdcluster test-scaling -w
```

#### 6.2 异常恢复测试

```bash
# 手动删除一个Pod，观察恢复
kubectl delete pod test-scaling-1

# 监控恢复过程
kubectl get pods -w
```

## 测试验收标准

### 功能性验收标准

**必须满足的条件**：
- ✅ 单节点集群能成功创建并运行
- ✅ 扩容操作能正确执行，目标节点数达成
- ✅ 缩容操作能正确执行，多余节点被删除
- ✅ etcd集群成员管理正确
- ✅ StatefulSet副本数与期望一致
- ✅ 所有Pod最终状态为Running

### 性能验收标准

**时间要求**：
- 单节点集群创建：< 3分钟
- 扩容操作完成：< 5分钟
- 缩容操作完成：< 3分钟
- 状态更新延迟：< 30秒

### 日志验收标准

**必须包含的日志**：
- `SCALING-DEBUG: Raw cluster spec` - 显示正确的目标大小
- `SCALING-DEBUG: StatefulSet info` - 显示正确的副本状态
- `Cluster needs scaling` - 显示扩缩容需求识别
- `Scaling completed` - 显示扩缩容完成

### 状态一致性验收标准

**状态一致性要求**：
- EtcdCluster.Status.ReadyReplicas = StatefulSet.Status.ReadyReplicas
- EtcdCluster.Spec.Size = 实际运行的Pod数量
- etcd集群成员数 = EtcdCluster.Spec.Size
- kubectl输出的READY字段 = 实际就绪的Pod数量

## 问题排查指南

### 常见问题及解决方法

**问题1：Pod创建失败**
```bash
# 检查Pod事件
kubectl describe pod <pod-name>

# 检查镜像拉取
kubectl get events --sort-by=.metadata.creationTimestamp
```

**问题2：扩缩容无响应**
```bash
# 检查Operator日志
kubectl logs -n etcd-operator-system deployment/etcd-operator-controller-manager --tail=100

# 检查EtcdCluster状态
kubectl describe etcdcluster test-scaling
```

**问题3：状态不一致**
```bash
# 强制状态更新
kubectl patch etcdcluster test-scaling --type='merge' -p='{"metadata":{"annotations":{"force-update":"'$(date)'"}}}'

# 检查控制器日志中的状态更新信息
kubectl logs -n etcd-operator-system deployment/etcd-operator-controller-manager | grep "updateClusterStatus"
```

## 测试报告模板

### 测试执行记录

**测试环境**：
- Kubernetes版本：
- 节点数量：
- 测试时间：

**测试结果**：
- [ ] 单节点集群创建：通过/失败
- [ ] 扩容测试：通过/失败
- [ ] 缩容测试：通过/失败
- [ ] 状态一致性：通过/失败
- [ ] 调试日志：通过/失败

**性能数据**：
- 单节点创建时间：
- 扩容完成时间：
- 缩容完成时间：
- 状态更新延迟：

**发现的问题**：
1. 问题描述
2. 重现步骤
3. 影响程度
4. 建议解决方案

**总体评估**：
- 功能完整性：
- 性能表现：
- 稳定性：
- 用户体验：

## 结论

通过执行以上测试流程，可以全面验证etcd-k8s-operator扩缩容功能的正确性、性能和稳定性。测试完成后，根据验收标准判断功能是否达成目标。
