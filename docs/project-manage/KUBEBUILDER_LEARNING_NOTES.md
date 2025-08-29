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