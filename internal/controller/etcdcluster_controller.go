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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	etcdclient "github.com/your-org/etcd-k8s-operator/pkg/etcd"
	"github.com/your-org/etcd-k8s-operator/pkg/k8s"
	"github.com/your-org/etcd-k8s-operator/pkg/utils"
)

// EtcdClusterReconciler reconciles a EtcdCluster object
type EtcdClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=etcd.etcd.io,resources=etcdclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=etcd.etcd.io,resources=etcdclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=etcd.etcd.io,resources=etcdclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("etcdcluster", req.NamespacedName)
	logger.Info("Starting reconciliation")

	// 1. 获取 EtcdCluster 实例
	cluster := &etcdv1alpha1.EtcdCluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("EtcdCluster resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get EtcdCluster")
		return ctrl.Result{}, err
	}

	// 2. 检查删除标记
	if cluster.DeletionTimestamp != nil {
		logger.Info("EtcdCluster is being deleted")
		return r.handleDeletion(ctx, cluster)
	}

	// 3. 确保 Finalizer
	if !controllerutil.ContainsFinalizer(cluster, utils.EtcdFinalizer) {
		logger.Info("Adding finalizer to EtcdCluster")
		controllerutil.AddFinalizer(cluster, utils.EtcdFinalizer)
		if err := r.Update(ctx, cluster); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 4. 设置默认值
	r.setDefaults(cluster)

	// 5. 状态机处理
	switch cluster.Status.Phase {
	case "":
		logger.Info("Initializing EtcdCluster")
		return r.handleInitialization(ctx, cluster)
	case etcdv1alpha1.EtcdClusterPhaseCreating:
		logger.Info("Creating EtcdCluster")
		return r.handleCreating(ctx, cluster)
	case etcdv1alpha1.EtcdClusterPhaseRunning:
		logger.Info("EtcdCluster is running, performing health check")
		return r.handleRunning(ctx, cluster)
	case etcdv1alpha1.EtcdClusterPhaseScaling:
		logger.Info("Scaling EtcdCluster")
		return r.handleScaling(ctx, cluster)
	case etcdv1alpha1.EtcdClusterPhaseFailed:
		logger.Info("EtcdCluster has failed, attempting recovery")
		return r.handleFailed(ctx, cluster)
	default:
		logger.Info("Unknown phase, resetting to initialization", "phase", cluster.Status.Phase)
		return r.handleInitialization(ctx, cluster)
	}
}

// setDefaults sets default values for the EtcdCluster
func (r *EtcdClusterReconciler) setDefaults(cluster *etcdv1alpha1.EtcdCluster) {
	if cluster.Spec.Size == 0 {
		cluster.Spec.Size = utils.DefaultClusterSize
	}
	if cluster.Spec.Version == "" {
		cluster.Spec.Version = utils.DefaultEtcdVersion
	}
	if cluster.Spec.Repository == "" {
		cluster.Spec.Repository = utils.DefaultEtcdRepository
	}
	if cluster.Spec.Storage.Size.IsZero() {
		cluster.Spec.Storage.Size = resource.MustParse(utils.DefaultStorageSize)
	}
}

// handleInitialization handles the initialization phase
func (r *EtcdClusterReconciler) handleInitialization(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 验证集群规范
	if err := r.validateClusterSpec(cluster); err != nil {
		logger.Error(err, "Cluster specification validation failed")
		return r.updateStatusWithError(ctx, cluster, etcdv1alpha1.EtcdClusterPhaseFailed, err)
	}

	// 转换到创建状态
	cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseCreating
	cluster.Status.ObservedGeneration = cluster.Generation

	// 设置初始条件
	r.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionTrue, utils.ReasonCreating, "Creating etcd cluster")

	if err := r.Status().Update(ctx, cluster); err != nil {
		logger.Error(err, "Failed to update cluster status")
		return ctrl.Result{}, err
	}

	r.Recorder.Event(cluster, corev1.EventTypeNormal, utils.EventReasonClusterCreated, "Started creating etcd cluster")
	return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
}

// handleCreating handles the creating phase
func (r *EtcdClusterReconciler) handleCreating(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. 创建必要的 Kubernetes 资源
	if err := r.ensureResources(ctx, cluster); err != nil {
		logger.Error(err, "Failed to ensure resources")
		return r.updateStatusWithError(ctx, cluster, etcdv1alpha1.EtcdClusterPhaseFailed, err)
	}

	// 2. 对于多节点集群，使用渐进式启动策略
	if cluster.Spec.Size > 1 {
		return r.handleMultiNodeClusterCreation(ctx, cluster)
	}

	// 3. 单节点集群的处理逻辑
	ready, err := r.checkClusterReady(ctx, cluster)
	if err != nil {
		logger.Error(err, "Failed to check cluster readiness")
		return r.updateStatusWithError(ctx, cluster, etcdv1alpha1.EtcdClusterPhaseFailed, err)
	}

	if ready {
		// 转换到运行状态
		cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseRunning
		r.setCondition(cluster, utils.ConditionTypeReady, metav1.ConditionTrue, utils.ReasonRunning, "Etcd cluster is running")
		r.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionFalse, utils.ReasonRunning, "Etcd cluster creation completed")

		if err := r.updateClusterStatus(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}

		r.Recorder.Event(cluster, corev1.EventTypeNormal, utils.EventReasonClusterCreated, "Etcd cluster created successfully")
		return ctrl.Result{RequeueAfter: utils.DefaultHealthCheckInterval}, nil
	}

	// 继续等待
	logger.Info("Waiting for cluster to be ready")
	return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
}

// handleMultiNodeClusterCreation handles multi-node cluster creation with progressive startup
func (r *EtcdClusterReconciler) handleMultiNodeClusterCreation(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 获取当前 StatefulSet 状态
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, sts)
	if err != nil {
		if errors.IsNotFound(err) {
			// StatefulSet 还没有创建，返回等待
			return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
		}
		return ctrl.Result{}, err
	}

	// 检查当前就绪的副本数
	readyReplicas := sts.Status.ReadyReplicas
	desiredSize := cluster.Spec.Size
	currentReplicas := *sts.Spec.Replicas

	logger.Info("Multi-node cluster creation progress",
		"readyReplicas", readyReplicas,
		"desiredSize", desiredSize,
		"currentReplicas", currentReplicas)

	// 如果所有副本都已就绪，集群创建完成
	if readyReplicas == desiredSize {
		cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseRunning
		r.setCondition(cluster, utils.ConditionTypeReady, metav1.ConditionTrue, utils.ReasonRunning, "Multi-node etcd cluster is running")
		r.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionFalse, utils.ReasonRunning, "Multi-node etcd cluster creation completed")

		if err := r.updateClusterStatus(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}

		r.Recorder.Event(cluster, corev1.EventTypeNormal, utils.EventReasonClusterCreated,
			fmt.Sprintf("Multi-node etcd cluster with %d nodes created successfully", desiredSize))
		return ctrl.Result{RequeueAfter: utils.DefaultHealthCheckInterval}, nil
	}

	// 实现分阶段启动策略
	// 1. 首先只启动第一个节点
	// 2. 等第一个节点就绪后，再逐步启动其他节点

	if currentReplicas == 0 {
		// 第一阶段：启动第一个节点
		logger.Info("Starting first node of multi-node cluster")
		*sts.Spec.Replicas = 1
		if err := r.Update(ctx, sts); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	if readyReplicas == 1 && currentReplicas == 1 && desiredSize > 1 {
		// 第二阶段：第一个节点就绪后，启动所有节点
		logger.Info("First node ready, starting all nodes")
		*sts.Spec.Replicas = desiredSize
		if err := r.Update(ctx, sts); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	// 更新状态信息
	r.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionTrue, utils.ReasonCreating,
		fmt.Sprintf("Creating multi-node cluster: %d/%d nodes ready", readyReplicas, desiredSize))

	if err := r.updateClusterStatus(ctx, cluster); err != nil {
		return ctrl.Result{}, err
	}

	// 继续等待更多节点就绪
	return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
}

// handleRunning handles the running phase
func (r *EtcdClusterReconciler) handleRunning(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. 检查是否需要扩缩容
	if r.needsScaling(cluster) {
		logger.Info("Cluster needs scaling", "current", cluster.Status.ReadyReplicas, "desired", cluster.Spec.Size)
		cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseScaling
		r.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionTrue, utils.ReasonScaling, "Scaling etcd cluster")

		if err := r.Status().Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 2. 执行健康检查
	if err := r.performHealthCheck(ctx, cluster); err != nil {
		logger.Error(err, "Health check failed")
		return r.updateStatusWithError(ctx, cluster, etcdv1alpha1.EtcdClusterPhaseFailed, err)
	}

	// 3. 更新集群状态
	if err := r.updateClusterStatus(ctx, cluster); err != nil {
		return ctrl.Result{}, err
	}

	// 定期重新调度进行健康检查
	return ctrl.Result{RequeueAfter: utils.DefaultHealthCheckInterval}, nil
}

// handleScaling handles the scaling phase
func (r *EtcdClusterReconciler) handleScaling(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	currentSize := cluster.Status.ReadyReplicas
	desiredSize := cluster.Spec.Size

	if currentSize < desiredSize {
		logger.Info("Scaling up cluster", "from", currentSize, "to", desiredSize)
		return r.handleScaleUp(ctx, cluster)
	} else if currentSize > desiredSize {
		logger.Info("Scaling down cluster", "from", currentSize, "to", desiredSize)
		return r.handleScaleDown(ctx, cluster)
	}

	// 扩缩容完成，转换回运行状态
	cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseRunning
	r.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionFalse, utils.ReasonRunning, "Scaling completed")

	if err := r.Status().Update(ctx, cluster); err != nil {
		return ctrl.Result{}, err
	}

	r.Recorder.Event(cluster, corev1.EventTypeNormal, utils.EventReasonClusterScaled, "Cluster scaling completed")
	return ctrl.Result{RequeueAfter: utils.DefaultHealthCheckInterval}, nil
}

// handleFailed handles the failed phase
func (r *EtcdClusterReconciler) handleFailed(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Attempting to recover from failed state")

	// 尝试重新创建资源
	if err := r.ensureResources(ctx, cluster); err != nil {
		logger.Error(err, "Failed to recreate resources during recovery")
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	// 检查是否恢复
	ready, err := r.checkClusterReady(ctx, cluster)
	if err != nil {
		logger.Error(err, "Failed to check cluster readiness during recovery")
		return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
	}

	if ready {
		logger.Info("Cluster recovered from failed state")
		cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseRunning
		r.setCondition(cluster, utils.ConditionTypeReady, metav1.ConditionTrue, utils.ReasonRunning, "Cluster recovered")

		if err := r.updateClusterStatus(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}

		r.Recorder.Event(cluster, corev1.EventTypeNormal, "ClusterRecovered", "Cluster recovered from failed state")
		return ctrl.Result{RequeueAfter: utils.DefaultHealthCheckInterval}, nil
	}

	// 继续尝试恢复
	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

// handleDeletion handles cluster deletion
func (r *EtcdClusterReconciler) handleDeletion(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(cluster, utils.EtcdFinalizer) {
		return ctrl.Result{}, nil
	}

	logger.Info("Cleaning up etcd cluster resources")

	// 执行清理逻辑
	if err := r.cleanupResources(ctx, cluster); err != nil {
		logger.Error(err, "Failed to cleanup resources")
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// 移除 finalizer
	controllerutil.RemoveFinalizer(cluster, utils.EtcdFinalizer)
	if err := r.Update(ctx, cluster); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	r.Recorder.Event(cluster, corev1.EventTypeNormal, utils.EventReasonClusterDeleted, "Etcd cluster deleted successfully")
	logger.Info("Etcd cluster deleted successfully")
	return ctrl.Result{}, nil
}

// validateClusterSpec validates the cluster specification
func (r *EtcdClusterReconciler) validateClusterSpec(cluster *etcdv1alpha1.EtcdCluster) error {
	// 验证集群大小必须是奇数
	if cluster.Spec.Size%2 == 0 {
		return fmt.Errorf("cluster size must be odd number, got %d", cluster.Spec.Size)
	}

	// 验证集群大小范围
	if cluster.Spec.Size < 1 || cluster.Spec.Size > 9 {
		return fmt.Errorf("cluster size must be between 1 and 9, got %d", cluster.Spec.Size)
	}

	return nil
}

// updateStatusWithError updates the cluster status with error
func (r *EtcdClusterReconciler) updateStatusWithError(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, phase etcdv1alpha1.EtcdClusterPhase, err error) (ctrl.Result, error) {
	cluster.Status.Phase = phase
	r.setCondition(cluster, utils.ConditionTypeReady, metav1.ConditionFalse, utils.ReasonFailed, err.Error())
	r.setCondition(cluster, utils.ConditionTypeDegraded, metav1.ConditionTrue, utils.ReasonFailed, err.Error())

	if updateErr := r.Status().Update(ctx, cluster); updateErr != nil {
		return ctrl.Result{}, updateErr
	}

	r.Recorder.Event(cluster, corev1.EventTypeWarning, utils.EventReasonClusterFailed, err.Error())
	return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
}

// setCondition sets a condition on the cluster status
func (r *EtcdClusterReconciler) setCondition(cluster *etcdv1alpha1.EtcdCluster, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	// 查找现有条件
	for i, existingCondition := range cluster.Status.Conditions {
		if existingCondition.Type == conditionType {
			// 如果状态没有变化，只更新时间戳
			if existingCondition.Status == status && existingCondition.Reason == reason {
				cluster.Status.Conditions[i].LastTransitionTime = condition.LastTransitionTime
				return
			}
			// 更新现有条件
			cluster.Status.Conditions[i] = condition
			return
		}
	}

	// 添加新条件
	cluster.Status.Conditions = append(cluster.Status.Conditions, condition)
}

// ensureResources ensures all necessary Kubernetes resources exist
func (r *EtcdClusterReconciler) ensureResources(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	// 1. 确保 ConfigMap
	if err := r.ensureConfigMap(ctx, cluster); err != nil {
		return fmt.Errorf("failed to ensure ConfigMap: %w", err)
	}

	// 2. 确保 Services
	if err := r.ensureServices(ctx, cluster); err != nil {
		return fmt.Errorf("failed to ensure Services: %w", err)
	}

	// 3. 确保 StatefulSet
	if err := r.ensureStatefulSet(ctx, cluster); err != nil {
		return fmt.Errorf("failed to ensure StatefulSet: %w", err)
	}

	return nil
}

// ensureConfigMap ensures the ConfigMap exists
func (r *EtcdClusterReconciler) ensureConfigMap(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	desired := k8s.BuildConfigMap(cluster)

	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, existing)

	if errors.IsNotFound(err) {
		// 创建新的 ConfigMap
		if err := ctrl.SetControllerReference(cluster, desired, r.Scheme); err != nil {
			return err
		}
		return r.Create(ctx, desired)
	} else if err != nil {
		return err
	}

	// ConfigMap 存在，检查是否需要更新
	if existing.Data["etcd.conf"] != desired.Data["etcd.conf"] {
		existing.Data = desired.Data
		return r.Update(ctx, existing)
	}

	return nil
}

// ensureServices ensures both client and peer services exist
func (r *EtcdClusterReconciler) ensureServices(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	// 确保客户端服务
	clientService := k8s.BuildClientService(cluster)
	if err := r.ensureService(ctx, cluster, clientService); err != nil {
		return fmt.Errorf("failed to ensure client service: %w", err)
	}

	// 确保对等服务
	peerService := k8s.BuildPeerService(cluster)
	if err := r.ensureService(ctx, cluster, peerService); err != nil {
		return fmt.Errorf("failed to ensure peer service: %w", err)
	}

	return nil
}

// ensureService ensures a service exists
func (r *EtcdClusterReconciler) ensureService(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, desired *corev1.Service) error {
	existing := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, existing)

	if errors.IsNotFound(err) {
		// 创建新的 Service
		if err := ctrl.SetControllerReference(cluster, desired, r.Scheme); err != nil {
			return err
		}
		return r.Create(ctx, desired)
	} else if err != nil {
		return err
	}

	// Service 存在，检查是否需要更新
	if !r.serviceNeedsUpdate(existing, desired) {
		return nil
	}

	// 保留 ClusterIP
	desired.Spec.ClusterIP = existing.Spec.ClusterIP
	desired.ResourceVersion = existing.ResourceVersion
	return r.Update(ctx, desired)
}

// serviceNeedsUpdate checks if a service needs to be updated
func (r *EtcdClusterReconciler) serviceNeedsUpdate(existing, desired *corev1.Service) bool {
	// 比较端口配置
	if len(existing.Spec.Ports) != len(desired.Spec.Ports) {
		return true
	}

	for i, existingPort := range existing.Spec.Ports {
		desiredPort := desired.Spec.Ports[i]
		if existingPort.Port != desiredPort.Port ||
			existingPort.TargetPort != desiredPort.TargetPort ||
			existingPort.Protocol != desiredPort.Protocol {
			return true
		}
	}

	// 比较选择器
	if len(existing.Spec.Selector) != len(desired.Spec.Selector) {
		return true
	}

	for k, v := range desired.Spec.Selector {
		if existing.Spec.Selector[k] != v {
			return true
		}
	}

	return false
}

// ensureStatefulSet ensures the StatefulSet exists
func (r *EtcdClusterReconciler) ensureStatefulSet(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	desired := k8s.BuildStatefulSet(cluster)

	existing := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, existing)

	if errors.IsNotFound(err) {
		// 创建新的 StatefulSet
		if err := ctrl.SetControllerReference(cluster, desired, r.Scheme); err != nil {
			return err
		}
		return r.Create(ctx, desired)
	} else if err != nil {
		return err
	}

	// StatefulSet 存在，检查是否需要更新
	if r.statefulSetNeedsUpdate(existing, desired) {
		existing.Spec = desired.Spec
		return r.Update(ctx, existing)
	}

	return nil
}

// statefulSetNeedsUpdate checks if StatefulSet needs update
func (r *EtcdClusterReconciler) statefulSetNeedsUpdate(existing, desired *appsv1.StatefulSet) bool {
	// 比较副本数
	if *existing.Spec.Replicas != *desired.Spec.Replicas {
		return true
	}

	// 比较镜像版本
	if len(existing.Spec.Template.Spec.Containers) > 0 && len(desired.Spec.Template.Spec.Containers) > 0 {
		if existing.Spec.Template.Spec.Containers[0].Image != desired.Spec.Template.Spec.Containers[0].Image {
			return true
		}
	}

	return false
}

// checkClusterReady checks if the etcd cluster is ready
func (r *EtcdClusterReconciler) checkClusterReady(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (bool, error) {
	// 获取 StatefulSet
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, sts)
	if err != nil {
		if errors.IsNotFound(err) {
			// StatefulSet 还没有创建，返回 false 但不报错
			return false, nil
		}
		return false, err
	}

	// 检查副本数是否匹配
	if sts.Status.ReadyReplicas != cluster.Spec.Size {
		return false, nil
	}

	// 检查所有副本是否就绪
	if sts.Status.ReadyReplicas != sts.Status.Replicas {
		return false, nil
	}

	return true, nil
}

// updateClusterStatus updates the cluster status with current information
func (r *EtcdClusterReconciler) updateClusterStatus(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	// 获取 StatefulSet 状态
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, sts)
	if err != nil {
		return err
	}

	// 更新副本数状态
	cluster.Status.ReadyReplicas = sts.Status.ReadyReplicas

	// 更新成员状态
	if err := r.updateMemberStatus(ctx, cluster, sts); err != nil {
		// 记录错误但不阻止状态更新
		log.FromContext(ctx).Error(err, "Failed to update member status")
	}

	// 更新客户端端点
	if sts.Status.ReadyReplicas > 0 {
		endpoints := make([]string, 0, sts.Status.ReadyReplicas)
		for i := int32(0); i < sts.Status.ReadyReplicas; i++ {
			endpoint := fmt.Sprintf("http://%s-%d.%s-peer.%s.svc.cluster.local:%d",
				cluster.Name, i, cluster.Name, cluster.Namespace, utils.EtcdClientPort)
			endpoints = append(endpoints, endpoint)
		}
		cluster.Status.ClientEndpoints = endpoints
	}

	// 更新最后更新时间
	now := metav1.Now()
	cluster.Status.LastUpdateTime = &now

	return r.Status().Update(ctx, cluster)
}

// updateMemberStatus updates the member status information
func (r *EtcdClusterReconciler) updateMemberStatus(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, sts *appsv1.StatefulSet) error {
	logger := log.FromContext(ctx)

	// 获取所有 Pod 的状态
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(cluster.Namespace), client.MatchingLabels{
		"app.kubernetes.io/name":     "etcd",
		"app.kubernetes.io/instance": cluster.Name,
	}); err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	// 构建成员状态列表
	members := make([]etcdv1alpha1.EtcdMember, 0, cluster.Spec.Size)

	for i := int32(0); i < cluster.Spec.Size; i++ {
		memberName := fmt.Sprintf("%s-%d", cluster.Name, i)
		member := etcdv1alpha1.EtcdMember{
			Name:      memberName,
			PeerURL:   fmt.Sprintf("http://%s.%s-peer.%s.svc.cluster.local:%d", memberName, cluster.Name, cluster.Namespace, utils.EtcdPeerPort),
			ClientURL: fmt.Sprintf("http://%s.%s-peer.%s.svc.cluster.local:%d", memberName, cluster.Name, cluster.Namespace, utils.EtcdClientPort),
			Ready:     false,
		}

		// 检查对应的 Pod 是否就绪
		for _, pod := range podList.Items {
			if pod.Name == memberName {
				// 检查 Pod 是否就绪
				for _, condition := range pod.Status.Conditions {
					if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
						member.Ready = true
						break
					}
				}
				break
			}
		}

		members = append(members, member)
	}

	cluster.Status.Members = members

	logger.V(1).Info("Updated member status", "members", len(members), "ready", func() int {
		count := 0
		for _, m := range members {
			if m.Ready {
				count++
			}
		}
		return count
	}())

	return nil
}

// needsScaling checks if the cluster needs scaling
func (r *EtcdClusterReconciler) needsScaling(cluster *etcdv1alpha1.EtcdCluster) bool {
	return cluster.Status.ReadyReplicas != cluster.Spec.Size
}

// performHealthCheck performs health check on the etcd cluster
func (r *EtcdClusterReconciler) performHealthCheck(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	// 基础健康检查：检查 StatefulSet 状态
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, sts)
	if err != nil {
		return fmt.Errorf("failed to get StatefulSet: %w", err)
	}

	// 检查是否有失败的副本
	if sts.Status.ReadyReplicas < sts.Status.Replicas {
		return fmt.Errorf("not all replicas are ready: %d/%d", sts.Status.ReadyReplicas, sts.Status.Replicas)
	}

	// 设置健康状态
	r.setCondition(cluster, utils.ConditionTypeReady, metav1.ConditionTrue, utils.ReasonHealthy, "Cluster is healthy")

	return nil
}

// handleScaleUp handles scaling up the cluster
func (r *EtcdClusterReconciler) handleScaleUp(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 获取当前 StatefulSet
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, sts); err != nil {
		return ctrl.Result{}, err
	}

	currentReplicas := *sts.Spec.Replicas
	desiredReplicas := cluster.Spec.Size

	logger.Info("Scaling up cluster", "current", currentReplicas, "desired", desiredReplicas)

	// 对于多节点集群，我们需要逐步添加成员
	if currentReplicas > 1 && desiredReplicas > currentReplicas {
		// 尝试使用 etcd 客户端添加成员
		if err := r.addEtcdMember(ctx, cluster, currentReplicas); err != nil {
			logger.Error(err, "Failed to add etcd member, falling back to StatefulSet scaling")
		}
	}

	// 更新 StatefulSet 副本数
	if currentReplicas < desiredReplicas {
		*sts.Spec.Replicas = desiredReplicas
		if err := r.Update(ctx, sts); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("Updated StatefulSet replicas", "replicas", desiredReplicas)
	}

	// 等待新副本就绪
	return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
}

// handleScaleDown handles scaling down the cluster
func (r *EtcdClusterReconciler) handleScaleDown(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	// 更新 StatefulSet 副本数
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, sts); err != nil {
		return ctrl.Result{}, err
	}

	if *sts.Spec.Replicas > cluster.Spec.Size {
		*sts.Spec.Replicas = cluster.Spec.Size
		if err := r.Update(ctx, sts); err != nil {
			return ctrl.Result{}, err
		}
	}

	// 等待副本缩减完成
	return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
}

// cleanupResources cleans up resources during deletion
func (r *EtcdClusterReconciler) cleanupResources(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	// 清理逻辑可以在这里实现
	// 例如：清理 PVC、备份等
	return nil
}

// addEtcdMember adds a new member to the etcd cluster
func (r *EtcdClusterReconciler) addEtcdMember(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, memberIndex int32) error {
	logger := log.FromContext(ctx)

	// 创建 etcd 客户端
	etcdClient, err := r.createEtcdClient(cluster)
	if err != nil {
		return fmt.Errorf("failed to create etcd client: %w", err)
	}
	defer etcdClient.Close()

	// 构建新成员的 peer URL
	memberName := fmt.Sprintf("%s-%d", cluster.Name, memberIndex)
	peerURL := fmt.Sprintf("http://%s.%s-peer.%s.svc.cluster.local:%d",
		memberName, cluster.Name, cluster.Namespace, utils.EtcdPeerPort)

	logger.Info("Adding etcd member", "name", memberName, "peerURL", peerURL)

	// 添加成员到 etcd 集群
	_, err = etcdClient.AddMember(ctx, peerURL)
	if err != nil {
		return fmt.Errorf("failed to add member %s: %w", memberName, err)
	}

	logger.Info("Successfully added etcd member", "name", memberName)
	return nil
}

// removeEtcdMember removes a member from the etcd cluster
func (r *EtcdClusterReconciler) removeEtcdMember(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, memberName string) error {
	logger := log.FromContext(ctx)

	// 创建 etcd 客户端
	etcdClient, err := r.createEtcdClient(cluster)
	if err != nil {
		return fmt.Errorf("failed to create etcd client: %w", err)
	}
	defer etcdClient.Close()

	// 获取集群成员列表
	members, err := etcdClient.GetClusterMembers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get cluster members: %w", err)
	}

	// 查找要移除的成员
	var memberID uint64
	found := false
	for _, member := range members {
		if member.Name == memberName {
			// 将十六进制字符串转换为 uint64
			if _, err := fmt.Sscanf(member.ID, "%x", &memberID); err != nil {
				return fmt.Errorf("failed to parse member ID %s: %w", member.ID, err)
			}
			found = true
			break
		}
	}

	if !found {
		logger.Info("Member not found in etcd cluster", "name", memberName)
		return nil // 成员已经不存在，认为成功
	}

	logger.Info("Removing etcd member", "name", memberName, "id", memberID)

	// 从 etcd 集群中移除成员
	if err := etcdClient.RemoveMember(ctx, memberID); err != nil {
		return fmt.Errorf("failed to remove member %s: %w", memberName, err)
	}

	logger.Info("Successfully removed etcd member", "name", memberName)
	return nil
}

// createEtcdClient creates an etcd client for the cluster
func (r *EtcdClusterReconciler) createEtcdClient(cluster *etcdv1alpha1.EtcdCluster) (*etcdclient.Client, error) {
	// 使用集群的客户端端点
	endpoints := cluster.Status.ClientEndpoints
	if len(endpoints) == 0 {
		// 如果状态中没有端点，构建默认端点
		endpoints = []string{
			fmt.Sprintf("http://%s-0.%s-peer.%s.svc.cluster.local:%d",
				cluster.Name, cluster.Name, cluster.Namespace, utils.EtcdClientPort),
		}
	}

	return etcdclient.NewClient(endpoints)
}

// SetupWithManager sets up the controller with the Manager.
func (r *EtcdClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&etcdv1alpha1.EtcdCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
