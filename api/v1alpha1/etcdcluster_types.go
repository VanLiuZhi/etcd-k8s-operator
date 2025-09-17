/*
Copyright 2025 ETCD Operator Team.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultRepository  = "quay.io/coreos/etcd"
	DefaultEtcdVersion = "3.5.21"
)

// ClusterPhase 表示 EtcdCluster 的阶段
type ClusterPhase string

const (
	// ClusterPhaseNone 表示集群尚未开始创建
	ClusterPhaseNone ClusterPhase = ""
	// ClusterPhaseCreating 表示集群正在创建中
	ClusterPhaseCreating = "Creating"
	// ClusterPhaseRunning 表示集群正在正常运行
	ClusterPhaseRunning = "Running"
	// ClusterPhaseFailed 表示集群创建或运行失败
	ClusterPhaseFailed = "Failed"
)

// ClusterConditionType 表示集群状况的类型
type ClusterConditionType string

const (
	// ClusterConditionAvailable 表示集群可用状况
	ClusterConditionAvailable ClusterConditionType = "Available"
	// ClusterConditionRecovering 表示集群恢复状况
	ClusterConditionRecovering = "Recovering"
	// ClusterConditionScaling 表示集群扩缩容状况
	ClusterConditionScaling = "Scaling"
	// ClusterConditionUpgrading 表示集群升级状况
	ClusterConditionUpgrading = "Upgrading"
)

// PodPolicy 定义了创建 etcd 容器 Pod 的策略配置
type PodPolicy struct {
	// Labels 指定要附加到 operator 为 etcd 集群创建的 Pod 上的标签
	// "app" 和 "etcd_*" 标签保留给 etcd operator 内部使用，请勿覆盖

	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// NodeSelector 指定键值对映射。Pod 要在节点上运行，节点必须具有所有指定的键值对标签

	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// etcd Pod 的调度约束配置

	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Resources 是 etcd 容器的资源需求配置
	// 集群创建后此字段无法更新

	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Tolerations 指定 Pod 的污点容忍配置

	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// EtcdEnv 是要在 etcd 容器中设置的环境变量列表
	// 用于配置 etcd 进程。如果提供了错误的环境变量，etcd 集群将无法创建
	// 不要覆盖用于引导集群的任何标志（例如 `--initial-cluster` 标志）
	// 此字段无法更新

	// +optional
	EtcdEnv []corev1.EnvVar `json:"etcdEnv,omitempty"`

	// PersistentVolumeClaimSpec 是描述 etcd 容器 PVC 的规格
	// 此字段是可选的。如果没有 PVC 规格，etcd 容器将使用 emptyDir 作为卷

	// +optional
	PersistentVolumeClaimSpec *corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaimSpec,omitempty"`

	// Annotations 指定要附加到 operator 为 etcd 集群创建的 Pod 上的注解
	// "etcd.version" 注解保留给 etcd operator 内部使用

	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// TLSPolicy 定义了 etcd 集群的 TLS 策略
type TLSPolicy struct {
	// 静态 TLS 配置

	// +optional
	Static *StaticTLS `json:"static,omitempty"`
}

// StaticTLS 定义静态 TLS 配置
type StaticTLS struct {
	// Member 包含每个 etcd 成员 Pod 使用的 TLS 证书的 secrets

	// +optional
	Member *MemberSecret `json:"member,omitempty"`
	// OperatorSecret 包含 operator 用于与此 etcd 集群安全通信的 TLS 证书的 secret

	// +optional
	OperatorSecret string `json:"operatorSecret,omitempty"`
}

// MemberSecret 定义包含 etcd 成员使用的 TLS 证书的 secret
type MemberSecret struct {
	// PeerSecret 是包含每个 etcd 成员 Pod 用于 peer 连接的 TLS 证书的 secret

	// +optional
	PeerSecret string `json:"peerSecret,omitempty"`
	// ServerSecret 是包含每个 etcd 成员 Pod 用于服务器连接的 TLS 证书的 secret

	// +optional
	ServerSecret string `json:"serverSecret,omitempty"`
}

// IsSecurePeer 检查是否启用 peer TLS
func (tp *TLSPolicy) IsSecurePeer() bool {
	if tp == nil || tp.Static == nil || tp.Static.Member == nil {
		return false
	}
	return len(tp.Static.Member.PeerSecret) != 0
}

// IsSecureClient 检查是否启用 client TLS
func (tp *TLSPolicy) IsSecureClient() bool {
	if tp == nil || tp.Static == nil || tp.Static.Member == nil {
		return false
	}
	return len(tp.Static.Member.ServerSecret) != 0
}

// ClusterSpec defines the desired state of EtcdCluster
// ClusterSpec 定义了 EtcdCluster 的期望状态
type ClusterSpec struct {
	// Size 是 etcd 集群的期望大小
	// etcd-operator 最终会使运行中的集群大小等于期望大小
	// 大小的有效范围是 1 到 7

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=7
	// +kubebuilder:default=3
	Size int `json:"size"`

	// Repository 是托管 etcd 容器镜像的仓库名称
	// 它应该是官方发布仓库的直接克隆：
	//   https://github.com/coreos/etcd/releases
	// 这意味着它应该具有完全相同的标签和标签的相同含义
	// 默认情况下，它是 `quay.io/coreos/etcd`

	// +optional
	Repository string `json:"repository,omitempty"`

	// Version 是 etcd 集群的期望版本
	// etcd-operator 最终会使 etcd 集群版本等于期望版本
	//
	// 版本必须遵循 [semver](http://semver.org) 格式，例如 "3.5.21"
	// 仅支持 etcd 发布的版本：https://github.com/coreos/etcd/releases
	//
	// 如果未设置版本，默认为 "3.5.21"

	// +kubebuilder:validation:Pattern=^[0-9]+\.[0-9]+\.[0-9]+$
	// +kubebuilder:default="3.5.21"
	// +optional
	Version string `json:"version,omitempty"`

	// Paused 用于暂停 operator 对 etcd 集群的控制

	// +kubebuilder:default=false
	// +optional
	Paused bool `json:"paused,omitempty"`

	// Pod 定义了创建 etcd Pod 的策略
	//
	// 更新 Pod 不会对任何现有的 etcd Pod 生效

	// +optional
	Pod *PodPolicy `json:"pod,omitempty"`

	// TLS 定义了 etcd 集群的 TLS 策略

	// +optional
	TLS *TLSPolicy `json:"tls,omitempty"`
}

// ClusterCondition 表示 etcd 集群的一个当前状况
type ClusterCondition struct {
	// 集群状况的类型
	Type ClusterConditionType `json:"type"`
	// 状况的状态，可以是 True、False 或 Unknown 之一
	Status corev1.ConditionStatus `json:"status"`
	// 此状况最后一次更新的时间

	// +optional
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// 状况从一个状态转换到另一个状态的最后时间

	// +optional
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// 状况最后一次转换的原因

	// +optional
	Reason string `json:"reason,omitempty"`
	// 指示转换详细信息的人类可读消息

	// +optional
	Message string `json:"message,omitempty"`
}

// MembersStatus 表示 etcd 集群成员的状态
type MembersStatus struct {
	// Ready 是准备好服务请求的 etcd 成员
	// 成员名称与 etcd Pod 名称相同

	// +optional
	Ready []string `json:"ready,omitempty"`
	// Unready 是尚未准备好服务请求的 etcd 成员

	// +optional
	Unready []string `json:"unready,omitempty"`
}

// ClusterStatus 定义了 EtcdCluster 的观察状态
type ClusterStatus struct {
	// Phase 是集群运行阶段
	Phase ClusterPhase `json:"phase"`
	// 当前阶段的原因

	// +optional
	Reason string `json:"reason,omitempty"`

	// ControlPaused 表示 operator 暂停了对集群的控制

	// +optional
	ControlPaused bool `json:"controlPaused,omitempty"`

	// Condition 跟踪所有集群状况（如果存在）

	// +optional
	Conditions []ClusterCondition `json:"conditions,omitempty"`

	// Size 是集群的当前大小
	Size int `json:"size"`

	// ServiceName 是用于访问 etcd 节点的负载均衡服务名称

	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	// ClientPort 是 etcd 客户端访问的端口
	// 在客户端负载均衡服务和 etcd 节点上是相同的

	// +optional
	ClientPort int `json:"clientPort,omitempty"`

	// Members 是集群中的 etcd 成员
	Members MembersStatus `json:"members"`
	// CurrentVersion 是当前集群版本
	CurrentVersion string `json:"currentVersion"`
	// TargetVersion 是集群正在升级到的版本
	// 如果集群没有在升级，TargetVersion 为空

	// +optional
	TargetVersion string `json:"targetVersion"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=etcd
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Size",type="integer",JSONPath=".spec.size"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.size"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// EtcdCluster 是 etcdclusters API 的模式定义
type EtcdCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec 定义了 EtcdCluster 的期望状态
	Spec ClusterSpec `json:"spec,omitempty"`
	// Status 定义了 EtcdCluster 的观察状态
	Status ClusterStatus `json:"status,omitempty"`
}

// AsOwner 返回此 EtcdCluster 的 OwnerReference
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

// SetDefaults 清理用户传入的 spec，例如设置默认值、转换字段
func (e *EtcdCluster) SetDefaults() {
	c := &e.Spec
	if len(c.Repository) == 0 {
		c.Repository = defaultRepository
	}

	if len(c.Version) == 0 {
		c.Version = DefaultEtcdVersion
	}

	// Remove 'v' prefix if present
	if len(c.Version) > 0 && c.Version[0] == 'v' {
		c.Version = c.Version[1:]
	}

	// 添加默认的反亲和性，将 etcd Pod 分散到不同节点上
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

// +kubebuilder:object:root=true

// EtcdClusterList 包含 EtcdCluster 列表
type EtcdClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EtcdCluster{}, &EtcdClusterList{})
}
