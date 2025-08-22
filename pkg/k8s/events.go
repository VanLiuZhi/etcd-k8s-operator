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

package k8s

import (
	"fmt"
	"time"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewMemberAddEvent 创建成员添加事件
func NewMemberAddEvent(memberName string, cluster *etcdv1alpha1.EtcdCluster) *corev1.Event {
	event := newClusterEvent(cluster)
	event.Type = corev1.EventTypeNormal
	event.Reason = "New Member Added"
	event.Message = fmt.Sprintf("New member %s added to cluster", memberName)
	return event
}

// NewMemberRemoveEvent 创建成员移除事件
func NewMemberRemoveEvent(memberName string, cluster *etcdv1alpha1.EtcdCluster) *corev1.Event {
	event := newClusterEvent(cluster)
	event.Type = corev1.EventTypeNormal
	event.Reason = "Member Removed"
	event.Message = fmt.Sprintf("Member %s removed from cluster", memberName)
	return event
}

// NewClusterCreatedEvent 创建集群创建事件
func NewClusterCreatedEvent(cluster *etcdv1alpha1.EtcdCluster) *corev1.Event {
	event := newClusterEvent(cluster)
	event.Type = corev1.EventTypeNormal
	event.Reason = "Cluster Created"
	event.Message = fmt.Sprintf("Etcd cluster %s created", cluster.Name)
	return event
}

// NewClusterFailedEvent 创建集群失败事件
func NewClusterFailedEvent(cluster *etcdv1alpha1.EtcdCluster, reason string) *corev1.Event {
	event := newClusterEvent(cluster)
	event.Type = corev1.EventTypeWarning
	event.Reason = "Cluster Failed"
	event.Message = fmt.Sprintf("Etcd cluster %s failed: %s", cluster.Name, reason)
	return event
}

// NewClusterScalingEvent 创建集群扩缩容事件
func NewClusterScalingEvent(cluster *etcdv1alpha1.EtcdCluster, from, to int) *corev1.Event {
	event := newClusterEvent(cluster)
	event.Type = corev1.EventTypeNormal
	event.Reason = "Cluster Scaling"
	event.Message = fmt.Sprintf("Etcd cluster %s scaling from %d to %d members", cluster.Name, from, to)
	return event
}

// NewMemberFailedEvent 创建成员失败事件
func NewMemberFailedEvent(memberName string, cluster *etcdv1alpha1.EtcdCluster, reason string) *corev1.Event {
	event := newClusterEvent(cluster)
	event.Type = corev1.EventTypeWarning
	event.Reason = "Member Failed"
	event.Message = fmt.Sprintf("Member %s failed: %s", memberName, reason)
	return event
}

// NewClusterRecoveryEvent 创建集群恢复事件
func NewClusterRecoveryEvent(cluster *etcdv1alpha1.EtcdCluster) *corev1.Event {
	event := newClusterEvent(cluster)
	event.Type = corev1.EventTypeNormal
	event.Reason = "Cluster Recovery"
	event.Message = fmt.Sprintf("Etcd cluster %s recovery started", cluster.Name)
	return event
}

// newClusterEvent 创建集群事件的基础函数
func newClusterEvent(cluster *etcdv1alpha1.EtcdCluster) *corev1.Event {
	t := time.Now()
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: cluster.Name + "-",
			Namespace:    cluster.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			APIVersion:      cluster.APIVersion,
			Kind:            cluster.Kind,
			Name:            cluster.Name,
			Namespace:       cluster.Namespace,
			UID:             cluster.UID,
			ResourceVersion: cluster.ResourceVersion,
		},
		FirstTimestamp: metav1.Time{Time: t},
		LastTimestamp:  metav1.Time{Time: t},
		Count:          1,
		Source: corev1.EventSource{
			Component: "etcd-operator",
		},
	}
}

// GetEventInvolvedObject 获取事件涉及的对象引用
func GetEventInvolvedObject(obj runtime.Object) corev1.ObjectReference {
	switch o := obj.(type) {
	case *etcdv1alpha1.EtcdCluster:
		return corev1.ObjectReference{
			APIVersion:      o.APIVersion,
			Kind:            o.Kind,
			Name:            o.Name,
			Namespace:       o.Namespace,
			UID:             o.UID,
			ResourceVersion: o.ResourceVersion,
		}
	case *corev1.Pod:
		return corev1.ObjectReference{
			APIVersion:      "v1",
			Kind:            "Pod",
			Name:            o.Name,
			Namespace:       o.Namespace,
			UID:             o.UID,
			ResourceVersion: o.ResourceVersion,
		}
	default:
		return corev1.ObjectReference{}
	}
}
