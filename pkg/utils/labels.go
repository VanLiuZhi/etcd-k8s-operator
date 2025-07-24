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

package utils

import (
	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
)

// LabelsForEtcdCluster returns the labels for selecting the resources
// belonging to the given EtcdCluster CR name.
func LabelsForEtcdCluster(cluster *etcdv1alpha1.EtcdCluster) map[string]string {
	return map[string]string{
		LabelAppName:      "etcd",
		LabelAppInstance:  cluster.Name,
		LabelAppComponent: "database",
		LabelAppManagedBy: "etcd-operator",
		LabelAppVersion:   cluster.Spec.Version,
		LabelEtcdCluster:  cluster.Name,
	}
}

// LabelsForEtcdMember returns the labels for selecting the resources
// belonging to the given EtcdCluster member.
func LabelsForEtcdMember(cluster *etcdv1alpha1.EtcdCluster, memberName string) map[string]string {
	labels := LabelsForEtcdCluster(cluster)
	labels[LabelEtcdMember] = memberName
	return labels
}

// LabelsForEtcdService returns the labels for etcd service
func LabelsForEtcdService(cluster *etcdv1alpha1.EtcdCluster, serviceType string) map[string]string {
	labels := LabelsForEtcdCluster(cluster)
	labels[LabelAppComponent] = serviceType // "client" or "peer"
	return labels
}

// SelectorLabelsForEtcdCluster returns the selector labels for EtcdCluster
func SelectorLabelsForEtcdCluster(cluster *etcdv1alpha1.EtcdCluster) map[string]string {
	return map[string]string{
		LabelAppName:     "etcd",
		LabelAppInstance: cluster.Name,
		LabelEtcdCluster: cluster.Name,
	}
}

// MergeLabels merges multiple label maps
func MergeLabels(labelMaps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, labelMap := range labelMaps {
		for k, v := range labelMap {
			result[k] = v
		}
	}
	return result
}

// AnnotationsForEtcdCluster returns the annotations for EtcdCluster resources
func AnnotationsForEtcdCluster(cluster *etcdv1alpha1.EtcdCluster) map[string]string {
	annotations := make(map[string]string)

	// Copy existing annotations from the cluster
	for k, v := range cluster.Annotations {
		annotations[k] = v
	}

	return annotations
}

// MergeAnnotations merges multiple annotation maps
func MergeAnnotations(annotationMaps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, annotationMap := range annotationMaps {
		for k, v := range annotationMap {
			result[k] = v
		}
	}
	return result
}
