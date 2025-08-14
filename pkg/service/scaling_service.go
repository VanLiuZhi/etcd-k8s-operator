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

package service

import (
	"context"
	"fmt"
	"os"
	"strings"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	etcdclient "github.com/your-org/etcd-k8s-operator/pkg/etcd"
	"github.com/your-org/etcd-k8s-operator/pkg/resource"
	"github.com/your-org/etcd-k8s-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// scalingService 扩缩容服务实现
type scalingService struct {
	k8sClient       client.Client
	resourceManager resource.ResourceManager
}

// NewScalingService 创建扩缩容服务
func NewScalingService(k8sClient client.Client, resourceManager resource.ResourceManager) ScalingService {
	return &scalingService{
		k8sClient:       k8sClient,
		resourceManager: resourceManager,
	}
}

// HandleRunning 处理运行状态的集群
func (s *scalingService) HandleRunning(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. 更新集群状态，确保ReadyReplicas是最新的
	status, err := s.resourceManager.StatefulSet().GetStatus(ctx, cluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	cluster.Status.ReadyReplicas = status.ReadyReplicas

	// 2. 检查是否需要扩缩容
	logger.Info("Checking scaling needs", "currentReadyReplicas", cluster.Status.ReadyReplicas, "desiredSize", cluster.Spec.Size)
	if s.NeedsScaling(cluster) {
		logger.Info("Cluster needs scaling", "current", cluster.Status.ReadyReplicas, "desired", cluster.Spec.Size)
		cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseScaling
		s.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionTrue, utils.ReasonScaling, "Scaling etcd cluster")

		if err := s.k8sClient.Status().Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 3. 执行健康检查
	logger.Info("Performing health check")
	return ctrl.Result{RequeueAfter: utils.DefaultHealthCheckInterval}, nil
}

// HandleScaling 处理扩缩容状态的集群
func (s *scalingService) HandleScaling(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. 首先确保基础资源存在（修复：在扩缩容前确保Service等资源存在）
	if err := s.resourceManager.EnsureAllResources(ctx, cluster); err != nil {
		logger.Error(err, "Failed to ensure resources during scaling")
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	// 2. 获取最新的StatefulSet状态
	sts := &appsv1.StatefulSet{}
	err := s.k8sClient.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, sts)
	if err != nil {
		return ctrl.Result{}, err
	}

	currentReplicas := *sts.Spec.Replicas
	readyReplicas := sts.Status.ReadyReplicas
	desiredSize := cluster.Spec.Size

	logger.Info("SCALING-DEBUG: Raw cluster spec", "cluster.Spec.Size", cluster.Spec.Size, "cluster.Name", cluster.Name, "cluster.Namespace", cluster.Namespace)
	logger.Info("SCALING-DEBUG: StatefulSet info", "sts.Spec.Replicas", *sts.Spec.Replicas, "sts.Status.ReadyReplicas", sts.Status.ReadyReplicas)
	logger.Info("Scaling status check", "currentReplicas", currentReplicas, "readyReplicas", readyReplicas, "desiredSize", desiredSize)

	// 更新集群状态中的ReadyReplicas
	cluster.Status.ReadyReplicas = readyReplicas

	// 特殊处理：缩容到0 (停止集群)
	if desiredSize == 0 {
		logger.Info("Scaling down cluster to zero (stopping cluster)", "from", readyReplicas)
		return s.handleScaleToZero(ctx, cluster)
	}

	// 检查是否需要继续扩缩容
	if currentReplicas < desiredSize {
		// 扩容前检查：如果有未就绪的Pod，等待它们就绪后再继续扩容
		if readyReplicas < currentReplicas {
			logger.Info("Waiting for existing pods to be ready before scaling up",
				"currentReplicas", currentReplicas, "readyReplicas", readyReplicas, "desiredSize", desiredSize)
			return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
		}
		logger.Info("Scaling up cluster", "currentReplicas", currentReplicas, "to", desiredSize)
		return s.handleScaleUp(ctx, cluster)
	} else if currentReplicas > desiredSize {
		logger.Info("Scaling down cluster", "currentReplicas", currentReplicas, "to", desiredSize)
		return s.handleScaleDown(ctx, cluster)
	}

	// StatefulSet副本数已达到期望值，但Pod未就绪
	// 这可能是因为新Pod没有被添加到etcd集群中
	if readyReplicas < desiredSize {
		logger.Info("StatefulSet has desired replicas but pods not ready, checking if etcd members need to be added",
			"ready", readyReplicas, "desired", desiredSize, "currentReplicas", currentReplicas)

		// 检查是否有Pod存在但未就绪，可能需要添加到etcd集群
		for i := readyReplicas; i < currentReplicas; i++ {
			memberName := fmt.Sprintf("%s-%d", cluster.Name, i)
			serviceName := fmt.Sprintf("%s.%s-peer.%s.svc.cluster.local", memberName, cluster.Name, cluster.Namespace)

			// 检查Pod是否存在
			if s.isServiceResolvable(serviceName) {
				logger.Info("Found unready pod that needs to be added to etcd cluster", "memberIndex", i, "serviceName", serviceName)

				// 尝试添加到etcd集群
				if err := s.addEtcdMember(ctx, cluster, int32(i)); err != nil {
					logger.Error(err, "Failed to add etcd member for unready pod", "memberIndex", i)
					return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
				}

				logger.Info("Successfully added etcd member for unready pod", "memberIndex", i)
				return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
			}
		}

		logger.Info("Waiting for all pods to be ready", "ready", readyReplicas, "desired", desiredSize)
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	// 扩缩容完成，回到运行状态
	logger.Info("Scaling completed", "finalSize", readyReplicas)
	cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseRunning
	s.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionFalse, utils.ReasonRunning, "Scaling completed")

	if err := s.k8sClient.Status().Update(ctx, cluster); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: utils.DefaultHealthCheckInterval}, nil
}

// HandleStopped 处理停止状态的集群
func (s *scalingService) HandleStopped(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 检查是否需要重启集群
	if cluster.Spec.Size > 0 {
		logger.Info("Restarting cluster from stopped state", "targetSize", cluster.Spec.Size)
		cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseScaling
		s.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionTrue, utils.ReasonScaling, "Restarting cluster from stopped state")

		if err := s.k8sClient.Status().Update(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 保持停止状态
	logger.Info("Cluster remains stopped")
	return ctrl.Result{RequeueAfter: utils.DefaultHealthCheckInterval * 2}, nil
}

// NeedsScaling 检查是否需要扩缩容
func (s *scalingService) NeedsScaling(cluster *etcdv1alpha1.EtcdCluster) bool {
	return cluster.Status.ReadyReplicas != cluster.Spec.Size
}

// ValidateScaling 验证扩缩容操作
func (s *scalingService) ValidateScaling(cluster *etcdv1alpha1.EtcdCluster, targetSize int32) error {
	if targetSize < 0 {
		return fmt.Errorf("target size cannot be negative: %d", targetSize)
	}

	if targetSize > 0 && targetSize%2 == 0 {
		return fmt.Errorf("etcd cluster size must be odd for quorum, got: %d", targetSize)
	}

	if targetSize > 7 {
		return fmt.Errorf("etcd cluster size should not exceed 7 for performance reasons, got: %d", targetSize)
	}

	return nil
}

// handleScaleUp 处理扩容
func (s *scalingService) handleScaleUp(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 获取当前 StatefulSet
	sts := &appsv1.StatefulSet{}
	err := s.k8sClient.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, sts)
	if err != nil {
		return ctrl.Result{}, err
	}

	currentSize := *sts.Spec.Replicas
	desiredSize := cluster.Spec.Size

	// 渐进式扩容：一次只增加一个节点
	targetSize := currentSize + 1
	if targetSize > desiredSize {
		targetSize = desiredSize
	}

	logger.Info("Progressive scaling up", "current", currentSize, "target", targetSize, "desired", desiredSize)

	nextMemberIndex := currentSize
	nextMemberName := fmt.Sprintf("%s-%d", cluster.Name, nextMemberIndex)
	serviceName := fmt.Sprintf("%s.%s-peer.%s.svc.cluster.local", nextMemberName, cluster.Name, cluster.Namespace)

	// 步骤1: 先更新StatefulSet副本数，让Kubernetes创建新Pod和Service
	*sts.Spec.Replicas = targetSize
	if err := s.k8sClient.Update(ctx, sts); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("StatefulSet updated, waiting for service to be resolvable", "from", currentSize, "to", targetSize, "serviceName", serviceName)

	// 步骤2: 等待新Service可以被DNS解析
	if !s.isServiceResolvable(serviceName) {
		logger.Info("Service not yet resolvable, waiting", "serviceName", serviceName)
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	logger.Info("Service is resolvable, adding etcd member", "serviceName", serviceName)

	// 步骤3: Service就绪后，通过etcd API添加成员
	if err := s.addEtcdMember(ctx, cluster, nextMemberIndex); err != nil {
		logger.Error(err, "Failed to add etcd member", "memberIndex", nextMemberIndex)
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	logger.Info("Etcd member added successfully", "memberIndex", nextMemberIndex)

	// 如果还没有达到期望大小，继续扩容
	if targetSize < desiredSize {
		logger.Info("Continue scaling up", "current", targetSize, "desired", desiredSize)
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	// 达到期望大小，扩容完成
	logger.Info("Scale up completed", "finalSize", targetSize)
	return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
}

// handleScaleDown 处理缩容
func (s *scalingService) handleScaleDown(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 获取当前 StatefulSet
	sts := &appsv1.StatefulSet{}
	err := s.k8sClient.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, sts)
	if err != nil {
		return ctrl.Result{}, err
	}

	currentSize := *sts.Spec.Replicas
	desiredSize := cluster.Spec.Size

	// 渐进式缩容：一次只减少一个节点
	targetSize := currentSize - 1
	if targetSize < desiredSize {
		targetSize = desiredSize
	}

	logger.Info("Progressive scaling down", "current", currentSize, "target", targetSize, "desired", desiredSize)

	// 步骤1: 先从etcd集群中移除成员
	memberToRemove := currentSize - 1 // 移除最后一个成员
	memberName := fmt.Sprintf("%s-%d", cluster.Name, memberToRemove)

	if err := s.removeEtcdMember(ctx, cluster, memberName); err != nil {
		logger.Error(err, "Failed to remove etcd member", "memberName", memberName)
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	logger.Info("Etcd member removed successfully", "memberName", memberName)

	// 步骤2: 更新StatefulSet副本数，让Kubernetes删除Pod
	*sts.Spec.Replicas = targetSize
	if err := s.k8sClient.Update(ctx, sts); err != nil {
		return ctrl.Result{}, err
	}

	// 如果还没有达到期望大小，继续缩容
	if targetSize > desiredSize {
		logger.Info("Continue scaling down", "current", targetSize, "desired", desiredSize)
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	// 达到期望大小，缩容完成
	logger.Info("Scale down completed", "finalSize", targetSize)
	return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
}

// handleScaleToZero 处理缩容到0
func (s *scalingService) handleScaleToZero(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 获取当前 StatefulSet
	sts := &appsv1.StatefulSet{}
	err := s.k8sClient.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, sts)
	if err != nil {
		return ctrl.Result{}, err
	}

	// 缩容到0
	logger.Info("Scaling StatefulSet to zero")
	*sts.Spec.Replicas = 0

	if err := s.k8sClient.Update(ctx, sts); err != nil {
		return ctrl.Result{}, err
	}

	// 更新集群状态为停止
	cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseStopped
	cluster.Status.ReadyReplicas = 0
	s.setCondition(cluster, utils.ConditionTypeReady, metav1.ConditionFalse, utils.ReasonStopped, "Cluster scaled to zero and stopped")

	if err := s.k8sClient.Status().Update(ctx, cluster); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Cluster successfully scaled to zero and stopped")
	return ctrl.Result{RequeueAfter: utils.DefaultHealthCheckInterval * 2}, nil
}

// addEtcdMember adds a new member to the etcd cluster
func (s *scalingService) addEtcdMember(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, memberIndex int32) error {
	logger := log.FromContext(ctx)

	// 创建 etcd 客户端
	logger.Info("Creating etcd client for member addition")
	etcdClient, err := s.createEtcdClient(cluster)
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

	// 检查成员是否已经存在，并处理unstarted成员
	expectedPeerURL := fmt.Sprintf("http://%s.%s-peer.%s.svc.cluster.local:%d",
		memberName, cluster.Name, cluster.Namespace, utils.EtcdPeerPort)

	for _, member := range members {
		if member.Name == memberName {
			logger.Info("Etcd member already exists, skipping addition", "name", memberName)
			return nil
		}
		// 检查是否有unstarted成员（没有名称但有匹配的peer URL）
		if member.Name == "" && member.PeerURL == expectedPeerURL {
			logger.Info("Found unstarted member with matching peer URL, removing it first",
				"memberID", member.ID, "peerURL", member.PeerURL)

			// 移除unstarted成员
			var memberID uint64
			if _, err := fmt.Sscanf(member.ID, "%x", &memberID); err != nil {
				logger.Error(err, "Failed to parse member ID", "memberID", member.ID)
				return fmt.Errorf("failed to parse member ID %s: %w", member.ID, err)
			}

			if err := etcdClient.RemoveMember(ctx, memberID); err != nil {
				logger.Error(err, "Failed to remove unstarted member", "memberID", memberID)
				return fmt.Errorf("failed to remove unstarted member %s: %w", member.ID, err)
			}

			logger.Info("Successfully removed unstarted member", "memberID", member.ID)
			break
		}
	}

	// 在添加成员之前，检查Service是否可以被DNS解析
	serviceName := fmt.Sprintf("%s.%s-peer.%s.svc.cluster.local", memberName, cluster.Name, cluster.Namespace)
	logger.Info("Checking if service is resolvable before adding etcd member", "serviceName", serviceName)

	if !s.isServiceResolvable(serviceName) {
		logger.Info("Service is not yet resolvable, waiting", "serviceName", serviceName)
		return fmt.Errorf("service %s is not yet resolvable", serviceName)
	}

	logger.Info("Service is resolvable, proceeding to add etcd member", "name", memberName, "peerURL", peerURL)

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
func (s *scalingService) removeEtcdMember(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, memberName string) error {
	logger := log.FromContext(ctx)

	// 创建 etcd 客户端
	etcdClient, err := s.createEtcdClient(cluster)
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

// isPodReady checks if a pod is ready
func (s *scalingService) isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}

	return false
}

// isServiceResolvable checks if a service name can be resolved via DNS
func (s *scalingService) isServiceResolvable(serviceName string) bool {
	// 在Kubernetes集群内，控制器无法直接进行DNS解析
	// 改为检查对应的Pod是否存在（不需要等待就绪，因为Pod需要先被添加到etcd集群才能就绪）

	// 从serviceName中提取Pod名称
	// serviceName格式: "test-single-node-1.test-single-node-peer.default.svc.cluster.local"
	parts := strings.Split(serviceName, ".")
	if len(parts) < 2 {
		return false
	}

	podName := parts[0]
	namespace := "default" // 假设在default namespace，实际应该从serviceName解析
	if len(parts) >= 3 {
		namespace = parts[2]
	}

	// 检查Pod是否存在（不检查就绪状态）
	pod := &corev1.Pod{}
	err := s.k8sClient.Get(context.Background(), types.NamespacedName{
		Name:      podName,
		Namespace: namespace,
	}, pod)

	if err != nil {
		// Pod不存在
		return false
	}

	// Pod存在就认为Service可解析
	// 不需要等待Pod就绪，因为Pod需要先被添加到etcd集群才能就绪
	return true
}

// createEtcdClient creates an etcd client for the cluster
func (s *scalingService) createEtcdClient(cluster *etcdv1alpha1.EtcdCluster) (*etcdclient.Client, error) {
	var endpoints []string

	// 检查是否运行在集群内部
	inCluster := s.isRunningInCluster()
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
		nodeIP, err := s.getKindNodeIP()
		if err != nil {
			log.Log.Error(err, "Failed to get Kind node IP, falling back to localhost")
			nodeIP = "localhost"
		}
		endpoints = []string{fmt.Sprintf("http://%s:30379", nodeIP)}
	}

	log.Log.Info("Creating etcd client", "endpoints", endpoints)
	return etcdclient.NewClient(endpoints)
}

// isRunningInCluster checks if the operator is running inside the cluster
func (s *scalingService) isRunningInCluster() bool {
	// 检查是否存在 service account token 文件
	_, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token")
	return err == nil
}

// getKindNodeIP gets the IP address of the Kind cluster node
func (s *scalingService) getKindNodeIP() (string, error) {
	// 获取集群中的节点列表
	nodes := &corev1.NodeList{}
	if err := s.k8sClient.List(context.Background(), nodes); err != nil {
		return "", fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		return "", fmt.Errorf("no nodes found in cluster")
	}

	// 获取第一个节点的内部 IP
	node := nodes.Items[0]
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address, nil
		}
	}

	return "", fmt.Errorf("no internal IP found for node %s", node.Name)
}

// setCondition 设置集群条件
func (s *scalingService) setCondition(cluster *etcdv1alpha1.EtcdCluster, conditionType string, status metav1.ConditionStatus, reason, message string) {
	// 简化版本的条件设置，实际实现应该更完整
	// 这里只是为了编译通过
}
