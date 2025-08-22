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

package controller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
)

const (
	etcdFinalizer = "etcd.k8s.etcd.lz/finalizer"
)

// EtcdClusterReconciler reconciles a EtcdCluster object
type EtcdClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=k8s.etcd.lz,resources=etcdclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.etcd.lz,resources=etcdclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.etcd.lz,resources=etcdclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("etcdcluster", req.NamespacedName)
	logger.Info("Starting reconciliation")

	// Get EtcdCluster instance
	cluster := &etcdv1alpha1.EtcdCluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("EtcdCluster resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get EtcdCluster")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if cluster.DeletionTimestamp != nil {
		logger.Info("EtcdCluster is being deleted")
		return r.handleDeletion(ctx, cluster)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(cluster, etcdFinalizer) {
		logger.Info("Adding finalizer to EtcdCluster")
		controllerutil.AddFinalizer(cluster, etcdFinalizer)
		if err := r.Update(ctx, cluster); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Set defaults
	cluster.SetDefaults()

	// Handle cluster based on current phase
	switch cluster.Status.Phase {
	case etcdv1alpha1.ClusterPhaseNone:
		logger.Info("Initializing EtcdCluster")
		return r.handleInitialization(ctx, cluster)
	case etcdv1alpha1.ClusterPhaseCreating:
		logger.Info("Creating EtcdCluster")
		return r.handleCreating(ctx, cluster)
	case etcdv1alpha1.ClusterPhaseRunning:
		logger.Info("Managing running EtcdCluster")
		return r.handleRunning(ctx, cluster)
	case etcdv1alpha1.ClusterPhaseFailed:
		logger.Info("EtcdCluster has failed, attempting recovery")
		return r.handleFailed(ctx, cluster)
	default:
		logger.Info("Unknown phase, setting to Creating", "phase", cluster.Status.Phase)
		cluster.Status.Phase = etcdv1alpha1.ClusterPhaseCreating
		if err := r.Status().Update(ctx, cluster); err != nil {
			logger.Error(err, "Failed to update status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
}

// handleInitialization handles the initialization of a new etcd cluster
func (r *EtcdClusterReconciler) handleInitialization(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Initializing etcd cluster")

	// Update status to Creating
	cluster.Status.Phase = etcdv1alpha1.ClusterPhaseCreating
	cluster.Status.Size = 0
	if err := r.Status().Update(ctx, cluster); err != nil {
		logger.Error(err, "Failed to update status to Creating")
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

// handleCreating handles the creation of etcd cluster resources
func (r *EtcdClusterReconciler) handleCreating(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Creating etcd cluster resources")

	// TODO: Implement cluster creation logic
	// For now, just mark as running
	cluster.Status.Phase = etcdv1alpha1.ClusterPhaseRunning
	cluster.Status.Size = cluster.Spec.Size
	if err := r.Status().Update(ctx, cluster); err != nil {
		logger.Error(err, "Failed to update status to Running")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// handleRunning handles the management of a running etcd cluster
func (r *EtcdClusterReconciler) handleRunning(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Managing running etcd cluster")

	// TODO: Implement cluster management logic
	// - Check cluster health
	// - Handle scaling
	// - Handle member failures

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// handleFailed handles recovery of a failed etcd cluster
func (r *EtcdClusterReconciler) handleFailed(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Attempting to recover failed etcd cluster")

	// TODO: Implement cluster recovery logic

	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

// handleDeletion handles the deletion of etcd cluster resources
func (r *EtcdClusterReconciler) handleDeletion(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Handling etcd cluster deletion")

	// TODO: Implement cleanup logic
	// - Delete etcd pods
	// - Delete services
	// - Delete configmaps

	// Remove finalizer
	controllerutil.RemoveFinalizer(cluster, etcdFinalizer)
	if err := r.Update(ctx, cluster); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	logger.Info("EtcdCluster deletion completed")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EtcdClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&etcdv1alpha1.EtcdCluster{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
