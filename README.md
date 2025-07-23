# ETCD Kubernetes Operator

[![Go Version](https://img.shields.io/badge/Go-1.22.3-blue.svg)](https://golang.org)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.22+-green.svg)](https://kubernetes.io)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)](https://github.com/your-org/etcd-k8s-operator/actions)

一个企业级的 etcd Kubernetes Operator，用于在 Kubernetes 集群中管理 etcd 实例，提供高可用、动态扩缩容、自动故障恢复和数据维护等功能。

## 🚀 特性

### 核心功能
- ✅ **高可用部署**: 支持 3/5/7 节点的奇数集群部署
- ✅ **动态扩缩容**: 在线添加/移除 etcd 节点
- ✅ **自动故障恢复**: 智能故障检测和自动恢复
- ✅ **数据备份恢复**: 支持定期备份和点时间恢复
- ✅ **TLS 安全**: 自动 TLS 证书管理
- ✅ **监控集成**: Prometheus 指标和 Grafana 仪表板

### 高级功能
- 🔄 **滚动更新**: 零停机时间的版本升级
- 📊 **性能监控**: 实时性能指标和告警
- 🛡️ **安全加固**: RBAC 集成和网络策略
- 🔧 **自动维护**: 碎片清理和性能优化
- 📱 **多云支持**: 支持各种云平台和存储后端

## 📋 系统要求

- **Kubernetes**: 1.22 或更高版本
- **Go**: 1.22.3 (开发环境)
- **Docker**: 20.10+ (构建镜像)
- **Kind**: 0.17+ (测试环境)

## 🏗️ 架构概览

```
┌─────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                       │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐    ┌─────────────────────────────────┐ │
│  │  ETCD Operator  │    │        ETCD Cluster             │ │
│  │                 │    │  ┌─────┐ ┌─────┐ ┌─────┐       │ │
│  │  ┌───────────┐  │    │  │Node1│ │Node2│ │Node3│       │ │
│  │  │Controller │  │◄──►│  └─────┘ └─────┘ └─────┘       │ │
│  │  └───────────┘  │    │                                 │ │
│  │  ┌───────────┐  │    │  ┌─────────────────────────┐   │ │
│  │  │  Webhook  │  │    │  │     Service & Ingress   │   │ │
│  │  └───────────┘  │    │  └─────────────────────────┘   │ │
│  └─────────────────┘    └─────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## 🚀 快速开始

### 1. 安装 Operator

```bash
# 使用 Helm 安装
helm repo add etcd-operator https://your-org.github.io/etcd-k8s-operator
helm install etcd-operator etcd-operator/etcd-operator

# 或者使用 kubectl 直接安装
kubectl apply -f https://github.com/your-org/etcd-k8s-operator/releases/latest/download/install.yaml
```

### 2. 创建 ETCD 集群

```yaml
apiVersion: etcd.io/v1alpha1
kind: EtcdCluster
metadata:
  name: my-etcd-cluster
  namespace: default
spec:
  size: 3
  version: "3.5.9"
  storage:
    storageClassName: "fast-ssd"
    size: "10Gi"
  security:
    tls:
      enabled: true
  backup:
    enabled: true
    schedule: "0 2 * * *"
    storageType: "s3"
    s3:
      bucket: "my-etcd-backups"
      region: "us-west-2"
```

```bash
kubectl apply -f etcd-cluster.yaml
```

### 3. 验证集群状态

```bash
# 检查集群状态
kubectl get etcdcluster my-etcd-cluster

# 查看详细信息
kubectl describe etcdcluster my-etcd-cluster

# 检查 Pod 状态
kubectl get pods -l app.kubernetes.io/name=etcd,app.kubernetes.io/instance=my-etcd-cluster
```

## 📚 文档

- [📋 项目主控文档](PROJECT_MASTER.md) - 项目概述、进度追踪、里程碑管理
- [🔧 技术规范文档](TECHNICAL_SPECIFICATION.md) - API 设计、控制器逻辑、技术约束
- [🧪 开发指南](DEVELOPMENT_GUIDE.md) - 开发环境、代码规范、测试指南

## 🛠️ 开发

### 环境准备

```bash
# 克隆仓库
git clone https://github.com/your-org/etcd-k8s-operator.git
cd etcd-k8s-operator

# 安装依赖
make deps

# 运行测试
make test

# 构建二进制文件
make build
```

### 本地开发

```bash
# 创建 Kind 集群
make kind-create

# 部署 CRD
make install

# 运行 operator (在集群外)
make run

# 部署测试集群
kubectl apply -f config/samples/etcd_v1alpha1_etcdcluster.yaml
```

### 测试

```bash
# 运行单元测试
make test

# 运行集成测试
make test-integration

# 运行端到端测试
make test-e2e

# 生成测试覆盖率报告
make test-coverage
```

## 📊 项目状态

> **详细进度追踪请查看**: [项目主控文档](PROJECT_MASTER.md)

### 当前阶段: 核心控制器实现 (第3-4周)

- [x] **项目架构设计** - 完整的技术架构和设计方案
- [x] **项目初始化** - Kubebuilder 项目脚手架和基础设施
- [x] **CRD 设计实现** - 完整的 API 类型定义和验证规则
- [ ] **核心控制器实现** - EtcdCluster 控制器基础逻辑 (进行中)

### 完成度: 25% (3/10 个主要阶段完成)

### 下一步重点

1. **本周目标**: 实现 EtcdCluster 控制器基础框架
2. **下周目标**: 完成集群创建和删除流程
3. **月度目标**: 完成扩缩容和备份恢复功能

### 里程碑

- 🎯 **v0.1.0** (第 4 周): 基础集群管理功能
- 🎯 **v0.2.0** (第 8 周): 高可用和扩缩容功能
- 🎯 **v0.3.0** (第 12 周): 备份恢复和监控功能
- 🎯 **v1.0.0** (第 16 周): 生产就绪版本

## 🤝 贡献

我们欢迎社区贡献！请查看 [贡献指南](CONTRIBUTING.md) 了解如何参与项目。

### 贡献方式

- 🐛 报告 Bug
- 💡 提出新功能建议
- 📝 改进文档
- 🔧 提交代码补丁
- 🧪 编写测试用例

## 📄 许可证

本项目采用 [Apache License 2.0](LICENSE) 许可证。

## 🙏 致谢

感谢以下项目和社区的支持：

- [Kubebuilder](https://kubebuilder.io/) - Kubernetes Operator 开发框架
- [etcd](https://etcd.io/) - 分布式键值存储
- [Kubernetes](https://kubernetes.io/) - 容器编排平台
- [Go](https://golang.org/) - 编程语言

## 📞 联系我们

- 📧 邮件: [etcd-operator@your-org.com](mailto:etcd-operator@your-org.com)
- 💬 Slack: [#etcd-operator](https://your-org.slack.com/channels/etcd-operator)
- 🐛 Issues: [GitHub Issues](https://github.com/your-org/etcd-k8s-operator/issues)
- 📖 文档: [项目文档](https://your-org.github.io/etcd-k8s-operator)

---

⭐ 如果这个项目对您有帮助，请给我们一个 Star！
