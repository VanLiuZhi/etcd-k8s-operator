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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EtcdRestorePhase represents the phase of an EtcdRestore
type EtcdRestorePhase string

const (
	// EtcdRestorePhaseRunning indicates the restore is running
	EtcdRestorePhaseRunning EtcdRestorePhase = "Running"
	// EtcdRestorePhaseCompleted indicates the restore is completed
	EtcdRestorePhaseCompleted EtcdRestorePhase = "Completed"
	// EtcdRestorePhaseFailed indicates the restore has failed
	EtcdRestorePhaseFailed EtcdRestorePhase = "Failed"
)

// EtcdRestoreType represents the type of restore operation
type EtcdRestoreType string

const (
	// EtcdRestoreTypeReplace replaces the existing cluster
	EtcdRestoreTypeReplace EtcdRestoreType = "Replace"
	// EtcdRestoreTypeNew creates a new cluster from backup
	EtcdRestoreTypeNew EtcdRestoreType = "New"
)

// EtcdRestoreSpec defines the desired state of EtcdRestore
type EtcdRestoreSpec struct {
	// BackupName is the name of the EtcdBackup to restore from
	BackupName string `json:"backupName"`

	// BackupNamespace is the namespace of the EtcdBackup
	BackupNamespace string `json:"backupNamespace,omitempty"`

	// ClusterName is the name of the target EtcdCluster
	ClusterName string `json:"clusterName"`

	// ClusterTemplate is the template for creating a new cluster (for new restore type)
	ClusterTemplate *EtcdClusterSpec `json:"clusterTemplate,omitempty"`

	// RestoreType is the type of restore operation
	RestoreType EtcdRestoreType `json:"restoreType,omitempty"`

	// DataDir is the data directory for etcd
	DataDir string `json:"dataDir,omitempty"`

	// SkipHashCheck skips hash check during restore
	SkipHashCheck bool `json:"skipHashCheck,omitempty"`

	// WalDir is the WAL directory for etcd
	WalDir string `json:"walDir,omitempty"`
}

// EtcdRestoreStatus defines the observed state of EtcdRestore
type EtcdRestoreStatus struct {
	// Phase is the current phase of the restore
	Phase EtcdRestorePhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the restore's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// StartTime is the time when the restore started
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time when the restore completed
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// RestoredCluster is the name of the restored cluster
	RestoredCluster string `json:"restoredCluster,omitempty"`

	// RestoredSize is the size of the restored data in bytes
	RestoredSize int64 `json:"restoredSize,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=etcdrestore
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Backup",type="string",JSONPath=".spec.backupName"
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.clusterName"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.restoreType"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// EtcdRestore is the Schema for the etcdrestores API
type EtcdRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EtcdRestoreSpec   `json:"spec,omitempty"`
	Status EtcdRestoreStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EtcdRestoreList contains a list of EtcdRestore
type EtcdRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EtcdRestore{}, &EtcdRestoreList{})
}
