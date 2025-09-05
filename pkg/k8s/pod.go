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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	etcdv1alpha1 "github.com/etcd-lz/etcd-k8s-operator/api/v1alpha1"
	"github.com/etcd-lz/etcd-k8s-operator/pkg/etcd"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
)

const (
	// EtcdClientPort etcd客户端端口
	EtcdClientPort = 2379
	// EtcdPeerPort etcd peer端口
	EtcdPeerPort = 2380

	// etcd数据目录相关常量
	etcdVolumeName     = "etcd-data"
	etcdVolumeMountDir = "/var/etcd"
	dataDir            = etcdVolumeMountDir + "/data"

	// etcd版本注解键
	etcdVersionAnnotationKey = "etcd.version"

	// 随机后缀长度
	randomSuffixLength = 10
	// k8s对象名称最大长度
	maxNameLength = 63 - randomSuffixLength - 1

	// 默认etcd镜像
	defaultEtcdRepository = "quay.io/coreos/etcd"
	defaultEtcdVersion    = "v3.5.21"
)

// IsPodReady 检查Pod是否就绪
func IsPodReady(pod *corev1.Pod) bool {
	condition := getPodReadyCondition(&pod.Status)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

// getPodReadyCondition 获取Pod就绪条件
func getPodReadyCondition(status *corev1.PodStatus) *corev1.PodCondition {
	for i := range status.Conditions {
		if status.Conditions[i].Type == corev1.PodReady {
			return &status.Conditions[i]
		}
	}
	return nil
}

// GetPodNames 获取Pod名称列表
func GetPodNames(pods []*corev1.Pod) []string {
	if len(pods) == 0 {
		return nil
	}
	res := []string{}
	for _, p := range pods {
		res = append(res, p.Name)
	}
	return res
}

// GetEtcdVersion 获取Pod的etcd版本
func GetEtcdVersion(pod *corev1.Pod) string {
	return pod.Annotations[etcdVersionAnnotationKey]
}

// SetEtcdVersion 设置Pod的etcd版本
func SetEtcdVersion(pod *corev1.Pod, version string) {
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations[etcdVersionAnnotationKey] = version
}

// PodSpecToPrettyJSON 将Pod规格转换为格式化的JSON字符串
func PodSpecToPrettyJSON(pod *corev1.Pod) (string, error) {
	bytes, err := json.MarshalIndent(pod.Spec, "", "    ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// UniqueMemberName 生成唯一的成员名称
func UniqueMemberName(clusterName string) string {
	suffix := rand.String(randomSuffixLength)
	if len(clusterName) > maxNameLength {
		clusterName = clusterName[:maxNameLength]
	}
	return clusterName + "-" + suffix
}

// etcdVolumeMounts 返回etcd卷挂载
func etcdVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{Name: etcdVolumeName, MountPath: etcdVolumeMountDir},
	}
}

// etcdContainer 创建etcd容器
func etcdContainer(cmd []string, repo, version string) corev1.Container {
	c := corev1.Container{
		Command: cmd,
		Name:    "etcd",
		Image:   ImageName(repo, version),
		Ports: []corev1.ContainerPort{
			{
				Name:          "server",
				ContainerPort: int32(EtcdPeerPort),
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          "client",
				ContainerPort: int32(EtcdClientPort),
				Protocol:      corev1.ProtocolTCP,
			},
		},
		VolumeMounts: etcdVolumeMounts(),
	}
	return c
}

// ImageName 构建镜像名称
func ImageName(repo, version string) string {
	// 确保版本号前有v前缀（但不要重复添加）
	if version != "" && !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return fmt.Sprintf("%s:%s", repo, version)
}

// newEtcdProbe 创建etcd探针
func newEtcdProbe(isSecure bool) *corev1.Probe {
	// etcd pod只有在线性化get成功时才算存活
	cmd := []string{"etcdctl", "get", "foo"}
	if isSecure {
		cmd = []string{"etcdctl", "--endpoints=https://localhost:" + fmt.Sprintf("%d", EtcdClientPort), "--insecure-skip-tls-verify", "get", "foo"}
	}
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: cmd,
			},
		},
		InitialDelaySeconds: 10,
		TimeoutSeconds:      10,
		PeriodSeconds:       60,
		FailureThreshold:    3,
	}
}

// containerWithProbes 为容器添加探针
func containerWithProbes(c corev1.Container, lp *corev1.Probe, rp *corev1.Probe) corev1.Container {
	c.LivenessProbe = lp
	c.ReadinessProbe = rp
	return c
}

// containerWithRequirements 为容器添加资源需求
func containerWithRequirements(c corev1.Container, r corev1.ResourceRequirements) corev1.Container {
	c.Resources = r
	return c
}

// AddEtcdVolumeToPod 向Pod添加etcd卷
func AddEtcdVolumeToPod(pod *corev1.Pod, pvc *corev1.PersistentVolumeClaim) {
	vol := corev1.Volume{Name: etcdVolumeName}
	if pvc != nil {
		vol.VolumeSource = corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name},
		}
	} else {
		vol.VolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, vol)
}

// addOwnerRefToObject 向对象添加所有者引用
func addOwnerRefToObject(o metav1.Object, r metav1.OwnerReference) {
	o.SetOwnerReferences(append(o.GetOwnerReferences(), r))
}

// CreateAndWaitPod 创建Pod并等待其运行
func CreateAndWaitPod(ctx context.Context, kubecli kubernetes.Interface, ns string, pod *corev1.Pod, timeout time.Duration) (*corev1.Pod, error) {
	_, err := kubecli.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	// 等待Pod运行
	interval := 5 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		retPod, err := kubecli.CoreV1().Pods(ns).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		switch retPod.Status.Phase {
		case corev1.PodRunning:
			return retPod, nil
		case corev1.PodPending:
			time.Sleep(interval)
			continue
		default:
			return nil, fmt.Errorf("unexpected pod status.phase: %v", retPod.Status.Phase)
		}
	}

	return nil, fmt.Errorf("timeout waiting for pod to be running")
}

// NewEtcdPod 创建新的etcd Pod
func NewEtcdPod(m *etcd.Member, initialCluster []string, clusterName, state, token string, cluster *etcdv1alpha1.EtcdCluster, owner metav1.OwnerReference) *corev1.Pod {
	pod := newEtcdPod(m, initialCluster, clusterName, state, token, cluster)
	applyPodPolicy(clusterName, pod, cluster.Spec.Pod)
	addOwnerRefToObject(pod, owner)
	return pod
}

// newEtcdPod 创建etcd Pod的内部函数
func newEtcdPod(m *etcd.Member, initialCluster []string, clusterName, state, token string, cluster *etcdv1alpha1.EtcdCluster) *corev1.Pod {
	// 构建etcd启动命令
	commands := fmt.Sprintf("/usr/local/bin/etcd --data-dir=%s --name=%s --initial-advertise-peer-urls=%s "+
		"--listen-peer-urls=%s --listen-client-urls=%s --advertise-client-urls=%s "+
		"--initial-cluster=%s --initial-cluster-state=%s",
		dataDir, m.Name, m.PeerURL(), m.ListenPeerURL(), m.ListenClientURL(), m.ClientURL(), strings.Join(initialCluster, ","), state)

	// 如果是新集群，添加token
	if state == "new" {
		commands = fmt.Sprintf("%s --initial-cluster-token=%s", commands, token)
	}

	// 设置Pod标签
	labels := map[string]string{
		"app":          "etcd",
		"etcd_node":    m.Name,
		"etcd_cluster": clusterName,
	}

	// 创建探针
	livenessProbe := newEtcdProbe(m.SecureClient)
	readinessProbe := newEtcdProbe(m.SecureClient)
	readinessProbe.InitialDelaySeconds = 1
	readinessProbe.TimeoutSeconds = 5
	readinessProbe.PeriodSeconds = 5
	readinessProbe.FailureThreshold = 3

	// 创建容器
	container := containerWithProbes(
		etcdContainer(strings.Split(commands, " "), getEtcdRepository(cluster), getEtcdVersion(cluster)),
		livenessProbe,
		readinessProbe)

	// 安全上下文设置
	runAsNonRoot := true
	podUID := int64(9000)
	fsGroup := podUID

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        m.Name,
			Labels:      labels,
			Annotations: map[string]string{},
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{{
				// 使用稳定版本的busybox
				Image: "busybox:1.28.0-glibc",
				Name:  "check-dns",
				// 在etcd 3.2中，TLS监听器会对pod IP进行反向DNS查找
				// 如果DNS条目没有预热，它将返回空结果，peer连接将被拒绝
				Command: []string{"/bin/sh", "-c", fmt.Sprintf(`
					while ( ! nslookup %s )
					do
						sleep 2
					done`, m.Addr())},
			}},
			Containers:    []corev1.Container{container},
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes:       []corev1.Volume{},
			// DNS A记录: `[m.Name].[clusterName].Namespace.svc`
			// 例如，default命名空间中的etcd-795649v9kq将有DNS名称
			// `etcd-795649v9kq.etcd.default.svc`
			Hostname:                     m.Name,
			Subdomain:                    clusterName,
			AutomountServiceAccountToken: func(b bool) *bool { return &b }(false),
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:    &podUID,
				RunAsNonRoot: &runAsNonRoot,
				FSGroup:      &fsGroup,
			},
		},
	}

	SetEtcdVersion(pod, getEtcdVersion(cluster))
	return pod
}

// getEtcdRepository 获取etcd镜像仓库
func getEtcdRepository(cluster *etcdv1alpha1.EtcdCluster) string {
	if cluster.Spec.Repository != "" {
		return cluster.Spec.Repository
	}
	return defaultEtcdRepository
}

// getEtcdVersion 获取etcd版本
func getEtcdVersion(cluster *etcdv1alpha1.EtcdCluster) string {
	if cluster.Spec.Version != "" {
		return cluster.Spec.Version
	}
	return defaultEtcdVersion
}

// applyPodPolicy 应用Pod策略
func applyPodPolicy(clusterName string, pod *corev1.Pod, policy *etcdv1alpha1.PodPolicy) {
	if policy == nil {
		return
	}

	// 应用亲和性
	if policy.Affinity != nil {
		pod.Spec.Affinity = policy.Affinity
	}

	// 应用节点选择器
	if len(policy.NodeSelector) != 0 {
		pod.Spec.NodeSelector = policy.NodeSelector
	}

	// 应用容忍度
	if len(policy.Tolerations) != 0 {
		pod.Spec.Tolerations = policy.Tolerations
	}

	// 合并标签
	mergeLabels(pod.Labels, policy.Labels)

	// 应用资源需求
	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i] = containerWithRequirements(pod.Spec.Containers[i], policy.Resources)
		if pod.Spec.Containers[i].Name == "etcd" {
			pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, policy.EtcdEnv...)
		}
	}

	// 应用注解
	if pod.ObjectMeta.Annotations == nil {
		pod.ObjectMeta.Annotations = make(map[string]string)
	}
	for key, value := range policy.Annotations {
		pod.ObjectMeta.Annotations[key] = value
	}
}

// mergeLabels 合并标签，冲突的标签将被跳过
func mergeLabels(l1, l2 map[string]string) {
	for k, v := range l2 {
		if _, ok := l1[k]; ok {
			continue
		}
		l1[k] = v
	}
}
