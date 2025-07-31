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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EtcdClusterPhase represents the phase of an EtcdCluster
type EtcdClusterPhase string

const (
	// EtcdClusterPhaseCreating indicates the cluster is being created
	EtcdClusterPhaseCreating EtcdClusterPhase = "Creating"
	// EtcdClusterPhaseRunning indicates the cluster is running normally
	EtcdClusterPhaseRunning EtcdClusterPhase = "Running"
	// EtcdClusterPhaseScaling indicates the cluster is scaling
	EtcdClusterPhaseScaling EtcdClusterPhase = "Scaling"
	// EtcdClusterPhaseUpgrading indicates the cluster is upgrading
	EtcdClusterPhaseUpgrading EtcdClusterPhase = "Upgrading"
	// EtcdClusterPhaseFailed indicates the cluster has failed
	EtcdClusterPhaseFailed EtcdClusterPhase = "Failed"
	// EtcdClusterPhaseDeleting indicates the cluster is being deleted
	EtcdClusterPhaseDeleting EtcdClusterPhase = "Deleting"
	// EtcdClusterPhaseStopped indicates the cluster is stopped (size=0)
	EtcdClusterPhaseStopped EtcdClusterPhase = "Stopped"
)

// EtcdStorageSpec defines the storage configuration for etcd
type EtcdStorageSpec struct {
	// StorageClassName is the name of the StorageClass to use for etcd data
	// +kubebuilder:validation:Optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// Size is the size of the storage volume
	// +kubebuilder:default="10Gi"
	Size resource.Quantity `json:"size,omitempty"`

	// VolumeClaimTemplate allows customizing the PVC template
	// +kubebuilder:validation:Optional
	VolumeClaimTemplate *corev1.PersistentVolumeClaimTemplate `json:"volumeClaimTemplate,omitempty"`
}

// EtcdTLSSpec defines TLS configuration for etcd
type EtcdTLSSpec struct {
	// Enabled indicates whether TLS is enabled
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// ClientTLSEnabled indicates whether client TLS is enabled
	// +kubebuilder:default=true
	ClientTLSEnabled bool `json:"clientTLSEnabled,omitempty"`

	// PeerTLSEnabled indicates whether peer TLS is enabled
	// +kubebuilder:default=true
	PeerTLSEnabled bool `json:"peerTLSEnabled,omitempty"`

	// CertificateSecret is the name of the secret containing TLS certificates
	// +kubebuilder:validation:Optional
	CertificateSecret string `json:"certificateSecret,omitempty"`

	// CASecret is the name of the secret containing the CA certificate
	// +kubebuilder:validation:Optional
	CASecret string `json:"caSecret,omitempty"`

	// AutoTLS indicates whether to automatically generate TLS certificates
	// +kubebuilder:default=true
	AutoTLS bool `json:"autoTLS,omitempty"`
}

// EtcdSecuritySpec defines security configuration for etcd
type EtcdSecuritySpec struct {
	// TLS configuration
	TLS EtcdTLSSpec `json:"tls,omitempty"`
}

// EtcdResourceSpec defines resource requirements for etcd
type EtcdResourceSpec struct {
	// Requests describes the minimum amount of compute resources required
	// +kubebuilder:validation:Optional
	Requests corev1.ResourceList `json:"requests,omitempty"`

	// Limits describes the maximum amount of compute resources allowed
	// +kubebuilder:validation:Optional
	Limits corev1.ResourceList `json:"limits,omitempty"`
}

// EtcdClusterSpec defines the desired state of EtcdCluster
type EtcdClusterSpec struct {
	// Size is the number of etcd members in the cluster
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=9
	// +kubebuilder:default=3
	Size int32 `json:"size,omitempty"`

	// Version is the etcd version to use (supports both "3.5.21" and "v3.5.21" formats)
	// +kubebuilder:validation:Pattern=^v?3\.[0-9]+\.[0-9]+$
	// +kubebuilder:default="v3.5.21"
	Version string `json:"version,omitempty"`

	// Repository is the container image repository
	// +kubebuilder:default="quay.io/coreos/etcd"
	Repository string `json:"repository,omitempty"`

	// Storage configuration
	Storage EtcdStorageSpec `json:"storage,omitempty"`

	// Security configuration
	Security EtcdSecuritySpec `json:"security,omitempty"`

	// Resources configuration
	Resources EtcdResourceSpec `json:"resources,omitempty"`
}

// EtcdMember represents an etcd cluster member
type EtcdMember struct {
	// Name is the name of the etcd member
	Name string `json:"name,omitempty"`

	// ID is the etcd member ID
	ID string `json:"id,omitempty"`

	// PeerURL is the peer URL of the etcd member
	PeerURL string `json:"peerURL,omitempty"`

	// ClientURL is the client URL of the etcd member
	ClientURL string `json:"clientURL,omitempty"`

	// Ready indicates if the member is ready
	Ready bool `json:"ready,omitempty"`

	// Role indicates the role of the member (leader/follower)
	Role string `json:"role,omitempty"`
}

// EtcdClusterStatus defines the observed state of EtcdCluster
type EtcdClusterStatus struct {
	// Phase is the current phase of the etcd cluster
	Phase EtcdClusterPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the cluster's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Members is the list of etcd cluster members
	Members []EtcdMember `json:"members,omitempty"`

	// ReadyReplicas is the number of ready etcd replicas
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// LeaderID is the ID of the current etcd leader
	LeaderID string `json:"leaderID,omitempty"`

	// ClusterID is the etcd cluster ID
	ClusterID string `json:"clusterID,omitempty"`

	// ClientEndpoints are the client endpoints of the etcd cluster
	ClientEndpoints []string `json:"clientEndpoints,omitempty"`

	// LastBackupTime is the time of the last successful backup
	LastBackupTime *metav1.Time `json:"lastBackupTime,omitempty"`

	// LastUpdateTime is the last time the status was updated
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=etcd
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Size",type="integer",JSONPath=".spec.size"
// +kubebuilder:printcolumn:name="Ready",type="integer",JSONPath=".status.readyReplicas"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// EtcdCluster is the Schema for the etcdclusters API
type EtcdCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EtcdClusterSpec   `json:"spec,omitempty"`
	Status EtcdClusterStatus `json:"status,omitempty"`
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
