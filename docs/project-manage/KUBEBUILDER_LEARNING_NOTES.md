# Kubebuilder Operator 开发学习笔记

## 1. 项目结构和启动流程

### 1.1 项目结构概览
Kubebuilder 生成的项目遵循标准结构：
```
├── api/                    # 自定义资源定义 (CRD)
│   └── v1alpha1/
│       ├── groupversion_info.go  # API 组和版本信息
│       ├── etcdcluster_types.go  # 自定义资源结构体定义
│       └── zz_generated.deepcopy.go  # 自动生成的 DeepCopy 方法
├── cmd/
│   └── main.go             # 程序入口点
├── internal/
│   └── controller/         # 控制器实现
│       └── etcdcluster_controller.go  # EtcdCluster 控制器
├── config/                 # 配置文件 (CRD, RBAC, 部署等)
└── pkg/                    # 共享包
```

### 1.2 启动流程分析

#### 程序入口: cmd/main.go
```go
// 1. 全局变量定义
var (
    scheme   = runtime.NewScheme()  // 资源序列化方案
    setupLog = ctrl.Log.WithName("setup")
)

// 2. 初始化阶段 - 注册资源类型
func init() {
    utilruntime.Must(clientgoscheme.AddToScheme(scheme))     // Kubernetes 内置资源
    utilruntime.Must(etcdv1alpha1.AddToScheme(scheme))       // 自定义资源
}

// 3. main 函数 - 启动控制器
func main() {
    // 创建控制器管理器
    mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
        Scheme: scheme,  // 使用注册了所有资源类型的 scheme
        // 其他配置...
    })
    
    // 创建并注册控制器
    if err = (&controller.EtcdClusterReconciler{
        Client:   mgr.GetClient(),
        Scheme:   mgr.GetScheme(),
        Recorder: mgr.GetEventRecorderFor("etcd-operator"),
        KubeCli:  kubeCli,
    }).SetupWithManager(mgr); err != nil {
        setupLog.Error(err, "unable to create controller", "controller", "EtcdCluster")
        os.Exit(1)
    }
    
    // 启动管理器
    setupLog.Info("starting manager")
    if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
        setupLog.Error(err, "problem running manager")
        os.Exit(1)
    }
}
```

### 1.3 核心组件关系

#### Scheme 的作用和注册流程
1. **定义阶段**: `runtime.NewScheme()` 创建空的 Scheme
2. **注册阶段**: `init()` 函数中注册所有需要的资源类型
3. **使用阶段**: Manager 和 Controller 使用这个 Scheme 进行资源序列化

#### Manager 和 Controller 的关系
- **Manager**: 控制器管理器，负责管理多个 Controller 的生命周期
- **Controller**: 具体的控制器实现，负责特定资源的协调逻辑
- **关系**: Controller 注册到 Manager，由 Manager 统一启动和管理

## 2. 核心概念详解

### 2.1 Scheme 注册机制

#### 问题：为什么需要在 init() 函数中注册 Scheme？
**解答**：init() 函数在 Go 程序启动时自动执行，用于注册资源类型到 Scheme，确保控制器能够识别和操作这些资源。

**案例**：
```go
func init() {
    // 注册 Kubernetes 内置资源
    utilruntime.Must(clientgoscheme.AddToScheme(scheme))
    // 注册自定义资源
    utilruntime.Must(etcdv1alpha1.AddToScheme(scheme))
}
```

#### 问题：utilruntime.Must 的作用是什么？
**解答**：简化错误处理，如果函数返回错误则直接 panic，避免冗长的 if err != nil 检查。

**案例**：
```go
// 不使用 Must
err := clientgoscheme.AddToScheme(scheme)
if err != nil {
    panic(err)
}

// 使用 Must
utilruntime.Must(clientgoscheme.AddToScheme(scheme))
```

### 2.2 Controller 初始化参数

#### 问题：Controller 初始化为什么要传递 Client、Scheme 等参数？
**解答**：采用依赖注入模式，便于测试、维护和扩展，每个参数都有明确职责。

**案例**：
```go
&controller.EtcdClusterReconciler{
    Client:   mgr.GetClient(),     // 缓存客户端，用于资源操作
    Scheme:   mgr.GetScheme(),     // 资源序列化方案
    Recorder: mgr.GetEventRecorderFor("etcd-operator"), // 事件记录器
    KubeCli:  kubeCli,             // 原生 Kubernetes 客户端
}
```

#### 问题：为什么 Controller 还需要 Scheme 成员变量？
**解答**：虽然框架自动处理大部分场景，但手动资源操作、自定义序列化等场景仍需要直接访问 Scheme。

**案例**：
```go
// 设置所有者引用需要 Scheme
if err := ctrl.SetControllerReference(etcdCluster, pod, r.Scheme); err != nil {
    return ctrl.Result{}, err
}
```

### 2.3 Result 和错误处理

#### 问题：ctrl.Result{} 是如何自动处理的？
**解答**：controller-runtime 根据 Result 和 error 自动决定后续行为（重试、延迟、正常结束）。

**案例**：
```go
// 正常结束，等待下次事件触发
return ctrl.Result{}, nil

// 立即重试
return ctrl.Result{Requeue: true}, nil

// 5秒后重试
return ctrl.Result{RequeueAfter: 5 * time.Second}, nil

// 返回错误，框架自动决定是否重试
return ctrl.Result{}, errors.New("something went wrong")
```

### 2.4 资源序列化过程

#### 问题：CRD 资源序列化需要手动实现吗？
**解答**：不需要，controller-runtime 自动处理序列化，开发者只需定义结构体。

**案例**：
```go
// 定义结构体即可
type EtcdCluster struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec ClusterSpec `json:"spec,omitempty"`
    Status ClusterStatus `json:"status,omitempty"`
}
// 框架自动处理 JSON <-> Go 对象转换
```

### 2.5 Manager 和 Controller 关系

#### 问题：Manager 和 Controller 是什么关系？
**解答**：Manager 是控制器管理器，负责管理多个 Controller 的生命周期，包括启动、停止、共享资源等。

**案例**：
```go
// 创建 Manager
mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
    Scheme: scheme,
})

// 将 Controller 注册到 Manager
if err = (&controller.EtcdClusterReconciler{
    Client: mgr.GetClient(),
    Scheme: mgr.GetScheme(),
}).SetupWithManager(mgr); err != nil {
    // 错误处理
}

// 启动 Manager，会同时启动所有注册的 Controller
if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
    // 错误处理
}
```

### 2.6 Reconcile 协调机制

#### 问题：Reconcile 接口是如何被注册和调用的？
**解答**：Reconcile 接口通过 SetupWithManager 方法注册到 Controller 中，当监听的资源发生变化时，controller-runtime 会自动调用 Reconcile 方法。

**案例**：
```go
// 1. 实现 Reconcile 接口
func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 处理资源协调逻辑
    return ctrl.Result{}, nil
}

// 2. 在 SetupWithManager 中注册
func (r *EtcdClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&etcdv1alpha1.EtcdCluster{}).  // 监听 EtcdCluster 资源
        Owns(&corev1.Pod{}).               // 监听关联的 Pod
        Complete(r)                        // 完成注册
}

// 3. 调用链：资源变化 → Informer → WorkQueue → Reconcile
```

#### 问题：Reconcile 的工作原理是什么？
**解答**：Reconcile 采用事件驱动模式，通过监听资源变化触发协调循环，确保资源的实际状态向期望状态收敛。

**案例**：
```go
// Reconcile 执行流程：
// 1. 用户创建/更新/删除 EtcdCluster 资源
// 2. Informer 监听到变化事件
// 3. 事件进入工作队列
// 4. Worker 从队列取出请求调用 Reconcile
// 5. Reconcile 执行业务逻辑
// 6. 根据返回结果决定是否重试

func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 获取资源
    etcdCluster := &etcdv1alpha1.EtcdCluster{}
    if err := r.Get(ctx, req.NamespacedName, etcdCluster); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // 业务逻辑处理
    // ...
    
    // 返回结果决定后续行为
    return ctrl.Result{}, nil  // 正常结束，等待下次事件
}
```

#### 问题：如何理解 Finalizer 在 Reconcile 中的作用？
**解答**：Finalizer 用于在资源删除前执行清理工作，防止资源被立即删除，确保清理逻辑执行完毕后再移除 Finalizer。

**案例**：
```go
const etcdFinalizer = "etcd.k8s.etcd.lz/finalizer"

func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    etcdCluster := &etcdv1alpha1.EtcdCluster{}
    if err := r.Get(ctx, req.NamespacedName, etcdCluster); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // 处理删除
    if etcdCluster.DeletionTimestamp != nil {
        if controllerutil.ContainsFinalizer(etcdCluster, etcdFinalizer) {
            // 执行清理逻辑
            if err := r.cleanupEtcdCluster(etcdCluster); err != nil {
                return ctrl.Result{}, err
            }
            
            // 移除 Finalizer
            controllerutil.RemoveFinalizer(etcdCluster, etcdFinalizer)
            if err := r.Update(ctx, etcdCluster); err != nil {
                return ctrl.Result{}, err
            }
        }
        return ctrl.Result{}, nil
    }
    
    // 添加 Finalizer（如果不存在）
    if !controllerutil.ContainsFinalizer(etcdCluster, etcdFinalizer) {
        controllerutil.AddFinalizer(etcdCluster, etcdFinalizer)
        if err := r.Update(ctx, etcdCluster); err != nil {
            return ctrl.Result{}, err
        }
    }
    
    // 正常处理逻辑
    // ...
    
    return ctrl.Result{}, nil
}
```

### 2.8 Reconcile 返回值机制

#### 问题：Reconcile 的返回值有哪些类型？分别在什么场景下使用？
**解答**：Reconcile 返回 `ctrl.Result` 和 `error`，用于控制协调循环的后续行为。不同的返回值决定了 controller-runtime 如何处理后续流程。

**案例**：
```go
// 1. 正常处理完成 - 等待下次事件触发
return ctrl.Result{}, nil

// 2. 立即重新排队 - 需要连续处理多个步骤
return ctrl.Result{Requeue: true}, nil

// 3. 延时重新排队 - 等待异步操作完成
return ctrl.Result{RequeueAfter: 5 * time.Second}, nil

// 4. 返回错误 - 让框架决定是否重试
return ctrl.Result{}, errors.New("something went wrong")

// 5. 返回错误并立即重试
return ctrl.Result{Requeue: true}, errors.New("critical error")
```

#### 问题：什么是"重新排队"？它与事件触发有什么区别？
**解答**："重新排队"是将当前正在处理的资源请求重新放入工作队列，而不是等待新的事件触发。事件触发是由外部资源变化引起的，而重新排队是由 Reconcile 内部逻辑决定的。

**案例对比**：
```go
// 事件触发场景：
// 1. 用户执行: kubectl patch etcdcluster my-cluster --patch '{"spec":{"size":5}}'
// 2. Kubernetes API Server 更新资源
// 3. Informer 监听到变化事件
// 4. Controller 将请求放入工作队列
// 5. Worker 从队列取出请求调用 Reconcile

// 重新排队场景：
func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    etcdCluster := &etcdv1alpha1.EtcdCluster{}
    r.Get(ctx, req.NamespacedName, etcdCluster)
    
    // 多步骤处理示例：
    
    // 步骤1：添加 Finalizer
    if !controllerutil.ContainsFinalizer(etcdCluster, etcdFinalizer) {
        controllerutil.AddFinalizer(etcdCluster, etcdFinalizer)
        r.Update(ctx, etcdCluster)
        // 重新排队处理下一步，确保使用最新的资源状态
        return ctrl.Result{Requeue: true}, nil
    }
    
    // 步骤2：创建 Service
    if !r.serviceExists(etcdCluster) {
        r.createService(etcdCluster)
        // 重新排队等待 Service 创建完成
        return ctrl.Result{Requeue: true}, nil
    }
    
    // 步骤3：创建 Seed Pod
    if !r.seedPodExists(etcdCluster) {
        r.createSeedPod(etcdCluster)
        // 延时重新排队，给 Pod 启动时间
        return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
    }
    
    // 所有步骤完成
    return ctrl.Result{}, nil
}
```

#### 问题：为什么需要重新排队而不是连续执行所有步骤？
**解答**：重新排队确保每次处理都基于最新的资源状态，避免竞态条件和状态不一致问题。每次 Reconcile 调用都会重新从 API Server 获取最新资源状态。

**案例说明**：
```go
// 不重新排队的问题示例：
func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    etcdCluster := &etcdv1alpha1.EtcdCluster{}
    r.Get(ctx, req.NamespacedName, etcdCluster)
    
    // 步骤1：添加 Finalizer
    if !controllerutil.ContainsFinalizer(etcdCluster, etcdFinalizer) {
        controllerutil.AddFinalizer(etcdCluster, etcdFinalizer)
        r.Update(ctx, etcdCluster)
        // 此时内存中的 etcdCluster 对象还没有 Finalizer！
        // 因为 Update() 只更新了 API Server，不更新内存对象
    }
    
    // 步骤2：创建 Service（可能会失败）
    // controllerutil.ContainsFinalizer(etcdCluster, etcdFinalizer) 可能还是 false
    // 因为我们使用的是过期的内存对象副本
    
    return ctrl.Result{}, nil
}

// 正确的做法（重新排队）：
func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    etcdCluster := &etcdv1alpha1.EtcdCluster{}
    r.Get(ctx, req.NamespacedName, etcdCluster)
    
    // 步骤1：添加 Finalizer
    if !controllerutil.ContainsFinalizer(etcdCluster, etcdFinalizer) {
        controllerutil.AddFinalizer(etcdCluster, etcdFinalizer)
        r.Update(ctx, etcdCluster)
        // 重新排队，下次调用时会重新从 API Server 获取最新状态
        return ctrl.Result{Requeue: true}, nil
    }
    
    // 第二次调用时：
    // 1. 重新调用 r.Get() 获取最新资源
    // 2. 此时资源已经包含 Finalizer
    // 3. 可以安全地执行下一步逻辑
    
    // 步骤2：创建 Service
    if !r.serviceExists(etcdCluster) {
        r.createService(etcdCluster)
        return ctrl.Result{Requeue: true}, nil
    }
    
    return ctrl.Result{}, nil
}
```

#### 问题：controller-runtime 如何处理不同的返回值？
**解答**：controller-runtime 根据返回值决定后续行为，确保系统的可靠性和正确性。

**处理逻辑**：
```go
// 简化的处理逻辑
func processResult(result ctrl.Result, err error) {
    switch {
    case err != nil:
        // 有错误，记录并决定是否重试
        if isRecoverableError(err) {
            // 可恢复错误，重新排队处理
            requeue()
        } else {
            // 不可恢复错误，记录并停止重试
            logError(err)
        }
    case result.Requeue:
        // 明确要求重新排队
        requeue()
    case result.RequeueAfter > 0:
        // 延时重新排队
        requeueAfter(result.RequeueAfter)
    default:
        // 正常结束，等待下次事件触发
        waitForNextEvent()
    }
}

// 实际使用场景示例：
func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    etcdCluster := &etcdv1alpha1.EtcdCluster{}
    if err := r.Get(ctx, req.NamespacedName, etcdCluster); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // 场景1：正常处理
    if etcdCluster.Status.Phase == etcdv1alpha1.ClusterPhaseRunning {
        return ctrl.Result{}, nil  // 等待下次事件
    }
    
    // 场景2：需要连续处理多个步骤
    if etcdCluster.Status.Phase == etcdv1alpha1.ClusterPhaseCreating {
        if !r.allPodsReady(etcdCluster) {
            return ctrl.Result{RequeueAfter: 10 * time.Second}, nil  // 等待 Pod 就绪
        }
        
        etcdCluster.Status.Phase = etcdv1alpha1.ClusterPhaseRunning
        if err := r.Status().Update(ctx, etcdCluster); err != nil {
            return ctrl.Result{}, err  // 返回错误让框架处理重试
        }
        
        return ctrl.Result{}, nil  // 处理完成
    }
    
    // 场景3：可恢复错误
    if err := r.ensureResources(etcdCluster); err != nil {
        r.Recorder.Event(etcdCluster, "Warning", "ProcessingFailed", err.Error())
        return ctrl.Result{}, err  // 让框架决定是否重试
    }
    
    return ctrl.Result{}, nil
}
```

### 2.7 OwnerReference 机制

#### 问题：OwnerReference 是什么？它是如何工作的？
**解答**：OwnerReference 是 Kubernetes 原生的垃圾回收机制，用于建立资源间的父子关系。当父资源被删除时，Kubernetes 会自动删除所有 OwnerReference 指向该父资源的子资源。

**案例**：
```go
// 1. 创建父资源引用
func (c *EtcdCluster) AsOwner() metav1.OwnerReference {
    trueVar := true
    return metav1.OwnerReference{
        APIVersion: GroupVersion.String(),
        Kind:       "EtcdCluster",
        Name:       c.Name,
        UID:        c.UID,
        Controller: &trueVar,
    }
}

// 2. 在创建子资源时设置 OwnerReference
func NewEtcdPod(m *etcd.Member, initialCluster []string, clusterName, state, token string, cluster *etcdv1alpha1.EtcdCluster, owner metav1.OwnerReference) *corev1.Pod {
    pod := newEtcdPod(m, initialCluster, clusterName, state, token, cluster)
    applyPodPolicy(clusterName, pod, cluster.Spec.Pod)
    addOwnerRefToObject(pod, owner)  // 设置 OwnerReference
    return pod
}

// 3. 添加 OwnerReference 到对象
func addOwnerRefToObject(o metav1.Object, r metav1.OwnerReference) {
    o.SetOwnerReferences(append(o.GetOwnerReferences(), r))
}

// 4. 工作流程：
// - 创建 EtcdCluster
// - 创建 Pod 时设置 OwnerReference 指向 EtcdCluster
// - 删除 EtcdCluster 时，Kubernetes 自动删除所有相关 Pod
```

#### 问题：OwnerReference 和 Finalizer 有什么区别？
**解答**：OwnerReference 是 Kubernetes 自动处理的父子资源关联和垃圾回收机制，而 Finalizer 是开发者控制的资源删除前的钩子机制，用于执行自定义清理逻辑。

**案例对比**：
```go
// OwnerReference - 自动垃圾回收
// Pod 的 OwnerReference 指向 EtcdCluster
// 当 EtcdCluster 被删除时，Kubernetes 自动删除所有相关 Pod

// Finalizer - 自定义清理逻辑
// 在 EtcdCluster 上设置 Finalizer
// 删除 EtcdCluster 时，先执行自定义清理逻辑，再移除 Finalizer
// 最后 Kubernetes 才会处理 OwnerReference 的自动清理

// 结合使用：
func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    etcdCluster := &etcdv1alpha1.EtcdCluster{}
    if err := r.Get(ctx, req.NamespacedName, etcdCluster); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    
    // 处理删除 - 使用 Finalizer 执行自定义清理
    if etcdCluster.DeletionTimestamp != nil {
        if controllerutil.ContainsFinalizer(etcdCluster, etcdFinalizer) {
            // 自定义清理逻辑（如备份数据、通知外部系统等）
            if err := r.customCleanup(etcdCluster); err != nil {
                return ctrl.Result{}, err
            }
            
            // 移除 Finalizer，允许 Kubernetes 进行自动垃圾回收
            controllerutil.RemoveFinalizer(etcdCluster, etcdFinalizer)
            if err := r.Update(ctx, etcdCluster); err != nil {
                return ctrl.Result{}, err
            }
        }
        return ctrl.Result{}, nil
    }
    
    // 添加 Finalizer
    if !controllerutil.ContainsFinalizer(etcdCluster, etcdFinalizer) {
        controllerutil.AddFinalizer(etcdCluster, etcdFinalizer)
        if err := r.Update(ctx, etcdCluster); err != nil {
            return ctrl.Result{}, err
        }
    }
    
    // 创建子资源时设置 OwnerReference
    pod := &corev1.Pod{...}
    if err := ctrl.SetControllerReference(etcdCluster, pod, r.Scheme); err != nil {
        return ctrl.Result{}, err
    }
    
    return ctrl.Result{}, nil
}
```

### 2.9 Cluster.New 执行流程

#### 问题：cluster.New() 函数执行流程是什么样的？
**解答**：cluster.New() 函数是集群管理模块的核心入口，它初始化一个 Cluster 结构体并启动后台协程来管理集群。执行流程包括初始化、启动后台协程、调用 setup() 方法和 run() 方法。

**执行流程**：
```go
// 1. 初始化 Cluster 结构体
func New(config Config, cl *etcdv1alpha1.EtcdCluster, logger logr.Logger) *Cluster {
    c := &Cluster{
        logger:  logger.WithValues("cluster-name", cl.Name),
        config:  config,
        cluster: cl,
        eventCh: make(chan *clusterEvent, 100),  // 事件通道
        stopCh:  make(chan struct{}),            // 停止通道
        status:  *cl.Status.DeepCopy(),          // 复制状态
    }

    // 2. 启动集群管理协程
    go func() {
        // 3. 调用 setup() 方法进行集群初始化
        if err := c.setup(); err != nil {
            // 错误处理，设置集群为失败状态
            c.logger.Error(err, "cluster failed to setup")
            if c.status.Phase != etcdv1alpha1.ClusterPhaseFailed {
                c.status.SetReason(err.Error())
                c.status.SetPhase(etcdv1alpha1.ClusterPhaseFailed)
                c.updateCRStatus()
            }
            return
        }
        // 4. 调用 run() 方法启动主运行循环
        c.run()
    }()

    return c
}

// 3. setup() 方法根据集群状态执行不同逻辑
func (c *Cluster) setup() error {
    var shouldCreateCluster bool
    switch c.status.Phase {
    case etcdv1alpha1.ClusterPhaseNone:
        // 新集群，需要创建
        shouldCreateCluster = true
    case etcdv1alpha1.ClusterPhaseCreating:
        // 从创建状态恢复
        return c.recoverFromCreating()
    case etcdv1alpha1.ClusterPhaseRunning:
        // 从运行状态恢复
        return c.recoverFromRunning()
    default:
        return fmt.Errorf("unexpected cluster phase: %s", c.status.Phase)
    }

    if shouldCreateCluster {
        return c.create()  // 创建新集群
    }
    return nil
}

// 4. run() 方法启动主运行循环
func (c *Cluster) run() {
    // 1. 设置集群状态为运行中
    c.status.SetPhase(etcdv1alpha1.ClusterPhaseRunning)
    c.updateCRStatus()

    c.logger.Info("start running cluster")

    var rerr error
    for {
        select {
        case <-c.stopCh:
            // 收到停止信号，退出循环
            return
        case event := <-c.eventCh:
            // 处理事件（如集群更新）
            switch event.typ {
            case eventModifyCluster:
                if !isSpecEqual(event.cluster.Spec, c.cluster.Spec) {
                    c.cluster = event.cluster  // 更新集群规格
                    c.logger.Info("cluster spec updated")
                }
            }
        case <-time.After(reconcileInterval):
            // 定期协调循环（每5秒）
            start := time.Now()

            // 1. 轮询Pod状态
            running, pending, err := c.pollPods()
            if err != nil {
                c.logger.Error(err, "failed to poll pods")
                continue
            }

            // 2. 处理pending状态的Pod
            if len(pending) > 0 {
                c.logger.Info("skip reconciliation: pods are pending")
                continue
            }

            // 3. 如果没有运行中的Pod，记录日志
            if len(running) == 0 {
                c.logger.Info("all etcd pods are dead")
                break
            }

            // 4. 更新成员信息
            if rerr != nil || c.members == nil {
                rerr = c.updateMembers(podsToMemberSet(running, c.isSecureClient()))
                if rerr != nil {
                    c.logger.Error(rerr, "failed to update members")
                    break
                }
            }

            // 5. 执行协调逻辑
            rerr = c.reconcile(running)
            if rerr != nil {
                c.logger.Error(rerr, "failed to reconcile")
                break
            }

            // 6. 更新成员状态和CR状态
            c.updateMemberStatus(running)
            c.updateCRStatus()

            c.logger.V(1).Info("reconcile completed", "duration", time.Since(start))
        }

        // 错误处理
        if rerr != nil {
            if etcd.IsFatalError(rerr) {
                c.status.SetReason(rerr.Error())
                c.logger.Error(rerr, "cluster failed")
                c.reportFailedStatus()
                return
            }
        }
    }
}
```

#### 问题：cluster.New() 中启动的后台协程负责什么？
**解答**：后台协程负责管理集群的整个生命周期，包括初始化、运行协调循环、处理事件和错误恢复。它是一个持续运行的独立线程，与 controller 的 Reconcile 循环并行工作。

**职责**：
1. **初始化**：调用 setup() 方法根据集群当前状态进行相应处理
2. **运行管理**：调用 run() 方法启动主协调循环
3. **事件处理**：监听和处理集群更新事件
4. **状态监控**：定期检查Pod状态并更新集群状态
5. **协调控制**：执行集群成员管理、扩缩容等操作
6. **错误恢复**：处理运行时错误并进行恢复
7. **生命周期管理**：响应停止信号，优雅关闭

#### 问题：为什么 cluster.New() 要启动一个独立的后台协程？
**解答**：启动独立后台协程的主要原因是为了实现持续的集群管理和监控，而不需要依赖 controller 的 Reconcile 事件驱动模型。这样可以提供更主动的管理能力。

**优势**：
1. **主动管理**：不需要等待Kubernetes事件就能主动进行协调
2. **持续监控**：可以定期检查集群状态，及时发现问题
3. **性能优化**：避免频繁触发Reconcile循环，减少API服务器压力
4. **异步处理**：长时间运行的操作不会阻塞controller工作队列
5. **精细控制**：可以实现更复杂的协调逻辑和状态机

#### 问题：Cluster 结构体中的成员变量有什么作用？
**解答**：Cluster 结构体的成员变量用于维护集群的完整状态和管理所需的所有信息。

**核心成员**：
```go
type Cluster struct {
    logger logr.Logger           // 日志记录器
    config Config               // 配置信息（包括各种客户端）
    cluster *etcdv1alpha1.EtcdCluster  // 集群CR对象
    
    // 集群的内存状态
    status etcdv1alpha1.ClusterStatus  // 集群状态副本
    
    eventCh chan *clusterEvent   // 事件通道
    stopCh  chan struct{}        // 停止信号通道
    
    // etcd集群成员集合
    members etcd.MemberSet       // 成员信息
    
    tlsConfig *tls.Config        // TLS配置（暂未使用）
}
```

#### 问题：cluster.New() 中 eventCh 和 stopCh 的作用是什么？
**解答**：eventCh 和 stopCh 是用于协程间通信的通道，实现事件传递和生命周期管理。

**eventCh**：
- 类型：`chan *clusterEvent`
- 作用：传递集群事件（如集群更新）
- 缓冲：100个事件的缓冲区
- 使用：run() 方法中通过 select 监听事件并处理

**stopCh**：
- 类型：`chan struct{}`
- 作用：传递停止信号，用于优雅关闭
- 使用：Delete() 方法中关闭通道，run() 方法监听关闭信号

**示例**：
```go
// 发送事件
func (c *Cluster) Update(cl *etcdv1alpha1.EtcdCluster) {
    c.send(&clusterEvent{
        typ:     eventModifyCluster,
        cluster: cl,
    })
}

// 优雅关闭
func (c *Cluster) Delete() {
    c.logger.Info("cluster is deleted by user")
    close(c.stopCh)  // 发送停止信号
}

// 协程中监听信号
func (c *Cluster) run() {
    for {
        select {
        case <-c.stopCh:
            return  // 收到停止信号，退出协程
        case event := <-c.eventCh:
            // 处理事件
        }
    }
}
```