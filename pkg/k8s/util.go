package k8s

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func LabelsForCluster(clusterName string) map[string]string {
	return map[string]string{
		"etcd_cluster": clusterName,
		"app":          "etcd",
	}
}

func IsKubernetesResourceNotFoundError(err error) bool {
	return apierrors.IsNotFound(err)
}
