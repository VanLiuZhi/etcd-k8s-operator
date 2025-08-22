# ETCD Operator 重构设计文档

## 📋 重构背景

基于对core/etcd-operator的深度分析，该项目采用早期Kubernetes Operator开发模式，存在以下问题：
- 使用过时的技术栈（k8s 1.12.6, Go 1.10）
- 复杂的三operator架构（etcd/backup/restore）
- 原生client-go开发，维护成本高
- 不支持现代Kubernetes版本

## 🎯 重构目标

### 技术现代化
- **框架升级**: client-go原生开发 → kubebuilder v4.0.0
- **版本升级**: k8s 1.12.6 → k8s 1.28+, Go 1.10 → Go 1.23.4+
- **API现代化**: `etcd.database.coreos.com` → `k8s.etcd.lz`

### 架构简化
- **单operator架构**: 移除backup/restore operator，专注集群管理
- **CRD简化**: 只保留EtcdCluster，移除EtcdBackup/EtcdRestore
- **功能聚焦**: 专注核心的集群生命周期管理

## 🔍 原项目分析总结

### 核心架构（基于分析报告）
```
原架构：
├── etcd-operator/          # 主集群管理
├── backup-operator/        # 备份管理
└── restore-operator/       # 恢复管理

新架构：
└── etcd-k8s-operator/     # 统一集群管理
```

### 关键技术点
1. **扩缩容机制**: 基于etcd member API的动态成员管理
2. **故障恢复**: 通过健康检查和自动重启实现
3. **状态管理**: 复杂的集群状态机和条件管理
4. **TLS安全**: 自动证书生成和轮换

## 🏗️ 新架构设计

### API设计
```go
// 新的API Group: k8s.etcd.lz/v1alpha1
type EtcdCluster struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              ClusterSpec   `json:"spec"`
    Status            ClusterStatus `json:"status"`
}

// 简化的Spec设计
type ClusterSpec struct {
    Size       int        `json:"size"`                    // 集群大小
    Version    string     `json:"version,omitempty"`       // etcd版本
    Repository string     `json:"repository,omitempty"`    // 镜像仓库
    Pod        *PodPolicy `json:"pod,omitempty"`          // Pod策略
}

// 简化的Status设计
type ClusterStatus struct {
    Phase          ClusterPhase      `json:"phase"`           // 集群阶段
    Size           int               `json:"size"`            // 当前大小
    Members        MembersStatus     `json:"members"`         // 成员状态
    CurrentVersion string            `json:"currentVersion"`  // 当前版本
}
```

### 控制器设计
```go
// 基于kubebuilder的控制器
type EtcdClusterReconciler struct {
    client.Client
    Scheme   *runtime.Scheme
    Recorder record.EventRecorder
}

// 简化的状态机
const (
    ClusterPhaseNone     = ""
    ClusterPhaseCreating = "Creating"
    ClusterPhaseRunning  = "Running"
    ClusterPhaseFailed   = "Failed"
)
```

## 📊 实现计划

### 阶段1: 基础框架 (Week 1-2)
- [x] 项目分析和设计文档
- [ ] kubebuilder项目初始化
- [ ] API定义和CRD生成
- [ ] 基础控制器框架

### 阶段2: 核心功能 (Week 3-6)
- [ ] 集群创建逻辑
- [ ] 生命周期管理
- [ ] 状态监控和更新
- [ ] 基础测试用例

### 阶段3: 高级功能 (Week 7-10)
- [ ] 扩缩容实现
- [ ] 故障检测和恢复
- [ ] 健康检查机制
- [ ] 完整测试覆盖

### 阶段4: 生产就绪 (Week 11-12)
- [ ] 性能优化
- [ ] 安全加固
- [ ] 文档完善
- [ ] 发布准备

## 🔧 关键技术决策

### 1. 框架选择
**决策**: 使用kubebuilder v4.0.0
**原因**:
- 现代化的开发体验
- 自动生成样板代码
- 内置最佳实践
- 活跃的社区支持

### 2. API设计
**决策**: 新API Group `k8s.etcd.lz`
**原因**:
- 避免与原项目冲突
- 体现项目所有权
- 支持独立演进

### 3. 功能范围
**决策**: 专注集群管理，移除备份恢复
**原因**:
- 降低复杂度
- 聚焦核心价值
- 便于维护和测试

### 4. 兼容性策略
**决策**: 支持k8s 1.28+, etcd v3.5.21
**原因**:
- 面向未来的技术选择
- 利用最新特性
- 避免历史包袱

## 📚 参考资料

基于以下分析报告：
- `etcd-operator技术栈分析报告.md` - 技术栈深度分析
- `etcd-operator-CRD功能分析报告.md` - CRD功能详细分析
- `etcd-operator扩缩容实现分析.md` - 扩缩容机制分析
- `etcd-operator故障恢复处理流程详细分析.md` - 故障恢复分析
- `etcd-operator-k8s-1.26-兼容性分析报告.md` - 兼容性分析

## 🎯 成功标准

### 功能标准
- ✅ 支持etcd集群创建、删除、扩缩容
- ✅ 自动故障检测和恢复
- ✅ 完整的状态监控和报告
- ✅ 兼容k8s 1.28+环境

### 质量标准
- ✅ 单元测试覆盖率 > 80%
- ✅ 集成测试覆盖核心场景
- ✅ 端到端测试验证完整流程
- ✅ 性能测试满足生产要求

### 文档标准
- ✅ 完整的API文档
- ✅ 详细的部署指南
- ✅ 丰富的使用示例
- ✅ 故障排查手册

---

**文档版本**: v1.0 | **最后更新**: 2025-08-21
