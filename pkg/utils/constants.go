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

package utils

import "time"

const (
	// EtcdFinalizer is the finalizer used by the etcd operator
	EtcdFinalizer = "etcd.etcd.io/finalizer"

	// DefaultEtcdVersion is the default etcd version (官方镜像)
	DefaultEtcdVersion = "v3.5.21"

	// DefaultEtcdRepository is the default etcd image repository (官方镜像)
	DefaultEtcdRepository = "quay.io/coreos/etcd"

	// DefaultClusterSize is the default cluster size
	DefaultClusterSize = 3

	// DefaultStorageSize is the default storage size
	DefaultStorageSize = "10Gi"

	// EtcdClientPort is the default etcd client port
	EtcdClientPort = 2379

	// EtcdPeerPort is the default etcd peer port
	EtcdPeerPort = 2380

	// EtcdDataDir is the default etcd data directory (官方镜像使用 /data)
	EtcdDataDir = "/data"

	// EtcdWALDir is the default etcd WAL directory (官方镜像使用 /data/wal)
	EtcdWALDir = "/data/wal"

	// DefaultRequeueInterval is the default requeue interval
	DefaultRequeueInterval = 30 * time.Second

	// DefaultHealthCheckInterval is the default health check interval
	DefaultHealthCheckInterval = 5 * time.Minute

	// DefaultReconcileTimeout is the default reconcile timeout
	DefaultReconcileTimeout = 10 * time.Minute
)

// 标签键定义
const (
	// LabelAppName 应用名称标签键
	LabelAppName = "app.kubernetes.io/name"

	// LabelAppInstance 应用实例标签键
	LabelAppInstance = "app.kubernetes.io/instance"

	// LabelAppComponent 应用组件标签键
	LabelAppComponent = "app.kubernetes.io/component"

	// LabelAppManagedBy 应用管理者标签键
	LabelAppManagedBy = "app.kubernetes.io/managed-by"

	// LabelAppVersion 应用版本标签键
	LabelAppVersion = "app.kubernetes.io/version"

	// LabelEtcdCluster etcd集群标签键
	LabelEtcdCluster = "etcd.etcd.io/cluster"

	// LabelEtcdMember is the label key for etcd member
	LabelEtcdMember = "etcd.etcd.io/member"
)

// Annotation keys
const (
	// AnnotationLastAppliedConfig is the annotation key for last applied config
	AnnotationLastAppliedConfig = "etcd.etcd.io/last-applied-config"

	// AnnotationLastBackupTime is the annotation key for last backup time
	AnnotationLastBackupTime = "etcd.etcd.io/last-backup-time"

	// AnnotationClusterID is the annotation key for cluster ID
	AnnotationClusterID = "etcd.etcd.io/cluster-id"
)

// Condition types
const (
	// ConditionTypeReady indicates whether the cluster is ready
	ConditionTypeReady = "Ready"

	// ConditionTypeProgressing indicates whether the cluster is progressing
	ConditionTypeProgressing = "Progressing"

	// ConditionTypeDegraded indicates whether the cluster is degraded
	ConditionTypeDegraded = "Degraded"

	// ConditionTypeAvailable indicates whether the cluster is available
	ConditionTypeAvailable = "Available"
)

// Condition reasons
const (
	// ReasonCreating indicates the cluster is being created
	ReasonCreating = "Creating"

	// ReasonRunning indicates the cluster is running
	ReasonRunning = "Running"

	// ReasonScaling indicates the cluster is scaling
	ReasonScaling = "Scaling"

	// ReasonUpgrading indicates the cluster is upgrading
	ReasonUpgrading = "Upgrading"

	// ReasonFailed indicates the cluster has failed
	ReasonFailed = "Failed"

	// ReasonDeleting indicates the cluster is being deleted
	ReasonDeleting = "Deleting"

	// ReasonHealthy indicates the cluster is healthy
	ReasonHealthy = "Healthy"

	// ReasonUnhealthy indicates the cluster is unhealthy
	ReasonUnhealthy = "Unhealthy"

	// ReasonStopped indicates the cluster is stopped
	ReasonStopped = "Stopped"
)

// Event reasons
const (
	// EventReasonClusterCreated indicates cluster created event
	EventReasonClusterCreated = "ClusterCreated"

	// EventReasonClusterDeleted indicates cluster deleted event
	EventReasonClusterDeleted = "ClusterDeleted"

	// EventReasonClusterScaled indicates cluster scaled event
	EventReasonClusterScaled = "ClusterScaled"

	// EventReasonClusterUpgraded indicates cluster upgraded event
	EventReasonClusterUpgraded = "ClusterUpgraded"

	// EventReasonClusterFailed indicates cluster failed event
	EventReasonClusterFailed = "ClusterFailed"

	// EventReasonMemberAdded indicates member added event
	EventReasonMemberAdded = "MemberAdded"

	// EventReasonMemberRemoved indicates member removed event
	EventReasonMemberRemoved = "MemberRemoved"

	// EventReasonBackupCreated indicates backup created event
	EventReasonBackupCreated = "BackupCreated"

	// EventReasonBackupFailed indicates backup failed event
	EventReasonBackupFailed = "BackupFailed"

	// EventReasonClusterStopped indicates cluster stopped event
	EventReasonClusterStopped = "ClusterStopped"
)
