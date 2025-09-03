# CRD 模块学习笔记

## 1. CRD 模块涉及的代码文件

### 1.1 核心文件
1. **groupversion_info.go** - API 组和版本信息定义
2. **etcdcluster_types.go** - EtcdCluster 自定义资源核心定义
3. **status.go** - 集群状态管理相关方法
4. **zz_generated.deepcopy.go** - 自动生成的 DeepCopy 方法（由代码生成工具生成）

## 2. CRD 模块核心知识点

### 2.1 API 组和版本管理 (groupversion_info.go)

#### 核心概念
- **GroupVersion**: 定义 API 组和版本信息
  - Group: "k8s.etcd.lz" - 自定义 API 组名
  - Version: "v1alpha1" - API 版本
- **SchemeBuilder**: 用于将 Go 类型注册到 Scheme
- **AddToScheme**: 将类型添加到 Scheme 的函数

#### 关键代码
```go
var (
    // GroupVersion is group version used to register these objects
    GroupVersion = schema.GroupVersion{Group: "k8s.etcd.lz", Version: "v1alpha1"}

    // SchemeBuilder is used to add go types to the GroupVersionKind scheme
    SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

    // AddToScheme adds the types in this group-version to the given scheme.
    AddToScheme = SchemeBuilder.AddToScheme
)
```

### 2.2 自定义资源定义 (etcdcluster_types.go)

#### 核心结构体

##### 1. EtcdCluster - 主资源定义
```go
type EtcdCluster struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec ClusterSpec `json:"spec,omitempty"`
    Status ClusterStatus `json:"status,omitempty"`
}
```

##### 2. ClusterSpec - 期望状态定义
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

##### 3. ClusterStatus - 观察状态定义
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

#### 重要子结构体

##### PodPolicy - Pod 策略配置
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

##### TLSPolicy - TLS 策略配置
```go
type TLSPolicy struct {
    Static *StaticTLS `json:"static,omitempty"`  // 静态 TLS 配置
}

type StaticTLS struct {
    Member *MemberSecret `json:"member,omitempty"`  // 成员证书
    OperatorSecret string `json:"operatorSecret,omitempty"` // Operator 证书
}
```

#### 集群阶段和条件类型

##### ClusterPhase - 集群阶段
```go
const (
    ClusterPhaseNone ClusterPhase = ""      // 未开始创建
    ClusterPhaseCreating = "Creating"       // 创建中
    ClusterPhaseRunning = "Running"         // 运行中
    ClusterPhaseFailed = "Failed"           // 失败
)
```

##### ClusterConditionType - 条件类型
```go
const (
    ClusterConditionAvailable ClusterConditionType = "Available"    // 可用
    ClusterConditionRecovering = "Recovering"                       // 恢复中
    ClusterConditionScaling = "Scaling"                             // 扩缩容中
    ClusterConditionUpgrading = "Upgrading"                         // 升级中
)
```

### 2.3 状态管理 (status.go)

#### 核心方法
- **SetPhase()** - 设置集群阶段
- **SetReason()** - 设置阶段原因
- **SetReadyCondition()** - 设置就绪条件
- **SetScalingUpCondition()** - 设置扩容条件
- **SetScalingDownCondition()** - 设置缩容条件
- **SetRecoveringCondition()** - 设置恢复条件
- **SetUpgradingCondition()** - 设置升级条件

#### 条件管理机制
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

## 3. CRD 模块学习重点

### 3.1 核心概念理解

#### Spec vs Status
- **Spec (期望状态)**: 用户期望的集群配置
- **Status (观察状态)**: 系统实际的集群状态
- **Controller 职责**: 不断调整实际状态向期望状态靠近

**深入理解**：
1. **声明式 API**: 用户只需声明期望状态，系统负责实现
2. **幂等性**: 多次应用相同的 Spec 应该产生相同的结果
3. **异步处理**: 复杂操作可以在后台进行，通过 Status 反馈进度
4. **状态收敛**: Controller 持续工作确保实际状态向期望状态收敛

#### Group-Version-Kind (GVK)
- **Group**: API 组名 (k8s.etcd.lz)
- **Version**: API 版本 (v1alpha1)
- **Kind**: 资源类型 (EtcdCluster)

**GVK 作用**：
1. **唯一标识**: 在 Kubernetes 中唯一标识一个资源类型
2. **API 路由**: Kubernetes API Server 根据 GVK 路由到相应的处理逻辑
3. **版本管理**: 支持 API 版本演进和向后兼容
4. **RBAC 控制**: 基于 GVK 的权限控制

### 3.2 状态管理深度解析

#### ClusterPhase 阶段管理
ClusterPhase 是集群生命周期的核心状态表示：

```go
const (
    ClusterPhaseNone ClusterPhase = ""      // 初始状态，未开始创建
    ClusterPhaseCreating = "Creating"       // 集群创建中
    ClusterPhaseRunning = "Running"         // 集群正常运行
    ClusterPhaseFailed = "Failed"           // 集群失败
)

// 状态转换示例：
// None → Creating → Running (正常创建流程)
// None → Creating → Failed (创建失败)
// Running → Failed (运行时故障)
```

**阶段管理要点**：
1. **明确性**: 每个阶段都有明确的含义和处理逻辑
2. **可追踪**: 通过 Reason 字段提供阶段转换的详细原因
3. **可观测**: 便于用户和监控系统理解集群状态

#### ClusterCondition 条件管理
ClusterCondition 提供更细粒度的状态信息：

```go
type ClusterCondition struct {
    Type ClusterConditionType `json:"type"`               // 条件类型
    Status corev1.ConditionStatus `json:"status"`          // 状态 (True/False/Unknown)
    LastUpdateTime string `json:"lastUpdateTime,omitempty"` // 最后更新时间
    LastTransitionTime string `json:"lastTransitionTime,omitempty"` // 最后转换时间
    Reason string `json:"reason,omitempty"`              // 转换原因
    Message string `json:"message,omitempty"`            // 详细信息
}
```

**条件使用场景**：
1. **Scaling**: 扩缩容操作的状态反馈
2. **Upgrading**: 升级操作的进度跟踪
3. **Recovering**: 故障恢复的操作状态
4. **Available**: 集群可用性状态

### 3.3 配置策略详解

#### PodPolicy 策略配置
PodPolicy 提供了丰富的 Pod 配置选项：

```go
type PodPolicy struct {
    Labels map[string]string `json:"labels,omitempty"`
    NodeSelector map[string]string `json:"nodeSelector,omitempty"`
    Affinity *corev1.Affinity `json:"affinity,omitempty"`
    Resources corev1.ResourceRequirements `json:"resources,omitempty"`
    Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
    EtcdEnv []corev1.EnvVar `json:"etcdEnv,omitempty"`
    PersistentVolumeClaimSpec *corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaimSpec,omitempty"`
    Annotations map[string]string `json:"annotations,omitempty"`
}
```

**配置要点**：
1. **资源管理**: 合理设置 Requests 和 Limits
2. **调度策略**: 通过 Affinity 和 NodeSelector 控制 Pod 分布
3. **存储配置**: PersistentVolumeClaimSpec 支持持久化存储
4. **环境变量**: EtcdEnv 允许自定义 etcd 进程配置

#### TLSPolicy 安全配置
TLSPolicy 提供了 etcd 集群的安全通信支持：

```go
type TLSPolicy struct {
    Static *StaticTLS `json:"static,omitempty"`
}

type StaticTLS struct {
    Member *MemberSecret `json:"member,omitempty"`
    OperatorSecret string `json:"operatorSecret,omitempty"`
}

type MemberSecret struct {
    PeerSecret string `json:"peerSecret,omitempty"`
    ServerSecret string `json:"serverSecret,omitempty"`
}
```

**安全要点**：
1. **Peer TLS**: 保护 etcd 成员间的通信
2. **Server TLS**: 保护客户端与 etcd 的通信
3. **Operator TLS**: 保护 Operator 与 etcd 集群的通信
4. **证书管理**: 通过 Kubernetes Secrets 管理证书

### 3.4 验证和默认值机制

#### 字段验证
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

#### SetDefaults 方法
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

### 3.2 关键知识点

#### 1. 资源验证和默认值
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

#### 2. Kubebuilder 注解
```go
// +kubebuilder:object:root=true          // 标记为根对象
// +kubebuilder:subresource:status        // 启用 status 子资源
// +kubebuilder:resource:shortName=etcd   // 设置短名称
// +kubebuilder:printcolumn:...           // kubectl get 输出列定义
```

#### 3. 所有权引用
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

### 3.3 学习建议

#### 1. 理解数据流向
1. 用户创建 EtcdCluster 资源 (设置 Spec)
2. Controller 监听到资源变化
3. Controller 根据 Spec 创建/管理实际集群
4. Controller 更新 Status 反映实际状态

#### 2. 掌握状态管理
- 理解 ClusterPhase 的各个阶段含义
- 掌握 ClusterCondition 的使用场景
- 学会通过条件来表达复杂的集群状态

#### 3. 关注验证机制
- 字段范围验证 (大小、格式等)
- 默认值设置逻辑
- 复杂配置的验证和清理

#### 4. 实践建议
- 尝试修改字段定义并重新生成代码
- 观察不同配置下 Status 的变化
- 理解各个注解对 CRD 行为的影响