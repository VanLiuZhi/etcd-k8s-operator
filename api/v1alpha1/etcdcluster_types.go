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

// ClusterPhase represents the phase of an EtcdCluster
type ClusterPhase string

const (
	ClusterPhaseNone     ClusterPhase = ""
	ClusterPhaseCreating              = "Creating"
	ClusterPhaseRunning               = "Running"
	ClusterPhaseFailed                = "Failed"
)

// ClusterConditionType represents the type of cluster condition
type ClusterConditionType string

const (
	ClusterConditionAvailable  ClusterConditionType = "Available"
	ClusterConditionRecovering                      = "Recovering"
	ClusterConditionScaling                         = "Scaling"
	ClusterConditionUpgrading                       = "Upgrading"
)

// PodPolicy defines the policy to create pod for the etcd container.
type PodPolicy struct {
	// Labels specifies the labels to attach to pods the operator creates for the
	// etcd cluster.
	// "app" and "etcd_*" labels are reserved for the internal use of the etcd operator.
	// Do not overwrite them.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// NodeSelector specifies a map of key-value pairs. For the pod to be eligible
	// to run on a node, the node must have each of the indicated key-value pairs as
	// labels.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// The scheduling constraints on etcd pods.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Resources is the resource requirements for the etcd container.
	// This field cannot be updated once the cluster is created.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Tolerations specifies the pod's tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// List of environment variables to set in the etcd container.
	// This is used to configure etcd process. etcd cluster cannot be created, when
	// bad environement variables are provided. Do not overwrite any flags used to
	// bootstrap the cluster (for example `--initial-cluster` flag).
	// This field cannot be updated.
	// +optional
	EtcdEnv []corev1.EnvVar `json:"etcdEnv,omitempty"`

	// PersistentVolumeClaimSpec is the spec to describe PVC for the etcd container
	// This field is optional. If no PVC spec, etcd container will use emptyDir as volume
	// +optional
	PersistentVolumeClaimSpec *corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaimSpec,omitempty"`

	// Annotations specifies the annotations to attach to pods the operator creates for the
	// etcd cluster.
	// The "etcd.version" annotation is reserved for the internal use of the etcd operator.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ClusterSpec defines the desired state of EtcdCluster
type ClusterSpec struct {
	// Size is the expected size of the etcd cluster.
	// The etcd-operator will eventually make the size of the running
	// cluster equal to the expected size.
	// The vaild range of the size is from 1 to 7.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=7
	// +kubebuilder:default=3
	Size int `json:"size"`

	// Repository is the name of the repository that hosts
	// etcd container images. It should be direct clone of the repository in official
	// release:
	//   https://github.com/coreos/etcd/releases
	// That means, it should have exact same tags and the same meaning for the tags.
	//
	// By default, it is `quay.io/coreos/etcd`.
	// +optional
	Repository string `json:"repository,omitempty"`

	// Version is the expected version of the etcd cluster.
	// The etcd-operator will eventually make the etcd cluster version
	// equal to the expected version.
	//
	// The version must follow the [semver]( http://semver.org) format, for example "3.5.21".
	// Only etcd released versions are supported: https://github.com/coreos/etcd/releases
	//
	// If version is not set, default is "3.5.21".
	// +kubebuilder:validation:Pattern=^[0-9]+\.[0-9]+\.[0-9]+$
	// +kubebuilder:default="3.5.21"
	// +optional
	Version string `json:"version,omitempty"`

	// Paused is to pause the control of the operator for the etcd cluster.
	// +kubebuilder:default=false
	// +optional
	Paused bool `json:"paused,omitempty"`

	// Pod defines the policy to create pod for the etcd pod.
	//
	// Updating Pod does not take effect on any existing etcd pods.
	// +optional
	Pod *PodPolicy `json:"pod,omitempty"`
}

// ClusterCondition represents one current condition of an etcd cluster.
type ClusterCondition struct {
	// Type of cluster condition.
	Type ClusterConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	// +optional
	LastUpdateTime string `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition.
	// +optional
	Message string `json:"message,omitempty"`
}

// MembersStatus represents the status of etcd cluster members
type MembersStatus struct {
	// Ready are the etcd members that are ready to serve requests
	// The member names are the same as the etcd pod names
	// +optional
	Ready []string `json:"ready,omitempty"`
	// Unready are the etcd members not ready to serve requests
	// +optional
	Unready []string `json:"unready,omitempty"`
}

// ClusterStatus defines the observed state of EtcdCluster
type ClusterStatus struct {
	// Phase is the cluster running phase
	Phase ClusterPhase `json:"phase"`
	// Reason for the current phase
	// +optional
	Reason string `json:"reason,omitempty"`

	// ControlPaused indicates the operator pauses the control of the cluster.
	// +optional
	ControlPaused bool `json:"controlPaused,omitempty"`

	// Condition keeps track of all cluster conditions, if they exist.
	// +optional
	Conditions []ClusterCondition `json:"conditions,omitempty"`

	// Size is the current size of the cluster
	Size int `json:"size"`

	// ServiceName is the LB service for accessing etcd nodes.
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	// ClientPort is the port for etcd client to access.
	// It's the same on client LB service and etcd nodes.
	// +optional
	ClientPort int `json:"clientPort,omitempty"`

	// Members are the etcd members in the cluster
	Members MembersStatus `json:"members"`
	// CurrentVersion is the current cluster version
	CurrentVersion string `json:"currentVersion"`
	// TargetVersion is the version the cluster upgrading to.
	// If the cluster is not upgrading, TargetVersion is empty.
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

// EtcdCluster is the Schema for the etcdclusters API
type EtcdCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

// AsOwner returns an OwnerReference for this EtcdCluster
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

// SetDefaults cleans up user passed spec, e.g. defaulting, transforming fields.
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

	// Add default anti-affinity to spread etcd pods across nodes
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

// EtcdClusterList contains a list of EtcdCluster
type EtcdClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EtcdCluster{}, &EtcdClusterList{})
}
