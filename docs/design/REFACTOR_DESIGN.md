# ETCD Operator 重构设计文档

## 概述

本文档记录了基于core/etcd-operator的完全重构过程，旨在创建一个简化、专注于核心功能的etcd Kubernetes Operator。

## 重构目标

1. **简化API设计**: 移除复杂的备份恢复功能，专注于集群管理
2. **新API Group**: 从`etcd.etcd.io`改为`k8s.etcd.lz`
3. **参考最佳实践**: 基于成熟的core/etcd-operator设计模式
4. **清理代码**: 删除不必要的功能和代码

## API设计变更

### 新的API Group
- **旧**: `etcd.etcd.io/v1alpha1`
- **新**: `k8s.etcd.lz/v1alpha1`

### CRD简化
保留的CRD:
- `EtcdCluster`: 核心集群管理资源

删除的CRD:
- `EtcdBackup`: 备份功能（未来版本可能重新添加）
- `EtcdRestore`: 恢复功能（未来版本可能重新添加）

### EtcdCluster API结构

参考core/etcd-operator的设计，简化为：

```go
type ClusterSpec struct {
    Size       int         `json:"size"`
    Repository string      `json:"repository,omitempty"`
    Version    string      `json:"version,omitempty"`
    Paused     bool        `json:"paused,omitempty"`
    Pod        *PodPolicy  `json:"pod,omitempty"`
}

type ClusterStatus struct {
    Phase             ClusterPhase        `json:"phase"`
    Reason            string              `json:"reason,omitempty"`
    ControlPaused     bool                `json:"controlPaused,omitempty"`
    Conditions        []ClusterCondition  `json:"conditions,omitempty"`
    Size              int                 `json:"size"`
    ServiceName       string              `json:"serviceName,omitempty"`
    ClientPort        int                 `json:"clientPort,omitempty"`
    Members           MembersStatus       `json:"members"`
    CurrentVersion    string              `json:"currentVersion"`
    TargetVersion     string              `json:"targetVersion"`
}
```

## 控制器架构

### 参考core/etcd-operator设计
- **状态机模式**: 使用明确的集群状态（None, Creating, Running, Failed）
- **简化逻辑**: 移除复杂的多阶段创建逻辑
- **错误处理**: 统一的错误处理和状态更新

### 控制器方法结构
```go
func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error)
func (r *EtcdClusterReconciler) handleInitialization(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error)
func (r *EtcdClusterReconciler) handleCreating(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error)
func (r *EtcdClusterReconciler) handleRunning(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error)
func (r *EtcdClusterReconciler) handleFailed(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error)
func (r *EtcdClusterReconciler) handleDeletion(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error)
```

## 文件清理

### 删除的文件
- `api/v1alpha1/etcdbackup_types.go`
- `api/v1alpha1/etcdrestore_types.go`
- `internal/controller/etcdbackup_controller.go`
- `internal/controller/etcdrestore_controller.go`
- 相关的RBAC和sample文件

### 更新的文件
- `api/v1alpha1/etcdcluster_types.go`: 完全重写，参考core/etcd-operator
- `api/v1alpha1/groupversion_info.go`: 更新API group
- `internal/controller/etcdcluster_controller.go`: 简化的控制器实现
- 所有配置文件中的API group引用

## 配置更新

### CRD生成
- 新的CRD文件: `config/crd/bases/k8s.etcd.lz_etcdclusters.yaml`
- 删除旧的CRD文件

### RBAC配置
- 更新所有RBAC规则使用新的API group `k8s.etcd.lz`
- 简化权限，只保留etcd集群管理相关权限

### Sample配置
- 更新sample文件使用新的API结构
- 简化配置示例，专注于核心功能

## 实现状态

### 已完成
- [x] API重新设计和实现
- [x] 控制器基础框架
- [x] CRD生成和配置
- [x] RBAC配置更新
- [x] 文档更新

### 待实现
- [ ] 集群创建逻辑实现
- [ ] 扩缩容功能实现
- [ ] 健康检查和故障恢复
- [ ] 完整的测试用例

## 技术债务

### 当前问题
1. pkg目录下的代码需要更新以匹配新的API结构
2. 测试用例需要重写
3. 集群管理逻辑需要完整实现

### 解决计划
1. 逐步更新pkg包中的代码
2. 实现核心的集群管理功能
3. 添加完整的测试覆盖

## 参考资料

- [core/etcd-operator](https://github.com/coreos/etcd-operator): 参考的原始项目
- [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/): Operator模式最佳实践
- [Controller Runtime](https://github.com/kubernetes-sigs/controller-runtime): 控制器框架文档
