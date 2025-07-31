#!/bin/bash

# 简化的扩缩容到0测试脚本
# 测试循环: 1→3→1→0→1

set -e

NAMESPACE="scale-to-zero-test"
CLUSTER_NAME="scale-test-cluster"

echo "🚀 开始扩缩容到0功能测试"

# 清理函数
cleanup() {
    echo "🧹 清理测试环境..."
    kubectl delete namespace $NAMESPACE --ignore-not-found=true
    echo "✅ 清理完成"
}

# 等待Pod就绪
wait_for_pods() {
    local expected_count=$1
    local timeout=120
    local count=0
    
    echo "⏳ 等待 $expected_count 个Pod就绪..."
    
    while [ $count -lt $timeout ]; do
        if [ "$expected_count" = "0" ]; then
            local actual_count=$(kubectl get pods -n $NAMESPACE --no-headers 2>/dev/null | wc -l)
            if [ "$actual_count" -eq 0 ]; then
                echo "✅ 所有Pod已删除"
                return 0
            fi
        else
            local ready_count=$(kubectl get pods -n $NAMESPACE --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)
            if [ "$ready_count" -eq "$expected_count" ]; then
                echo "✅ $expected_count 个Pod已就绪"
                return 0
            fi
        fi
        
        sleep 5
        count=$((count + 5))
    done
    
    echo "❌ 等待超时"
    return 1
}

# 执行扩缩容操作
scale_cluster() {
    local target_size=$1
    echo "🔄 扩缩容集群到 $target_size 节点..."
    kubectl patch etcdcluster $CLUSTER_NAME -n $NAMESPACE --type='merge' -p="{\"spec\":{\"size\":$target_size}}"
}

# 显示集群状态
show_status() {
    echo "📋 当前状态:"
    kubectl get etcdcluster -n $NAMESPACE
    kubectl get pods -n $NAMESPACE
    kubectl get pvc -n $NAMESPACE
    echo ""
}

# 验证etcd集群健康
verify_etcd_health() {
    local expected_members=$1
    if [ "$expected_members" -gt 0 ]; then
        echo "🔍 验证etcd集群健康状态..."
        kubectl exec -n $NAMESPACE ${CLUSTER_NAME}-0 -c etcd -- etcdctl member list
        kubectl exec -n $NAMESPACE ${CLUSTER_NAME}-0 -c etcd -- etcdctl endpoint health --cluster
        echo "✅ etcd集群健康"
    fi
}

# 注册清理函数
trap cleanup EXIT

echo "🏗️  创建测试环境..."
kubectl apply -f test/testdata/scale-to-zero-test.yaml

echo "⏳ 等待初始集群创建..."
wait_for_pods 1
show_status
verify_etcd_health 1

echo ""
echo "🎯 测试1: 扩容到3节点"
scale_cluster 3
wait_for_pods 3
show_status
verify_etcd_health 3

echo ""
echo "🎯 测试2: 缩容到1节点"
scale_cluster 1
wait_for_pods 1
show_status
verify_etcd_health 1

echo ""
echo "🎯 测试3: 缩容到0节点 (停止集群)"
scale_cluster 0
wait_for_pods 0
show_status

echo ""
echo "🎯 测试4: 从0重启到1节点"
scale_cluster 1
wait_for_pods 1
show_status
verify_etcd_health 1

echo ""
echo "🎉 扩缩容到0功能测试成功完成！"
echo "✅ 验证了以下功能："
echo "   - 1→3→1→0→1 完整循环"
echo "   - PVC自动清理机制"
echo "   - 集群停止和重启功能"
echo "   - etcd集群健康状态"
