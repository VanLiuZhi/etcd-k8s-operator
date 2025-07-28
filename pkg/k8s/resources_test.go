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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	"github.com/your-org/etcd-k8s-operator/pkg/utils"
)

// ResourcesTestSuite 资源构建器测试套件
type ResourcesTestSuite struct {
	suite.Suite
	cluster *etcdv1alpha1.EtcdCluster
}

// SetupTest 设置测试环境
func (suite *ResourcesTestSuite) SetupTest() {
	suite.cluster = &etcdv1alpha1.EtcdCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: etcdv1alpha1.EtcdClusterSpec{
			Size:       3,
			Version:    "3.5.9",
			Repository: "quay.io/coreos/etcd",
			Storage: etcdv1alpha1.EtcdStorageSpec{
				Size: resource.MustParse("10Gi"),
			},
			Security: etcdv1alpha1.EtcdSecuritySpec{
				TLS: etcdv1alpha1.EtcdTLSSpec{
					Enabled: true,
					AutoTLS: true,
				},
			},
		},
	}
}

// TestStatefulSetBuilder 测试 StatefulSet 构建器
func (suite *ResourcesTestSuite) TestStatefulSetBuilder() {
	sts := BuildStatefulSet(suite.cluster)

	// 验证基本属性
	assert.Equal(suite.T(), suite.cluster.Name, sts.Name)
	assert.Equal(suite.T(), suite.cluster.Namespace, sts.Namespace)
	assert.Equal(suite.T(), suite.cluster.Spec.Size, *sts.Spec.Replicas)

	// 验证标签
	expectedLabels := utils.LabelsForEtcdCluster(suite.cluster)
	assert.Equal(suite.T(), expectedLabels, sts.Labels)

	// 验证选择器
	expectedSelector := utils.SelectorLabelsForEtcdCluster(suite.cluster)
	assert.Equal(suite.T(), expectedSelector, sts.Spec.Selector.MatchLabels)

	// 验证服务名称
	assert.Equal(suite.T(), "test-cluster-peer", sts.Spec.ServiceName)

	// 验证容器配置
	assert.Len(suite.T(), sts.Spec.Template.Spec.Containers, 1)
	container := sts.Spec.Template.Spec.Containers[0]
	assert.Equal(suite.T(), "etcd", container.Name)
	assert.Equal(suite.T(), "quay.io/coreos/etcd:3.5.9", container.Image)

	// 验证端口配置
	assert.Len(suite.T(), container.Ports, 2)
	clientPort := container.Ports[0]
	peerPort := container.Ports[1]
	assert.Equal(suite.T(), "client", clientPort.Name)
	assert.Equal(suite.T(), int32(utils.EtcdClientPort), clientPort.ContainerPort)
	assert.Equal(suite.T(), "peer", peerPort.Name)
	assert.Equal(suite.T(), int32(utils.EtcdPeerPort), peerPort.ContainerPort)

	// 验证环境变量
	envVars := container.Env
	assert.NotEmpty(suite.T(), envVars)

	// 检查关键环境变量
	envMap := make(map[string]string)
	for _, env := range envVars {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}
	assert.Equal(suite.T(), utils.EtcdDataDir, envMap["ETCD_DATA_DIR"])
	assert.Equal(suite.T(), "new", envMap["ETCD_INITIAL_CLUSTER_STATE"])
	assert.Equal(suite.T(), suite.cluster.Name, envMap["ETCD_INITIAL_CLUSTER_TOKEN"])

	// 验证健康检查配置
	assert.NotNil(suite.T(), container.LivenessProbe)
	assert.NotNil(suite.T(), container.ReadinessProbe)

	// 对于非 Bitnami 镜像，应该使用 HTTP 健康检查
	assert.NotNil(suite.T(), container.LivenessProbe.HTTPGet)
	assert.Equal(suite.T(), "/health", container.LivenessProbe.HTTPGet.Path)
	assert.NotNil(suite.T(), container.ReadinessProbe.HTTPGet)
	assert.Equal(suite.T(), "/health", container.ReadinessProbe.HTTPGet.Path)

	// 验证存储卷声明模板
	assert.Len(suite.T(), sts.Spec.VolumeClaimTemplates, 1)
	pvc := sts.Spec.VolumeClaimTemplates[0]
	assert.Equal(suite.T(), "data", pvc.Name)
	assert.Equal(suite.T(), resource.MustParse("10Gi"), pvc.Spec.Resources.Requests[corev1.ResourceStorage])
}

// TestBitnamiStatefulSetBuilder 测试 Bitnami 镜像的 StatefulSet 构建器
func (suite *ResourcesTestSuite) TestBitnamiStatefulSetBuilder() {
	// 创建使用 Bitnami 镜像的集群
	bitnamiCluster := &etcdv1alpha1.EtcdCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-bitnami-cluster",
			Namespace: "default",
		},
		Spec: etcdv1alpha1.EtcdClusterSpec{
			Size:       3,
			Version:    "3.5.9",
			Repository: "bitnami/etcd",
			Storage: etcdv1alpha1.EtcdStorageSpec{
				Size: resource.MustParse("10Gi"),
			},
		},
	}

	sts := BuildStatefulSet(bitnamiCluster)
	container := sts.Spec.Template.Spec.Containers[0]

	// 验证 Bitnami 镜像使用 exec 健康检查
	assert.NotNil(suite.T(), container.LivenessProbe)
	assert.NotNil(suite.T(), container.ReadinessProbe)

	// 应该使用 exec 而不是 HTTP
	assert.NotNil(suite.T(), container.LivenessProbe.Exec)
	assert.Nil(suite.T(), container.LivenessProbe.HTTPGet)
	assert.Equal(suite.T(), []string{"/opt/bitnami/scripts/etcd/healthcheck.sh"}, container.LivenessProbe.Exec.Command)

	assert.NotNil(suite.T(), container.ReadinessProbe.Exec)
	assert.Nil(suite.T(), container.ReadinessProbe.HTTPGet)
	assert.Equal(suite.T(), []string{"/opt/bitnami/scripts/etcd/healthcheck.sh"}, container.ReadinessProbe.Exec.Command)

	// 验证 Bitnami 特定的环境变量
	envMap := make(map[string]string)
	for _, env := range container.Env {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}
	assert.Equal(suite.T(), "yes", envMap["ALLOW_NONE_AUTHENTICATION"])
	assert.Equal(suite.T(), "yes", envMap["ETCD_ON_K8S"])
	assert.Equal(suite.T(), "test-bitnami-cluster-peer.default.svc.cluster.local", envMap["ETCD_CLUSTER_DOMAIN"])
	assert.Equal(suite.T(), "test-bitnami-cluster", envMap["MY_STS_NAME"])
}

// TestClientServiceBuilder 测试客户端服务构建器
func (suite *ResourcesTestSuite) TestClientServiceBuilder() {
	svc := BuildClientService(suite.cluster)

	// 验证基本属性
	assert.Equal(suite.T(), "test-cluster-client", svc.Name)
	assert.Equal(suite.T(), suite.cluster.Namespace, svc.Namespace)
	assert.Equal(suite.T(), corev1.ServiceTypeClusterIP, svc.Spec.Type)

	// 验证标签
	expectedLabels := utils.LabelsForEtcdService(suite.cluster, "client")
	assert.Equal(suite.T(), expectedLabels, svc.Labels)

	// 验证选择器
	expectedSelector := utils.SelectorLabelsForEtcdCluster(suite.cluster)
	assert.Equal(suite.T(), expectedSelector, svc.Spec.Selector)

	// 验证端口配置
	assert.Len(suite.T(), svc.Spec.Ports, 1)
	port := svc.Spec.Ports[0]
	assert.Equal(suite.T(), "client", port.Name)
	assert.Equal(suite.T(), int32(utils.EtcdClientPort), port.Port)
	assert.Equal(suite.T(), int32(utils.EtcdClientPort), port.TargetPort.IntVal)
}

// TestPeerServiceBuilder 测试对等服务构建器
func (suite *ResourcesTestSuite) TestPeerServiceBuilder() {
	svc := BuildPeerService(suite.cluster)

	// 验证基本属性
	assert.Equal(suite.T(), "test-cluster-peer", svc.Name)
	assert.Equal(suite.T(), suite.cluster.Namespace, svc.Namespace)
	assert.Equal(suite.T(), corev1.ServiceTypeClusterIP, svc.Spec.Type)
	assert.Equal(suite.T(), corev1.ClusterIPNone, svc.Spec.ClusterIP) // Headless service

	// 验证标签
	expectedLabels := utils.LabelsForEtcdService(suite.cluster, "peer")
	assert.Equal(suite.T(), expectedLabels, svc.Labels)

	// 验证端口配置
	assert.Len(suite.T(), svc.Spec.Ports, 1)
	port := svc.Spec.Ports[0]
	assert.Equal(suite.T(), "peer", port.Name)
	assert.Equal(suite.T(), int32(utils.EtcdPeerPort), port.Port)
	assert.Equal(suite.T(), int32(utils.EtcdPeerPort), port.TargetPort.IntVal)
}

// TestConfigMapBuilder 测试 ConfigMap 构建器
func (suite *ResourcesTestSuite) TestConfigMapBuilder() {
	cm := BuildConfigMap(suite.cluster)

	// 验证基本属性
	assert.Equal(suite.T(), "test-cluster-config", cm.Name)
	assert.Equal(suite.T(), suite.cluster.Namespace, cm.Namespace)

	// 验证标签
	expectedLabels := utils.LabelsForEtcdCluster(suite.cluster)
	assert.Equal(suite.T(), expectedLabels, cm.Labels)

	// 验证配置数据
	assert.Contains(suite.T(), cm.Data, "etcd.conf")
	etcdConf := cm.Data["etcd.conf"]
	assert.Contains(suite.T(), etcdConf, "name: $(ETCD_NAME)")
	assert.Contains(suite.T(), etcdConf, utils.EtcdDataDir)
	assert.Contains(suite.T(), etcdConf, suite.cluster.Name)
}

// TestResourceRequirements 测试资源需求构建
func (suite *ResourcesTestSuite) TestResourceRequirements() {
	// 测试默认资源需求
	requirements := buildResourceRequirements(suite.cluster)

	assert.NotNil(suite.T(), requirements.Requests)
	assert.NotNil(suite.T(), requirements.Limits)

	// 验证默认值
	assert.Equal(suite.T(), resource.MustParse("100m"), requirements.Requests[corev1.ResourceCPU])
	assert.Equal(suite.T(), resource.MustParse("128Mi"), requirements.Requests[corev1.ResourceMemory])
	assert.Equal(suite.T(), resource.MustParse("1000m"), requirements.Limits[corev1.ResourceCPU])
	assert.Equal(suite.T(), resource.MustParse("1Gi"), requirements.Limits[corev1.ResourceMemory])
}

// TestResourceRequirementsWithCustomValues 测试自定义资源需求
func (suite *ResourcesTestSuite) TestResourceRequirementsWithCustomValues() {
	// 设置自定义资源需求
	suite.cluster.Spec.Resources = etcdv1alpha1.EtcdResourceSpec{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("2000m"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}

	requirements := buildResourceRequirements(suite.cluster)

	// 验证自定义值
	assert.Equal(suite.T(), resource.MustParse("200m"), requirements.Requests[corev1.ResourceCPU])
	assert.Equal(suite.T(), resource.MustParse("256Mi"), requirements.Requests[corev1.ResourceMemory])
	assert.Equal(suite.T(), resource.MustParse("2000m"), requirements.Limits[corev1.ResourceCPU])
	assert.Equal(suite.T(), resource.MustParse("2Gi"), requirements.Limits[corev1.ResourceMemory])
}

// TestInitialClusterGeneration 测试初始集群配置生成
func (suite *ResourcesTestSuite) TestInitialClusterGeneration() {
	initialCluster := buildInitialCluster(suite.cluster)

	// 验证初始集群配置格式
	assert.Contains(suite.T(), initialCluster, "test-cluster-0=")
	assert.Contains(suite.T(), initialCluster, "test-cluster-1=")
	assert.Contains(suite.T(), initialCluster, "test-cluster-2=")
	assert.Contains(suite.T(), initialCluster, "test-cluster-peer.default.svc.cluster.local:2380")
}

// TestVolumeClaimTemplates 测试存储卷声明模板
func (suite *ResourcesTestSuite) TestVolumeClaimTemplates() {
	// 测试默认存储配置
	templates := buildVolumeClaimTemplates(suite.cluster)

	assert.Len(suite.T(), templates, 1)
	pvc := templates[0]
	assert.Equal(suite.T(), "data", pvc.Name)
	assert.Equal(suite.T(), resource.MustParse("10Gi"), pvc.Spec.Resources.Requests[corev1.ResourceStorage])
	assert.Nil(suite.T(), pvc.Spec.StorageClassName) // 默认为 nil
}

// TestVolumeClaimTemplatesWithStorageClass 测试带存储类的存储卷声明模板
func (suite *ResourcesTestSuite) TestVolumeClaimTemplatesWithStorageClass() {
	storageClass := "fast-ssd"
	suite.cluster.Spec.Storage.StorageClassName = &storageClass

	templates := buildVolumeClaimTemplates(suite.cluster)

	assert.Len(suite.T(), templates, 1)
	pvc := templates[0]
	assert.NotNil(suite.T(), pvc.Spec.StorageClassName)
	assert.Equal(suite.T(), storageClass, *pvc.Spec.StorageClassName)
}

// TestResourcesTestSuite 运行测试套件
func TestResourcesTestSuite(t *testing.T) {
	suite.Run(t, new(ResourcesTestSuite))
}
