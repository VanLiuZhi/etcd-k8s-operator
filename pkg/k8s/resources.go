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
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	"github.com/your-org/etcd-k8s-operator/pkg/utils"
)

// BuildStatefulSet creates a StatefulSet for the EtcdCluster
func BuildStatefulSet(cluster *etcdv1alpha1.EtcdCluster) *appsv1.StatefulSet {
	labels := utils.LabelsForEtcdCluster(cluster)
	selectorLabels := utils.SelectorLabelsForEtcdCluster(cluster)

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cluster.Name,
			Namespace:   cluster.Namespace,
			Labels:      labels,
			Annotations: utils.AnnotationsForEtcdCluster(cluster),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    &cluster.Spec.Size,
			ServiceName: fmt.Sprintf("%s-peer", cluster.Name),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: utils.AnnotationsForEtcdCluster(cluster),
				},
				Spec: buildPodSpec(cluster),
			},
			VolumeClaimTemplates: buildVolumeClaimTemplates(cluster),
			PodManagementPolicy:  appsv1.ParallelPodManagement,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
		},
	}

	return sts
}

// buildPodSpec creates the pod specification for etcd
func buildPodSpec(cluster *etcdv1alpha1.EtcdCluster) corev1.PodSpec {
	containers := []corev1.Container{
		buildEtcdContainer(cluster),
	}

	// Add netshoot sidecar container for debugging if using Bitnami image
	if strings.Contains(cluster.Spec.Repository, "bitnami") {
		netshootContainer := corev1.Container{
			Name:    "netshoot",
			Image:   "nicolaka/netshoot:latest",
			Command: []string{"sleep", "3600"},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
		}
		containers = append(containers, netshootContainer)
	}

	return corev1.PodSpec{
		Containers:                    containers,
		RestartPolicy:                 corev1.RestartPolicyAlways,
		TerminationGracePeriodSeconds: &[]int64{30}[0],
		DNSPolicy:                     corev1.DNSClusterFirst,
		SecurityContext: &corev1.PodSecurityContext{
			FSGroup: &[]int64{1000}[0],
		},
	}
}

// buildEtcdContainer creates the etcd container specification
func buildEtcdContainer(cluster *etcdv1alpha1.EtcdCluster) corev1.Container {
	image := fmt.Sprintf("%s:%s", cluster.Spec.Repository, cluster.Spec.Version)

	container := corev1.Container{
		Name:  "etcd",
		Image: image,
		Ports: []corev1.ContainerPort{
			{
				Name:          "client",
				ContainerPort: utils.EtcdClientPort,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          "peer",
				ContainerPort: utils.EtcdPeerPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: buildEtcdEnvironment(cluster),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "data",
				MountPath: utils.EtcdDataDir,
			},
		},
		LivenessProbe:  buildLivenessProbe(cluster),
		ReadinessProbe: buildReadinessProbe(cluster),
		Resources:      buildResourceRequirements(cluster),
	}

	return container
}

// buildEtcdEnvironment creates environment variables for etcd
func buildEtcdEnvironment(cluster *etcdv1alpha1.EtcdCluster) []corev1.EnvVar {
	envVars := []corev1.EnvVar{
		{
			Name: "ETCD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name:  "ETCD_DATA_DIR",
			Value: utils.EtcdDataDir,
		},
		{
			Name:  "ETCD_LISTEN_CLIENT_URLS",
			Value: fmt.Sprintf("http://0.0.0.0:%d", utils.EtcdClientPort),
		},
		{
			Name:  "ETCD_LISTEN_PEER_URLS",
			Value: fmt.Sprintf("http://0.0.0.0:%d", utils.EtcdPeerPort),
		},
		{
			Name:  "ETCD_ADVERTISE_CLIENT_URLS",
			Value: fmt.Sprintf("http://$(ETCD_NAME).%s-peer.%s.svc.cluster.local:%d", cluster.Name, cluster.Namespace, utils.EtcdClientPort),
		},
		{
			Name:  "ETCD_INITIAL_ADVERTISE_PEER_URLS",
			Value: fmt.Sprintf("http://$(ETCD_NAME).%s-peer.%s.svc.cluster.local:%d", cluster.Name, cluster.Namespace, utils.EtcdPeerPort),
		},
		{
			Name:  "ETCD_INITIAL_CLUSTER_STATE",
			Value: "new", // 对于新集群，所有节点都使用 "new"
		},
		{
			Name:  "ETCD_INITIAL_CLUSTER_TOKEN",
			Value: cluster.Name,
		},
		{
			Name:  "ETCD_INITIAL_CLUSTER",
			Value: buildInitialCluster(cluster),
		},
	}

	// Add Bitnami-specific environment variables if using Bitnami image
	if strings.Contains(cluster.Spec.Repository, "bitnami") {
		bitnamiEnvVars := []corev1.EnvVar{
			{
				Name:  "ALLOW_NONE_AUTHENTICATION",
				Value: "yes",
			},
			{
				Name:  "ETCD_ROOT_PASSWORD",
				Value: "",
			},
			{
				Name:  "MY_STS_NAME",
				Value: cluster.Name,
			},
		}

		// For multi-node clusters, we need special handling
		if cluster.Spec.Size > 1 {
			// Use a different approach for multi-node clusters
			bitnamiEnvVars = append(bitnamiEnvVars, []corev1.EnvVar{
				{
					Name:  "ETCD_ON_K8S",
					Value: "yes",
				},
				{
					Name:  "ETCD_CLUSTER_DOMAIN",
					Value: fmt.Sprintf("%s-peer.%s.svc.cluster.local", cluster.Name, cluster.Namespace),
				},
				// Skip the headless service domain check for multi-node clusters
				{
					Name:  "ETCD_SKIP_DOMAIN_CHECK",
					Value: "yes",
				},
			}...)
		} else {
			// Single node configuration
			bitnamiEnvVars = append(bitnamiEnvVars, []corev1.EnvVar{
				{
					Name:  "ETCD_ON_K8S",
					Value: "yes",
				},
				{
					Name:  "ETCD_CLUSTER_DOMAIN",
					Value: fmt.Sprintf("%s-peer.%s.svc.cluster.local", cluster.Name, cluster.Namespace),
				},
			}...)
		}

		envVars = append(envVars, bitnamiEnvVars...)
	}

	return envVars
}

// buildLivenessProbe creates liveness probe for etcd container
func buildLivenessProbe(cluster *etcdv1alpha1.EtcdCluster) *corev1.Probe {
	if strings.Contains(cluster.Spec.Repository, "bitnami") {
		// Use Bitnami's healthcheck script
		return &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"/opt/bitnami/scripts/etcd/healthcheck.sh"},
				},
			},
			InitialDelaySeconds: 60, // Bitnami etcd takes longer to start
			PeriodSeconds:       10,
			TimeoutSeconds:      5,
			FailureThreshold:    3,
		}
	}

	// Use standard HTTP health check for other images
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/health",
				Port: intstr.FromInt(utils.EtcdClientPort),
			},
		},
		InitialDelaySeconds: 30,
		PeriodSeconds:       10,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// buildReadinessProbe creates readiness probe for etcd container
func buildReadinessProbe(cluster *etcdv1alpha1.EtcdCluster) *corev1.Probe {
	if strings.Contains(cluster.Spec.Repository, "bitnami") {
		// For multi-node clusters, use a more lenient readiness probe
		if cluster.Spec.Size > 1 {
			// Use TCP probe to allow Pod to become ready faster
			// This helps with DNS record creation for headless service
			return &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt(utils.EtcdClientPort),
					},
				},
				InitialDelaySeconds: 15, // Shorter delay for multi-node
				PeriodSeconds:       5,
				TimeoutSeconds:      3,
				FailureThreshold:    5, // More tolerant for multi-node startup
			}
		}

		// Use Bitnami's healthcheck script for single-node clusters
		return &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"/opt/bitnami/scripts/etcd/healthcheck.sh"},
				},
			},
			InitialDelaySeconds: 30, // Shorter delay for readiness
			PeriodSeconds:       5,
			TimeoutSeconds:      3,
			FailureThreshold:    3,
		}
	}

	// Use standard HTTP health check for other images
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/health",
				Port: intstr.FromInt(utils.EtcdClientPort),
			},
		},
		InitialDelaySeconds: 10,
		PeriodSeconds:       5,
		TimeoutSeconds:      3,
		FailureThreshold:    3,
	}
}

// buildInitialCluster creates the initial cluster configuration
func buildInitialCluster(cluster *etcdv1alpha1.EtcdCluster) string {
	var members []string
	for i := int32(0); i < cluster.Spec.Size; i++ {
		memberName := fmt.Sprintf("%s-%d", cluster.Name, i)
		memberURL := fmt.Sprintf("http://%s.%s-peer.%s.svc.cluster.local:%d",
			memberName, cluster.Name, cluster.Namespace, utils.EtcdPeerPort)
		members = append(members, fmt.Sprintf("%s=%s", memberName, memberURL))
	}
	return strings.Join(members, ",")
}

// buildResourceRequirements creates resource requirements for etcd container
func buildResourceRequirements(cluster *etcdv1alpha1.EtcdCluster) corev1.ResourceRequirements {
	// Use cluster-specific resources if provided, otherwise use defaults
	if cluster.Spec.Resources.Requests != nil || cluster.Spec.Resources.Limits != nil {
		return corev1.ResourceRequirements{
			Requests: cluster.Spec.Resources.Requests,
			Limits:   cluster.Spec.Resources.Limits,
		}
	}

	// Default resource requirements
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1000m"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}
}

// buildVolumeClaimTemplates creates volume claim templates for StatefulSet
func buildVolumeClaimTemplates(cluster *etcdv1alpha1.EtcdCluster) []corev1.PersistentVolumeClaim {
	storageSize := cluster.Spec.Storage.Size
	if storageSize.IsZero() {
		storageSize = resource.MustParse(utils.DefaultStorageSize)
	}

	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "data",
			Labels: utils.LabelsForEtcdCluster(cluster),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageSize,
				},
			},
		},
	}

	// Set storage class if specified
	if cluster.Spec.Storage.StorageClassName != nil {
		pvc.Spec.StorageClassName = cluster.Spec.Storage.StorageClassName
	}

	return []corev1.PersistentVolumeClaim{pvc}
}

// BuildClientService creates a client service for the EtcdCluster
func BuildClientService(cluster *etcdv1alpha1.EtcdCluster) *corev1.Service {
	labels := utils.LabelsForEtcdService(cluster, "client")
	selectorLabels := utils.SelectorLabelsForEtcdCluster(cluster)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-client", cluster.Name),
			Namespace:   cluster.Namespace,
			Labels:      labels,
			Annotations: utils.AnnotationsForEtcdCluster(cluster),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: selectorLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "client",
					Port:       utils.EtcdClientPort,
					TargetPort: intstr.FromInt(utils.EtcdClientPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// BuildPeerService creates a peer service for the EtcdCluster
func BuildPeerService(cluster *etcdv1alpha1.EtcdCluster) *corev1.Service {
	labels := utils.LabelsForEtcdService(cluster, "peer")
	selectorLabels := utils.SelectorLabelsForEtcdCluster(cluster)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-peer", cluster.Name),
			Namespace:   cluster.Namespace,
			Labels:      labels,
			Annotations: utils.AnnotationsForEtcdCluster(cluster),
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: corev1.ClusterIPNone, // Headless service
			Selector:  selectorLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "peer",
					Port:       utils.EtcdPeerPort,
					TargetPort: intstr.FromInt(utils.EtcdPeerPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// BuildConfigMap creates a ConfigMap for etcd configuration
func BuildConfigMap(cluster *etcdv1alpha1.EtcdCluster) *corev1.ConfigMap {
	labels := utils.LabelsForEtcdCluster(cluster)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-config", cluster.Name),
			Namespace:   cluster.Namespace,
			Labels:      labels,
			Annotations: utils.AnnotationsForEtcdCluster(cluster),
		},
		Data: map[string]string{
			"etcd.conf": buildEtcdConfig(cluster),
		},
	}
}

// buildEtcdConfig creates etcd configuration content
func buildEtcdConfig(cluster *etcdv1alpha1.EtcdCluster) string {
	config := fmt.Sprintf(`# etcd configuration for cluster %s
name: $(ETCD_NAME)
data-dir: %s
listen-client-urls: http://0.0.0.0:%d
listen-peer-urls: http://0.0.0.0:%d
advertise-client-urls: http://$(ETCD_NAME).%s-peer.%s.svc.cluster.local:%d
initial-advertise-peer-urls: http://$(ETCD_NAME).%s-peer.%s.svc.cluster.local:%d
initial-cluster-state: new
initial-cluster-token: %s
initial-cluster: %s
`,
		cluster.Name,
		utils.EtcdDataDir,
		utils.EtcdClientPort,
		utils.EtcdPeerPort,
		cluster.Name, cluster.Namespace, utils.EtcdClientPort,
		cluster.Name, cluster.Namespace, utils.EtcdPeerPort,
		cluster.Name,
		buildInitialCluster(cluster),
	)

	return config
}
