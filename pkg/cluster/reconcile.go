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
	"fmt"

	etcdv1alpha1 "github.com/etcd-lz/etcd-k8s-operator/api/v1alpha1"
	"github.com/etcd-lz/etcd-k8s-operator/pkg/etcd"
	"github.com/etcd-lz/etcd-k8s-operator/pkg/k8s"

	corev1 "k8s.io/api/core/v1"
)

// reconcile 协调集群当前状态到期望状态
// - 尝试将集群协调到期望大小
// - 如果集群需要升级，尝试逐个升级旧成员
func (c *Cluster) reconcile(pods []*corev1.Pod) error {
	c.logger.Info("Start reconciling")
	defer c.logger.Info("Finish reconciling")

	defer func() {
		c.status.Size = c.members.Size()
	}()

	sp := c.cluster.Spec
	running := podsToMemberSet(pods, c.isSecureClient())
	if !running.IsEqual(c.members) || c.members.Size() != sp.Size {
		return c.reconcileMembers(running)
	}

	// TODO: 检查是否需要升级
	// if needUpgrade(pods, c.cluster.Spec) {
	//     return c.upgradeOneMember(pods)
	// }

	c.status.SetReadyCondition()
	return nil
}

// reconcileMembers 协调成员
// - 运行中的pods和集群成员关系
// - 集群成员关系和etcd集群的期望大小
// 步骤:
// 1. 从运行集合中移除所有不属于成员集合的pod
// 2. L由运行中的剩余pod组成
// 3. 如果L = members，当前状态匹配成员状态。结束。
// 4. 如果len(L) < len(members)/2 + 1，返回法定人数丢失错误。
// 5. 添加一个缺失的成员。结束。
func (c *Cluster) reconcileMembers(running etcd.MemberSet) error {
	c.logger.Info("running members", "members", running.String())
	c.logger.Info("cluster membership", "members", c.members.String())

	unknownMembers := running.Diff(c.members)
	if unknownMembers.Size() > 0 {
		c.logger.Info("removing unexpected pods", "members", unknownMembers.String())
		for _, m := range unknownMembers {
			if err := c.removePod(m.Name); err != nil {
				return err
			}
		}
	}
	L := running.Diff(unknownMembers)

	if L.Size() == c.members.Size() {
		return c.resize()
	}

	if L.Size() < c.members.Size()/2+1 {
		return etcd.ErrLostQuorum
	}

	c.logger.Info("removing one dead member")
	// 在调整大小之前移除没有任何运行pod的死亡成员
	return c.removeDeadMember(c.members.Diff(L).PickOne())
}

// resize 调整集群大小
func (c *Cluster) resize() error {
	if c.members.Size() == c.cluster.Spec.Size {
		return nil
	}

	if c.members.Size() < c.cluster.Spec.Size {
		return c.addOneMember()
	}

	return c.removeOneMember()
}

// addOneMember 添加一个成员
func (c *Cluster) addOneMember() error {
	c.status.SetScalingUpCondition(c.members.Size(), c.cluster.Spec.Size)

	// 创建etcd客户端连接
	_, err := etcd.CreateClient(c.members.ClientURLs(), c.tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to create etcd client: %v", err)
	}

	// 生成新成员
	newMember := c.newMember()

	// 向etcd集群添加成员
	resp, err := etcd.AddMember(c.members.ClientURLs(), c.tlsConfig, []string{newMember.PeerURL()})
	if err != nil {
		return fmt.Errorf("failed to add new member (%s): %v", newMember.Name, err)
	}
	newMember.ID = resp.Member.ID
	c.members.Add(newMember)

	// 创建Kubernetes Pod
	if err := c.createPod(c.members, newMember, "existing"); err != nil {
		// 需要从etcd集群中移除已添加的成员
		etcd.RemoveMember(c.members.ClientURLs(), c.tlsConfig, newMember.ID)
		c.members.Remove(newMember.Name)
		return fmt.Errorf("failed to create member's pod (%s): %v", newMember.Name, err)
	}

	c.logger.Info("added member", "member", newMember.Name)

	// 记录事件
	event := k8s.NewMemberAddEvent(newMember.Name, c.cluster)
	c.config.Recorder.Event(c.cluster, event.Type, event.Reason, event.Message)

	return nil
}

// removeOneMember 移除一个成员
func (c *Cluster) removeOneMember() error {
	c.status.SetScalingDownCondition(c.members.Size(), c.cluster.Spec.Size)

	return c.removeMember(c.members.PickOne())
}

// needUpgrade 检查是否需要升级
// TODO: 实现升级逻辑
func needUpgrade(pods []*corev1.Pod, spec etcdv1alpha1.ClusterSpec) bool {
	return false
}

// upgradeOneMember 升级一个成员
// TODO: 实现升级逻辑
func (c *Cluster) upgradeOneMember(pods []*corev1.Pod) error {
	return nil
}

// removeDeadMember 移除死亡成员
func (c *Cluster) removeDeadMember(toRemove *etcd.Member) error {
	c.logger.Info("removing dead member", "member", toRemove.Name)
	event := k8s.ReplacingDeadMemberEvent(toRemove.Name, c.cluster)
	c.config.Recorder.Event(c.cluster, event.Type, event.Reason, event.Message)

	return c.removeMember(toRemove)
}

// removeMember 移除成员
func (c *Cluster) removeMember(toRemove *etcd.Member) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("remove member (%s) failed: %v", toRemove.Name, err)
		}
	}()

	// 从etcd集群中移除成员
	err = etcd.RemoveMember(c.members.ClientURLs(), c.tlsConfig, toRemove.ID)
	if err != nil {
		return err
	}
	c.members.Remove(toRemove.Name)

	// 记录事件
	event := k8s.NewMemberRemoveEvent(toRemove.Name, c.cluster)
	c.config.Recorder.Event(c.cluster, event.Type, event.Reason, event.Message)

	// 删除Pod
	if err := c.removePod(toRemove.Name); err != nil {
		return err
	}

	if c.isPodPVEnabled() {
		err = k8s.DeletePVC(c.config.KubeCli, c.cluster.Namespace, k8s.PVCNameFromMember(toRemove.Name))
		if err != nil {
			return err
		}
	}

	c.logger.Info("removed member", "member", toRemove.Name, "id", toRemove.ID)
	return nil
}

// createPod 创建Pod
func (c *Cluster) createPod(members etcd.MemberSet, m *etcd.Member, state string) error {
	pod := k8s.NewEtcdPod(m, members.PeerURLPairs(), c.cluster.Name, state, "", c.cluster, c.cluster.AsOwner())
	if c.isPodPVEnabled() {
		pvcSpec := c.cluster.Spec.Pod.PersistentVolumeClaimSpec
		pvc := k8s.NewEtcdPodPVC(m, *pvcSpec, c.cluster.Name, c.cluster.Namespace, c.cluster.AsOwner())
		err := k8s.CreatePVC(c.config.KubeCli, pvc)
		if err != nil {
			return err
		}
		k8s.AddEtcdVolumeToPod(pod, pvc)
	} else {
		k8s.AddEtcdVolumeToPod(pod, nil)
	}
	return k8s.CreatePod(c.config.KubeCli, c.cluster.Namespace, pod)
}

// removePod 删除Pod
func (c *Cluster) removePod(name string) error {
	return k8s.DeletePod(c.config.KubeCli, c.cluster.Namespace, name)
}

// podsToMemberSet 将pods转换为成员集合
func podsToMemberSet(pods []*corev1.Pod, secureClient bool) etcd.MemberSet {
	members := etcd.NewMemberSet()
	for _, pod := range pods {
		m := &etcd.Member{
			Name:         pod.Name,
			Namespace:    pod.Namespace,
			SecureClient: secureClient,
		}
		members.Add(m)
	}
	return members
}
