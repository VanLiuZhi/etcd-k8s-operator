#!/bin/bash

# ETCD Operator 端到端测试执行脚本

set -euo pipefail

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

# 配置变量
PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
E2E_NAMESPACE="etcd-operator-e2e"
CLUSTER_NAME="etcd-operator-e2e"
OPERATOR_IMAGE="etcd-operator:e2e"

# 检查环境
check_environment() {
    log_info "检查端到端测试环境..."
    
    # 检查必需工具
    local required_tools=("kubectl" "kind" "docker")
    for tool in "${required_tools[@]}"; do
        if ! command -v "$tool" &> /dev/null; then
            log_error "$tool 未安装"
            exit 1
        fi
    done
    
    # 检查 Kind 集群
    if ! kind get clusters | grep -q "^$CLUSTER_NAME$"; then
        log_error "Kind 集群 $CLUSTER_NAME 不存在"
        log_info "请运行: scripts/test/setup-test-env.sh $CLUSTER_NAME"
        exit 1
    fi
    
    # 切换到正确的 kubectl 上下文
    kubectl config use-context "kind-$CLUSTER_NAME"
    
    log_success "环境检查通过"
}

# 部署完整的 Operator
deploy_operator() {
    log_info "部署完整的 ETCD Operator..."
    
    cd "$PROJECT_ROOT"
    
    # 构建最新镜像
    make docker-build IMG="$OPERATOR_IMAGE"
    
    # 加载镜像到 Kind 集群
    kind load docker-image "$OPERATOR_IMAGE" --name "$CLUSTER_NAME"
    
    # 创建命名空间
    kubectl create namespace "$E2E_NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
    
    # 安装 CRD
    make install
    
    # 部署 Operator
    make deploy IMG="$OPERATOR_IMAGE"
    
    # 等待 Operator 就绪
    kubectl wait --for=condition=available --timeout=300s deployment/etcd-k8s-operator-controller-manager -n etcd-k8s-operator-system
    
    log_success "Operator 部署完成"
}

# 运行完整场景测试
run_full_scenario_tests() {
    log_info "运行完整场景测试..."
    
    # 测试场景 1: 基础集群生命周期
    test_basic_cluster_lifecycle
    
    # 测试场景 2: 集群扩缩容
    test_cluster_scaling
    
    # 测试场景 3: 故障恢复
    test_failure_recovery
    
    # 测试场景 4: 数据持久化
    test_data_persistence
    
    log_success "完整场景测试完成"
}

# 测试基础集群生命周期
test_basic_cluster_lifecycle() {
    log_info "测试场景 1: 基础集群生命周期"
    
    local cluster_name="e2e-basic-$(date +%s)"
    local cluster_manifest="/tmp/e2e-basic-cluster.yaml"
    
    # 创建集群配置
    cat > "$cluster_manifest" << EOF
apiVersion: etcd.etcd.io/v1alpha1
kind: EtcdCluster
metadata:
  name: $cluster_name
  namespace: $E2E_NAMESPACE
spec:
  size: 3
  version: "v3.5.21"
  repository: "quay.io/coreos/etcd"
  storage:
    size: "2Gi"
  security:
    tls:
      enabled: true
      autoTLS: true
  resources:
    requests:
      cpu: "100m"
      memory: "128Mi"
    limits:
      cpu: "500m"
      memory: "512Mi"
EOF
    
    # 创建集群
    log_info "创建集群: $cluster_name"
    kubectl apply -f "$cluster_manifest"
    
    # 等待集群就绪
    wait_for_cluster_ready "$cluster_name" 600
    
    # 验证集群功能
    verify_cluster_functionality "$cluster_name"
    
    # 删除集群
    log_info "删除集群: $cluster_name"
    kubectl delete -f "$cluster_manifest"
    
    # 等待集群清理
    wait_for_cluster_deletion "$cluster_name" 300
    
    rm -f "$cluster_manifest"
    log_success "基础集群生命周期测试完成"
}

# 测试集群扩缩容
test_cluster_scaling() {
    log_info "测试场景 2: 集群扩缩容"
    
    local cluster_name="e2e-scaling-$(date +%s)"
    local cluster_manifest="/tmp/e2e-scaling-cluster.yaml"
    
    # 创建初始集群 (3 节点)
    cat > "$cluster_manifest" << EOF
apiVersion: etcd.etcd.io/v1alpha1
kind: EtcdCluster
metadata:
  name: $cluster_name
  namespace: $E2E_NAMESPACE
spec:
  size: 3
  version: "v3.5.21"
  repository: "quay.io/coreos/etcd"
  storage:
    size: "1Gi"
EOF
    
    kubectl apply -f "$cluster_manifest"
    wait_for_cluster_ready "$cluster_name" 600
    
    # 扩容到 5 节点
    log_info "扩容集群到 5 节点"
    kubectl patch etcdcluster "$cluster_name" -n "$E2E_NAMESPACE" --type='merge' -p='{"spec":{"size":5}}'
    wait_for_cluster_size "$cluster_name" 5 300
    
    # 缩容到 3 节点
    log_info "缩容集群到 3 节点"
    kubectl patch etcdcluster "$cluster_name" -n "$E2E_NAMESPACE" --type='merge' -p='{"spec":{"size":3}}'
    wait_for_cluster_size "$cluster_name" 3 300
    
    # 清理
    kubectl delete -f "$cluster_manifest"
    wait_for_cluster_deletion "$cluster_name" 300
    
    rm -f "$cluster_manifest"
    log_success "集群扩缩容测试完成"
}

# 测试故障恢复
test_failure_recovery() {
    log_info "测试场景 3: 故障恢复"
    
    local cluster_name="e2e-recovery-$(date +%s)"
    local cluster_manifest="/tmp/e2e-recovery-cluster.yaml"
    
    # 创建集群
    cat > "$cluster_manifest" << EOF
apiVersion: etcd.etcd.io/v1alpha1
kind: EtcdCluster
metadata:
  name: $cluster_name
  namespace: $E2E_NAMESPACE
spec:
  size: 3
  version: "3.5.9"
  repository: "quay.io/coreos/etcd"
  storage:
    size: "1Gi"
EOF
    
    kubectl apply -f "$cluster_manifest"
    wait_for_cluster_ready "$cluster_name" 600
    
    # 模拟 Pod 故障
    log_info "模拟 Pod 故障"
    kubectl delete pod "$cluster_name-0" -n "$E2E_NAMESPACE"
    
    # 等待自动恢复
    log_info "等待自动恢复..."
    sleep 60
    wait_for_cluster_ready "$cluster_name" 300
    
    # 验证集群仍然正常工作
    verify_cluster_functionality "$cluster_name"
    
    # 清理
    kubectl delete -f "$cluster_manifest"
    wait_for_cluster_deletion "$cluster_name" 300
    
    rm -f "$cluster_manifest"
    log_success "故障恢复测试完成"
}

# 测试数据持久化
test_data_persistence() {
    log_info "测试场景 4: 数据持久化"
    
    local cluster_name="e2e-persistence-$(date +%s)"
    local cluster_manifest="/tmp/e2e-persistence-cluster.yaml"
    
    # 创建集群
    cat > "$cluster_manifest" << EOF
apiVersion: etcd.etcd.io/v1alpha1
kind: EtcdCluster
metadata:
  name: $cluster_name
  namespace: $E2E_NAMESPACE
spec:
  size: 3
  version: "3.5.9"
  repository: "quay.io/coreos/etcd"
  storage:
    size: "1Gi"
EOF
    
    kubectl apply -f "$cluster_manifest"
    wait_for_cluster_ready "$cluster_name" 600
    
    # 写入测试数据
    log_info "写入测试数据"
    write_test_data "$cluster_name"
    
    # 重启集群 (删除所有 Pod)
    log_info "重启集群"
    kubectl delete pods -l "etcd.etcd.io/cluster=$cluster_name" -n "$E2E_NAMESPACE"
    
    # 等待集群恢复
    wait_for_cluster_ready "$cluster_name" 300
    
    # 验证数据持久化
    log_info "验证数据持久化"
    verify_test_data "$cluster_name"
    
    # 清理
    kubectl delete -f "$cluster_manifest"
    wait_for_cluster_deletion "$cluster_name" 300
    
    rm -f "$cluster_manifest"
    log_success "数据持久化测试完成"
}

# 等待集群就绪
wait_for_cluster_ready() {
    local cluster_name=$1
    local timeout=$2
    local elapsed=0
    
    log_info "等待集群就绪: $cluster_name (超时: ${timeout}s)"
    
    while [[ $elapsed -lt $timeout ]]; do
        local phase=$(kubectl get etcdcluster "$cluster_name" -n "$E2E_NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
        local ready_replicas=$(kubectl get etcdcluster "$cluster_name" -n "$E2E_NAMESPACE" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        local desired_size=$(kubectl get etcdcluster "$cluster_name" -n "$E2E_NAMESPACE" -o jsonpath='{.spec.size}' 2>/dev/null || echo "0")
        
        if [[ "$phase" == "Running" && "$ready_replicas" == "$desired_size" ]]; then
            log_success "集群就绪: $cluster_name"
            return 0
        elif [[ "$phase" == "Failed" ]]; then
            log_error "集群创建失败: $cluster_name"
            kubectl describe etcdcluster "$cluster_name" -n "$E2E_NAMESPACE"
            return 1
        fi
        
        sleep 10
        elapsed=$((elapsed + 10))
        log_info "等待中... ($elapsed/${timeout}s) 状态: $phase, 就绪: $ready_replicas/$desired_size"
    done
    
    log_error "等待集群就绪超时: $cluster_name"
    return 1
}

# 等待集群大小变更
wait_for_cluster_size() {
    local cluster_name=$1
    local expected_size=$2
    local timeout=$3
    local elapsed=0
    
    log_info "等待集群大小变更: $cluster_name -> $expected_size"
    
    while [[ $elapsed -lt $timeout ]]; do
        local ready_replicas=$(kubectl get etcdcluster "$cluster_name" -n "$E2E_NAMESPACE" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        
        if [[ "$ready_replicas" == "$expected_size" ]]; then
            log_success "集群大小变更完成: $cluster_name -> $expected_size"
            return 0
        fi
        
        sleep 10
        elapsed=$((elapsed + 10))
        log_info "等待中... ($elapsed/${timeout}s) 当前大小: $ready_replicas/$expected_size"
    done
    
    log_error "等待集群大小变更超时: $cluster_name"
    return 1
}

# 等待集群删除
wait_for_cluster_deletion() {
    local cluster_name=$1
    local timeout=$2
    local elapsed=0
    
    log_info "等待集群删除: $cluster_name"
    
    while [[ $elapsed -lt $timeout ]]; do
        if ! kubectl get etcdcluster "$cluster_name" -n "$E2E_NAMESPACE" &> /dev/null; then
            log_success "集群删除完成: $cluster_name"
            return 0
        fi
        
        sleep 5
        elapsed=$((elapsed + 5))
        log_info "等待删除中... ($elapsed/${timeout}s)"
    done
    
    log_error "等待集群删除超时: $cluster_name"
    return 1
}

# 验证集群功能
verify_cluster_functionality() {
    local cluster_name=$1
    
    log_info "验证集群功能: $cluster_name"
    
    # 检查 StatefulSet
    kubectl get statefulset "$cluster_name" -n "$E2E_NAMESPACE"
    
    # 检查 Services
    kubectl get service "$cluster_name-client" -n "$E2E_NAMESPACE"
    kubectl get service "$cluster_name-peer" -n "$E2E_NAMESPACE"
    
    # 检查 Pods
    kubectl get pods -l "etcd.etcd.io/cluster=$cluster_name" -n "$E2E_NAMESPACE"
    
    # 检查 PVC
    kubectl get pvc -l "etcd.etcd.io/cluster=$cluster_name" -n "$E2E_NAMESPACE"
    
    log_success "集群功能验证完成: $cluster_name"
}

# 写入测试数据
write_test_data() {
    local cluster_name=$1
    
    log_info "写入测试数据到集群: $cluster_name"
    
    # 使用 etcdctl 写入数据
    kubectl exec "$cluster_name-0" -n "$E2E_NAMESPACE" -- etcdctl put test-key "test-value-$(date +%s)"
    kubectl exec "$cluster_name-0" -n "$E2E_NAMESPACE" -- etcdctl put persistent-key "persistent-value"
    
    log_success "测试数据写入完成"
}

# 验证测试数据
verify_test_data() {
    local cluster_name=$1
    
    log_info "验证测试数据: $cluster_name"
    
    # 验证数据存在
    local value=$(kubectl exec "$cluster_name-0" -n "$E2E_NAMESPACE" -- etcdctl get persistent-key --print-value-only 2>/dev/null || echo "")
    
    if [[ "$value" == "persistent-value" ]]; then
        log_success "数据持久化验证成功"
    else
        log_error "数据持久化验证失败: 期望 'persistent-value', 实际 '$value'"
        return 1
    fi
}

# 清理测试环境
cleanup_e2e_environment() {
    log_info "清理端到端测试环境..."
    
    # 清理所有测试资源
    kubectl delete etcdclusters --all -n "$E2E_NAMESPACE" --timeout=300s || true
    kubectl delete namespace "$E2E_NAMESPACE" --timeout=60s || true
    
    # 卸载 Operator
    make undeploy || true
    
    log_success "端到端测试环境清理完成"
}

# 主函数
main() {
    local cleanup=true
    
    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --no-cleanup)
                cleanup=false
                shift
                ;;
            -h|--help)
                echo "用法: $0 [选项]"
                echo "选项:"
                echo "  --no-cleanup    不清理测试环境"
                echo "  -h, --help      显示帮助信息"
                exit 0
                ;;
            *)
                log_error "未知选项: $1"
                exit 1
                ;;
        esac
    done
    
    log_info "开始运行端到端测试..."
    
    # 设置清理陷阱
    if [[ "$cleanup" == true ]]; then
        trap cleanup_e2e_environment EXIT
    fi
    
    check_environment
    deploy_operator
    run_full_scenario_tests
    
    log_success "端到端测试完成！"
}

# 脚本入口
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
