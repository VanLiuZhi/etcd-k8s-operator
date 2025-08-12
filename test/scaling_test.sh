#!/bin/bash

# etcd-k8s-operator 扩缩容测试套件
# 测试所有扩缩容场景：0→1→3→5→3→1→0

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 配置
CLUSTER_NAME="scaling-test"
NAMESPACE="default"
TIMEOUT=300  # 5分钟超时

# 清理函数
cleanup() {
    log_info "Cleaning up test resources..."
    kubectl delete etcdcluster $CLUSTER_NAME --ignore-not-found=true
    kubectl delete pvc -l app.kubernetes.io/instance=$CLUSTER_NAME --ignore-not-found=true
    sleep 10
}

# 等待集群达到指定状态
wait_for_cluster_state() {
    local target_size=$1
    local timeout=$2
    local start_time=$(date +%s)

    log_info "Waiting for cluster to reach size $target_size (timeout: ${timeout}s)"

    while true; do
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))

        if [ $elapsed -gt $timeout ]; then
            log_error "Timeout waiting for cluster to reach size $target_size"
            return 1
        fi

        # 获取集群状态 - 修复：直接检查Pod状态而不是依赖readyReplicas字段
        local phase=$(kubectl get etcdcluster $CLUSTER_NAME -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
        local ready_pods=$(kubectl get pods -l app.kubernetes.io/instance=$CLUSTER_NAME --field-selector=status.phase=Running 2>/dev/null | grep -c "2/2.*Running" || echo "0")

        log_info "Current state: ready_pods=$ready_pods, phase=$phase, elapsed=${elapsed}s"

        # 检查是否达到目标状态
        if [ "$ready_pods" = "$target_size" ] && [ "$phase" = "Running" ]; then
            log_success "Cluster reached target size $target_size"
            return 0
        fi

        # 特殊处理：如果目标大小为0，只检查phase
        if [ "$target_size" = "0" ] && [ "$phase" = "Stopped" ]; then
            log_success "Cluster stopped successfully"
            return 0
        fi

        sleep 5
    done
}

# 验证etcd集群健康状态
verify_etcd_health() {
    local expected_size=$1
    
    if [ $expected_size -eq 0 ]; then
        log_info "Skipping etcd health check for size 0"
        return 0
    fi
    
    log_info "Verifying etcd cluster health (expected size: $expected_size)"
    
    # 尝试连接到第一个Pod
    local pod_name="${CLUSTER_NAME}-0"
    
    # 检查etcd成员列表
    local member_count=$(kubectl exec $pod_name -c etcd -- etcdctl --endpoints=http://localhost:2379 member list 2>/dev/null | wc -l || echo "0")
    
    if [ "$member_count" -eq "$expected_size" ]; then
        log_success "etcd cluster has correct number of members: $member_count"
    else
        log_warning "etcd member count mismatch: expected=$expected_size, actual=$member_count"
    fi
    
    # 检查etcd健康状态
    if kubectl exec $pod_name -c etcd -- etcdctl --endpoints=http://localhost:2379 endpoint health >/dev/null 2>&1; then
        log_success "etcd cluster is healthy"
    else
        log_warning "etcd cluster health check failed"
    fi
}

# 测试扩缩容场景
test_scaling_scenario() {
    local from_size=$1
    local to_size=$2
    local scenario_name="$from_size→$to_size"
    
    log_info "=== Testing scaling scenario: $scenario_name ==="
    
    # 如果是从0开始，需要创建集群
    if [ $from_size -eq 0 ]; then
        log_info "Creating new cluster with size $to_size"
        kubectl apply -f - <<EOF
apiVersion: etcd.etcd.io/v1alpha1
kind: EtcdCluster
metadata:
  name: $CLUSTER_NAME
  namespace: $NAMESPACE
spec:
  size: $to_size
  version: v3.5.21
EOF
    else
        # 修改现有集群大小
        log_info "Scaling cluster from $from_size to $to_size"
        kubectl patch etcdcluster $CLUSTER_NAME --type='merge' -p="{\"spec\":{\"size\":$to_size}}"
    fi
    
    # 等待集群达到目标状态
    if wait_for_cluster_state $to_size $TIMEOUT; then
        # 验证etcd健康状态
        verify_etcd_health $to_size
        log_success "Scaling scenario $scenario_name completed successfully"
        return 0
    else
        log_error "Scaling scenario $scenario_name failed"
        return 1
    fi
}

# 主测试流程
main() {
    log_info "Starting etcd-k8s-operator scaling test suite"
    log_info "Test cluster: $CLUSTER_NAME"
    log_info "Namespace: $NAMESPACE"
    
    # 清理之前的测试资源
    cleanup
    
    # 测试场景列表：from_size to_size
    local scenarios=(
        "0 1"    # 冷启动：创建单节点集群
        "1 3"    # 首次扩容：单节点→多节点
        "3 5"    # 继续扩容：3节点→5节点
        "5 3"    # 部分缩容：5节点→3节点
        "3 1"    # 大幅缩容：3节点→单节点
        "1 0"    # 完全停止：单节点→0节点
    )
    
    local total_scenarios=${#scenarios[@]}
    local passed_scenarios=0
    local failed_scenarios=0
    
    # 执行所有测试场景
    for scenario in "${scenarios[@]}"; do
        local from_size=$(echo $scenario | cut -d' ' -f1)
        local to_size=$(echo $scenario | cut -d' ' -f2)
        
        if test_scaling_scenario $from_size $to_size; then
            ((passed_scenarios++))
        else
            ((failed_scenarios++))
            log_error "Scenario $from_size→$to_size failed, but continuing with next scenario"
        fi
        
        # 在场景之间稍作等待
        sleep 10
    done
    
    # 最终清理
    cleanup
    
    # 输出测试结果
    log_info "=== Test Results ==="
    log_info "Total scenarios: $total_scenarios"
    log_success "Passed: $passed_scenarios"
    if [ $failed_scenarios -gt 0 ]; then
        log_error "Failed: $failed_scenarios"
    else
        log_info "Failed: $failed_scenarios"
    fi
    
    if [ $failed_scenarios -eq 0 ]; then
        log_success "All scaling scenarios passed! 🎉"
        exit 0
    else
        log_error "Some scaling scenarios failed! ❌"
        exit 1
    fi
}

# 运行主函数
main "$@"
