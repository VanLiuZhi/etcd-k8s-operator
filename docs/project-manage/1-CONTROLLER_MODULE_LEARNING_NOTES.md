# Controller 模块学习笔记

## 1. 模块概述

### 1.1 模块作用
Controller 模块是 Kubernetes Operator 的核心，负责监听 EtcdCluster 资源的变化并协调实际集群状态向期望状态收敛。

### 1.2 核心文件
```
internal/controller/
└── etcdcluster_controller.go  # EtcdCluster 控制器实现
```

## 2. 核心概念详解

### 2.1 Reconcile 协调循环

#### 问题：什么是 Reconcile 协调循环？
**解答**：Reconcile 是 Kubernetes Controller 的核心机制，通过持续比较资源的期望状态和实际状态，并采取行动使两者一致。

**实现结构**：
```go
func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. 获取资源实例
    // 2. 处理删除事件
    // 3. 添加 Finalizer
    // 4. 设置默认值
    // 5. 验证规格
    // 6. 管理集群实例
    // 7. 返回结果
}
```

#### 问题：Reconcile 的工作流程是怎样的？
**解答**：Reconcile 遵循标准的 Kubernetes Controller 模式：

**完整流程**：
```go
func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    logger := log.FromContext(ctx).WithValues("etcdcluster", req.NamespacedName)
    logger.Info("Starting reconciliation")

    // 1. 获取 EtcdCluster 实例
    etcdCluster := &etcdv1alpha1.EtcdCluster{}
    if err := r.Get(ctx, req.NamespacedName, etcdCluster); err != nil {
        if apierrors.IsNotFound(err) {
            // 资源已删除，清理相关资源
            if r.clusters != nil {
                delete(r.clusters, req.NamespacedName.String())
            }
            return ctrl.Result{}, nil
        }
        return ctrl.Result{}, err
    }

    // 2. 处理删除事件
    if etcdCluster.DeletionTimestamp != nil {
        return r.handleDeletion(ctx, etcdCluster, logger)
    }

    // 3. 添加 finalizer（如果不存在）
    if !controllerutil.ContainsFinalizer(etcdCluster, etcdFinalizer) {
        controllerutil.AddFinalizer(etcdCluster, etcdFinalizer)
        if err := r.Update(ctx, etcdCluster); err != nil {
            return ctrl.Result{}, err
        }
        return ctrl.Result{Requeue: true}, nil
    }

    // 4. 设置默认值和验证
    etcdCluster.SetDefaults()
    if err := r.validateClusterSpec(etcdCluster.Spec); err != nil {
        return ctrl.Result{}, err
    }

    // 5. 管理集群实例
    if r.clusters == nil {
        r.clusters = make(map[string]*cluster.Cluster)
    }

    clusterKey := req.NamespacedName.String()
    if existingCluster, exists := r.clusters[clusterKey]; exists {
        // 更新现有集群实例
        existingCluster.Update(etcdCluster)
    } else {
        // 创建新的集群实例
        config := cluster.Config{
            ServiceAccount: "default",
            KubeCli:        r.KubeCli,
            Client:         r.Client,
            Recorder:       r.Recorder,
        }
        newCluster := cluster.New(config, etcdCluster, logger)
        r.clusters[clusterKey] = newCluster
    }

    return ctrl.Result{}, nil
}
```

### 2.2 Finalizer 机制

#### 问题：Finalizer 的作用是什么？
**解答**：Finalizer 是 Kubernetes 中用于实现自定义资源清理逻辑的机制，防止资源在自定义清理完成前被删除。

**工作机制**：
```go
const (
    etcdFinalizer = "etcd.k8s.etcd.lz/finalizer"
)

// 添加 Finalizer
if !controllerutil.ContainsFinalizer(etcdCluster, etcdFinalizer) {
    controllerutil.AddFinalizer(etcdCluster, etcdFinalizer)
    if err := r.Update(ctx, etcdCluster); err != nil {
        return ctrl.Result{}, err
    }
    return ctrl.Result{Requeue: true}, nil
}

// 处理删除时移除 Finalizer
func (r *EtcdClusterReconciler) handleDeletion(ctx context.Context, etcdCluster *etcdv1alpha1.EtcdCluster, logger logr.Logger) (ctrl.Result, error) {
    // 执行自定义清理逻辑
    if err := r.cleanupClusterResources(ctx, etcdCluster, logger); err != nil {
        return ctrl.Result{}, err
    }

    // 移除 Finalizer
    controllerutil.RemoveFinalizer(etcdCluster, etcdFinalizer)
    if err := r.Update(ctx, etcdCluster); err != nil {
        return ctrl.Result{}, err
    }

    return ctrl.Result{}, nil
}
```

### 2.3 资源所有权管理

#### 问题：Controller 如何管理集群实例？
**解答**：Controller 通过内存映射维护所有正在管理的集群实例，并在资源变化时更新或创建实例。

**实例管理**：
```go
type EtcdClusterReconciler struct {
    // ...
    clusters map[string]*cluster.Cluster  // 存储集群实例
}

// 管理集群实例
clusterKey := req.NamespacedName.String()
if existingCluster, exists := r.clusters[clusterKey]; exists {
    // 更新现有集群实例
    existingCluster.Update(etcdCluster)
} else {
    // 创建新的集群实例
    config := cluster.Config{
        ServiceAccount: "default",
        KubeCli:        r.KubeCli,
        Client:         r.Client,
        Recorder:       r.Recorder,
    }
    newCluster := cluster.New(config, etcdCluster, logger)
    r.clusters[clusterKey] = newCluster
}
```

## 3. 关键方法实现

### 3.1 资源删除处理

#### 问题：如何处理 EtcdCluster 资源的删除？
**解答**：通过 handleDeletion 方法实现三步删除流程：清理集群实例、执行自定义清理、移除 Finalizer。

**删除流程**：
```go
func (r *EtcdClusterReconciler) handleDeletion(ctx context.Context, etcdCluster *etcdv1alpha1.EtcdCluster, logger logr.Logger) (ctrl.Result, error) {
    logger.Info("Handling etcd cluster deletion")

    clusterKey := fmt.Sprintf("%s/%s", etcdCluster.Namespace, etcdCluster.Name)

    // 1. 删除集群实例
    if r.clusters != nil {
        if clusterInstance, exists := r.clusters[clusterKey]; exists {
            clusterInstance.Delete()
            delete(r.clusters, clusterKey)
        }
    }

    // 2. 主动清理相关资源
    if err := r.cleanupClusterResources(ctx, etcdCluster, logger); err != nil {
        return ctrl.Result{}, err
    }

    // 3. 移除 finalizer
    controllerutil.RemoveFinalizer(etcdCluster, etcdFinalizer)
    if err := r.Update(ctx, etcdCluster); err != nil {
        return ctrl.Result{}, err
    }

    return ctrl.Result{}, nil
}
```

### 3.2 主动资源清理

#### 问题：为什么要实现主动资源清理？
**解答**：虽然 Kubernetes 有基于 OwnerReference 的自动垃圾回收，但主动清理提供了更强的可靠性和错误处理能力。

**清理实现**：
```go
func (r *EtcdClusterReconciler) cleanupClusterResources(ctx context.Context, etcdCluster *etcdv1alpha1.EtcdCluster, logger logr.Logger) error {
    // 清理 Pods
    if err := r.deleteEtcdPods(ctx, etcdCluster, logger); err != nil {
        return fmt.Errorf("failed to delete etcd pods: %v", err)
    }

    // 清理 Services
    if err := r.deleteEtcdServices(ctx, etcdCluster, logger); err != nil {
        return fmt.Errorf("failed to delete etcd services: %v", err)
    }

    // 清理 PVCs
    if err := r.deleteEtcdPVCs(ctx, etcdCluster, logger); err != nil {
        return fmt.Errorf("failed to delete etcd PVCs: %v", err)
    }

    return nil
}
```

## 4. RBAC 权限配置

### 4.1 权限注解
```go
// +kubebuilder:rbac:groups=k8s.etcd.lz,resources=etcdclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.etcd.lz,resources=etcdclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.etcd.lz,resources=etcdclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
```

## 5. Controller 设置

### 5.1 SetupWithManager 方法
```go
func (r *EtcdClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&etcdv1alpha1.EtcdCluster{}).     // 监听 EtcdCluster 资源
        Owns(&corev1.Pod{}).                  // 监听所属的 Pod
        Owns(&corev1.Service{}).              // 监听所属的 Service
        Owns(&corev1.PersistentVolumeClaim{}). // 监听所属的 PVC
        Complete(r)
}
```

## 6. 最佳实践

### 6.1 错误处理
1. **资源不存在错误**：使用 `apierrors.IsNotFound()` 检查并正确处理
2. **重试机制**：通过返回 `ctrl.Result{Requeue: true}` 实现重试
3. **详细日志**：记录关键操作和错误信息

### 6.2 状态管理
1. **Finalizer 管理**：确保自定义清理逻辑执行完毕
2. **集群实例缓存**：避免重复创建集群管理实例
3. **资源清理**：实现完整的资源清理逻辑

### 6.3 性能优化
1. **批量操作**：合理使用 Kubernetes API 批量操作
2. **缓存利用**：利用 controller-runtime 的缓存机制
3. **并发控制**：通过集群映射管理并发实例
