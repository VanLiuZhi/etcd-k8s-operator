# Kubebuilder Operator 开发学习笔记

## 1. 项目结构和启动流程

### 1.1 项目结构概览
Kubebuilder 生成的项目遵循标准结构：
```
├── api/                    # 自定义资源定义 (CRD)
│   └── v1alpha1/
│       ├── groupversion_info.go  # API 组和版本信息
│       ├── etcdcluster_types.go  # 自定义资源结构体定义
│       ├── status.go          # 集群状态管理相关方法
│       └── zz_generated.deepcopy.go  # 自动生成的 DeepCopy 方法
├── cmd/
│   └── main.go             # 程序入口点
├── internal/
│   └── controller/         # 控制器实现
│       └── etcdcluster_controller.go  # EtcdCluster 控制器
├── config/                 # 配置文件 (CRD, RBAC, 部署等)
└── pkg/                    # 共享包
    ├── cluster/            # 集群管理逻辑
    ├── k8s/               # Kubernetes 资源操作
    └── etcd/              # etcd 客户端操作
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

## 2. CRD 模块详解

### 2.1 核心文件
1. **groupversion_info.go** - API 组和版本信息定义
2. **etcdcluster_types.go** - EtcdCluster 自定义资源核心定义
3. **status.go** - 集群状态管理相关方法
4. **zz_generated.deepcopy.go** - 自动生成的 DeepCopy 方法

### 2.2 核心结构体定义

#### 2.2.1 EtcdCluster 主资源
```go
type EtcdCluster struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec ClusterSpec `json:"spec,omitempty"`
    Status ClusterStatus `json:"status,omitempty"`
}
```

#### 2.2.2 ClusterSpec 期望状态
```go
type ClusterSpec struct {
    Size int `json:"size"`           // 集群大小 (1-7)
    Repository string `json:"repository,omitempty"`  // 镜像仓库
    Version string `json:"version,omitempty"`       // etcd 版本
    Paused bool `json:"paused,omitempty"`          // 是否暂停控制
    Pod *PodPolicy `json:"pod,omitempty"`           // Pod 策略
    TLS *TLSPolicy `json:"tls,omitempty"`           // TLS 策略
}
```

#### 2.2.3 ClusterStatus 观察状态
```go
type ClusterStatus struct {
    Phase ClusterPhase `json:"phase"`              // 集群阶段
    Reason string `json:"reason,omitempty"`         // 阶段原因
    ControlPaused bool `json:"controlPaused,omitempty"` // 控制是否暂停
    Conditions []ClusterCondition `json:"conditions,omitempty"` // 集群条件
    Size int `json:"size"`                         // 当前集群大小
    ServiceName string `json:"serviceName,omitempty"`   // 服务名称
    ClientPort int `json:"clientPort,omitempty"`       // 客户端端口
    Members MembersStatus `json:"members"`            // 成员状态
    CurrentVersion string `json:"currentVersion"`     // 当前版本
    TargetVersion string `json:"targetVersion"`       // 目标版本
}
```

### 2.3 重要子结构体

#### 2.3.1 PodPolicy 策略配置
```go
type PodPolicy struct {
    Labels map[string]string `json:"labels,omitempty"`           // 标签
    NodeSelector map[string]string `json:"nodeSelector,omitempty"`   // 节点选择器
    Affinity *corev1.Affinity `json:"affinity,omitempty"`          // 亲和性
    Resources corev1.ResourceRequirements `json:"resources,omitempty"` // 资源需求
    Tolerations []corev1.Toleration `json:"tolerations,omitempty"`   // 污点容忍
    EtcdEnv []corev1.EnvVar `json:"etcdEnv,omitempty"`             // 环境变量
    PersistentVolumeClaimSpec *corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaimSpec,omitempty"` // PVC 规格
    Annotations map[string]string `json:"annotations,omitempty"`   // 注解
}
```

#### 2.3.2 TLSPolicy 安全配置
```go
type TLSPolicy struct {
    Static *StaticTLS `json:"static,omitempty"`  // 静态 TLS 配置
}

type StaticTLS struct {
    Member *MemberSecret `json:"member,omitempty"`  // 成员证书
    OperatorSecret string `json:"operatorSecret,omitempty"` // Operator 证书
}
```

### 2.4 集群阶段和条件类型

#### 2.4.1 ClusterPhase 集群阶段
```go
const (
    ClusterPhaseNone ClusterPhase = ""      // 未开始创建
    ClusterPhaseCreating = "Creating"       // 创建中
    ClusterPhaseRunning = "Running"         // 运行中
    ClusterPhaseFailed = "Failed"           // 失败
)
```

#### 2.4.2 ClusterConditionType 条件类型
```go
const (
    ClusterConditionAvailable ClusterConditionType = "Available"    // 可用
    ClusterConditionRecovering = "Recovering"                       // 恢复中
    ClusterConditionScaling = "Scaling"                             // 扩缩容中
    ClusterConditionUpgrading = "Upgrading"                         // 升级中
)
```

### 2.5 状态管理 (status.go)

#### 2.5.1 核心方法
- **SetPhase()** - 设置集群阶段
- **SetReason()** - 设置阶段原因
- **SetReadyCondition()** - 设置就绪条件
- **SetScalingUpCondition()** - 设置扩容条件
- **SetScalingDownCondition()** - 设置缩容条件
- **SetRecoveringCondition()** - 设置恢复条件
- **SetUpgradingCondition()** - 设置升级条件

#### 2.5.2 条件管理机制
```go
// 条件更新逻辑
func (cs *ClusterStatus) setClusterCondition(newCondition ClusterCondition) {
    pos, cp := getClusterCondition(cs, newCondition.Type)
    if cp != nil &&
        cp.Status == newCondition.Status && cp.Reason == newCondition.Reason {
        return  // 条件未变化，不更新
    }

    if cp != nil {
        cs.Conditions[pos] = newCondition  // 更新现有条件
    } else {
        cs.Conditions = append(cs.Conditions, newCondition)  // 添加新条件
    }
}
```

### 2.6 核心概念理解

#### 2.6.1 Spec vs Status
- **Spec (期望状态)**: 用户期望的集群配置
- **Status (观察状态)**: 系统实际的集群状态
- **Controller 职责**: 不断调整实际状态向期望状态靠近

**深入理解**：
1. **声明式 API**: 用户只需声明期望状态，系统负责实现
2. **幂等性**: 多次应用相同的 Spec 应该产生相同的结果
3. **异步处理**: 复杂操作可以在后台进行，通过 Status 反馈进度
4. **状态收敛**: Controller 持续工作确保实际状态向期望状态收敛

#### 2.6.2 Group-Version-Kind (GVK)
- **Group**: API 组名 (k8s.etcd.lz)
- **Version**: API 版本 (v1alpha1)
- **Kind**: 资源类型 (EtcdCluster)

**GVK 作用**：
1. **唯一标识**: 在 Kubernetes 中唯一标识一个资源类型
2. **API 路由**: Kubernetes API Server 根据 GVK 路由到相应的处理逻辑
3. **版本管理**: 支持 API 版本演进和向后兼容
4. **RBAC 控制**: 基于 GVK 的权限控制

### 2.7 验证和默认值机制

#### 2.7.1 字段验证
Kubebuilder 注解提供了强大的字段验证能力：

```go
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:Maximum=7
// +kubebuilder:validation:Pattern=^[0-9]+\.[0-9]+\.[0-9]+$
// +kubebuilder:default=3
Size int `json:"size"`
Version string `json:"version,omitempty"`
```

**验证类型**：
1. **数值范围验证**: Minimum/Maximum 限制数值范围
2. **格式验证**: Pattern 使用正则表达式验证格式
3. **默认值**: default 提供合理的默认值
4. **可选性**: omitempty 标记可选字段

#### 2.7.2 SetDefaults 方法
SetDefaults 方法处理复杂的默认值逻辑：

```go
func (e *EtcdCluster) SetDefaults() {
    c := &e.Spec
    // 设置默认镜像仓库和版本
    if len(c.Repository) == 0 {
        c.Repository = defaultRepository
    }
    if len(c.Version) == 0 {
        c.Version = DefaultEtcdVersion
    }

    // 移除版本号前的 'v' 前缀
    if len(c.Version) > 0 && c.Version[0] == 'v' {
        c.Version = c.Version[1:]
    }

    // 添加默认反亲和性配置
    if c.Pod != nil && c.Pod.Affinity == nil {
        c.Pod.Affinity = &corev1.Affinity{
            PodAntiAffinity: &corev1.PodAntiAffinity{
                PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
                    {
                        Weight: 100,
                        PodAffinityTerm: corev1.PodAffinityTerm{
                            LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
                                "etcd_cluster": e.Name,
                            }},
                            TopologyKey: "kubernetes.io/hostname",
                        },
                    },
                },
            },
        }
    }
}
```

### 2.8 关键知识点

#### 2.8.1 资源验证和默认值
```go
// 字段验证
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:Maximum=7
Size int `json:"size"`

// 默认值设置
// +kubebuilder:default=3
Size int `json:"size"`

// SetDefaults 方法处理复杂默认值逻辑
func (e *EtcdCluster) SetDefaults() {
    // 设置默认仓库和版本
    // 添加默认反亲和性配置
}
```

#### 2.8.2 Kubebuilder 注解
```go
// +kubebuilder:object:root=true          // 标记为根对象
// +kubebuilder:subresource:status        // 启用 status 子资源
// +kubebuilder:resource:shortName=etcd   // 设置短名称
// +kubebuilder:printcolumn:...           // kubectl get 输出列定义
```

#### 2.8.3 所有权引用
```go
// AsOwner() 方法创建所有权引用
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
```

## 3. Controller 模块详解

### 3.1 Controller 模块涉及的代码文件

#### 3.1.1 核心文件
1. **internal/controller/etcdcluster_controller.go** - EtcdCluster 控制器核心实现

#### 3.1.2 依赖模块
1. **api/v1alpha1/** - CRD 定义
2. **pkg/cluster/** - 集群管理逻辑
3. **pkg/k8s/** - Kubernetes 资源操作
4. **pkg/etcd/** - etcd 客户端操作

### 3.2 Controller 模块执行流程

#### 3.2.1 控制器启动流程
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

#### 3.2.2 Reconcile 协调循环执行流程

##### 1. 资源获取阶段
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

##### 2. 删除处理阶段
```go
// 处理删除
if etcdCluster.DeletionTimestamp != nil {
    return r.handleDeletion(ctx, etcdCluster, logger)
}
```

##### 3. Finalizer 添加阶段
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

##### 4. 集群管理阶段
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

#### 3.2.3 删除处理流程 (handleDeletion)

##### 1. 集群实例清理
```go
// 删除集群实例
if r.clusters != nil {
    if clusterInstance, exists := r.clusters[clusterKey]; exists {
        clusterInstance.Delete()
        delete(r.clusters, clusterKey)
    }
}
```

##### 2. Finalizer 移除
```go
// 移除 finalizer
controllerutil.RemoveFinalizer(etcdCluster, etcdFinalizer)
if err := r.Update(ctx, etcdCluster); err != nil {
    return ctrl.Result{}, err
}
```

### 3.3 Controller 模块核心概念

#### 3.3.1 Reconcile 模式
Controller 采用事件驱动的协调模式：
1. **监听资源变化**: 监听 EtcdCluster 及其相关资源的变化
2. **触发协调循环**: 资源变化时触发 Reconcile 函数
3. **状态同步**: 将实际状态调整到期望状态
4. **结果返回**: 返回处理结果和重试策略

#### 3.3.2 Finalizer 机制
Finalizer 用于资源删除前的清理工作：
1. **添加 Finalizer**: 在资源创建时添加自定义 finalizer
2. **删除拦截**: 删除资源时 Kubernetes 会设置 DeletionTimestamp
3. **清理工作**: Controller 处理清理逻辑
4. **移除 Finalizer**: 清理完成后移除 finalizer，资源真正删除

#### 3.3.3 资源所有权管理
通过 `Owns()` 方法建立资源所有权关系：
1. **Pod**: EtcdCluster 拥有其创建的 Pod
2. **Service**: EtcdCluster 拥有其创建的 Service
3. **PVC**: EtcdCluster 拥有其创建的 PersistentVolumeClaim

#### 3.3.4 本地缓存管理
Controller 维护本地集群实例缓存：
```go
// clusters 存储正在管理的集群实例
clusters map[string]*cluster.Cluster
```

#### 3.3.5 事件记录机制
使用 EventRecorder 记录重要事件：
```go
Recorder record.EventRecorder
```

### 3.4 核心概念详解

#### 3.4.1 Scheme 注册机制

##### 问题：为什么需要在 init() 函数中注册 Scheme？
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

##### 问题：utilruntime.Must 的作用是什么？
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

#### 3.4.2 Controller 初始化参数

##### 问题：Controller 初始化为什么要传递 Client、Scheme 等参数？
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

##### 问题：为什么 Controller 还需要 Scheme 成员变量？
**解答**：虽然框架自动处理大部分场景，但手动资源操作、自定义序列化等场景仍需要直接访问 Scheme。

**案例**：
```go
// 设置所有者引用需要 Scheme
if err := ctrl.SetControllerReference(etcdCluster, pod, r.Scheme); err != nil {
    return ctrl.Result{}, err
}
```

#### 3.4.3 Result 和错误处理

##### 问题：ctrl.Result{} 是如何自动处理的？
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

#### 3.4.4 资源序列化过程

##### 问题：CRD 资源序列化需要手动实现吗？
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

#### 3.4.5 Manager 和 Controller 关系

##### 问题：Manager 和 Controller 是什么关系？
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

## 4. Operator 学习路径和模块概览

### 4.1 推荐学习顺序
1. **CRD 定义模块** (`api/v1alpha1/`) - 理解自定义资源结构
2. **Controller 核心模块** (`internal/controller/`) - 理解协调循环机制
3. **Cluster 管理模块** (`pkg/cluster/`) - 理解集群生命周期管理
4. **Kubernetes 资源管理模块** (`pkg/k8s/`) - 理解 Pod/Service/PVC 操作
5. **Etcd 客户端模块** (`pkg/etcd/`) - 理解与 etcd 集群交互

### 4.2 下一步建议
CRD 定义模块学习已完成，Controller 核心模块学习进行中，建议下一步学习 **Cluster 管理模块** (`pkg/cluster/`)，因为：
- Cluster 模块是 Controller 的核心依赖
- 理解了 Cluster 模块才能理解具体的集群管理逻辑
- Cluster 模块相对独立，但与 Controller 紧密协作
- 需要结合 Controller 知识理解 Cluster 的实现

### 4.3 需要掌握的核心模块

#### 4.3.1 必须掌握的模块

##### 1. CRD 定义模块 (`api/v1alpha1/`)
- **etcdcluster_types.go**: EtcdCluster 自定义资源定义
- **status.go**: 集群状态管理
- **groupversion_info.go**: API 组和版本信息

**核心概念**:
- ClusterSpec (期望状态)
- ClusterStatus (观察状态)
- ClusterPhase (集群阶段)
- ClusterCondition (集群条件)

##### 2. Controller 模块 (`internal/controller/`)
- **etcdcluster_controller.go**: EtcdCluster 控制器实现

**核心概念**:
- Reconcile 协调循环
- 事件处理机制
- Finalizer 机制
- 资源监听和所有权管理

##### 3. Cluster 管理模块 (`pkg/cluster/`)
- **cluster.go**: 集群核心管理逻辑
- **reconcile.go**: 集群协调逻辑（扩缩容、成员管理）

**核心概念**:
- 集群生命周期管理
- 成员管理（添加、移除、更新）
- 状态同步机制
- 协调循环实现

##### 4. Kubernetes 资源管理模块 (`pkg/k8s/`)
- **pod.go**: Pod 创建和管理
- **service.go**: Service 创建和管理
- **pvc.go**: PVC 创建和管理
- **events.go**: 事件记录

**核心概念**:
- Pod 模板和配置
- Service 类型和端口映射
- PVC 持久化存储
- Kubernetes API 操作

##### 5. Etcd 客户端模块 (`pkg/etcd/`)
- **client.go**: etcd 客户端操作
- **member.go**: etcd 成员管理
- **errors.go**: etcd 错误处理

**核心概念**:
- etcd 成员管理 API
- 集群健康检查
- 成员添加/移除操作
- TLS 安全连接

#### 4.3.2 其他模块

##### 6. 配置和部署模块 (`config/`)
- CRD 定义和部署
- RBAC 权限配置
- 部署清单

##### 7. 测试模块 (`test/`)
- 单元测试
- 集成测试
- E2E 测试

