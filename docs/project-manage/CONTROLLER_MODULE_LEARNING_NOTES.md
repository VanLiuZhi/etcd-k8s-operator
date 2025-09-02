# Controller 模块学习笔记

## 1. Controller 模块涉及的代码文件

### 1.1 核心文件
1. **internal/controller/etcdcluster_controller.go** - EtcdCluster 控制器核心实现

### 1.2 依赖模块
1. **api/v1alpha1/** - CRD 定义
2. **pkg/cluster/** - 集群管理逻辑
3. **pkg/k8s/** - Kubernetes 资源操作
4. **pkg/etcd/** - etcd 客户端操作

## 2. Controller 模块执行流程

### 2.1 控制器启动流程
1. **main.go 中注册控制器**:
   ```go
   if err = (&controller.EtcdClusterReconciler{
       Client:   mgr.GetClient(),
       Scheme:   mgr.GetScheme(),
       Recorder: mgr.GetEventRecorderFor("etcd-operator"),
       KubeCli:  kubeCli,
   }).SetupWithManager(mgr); err != nil {
       // 错误处理
   }
   ```

2. **SetupWithManager 配置监听**:
   ```go
   func (r *EtcdClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
       return ctrl.NewControllerManagedBy(mgr).
           For(&etcdv1alpha1.EtcdCluster{}).  // 监听 EtcdCluster 资源
           Owns(&corev1.Pod{}).               // 监听 Pod 变化
           Owns(&corev1.Service{}).           // 监听 Service 变化
           Owns(&corev1.PersistentVolumeClaim{}). // 监听 PVC 变化
           Complete(r)
   }
   ```

### 2.2 Reconcile 协调循环执行流程

#### 1. 资源获取阶段
```go
// 获取 EtcdCluster 实例
etcdCluster := &etcdv1alpha1.EtcdCluster{}
if err := r.Get(ctx, req.NamespacedName, etcdCluster); err != nil {
    if apierrors.IsNotFound(err) {
        // 资源已删除，清理本地缓存
        if r.clusters != nil {
            delete(r.clusters, req.NamespacedName.String())
        }
        return ctrl.Result{}, nil
    }
    return ctrl.Result{}, err
}
```

#### 2. 删除处理阶段
```go
// 处理删除
if etcdCluster.DeletionTimestamp != nil {
    return r.handleDeletion(ctx, etcdCluster, logger)
}
```

#### 3. Finalizer 添加阶段
```go
// 添加 finalizer（如果不存在）
if !controllerutil.ContainsFinalizer(etcdCluster, etcdFinalizer) {
    controllerutil.AddFinalizer(etcdCluster, etcdFinalizer)
    if err := r.Update(ctx, etcdCluster); err != nil {
        return ctrl.Result{}, err
    }
    return ctrl.Result{Requeue: true}, nil
}
```

#### 4. 集群管理阶段
```go
// 设置默认值
etcdCluster.SetDefaults()

// 验证集群规格
if err := r.validateClusterSpec(etcdCluster.Spec); err != nil {
    return ctrl.Result{}, err
}

// 检查是否已存在集群实例
clusterKey := req.NamespacedName.String()
if existingCluster, exists := r.clusters[clusterKey]; exists {
    // 更新现有集群
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

### 2.3 删除处理流程 (handleDeletion)

#### 1. 集群实例清理
```go
// 删除集群实例
if r.clusters != nil {
    if clusterInstance, exists := r.clusters[clusterKey]; exists {
        clusterInstance.Delete()
        delete(r.clusters, clusterKey)
    }
}
```

#### 2. Finalizer 移除
```go
// 移除 finalizer
controllerutil.RemoveFinalizer(etcdCluster, etcdFinalizer)
if err := r.Update(ctx, etcdCluster); err != nil {
    return ctrl.Result{}, err
}
```

## 3. Controller 模块核心概念

### 3.1 Reconcile 模式
Controller 采用事件驱动的协调模式：
1. **监听资源变化**: 监听 EtcdCluster 及其相关资源的变化
2. **触发协调循环**: 资源变化时触发 Reconcile 函数
3. **状态同步**: 将实际状态调整到期望状态
4. **结果返回**: 返回处理结果和重试策略

### 3.2 Finalizer 机制
Finalizer 用于资源删除前的清理工作：
1. **添加 Finalizer**: 在资源创建时添加自定义 finalizer
2. **删除拦截**: 删除资源时 Kubernetes 会设置 DeletionTimestamp
3. **清理工作**: Controller 处理清理逻辑
4. **移除 Finalizer**: 清理完成后移除 finalizer，资源真正删除

### 3.3 资源所有权管理
通过 `Owns()` 方法建立资源所有权关系：
1. **Pod**: EtcdCluster 拥有其创建的 Pod
2. **Service**: EtcdCluster 拥有其创建的 Service
3. **PVC**: EtcdCluster 拥有其创建的 PersistentVolumeClaim

### 3.4 本地缓存管理
Controller 维护本地集群实例缓存：
```go
// clusters 存储正在管理的集群实例
clusters map[string]*cluster.Cluster
```

### 3.5 事件记录机制
使用 EventRecorder 记录重要事件：
```go
Recorder record.EventRecorder
```

## 4. 关键代码分析

### 4.1 Reconcile 函数结构
```go
func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. 日志和上下文设置
    logger := log.FromContext(ctx).WithValues("etcdcluster", req.NamespacedName)
    
    // 2. 资源获取和错误处理
    // 3. 删除处理
    // 4. Finalizer 管理
    // 5. 集群实例管理
    // 6. 返回结果
}
```

### 4.2 集群实例生命周期管理
```go
// 创建新集群实例
newCluster := cluster.New(config, etcdCluster, logger)

// 更新现有集群实例
existingCluster.Update(etcdCluster)

// 删除集群实例
clusterInstance.Delete()
```

### 4.3 RBAC 权限配置
通过注解定义控制器所需权限：
```go
// +kubebuilder:rbac:groups=k8s.etcd.lz,resources=etcdclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.etcd.lz,resources=etcdclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.etcd.lz,resources=etcdclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
```

## 5. 学习重点

### 5.1 理解的核心概念
1. **Reconcile 模式**: 事件驱动的状态协调
2. **Finalizer 机制**: 资源删除前的清理工作
3. **资源所有权**: OwnerReference 和垃圾回收
4. **本地缓存**: 集群实例的生命周期管理

### 5.2 关键执行流程
1. **资源监听和事件触发**
2. **资源获取和验证**
3. **Finalizer 管理**
4. **集群实例创建/更新**
5. **删除处理和清理**

### 5.3 实践建议
1. **跟踪调试**: 观察 Reconcile 函数的执行流程
2. **日志分析**: 理解各阶段的日志输出
3. **事件查看**: 使用 `kubectl describe` 查看事件记录
4. **状态监控**: 观察 EtcdCluster 状态变化