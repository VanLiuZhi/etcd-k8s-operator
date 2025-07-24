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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
)

// LabelsTestSuite 标签工具测试套件
type LabelsTestSuite struct {
	suite.Suite
	cluster *etcdv1alpha1.EtcdCluster
}

// SetupTest 设置测试环境
func (suite *LabelsTestSuite) SetupTest() {
	suite.cluster = &etcdv1alpha1.EtcdCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
			Annotations: map[string]string{
				"test-annotation": "test-value",
			},
		},
		Spec: etcdv1alpha1.EtcdClusterSpec{
			Version: "3.5.9",
		},
	}
}

// TestLabelsForEtcdCluster 测试集群标签生成
func (suite *LabelsTestSuite) TestLabelsForEtcdCluster() {
	labels := LabelsForEtcdCluster(suite.cluster)

	// 验证必需的标签
	assert.Equal(suite.T(), "etcd", labels[LabelAppName])
	assert.Equal(suite.T(), "test-cluster", labels[LabelAppInstance])
	assert.Equal(suite.T(), "database", labels[LabelAppComponent])
	assert.Equal(suite.T(), "etcd-operator", labels[LabelAppManagedBy])
	assert.Equal(suite.T(), "3.5.9", labels[LabelAppVersion])
	assert.Equal(suite.T(), "test-cluster", labels[LabelEtcdCluster])

	// 验证标签数量
	assert.Len(suite.T(), labels, 6)
}

// TestLabelsForEtcdMember 测试成员标签生成
func (suite *LabelsTestSuite) TestLabelsForEtcdMember() {
	memberName := "test-cluster-0"
	labels := LabelsForEtcdMember(suite.cluster, memberName)

	// 验证包含集群标签
	clusterLabels := LabelsForEtcdCluster(suite.cluster)
	for k, v := range clusterLabels {
		assert.Equal(suite.T(), v, labels[k])
	}

	// 验证成员特定标签
	assert.Equal(suite.T(), memberName, labels[LabelEtcdMember])

	// 验证标签数量（集群标签 + 成员标签）
	assert.Len(suite.T(), labels, 7)
}

// TestLabelsForEtcdService 测试服务标签生成
func (suite *LabelsTestSuite) TestLabelsForEtcdService() {
	// 测试客户端服务标签
	clientLabels := LabelsForEtcdService(suite.cluster, "client")
	assert.Equal(suite.T(), "client", clientLabels[LabelAppComponent])

	// 测试对等服务标签
	peerLabels := LabelsForEtcdService(suite.cluster, "peer")
	assert.Equal(suite.T(), "peer", peerLabels[LabelAppComponent])

	// 验证其他标签保持不变
	assert.Equal(suite.T(), "etcd", clientLabels[LabelAppName])
	assert.Equal(suite.T(), "test-cluster", clientLabels[LabelAppInstance])
}

// TestSelectorLabelsForEtcdCluster 测试选择器标签生成
func (suite *LabelsTestSuite) TestSelectorLabelsForEtcdCluster() {
	selectorLabels := SelectorLabelsForEtcdCluster(suite.cluster)

	// 验证选择器标签
	assert.Equal(suite.T(), "etcd", selectorLabels[LabelAppName])
	assert.Equal(suite.T(), "test-cluster", selectorLabels[LabelAppInstance])
	assert.Equal(suite.T(), "test-cluster", selectorLabels[LabelEtcdCluster])

	// 验证选择器标签数量（应该只包含核心标签）
	assert.Len(suite.T(), selectorLabels, 3)
}

// TestMergeLabels 测试标签合并
func (suite *LabelsTestSuite) TestMergeLabels() {
	labels1 := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	labels2 := map[string]string{
		"key2": "updated-value2", // 覆盖 labels1 中的值
		"key3": "value3",
	}

	labels3 := map[string]string{
		"key4": "value4",
	}

	merged := MergeLabels(labels1, labels2, labels3)

	// 验证合并结果
	assert.Equal(suite.T(), "value1", merged["key1"])
	assert.Equal(suite.T(), "updated-value2", merged["key2"]) // 应该被覆盖
	assert.Equal(suite.T(), "value3", merged["key3"])
	assert.Equal(suite.T(), "value4", merged["key4"])

	// 验证合并后的标签数量
	assert.Len(suite.T(), merged, 4)
}

// TestMergeLabelsWithEmptyMaps 测试空标签映射合并
func (suite *LabelsTestSuite) TestMergeLabelsWithEmptyMaps() {
	labels1 := map[string]string{
		"key1": "value1",
	}

	emptyLabels := map[string]string{}

	merged := MergeLabels(labels1, emptyLabels)

	// 验证空映射不影响结果
	assert.Equal(suite.T(), "value1", merged["key1"])
	assert.Len(suite.T(), merged, 1)
}

// TestAnnotationsForEtcdCluster 测试集群注解生成
func (suite *LabelsTestSuite) TestAnnotationsForEtcdCluster() {
	annotations := AnnotationsForEtcdCluster(suite.cluster)

	// 验证包含原有注解
	assert.Equal(suite.T(), "test-value", annotations["test-annotation"])

	// 验证注解数量
	assert.Len(suite.T(), annotations, 1)
}

// TestAnnotationsForEtcdClusterWithNilAnnotations 测试空注解的集群
func (suite *LabelsTestSuite) TestAnnotationsForEtcdClusterWithNilAnnotations() {
	suite.cluster.Annotations = nil

	annotations := AnnotationsForEtcdCluster(suite.cluster)

	// 验证返回空映射而不是 nil
	assert.NotNil(suite.T(), annotations)
	assert.Len(suite.T(), annotations, 0)
}

// TestMergeAnnotations 测试注解合并
func (suite *LabelsTestSuite) TestMergeAnnotations() {
	annotations1 := map[string]string{
		"annotation1": "value1",
		"annotation2": "value2",
	}

	annotations2 := map[string]string{
		"annotation2": "updated-value2", // 覆盖 annotations1 中的值
		"annotation3": "value3",
	}

	merged := MergeAnnotations(annotations1, annotations2)

	// 验证合并结果
	assert.Equal(suite.T(), "value1", merged["annotation1"])
	assert.Equal(suite.T(), "updated-value2", merged["annotation2"]) // 应该被覆盖
	assert.Equal(suite.T(), "value3", merged["annotation3"])

	// 验证合并后的注解数量
	assert.Len(suite.T(), merged, 3)
}

// TestLabelConstants 测试标签常量
func (suite *LabelsTestSuite) TestLabelConstants() {
	// 验证标签键常量
	assert.Equal(suite.T(), "app.kubernetes.io/name", LabelAppName)
	assert.Equal(suite.T(), "app.kubernetes.io/instance", LabelAppInstance)
	assert.Equal(suite.T(), "app.kubernetes.io/component", LabelAppComponent)
	assert.Equal(suite.T(), "app.kubernetes.io/managed-by", LabelAppManagedBy)
	assert.Equal(suite.T(), "app.kubernetes.io/version", LabelAppVersion)
	assert.Equal(suite.T(), "etcd.etcd.io/cluster", LabelEtcdCluster)
	assert.Equal(suite.T(), "etcd.etcd.io/member", LabelEtcdMember)
}

// TestAnnotationConstants 测试注解常量
func (suite *LabelsTestSuite) TestAnnotationConstants() {
	// 验证注解键常量
	assert.Equal(suite.T(), "etcd.etcd.io/last-applied-config", AnnotationLastAppliedConfig)
	assert.Equal(suite.T(), "etcd.etcd.io/last-backup-time", AnnotationLastBackupTime)
	assert.Equal(suite.T(), "etcd.etcd.io/cluster-id", AnnotationClusterID)
}

// TestLabelsConsistency 测试标签一致性
func (suite *LabelsTestSuite) TestLabelsConsistency() {
	clusterLabels := LabelsForEtcdCluster(suite.cluster)
	selectorLabels := SelectorLabelsForEtcdCluster(suite.cluster)

	// 验证选择器标签是集群标签的子集
	for k, v := range selectorLabels {
		assert.Equal(suite.T(), v, clusterLabels[k], "Selector label %s should match cluster label", k)
	}
}

// TestLabelsForDifferentClusters 测试不同集群的标签生成
func (suite *LabelsTestSuite) TestLabelsForDifferentClusters() {
	cluster2 := &etcdv1alpha1.EtcdCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "another-cluster",
			Namespace: "kube-system",
		},
		Spec: etcdv1alpha1.EtcdClusterSpec{
			Version: "3.5.10",
		},
	}

	labels1 := LabelsForEtcdCluster(suite.cluster)
	labels2 := LabelsForEtcdCluster(cluster2)

	// 验证不同集群的标签不同
	assert.NotEqual(suite.T(), labels1[LabelAppInstance], labels2[LabelAppInstance])
	assert.NotEqual(suite.T(), labels1[LabelAppVersion], labels2[LabelAppVersion])
	assert.NotEqual(suite.T(), labels1[LabelEtcdCluster], labels2[LabelEtcdCluster])

	// 验证相同的标签值
	assert.Equal(suite.T(), labels1[LabelAppName], labels2[LabelAppName])
	assert.Equal(suite.T(), labels1[LabelAppComponent], labels2[LabelAppComponent])
	assert.Equal(suite.T(), labels1[LabelAppManagedBy], labels2[LabelAppManagedBy])
}

// TestLabelsTestSuite 运行测试套件
func TestLabelsTestSuite(t *testing.T) {
	suite.Run(t, new(LabelsTestSuite))
}
