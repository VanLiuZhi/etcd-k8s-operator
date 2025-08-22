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
	"fmt"

	"github.com/your-org/etcd-k8s-operator/pkg/etcd"
	"github.com/your-org/etcd-k8s-operator/pkg/k8s"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// updateMembers 从etcd集群更新成员信息
func (c *Cluster) updateMembers(known etcd.MemberSet) error {
	resp, err := etcd.ListMembers(known.ClientURLs(), c.tlsConfig)
	if err != nil {
		return err
	}

	members := etcd.MemberSet{}
	for _, m := range resp.Members {
		name, err := getMemberName(m, c.cluster.GetName())
		if err != nil {
			return fmt.Errorf("get member name failed: %v", err)
		}

		members[name] = &etcd.Member{
			Name:         name,
			Namespace:    c.cluster.Namespace,
			ID:           m.ID,
			SecurePeer:   c.isSecurePeer(),
			SecureClient: c.isSecureClient(),
		}
	}
	c.members = members
	return nil
}

// getMemberName 从etcd成员信息中获取成员名称
func getMemberName(m *etcdserverpb.Member, clusterName string) (string, error) {
	if len(m.PeerURLs) == 0 {
		return "", fmt.Errorf("member has no peer URLs")
	}

	name, err := etcd.MemberNameFromPeerURL(m.PeerURLs[0])
	if err != nil {
		return "", fmt.Errorf("invalid member peerURL (%s): %v", m.PeerURLs[0], err)
	}
	return name, nil
}

// podsToMemberSet 将Pod列表转换为成员集合
func podsToMemberSet(pods []*corev1.Pod, sc bool) etcd.MemberSet {
	members := etcd.MemberSet{}
	for _, pod := range pods {
		m := &etcd.Member{
			Name:         pod.Name,
			Namespace:    pod.Namespace,
			SecureClient: sc,
		}
		members.Add(m)
	}
	return members
}

// removePod 删除Pod
func (c *Cluster) removePod(name string) error {
	ctx := context.TODO()
	opts := k8s.CascadeDeleteOptions(podTerminationGracePeriod)
	err := c.config.KubeCli.CoreV1().Pods(c.cluster.Namespace).Delete(ctx, name, *opts)
	if err != nil && !k8s.IsKubernetesResourceNotFoundError(err) {
		return err
	}
	c.logger.Info("pod deleted", "pod", name)
	return nil
}

// removePVC 删除PVC
func (c *Cluster) removePVC(pvcName string) error {
	ctx := context.TODO()
	err := c.config.KubeCli.CoreV1().PersistentVolumeClaims(c.cluster.Namespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
	if err != nil && !k8s.IsKubernetesResourceNotFoundError(err) {
		return err
	}
	c.logger.Info("pvc deleted", "pvc", pvcName)
	return nil
}

// createPod 创建Pod
func (c *Cluster) createPod(members etcd.MemberSet, m *etcd.Member, state string) error {
	pod := k8s.NewEtcdPod(m, members.PeerURLPairs(), c.cluster.Name, state, "", c.cluster, c.cluster.AsOwner())

	// 处理持久化存储
	if c.isPodPVEnabled() {
		pvcSpec := c.cluster.Spec.Pod.PersistentVolumeClaimSpec
		if pvcSpec != nil {
			pvc := k8s.NewEtcdPodPVC(m, *pvcSpec, c.cluster.Name, c.cluster.Namespace, c.cluster.AsOwner())
			ctx := context.TODO()
			_, err := c.config.KubeCli.CoreV1().PersistentVolumeClaims(c.cluster.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
			if err != nil && !k8s.IsKubernetesResourceAlreadyExistError(err) {
				return fmt.Errorf("failed to create PVC: %v", err)
			}
			k8s.AddEtcdVolumeToPod(pod, pvc)
		}
	} else {
		k8s.AddEtcdVolumeToPod(pod, nil)
	}

	ctx := context.TODO()
	_, err := c.config.KubeCli.CoreV1().Pods(c.cluster.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create pod: %v", err)
	}

	c.logger.Info("pod created", "pod", m.Name)
	return nil
}

// isPodPVEnabled 检查是否启用Pod持久化卷
func (c *Cluster) isPodPVEnabled() bool {
	return c.cluster.Spec.Pod != nil && c.cluster.Spec.Pod.PersistentVolumeClaimSpec != nil
}

// removeMember 从集群中移除成员
func (c *Cluster) removeMember(toRemove *etcd.Member) error {
	c.logger.Info("removing member", "member", toRemove.Name)

	// 1. 从etcd集群移除成员
	err := etcd.RemoveMember(c.members.ClientURLs(), c.tlsConfig, toRemove.ID)
	if err != nil {
		return fmt.Errorf("failed to remove member from etcd cluster: %v", err)
	}

	// 2. 从内存状态移除
	c.members.Remove(toRemove.Name)

	// 3. 删除Kubernetes Pod
	if err := c.removePod(toRemove.Name); err != nil {
		return fmt.Errorf("failed to remove pod: %v", err)
	}

	// 4. 删除PVC (如果启用持久化)
	if c.isPodPVEnabled() {
		err = c.removePVC(k8s.PVCNameFromMember(toRemove.Name))
		if err != nil {
			return fmt.Errorf("failed to remove PVC: %v", err)
		}
	}

	// 5. 记录事件
	event := k8s.NewMemberRemoveEvent(toRemove.Name, c.cluster)
	c.config.Recorder.Event(c.cluster, event.Type, event.Reason, event.Message)

	c.logger.Info("member removed", "member", toRemove.Name)
	return nil
}

// removeDeadMember 移除死亡成员
func (c *Cluster) removeDeadMember(toRemove *etcd.Member) error {
	c.logger.Info("removing dead member", "member", toRemove.Name)
	return c.removeMember(toRemove)
}
