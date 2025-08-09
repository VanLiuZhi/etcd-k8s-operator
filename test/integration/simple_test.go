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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	etcdv1alpha1 "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
	controller "github.com/your-org/etcd-k8s-operator/internal/controller"
)

// TestSimpleClusterReconcile 简化的集成测试
// 专注于调试控制器的基本Reconcile流程
func TestSimpleClusterReconcile(t *testing.T) {
	// 1. 设置fake K8s环境
	testScheme := runtime.NewScheme()
	err := scheme.AddToScheme(testScheme)
	require.NoError(t, err, "添加K8s scheme失败")

	err = etcdv1alpha1.AddToScheme(testScheme)
	require.NoError(t, err, "添加etcd scheme失败")

	// 2. 创建测试集群对象
	cluster := &etcdv1alpha1.EtcdCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-simple",
			Namespace: "default",
		},
		Spec: etcdv1alpha1.EtcdClusterSpec{
			Size:    1,
			Version: "v3.5.21",
			Storage: etcdv1alpha1.EtcdStorageSpec{
				Size: resource.MustParse("1Gi"),
			},
		},
	}

	// 3. 创建fake客户端并预置集群对象
	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithObjects(cluster).
		WithStatusSubresource(cluster). // 重要：启用状态子资源
		Build()

	// 4. 创建控制器
	clusterController := controller.NewClusterController(
		fakeClient,
		testScheme,
		nil, // EventRecorder可以为nil
	)

	// 5. 准备Reconcile请求
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		},
	}

	ctx := context.Background()

	// 6. 第一次Reconcile - 应该添加Finalizer
	t.Log("🔄 执行第一次Reconcile (添加Finalizer)")
	result, err := clusterController.Reconcile(ctx, req)
	require.NoError(t, err, "第一次Reconcile失败")
	assert.True(t, result.Requeue, "应该重新入队")

	// 验证Finalizer已添加
	err = fakeClient.Get(ctx, req.NamespacedName, cluster)
	require.NoError(t, err)
	assert.Contains(t, cluster.Finalizers, "etcd.etcd.io/finalizer", "应该添加Finalizer")

	// 7. 第二次Reconcile - 应该初始化集群
	t.Log("🔄 执行第二次Reconcile (初始化集群)")
	result, err = clusterController.Reconcile(ctx, req)
	require.NoError(t, err, "第二次Reconcile失败")
	assert.True(t, result.Requeue, "初始化后应该重新入队")

	// 验证状态已更新为Creating
	err = fakeClient.Get(ctx, req.NamespacedName, cluster)
	require.NoError(t, err)
	t.Logf("📊 当前集群状态: %s", cluster.Status.Phase)
	assert.Equal(t, etcdv1alpha1.EtcdClusterPhaseCreating, cluster.Status.Phase,
		"集群状态应该更新为Creating")

	// 8. 第三次Reconcile - 应该创建资源
	t.Log("🔄 执行第三次Reconcile (创建资源)")
	result, err = clusterController.Reconcile(ctx, req)
	require.NoError(t, err, "第三次Reconcile失败")

	t.Log("✅ 简化集成测试完成")
}

// TestFakeClientBasics 测试fake客户端的基本功能
// 确保我们的Mock K8s环境设置正确
func TestFakeClientBasics(t *testing.T) {
	// 1. 设置scheme
	testScheme := runtime.NewScheme()
	err := scheme.AddToScheme(testScheme)
	require.NoError(t, err)
	err = etcdv1alpha1.AddToScheme(testScheme)
	require.NoError(t, err)

	// 2. 创建fake客户端
	fakeClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithStatusSubresource(&etcdv1alpha1.EtcdCluster{}). // 启用状态子资源
		Build()

	ctx := context.Background()

	// 3. 测试创建资源
	cluster := &etcdv1alpha1.EtcdCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-fake",
			Namespace: "default",
		},
		Spec: etcdv1alpha1.EtcdClusterSpec{
			Size:    1,
			Version: "v3.5.21",
		},
	}

	err = fakeClient.Create(ctx, cluster)
	require.NoError(t, err, "创建集群应该成功")

	// 4. 测试获取资源
	retrieved := &etcdv1alpha1.EtcdCluster{}
	err = fakeClient.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, retrieved)
	require.NoError(t, err, "获取集群应该成功")
	assert.Equal(t, cluster.Name, retrieved.Name)

	// 5. 测试状态更新
	retrieved.Status.Phase = etcdv1alpha1.EtcdClusterPhaseCreating
	err = fakeClient.Status().Update(ctx, retrieved)
	require.NoError(t, err, "更新状态应该成功")

	// 6. 验证状态更新
	final := &etcdv1alpha1.EtcdCluster{}
	err = fakeClient.Get(ctx, types.NamespacedName{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
	}, final)
	require.NoError(t, err)
	assert.Equal(t, etcdv1alpha1.EtcdClusterPhaseCreating, final.Status.Phase,
		"状态应该正确更新")

	t.Log("✅ Fake客户端基础功能测试完成")
}
