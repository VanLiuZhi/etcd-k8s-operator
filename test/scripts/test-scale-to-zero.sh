#!/bin/bash

# 完整的扩缩容到0测试脚本
# 测试循环: 0→1→3→1→0→1→3→1→0

set -e

NAMESPACE="scale-to-zero-test"
CLUSTER_NAME="scale-test-cluster"

echo "🚀 开始完整的扩缩容到0测试"

# 清理函数
cleanup() {
    echo "🧹 清理测试环境..."
    kubectl delete namespace $NAMESPACE --ignore-not-found=true
    echo "✅ 清理完成"
}

# 等待集群达到指定状态
wait_for_cluster() {
    local expected_size=$1
    local expected_phase=$2
    local timeout=300
    local count=0
    
    echo "⏳ 等待集群达到 size=$expected_size, phase=$expected_phase..."
    
    while [ $count -lt $timeout ]; do
        local current_size=$(kubectl get etcdcluster $CLUSTER_NAME -n $NAMESPACE -o jsonpath='{.spec.size}' 2>/dev/null || echo "0")
        local current_phase=$(kubectl get etcdcluster $CLUSTER_NAME -n $NAMESPACE -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
        local ready_replicas=$(kubectl get etcdcluster $CLUSTER_NAME -n $NAMESPACE -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        
        echo "  当前状态: size=$current_size, phase=$current_phase, ready=$ready_replicas"
        
        if [ "$current_size" = "$expected_size" ] && [ "$current_phase" = "$expected_phase" ]; then
            if [ "$expected_size" = "0" ] || [ "$ready_replicas" = "$expected_size" ]; then
                echo "✅ 集群已达到预期状态"
                return 0
            fi
        fi
        
        sleep 5
        count=$((count + 5))
    done
    
    echo "❌ 等待超时，集群未达到预期状态"
    return 1
}

# 验证PVC数量
verify_pvc_count() {
    local expected_count=$1
    local actual_count=$(kubectl get pvc -n $NAMESPACE --no-headers 2>/dev/null | wc -l)
    
    echo "📊 验证PVC数量: 期望=$expected_count, 实际=$actual_count"
    
    if [ "$actual_count" -eq "$expected_count" ]; then
        echo "✅ PVC数量正确"
        return 0
    else
        echo "❌ PVC数量不匹配"
        kubectl get pvc -n $NAMESPACE
        return 1
    fi
}

# 验证Pod状态
verify_pod_status() {
    local expected_count=$1
    
    if [ "$expected_count" = "0" ]; then
        local actual_count=$(kubectl get pods -n $NAMESPACE --no-headers 2>/dev/null | wc -l)
        echo "📊 验证Pod数量: 期望=0, 实际=$actual_count"
        if [ "$actual_count" -eq 0 ]; then
            echo "✅ 无Pod运行，符合预期"
            return 0
        else
            echo "❌ 仍有Pod运行"
            kubectl get pods -n $NAMESPACE
            return 1
        fi
    else
        local running_count=$(kubectl get pods -n $NAMESPACE --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)
        echo "📊 验证运行Pod数量: 期望=$expected_count, 实际=$running_count"
        if [ "$running_count" -eq "$expected_count" ]; then
            echo "✅ Pod数量正确且运行正常"
            return 0
        else
            echo "❌ Pod数量或状态不正确"
            kubectl get pods -n $NAMESPACE
            return 1
        fi
    fi
}

# 执行扩缩容操作
scale_cluster() {
    local target_size=$1
    echo "🔄 扩缩容集群到 $target_size 节点..."
    kubectl patch etcdcluster $CLUSTER_NAME -n $NAMESPACE --type='merge' -p="{\"spec\":{\"size\":$target_size}}"
}

# 显示集群状态
show_cluster_status() {
    echo "📋 当前集群状态:"
    kubectl get etcdcluster -n $NAMESPACE -o wide
    echo ""
    kubectl get pods -n $NAMESPACE
    echo ""
    kubectl get pvc -n $NAMESPACE
    echo ""
}

# 注册清理函数
trap cleanup EXIT

echo "🏗️  创建测试环境..."
kubectl apply -f test/testdata/scale-to-zero-test.yaml

echo "⏳ 等待初始集群创建..."
wait_for_cluster 1 "Running"
verify_pod_status 1
verify_pvc_count 1
show_cluster_status

echo ""
echo "🎯 开始第一轮测试循环: 1→3→1→0"

# 1→3
echo "📈 扩容到3节点..."
scale_cluster 3
wait_for_cluster 3 "Running"
verify_pod_status 3
verify_pvc_count 3
show_cluster_status

# 3→1
echo "📉 缩容到1节点..."
scale_cluster 1
wait_for_cluster 1 "Running"
verify_pod_status 1
verify_pvc_count 1  # 关键测试：PVC应该被清理
show_cluster_status

# 1→0
echo "🛑 缩容到0节点 (停止集群)..."
scale_cluster 0
wait_for_cluster 0 "Stopped"
verify_pod_status 0
verify_pvc_count 0  # 关键测试：所有PVC应该被清理
show_cluster_status

echo ""
echo "🎯 开始第二轮测试循环: 0→1→3→1→0"

# 0→1
echo "🚀 从停止状态重启到1节点..."
scale_cluster 1
wait_for_cluster 1 "Running"
verify_pod_status 1
verify_pvc_count 1
show_cluster_status

# 1→3
echo "📈 扩容到3节点..."
scale_cluster 3
wait_for_cluster 3 "Running"
verify_pod_status 3
verify_pvc_count 3
show_cluster_status

# 3→1
echo "📉 缩容到1节点..."
scale_cluster 1
wait_for_cluster 1 "Running"
verify_pod_status 1
verify_pvc_count 1
show_cluster_status

# 1→0
echo "🛑 最终缩容到0节点..."
scale_cluster 0
wait_for_cluster 0 "Stopped"
verify_pod_status 0
verify_pvc_count 0
show_cluster_status

echo ""
echo "🎉 完整的扩缩容到0测试成功完成！"
echo "✅ 验证了以下功能："
echo "   - 1→3→1→0→1→3→1→0 完整循环"
echo "   - PVC自动清理机制"
echo "   - 集群停止和重启功能"
echo "   - 多轮循环稳定性"
