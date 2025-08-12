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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	clientpkg "github.com/your-org/etcd-k8s-operator/pkg/client"
	"github.com/your-org/etcd-k8s-operator/pkg/resource"
	"github.com/your-org/etcd-k8s-operator/pkg/service"
	"github.com/your-org/etcd-k8s-operator/pkg/utils"
)

// ClusterController 集群控制器 (重构后的简化版本)
type ClusterController struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	// 服务层依赖
	clusterService service.ClusterService
	scalingService service.ScalingService
	healthService  service.HealthService
}

// NewClusterController 创建集群控制器
func NewClusterController(
	client client.Client,
	scheme *runtime.Scheme,
	recorder record.EventRecorder,
) *ClusterController {
	// 创建客户端层
	k8sClient := clientpkg.NewKubernetesClient(client, recorder)

	// 创建资源层
	resourceManager := resource.NewResourceManager(k8sClient)

	// 创建服务层
	clusterService := service.NewClusterService(k8sClient, resourceManager)
	scalingService := service.NewScalingService(client, resourceManager)
	// TODO: 创建其他服务

	return &ClusterController{
		Client:   client,
		Scheme:   scheme,
		Recorder: recorder,

		clusterService: clusterService,
		scalingService: scalingService,
		// healthService:  healthService,
	}
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

// Reconcile 主要的调谐逻辑 (大幅简化)
func (r *ClusterController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
	r.clusterService.SetDefaults(cluster)

	// 5. 状态机处理 (委托给服务层)
	return r.handleStateMachine(ctx, cluster)
}

// handleStateMachine 处理状态机 (简化版)
func (r *ClusterController) handleStateMachine(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	switch cluster.Status.Phase {
	case "":
		logger.Info("Initializing EtcdCluster")
		return r.clusterService.InitializeCluster(ctx, cluster)

	case etcdv1alpha1.EtcdClusterPhaseCreating:
		logger.Info("Creating EtcdCluster")
		return r.clusterService.CreateCluster(ctx, cluster)

	case etcdv1alpha1.EtcdClusterPhaseRunning:
		logger.Info("CLUSTER-CONTROLLER-DEBUG: EtcdCluster is running, performing health check", "DEBUG_VERSION", "cluster-controller-v1", "LINE", 144)
		logger.Info("CLUSTER-CONTROLLER-DEBUG: About to check scalingService", "DEBUG_VERSION", "cluster-controller-v1", "LINE", 145, "scalingService", r.scalingService)
		if r.scalingService != nil {
			logger.Info("CLUSTER-CONTROLLER-DEBUG: Calling scalingService.HandleRunning", "DEBUG_VERSION", "cluster-controller-v1", "LINE", 146)
			result, err := r.scalingService.HandleRunning(ctx, cluster)
			logger.Info("CLUSTER-CONTROLLER-DEBUG: scalingService.HandleRunning returned", "DEBUG_VERSION", "cluster-controller-v1", "LINE", 147, "result", result, "error", err)
			return result, err
		}
		// 临时处理，直到实现 scalingService
		logger.Info("CLUSTER-CONTROLLER-DEBUG: scalingService is nil, returning requeue", "DEBUG_VERSION", "cluster-controller-v1", "LINE", 149)
		return ctrl.Result{RequeueAfter: 30 * 1000000000}, nil // 30秒

	case etcdv1alpha1.EtcdClusterPhaseScaling:
		logger.Info("Scaling EtcdCluster")
		if r.scalingService != nil {
			return r.scalingService.HandleScaling(ctx, cluster)
		}
		return ctrl.Result{RequeueAfter: 30 * 1000000000}, nil

	case etcdv1alpha1.EtcdClusterPhaseStopped:
		logger.Info("EtcdCluster is stopped, checking if restart needed")
		if r.scalingService != nil {
			return r.scalingService.HandleStopped(ctx, cluster)
		}
		return ctrl.Result{RequeueAfter: 30 * 1000000000}, nil

	case etcdv1alpha1.EtcdClusterPhaseFailed:
		logger.Info("EtcdCluster has failed, attempting recovery")
		if r.healthService != nil {
			return r.healthService.HandleFailed(ctx, cluster)
		}
		return ctrl.Result{RequeueAfter: 30 * 1000000000}, nil

	default:
		logger.Info("Unknown phase, resetting to initialization", "phase", cluster.Status.Phase)
		return r.clusterService.InitializeCluster(ctx, cluster)
	}
}

// handleDeletion 处理删除
func (r *ClusterController) handleDeletion(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(cluster, utils.EtcdFinalizer) {
		// 执行清理逻辑
		result, err := r.clusterService.DeleteCluster(ctx, cluster)
		if err != nil {
			return result, err
		}

		// 移除 finalizer
		controllerutil.RemoveFinalizer(cluster, utils.EtcdFinalizer)
		if err := r.Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager 设置控制器管理器
func (r *ClusterController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&etcdv1alpha1.EtcdCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
