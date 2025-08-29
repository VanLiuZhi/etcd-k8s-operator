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

#### Group-Version-Kind (GVK)
- **Group**: API 组名 (k8s.etcd.lz)
- **Version**: API 版本 (v1alpha1)
- **Kind**: 资源类型 (EtcdCluster)

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