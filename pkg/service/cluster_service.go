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
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	"github.com/your-org/etcd-k8s-operator/pkg/client"
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
	// TODO: 实现多节点集群创建逻辑
	logger := log.FromContext(ctx)
	logger.Info("Multi-node cluster creation not yet implemented")
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
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
