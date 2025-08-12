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
	"os"
	"strings"
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
// DEPRECATED: 使用 ClusterController 替代
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
	logger.Info("RECONCILE-FUNCTION-TEST-12345-UNIQUE-STRING")
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
		logger.Info("DOCKER-BUILD-TEST-12345-UNIQUE-STRING", "DEBUG_VERSION", "docker-build-test", "LINE", 109)
		return r.handleRunning(ctx, cluster)
	case etcdv1alpha1.EtcdClusterPhaseScaling:
		logger.Info("Scaling EtcdCluster")
		return r.handleScaling(ctx, cluster)
	case etcdv1alpha1.EtcdClusterPhaseStopped:
		logger.Info("EtcdCluster is stopped, checking if restart needed")
		return r.handleStopped(ctx, cluster)
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
	// 不再强制设置Size默认值，允许size=0用于集群删除
	// 只有在创建新集群时才设置默认值
	if cluster.Spec.Size == 0 && cluster.Status.Phase == "" {
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

	// 调试：检查动态扩容条件
	logger.Info("Checking dynamic expansion conditions",
		"readyReplicas >= 1", readyReplicas >= 1,
		"currentReplicas > readyReplicas", currentReplicas > readyReplicas,
		"desiredSize > 1", desiredSize > 1)

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

	if readyReplicas >= 1 && currentReplicas < desiredSize {
		// 第一个节点已就绪，且当前副本数小于期望副本数，开始动态扩容
		logger.Info("Starting dynamic expansion", "ready", readyReplicas, "current", currentReplicas, "desired", desiredSize)

		// 使用动态扩容逻辑添加其他节点
		return r.handleDynamicExpansion(ctx, cluster, sts)
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

// handleDynamicExpansion handles expanding from single-node to multi-node cluster
func (r *EtcdClusterReconciler) handleDynamicExpansion(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, sts *appsv1.StatefulSet) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	currentSize := *sts.Spec.Replicas
	desiredSize := cluster.Spec.Size

	logger.Info("Starting dynamic expansion", "currentSize", currentSize, "desiredSize", desiredSize)

	// 逐步扩展，每次添加一个节点
	nextMemberIndex := currentSize
	nextMemberName := fmt.Sprintf("%s-%d", cluster.Name, nextMemberIndex)

	logger.Info("Adding new etcd member", "memberIndex", nextMemberIndex, "memberName", nextMemberName)

	// 步骤 1: 先通过 etcd API 添加成员
	if err := r.addEtcdMember(ctx, cluster, nextMemberIndex); err != nil {
		logger.Error(err, "Failed to add etcd member", "memberIndex", nextMemberIndex)
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	logger.Info("Etcd member added successfully", "memberName", nextMemberName)

	// 步骤 2: 增加 StatefulSet 副本数，让 Kubernetes 创建新 Pod
	nextSize := currentSize + 1
	*sts.Spec.Replicas = nextSize
	if err := r.Update(ctx, sts); err != nil {
		logger.Error(err, "Failed to update StatefulSet replicas")
		return ctrl.Result{}, err
	}

	logger.Info("StatefulSet replicas updated", "from", currentSize, "to", nextSize)

	// 如果还没有达到目标大小，继续扩展
	if nextSize < desiredSize {
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	// 达到目标大小，完成扩展
	logger.Info("Dynamic expansion completed", "finalSize", nextSize)
	return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
}

// handleRunning handles the running phase
func (r *EtcdClusterReconciler) handleRunning(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// CRITICAL DEBUG: This should be the first line executed in handleRunning
	logger.Info("CRITICAL DEBUG: handleRunning function started!!!")

	// 1. 先更新集群状态，确保ReadyReplicas是最新的
	logger.Info("DEBUG: About to call updateClusterStatus")
	if err := r.updateClusterStatus(ctx, cluster); err != nil {
		logger.Error(err, "DEBUG: updateClusterStatus failed")
		return ctrl.Result{}, err
	}
	logger.Info("DEBUG: updateClusterStatus completed successfully")

	// 2. 检查是否需要扩缩容
	logger.Info("Checking scaling needs", "currentReadyReplicas", cluster.Status.ReadyReplicas, "desiredSize", cluster.Spec.Size)
	logger.Info("DEBUG: Status after updateClusterStatus", "readyReplicas", cluster.Status.ReadyReplicas, "phase", cluster.Status.Phase)
	if r.needsScaling(cluster) {
		logger.Info("Cluster needs scaling", "current", cluster.Status.ReadyReplicas, "desired", cluster.Spec.Size)
		cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseScaling
		r.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionTrue, utils.ReasonScaling, "Scaling etcd cluster")

		if err := r.Status().Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 3. 执行健康检查
	if err := r.performHealthCheck(ctx, cluster); err != nil {
		logger.Error(err, "Health check failed")
		return r.updateStatusWithError(ctx, cluster, etcdv1alpha1.EtcdClusterPhaseFailed, err)
	}

	// 4. 确保状态已保存（修复：确保ReadyReplicas字段被正确保存）
	if err := r.Status().Update(ctx, cluster); err != nil {
		logger.Error(err, "Failed to update cluster status")
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

	// 特殊处理：缩容到0 (停止集群)
	if desiredSize == 0 {
		logger.Info("Scaling down cluster to zero (stopping cluster)", "from", currentSize)
		return r.handleScaleToZero(ctx, cluster)
	}

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
	// 允许size=0用于集群删除/停止
	if cluster.Spec.Size == 0 {
		return nil
	}

	// 验证集群大小必须是奇数 (size > 0时)
	if cluster.Spec.Size%2 == 0 {
		return fmt.Errorf("cluster size must be odd number, got %d", cluster.Spec.Size)
	}

	// 验证集群大小范围 (允许0-9)
	if cluster.Spec.Size < 0 || cluster.Spec.Size > 9 {
		return fmt.Errorf("cluster size must be between 0 and 9, got %d", cluster.Spec.Size)
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

	// 确保 NodePort 服务（用于 operator 外部访问）
	nodePortService := k8s.BuildNodePortService(cluster)
	if err := r.ensureService(ctx, cluster, nodePortService); err != nil {
		return fmt.Errorf("failed to ensure nodeport service: %w", err)
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
	// 对于多节点集群，使用分阶段启动策略
	var desired *appsv1.StatefulSet
	if cluster.Spec.Size > 1 {
		// 多节点集群：初始创建时只启动第一个节点
		desired = k8s.BuildStatefulSetWithReplicas(cluster, 1)
	} else {
		// 单节点集群：直接创建
		desired = k8s.BuildStatefulSet(cluster)
	}

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
		err := r.Create(ctx, desired)
		if err != nil && errors.IsAlreadyExists(err) {
			// 如果资源已存在，这可能是并发创建导致的，重新获取资源
			if getErr := r.Get(ctx, types.NamespacedName{
				Name:      desired.Name,
				Namespace: desired.Namespace,
			}, existing); getErr != nil {
				return getErr
			}
			// 继续执行更新检查逻辑
		} else if err != nil {
			return err
		} else {
			// 创建成功，直接返回
			return nil
		}
	} else if err != nil {
		return err
	}

	// StatefulSet 已存在，对于单节点集群检查是否需要更新
	if cluster.Spec.Size == 1 {
		if r.statefulSetNeedsUpdate(existing, desired) {
			existing.Spec = desired.Spec
			return r.Update(ctx, existing)
		}
	}
	// 对于多节点集群，副本数的更新由 handleMultiNodeClusterCreation 处理
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

	// 添加调试日志
	logger := log.FromContext(ctx)
	logger.Info("DEBUG: About to update status",
		"readyReplicas", cluster.Status.ReadyReplicas,
		"phase", cluster.Status.Phase,
		"clientEndpoints", cluster.Status.ClientEndpoints)

	err = r.Status().Update(ctx, cluster)
	if err != nil {
		logger.Error(err, "Failed to update cluster status")
		return err
	}

	logger.Info("DEBUG: Status updated successfully", "readyReplicas", cluster.Status.ReadyReplicas)
	return nil
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

	// 构建成员状态列表 - 基于实际存在的Pod而不是期望的Size
	members := make([]etcdv1alpha1.EtcdMember, 0, len(podList.Items))

	// 遍历实际存在的Pod来构建成员状态
	for _, pod := range podList.Items {
		member := etcdv1alpha1.EtcdMember{
			Name:      pod.Name,
			PeerURL:   fmt.Sprintf("http://%s.%s-peer.%s.svc.cluster.local:%d", pod.Name, cluster.Name, cluster.Namespace, utils.EtcdPeerPort),
			ClientURL: fmt.Sprintf("http://%s.%s-peer.%s.svc.cluster.local:%d", pod.Name, cluster.Name, cluster.Namespace, utils.EtcdClientPort),
			Ready:     false,
		}

		// 检查 Pod 是否就绪
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				member.Ready = true
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
	logger := log.FromContext(ctx)

	// 基础健康检查：检查 StatefulSet 状态
	sts := &appsv1.StatefulSet{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, sts)
	if err != nil {
		return fmt.Errorf("failed to get StatefulSet: %w", err)
	}

	logger.Info("Health check status",
		"readyReplicas", sts.Status.ReadyReplicas,
		"replicas", sts.Status.Replicas,
		"desiredSize", cluster.Spec.Size)

	// 修复：只有当没有任何就绪副本时才认为是失败
	// 扩缩容过程中ReadyReplicas < Replicas是正常的
	if sts.Status.ReadyReplicas == 0 && sts.Status.Replicas > 0 {
		return fmt.Errorf("no replicas are ready: %d/%d", sts.Status.ReadyReplicas, sts.Status.Replicas)
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

	readyReplicas := sts.Status.ReadyReplicas

	logger.Info("Scale up status check",
		"currentReplicas", currentReplicas,
		"readyReplicas", readyReplicas,
		"desiredReplicas", desiredReplicas)

	// 动态 Pod 管理策略：
	// 1. 确保第一个节点就绪
	// 2. 逐个添加后续节点（先添加 etcd 成员，再创建 Pod）

	if readyReplicas == 0 {
		logger.Info("Waiting for first node to be ready")
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	// 如果当前副本数小于期望副本数，需要添加新节点
	if currentReplicas < desiredReplicas {
		nextNodeIndex := currentReplicas
		nextNodeName := fmt.Sprintf("%s-%d", cluster.Name, nextNodeIndex)

		logger.Info("Adding new etcd node", "nodeIndex", nextNodeIndex, "nodeName", nextNodeName)

		// 步骤 1: 先通过 etcd API 添加成员
		if err := r.addEtcdMember(ctx, cluster, nextNodeIndex); err != nil {
			logger.Error(err, "Failed to add etcd member", "memberIndex", nextNodeIndex)
			return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
		}

		logger.Info("Etcd member added successfully", "memberName", nextNodeName)

		// 步骤 2: 为新节点创建专用配置
		if err := r.createNodeConfigMap(ctx, cluster, nextNodeIndex); err != nil {
			logger.Error(err, "Failed to create node config", "nodeIndex", nextNodeIndex)
			return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
		}

		// 步骤 3: 增加 StatefulSet 副本数，让 Kubernetes 创建新 Pod
		nextReplicas := currentReplicas + 1
		*sts.Spec.Replicas = nextReplicas
		if err := r.Update(ctx, sts); err != nil {
			return ctrl.Result{}, err
		}

		logger.Info("Updated StatefulSet replicas", "from", currentReplicas, "to", nextReplicas)
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	// 等待新副本就绪
	return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
}

// handleScaleDown handles scaling down the cluster
func (r *EtcdClusterReconciler) handleScaleDown(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
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

	logger.Info("Scaling down cluster", "current", currentReplicas, "desired", desiredReplicas)

	if currentReplicas <= desiredReplicas {
		// 已经达到目标大小
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	// 需要移除的节点数量
	nodesToRemove := currentReplicas - desiredReplicas

	// 逐个移除节点（从最高索引开始）
	for i := int32(0); i < nodesToRemove; i++ {
		nodeIndex := currentReplicas - 1 - i
		nodeName := fmt.Sprintf("%s-%d", cluster.Name, nodeIndex)

		logger.Info("Removing etcd member", "nodeIndex", nodeIndex, "nodeName", nodeName)

		// 步骤 1: 从 etcd 集群中移除成员
		if err := r.removeEtcdMember(ctx, cluster, nodeName); err != nil {
			logger.Error(err, "Failed to remove etcd member", "nodeName", nodeName)
			return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
		}

		logger.Info("Successfully removed etcd member", "nodeName", nodeName)
	}

	// 步骤 2: 更新 StatefulSet 副本数
	*sts.Spec.Replicas = desiredReplicas
	if err := r.Update(ctx, sts); err != nil {
		logger.Error(err, "Failed to update StatefulSet replicas")
		return ctrl.Result{}, err
	}

	logger.Info("Updated StatefulSet replicas", "from", currentReplicas, "to", desiredReplicas)

	// 步骤 3: 清理多余的 PVC (关键修复)
	if err := r.cleanupExtraPVCs(ctx, cluster, desiredReplicas, currentReplicas); err != nil {
		logger.Error(err, "Failed to cleanup extra PVCs")
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	// 步骤 4: 如果缩容到单节点，重置集群状态 (关键修复)
	if desiredReplicas == 1 {
		if err := r.resetSingleNodeCluster(ctx, cluster); err != nil {
			logger.Error(err, "Failed to reset single node cluster")
			return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
		}
	}

	// 等待副本缩减完成
	return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
}

// handleScaleToZero handles scaling down the cluster to zero (stopping cluster)
func (r *EtcdClusterReconciler) handleScaleToZero(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Scaling cluster to zero - stopping all etcd instances")

	// 获取当前 StatefulSet
	sts := &appsv1.StatefulSet{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, sts); err != nil {
		if errors.IsNotFound(err) {
			// StatefulSet已经不存在，直接更新状态
			return r.updateStatusAfterScaleToZero(ctx, cluster)
		}
		return ctrl.Result{}, err
	}

	currentReplicas := *sts.Spec.Replicas

	// 如果已经是0副本，检查是否完成
	if currentReplicas == 0 {
		if sts.Status.Replicas == 0 {
			logger.Info("Scale to zero completed")
			return r.updateStatusAfterScaleToZero(ctx, cluster)
		}
		// 等待Pod终止完成
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	// 设置StatefulSet副本数为0
	logger.Info("Setting StatefulSet replicas to zero", "current", currentReplicas)
	*sts.Spec.Replicas = 0
	if err := r.Update(ctx, sts); err != nil {
		logger.Error(err, "Failed to update StatefulSet replicas to zero")
		return ctrl.Result{}, err
	}

	// 等待Pod终止
	return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
}

// updateStatusAfterScaleToZero updates cluster status after scaling to zero
func (r *EtcdClusterReconciler) updateStatusAfterScaleToZero(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 更新集群状态为Stopped
	cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseStopped
	cluster.Status.ReadyReplicas = 0
	cluster.Status.Members = []etcdv1alpha1.EtcdMember{}
	cluster.Status.LeaderID = ""
	cluster.Status.ClusterID = ""
	cluster.Status.ClientEndpoints = []string{}

	r.setCondition(cluster, utils.ConditionTypeReady, metav1.ConditionFalse, utils.ReasonStopped, "Cluster scaled to zero")
	r.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionFalse, utils.ReasonStopped, "Cluster stopped")

	if err := r.Status().Update(ctx, cluster); err != nil {
		return ctrl.Result{}, err
	}

	r.Recorder.Event(cluster, corev1.EventTypeNormal, utils.EventReasonClusterStopped, "Cluster scaled to zero and stopped")
	logger.Info("Cluster successfully scaled to zero and stopped")

	// 停止状态下不需要频繁检查
	return ctrl.Result{RequeueAfter: utils.DefaultHealthCheckInterval * 2}, nil
}

// handleStopped handles the stopped phase (size=0)
func (r *EtcdClusterReconciler) handleStopped(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 如果用户将size从0改为>0，需要重新启动集群
	if cluster.Spec.Size > 0 {
		logger.Info("Restarting cluster from stopped state", "desiredSize", cluster.Spec.Size)

		// 转换到创建状态重新启动集群
		cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseCreating
		r.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionTrue, utils.ReasonCreating, "Restarting cluster from stopped state")

		if err := r.Status().Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}

		r.Recorder.Event(cluster, corev1.EventTypeNormal, utils.EventReasonClusterCreated, "Restarting cluster from stopped state")
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	// 确保所有PVC都被清理（停止状态下不应该有任何PVC）
	if err := r.cleanupAllPVCs(ctx, cluster); err != nil {
		logger.Error(err, "Failed to cleanup PVCs in stopped state")
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, err
	}

	// 保持停止状态，较少频率检查
	return ctrl.Result{RequeueAfter: utils.DefaultHealthCheckInterval * 3}, nil
}

// cleanupExtraPVCs cleans up extra PVCs after scaling down
func (r *EtcdClusterReconciler) cleanupExtraPVCs(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, desiredReplicas, currentReplicas int32) error {
	logger := log.FromContext(ctx)

	// 删除多余的PVC，让StorageClass的Delete策略生效
	for i := desiredReplicas; i < currentReplicas; i++ {
		pvcName := fmt.Sprintf("data-%s-%d", cluster.Name, i)

		logger.Info("Deleting extra PVC", "pvcName", pvcName)

		pvc := &corev1.PersistentVolumeClaim{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      pvcName,
			Namespace: cluster.Namespace,
		}, pvc)

		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info("PVC already deleted", "pvcName", pvcName)
				continue
			}
			return fmt.Errorf("failed to get PVC %s: %w", pvcName, err)
		}

		// 删除PVC，StorageClass的Delete策略会自动删除PV
		if err := r.Delete(ctx, pvc); err != nil {
			return fmt.Errorf("failed to delete PVC %s: %w", pvcName, err)
		}

		logger.Info("Successfully deleted PVC", "pvcName", pvcName)
	}

	return nil
}

// resetSingleNodeCluster resets single node cluster state after scaling down
func (r *EtcdClusterReconciler) resetSingleNodeCluster(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	logger := log.FromContext(ctx)

	logger.Info("Resetting single node cluster state")

	// 对于单节点集群，我们需要确保etcd以单节点模式运行
	// 这里可以通过重启Pod来实现，让etcd重新初始化

	// 获取单节点Pod
	podName := fmt.Sprintf("%s-0", cluster.Name)
	pod := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      podName,
		Namespace: cluster.Namespace,
	}, pod)

	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Single node pod not found, skipping reset", "podName", podName)
			return nil
		}
		return fmt.Errorf("failed to get single node pod: %w", err)
	}

	// 删除Pod，让StatefulSet重新创建，这样etcd会以单节点模式启动
	logger.Info("Deleting single node pod to reset cluster state", "podName", podName)
	if err := r.Delete(ctx, pod); err != nil {
		return fmt.Errorf("failed to delete single node pod: %w", err)
	}

	logger.Info("Successfully triggered single node cluster reset")
	return nil
}

// cleanupResources cleans up resources during deletion
func (r *EtcdClusterReconciler) cleanupResources(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	// 清理逻辑可以在这里实现
	// 例如：清理 PVC、备份等
	return nil
}

// cleanupAllPVCs cleans up all PVCs for the cluster (used when size=0)
func (r *EtcdClusterReconciler) cleanupAllPVCs(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	logger := log.FromContext(ctx)

	// 列出所有相关的PVC
	pvcList := &corev1.PersistentVolumeClaimList{}
	listOpts := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{
			"app.kubernetes.io/instance": cluster.Name,
			"app.kubernetes.io/name":     "etcd",
		},
	}

	if err := r.List(ctx, pvcList, listOpts...); err != nil {
		return fmt.Errorf("failed to list PVCs: %w", err)
	}

	// 删除所有找到的PVC
	for _, pvc := range pvcList.Items {
		logger.Info("Deleting PVC for stopped cluster", "pvc", pvc.Name)
		if err := r.Delete(ctx, &pvc); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Failed to delete PVC", "pvc", pvc.Name)
			return fmt.Errorf("failed to delete PVC %s: %w", pvc.Name, err)
		}
	}

	if len(pvcList.Items) > 0 {
		logger.Info("Cleaned up PVCs for stopped cluster", "count", len(pvcList.Items))
	}

	return nil
}

// addEtcdMember adds a new member to the etcd cluster
func (r *EtcdClusterReconciler) addEtcdMember(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, memberIndex int32) error {
	logger := log.FromContext(ctx)

	// 创建 etcd 客户端
	logger.Info("Creating etcd client for member addition")
	etcdClient, err := r.createEtcdClient(cluster)
	if err != nil {
		logger.Error(err, "Failed to create etcd client")
		return fmt.Errorf("failed to create etcd client: %w", err)
	}
	defer etcdClient.Close()

	// 构建新成员的信息
	memberName := fmt.Sprintf("%s-%d", cluster.Name, memberIndex)
	peerURL := fmt.Sprintf("http://%s.%s-peer.%s.svc.cluster.local:%d",
		memberName, cluster.Name, cluster.Namespace, utils.EtcdPeerPort)

	logger.Info("Checking if etcd member already exists", "name", memberName, "peerURL", peerURL)

	// 检查成员是否已经存在
	members, err := etcdClient.GetClusterMembers(ctx)
	if err != nil {
		logger.Error(err, "Failed to get cluster members")
		return fmt.Errorf("failed to get cluster members: %w", err)
	}

	// 检查成员是否已经存在
	for _, member := range members {
		if member.Name == memberName {
			logger.Info("Etcd member already exists, skipping addition", "name", memberName)
			return nil
		}
	}

	logger.Info("Adding etcd member", "name", memberName, "peerURL", peerURL)

	// 添加成员到 etcd 集群
	resp, err := etcdClient.AddMember(ctx, peerURL)
	if err != nil {
		logger.Error(err, "Failed to add etcd member", "name", memberName, "peerURL", peerURL)
		return fmt.Errorf("failed to add member %s: %w", memberName, err)
	}

	logger.Info("Successfully added etcd member", "name", memberName, "memberID", resp.Member.ID)
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
	var endpoints []string

	// 检查是否运行在集群内部
	inCluster := r.isRunningInCluster()
	log.Log.Info("Checking cluster environment", "inCluster", inCluster)

	if inCluster {
		// 运行在集群内部，使用集群内端点
		endpoints = cluster.Status.ClientEndpoints
		if len(endpoints) == 0 {
			endpoints = []string{
				fmt.Sprintf("http://%s-0.%s-peer.%s.svc.cluster.local:%d",
					cluster.Name, cluster.Name, cluster.Namespace, utils.EtcdClientPort),
			}
		}
	} else {
		// 运行在集群外部，使用 NodePort 服务连接到第一个节点
		// 获取 Kind 集群的节点 IP
		nodeIP, err := r.getKindNodeIP()
		if err != nil {
			log.Log.Error(err, "Failed to get Kind node IP, falling back to localhost")
			nodeIP = "localhost"
		}
		endpoints = []string{fmt.Sprintf("http://%s:30379", nodeIP)}
	}

	log.Log.Info("Creating etcd client", "endpoints", endpoints)
	return etcdclient.NewClient(endpoints)
}

// getKindNodeIP gets the IP address of the Kind cluster node
func (r *EtcdClusterReconciler) getKindNodeIP() (string, error) {
	// 获取集群中的节点列表
	nodes := &corev1.NodeList{}
	if err := r.List(context.Background(), nodes); err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		return "", fmt.Errorf("no nodes found in cluster")
	}

	// 获取第一个节点的 IP 地址
	node := nodes.Items[0]
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address, nil
		}
	}

	return "", fmt.Errorf("no internal IP found for node %s", node.Name)
}

// createNodeConfigMap creates a ConfigMap with configuration for a specific node
func (r *EtcdClusterReconciler) createNodeConfigMap(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, nodeIndex int32) error {
	logger := log.FromContext(ctx)

	nodeName := fmt.Sprintf("%s-%d", cluster.Name, nodeIndex)
	configMapName := fmt.Sprintf("%s-node-%d-config", cluster.Name, nodeIndex)

	// 为新节点生成配置
	initialClusterState := "existing"
	initialCluster := r.buildInitialClusterForExistingNode(cluster, nodeIndex)

	configData := map[string]string{
		"ETCD_INITIAL_CLUSTER_STATE": initialClusterState,
		"ETCD_INITIAL_CLUSTER":       initialCluster,
		"ETCD_NAME":                  nodeName,
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "etcd",
				"app.kubernetes.io/instance":   cluster.Name,
				"app.kubernetes.io/component":  "etcd",
				"app.kubernetes.io/managed-by": "etcd-operator",
				"etcd.etcd.io/cluster-name":    cluster.Name,
				"etcd.etcd.io/node-index":      fmt.Sprintf("%d", nodeIndex),
			},
		},
		Data: configData,
	}

	// 设置 owner reference
	if err := ctrl.SetControllerReference(cluster, configMap, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	// 创建 ConfigMap
	if err := r.Create(ctx, configMap); err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create node config map: %w", err)
		}
		logger.Info("Node config map already exists", "configMap", configMapName)
	} else {
		logger.Info("Created node config map", "configMap", configMapName, "nodeIndex", nodeIndex)
	}

	return nil
}

// buildInitialClusterForExistingNode builds the initial cluster configuration for a node joining an existing cluster
func (r *EtcdClusterReconciler) buildInitialClusterForExistingNode(cluster *etcdv1alpha1.EtcdCluster, nodeIndex int32) string {
	var members []string

	// 包含所有现有节点和当前节点
	for i := int32(0); i <= nodeIndex; i++ {
		memberName := fmt.Sprintf("%s-%d", cluster.Name, i)
		memberURL := fmt.Sprintf("http://%s.%s-peer.%s.svc.cluster.local:%d",
			memberName, cluster.Name, cluster.Namespace, utils.EtcdPeerPort)
		members = append(members, fmt.Sprintf("%s=%s", memberName, memberURL))
	}

	return strings.Join(members, ",")
}

// isRunningInCluster checks if the operator is running inside the cluster
func (r *EtcdClusterReconciler) isRunningInCluster() bool {
	// 检查是否存在 service account token 文件
	_, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token")
	return err == nil
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
