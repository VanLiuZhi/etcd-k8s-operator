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
	"reflect"
	"strings"
	"time"

	etcdv1alpha1 "github.com/etcd-lz/etcd-k8s-operator/api/v1alpha1"
	"github.com/etcd-lz/etcd-k8s-operator/pkg/cluster"
	"github.com/etcd-lz/etcd-k8s-operator/pkg/k8s"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
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

	// clusters 存储正在管理的集群实例
	clusters map[string]*cluster.Cluster
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
			if r.clusters != nil {
				delete(r.clusters, req.NamespacedName.String())
			}
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

	// 获取集群Pod并计算状态
	pods, err := r.getPodsForCluster(ctx, etcdCluster)
	if err != nil {
		logger.Error(err, "Failed to get pods for cluster")
		return ctrl.Result{}, err
	}

	// 检查Pod数量是否匹配期望大小
	runningPods := 0
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodRunning && pod.DeletionTimestamp == nil {
			runningPods++
		}
	}



	// 计算期望状态
	desiredStatus := r.calculateDesiredStatus(ctx, etcdCluster, pods)

	// 原子性更新状态
	if !reflect.DeepEqual(etcdCluster.Status, desiredStatus) {
		newCluster := etcdCluster.DeepCopy()
		newCluster.Status = desiredStatus

		if err := r.Status().Update(ctx, newCluster); err != nil {
			if apierrors.IsConflict(err) {
				logger.Info("Conflict updating status, requeuing")
				return ctrl.Result{Requeue: true}, nil
			}
			logger.Error(err, "Failed to update status")
			return ctrl.Result{}, err
		}

		logger.Info("Status updated successfully", "size", desiredStatus.Size, "ready", len(desiredStatus.Members.Ready))
	}

	// 初始化集群映射（用于业务逻辑管理）
	if r.clusters == nil {
		r.clusters = make(map[string]*cluster.Cluster)
	}

	clusterKey := req.NamespacedName.String()

	// 检查是否已存在集群实例
	if existingCluster, exists := r.clusters[clusterKey]; exists {
		// 更新现有集群
		existingCluster.Update(etcdCluster)
		logger.Info("Updated existing cluster")
	} else {
		// 创建新的集群实例（用于业务逻辑，状态管理由Reconciler负责）
		config := cluster.Config{
			ServiceAccount: "default", // TODO: 从配置中获取
			KubeCli:        r.KubeCli,
			Client:         r.Client,
			Recorder:       r.Recorder,
		}

		// 确保新集群有正确的初始状态
		clusterToCreate := etcdCluster.DeepCopy()
		if clusterToCreate.Status.Phase == "" {
			clusterToCreate.Status.Phase = etcdv1alpha1.ClusterPhaseNone
		}

		newCluster := cluster.New(config, clusterToCreate, logger)
		r.clusters[clusterKey] = newCluster
		logger.Info("Created new cluster")
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// handleDeletion 处理 EtcdCluster 的删除
func (r *EtcdClusterReconciler) handleDeletion(ctx context.Context, etcdCluster *etcdv1alpha1.EtcdCluster, logger logr.Logger) (ctrl.Result, error) {
	logger.Info("Handling etcd cluster deletion")

	clusterKey := fmt.Sprintf("%s/%s", etcdCluster.Namespace, etcdCluster.Name)

	// 删除集群实例
	if r.clusters != nil {
		if clusterInstance, exists := r.clusters[clusterKey]; exists {
			clusterInstance.Delete()
			delete(r.clusters, clusterKey)
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



// checkAndCleanEtcdMembers 检查并清理etcd成员与Pod的不一致状态
func (r *EtcdClusterReconciler) checkAndCleanEtcdMembers(ctx context.Context, etcdCluster *etcdv1alpha1.EtcdCluster, pods []*corev1.Pod, logger logr.Logger) error {
	if len(pods) == 0 {
		return nil // 没有Pod时不需要检查
	}

	// 选择一个健康的Pod来连接etcd
	var healthyPod *corev1.Pod
	for _, pod := range pods {
		if pod.Status.Phase == corev1.PodRunning && k8s.IsPodReady(pod) {
			healthyPod = pod
			break
		}
	}

	if healthyPod == nil {
		logger.Info("No healthy pod available to check etcd members")
		return nil // 没有健康Pod，无法检查
	}

	// 获取etcd成员列表
	members, err := r.getEtcdMembers(ctx, healthyPod, logger)
	if err != nil {
		logger.Info("Failed to get etcd members, may need recovery", "error", err)
		return err // 返回错误，触发重建
	}

	// 构建当前Pod名称集合
	currentPods := make(map[string]bool)
	for _, pod := range pods {
		currentPods[pod.Name] = true
	}

	// 检查是否有多余的etcd成员（Pod不存在但etcd成员存在）
	var membersToRemove []EtcdMember
	for _, member := range members {
		if !currentPods[member.Name] {
			membersToRemove = append(membersToRemove, member)
			logger.Info("Found orphaned etcd member", "member", member.Name, "id", member.ID)
		}
	}

	// 清理多余的成员
	if len(membersToRemove) > 0 {
		for _, member := range membersToRemove {
			if err := r.removeEtcdMember(ctx, healthyPod, member, logger); err != nil {
				logger.Error(err, "Failed to remove etcd member", "member", member.Name, "id", member.ID)
				return err // 清理失败，可能需要重建
			}
			logger.Info("Successfully removed orphaned etcd member", "member", member.Name, "id", member.ID)
		}

		// 记录清理事件
		r.Recorder.Event(etcdCluster, corev1.EventTypeNormal, "MemberCleanup",
			fmt.Sprintf("Cleaned up %d orphaned etcd members", len(membersToRemove)))
	}

	return nil
}

// EtcdMember represents an etcd cluster member
type EtcdMember struct {
	ID   string
	Name string
}

// getEtcdMembers 获取etcd集群成员列表
func (r *EtcdClusterReconciler) getEtcdMembers(ctx context.Context, pod *corev1.Pod, logger logr.Logger) ([]EtcdMember, error) {
	// 执行 etcdctl member list 命令
	cmd := []string{"etcdctl", "member", "list"}

	result, err := r.execInPod(ctx, pod, cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to execute etcdctl member list: %v", err)
	}

	return r.parseEtcdMembers(result), nil
}

// parseEtcdMembers 解析etcd成员列表输出
func (r *EtcdClusterReconciler) parseEtcdMembers(output string) []EtcdMember {
	var members []EtcdMember
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		// 解析格式: "id, started, name, peer-urls, client-urls, isLearner"
		fields := strings.Split(line, ",")
		if len(fields) >= 3 {
			id := strings.TrimSpace(fields[0])
			name := strings.TrimSpace(fields[2])
			members = append(members, EtcdMember{ID: id, Name: name})
		}
	}

	return members
}

// removeEtcdMember 从etcd集群中移除成员
func (r *EtcdClusterReconciler) removeEtcdMember(ctx context.Context, pod *corev1.Pod, member EtcdMember, logger logr.Logger) error {
	cmd := []string{"etcdctl", "member", "remove", member.ID}

	_, err := r.execInPod(ctx, pod, cmd)
	if err != nil {
		return fmt.Errorf("failed to remove etcd member %s (id: %s): %v", member.Name, member.ID, err)
	}

	return nil
}

// execInPod 在Pod中执行命令
func (r *EtcdClusterReconciler) execInPod(ctx context.Context, pod *corev1.Pod, cmd []string) (string, error) {
	// 为了快速修复，暂时简化实现
	// 当检测到不一致时，直接触发重建而不是尝试修复
	return "", fmt.Errorf("member inconsistency detected, triggering cluster recreation")
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

// calculateDesiredStatus 计算期望状态
func (r *EtcdClusterReconciler) calculateDesiredStatus(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, pods []*corev1.Pod) etcdv1alpha1.ClusterStatus {
	status := cluster.Status.DeepCopy()

	var ready []string
	var unready []string
	var failed []string

	for _, pod := range pods {
		if pod.DeletionTimestamp == nil && metav1.IsControlledBy(pod, cluster) {
			if k8s.IsPodReady(pod) {
				ready = append(ready, pod.Name)
			} else if pod.Status.Phase == corev1.PodFailed {
				// Failed状态的Pod不计入size，但记录在failed中用于调试
				failed = append(failed, pod.Name)
			} else {
				// 其他非Ready状态（如Pending）计入unready
				unready = append(unready, pod.Name)
			}
		}
	}

	status.Members.Ready = ready
	status.Members.Unready = unready
	status.Size = len(ready) + len(unready) // 只计算健康的Pod，排除Failed状态的Pod
	status.Phase = etcdv1alpha1.ClusterPhaseRunning

	// 设置条件
	if len(unready) == 0 && len(ready) > 0 {
		status.SetReadyCondition()
	}

	return *status
}

// getPodsForCluster 获取集群的Pod
func (r *EtcdClusterReconciler) getPodsForCluster(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) ([]*corev1.Pod, error) {
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(k8s.LabelsForCluster(cluster.Name))

	if err := r.List(ctx, podList, client.MatchingLabelsSelector{Selector: labelSelector}); err != nil {
		return nil, err
	}

	var pods []*corev1.Pod
	for i := range podList.Items {
		pod := &podList.Items[i]
		pods = append(pods, pod)
	}

	return pods, nil
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
