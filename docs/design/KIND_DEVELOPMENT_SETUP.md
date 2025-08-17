# Kind 开发环境配置指南

## 📋 概述

本文档描述了为 etcd-k8s-operator 开发的简化 Kind 集群配置。该配置提供了基本的多节点 Kubernetes 环境用于 Operator 开发和测试。

## 🎯 配置特点

### 🏗️ 集群架构
- **Kubernetes 版本**: v1.28.0
- **节点配置**: 1个控制平面节点 + 3个工作节点
- **集群名称**: `etcd-operator-dev`
- **网络**: Kind 默认 CNI (kindnet)
- **配置**: 最小化配置，使用 Kubernetes 默认设置

## 📁 相关文件

### hack/kind-config.yaml
简化的 Kind 集群配置文件，包含：
- 4个节点配置 (1个控制平面 + 3个工作节点)
- Kubernetes v1.28.0 镜像
- 使用默认网络和存储配置

## 🚀 使用方法

### 快速开始

```bash
# 创建开发集群
make kind-create

# 安装 CRD
make install

# 本地运行 operator
make run

# 或者部署到集群
make deploy

# 测试功能
kubectl apply -f config/samples/

# 删除集群
make kind-delete
```

### 手动操作

```bash
# 使用 kind 命令直接创建
kind create cluster --name etcd-operator-dev --config hack/kind-config.yaml

# 删除集群
kind delete cluster --name etcd-operator-dev
```

## 🔍 基本调试功能

### 集群状态检查
```bash
# 检查集群状态
kubectl cluster-info
kubectl get nodes

# 查看系统 Pod
kubectl get pods -A

# 查看 etcd 健康状态
kubectl exec -n kube-system etcd-etcd-operator-dev-control-plane -- etcdctl endpoint health
```

### Operator 调试
```bash
# 查看 operator 日志 (本地运行时)
# 日志会直接输出到终端

# 查看 operator 日志 (部署到集群时)
kubectl logs -n etcd-k8s-operator-system deployment/etcd-k8s-operator-controller-manager

# 查看 CRD 状态
kubectl get crd | grep etcd

# 查看自定义资源
kubectl get etcd,etcdbackup,etcdrestore -A
```

## 🧪 测试场景支持

### 扩缩容测试
- 3个工作节点支持多节点 etcd 集群测试
- 可以测试从单节点到多节点的扩容
- 可以测试多节点到单节点的缩容

### 故障恢复测试
```bash
# 模拟节点故障
docker pause etcd-operator-dev-worker
docker unpause etcd-operator-dev-worker

# 查看 Pod 重新调度
kubectl get pods -o wide
```

### 基本功能测试
- 创建、更新、删除 etcd 集群
- 备份和恢复功能测试
- 控制器逻辑验证

## 🔧 自定义配置

如果需要自定义配置，可以编辑 `hack/kind-config.yaml`:

### 添加端口映射
```yaml
nodes:
- role: control-plane
  image: kindest/node:v1.28.0@sha256:b7a4cad12c197af3ba43202d3efe03246b3f0793f162afb40a33c923952d5b31
  extraPortMappings:
  - containerPort: 8080    # 添加端口映射
    hostPort: 8080
    protocol: TCP
```

### 修改节点数量
```yaml
# 减少到2个工作节点
nodes:
- role: control-plane
  image: kindest/node:v1.28.0@sha256:b7a4cad12c197af3ba43202d3efe03246b3f0793f162afb40a33c923952d5b31
- role: worker
  image: kindest/node:v1.28.0@sha256:b7a4cad12c197af3ba43202d3efe03246b3f0793f162afb40a33c923952d5b31
- role: worker
  image: kindest/node:v1.28.0@sha256:b7a4cad12c197af3ba43202d3efe03246b3f0793f162afb40a33c923952d5b31
```

## 📊 资源使用

### 系统要求
- **CPU**: 最少 2 核 (推荐 4 核)
- **内存**: 最少 4GB (推荐 8GB)
- **磁盘**: 最少 10GB 可用空间
- **Docker**: 20.10+ 版本

### 集群资源分配
- **控制平面**: ~1GB 内存, ~0.5 CPU
- **每个工作节点**: ~512MB 内存, ~0.25 CPU
- **系统组件**: ~512MB 内存, ~0.25 CPU

## 🚨 故障排查

### 常见问题

1. **集群创建失败**
   ```bash
   # 检查 Docker 状态
   docker info
   
   # 检查端口占用
   lsof -i :6443
   
   # 清理旧集群
   kind delete cluster --name etcd-operator-dev
   ```

2. **网络问题**
   ```bash
   # 检查 CNI 状态 (Kind 默认使用 kindnet)
   kubectl get pods -n kube-system -l app=kindnet

   # 重启网络组件
   kubectl delete pods -n kube-system -l app=kindnet
   ```

3. **节点未就绪**
   ```bash
   # 检查节点状态
   kubectl get nodes
   kubectl describe node <node-name>

   # 等待节点就绪
   kubectl wait --for=condition=Ready nodes --all --timeout=300s
   ```

## 📚 相关文档

- [Kind 官方文档](https://kind.sigs.k8s.io/)
- [Kind 配置参考](https://kind.sigs.k8s.io/docs/user/configuration/)
- [Kubernetes 开发指南](https://kubernetes.io/docs/concepts/)
- [etcd 运维指南](https://etcd.io/docs/v3.5/op-guide/)

---

*本文档最后更新: 2025-08-15*
*维护者: 开发团队*
