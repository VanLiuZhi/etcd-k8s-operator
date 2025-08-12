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
	return BuildStatefulSetWithReplicas(cluster, cluster.Spec.Size)
}

// BuildStatefulSetWithReplicas creates a StatefulSet with specified replica count
func BuildStatefulSetWithReplicas(cluster *etcdv1alpha1.EtcdCluster, replicas int32) *appsv1.StatefulSet {
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
			Replicas:    &replicas,
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
	// Build init containers for all clusters (single and multi-node)
	var initContainers []corev1.Container
	initContainers = append(initContainers, buildEtcdInitContainer(cluster))

	containers := []corev1.Container{
		buildEtcdContainer(cluster, 0), // StatefulSet 模板中使用默认配置
	}

	// Add netshoot sidecar container for debugging (always available for troubleshooting)
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

	// Build volumes for all clusters (single and multi-node)
	var volumes []corev1.Volume
	volumes = append(volumes, corev1.Volume{
		Name: "etcd-config",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	return corev1.PodSpec{
		InitContainers:                initContainers,
		Containers:                    containers,
		Volumes:                       volumes,
		RestartPolicy:                 corev1.RestartPolicyAlways,
		TerminationGracePeriodSeconds: &[]int64{30}[0],
		DNSPolicy:                     corev1.DNSClusterFirst,
		SecurityContext: &corev1.PodSecurityContext{
			FSGroup: &[]int64{1000}[0],
		},
	}
}

// buildEtcdContainer creates the etcd container specification
func buildEtcdContainer(cluster *etcdv1alpha1.EtcdCluster, podIndex int) corev1.Container {
	image := fmt.Sprintf("%s:%s", cluster.Spec.Repository, cluster.Spec.Version)

	container := corev1.Container{
		Name:  "etcd",
		Image: image,
		// 使用配置文件启动 etcd（由 Init Container 生成）
		Command: []string{"/usr/local/bin/etcd"},
		Args:    []string{"--config-file=/etc/etcd/etcd.conf"},
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
		Env: []corev1.EnvVar{
			{
				Name: "HOSTNAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
		},
		VolumeMounts:   buildVolumeMounts(cluster),
		LivenessProbe:  buildLivenessProbe(cluster),
		ReadinessProbe: buildReadinessProbe(cluster),
		Resources:      buildResourceRequirements(cluster),
	}

	// 对于多节点集群，不使用启动脚本（官方镜像没有 shell）
	// 而是通过环境变量覆盖来实现动态配置

	return container
}

// buildVolumeMounts creates volume mounts for etcd container
func buildVolumeMounts(cluster *etcdv1alpha1.EtcdCluster) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      "data",
			MountPath: utils.EtcdDataDir,
		},
	}

	// Add etcd config mount for all clusters
	mounts = append(mounts, corev1.VolumeMount{
		Name:      "etcd-config",
		MountPath: "/etc/etcd",
		ReadOnly:  true,
	})

	return mounts
}

// buildEtcdEnvironment 创建etcd的环境变量
func buildEtcdEnvironment(cluster *etcdv1alpha1.EtcdCluster, podIndex int) []corev1.EnvVar {
	// 基础环境变量 - 适用于官方镜像
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
			Name: "POD_NAME",
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
			Value: getInitialClusterState(cluster, podIndex),
		},
		{
			Name:  "ETCD_INITIAL_CLUSTER_TOKEN",
			Value: cluster.Name,
		},
	}

	// 根据节点索引和集群大小决定初始集群配置
	envVars = append(envVars, corev1.EnvVar{
		Name:  "ETCD_INITIAL_CLUSTER",
		Value: buildInitialClusterForNode(cluster, podIndex),
	})

	// 官方镜像配置 - 使用标准 etcd 环境变量
	// 官方镜像不需要额外的环境变量，使用标准配置即可

	return envVars
}

// buildLivenessProbe 创建存活检查探针
func buildLivenessProbe(cluster *etcdv1alpha1.EtcdCluster) *corev1.Probe {
	// 官方镜像使用 etcdctl 进行健康检查
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"etcdctl",
					"--endpoints=http://localhost:2379",
					"endpoint",
					"health",
				},
			},
		},
		InitialDelaySeconds: 30,
		PeriodSeconds:       10,
		TimeoutSeconds:      5,
		FailureThreshold:    3,
	}
}

// buildReadinessProbe 创建就绪检查探针
func buildReadinessProbe(cluster *etcdv1alpha1.EtcdCluster) *corev1.Probe {
	// 官方镜像的就绪检查策略
	if cluster.Spec.Size > 1 {
		// 多节点集群使用 TCP 探针，避免循环依赖问题
		// etcd 进程启动后 Pod 就变为就绪，这样可以创建 DNS 记录
		return &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{
					Port: intstr.FromInt(utils.EtcdClientPort),
				},
			},
			InitialDelaySeconds: 10, // 等待 etcd 进程启动
			PeriodSeconds:       5,
			TimeoutSeconds:      3,
			FailureThreshold:    5, // 更宽容的失败阈值
		}
	}

	// 单节点集群使用健康检查
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{
					"etcdctl",
					"--endpoints=http://localhost:2379",
					"endpoint",
					"health",
				},
			},
		},
		InitialDelaySeconds: 15, // 官方镜像启动较快
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

// buildDynamicInitialCluster creates initial cluster configuration for dynamic multi-node setup
// For multi-node clusters, the first node starts as a single-node cluster
// Other nodes will be added dynamically through etcd member management API
func buildDynamicInitialCluster(cluster *etcdv1alpha1.EtcdCluster) string {
	// 对于多节点集群，第一个节点以单节点模式启动
	// 这样避免了等待其他节点的问题
	firstNodeName := fmt.Sprintf("%s-0", cluster.Name)
	firstNodeURL := fmt.Sprintf("http://%s.%s-peer.%s.svc.cluster.local:%d",
		firstNodeName, cluster.Name, cluster.Namespace, utils.EtcdPeerPort)

	// 只返回第一个节点的配置，其他节点将通过动态扩容添加
	return fmt.Sprintf("%s=%s", firstNodeName, firstNodeURL)
}

// buildEtcdInitContainer creates an init container for multi-node etcd setup
func buildEtcdInitContainer(cluster *etcdv1alpha1.EtcdCluster) corev1.Container {
	script := `#!/bin/sh
set -e

# 获取当前节点信息
HOSTNAME=$(hostname)
POD_INDEX=$(echo $HOSTNAME | sed 's/.*-//')

echo "Current hostname: $HOSTNAME"
echo "Pod index: $POD_INDEX"

# 创建配置目录
mkdir -p /etc/etcd

# 根据节点索引设置集群配置
if [ "$POD_INDEX" = "0" ]; then
    # 第一个节点：使用 new 模式启动单节点集群
    echo "Configuring as first node (new cluster)"
    cat > /etc/etcd/etcd.conf << EOF
# etcd configuration for $HOSTNAME
name: $HOSTNAME
data-dir: /data
listen-client-urls: http://0.0.0.0:2379
listen-peer-urls: http://0.0.0.0:2380
advertise-client-urls: http://$HOSTNAME.` + cluster.Name + `-peer.` + cluster.Namespace + `.svc.cluster.local:2379
initial-advertise-peer-urls: http://$HOSTNAME.` + cluster.Name + `-peer.` + cluster.Namespace + `.svc.cluster.local:2380
initial-cluster-token: ` + cluster.Name + `
initial-cluster-state: new
initial-cluster: $HOSTNAME=http://$HOSTNAME.` + cluster.Name + `-peer.` + cluster.Namespace + `.svc.cluster.local:2380
EOF
else
    # 后续节点：使用 existing 模式，但只包含第一个节点
    # 控制器会在启动前添加这个节点到集群中
    echo "Configuring as additional node (joining existing cluster)"

    # 等待第一个节点就绪
    echo "Waiting for first node to be ready..."
    while ! nslookup ` + cluster.Name + `-0.` + cluster.Name + `-peer.` + cluster.Namespace + `.svc.cluster.local; do
        echo "First node not ready, waiting..."
        sleep 2
    done

    # 构建包含从0到POD_INDEX的所有成员的列表
    # 控制器会确保在Pod就绪后才添加etcd成员，所以这个配置是安全的
    echo "Building initial cluster configuration for $HOSTNAME (index: $POD_INDEX)"

    members=""
    for i in $(seq 0 $POD_INDEX); do
        m="` + cluster.Name + `-$i=http://` + cluster.Name + `-$i.` + cluster.Name + `-peer.` + cluster.Namespace + `.svc.cluster.local:2380"
        if [ -z "$members" ]; then
            members="$m"
        else
            members="$members,$m"
        fi
    done

    echo "Using member list: $members"


    cat > /etc/etcd/etcd.conf << EOF
# etcd configuration for $HOSTNAME
name: $HOSTNAME
data-dir: /data
listen-client-urls: http://0.0.0.0:2379
listen-peer-urls: http://0.0.0.0:2380
advertise-client-urls: http://$HOSTNAME.` + cluster.Name + `-peer.` + cluster.Namespace + `.svc.cluster.local:2379
initial-advertise-peer-urls: http://$HOSTNAME.` + cluster.Name + `-peer.` + cluster.Namespace + `.svc.cluster.local:2380
initial-cluster-token: ` + cluster.Name + `
initial-cluster-state: existing
initial-cluster: $members
EOF

    echo "Configuration completed for additional node"
fi

echo "Generated etcd configuration:"
cat /etc/etcd/etcd.conf
echo "Init container completed successfully"
`

	return corev1.Container{
		Name:    "etcd-init",
		Image:   "busybox:1.35",
		Command: []string{"/bin/sh", "-c", script},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "etcd-config",
				MountPath: "/etc/etcd",
			},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("32Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
		},
	}
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

// BuildNodePortService 创建NodePort服务用于外部访问
// 此服务负载均衡到所有健康的etcd节点，提供稳定的外部连接
func BuildNodePortService(cluster *etcdv1alpha1.EtcdCluster) *corev1.Service {
	labels := utils.LabelsForEtcdService(cluster, "nodeport")

	// 使用标准的集群选择器，负载均衡到所有健康的 etcd 节点
	selectorLabels := utils.SelectorLabelsForEtcdCluster(cluster)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-nodeport", cluster.Name),
			Namespace:   cluster.Namespace,
			Labels:      labels,
			Annotations: utils.AnnotationsForEtcdCluster(cluster),
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: selectorLabels,
			Ports: []corev1.ServicePort{
				{
					Name:       "client",
					Port:       utils.EtcdClientPort,
					TargetPort: intstr.FromInt(utils.EtcdClientPort),
					Protocol:   corev1.ProtocolTCP,
					NodePort:   30379, // 固定的 NodePort，方便 operator 连接
				},
			},
		},
	}
}

// BuildConfigMap 创建etcd配置的ConfigMap
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

// getInitialClusterState determines the initial cluster state based on node index
func getInitialClusterState(cluster *etcdv1alpha1.EtcdCluster, podIndex int) string {
	if cluster.Spec.Size == 1 {
		// 单节点集群始终使用 "new"
		return "new"
	}

	// 多节点集群：第一个节点使用 "new"，其他节点使用 "existing"
	if podIndex == 0 {
		return "new"
	}
	return "existing"
}

// buildInitialClusterForNode creates initial cluster configuration for a specific node
func buildInitialClusterForNode(cluster *etcdv1alpha1.EtcdCluster, podIndex int) string {
	if cluster.Spec.Size == 1 {
		// 单节点集群
		return buildInitialCluster(cluster)
	}

	// 多节点集群：根据节点索引构建正确的初始集群配置
	if podIndex == 0 {
		// 第一个节点：只包含自己
		firstNodeName := fmt.Sprintf("%s-0", cluster.Name)
		firstNodeURL := fmt.Sprintf("http://%s.%s-peer.%s.svc.cluster.local:%d",
			firstNodeName, cluster.Name, cluster.Namespace, utils.EtcdPeerPort)
		return fmt.Sprintf("%s=%s", firstNodeName, firstNodeURL)
	} else {
		// 后续节点：包含从0到当前节点的所有节点
		var members []string
		for i := 0; i <= podIndex; i++ {
			memberName := fmt.Sprintf("%s-%d", cluster.Name, i)
			memberURL := fmt.Sprintf("http://%s.%s-peer.%s.svc.cluster.local:%d",
				memberName, cluster.Name, cluster.Namespace, utils.EtcdPeerPort)
			members = append(members, fmt.Sprintf("%s=%s", memberName, memberURL))
		}
		return strings.Join(members, ",")
	}
}
