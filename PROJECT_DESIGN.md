# ETCD Kubernetes Operator 项目设计文档

## 项目概述

本项目旨在开发一个企业级的 etcd Kubernetes Operator，用于在 Kubernetes 集群中管理 etcd 实例，实现高可用、动态扩缩容、自动故障恢复和数据维护等功能。

### 技术栈
- **Kubernetes**: 1.22+
- **Go**: 1.22.3
- **框架**: Kubebuilder v3+
- **测试环境**: Kind
- **容器运行时**: Docker/Containerd

## 架构设计

### 整体架构

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

### 核心组件

1. **EtcdCluster CRD**: 定义 etcd 集群的期望状态
2. **EtcdCluster Controller**: 核心控制器，负责集群生命周期管理
3. **Backup Controller**: 负责数据备份和恢复
4. **Monitoring Controller**: 负责健康检查和监控
5. **Webhook**: 验证和变更准入控制

## CRD 设计

### EtcdCluster CRD 规范

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: etcdclusters.etcd.io
spec:
  group: etcd.io
  versions:
  - name: v1alpha1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              size:
                type: integer
                minimum: 1
                maximum: 9
                default: 3
              version:
                type: string
                default: "3.5.9"
              repository:
                type: string
                default: "quay.io/coreos/etcd"
              storage:
                type: object
                properties:
                  storageClassName:
                    type: string
                  size:
                    type: string
                    default: "10Gi"
              security:
                type: object
                properties:
                  tls:
                    type: object
                    properties:
                      enabled:
                        type: boolean
                        default: true
          status:
            type: object
            properties:
              phase:
                type: string
                enum: ["Creating", "Running", "Failed", "Scaling"]
              readyReplicas:
                type: integer
              members:
                type: array
                items:
                  type: object
                  properties:
                    name:
                      type: string
                    id:
                      type: string
                    peerURL:
                      type: string
                    clientURL:
                      type: string
```

## 功能特性

### 1. 高可用性
- **多节点部署**: 支持 3/5/7 节点的奇数集群部署
- **故障转移**: 自动检测节点故障并进行故障转移
- **数据一致性**: 确保 etcd 集群数据的强一致性
- **网络分区容错**: 处理网络分区场景

### 2. 动态扩缩容
- **水平扩容**: 支持在线添加 etcd 节点
- **安全缩容**: 支持安全移除 etcd 节点
- **数据迁移**: 扩缩容过程中的数据平滑迁移
- **配置更新**: 动态更新集群配置

### 3. 自动故障恢复
- **健康检查**: 定期检查 etcd 节点健康状态
- **故障检测**: 快速检测节点故障和网络问题
- **自动重启**: 自动重启失败的 etcd 实例
- **数据恢复**: 从备份自动恢复损坏的数据

### 4. 数据管理
- **自动备份**: 定期创建 etcd 数据快照
- **备份存储**: 支持多种存储后端（S3、GCS、本地存储）
- **数据恢复**: 从备份快速恢复集群
- **碎片清理**: 自动清理 etcd 数据碎片

## 项目结构

```
etcd-k8s-operator/
├── api/
│   └── v1alpha1/
│       ├── etcdcluster_types.go
│       ├── etcdbackup_types.go
│       └── groupversion_info.go
├── controllers/
│   ├── etcdcluster_controller.go
│   ├── etcdbackup_controller.go
│   └── suite_test.go
├── pkg/
│   ├── etcd/
│   │   ├── client.go
│   │   ├── cluster.go
│   │   └── backup.go
│   ├── k8s/
│   │   ├── resources.go
│   │   └── utils.go
│   └── utils/
│       ├── hash.go
│       └── labels.go
├── config/
│   ├── crd/
│   ├── rbac/
│   ├── manager/
│   └── samples/
├── test/
│   ├── e2e/
│   ├── integration/
│   └── unit/
├── hack/
├── docs/
└── deploy/
    ├── helm/
    └── manifests/
```

## 开发计划

### 阶段 1: 项目基础设施 (1-2 周)
- [x] 项目架构设计与分析
- [ ] 使用 Kubebuilder 初始化项目
- [ ] 设置 CI/CD 流水线
- [ ] 配置开发环境和工具链

### 阶段 2: 核心功能开发 (3-4 周)
- [ ] 设计和实现 CRD
- [ ] 实现核心控制器逻辑
- [ ] 实现基础的集群管理功能
- [ ] 添加基本的健康检查

### 阶段 3: 高级功能开发 (4-5 周)
- [ ] 实现高可用性功能
- [ ] 实现动态扩缩容
- [ ] 实现故障检测和自动恢复
- [ ] 实现数据备份和恢复

### 阶段 4: 测试和优化 (2-3 周)
- [ ] 编写全面的测试用例
- [ ] 性能测试和优化
- [ ] 安全性测试
- [ ] 文档编写

### 阶段 5: 部署和发布 (1-2 周)
- [ ] 准备 Helm Charts
- [ ] 编写部署文档
- [ ] 准备发布版本
- [ ] 社区反馈和改进

## 质量保证

### 测试策略
1. **单元测试**: 覆盖率 > 80%
2. **集成测试**: 测试组件间交互
3. **端到端测试**: 使用 Kind 进行完整场景测试
4. **性能测试**: 压力测试和基准测试
5. **混沌工程**: 故障注入测试

### 代码质量
- 使用 golangci-lint 进行代码检查
- 遵循 Go 编码规范
- 代码审查流程
- 自动化测试流水线

## 风险评估

### 技术风险
1. **etcd 版本兼容性**: 不同版本间的兼容性问题
2. **Kubernetes API 变更**: K8s 版本升级带来的 API 变更
3. **网络分区处理**: 复杂网络环境下的一致性保证

### 缓解措施
1. 支持多个 etcd 版本
2. 使用稳定的 K8s API
3. 实现完善的网络分区检测和处理机制

## 下一步行动

1. **立即开始**: 项目初始化和环境搭建
2. **本周完成**: CRD 设计和基础控制器框架
3. **下周目标**: 实现基本的集群创建和管理功能

## 详细技术规范

### 控制器设计模式

#### Reconcile 循环设计
```go
func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. 获取 EtcdCluster 实例
    // 2. 检查删除标记，处理清理逻辑
    // 3. 验证规范合法性
    // 4. 确保必要的 K8s 资源存在
    // 5. 管理 etcd 集群状态
    // 6. 更新状态和事件
    // 7. 返回重新调度间隔
}
```

#### 状态机设计
```
Creating → Running → Scaling → Running
    ↓         ↓         ↓         ↓
  Failed ←  Failed ←  Failed ←  Failed
    ↓
  Recovering → Running
```

### 安全性设计

#### TLS 配置
- **客户端-服务器 TLS**: 保护客户端与 etcd 的通信
- **对等 TLS**: 保护 etcd 节点间的通信
- **证书管理**: 自动生成和轮换 TLS 证书
- **RBAC 集成**: 与 Kubernetes RBAC 集成

#### 网络安全
- **网络策略**: 限制 etcd 集群的网络访问
- **服务网格集成**: 支持 Istio 等服务网格
- **加密传输**: 所有数据传输加密

### 监控和可观测性

#### 指标收集
- **etcd 内置指标**: 延迟、吞吐量、存储使用率
- **集群健康指标**: 节点状态、leader 选举
- **操作指标**: 备份成功率、故障恢复时间

#### 日志管理
- **结构化日志**: 使用 JSON 格式的结构化日志
- **日志级别**: 支持动态调整日志级别
- **审计日志**: 记录所有重要操作

#### 告警规则
```yaml
groups:
- name: etcd-cluster
  rules:
  - alert: EtcdClusterDown
    expr: up{job="etcd"} == 0
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "Etcd cluster is down"
```

### 备份和恢复策略

#### 备份类型
1. **快照备份**: 定期创建 etcd 数据快照
2. **增量备份**: 基于 WAL 的增量备份
3. **跨区域备份**: 多区域备份策略

#### 恢复流程
1. **验证备份**: 检查备份完整性
2. **停止集群**: 安全停止现有集群
3. **恢复数据**: 从备份恢复数据
4. **重建集群**: 重新启动集群
5. **验证恢复**: 验证数据完整性

### 性能优化

#### 资源管理
- **CPU 限制**: 合理设置 CPU 请求和限制
- **内存管理**: 优化内存使用和垃圾回收
- **存储优化**: 使用高性能存储类

#### 网络优化
- **本地化部署**: 优先在同一节点部署
- **网络拓扑**: 考虑网络延迟和带宽
- **连接池**: 优化客户端连接管理

## 测试计划详细说明

### 单元测试
```bash
# 运行所有单元测试
make test

# 运行特定包的测试
go test ./controllers/... -v

# 生成测试覆盖率报告
make test-coverage
```

### 集成测试
```bash
# 启动测试环境
make test-integration-setup

# 运行集成测试
make test-integration

# 清理测试环境
make test-integration-cleanup
```

### 端到端测试
```bash
# 创建 Kind 集群
make kind-create

# 部署 operator
make deploy-test

# 运行 E2E 测试
make test-e2e

# 清理环境
make kind-delete
```

### 性能测试
```bash
# 运行基准测试
make benchmark

# 压力测试
make stress-test

# 生成性能报告
make performance-report
```

## 部署和运维

### Helm Chart 结构
```
helm/etcd-operator/
├── Chart.yaml
├── values.yaml
├── templates/
│   ├── deployment.yaml
│   ├── rbac.yaml
│   ├── crd.yaml
│   ├── service.yaml
│   └── configmap.yaml
└── crds/
    └── etcdclusters.yaml
```

### 运维手册
1. **安装指南**: 详细的安装步骤
2. **配置指南**: 各种配置选项说明
3. **故障排除**: 常见问题和解决方案
4. **升级指南**: 版本升级步骤
5. **监控指南**: 监控配置和告警设置

---

*本文档将随着项目进展持续更新*
