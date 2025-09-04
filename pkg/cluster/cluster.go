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
	"time"

	etcdv1alpha1 "github.com/etcd-lz/etcd-k8s-operator/api/v1alpha1"
	"github.com/etcd-lz/etcd-k8s-operator/pkg/etcd"
	"github.com/etcd-lz/etcd-k8s-operator/pkg/k8s"

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

	eventCh chan *clusterEvent
	stopCh  chan struct{}

	// members表示etcd集群中的成员
	// 成员的名称是成员进程运行的pod的名称
	members etcd.MemberSet

	tlsConfig *tls.Config
}

// New 创建新的集群实例
func New(config Config, cl *etcdv1alpha1.EtcdCluster, logger logr.Logger) *Cluster {
	c := &Cluster{
		logger:  logger.WithValues("cluster-name", cl.Name),
		config:  config,
		cluster: cl,
		eventCh: make(chan *clusterEvent, 100),
		stopCh:  make(chan struct{}),
		status:  *cl.Status.DeepCopy(),
	}

	// 启动集群管理协程
	go func() {
		if err := c.setup(); err != nil {
			c.logger.Error(err, "cluster failed to setup")
			if c.status.Phase != etcdv1alpha1.ClusterPhaseFailed {
				c.status.SetReason(err.Error())
				c.status.SetPhase(etcdv1alpha1.ClusterPhaseFailed)
				// 状态管理现在由Reconciler负责，这里不再更新
				// if err := c.updateCRStatus(); err != nil {
				// 	c.logger.Error(err, "failed to update cluster phase", "phase", etcdv1alpha1.ClusterPhaseFailed)
				// }
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
	c.status.SetPhase(etcdv1alpha1.ClusterPhaseCreating)

	// 状态管理现在由Reconciler负责，这里不再更新
	// if err := c.updateCRStatus(); err != nil {
	// 	return fmt.Errorf("cluster create: failed to update cluster phase (%v): %v", etcdv1alpha1.ClusterPhaseCreating, err)
	// }
	c.logClusterCreation()

	return c.prepareSeedMember()
}

// prepareSeedMember 准备种子成员
func (c *Cluster) prepareSeedMember() error {
	c.status.SetScalingUpCondition(0, c.cluster.Spec.Size)

	err := c.bootstrap()
	if err != nil {
		return err
	}

	c.status.Size = 1
	return nil
}

// bootstrap 引导集群
func (c *Cluster) bootstrap() error {
	return c.startSeedMember()
}

// startSeedMember 启动种子成员
func (c *Cluster) startSeedMember() error {
	c.logger.Info("starting seed member creation")
	m := c.newMember()
	ms := etcd.NewMemberSet(m)
	c.members = ms

	// 创建服务
	c.logger.Info("setting up services")
	if err := c.setupServices(); err != nil {
		c.logger.Error(err, "failed to setup services")
		return fmt.Errorf("failed to setup services: %v", err)
	}
	c.logger.Info("services setup completed")

	// 创建种子Pod
	c.logger.Info("creating seed pod")
	pod := k8s.NewEtcdPod(m, ms.PeerURLPairs(), c.cluster.Name, "new", "", c.cluster, c.cluster.AsOwner())
	k8s.AddEtcdVolumeToPod(pod, nil) // 暂时不使用PVC

	ctx := context.TODO()
	_, err := c.config.KubeCli.CoreV1().Pods(c.cluster.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		c.logger.Error(err, "failed to create seed pod")
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

	c.logger.Info("creating client service", "name", c.cluster.Name, "namespace", c.cluster.Namespace)
	err := k8s.CreateClientService(ctx, c.config.KubeCli, c.cluster.Name, c.cluster.Namespace, c.cluster.AsOwner())
	if err != nil {
		c.logger.Error(err, "failed to create client service")
		return err
	}
	c.logger.Info("client service created successfully")

	c.logger.Info("creating peer service", "name", c.cluster.Name, "namespace", c.cluster.Namespace)
	err = k8s.CreatePeerService(ctx, c.config.KubeCli, c.cluster.Name, c.cluster.Namespace, c.cluster.AsOwner())
	if err != nil {
		c.logger.Error(err, "failed to create peer service")
		return err
	}
	c.logger.Info("peer service created successfully")

	return nil
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
		// If there are no running pods, this indicates a disaster scenario
		// Reset the cluster state to trigger recreation
		c.logger.Info("No running pods found during recovery, resetting cluster state")
		c.status.SetPhase(etcdv1alpha1.ClusterPhaseNone)
		c.status.Size = 0
		c.status.Members.Ready = []string{}
		c.status.Members.Unready = []string{}
		c.members = etcd.NewMemberSet()

		// Trigger cluster recreation by changing phase to None
		// This will cause the setup() method to create a new cluster
		return c.create()
	}

	c.members = podsToMemberSet(running, c.isSecureClient())
	return nil
}

// updateCRStatus 更新CR状态
func (c *Cluster) updateCRStatus() error {
	if reflect.DeepEqual(c.cluster.Status, c.status) {
		return nil
	}

	newCluster := c.cluster.DeepCopy()
	newCluster.Status = c.status

	ctx := context.TODO()
	err := c.config.Client.Status().Update(ctx, newCluster)
	if err != nil {
		return err
	}

	c.cluster = newCluster
	return nil
}

// run 运行集群管理主循环
func (c *Cluster) run() {
	c.status.SetPhase(etcdv1alpha1.ClusterPhaseRunning)
	// 状态管理现在由Reconciler负责，这里不再更新
	// if err := c.updateCRStatus(); err != nil {
	// 	c.logger.Error(err, "failed to update cluster phase to running")
	// 	return
	// }

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
				c.cluster = event.cluster
				c.logger.Info("cluster spec updated")
			}
		case <-time.After(reconcileInterval):
			start := time.Now()

			running, pending, err := c.pollPods()
			if err != nil {
				c.logger.Error(err, "failed to poll pods")
				continue
			}

			if len(pending) > 0 {
				c.logger.Info("skip reconciliation: pods are pending",
					"running", k8s.GetPodNames(running),
					"pending", k8s.GetPodNames(pending))
				continue
			}

			if len(running) == 0 {
				c.logger.Info("all etcd pods are dead")
				break
			}

			if c.members == nil {
				c.members = podsToMemberSet(running, c.isSecureClient())
			}

			rerr = c.reconcile(running)
			if rerr != nil {
				c.logger.Error(rerr, "failed to reconcile")
				break
			}

			// 状态管理现在由Reconciler负责，这里不再更新
			c.updateMemberStatus(running)
			// if err := c.updateCRStatus(); err != nil {
			// 	c.logger.Error(err, "periodic update CR status failed")
			// }

			c.logger.V(1).Info("reconcile completed", "duration", time.Since(start))
		}

		if rerr != nil {
			if etcd.IsFatalError(rerr) {
				c.status.SetReason(rerr.Error())
				c.logger.Error(rerr, "cluster failed")
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

// reportFailedStatus 报告失败状态
func (c *Cluster) reportFailedStatus() {
	c.status.SetPhase(etcdv1alpha1.ClusterPhaseFailed)
	// 状态管理现在由Reconciler负责，这里不再更新
	// if err := c.updateCRStatus(); err != nil {
	// 	c.logger.Error(err, "failed to update failed status")
	// }
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

// updateMemberStatus 更新成员状态
func (c *Cluster) updateMemberStatus(running []*corev1.Pod) {
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
