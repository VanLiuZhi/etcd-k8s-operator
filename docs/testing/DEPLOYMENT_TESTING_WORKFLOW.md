# ETCD Kubernetes Operator 部署和测试工作流

## 📋 文档概述

本文档旨在指导如何将ETCD Kubernetes Operator部署到Kubernetes集群中，并执行扩缩容和故障恢复测试。通过遵循本文档，您可以验证Operator的各项功能是否正常工作。

## 🎯 测试目标

1. **部署验证** - 确保Operator能够正确部署到Kubernetes集群
2. **CRD部署** - 验证自定义资源定义能够正确安装
3. **集群创建** - 验证Operator能够成功创建etcd集群
4. **扩缩容测试** - 验证集群能够正确进行扩缩容操作
5. **故障恢复测试** - 验证集群在节点故障时能够自动恢复

## 📚 文档结构

- 环境准备
- 部署流程
- 功能测试
- 扩缩容测试
- 故障恢复测试
- 清理环境

## 🛠️ 环境准备

### 系统要求

在开始部署和测试之前，请确保您的系统满足以下要求：

1. **Kubernetes集群**: Kind集群 (名称: etcd-operator-dev)
2. **kubectl**: 与Kubernetes集群版本匹配
3. **Docker**: 20.10 或更高版本
4. **Go**: 1.23.4 或更高版本

### 集群状态检查

```bash
# 检查Kind集群状态
kubectl cluster-info --context kind-etcd-operator-dev

# 检查节点状态
kubectl get nodes

# 检查集群组件
kubectl get pods -n kube-system
```

## 🚀 部署流程

### 1. 清理现有环境

在部署之前，先清理可能存在的旧资源：

```bash
# 删除现有的EtcdCluster资源
kubectl delete etcdcluster --all --all-namespaces

# 卸载CRD
make uninstall

# 删除Operator部署
make undeploy
```

### 2. 部署CRD

```bash
# 部署自定义资源定义
make install

# 验证CRD部署
kubectl get crd etcdclusters.k8s.etcd.lz
```

### 3. 构建和部署Operator

```bash
# 构建Operator镜像
make docker-build IMG=etcd-operator:dev

# 部署Operator到集群
make deploy IMG=etcd-operator:dev

# 验证Operator部署
kubectl get pods -n etcd-system
```

### 4. 检查部署状态

```bash
# 检查Operator Pod状态
kubectl get pods -n etcd-system

# 检查Operator日志
kubectl logs -n etcd-system -l control-plane=controller-manager

# 验证RBAC权限
kubectl auth can-i create etcdclusters.k8s.etcd.lz --all-namespaces
```

## 🧪 功能测试

### 1. 创建测试集群

```bash
# 应用示例EtcdCluster资源
kubectl apply -f config/samples/etcd_v1alpha1_etcdcluster.yaml

# 验证资源创建
kubectl get etcdcluster

# 查看集群详细信息
kubectl describe etcdcluster etcdcluster-sample
```

### 2. 验证集群创建

```bash
# 检查集群状态
kubectl get etcdcluster etcdcluster-sample

# 检查Pod创建
kubectl get pods -l app.kubernetes.io/name=etcd

# 检查服务创建
kubectl get services -l app.kubernetes.io/name=etcd

# 查看集群事件
kubectl describe etcdcluster etcdcluster-sample
```

## 📈 扩缩容测试

### 1. 扩容测试

```bash
# 修改集群大小进行扩容
kubectl patch etcdcluster etcdcluster-sample --type='merge' -p '{"spec":{"size":5}}'

# 观察扩容过程
kubectl get pods -l app.kubernetes.io/name=etcd -w

# 验证扩容结果
kubectl get etcdcluster etcdcluster-sample
```

### 2. 缩容测试

```bash
# 修改集群大小进行缩容
kubectl patch etcdcluster etcdcluster-sample --type='merge' -p '{"spec":{"size":3}}'

# 观察缩容过程
kubectl get pods -l app.kubernetes.io/name=etcd -w

# 验证缩容结果
kubectl get etcdcluster etcdcluster-sample
```

### 3. 扩缩容验证

```bash
# 检查集群状态
kubectl get etcdcluster etcdcluster-sample -o jsonpath='{.status.members.ready}'

# 检查Pod状态
kubectl get pods -l app.kubernetes.io/name=etcd

# 查看集群事件
kubectl describe etcdcluster etcdcluster-sample
```

## 🛡️ 故障恢复测试

### 1. 模拟节点故障

```bash
# 选择一个etcd Pod进行删除模拟故障
kubectl delete pod -l app.kubernetes.io/name=etcd --grace-period=0 --force

# 观察故障恢复过程
kubectl get pods -l app.kubernetes.io/name=etcd -w

# 检查集群状态
kubectl get etcdcluster etcdcluster-sample
```

### 2. 验证故障恢复

```bash
# 检查集群成员状态
kubectl get etcdcluster etcdcluster-sample -o jsonpath='{.status.members.ready}'

# 检查集群是否恢复正常
kubectl get pods -l app.kubernetes.io/name=etcd

# 查看恢复事件
kubectl describe etcdcluster etcdcluster-sample
```

## 🧹 清理环境

### 1. 删除测试资源

```bash
# 删除EtcdCluster资源
kubectl delete -f config/samples/etcd_v1alpha1_etcdcluster.yaml

# 等待资源清理完成
kubectl get pods -l app.kubernetes.io/name=etcd
```

### 2. 卸载Operator

```bash
# 卸载Operator
make undeploy

# 卸载CRD
make uninstall

# 验证清理结果
kubectl get crd | grep etcd
```

## 📊 测试结果分析

### 成功标准

1. **集群创建成功**: EtcdCluster资源状态为Running
2. **扩缩容正常**: 集群能够按要求调整成员数量
3. **故障恢复**: 集群在成员故障后能够自动恢复
4. **无错误日志**: Operator和etcd Pod中无严重错误

### 失败处理

如果测试失败，请按以下步骤处理：

1. **查看日志**:
   ```bash
   # 查看Operator日志
   kubectl logs -n etcd-system -l control-plane=controller-manager
   
   # 查看etcd Pod日志
   kubectl logs -l app.kubernetes.io/name=etcd
   ```

2. **检查事件**:
   ```bash
   # 查看集群事件
   kubectl describe etcdcluster etcdcluster-sample
   
   # 查看Pod事件
   kubectl describe pods -l app.kubernetes.io/name=etcd
   ```

3. **重新部署**:
   ```bash
   # 清理环境后重新部署
   make undeploy
   make uninstall
   make install
   make deploy
   ```

## 🔧 常见问题和解决方案

### 1. CRD部署失败

**问题**: CRD已存在导致部署失败
**解决方案**: 
```bash
kubectl delete crd etcdclusters.k8s.etcd.lz
make install
```

### 2. Operator无法启动

**问题**: Operator Pod处于CrashLoopBackOff状态
**解决方案**:
```bash
# 查看详细错误信息
kubectl logs -n etcd-system -l control-plane=controller-manager

# 检查RBAC权限
kubectl auth can-i create etcdclusters.k8s.etcd.lz --all-namespaces
```

### 3. 集群创建卡住

**问题**: EtcdCluster状态长时间处于Creating
**解决方案**:
```bash
# 查看集群详细信息
kubectl describe etcdcluster etcdcluster-sample

# 检查Pod状态
kubectl get pods -l app.kubernetes.io/name=etcd

# 查看Operator日志
kubectl logs -n etcd-system -l control-plane=controller-manager
```

---
**文档版本**: v1.0
**最后更新**: 2025-08-28
**维护者**: ETCD Kubernetes Operator Team