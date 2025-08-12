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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	"github.com/your-org/etcd-k8s-operator/pkg/client"
	etcdclient "github.com/your-org/etcd-k8s-operator/pkg/etcd"
	resourcepkg "github.com/your-org/etcd-k8s-operator/pkg/resource"
	"github.com/your-org/etcd-k8s-operator/pkg/utils"
)

// clusterService 集群管理服务实现
type clusterService struct {
	k8sClient       client.KubernetesClient
	resourceManager resourcepkg.ResourceManager
}

// NewClusterService 创建集群服务实例
func NewClusterService(
	k8sClient client.KubernetesClient,
	resourceManager resourcepkg.ResourceManager,
) ClusterService {
	return &clusterService{
		k8sClient:       k8sClient,
		resourceManager: resourceManager,
	}
}

// SetDefaults 设置默认值
func (s *clusterService) SetDefaults(cluster *etcdv1alpha1.EtcdCluster) {
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

// ValidateClusterSpec 验证集群规范
func (s *clusterService) ValidateClusterSpec(cluster *etcdv1alpha1.EtcdCluster) error {
	// 验证集群大小
	if cluster.Spec.Size < 0 {
		return fmt.Errorf("cluster size cannot be negative")
	}

	// 验证奇数大小（etcd要求）
	if cluster.Spec.Size > 1 && cluster.Spec.Size%2 == 0 {
		return fmt.Errorf("cluster size must be odd for multi-node clusters")
	}

	// 验证版本格式
	if cluster.Spec.Version == "" {
		return fmt.Errorf("etcd version cannot be empty")
	}

	// 验证存储大小
	if cluster.Spec.Storage.Size.IsZero() {
		return fmt.Errorf("storage size cannot be zero")
	}

	return nil
}

// InitializeCluster 初始化集群
func (s *clusterService) InitializeCluster(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 验证集群规范
	if err := s.ValidateClusterSpec(cluster); err != nil {
		logger.Error(err, "Cluster specification validation failed")
		return s.updateStatusWithError(ctx, cluster, etcdv1alpha1.EtcdClusterPhaseFailed, err)
	}

	// 设置初始状态
	cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseCreating
	s.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionTrue, utils.ReasonCreating, "Starting cluster creation")

	if err := s.UpdateClusterStatus(ctx, cluster); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Cluster initialization completed, transitioning to creating phase")
	return ctrl.Result{Requeue: true}, nil
}

// CreateCluster 创建集群
func (s *clusterService) CreateCluster(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 1. 创建必要的 Kubernetes 资源
	if err := s.resourceManager.EnsureAllResources(ctx, cluster); err != nil {
		logger.Error(err, "Failed to ensure resources")
		return s.updateStatusWithError(ctx, cluster, etcdv1alpha1.EtcdClusterPhaseFailed, err)
	}

	// 2. 对于多节点集群，使用渐进式启动策略
	if cluster.Spec.Size > 1 {
		return s.handleMultiNodeClusterCreation(ctx, cluster)
	}

	// 3. 单节点集群的处理逻辑
	ready, err := s.IsClusterReady(ctx, cluster)
	if err != nil {
		logger.Error(err, "Failed to check cluster readiness")
		return s.updateStatusWithError(ctx, cluster, etcdv1alpha1.EtcdClusterPhaseFailed, err)
	}

	if ready {
		// 转换到运行状态
		cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseRunning
		s.setCondition(cluster, utils.ConditionTypeReady, metav1.ConditionTrue, utils.ReasonRunning, "Etcd cluster is running")
		s.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionFalse, utils.ReasonRunning, "Etcd cluster creation completed")

		if err := s.UpdateClusterStatus(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}

		logger.Info("Single-node cluster is ready, transitioning to running phase")
		return ctrl.Result{}, nil
	}

	// 集群还未就绪，继续等待
	logger.Info("Cluster is not ready yet, requeuing")
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// IsClusterReady 检查集群是否就绪
func (s *clusterService) IsClusterReady(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (bool, error) {
	status, err := s.resourceManager.StatefulSet().GetStatus(ctx, cluster)
	if err != nil {
		return false, err
	}

	// 检查副本数是否匹配
	if status.ReadyReplicas != cluster.Spec.Size {
		return false, nil
	}

	// 检查所有副本是否就绪
	if status.ReadyReplicas != status.Replicas {
		return false, nil
	}

	return true, nil
}

// GetClusterStatus 获取集群状态
func (s *clusterService) GetClusterStatus(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (*ClusterStatus, error) {
	stsStatus, err := s.resourceManager.StatefulSet().GetStatus(ctx, cluster)
	if err != nil {
		return nil, err
	}

	return &ClusterStatus{
		Phase:         string(cluster.Status.Phase),
		ReadyReplicas: stsStatus.ReadyReplicas,
		// TODO: 添加更多状态信息
	}, nil
}

// UpdateClusterStatus 更新集群状态
func (s *clusterService) UpdateClusterStatus(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) error {
	return s.k8sClient.UpdateStatus(ctx, cluster)
}

// DeleteCluster 删除集群
func (s *clusterService) DeleteCluster(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Starting cluster deletion")

	// 清理资源
	if err := s.resourceManager.CleanupResources(ctx, cluster); err != nil {
		logger.Error(err, "Failed to cleanup resources")
		return ctrl.Result{}, err
	}

	logger.Info("Cluster deletion completed")
	return ctrl.Result{}, nil
}

// handleMultiNodeClusterCreation 处理多节点集群创建
func (s *clusterService) handleMultiNodeClusterCreation(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 获取当前 StatefulSet 状态
	sts := &appsv1.StatefulSet{}
	err := s.k8sClient.Get(ctx, types.NamespacedName{
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

	currentReplicas := *sts.Spec.Replicas
	readyReplicas := sts.Status.ReadyReplicas
	desiredSize := cluster.Spec.Size

	logger.Info("Multi-node cluster status check",
		"currentReplicas", currentReplicas,
		"readyReplicas", readyReplicas,
		"desiredSize", desiredSize)

	// 如果所有副本都已就绪，集群创建完成
	if readyReplicas == desiredSize {
		cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseRunning
		s.setCondition(cluster, utils.ConditionTypeReady, metav1.ConditionTrue, utils.ReasonRunning, "Multi-node etcd cluster is running")
		s.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionFalse, utils.ReasonRunning, "Multi-node etcd cluster creation completed")

		if err := s.UpdateClusterStatus(ctx, cluster); err != nil {
			return ctrl.Result{}, err
		}

		s.k8sClient.GetRecorder().Event(cluster, corev1.EventTypeNormal, utils.EventReasonClusterCreated,
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
		if err := s.k8sClient.Update(ctx, sts); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	if readyReplicas >= 1 && readyReplicas < desiredSize {
		// 第一个节点已就绪，但还有节点未就绪，需要动态扩容
		logger.Info("Starting dynamic expansion", "ready", readyReplicas, "current", currentReplicas, "desired", desiredSize)

		// 使用动态扩容逻辑添加其他节点
		return s.handleDynamicExpansion(ctx, cluster, sts)
	}

	// 更新状态信息
	s.setCondition(cluster, utils.ConditionTypeProgressing, metav1.ConditionTrue, utils.ReasonCreating,
		fmt.Sprintf("Creating multi-node cluster: %d/%d nodes ready", readyReplicas, desiredSize))

	if err := s.UpdateClusterStatus(ctx, cluster); err != nil {
		return ctrl.Result{}, err
	}

	// 继续等待更多节点就绪
	return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
}

// handleDynamicExpansion handles expanding from single-node to multi-node cluster
func (s *clusterService) handleDynamicExpansion(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, sts *appsv1.StatefulSet) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	currentSize := *sts.Spec.Replicas
	readyReplicas := sts.Status.ReadyReplicas
	desiredSize := cluster.Spec.Size

	logger.Info("Starting dynamic expansion", "currentSize", currentSize, "readyReplicas", readyReplicas, "desiredSize", desiredSize)

	// 确定下一个要添加的成员索引
	// 应该是当前就绪的副本数（即下一个需要添加到etcd集群的成员）
	nextMemberIndex := readyReplicas
	nextMemberName := fmt.Sprintf("%s-%d", cluster.Name, nextMemberIndex)

	logger.Info("Adding new etcd member", "memberIndex", nextMemberIndex, "memberName", nextMemberName)

	// 步骤 1: 先通过 etcd API 添加成员
	if err := s.addEtcdMember(ctx, cluster, nextMemberIndex); err != nil {
		logger.Error(err, "Failed to add etcd member", "memberIndex", nextMemberIndex)
		return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
	}

	logger.Info("Etcd member added successfully", "memberName", nextMemberName)

	// 步骤 2: 如果StatefulSet副本数还不够，增加副本数
	if currentSize < desiredSize {
		nextSize := currentSize + 1
		*sts.Spec.Replicas = nextSize
		if err := s.k8sClient.Update(ctx, sts); err != nil {
			logger.Error(err, "Failed to update StatefulSet replicas")
			return ctrl.Result{}, err
		}
		logger.Info("StatefulSet replicas updated", "from", currentSize, "to", nextSize)
	}

	// 等待新成员加入并就绪
	logger.Info("Waiting for new member to join and become ready")
	return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
}

// addEtcdMember adds a new member to the etcd cluster
func (s *clusterService) addEtcdMember(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, memberIndex int32) error {
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

// createEtcdClient creates an etcd client for the cluster
func (s *clusterService) createEtcdClient(cluster *etcdv1alpha1.EtcdCluster) (*etcdclient.Client, error) {
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
func (s *clusterService) isRunningInCluster() bool {
	// 检查是否存在 service account token 文件
	_, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token")
	return err == nil
}

// getKindNodeIP gets the IP address of the Kind cluster node
func (s *clusterService) getKindNodeIP() (string, error) {
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

// updateStatusWithError 更新状态并记录错误
func (s *clusterService) updateStatusWithError(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, phase etcdv1alpha1.EtcdClusterPhase, err error) (ctrl.Result, error) {
	cluster.Status.Phase = phase
	s.setCondition(cluster, utils.ConditionTypeReady, metav1.ConditionFalse, utils.ReasonFailed, err.Error())

	s.k8sClient.RecordEvent(cluster, "Warning", "Failed", err.Error())

	if updateErr := s.UpdateClusterStatus(ctx, cluster); updateErr != nil {
		return ctrl.Result{}, updateErr
	}

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// setCondition 设置条件
func (s *clusterService) setCondition(cluster *etcdv1alpha1.EtcdCluster, conditionType string, status metav1.ConditionStatus, reason, message string) {
	// TODO: 实现条件设置逻辑
}
