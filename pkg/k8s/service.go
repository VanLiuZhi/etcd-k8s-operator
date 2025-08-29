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
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	// TolerateUnreadyEndpointsAnnotation 容忍未就绪端点的注解
	TolerateUnreadyEndpointsAnnotation = "service.alpha.kubernetes.io/tolerate-unready-endpoints"
)

// ClientServiceName 返回客户端服务名称
func ClientServiceName(clusterName string) string {
	return clusterName + "-client"
}

// CreateClientService 创建etcd客户端服务
func CreateClientService(ctx context.Context, kubecli kubernetes.Interface, clusterName, ns string, owner metav1.OwnerReference) error {
	ports := []corev1.ServicePort{{
		Name:       "client",
		Port:       EtcdClientPort,
		TargetPort: intstr.FromInt(EtcdClientPort),
		Protocol:   corev1.ProtocolTCP,
	}}
	return createService(ctx, kubecli, ClientServiceName(clusterName), clusterName, ns, "", ports, owner)
}

// CreatePeerService 创建etcd peer服务
func CreatePeerService(ctx context.Context, kubecli kubernetes.Interface, clusterName, ns string, owner metav1.OwnerReference) error {
	ports := []corev1.ServicePort{{
		Name:       "client",
		Port:       EtcdClientPort,
		TargetPort: intstr.FromInt(EtcdClientPort),
		Protocol:   corev1.ProtocolTCP,
	}, {
		Name:       "peer",
		Port:       EtcdPeerPort,
		TargetPort: intstr.FromInt(EtcdPeerPort),
		Protocol:   corev1.ProtocolTCP,
	}}

	return createService(ctx, kubecli, clusterName, clusterName, ns, corev1.ClusterIPNone, ports, owner)
}

// createService 创建服务的内部函数
func createService(ctx context.Context, kubecli kubernetes.Interface, svcName, clusterName, ns, clusterIP string, ports []corev1.ServicePort, owner metav1.OwnerReference) error {
	svc := newEtcdServiceManifest(svcName, clusterName, clusterIP, ports)
	addOwnerRefToObject(svc, owner)

	_, err := kubecli.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// newEtcdServiceManifest 创建etcd服务清单
func newEtcdServiceManifest(svcName, clusterName, clusterIP string, ports []corev1.ServicePort) *corev1.Service {
	labels := LabelsForCluster(clusterName)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   svcName,
			Labels: labels,
			Annotations: map[string]string{
				TolerateUnreadyEndpointsAnnotation: "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports:     ports,
			Selector:  labels,
			ClusterIP: clusterIP,
		},
	}
	return svc
}

// DeleteService 删除服务
func DeleteService(ctx context.Context, kubecli kubernetes.Interface, serviceName, namespace string) error {
	err := kubecli.CoreV1().Services(namespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// IsKubernetesResourceAlreadyExistError 检查是否为资源已存在错误
func IsKubernetesResourceAlreadyExistError(err error) bool {
	return apierrors.IsAlreadyExists(err)
}

// CascadeDeleteOptions 返回级联删除选项
func CascadeDeleteOptions(gracePeriodSeconds int64) *metav1.DeleteOptions {
	return &metav1.DeleteOptions{
		GracePeriodSeconds: func(t int64) *int64 { return &t }(gracePeriodSeconds),
		PropagationPolicy: func() *metav1.DeletionPropagation {
			foreground := metav1.DeletePropagationForeground
			return &foreground
		}(),
	}
}

// DeletePods 删除Pods
func DeletePods(ctx context.Context, kubecli kubernetes.Interface, namespace string, labels map[string]string, gracePeriodSeconds int64) error {
	selector := metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(
			&metav1.LabelSelector{MatchLabels: labels},
		),
	}
	return kubecli.CoreV1().Pods(namespace).DeleteCollection(ctx, *CascadeDeleteOptions(gracePeriodSeconds), selector)
}

// CreatePod 创建Pod
func CreatePod(kubecli kubernetes.Interface, ns string, pod *corev1.Pod) error {
	_, err := kubecli.CoreV1().Pods(ns).Create(context.TODO(), pod, metav1.CreateOptions{})
	return err
}

// DeletePod 删除Pod
func DeletePod(kubecli kubernetes.Interface, ns, podName string) error {
	return kubecli.CoreV1().Pods(ns).Delete(context.TODO(), podName, metav1.DeleteOptions{})
}
