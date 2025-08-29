# ETCD Kubernetes Operator

[![Go Version](https://img.shields.io/badge/Go-1.22.3-blue.svg)](https://golang.org)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.22+-green.svg)](https://kubernetes.io)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)](https://github.com/etcd-lz/etcd-k8s-operator/actions)

一个基于core/etcd-operator重构的企业级 etcd Kubernetes Operator，专注于在 Kubernetes 集群中管理 etcd 实例，提供高可用、动态扩缩容和自动故障恢复等核心功能。

## 🚀 特性

### ✅ 重构完成功能
- ✅ **新API设计**: 基于k8s.etcd.lz API group的简化CRD定义
- ✅ **核心控制器**: 参考core/etcd-operator重新设计的控制器架构
- ✅ **集群管理**: 支持etcd集群的创建、删除、扩缩容
- ✅ **状态管理**: 完整的集群状态跟踪和条件管理
- ✅ **资源清理**: 优雅的资源清理和finalizer管理

### 🚧 开发中功能
- 🚧 **集群创建逻辑**: 实现完整的etcd集群创建流程
- 🚧 **扩缩容实现**: 动态成员管理和集群重配置
- 🚧 **健康检查**: etcd集群健康监控和故障检测
- 🚧 **故障恢复**: 自动故障恢复和集群修复

### 📋 计划功能
- 📋 **TLS安全**: 自动证书生成和管理
- 📋 **监控集成**: Prometheus指标和告警
- 📋 **备份恢复**: 数据备份和恢复功能（未来版本）

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
helm repo add etcd-operator https://etcd-lz.github.io/etcd-k8s-operator
helm install etcd-operator etcd-operator/etcd-operator

# 或者使用 kubectl 直接安装
kubectl apply -f https://github.com/etcd-lz/etcd-k8s-operator/releases/latest/download/install.yaml
```

### 2. 创建 ETCD 集群

#### 使用官方 etcd 镜像
```yaml
apiVersion: etcd.etcd.io/v1alpha1
kind: EtcdCluster
metadata:
  name: my-etcd-cluster
  namespace: default
spec:
  size: 3
  version: "v3.5.21"
  repository: "quay.io/coreos/etcd"
  storage:
    size: "10Gi"
    storageClassName: "fast-ssd"  # 可选
  security:
    tls:
      enabled: true
      autoTLS: true
  resources:
    requests:
      cpu: "100m"
      memory: "128Mi"
    limits:
      cpu: "500m"
      memory: "512Mi"
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

### 📋 项目管理文档
- [📋 项目主控文档](docs/project-manage/PROJECT_MASTER.md) - 项目概述、进度追踪、里程碑管理
- [🔧 技术规范文档](docs/project-manage/TECHNICAL_SPECIFICATION.md) - API 设计、控制器逻辑、技术约束
- [🧪 开发指南](docs/project-manage/DEVELOPMENT_GUIDE.md) - 开发环境、代码规范、测试指南

### 🔧 技术设计文档
- [📋 执行摘要](docs/design/EXECUTIVE_SUMMARY.md) - 动态扩缩容修复项目执行摘要
- [📊 详细修复报告](docs/design/DYNAMIC_SCALING_REPAIR_REPORT.md) - 扩缩容功能修复详细技术报告
- [🧪 端到端测试发现](docs/design/E2E_TEST_FINDINGS.md) - 测试过程中的发现和问题分析
- [🎯 扩缩容实现总结](docs/design/SCALING_IMPLEMENTATION_SUMMARY.md) - 扩缩容功能完整实现过程总结
- [🧪 扩缩容测试流程](docs/design/SCALING_TEST_PROCEDURES.md) - 扩缩容功能验证测试流程


## 🛠️ 开发

### 环境准备

```bash
# 克隆仓库
git clone https://github.com/etcd-lz/etcd-k8s-operator.git
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
# 设置测试环境
make test-setup

# 运行单元测试
make test-unit

# 运行集成测试
make test-integration

# 运行端到端测试
make test-e2e

# 运行所有测试
make test-all

# 快速测试模式
make test-fast

# 清理测试环境
make test-cleanup
```

## 📊 项目状态

> **详细进度追踪请查看**: [项目主控文档](PROJECT_MASTER.md)

### 当前阶段: 集群生命周期管理 (第4-5周)

- [x] **项目架构设计** - 完整的技术架构和设计方案
- [x] **项目初始化** - Kubebuilder 项目脚手架和基础设施
- [x] **CRD 设计实现** - 完整的 API 类型定义和验证规则
- [x] **核心控制器实现** - EtcdCluster 控制器基础逻辑
- [x] **完整测试系统** - 多层次测试架构和自动化测试
- [ ] **集群生命周期管理** - 完善集群创建、删除、更新流程 (进行中)

### 完成度: 55% (5/10 个主要阶段完成)

### 下一步重点

1. **本周目标**: 修复控制器资源创建逻辑，实现 TLS 安全配置
2. **下周目标**: 完成 etcd 客户端健康检查和高级扩缩容
3. **月度目标**: 完成备份恢复和监控集成功能

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

- 📧 邮件: [etcd-operator@etcd-lz.com](mailto:etcd-operator@etcd-lz.com)
- 💬 Slack: [#etcd-operator](https://etcd-lz.slack.com/channels/etcd-operator)
- 🐛 Issues: [GitHub Issues](https://github.com/etcd-lz/etcd-k8s-operator/issues)
- 📖 文档: [项目文档](https://etcd-lz.github.io/etcd-k8s-operator)

---

⭐ 如果这个项目对您有帮助，请给我们一个 Star！
