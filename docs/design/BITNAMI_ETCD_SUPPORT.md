# Bitnami etcd 支持文档

## 📋 概述

本文档详细说明了 etcd-k8s-operator 对 Bitnami etcd 镜像的支持实现，包括遇到的问题、解决方案和最佳实践。

## 🔍 问题背景

### 原始问题
在使用 Bitnami etcd 镜像 (`bitnami/etcd:3.5.9`) 时，遇到以下问题：

1. **脚本错误**: `/opt/bitnami/scripts/libetcd.sh: line 269: MY_STS_NAME: unbound variable`
2. **集群组建失败**: 每个节点都显示 "Bootstrapping a new cluster"，无法形成集群
3. **DNS 解析问题**: headless service 无法正确解析，endpoints 为空

### 根本原因分析
Bitnami etcd 在 Kubernetes 环境中需要特定的环境变量来启用集群模式：
- 缺少 `ETCD_ON_K8S=yes` 导致未启用 Kubernetes 集群模式
- 缺少 `ETCD_CLUSTER_DOMAIN` 导致 DNS 解析配置错误
- 缺少 `MY_STS_NAME` 导致脚本变量未定义错误

## ✅ 解决方案

### 1. 环境变量增强

在 `pkg/k8s/resources.go` 中添加 Bitnami 特定环境变量：

```go
// Add Bitnami-specific environment variables if using Bitnami image
if strings.Contains(cluster.Spec.Repository, "bitnami") {
    envVars = append(envVars, []corev1.EnvVar{
        {
            Name:  "ALLOW_NONE_AUTHENTICATION",
            Value: "yes",
        },
        {
            Name:  "ETCD_ROOT_PASSWORD", 
            Value: "",
        },
        {
            Name:  "ETCD_ON_K8S",
            Value: "yes",
        },
        {
            Name:  "ETCD_CLUSTER_DOMAIN",
            Value: fmt.Sprintf("%s-peer.%s.svc.cluster.local", cluster.Name, cluster.Namespace),
        },
        {
            Name:  "MY_STS_NAME",
            Value: cluster.Name,
        },
    }...)
}
```

### 2. 网络调试工具集成

添加 netshoot sidecar 容器用于网络调试：

```go
// Add netshoot sidecar container for debugging if using Bitnami image
if strings.Contains(cluster.Spec.Repository, "bitnami") {
    netshootContainer := corev1.Container{
        Name:    "netshoot",
        Image:   "nicolaka/netshoot:latest",
        Command: []string{"sleep", "3600"},
        Resources: corev1.ResourceRequirements{
            Requests: corev1.ResourceList{
                corev1.ResourceCPU:    resource.MustParse("50m"),
                corev1.ResourceMemory: resource.MustParse("64Mi"),
            },
            Limits: corev1.ResourceList{
                corev1.ResourceCPU:    resource.MustParse("100m"),
                corev1.ResourceMemory: resource.MustParse("128Mi"),
            },
        },
    }
    containers = append(containers, netshootContainer)
}
```

### 3. 测试用例更新

在 `pkg/k8s/resources_test.go` 中添加环境变量验证：

```go
assert.Equal(suite.T(), "yes", envMap["ALLOW_NONE_AUTHENTICATION"])
assert.Equal(suite.T(), "yes", envMap["ETCD_ON_K8S"])
assert.Equal(suite.T(), "test-bitnami-cluster-peer.default.svc.cluster.local", envMap["ETCD_CLUSTER_DOMAIN"])
assert.Equal(suite.T(), "test-bitnami-cluster", envMap["MY_STS_NAME"])
```

## 🔧 环境变量详解

| 环境变量 | 值 | 作用 |
|----------|----|----- |
| `ALLOW_NONE_AUTHENTICATION` | `yes` | 允许无认证访问（开发环境） |
| `ETCD_ROOT_PASSWORD` | `""` | 根用户密码（空表示无密码） |
| `ETCD_ON_K8S` | `yes` | **关键**：启用 Kubernetes 集群模式 |
| `ETCD_CLUSTER_DOMAIN` | `{cluster-name}-peer.{namespace}.svc.cluster.local` | **关键**：指定 headless service 域名 |
| `MY_STS_NAME` | `{cluster-name}` | **关键**：StatefulSet 名称，避免脚本错误 |

## 🛠️ 调试工具使用

### netshoot 容器功能
- **DNS 测试**: `nslookup`, `dig`
- **网络连通性**: `ping`, `telnet`, `nc`
- **HTTP 测试**: `curl`, `wget`
- **网络分析**: `tcpdump`, `ss`, `netstat`

### 常用调试命令

```bash
# 进入 netshoot 容器
kubectl exec -it <pod-name> -c netshoot -- bash

# 测试 DNS 解析
kubectl exec -it <pod-name> -c netshoot -- nslookup <service-name>

# 测试网络连通性
kubectl exec -it <pod-name> -c netshoot -- ping <target-ip>

# 测试 etcd 连接
kubectl exec -it <pod-name> -c etcd -- etcdctl --endpoints=http://localhost:2379 member list
```

## 📊 验证结果

### 成功指标
- ✅ Pod 状态: `2/2 Running` (etcd + netshoot)
- ✅ etcd 日志: 显示 "became leader" 和 "ready to serve client requests"
- ✅ DNS 解析: headless service 正确解析到 Pod IP
- ✅ endpoints: 包含正确的 Pod IP 地址
- ✅ etcd 功能: 可以正常读写数据

### 测试命令
```bash
# 检查 Pod 状态
kubectl get pods -l app.kubernetes.io/name=etcd

# 检查 etcd 日志
kubectl logs <pod-name> -c etcd

# 测试 etcd 功能
kubectl exec -it <pod-name> -c etcd -- etcdctl --endpoints=http://localhost:2379 put test-key test-value
kubectl exec -it <pod-name> -c etcd -- etcdctl --endpoints=http://localhost:2379 get test-key
```

## 🎯 最佳实践

### 1. 镜像选择
- **开发环境**: 推荐使用 `bitnami/etcd` (易于调试)
- **生产环境**: 推荐使用 `quay.io/coreos/etcd` (官方镜像)

### 2. 资源配置
```yaml
resources:
  requests:
    cpu: "100m"
    memory: "128Mi"
  limits:
    cpu: "500m" 
    memory: "512Mi"
```

### 3. 存储配置
```yaml
storage:
  size: "10Gi"
  storageClassName: "fast-ssd"
```

## 🔮 未来改进

1. **自动镜像检测**: 根据镜像类型自动配置环境变量
2. **调试模式开关**: 可选择是否启用 netshoot sidecar
3. **多镜像支持**: 扩展对其他 etcd 镜像的支持
4. **安全增强**: 生产环境的安全配置优化

## 📚 相关文档

- [项目主控文档](../PROJECT_MASTER.md)
- [技术规范文档](../TECHNICAL_SPECIFICATION.md)
- [开发指南](../DEVELOPMENT_GUIDE.md)
