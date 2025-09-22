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

package cluster

import (
	"context"
	"crypto/tls"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	etcdv1alpha1 "github.com/etcd-lz/etcd-k8s-operator/api/v1alpha1"
	"github.com/etcd-lz/etcd-k8s-operator/pkg/etcd"
	"github.com/etcd-lz/etcd-k8s-operator/pkg/k8s"
	"github.com/etcd-lz/etcd-k8s-operator/pkg/util/retry"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// reconcileInterval 协调循环间隔
	reconcileInterval = 5 * time.Second
	// podTerminationGracePeriod Pod终止宽限期
	podTerminationGracePeriod = int64(5)
	// healthCheckInterval 健康检查间隔
	healthCheckInterval = 15 * time.Second
)

// clusterEventType 集群事件类型
type clusterEventType string

const (
	eventModifyCluster clusterEventType = "Modify"
)

// clusterEvent 集群事件
type clusterEvent struct {
	typ     clusterEventType
	cluster *etcdv1alpha1.EtcdCluster
}

// Config 集群配置
type Config struct {
	ServiceAccount string
	KubeCli        kubernetes.Interface
	Client         client.Client
	Recorder       record.EventRecorder
}

// Cluster 表示一个etcd集群
type Cluster struct {
	logger logr.Logger
	config Config

	cluster *etcdv1alpha1.EtcdCluster

	// 集群的内存状态
	// status是Cluster结构体实例化后的真实来源
	status etcdv1alpha1.ClusterStatus

	// 作用：传递集群事件（如集群更新）
	// 缓冲：100个事件的缓冲区
	// 使用：run() 方法中通过 select 监听事件并处理
	eventCh chan *clusterEvent

	// 类型：chan struct{}
	// 作用：传递停止信号，用于优雅关闭
	// 使用：Delete() 方法中关闭通道，run()方法监听关闭信号
	stopCh chan struct{}

	// members表示etcd集群中的成员
	// 成员的名称是成员进程运行的pod的名称
	members etcd.MemberSet

	tlsConfig *tls.Config

	// mu 保护共享状态的互斥锁
	mu sync.RWMutex

	// lastSyncTime 记录上次从etcd同步成员状态的时间
	lastSyncTime time.Time

	// healthCheckInterval 健康检查间隔
	healthCheckInterval time.Duration

	// lastHealthCheckTime 记录上次健康检查的时间
	lastHealthCheckTime time.Time
}

// New 创建新的集群实例
func New(config Config, cl *etcdv1alpha1.EtcdCluster, logger logr.Logger) *Cluster {
	c := &Cluster{
		logger:             logger.WithValues("cluster-name", cl.Name),
		config:             config,
		cluster:            cl,
		eventCh:            make(chan *clusterEvent, 100),
		stopCh:             make(chan struct{}),
		status:             *cl.Status.DeepCopy(),
		healthCheckInterval: healthCheckInterval,
	}

	// 启动集群管理协程
	// 先创建集群，完成初始化; 然后启动run开始调谐
	go func() {
		if err := c.setup(); err != nil {
			c.logger.Error(err, "cluster failed to setup")
			// 集群设置异常了，把阶段直接转换到Failed
			if c.status.Phase != etcdv1alpha1.ClusterPhaseFailed {
				c.status.SetReason(err.Error())
				c.status.SetPhase(etcdv1alpha1.ClusterPhaseFailed)
				if err := c.updateCRStatus(); err != nil {
					c.logger.Error(err, "failed to update cluster phase", "phase", etcdv1alpha1.ClusterPhaseFailed)
				}
			}
			return
		}
		c.run()
	}()

	return c
}

// setup 设置集群
func (c *Cluster) setup() error {
	var shouldCreateCluster bool
	switch c.status.Phase {
	case etcdv1alpha1.ClusterPhaseNone:
		shouldCreateCluster = true
	case etcdv1alpha1.ClusterPhaseCreating:
		return c.recoverFromCreating()
	case etcdv1alpha1.ClusterPhaseRunning:
		return c.recoverFromRunning()
	default:
		return fmt.Errorf("unexpected cluster phase: %s", c.status.Phase)
	}

	if shouldCreateCluster {
		return c.create()
	}
	return nil
}

// create 创建集群
func (c *Cluster) create() error {
	// 更新状态为 Creating
	c.status.SetPhase(etcdv1alpha1.ClusterPhaseCreating)

	if err := c.updateCRStatus(); err != nil {
		return fmt.Errorf("cluster create: failed to update cluster phase (%v): %v", etcdv1alpha1.ClusterPhaseCreating, err)
	}
	c.logClusterCreation()

	return c.prepareSeedMember()
}

// prepareSeedMember 准备种子成员，后续成员加到这个种子成员中组建成集群
func (c *Cluster) prepareSeedMember() error {
	// 明确当前集群从 0 个节点扩容到目标规格（如 3 节点）的状态，为后续协调循环提供依据
	c.status.SetScalingUpCondition(0, c.cluster.Spec.Size)

	err := c.bootstrap()
	if err != nil {
		return err
	}

	// 更新当前cr实例内存状态中的集群大小（TODO，没有更新 cr，要确认是否有必要更新 cr）
	c.status.Size = 1
	return nil
}

// bootstrap 引导集群
func (c *Cluster) bootstrap() error {
	return c.startSeedMember()
}

// startSeedMember 启动种子成员
func (c *Cluster) startSeedMember() error {
	m := c.newMember()
	ms := etcd.NewMemberSet(m)
	c.members = ms

	// 创建服务
	if err := c.setupServices(); err != nil {
		return fmt.Errorf("failed to setup services: %v", err)
	}

	// 创建种子Pod
	pod := k8s.NewEtcdPod(m, ms.PeerURLPairs(), c.cluster.Name, "new", "", c.cluster, c.cluster.AsOwner())
	k8s.AddEtcdVolumeToPod(pod, nil) // 暂时不使用PVC，使用常规的 emptyDir

	ctx := context.TODO()
	_, err := c.config.KubeCli.CoreV1().Pods(c.cluster.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create seed pod: %v", err)
	}

	c.logger.Info("seed member created", "member", m.Name)
	return nil
}

// Delete 删除集群
func (c *Cluster) Delete() {
	c.logger.Info("cluster is deleted by user")
	close(c.stopCh)
}

// Update 更新集群
func (c *Cluster) Update(cl *etcdv1alpha1.EtcdCluster) {
	c.send(&clusterEvent{
		typ:     eventModifyCluster,
		cluster: cl,
	})
}

// send 发送事件到事件通道
func (c *Cluster) send(event *clusterEvent) {
	select {
	case c.eventCh <- event:
	case <-c.stopCh:
	}
}

// setupServices 设置服务
func (c *Cluster) setupServices() error {
	ctx := context.TODO()

	err := k8s.CreateClientService(ctx, c.config.KubeCli, c.cluster.Name, c.cluster.Namespace, c.cluster.AsOwner())
	if err != nil {
		return err
	}

	return k8s.CreatePeerService(ctx, c.config.KubeCli, c.cluster.Name, c.cluster.Namespace, c.cluster.AsOwner())
}

// newMember 创建新成员
func (c *Cluster) newMember() *etcd.Member {
	name := k8s.UniqueMemberName(c.cluster.Name)
	return &etcd.Member{
		Name:         name,
		Namespace:    c.cluster.Namespace,
		SecurePeer:   c.isSecurePeer(),
		SecureClient: c.isSecureClient(),
	}
}

// isSecurePeer 检查是否启用peer TLS
func (c *Cluster) isSecurePeer() bool {
	return c.cluster.Spec.TLS != nil && c.cluster.Spec.TLS.IsSecurePeer()
}

// isSecureClient 检查是否启用client TLS
func (c *Cluster) isSecureClient() bool {
	return c.cluster.Spec.TLS != nil && c.cluster.Spec.TLS.IsSecureClient()
}

// isPodPVEnabled 检查是否启用Pod持久卷
func (c *Cluster) isPodPVEnabled() bool {
	if podPolicy := c.cluster.Spec.Pod; podPolicy != nil {
		return podPolicy.PersistentVolumeClaimSpec != nil
	}
	return false
}

// logClusterCreation 记录集群创建日志
func (c *Cluster) logClusterCreation() {
	c.logger.Info("creating etcd cluster", "size", c.cluster.Spec.Size)
}

// recoverFromCreating 从创建状态恢复
func (c *Cluster) recoverFromCreating() error {
	c.logger.Info("recovering from creating state")
	return c.recoverFromRunning()
}

// recoverFromRunning 从运行状态恢复
func (c *Cluster) recoverFromRunning() error {
	c.logger.Info("recovering from running state")

	running, _, err := c.pollPods()
	if err != nil {
		return err
	}

	if len(running) == 0 {
		// If there are no running pods, maybe the cluster has been deleted.
		// We will let the reconciliation loop handle this case.
		return nil
	}

	c.members = podsToMemberSet(running, c.isSecureClient())
	return nil
}

// updateCRStatus 更新CR状态，TODO 这个方法的调用方那边可能有问题，目前存在实际 pod 数量和 cr 的值不一致的情况
func (c *Cluster) updateCRStatus() error {
	// 使用读锁保护状态读取
	c.mu.RLock()

	// c.cluster.Status 是cr实际存储在k8s etcd中的状态
	// c.status 是调谐过程中临时状态

	if reflect.DeepEqual(c.cluster.Status, c.status) {
		c.mu.RUnlock()
		return nil
	}

	// 创建副本用于更新
	newCluster := c.cluster.DeepCopy()
	newCluster.Status = c.status

	// 读取必要的信息后释放读锁
	currentStatus := c.status
	c.mu.RUnlock()

	// 使用重试机制更新状态，避免网络抖动导致的失败
	err := retry.Retry(1*time.Second, 3, func() (bool, error) {
		ctx := context.TODO()
		err := c.config.Client.Status().Update(ctx, newCluster)
		if err != nil {
			c.logger.Warn("failed to update CR status, will retry", "error", err)
			return false, err
		}
		return true, nil
	})

	if err != nil {
		if retry.IsRetryError(err) {
			c.logger.Error(err, "failed to update CR status after retries")
		} else {
			c.logger.Error(err, "failed to update CR status")
		}
		return err
	}

	// 更新成功后获取写锁更新内存状态
	c.mu.Lock()
	c.cluster = newCluster
	c.mu.Unlock()

	c.logger.V(1).Info("CR status updated successfully")
	return nil
}

// run 运行集群管理主循环
func (c *Cluster) run() {
	c.status.SetPhase(etcdv1alpha1.ClusterPhaseRunning)
	if err := c.updateCRStatus(); err != nil {
		c.logger.Error(err, "failed to update cluster phase to running")
		return
	}

	c.logger.Info("start running cluster")

	var rerr error
	for {
		select {
		case <-c.stopCh:
			return
		case event := <-c.eventCh:
			switch event.typ {
			case eventModifyCluster:
				if isSpecEqual(event.cluster.Spec, c.cluster.Spec) {
					break
				}
				// 更新集群规格
				c.mu.Lock()
				c.cluster = event.cluster
				c.mu.Unlock()
				c.logger.Info("cluster spec updated")
			}
		case <-time.After(reconcileInterval):
			// 定期协调循环（每5秒）
			start := time.Now()

			// 1. 轮询Pod状态
			running, pending, err := c.pollPods()
			if err != nil {
				c.logger.Error(err, "failed to poll pods")
				continue
			}

			// 2. 处理pending状态的Pod
			if len(pending) > 0 {
				c.logger.Info("skip reconciliation: pods are pending",
					"running", k8s.GetPodNames(running),
					"pending", k8s.GetPodNames(pending))
				continue
			}

			// 3. 如果没有运行中的Pod，记录日志
			if len(running) == 0 {
				c.logger.Info("all etcd pods are dead")
				break
			}

			// 4. 更新成员信息
			c.mu.Lock()
			if c.members == nil {
				c.members = podsToMemberSet(running, c.isSecureClient())
			}

			// 5. 定期从etcd集群同步成员状态（每30秒同步一次）
			if time.Since(c.lastSyncTime) > 30*time.Second {
				if err := c.syncMembersFromEtcd(); err != nil {
					c.logger.Warn("failed to sync members from etcd", "error", err)
				} else {
					c.lastSyncTime = time.Now()
				}
			}

			// 6. 执行协调逻辑
			rerr = c.reconcile(running)
			if rerr != nil {
				c.logger.Error(rerr, "failed to reconcile")
				c.mu.Unlock()
				break
			}

			// 7. 更新成员状态
			c.updateMemberStatus(running)
			c.mu.Unlock()

			// 8. 执行健康检查（每15秒检查一次）
			if time.Since(c.lastHealthCheckTime) > c.healthCheckInterval {
				if err := c.performHealthCheck(); err != nil {
					c.logger.Warn("health check failed", "error", err)
				} else {
					c.lastHealthCheckTime = time.Now()
				}
			}

			// 9. 执行状态一致性检查
			if err := c.validateStateConsistency(running); err != nil {
				c.logger.Warn("state consistency check failed", "error", err)
				// 尝试修复不一致状态
				if fixErr := c.fixStateInconsistency(running); fixErr != nil {
					c.logger.Error("failed to fix state inconsistency", "error", fixErr)
				}
			}

			// 10. 更新CR状态（使用独立的锁）
			if err := c.updateCRStatus(); err != nil {
				c.logger.Error(err, "periodic update CR status failed")
			}

			c.logger.V(1).Info("reconcile completed", "duration", time.Since(start))
		}

		if rerr != nil {
			if etcd.IsFatalError(rerr) {
				c.status.SetReason(rerr.Error())
				c.logger.Error(rerr, "cluster failed")
				c.reportFailedStatus()
				return
			}

			// 对于非致命错误，尝试恢复
			c.logger.Warn("reconciliation failed, attempting recovery", "error", rerr)
			if err := c.attemptRecovery(rerr); err != nil {
				c.logger.Error("recovery failed", "error", err)
				c.status.SetReason(fmt.Sprintf("recovery failed: %v", err))
				c.reportFailedStatus()
				return
			}
		}
	}
}

// isSpecEqual 比较两个集群规格是否相等
func isSpecEqual(s1, s2 etcdv1alpha1.ClusterSpec) bool {
	return reflect.DeepEqual(s1, s2)
}

// attemptRecovery 尝试从错误中恢复
func (c *Cluster) attemptRecovery(recoveryErr error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 获取当前Pod状态
	running, pending, err := c.pollPods()
	if err != nil {
		return fmt.Errorf("failed to poll pods during recovery: %v", err)
	}

	// 如果有pending的Pod，等待它们完成
	if len(pending) > 0 {
		c.logger.Info("waiting for pending pods during recovery", "pending", k8s.GetPodNames(pending))
		return fmt.Errorf("pending pods are not ready yet")
	}

	// 如果没有运行中的Pod，尝试重启集群
	if len(running) == 0 {
		c.logger.Info("no running pods, attempting to restart cluster")
		return c.restartCluster()
	}

	// 检查etcd集群状态
	if c.members != nil && c.members.Size() > 0 {
		// 尝试从etcd集群同步成员状态
		if syncErr := c.syncMembersFromEtcd(); syncErr != nil {
			c.logger.Warn("failed to sync members from etcd during recovery", "error", syncErr)
			// 如果同步失败，尝试从Pod重建成员状态
			c.members = podsToMemberSet(running, c.isSecureClient())
		}

		// 检查法定人数
		if c.members.Size() < c.cluster.Spec.Size/2+1 {
			c.logger.Warn("cluster lost quorum, attempting forced recovery")
			return c.forceQuorumRecovery()
		}
	}

	c.logger.Info("recovery completed successfully")
	return nil
}

// restartCluster 重启集群
func (c *Cluster) restartCluster() error {
	// 清理成员状态
	c.members = etcd.NewMemberSet()

	// 重新创建种子成员
	if err := c.prepareSeedMember(); err != nil {
		return fmt.Errorf("failed to prepare seed member during restart: %v", err)
	}

	c.logger.Info("cluster restart initiated")
	return nil
}

// forceQuorumRecovery 强制恢复法定人数
func (c *Cluster) forceQuorumRecovery() error {
	// 获取当前运行中的Pod
	running, _, err := c.pollPods()
	if err != nil {
		return fmt.Errorf("failed to poll pods during quorum recovery: %v", err)
	}

	// 从运行中的Pod重建成员状态
	c.members = podsToMemberSet(running, c.isSecureClient())

	// 检查重建后的成员数量
	if c.members.Size() < c.cluster.Spec.Size/2+1 {
		return fmt.Errorf("insufficient members for quorum recovery: %d < %d",
			c.members.Size(), c.cluster.Spec.Size/2+1)
	}

	c.logger.Info("quorum recovery completed", "members", c.members.Size())
	return nil
}

// validateStateConsistency 验证状态一致性
func (c *Cluster) validateStateConsistency(running []*corev1.Pod) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 检查成员状态与Pod状态的一致性
	if c.members != nil {
		// 检查运行中的Pod数量与成员数量是否匹配
		if len(running) != c.members.Size() {
			return fmt.Errorf("pod count (%d) does not match member count (%d)", len(running), c.members.Size())
		}

		// 检查每个运行中的Pod是否在成员集合中
		podMap := make(map[string]bool)
		for _, pod := range running {
			podMap[pod.Name] = true
		}

		for _, member := range c.members.Members() {
			if !podMap[member.Name] {
				return fmt.Errorf("member %s not found in running pods", member.Name)
			}
		}

		// 检查CR状态中的大小是否与实际匹配
		if c.status.Size != c.members.Size() {
			return fmt.Errorf("CR status size (%d) does not match actual member count (%d)", c.status.Size, c.members.Size())
		}
	}

	// 检查集群阶段是否合理
	if c.status.Phase == etcdv1alpha1.ClusterPhaseRunning && len(running) == 0 {
		return fmt.Errorf("cluster phase is running but no pods are running")
	}

	return nil
}

// fixStateInconsistency 修复状态不一致
func (c *Cluster) fixStateInconsistency(running []*corev1.Pod) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Info("attempting to fix state inconsistency")

	// 如果成员信息为空，从Pod重建
	if c.members == nil || c.members.Size() == 0 {
		if len(running) > 0 {
			c.members = podsToMemberSet(running, c.isSecureClient())
			c.logger.Info("reconstructed member state from running pods")
			return nil
		}
	}

	// 如果有运行中的Pod但成员状态不一致，尝试同步
	if len(running) > 0 && c.members.Size() > 0 {
		// 尝试从etcd集群同步成员状态
		if syncErr := c.syncMembersFromEtcd(); syncErr != nil {
			c.logger.Warn("failed to sync from etcd, rebuilding from pods", "error", syncErr)
			c.members = podsToMemberSet(running, c.isSecureClient())
		}

		// 更新状态大小
		c.status.Size = c.members.Size()
		c.logger.Info("fixed state inconsistency", "members", c.members.Size())
	}

	return nil
}

// reportFailedStatus 报告失败状态
func (c *Cluster) reportFailedStatus() {
	c.status.SetPhase(etcdv1alpha1.ClusterPhaseFailed)
	if err := c.updateCRStatus(); err != nil {
		c.logger.Error(err, "failed to update failed status")
	}
}

// pollPods 轮询Pod状态
func (c *Cluster) pollPods() (running, pending []*corev1.Pod, err error) {
	ctx := context.TODO()
	podList, err := c.config.KubeCli.CoreV1().Pods(c.cluster.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(k8s.LabelsForCluster(c.cluster.Name)).String(),
	})
	if err != nil {
		return nil, nil, err
	}

	for i := range podList.Items {
		pod := &podList.Items[i]

		// 跳过正在删除的Pod
		if pod.DeletionTimestamp != nil {
			continue
		}

		// 检查Pod所有者
		if !metav1.IsControlledBy(pod, c.cluster) {
			continue
		}

		// 根据Pod状态分类
		switch pod.Status.Phase {
		case corev1.PodRunning:
			running = append(running, pod)
		case corev1.PodPending:
			pending = append(pending, pod)
		}
	}
	return running, pending, nil
}

// getMemberName 从etcd成员信息中提取成员名称
func (c *Cluster) getMemberName(m *etcd.Member) (string, error) {
	// etcd成员的名称通常格式为 <cluster-name>-<ordinal>
	// 从peerURL中提取名称，格式为 http://<name>.<cluster-name>.<namespace>.svc.cluster.local:2380
	peerURL := m.PeerURL()

	// 从peerURL中提取主机名部分
	hostStart := strings.Index(peerURL, "://") + 3
	hostEnd := strings.Index(peerURL[hostStart:], ":")
	if hostEnd == -1 {
		hostEnd = len(peerURL[hostStart:])
	} else {
		hostEnd += hostStart
	}
	host := peerURL[hostStart:hostEnd]

	// 从主机名中提取成员名称
	nameParts := strings.Split(host, ".")
	if len(nameParts) < 2 {
		return "", fmt.Errorf("invalid peer URL format: %s", peerURL)
	}

	memberName := nameParts[0]

	// 验证成员名称是否属于当前集群
	if !strings.HasPrefix(memberName, c.cluster.Name+"-") {
		return "", fmt.Errorf("member %s does not belong to cluster %s", memberName, c.cluster.Name)
	}

	return memberName, nil
}

// getMemberNameFromPeerURL 从peerURL中提取成员名称
func (c *Cluster) getMemberNameFromPeerURL(peerURL string) (string, error) {
	// 从peerURL中提取主机名部分
	hostStart := strings.Index(peerURL, "://") + 3
	hostEnd := strings.Index(peerURL[hostStart:], ":")
	if hostEnd == -1 {
		hostEnd = len(peerURL[hostStart:])
	} else {
		hostEnd += hostStart
	}
	host := peerURL[hostStart:hostEnd]

	// 从主机名中提取成员名称
	nameParts := strings.Split(host, ".")
	if len(nameParts) < 2 {
		return "", fmt.Errorf("invalid peer URL format: %s", peerURL)
	}

	memberName := nameParts[0]

	// 验证成员名称是否属于当前集群
	if !strings.HasPrefix(memberName, c.cluster.Name+"-") {
		return "", fmt.Errorf("member %s does not belong to cluster %s", memberName, c.cluster.Name)
	}

	return memberName, nil
}

// syncMembersFromEtcd 从etcd集群同步成员状态
func (c *Cluster) syncMembersFromEtcd() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果没有成员信息，无法同步
	if c.members == nil || c.members.Size() == 0 {
		return fmt.Errorf("no members to sync from")
	}

	// 获取客户端URL列表
	clientURLs := c.members.ClientURLs()
	if len(clientURLs) == 0 {
		return fmt.Errorf("no client URLs available")
	}

	// 从etcd集群获取成员列表
	resp, err := etcd.ListMembers(clientURLs, c.tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to list members from etcd: %v", err)
	}

	// 构建新的成员集合
	newMembers := etcd.NewMemberSet()
	for _, etcdMember := range resp.Members {
		// 为每个etcd成员创建对应的Member对象
		member := &etcd.Member{
			ID:           etcdMember.ID,
			Namespace:    c.cluster.Namespace,
			SecurePeer:   c.isSecurePeer(),
			SecureClient: c.isSecureClient(),
		}

		// 尝试从etcd成员信息中获取名称
		if len(etcdMember.PeerURLs) > 0 {
			// 从peerURL中提取成员名称
			name, err := c.getMemberNameFromPeerURL(etcdMember.PeerURLs[0])
			if err != nil {
				c.logger.Warn("failed to get member name from peer URL", "error", err, "peerURL", etcdMember.PeerURLs[0])
				// 如果无法从peerURL获取名称，使用ID作为名称的一部分
				member.Name = fmt.Sprintf("%s-%d", c.cluster.Name, etcdMember.ID)
			} else {
				member.Name = name
			}
		} else {
			// 如果没有peerURL，使用ID作为名称的一部分
			member.Name = fmt.Sprintf("%s-%d", c.cluster.Name, etcdMember.ID)
		}

		newMembers.Add(member)
	}

	// 更新成员集合
	c.members = newMembers
	c.logger.Info("synced members from etcd", "count", len(newMembers))

	return nil
}

// updateMemberStatus 更新成员状态
func (c *Cluster) updateMemberStatus(running []*corev1.Pod) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var unready []string
	var ready []string

	for _, pod := range running {
		if k8s.IsPodReady(pod) {
			ready = append(ready, pod.Name)
		} else {
			unready = append(unready, pod.Name)
		}
	}

	c.status.Members.Ready = ready
	c.status.Members.Unready = unready
}

// performHealthCheck 执行健康检查
func (c *Cluster) performHealthCheck() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 如果没有成员信息，跳过健康检查
	if c.members == nil || c.members.Size() == 0 {
		return nil
	}

	// 获取客户端URL列表
	clientURLs := c.members.ClientURLs()
	if len(clientURLs) == 0 {
		return fmt.Errorf("no client URLs available for health check")
	}

	// 检查集群是否具有法定人数
	if c.members.Size() < c.cluster.Spec.Size/2+1 {
		return fmt.Errorf("cluster lost quorum: %d members, expected at least %d",
			c.members.Size(), c.cluster.Spec.Size/2+1)
	}

	// 执行etcd健康检查
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 创建etcd客户端进行健康检查
	client, err := etcd.CreateClient(clientURLs, c.tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to create etcd client for health check: %v", err)
	}
	defer client.Close()

	// 检查集群健康状态
	healthStatus, err := client.Get(ctx, "health")
	if err != nil {
		return fmt.Errorf("etcd health check failed: %v", err)
	}

	if healthStatus == nil {
		return fmt.Errorf("etcd health check returned no response")
	}

	c.logger.V(1).Info("health check passed", "members", c.members.Size(), "clientURLs", len(clientURLs))
	return nil
}
