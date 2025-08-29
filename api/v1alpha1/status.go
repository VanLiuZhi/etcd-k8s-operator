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

package v1alpha1

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// SetPhase 设置集群阶段
func (cs *ClusterStatus) SetPhase(p ClusterPhase) {
	cs.Phase = p
}

// SetReason 设置状态原因
func (cs *ClusterStatus) SetReason(r string) {
	cs.Reason = r
}

// IsFailed 检查集群是否失败
func (cs *ClusterStatus) IsFailed() bool {
	return cs.Phase == ClusterPhaseFailed
}

// SetReadyCondition 设置就绪条件
func (cs *ClusterStatus) SetReadyCondition() {
	c := newClusterCondition(ClusterConditionAvailable, corev1.ConditionTrue, "Cluster available", "")
	cs.setClusterCondition(*c)
}

// SetScalingUpCondition 设置扩容条件
func (cs *ClusterStatus) SetScalingUpCondition(from, to int) {
	c := newClusterCondition(ClusterConditionScaling, corev1.ConditionTrue,
		"Scaling up", scalingMsg(from, to))
	cs.setClusterCondition(*c)
}

// SetScalingDownCondition 设置缩容条件
func (cs *ClusterStatus) SetScalingDownCondition(from, to int) {
	c := newClusterCondition(ClusterConditionScaling, corev1.ConditionTrue,
		"Scaling down", scalingMsg(from, to))
	cs.setClusterCondition(*c)
}

// SetRecoveringCondition 设置恢复条件
func (cs *ClusterStatus) SetRecoveringCondition() {
	c := newClusterCondition(ClusterConditionRecovering, corev1.ConditionTrue,
		"Disaster recovery", "")
	cs.setClusterCondition(*c)
}

// SetUpgradingCondition 设置升级条件
func (cs *ClusterStatus) SetUpgradingCondition(to string) {
	c := newClusterCondition(ClusterConditionUpgrading, corev1.ConditionTrue,
		"Cluster upgrading", fmt.Sprintf("upgrading to %s", to))
	cs.setClusterCondition(*c)
}

// ClearCondition 清除指定类型的条件
func (cs *ClusterStatus) ClearCondition(t ClusterConditionType) {
	pos, _ := getClusterCondition(cs, t)
	if pos == -1 {
		return
	}
	cs.Conditions = append(cs.Conditions[:pos], cs.Conditions[pos+1:]...)
}

// setClusterCondition 设置集群条件
func (cs *ClusterStatus) setClusterCondition(newCondition ClusterCondition) {
	pos, cp := getClusterCondition(cs, newCondition.Type)
	if cp != nil &&
		cp.Status == newCondition.Status && cp.Reason == newCondition.Reason {
		return
	}

	if cp != nil {
		cs.Conditions[pos] = newCondition
	} else {
		cs.Conditions = append(cs.Conditions, newCondition)
	}
}

// getClusterCondition 获取集群条件
func getClusterCondition(status *ClusterStatus, t ClusterConditionType) (int, *ClusterCondition) {
	for i, c := range status.Conditions {
		if t == c.Type {
			return i, &c
		}
	}
	return -1, nil
}

// newClusterCondition 创建新的集群条件
func newClusterCondition(condType ClusterConditionType, status corev1.ConditionStatus, reason, message string) *ClusterCondition {
	now := time.Now().Format(time.RFC3339)
	return &ClusterCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     now,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}
}

// scalingMsg 生成扩缩容消息
func scalingMsg(from, to int) string {
	return fmt.Sprintf("Current cluster size: %d, desired cluster size: %d", from, to)
}
