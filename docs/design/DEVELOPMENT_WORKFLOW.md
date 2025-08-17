# 开发工作流指南

## 📋 概述

本文档描述了 etcd-k8s-operator 的开发工作流，包括镜像管理、Kind 集群部署和测试流程。

## 🏗️ 镜像管理策略

### 镜像命名规范

- **开发镜像**: `etcd-operator:dev-YYYYMMDD-HHMMSS`
- **示例**: `etcd-operator:dev-20250815-143022`
- **优势**: 
  - 避免缓存问题
  - 确保每次都使用最新代码
  - 便于追踪和调试

### 自动时间戳生成

```makefile
TIMESTAMP := $(shell date +%Y%m%d-%H%M%S)
IMG ?= etcd-operator:dev-$(TIMESTAMP)
```

每次执行 make 命令时都会生成新的时间戳，确保镜像唯一性。

## 🚀 开发工作流

### 1. 完整环境设置

```bash
# 一键设置完整开发环境
make dev-setup
```

这个命令会：
1. 创建 Kind 集群
2. 等待集群就绪
3. 构建最新镜像
4. 加载镜像到 Kind
5. 安装 CRD
6. 部署 Operator

### 2. 代码修改后的快速部署

```bash
# 重新构建并部署到 Kind
make kind-deploy
```

这个命令会：
1. 构建新的镜像（带新时间戳）
2. 加载到 Kind 集群
3. 更新部署配置
4. 应用到集群

### 3. 查看状态和日志

```bash
# 查看集群和 Operator 状态
make kind-status

# 查看 Operator 日志
make kind-logs

# 查看当前镜像信息
make show-image
```

## 🔧 常用命令

### 集群管理

```bash
# 创建 Kind 集群
make kind-create

# 删除 Kind 集群
make kind-delete

# 查看集群状态
make kind-status
```

### 镜像管理

```bash
# 构建镜像
make docker-build

# 加载镜像到 Kind
make kind-load

# 查看当前镜像
make show-image

# 清理旧的开发镜像
make kind-clean-images
```

### 部署管理

```bash
# 安装 CRD
make install

# 部署 Operator
make deploy

# 完整部署到 Kind
make kind-deploy

# 生成部署文件
make deploy-files
```

## 🧪 测试工作流

### 1. 功能测试

```bash
# 1. 设置环境
make dev-setup

# 2. 应用测试用例
kubectl apply -f config/samples/etcd_v1alpha1_etcdcluster.yaml

# 3. 查看结果
kubectl get etcd -A
kubectl describe etcd sample-etcdcluster

# 4. 查看日志
make kind-logs
```

### 2. 扩缩容测试

```bash
# 创建集群
kubectl apply -f config/samples/etcd_v1alpha1_etcdcluster.yaml

# 等待集群就绪
kubectl wait --for=condition=Ready etcd/sample-etcdcluster --timeout=300s

# 扩容到 5 个节点
kubectl patch etcd sample-etcdcluster --type='merge' -p='{"spec":{"size":5}}'

# 观察扩容过程
kubectl get pods -l app=etcd -w

# 缩容到 3 个节点
kubectl patch etcd sample-etcdcluster --type='merge' -p='{"spec":{"size":3}}'
```

### 3. 故障恢复测试

```bash
# 模拟节点故障
docker pause etcd-operator-dev-worker

# 观察 Pod 重新调度
kubectl get pods -o wide -w

# 恢复节点
docker unpause etcd-operator-dev-worker
```

## 🔍 调试技巧

### 1. 查看 Operator 日志

```bash
# 实时查看日志
make kind-logs

# 查看特定时间段的日志
kubectl logs -n etcd-k8s-operator-system deployment/etcd-k8s-operator-controller-manager --since=5m
```

### 2. 检查资源状态

```bash
# 查看所有相关资源
kubectl get etcd,sts,svc,cm,pvc -A

# 查看事件
kubectl get events --sort-by=.metadata.creationTimestamp

# 描述资源详情
kubectl describe etcd sample-etcdcluster
```

### 3. 进入容器调试

```bash
# 进入 etcd 容器
kubectl exec -it <etcd-pod-name> -- /bin/sh

# 检查 etcd 状态
etcdctl endpoint health
etcdctl member list
```

## 🛠️ 故障排查

### 常见问题

1. **镜像加载失败**
   ```bash
   # 检查镜像是否存在
   docker images etcd-operator
   
   # 重新构建和加载
   make docker-build
   make kind-load
   ```

2. **Pod 启动失败**
   ```bash
   # 查看 Pod 状态
   kubectl get pods -n etcd-k8s-operator-system
   
   # 查看 Pod 详情
   kubectl describe pod <pod-name> -n etcd-k8s-operator-system
   
   # 查看日志
   kubectl logs <pod-name> -n etcd-k8s-operator-system
   ```

3. **CRD 更新问题**
   ```bash
   # 重新安装 CRD
   make uninstall
   make install
   
   # 或强制更新
   kubectl replace -f config/crd/bases/
   ```

## 📊 性能监控

### 资源使用情况

```bash
# 查看节点资源使用
kubectl top nodes

# 查看 Pod 资源使用
kubectl top pods -A

# 查看 Operator 资源使用
kubectl top pods -n etcd-k8s-operator-system
```

### etcd 指标

```bash
# 进入 etcd Pod 查看指标
kubectl exec -it <etcd-pod> -- wget -qO- http://localhost:2379/metrics
```

## 🧹 清理和维护

### 清理开发环境

```bash
# 清理旧镜像
make kind-clean-images

# 删除集群
make kind-delete

# 清理 Docker 系统
docker system prune -f
```

### 重置环境

```bash
# 完全重置开发环境
make kind-delete
make kind-clean-images
make dev-setup
```

## 📚 最佳实践

1. **频繁提交**: 每次功能修改后及时测试
2. **清理镜像**: 定期运行 `make kind-clean-images`
3. **查看日志**: 遇到问题先查看 `make kind-logs`
4. **状态检查**: 使用 `make kind-status` 了解整体状态
5. **增量测试**: 先测试基本功能，再测试复杂场景

---

*本文档最后更新: 2025-08-15*
*维护者: 开发团队*
