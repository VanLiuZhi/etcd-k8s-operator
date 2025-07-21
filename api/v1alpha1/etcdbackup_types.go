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

// EtcdBackupPhase represents the phase of an EtcdBackup
type EtcdBackupPhase string

const (
	// EtcdBackupPhaseRunning indicates the backup is running
	EtcdBackupPhaseRunning EtcdBackupPhase = "Running"
	// EtcdBackupPhaseCompleted indicates the backup is completed
	EtcdBackupPhaseCompleted EtcdBackupPhase = "Completed"
	// EtcdBackupPhaseFailed indicates the backup has failed
	EtcdBackupPhaseFailed EtcdBackupPhase = "Failed"
)

// EtcdBackupStorageType represents the storage type for backup
type EtcdBackupStorageType string

const (
	// EtcdBackupStorageTypeS3 indicates S3 storage
	EtcdBackupStorageTypeS3 EtcdBackupStorageType = "S3"
	// EtcdBackupStorageTypeGCS indicates GCS storage
	EtcdBackupStorageTypeGCS EtcdBackupStorageType = "GCS"
	// EtcdBackupStorageTypeLocal indicates local storage
	EtcdBackupStorageTypeLocal EtcdBackupStorageType = "Local"
)

// EtcdS3BackupSpec defines S3 backup configuration
type EtcdS3BackupSpec struct {
	// Bucket is the S3 bucket name
	Bucket string `json:"bucket"`

	// Region is the S3 region
	Region string `json:"region,omitempty"`

	// Endpoint is the S3 endpoint URL
	Endpoint string `json:"endpoint,omitempty"`

	// AccessKeySecret is the secret containing S3 access key
	AccessKeySecret string `json:"accessKeySecret,omitempty"`

	// SecretKeySecret is the secret containing S3 secret key
	SecretKeySecret string `json:"secretKeySecret,omitempty"`

	// Path is the path prefix in the bucket
	Path string `json:"path,omitempty"`
}

// EtcdRetentionPolicy defines backup retention policy
type EtcdRetentionPolicy struct {
	// MaxBackups is the maximum number of backups to retain
	MaxBackups int32 `json:"maxBackups,omitempty"`

	// MaxAge is the maximum age of backups to retain
	MaxAge string `json:"maxAge,omitempty"`
}

// EtcdBackupSpec defines the desired state of EtcdBackup
type EtcdBackupSpec struct {
	// ClusterName is the name of the EtcdCluster to backup
	ClusterName string `json:"clusterName"`

	// ClusterNamespace is the namespace of the EtcdCluster
	ClusterNamespace string `json:"clusterNamespace,omitempty"`

	// StorageType is the type of storage backend
	StorageType EtcdBackupStorageType `json:"storageType"`

	// Schedule is the cron schedule for automatic backups
	Schedule string `json:"schedule,omitempty"`

	// S3 configuration for S3 storage
	S3 *EtcdS3BackupSpec `json:"s3,omitempty"`

	// RetentionPolicy defines backup retention
	RetentionPolicy EtcdRetentionPolicy `json:"retentionPolicy,omitempty"`

	// Compression indicates whether to compress the backup
	Compression bool `json:"compression,omitempty"`
}

// EtcdBackupStatus defines the observed state of EtcdBackup
type EtcdBackupStatus struct {
	// Phase is the current phase of the backup
	Phase EtcdBackupPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the backup's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// BackupSize is the size of the backup in bytes
	BackupSize int64 `json:"backupSize,omitempty"`

	// StartTime is the time when the backup started
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time when the backup completed
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// StoragePath is the path where the backup is stored
	StoragePath string `json:"storagePath,omitempty"`

	// EtcdVersion is the version of etcd that was backed up
	EtcdVersion string `json:"etcdVersion,omitempty"`

	// EtcdRevision is the etcd revision that was backed up
	EtcdRevision int64 `json:"etcdRevision,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=etcdbackup
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.clusterName"
// +kubebuilder:printcolumn:name="Storage",type="string",JSONPath=".spec.storageType"
// +kubebuilder:printcolumn:name="Size",type="string",JSONPath=".status.backupSize"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// EtcdBackup is the Schema for the etcdbackups API
type EtcdBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EtcdBackupSpec   `json:"spec,omitempty"`
	Status EtcdBackupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EtcdBackupList contains a list of EtcdBackup
type EtcdBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EtcdBackup{}, &EtcdBackupList{})
}
