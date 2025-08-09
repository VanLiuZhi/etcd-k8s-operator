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

package integration

import (
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
)

// TestConfig 集成测试配置
type TestConfig struct {
	// 测试超时配置
	DefaultTimeout      time.Duration
	ReconcileTimeout    time.Duration
	ClusterReadyTimeout time.Duration

	// 默认集群配置
	DefaultEtcdVersion    string
	DefaultEtcdRepository string
	DefaultStorageSize    resource.Quantity

	// 测试环境配置
	TestNamespace    string
	CleanupAfterTest bool
}

// DefaultTestConfig 返回默认的测试配置
func DefaultTestConfig() *TestConfig {
	return &TestConfig{
		// 超时配置
		DefaultTimeout:      30 * time.Second,
		ReconcileTimeout:    10 * time.Second,
		ClusterReadyTimeout: 60 * time.Second,

		// 集群配置
		DefaultEtcdVersion:    "v3.5.21",
		DefaultEtcdRepository: "quay.io/coreos/etcd",
		DefaultStorageSize:    resource.MustParse("1Gi"),

		// 环境配置
		TestNamespace:    "integration-test",
		CleanupAfterTest: true,
	}
}

// TestClusterSpec 测试集群规范模板
type TestClusterSpec struct {
	Name       string
	Size       int32
	Version    string
	Repository string
	Storage    resource.Quantity
}

// DefaultSingleNodeCluster 默认单节点集群配置
func DefaultSingleNodeCluster(name string) *TestClusterSpec {
	config := DefaultTestConfig()
	return &TestClusterSpec{
		Name:       name,
		Size:       1,
		Version:    config.DefaultEtcdVersion,
		Repository: config.DefaultEtcdRepository,
		Storage:    config.DefaultStorageSize,
	}
}

// DefaultMultiNodeCluster 默认多节点集群配置
func DefaultMultiNodeCluster(name string, size int32) *TestClusterSpec {
	config := DefaultTestConfig()
	return &TestClusterSpec{
		Name:       name,
		Size:       size,
		Version:    config.DefaultEtcdVersion,
		Repository: config.DefaultEtcdRepository,
		Storage:    config.DefaultStorageSize,
	}
}

// ToEtcdCluster 转换为EtcdCluster对象
func (spec *TestClusterSpec) ToEtcdCluster(namespace string) *etcdv1alpha1.EtcdCluster {
	return &etcdv1alpha1.EtcdCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: namespace,
		},
		Spec: etcdv1alpha1.EtcdClusterSpec{
			Size:       spec.Size,
			Version:    spec.Version,
			Repository: spec.Repository,
			Storage: etcdv1alpha1.EtcdStorageSpec{
				Size: spec.Storage,
			},
		},
	}
}

// TestScenario 测试场景定义
type TestScenario struct {
	Name        string
	Description string
	Cluster     *TestClusterSpec
	Steps       []TestStep
}

// TestStep 测试步骤
type TestStep struct {
	Name        string
	Description string
	Action      TestAction
	Validation  TestValidation
	Timeout     time.Duration
}

// TestAction 测试动作类型
type TestAction string

const (
	ActionCreate    TestAction = "create"
	ActionUpdate    TestAction = "update"
	ActionDelete    TestAction = "delete"
	ActionScale     TestAction = "scale"
	ActionReconcile TestAction = "reconcile"
	ActionWait      TestAction = "wait"
)

// TestValidation 测试验证类型
type TestValidation string

const (
	ValidatePhase     TestValidation = "phase"
	ValidateResources TestValidation = "resources"
	ValidateReplicas  TestValidation = "replicas"
	ValidateDeleted   TestValidation = "deleted"
)

// PredefinedScenarios 预定义的测试场景
func PredefinedScenarios() []*TestScenario {
	return []*TestScenario{
		{
			Name:        "SingleNodeLifecycle",
			Description: "单节点集群完整生命周期测试",
			Cluster:     DefaultSingleNodeCluster("test-single"),
			Steps: []TestStep{
				{
					Name:        "CreateCluster",
					Description: "创建集群资源",
					Action:      ActionCreate,
					Validation:  ValidatePhase,
					Timeout:     30 * time.Second,
				},
				{
					Name:        "WaitForRunning",
					Description: "等待集群运行",
					Action:      ActionWait,
					Validation:  ValidatePhase,
					Timeout:     60 * time.Second,
				},
				{
					Name:        "DeleteCluster",
					Description: "删除集群",
					Action:      ActionDelete,
					Validation:  ValidateDeleted,
					Timeout:     30 * time.Second,
				},
			},
		},
		{
			Name:        "MultiNodeCreation",
			Description: "多节点集群创建测试",
			Cluster:     DefaultMultiNodeCluster("test-multi", 3),
			Steps: []TestStep{
				{
					Name:        "CreateCluster",
					Description: "创建3节点集群",
					Action:      ActionCreate,
					Validation:  ValidateResources,
					Timeout:     30 * time.Second,
				},
				{
					Name:        "ValidateResources",
					Description: "验证Kubernetes资源创建",
					Action:      ActionWait,
					Validation:  ValidateResources,
					Timeout:     30 * time.Second,
				},
			},
		},
		{
			Name:        "ClusterScaling",
			Description: "集群扩缩容测试",
			Cluster:     DefaultSingleNodeCluster("test-scaling"),
			Steps: []TestStep{
				{
					Name:        "CreateSingleNode",
					Description: "创建单节点集群",
					Action:      ActionCreate,
					Validation:  ValidatePhase,
					Timeout:     30 * time.Second,
				},
				{
					Name:        "ScaleToThree",
					Description: "扩容到3节点",
					Action:      ActionScale,
					Validation:  ValidateReplicas,
					Timeout:     30 * time.Second,
				},
				{
					Name:        "ScaleToOne",
					Description: "缩容到1节点",
					Action:      ActionScale,
					Validation:  ValidateReplicas,
					Timeout:     30 * time.Second,
				},
			},
		},
	}
}
