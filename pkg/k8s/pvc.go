package k8s

import (
	"context"
	"fmt"

	"github.com/your-org/etcd-k8s-operator/pkg/etcd"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func PVCNameFromMember(memberName string) string {
	return memberName
}

func NewEtcdPodPVC(member *etcd.Member, spec corev1.PersistentVolumeClaimSpec, clusterName, namespace string, owner metav1.OwnerReference) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PVCNameFromMember(member.Name),
			Namespace: namespace,
			Labels:    LabelsForCluster(clusterName),
		},
		Spec: spec,
	}
	pvc.OwnerReferences = append(pvc.OwnerReferences, owner)
	return pvc
}

func CreatePVC(kubecli kubernetes.Interface, pvc *corev1.PersistentVolumeClaim) error {
	_, err := kubecli.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(context.TODO(), pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PVC %s: %v", pvc.Name, err)
	}
	return nil
}

func DeletePVC(kubecli kubernetes.Interface, namespace, pvcName string) error {
	err := kubecli.CoreV1().PersistentVolumeClaims(namespace).Delete(context.TODO(), pvcName, metav1.DeleteOptions{})
	if err != nil && !IsKubernetesResourceNotFoundError(err) {
		return fmt.Errorf("failed to delete PVC %s: %v", pvcName, err)
	}
	return nil
}