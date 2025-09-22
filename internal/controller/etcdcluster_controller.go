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
	"fmt"
	"sync"

	etcdv1alpha1 "github.com/etcd-lz/etcd-k8s-operator/api/v1alpha1"
	"github.com/etcd-lz/etcd-k8s-operator/pkg/cluster"
	"github.com/etcd-lz/etcd-k8s-operator/pkg/k8s"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	etcdFinalizer = "etcd.k8s.etcd.lz/finalizer"
)

// EtcdClusterReconciler 协调 EtcdCluster 对象
type EtcdClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	KubeCli  kubernetes.Interface

	// clusters 存储正在管理的集群实例，使用sync.Map保证并发安全
	clusters sync.Map
}

// +kubebuilder:rbac:groups=k8s.etcd.lz,resources=etcdclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8s.etcd.lz,resources=etcdclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=k8s.etcd.lz,resources=etcdclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile 是主要的 kubernetes 协调循环的一部分，旨在
// 将集群的当前状态移动到更接近期望状态
func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("etcdcluster", req.NamespacedName)
	logger.Info("Starting reconciliation")

	// 获取 EtcdCluster 实例
	etcdCluster := &etcdv1alpha1.EtcdCluster{}
	if err := r.Get(ctx, req.NamespacedName, etcdCluster); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("EtcdCluster resource not found, ignoring since object must be deleted")
			// 清理集群实例
			r.clusters.Delete(req.NamespacedName.String())
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get EtcdCluster")
		return ctrl.Result{}, err
	}

	// 处理删除
	if etcdCluster.DeletionTimestamp != nil {
		logger.Info("EtcdCluster is being deleted")
		return r.handleDeletion(ctx, etcdCluster, logger)
	}

	// 添加 finalizer（如果不存在）
	if !controllerutil.ContainsFinalizer(etcdCluster, etcdFinalizer) {
		logger.Info("Adding finalizer to EtcdCluster")
		controllerutil.AddFinalizer(etcdCluster, etcdFinalizer)
		if err := r.Update(ctx, etcdCluster); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 设置默认值
	etcdCluster.SetDefaults()

	// 验证集群规格
	if err := r.validateClusterSpec(etcdCluster.Spec); err != nil {
		logger.Error(err, "Invalid cluster spec")
		return ctrl.Result{}, err
	}

	clusterKey := req.NamespacedName.String()

	// 检查是否已存在集群实例
	if existingClusterInterface, exists := r.clusters.Load(clusterKey); exists {
		// 更新现有集群
		if existingCluster, ok := existingClusterInterface.(*cluster.Cluster); ok {
			existingCluster.Update(etcdCluster)
			logger.Info("Updated existing cluster")
		}
	} else {
		// 创建新的集群实例
		config := cluster.Config{
			ServiceAccount: "default", // TODO: 从配置中获取
			KubeCli:        r.KubeCli,
			Client:         r.Client,
			Recorder:       r.Recorder,
		}

		newCluster := cluster.New(config, etcdCluster, logger)
		r.clusters.Store(clusterKey, newCluster)
		logger.Info("Created new cluster")
	}

	return ctrl.Result{}, nil
}

// handleDeletion 处理 EtcdCluster 的删除
func (r *EtcdClusterReconciler) handleDeletion(ctx context.Context, etcdCluster *etcdv1alpha1.EtcdCluster, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Handling etcd cluster deletion")

	clusterKey := fmt.Sprintf("%s/%s", etcdCluster.Namespace, etcdCluster.Name)

	// 删除集群实例
	if clusterInstanceInterface, exists := r.clusters.Load(clusterKey); exists {
		if clusterInstance, ok := clusterInstanceInterface.(*cluster.Cluster); ok {
			clusterInstance.Delete()
			r.clusters.Delete(clusterKey)
			logger.Info("Cluster instance deleted")
		}
	}

	// TODO: 实现清理逻辑
	// - 删除 etcd pods
	// - 删除 services
	// - 删除 configmaps
	// - 删除 PVCs

	// 主动清理相关资源（未验证）
	//if err := r.cleanupClusterResources(ctx, etcdCluster, logger); err != nil {
	//	logger.Error(err, "Failed to cleanup cluster resources")
	//	return ctrl.Result{}, err
	//}

	// 移除 finalizer
	controllerutil.RemoveFinalizer(etcdCluster, etcdFinalizer)
	if err := r.Update(ctx, etcdCluster); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	logger.Info("EtcdCluster deletion completed")
	return ctrl.Result{}, nil
}

// cleanupClusterResources 清理集群相关资源
func (r *EtcdClusterReconciler) cleanupClusterResources(ctx context.Context, etcdCluster *etcdv1alpha1.EtcdCluster, logger logr.Logger) error {
	// 删除 etcd pods
	if err := r.deleteEtcdPods(ctx, etcdCluster, logger); err != nil {
		return fmt.Errorf("failed to delete etcd pods: %v", err)
	}

	// 删除 services
	if err := r.deleteEtcdServices(ctx, etcdCluster, logger); err != nil {
		return fmt.Errorf("failed to delete etcd services: %v", err)
	}

	// 删除 PVCs
	if err := r.deleteEtcdPVCs(ctx, etcdCluster, logger); err != nil {
		return fmt.Errorf("failed to delete etcd PVCs: %v", err)
	}

	return nil
}

// deleteEtcdPods 删除etcd pods
func (r *EtcdClusterReconciler) deleteEtcdPods(ctx context.Context, etcdCluster *etcdv1alpha1.EtcdCluster, logger logr.Logger) error {
	// 使用标签选择器删除所有属于该集群的Pods
	labels := map[string]string{
		"etcd_cluster": etcdCluster.Name,
		"app":          "etcd",
	}

	err := k8s.DeletePods(ctx, r.KubeCli, etcdCluster.Namespace, labels, 30)
	if err != nil {
		return fmt.Errorf("failed to delete etcd pods: %v", err)
	}

	logger.Info("Etcd pods deleted")
	return nil
}

// deleteEtcdServices 删除etcd services
func (r *EtcdClusterReconciler) deleteEtcdServices(ctx context.Context, etcdCluster *etcdv1alpha1.EtcdCluster, logger logr.Logger) error {
	// 删除客户端服务
	clientServiceName := k8s.ClientServiceName(etcdCluster.Name)
	if err := k8s.DeleteService(ctx, r.KubeCli, clientServiceName, etcdCluster.Namespace); err != nil {
		return fmt.Errorf("failed to delete client service: %v", err)
	}

	// 删除peer服务
	if err := k8s.DeleteService(ctx, r.KubeCli, etcdCluster.Name, etcdCluster.Namespace); err != nil {
		return fmt.Errorf("failed to delete peer service: %v", err)
	}

	logger.Info("Etcd services deleted")
	return nil
}

// deleteEtcdPVCs 删除etcd PVCs
func (r *EtcdClusterReconciler) deleteEtcdPVCs(ctx context.Context, etcdCluster *etcdv1alpha1.EtcdCluster, logger logr.Logger) error {
	// 获取所有属于该集群的PVCs
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{
		"etcd_cluster": etcdCluster.Name,
	}}

	pvcList, err := r.KubeCli.CoreV1().PersistentVolumeClaims(etcdCluster.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&labelSelector),
	})
	if err != nil {
		return fmt.Errorf("failed to list PVCs: %v", err)
	}

	// 删除每个PVC
	for _, pvc := range pvcList.Items {
		if err := k8s.DeletePVC(r.KubeCli, etcdCluster.Namespace, pvc.Name); err != nil {
			return fmt.Errorf("failed to delete PVC %s: %v", pvc.Name, err)
		}
		logger.Info("Etcd PVC deleted", "pvc", pvc.Name)
	}

	return nil
}

// validateClusterSpec 验证集群规格
func (r *EtcdClusterReconciler) validateClusterSpec(spec etcdv1alpha1.ClusterSpec) error {
	if spec.Size <= 0 {
		return fmt.Errorf("cluster size must be positive")
	}
	if spec.Size%2 == 0 {
		return fmt.Errorf("cluster size must be odd")
	}
	if spec.Size > 7 {
		return fmt.Errorf("cluster size must not exceed 7")
	}
	return nil
}

// SetupWithManager 设置控制器与管理器
func (r *EtcdClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&etcdv1alpha1.EtcdCluster{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
