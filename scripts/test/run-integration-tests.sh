#!/bin/bash

# ETCD Operator 集成测试执行脚本

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
TEST_NAMESPACE="etcd-operator-test"
CLUSTER_NAME="etcd-operator-test"

# 检查环境
check_environment() {
    log_info "检查集成测试环境..."
    
    # 检查 kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl 未安装"
        exit 1
    fi
    
    # 检查 Kubernetes 连接
    if ! kubectl cluster-info &> /dev/null; then
        log_error "无法连接到 Kubernetes 集群"
        log_info "请确保 Kind 集群正在运行: kind get clusters"
        exit 1
    fi
    
    # 检查 ginkgo
    if ! command -v ginkgo &> /dev/null; then
        log_warning "ginkgo 未安装，使用 go test 运行"
    fi
    
    log_success "环境检查通过"
}

# 设置测试命名空间
setup_test_namespace() {
    log_info "设置测试命名空间: $TEST_NAMESPACE"
    
    # 创建命名空间（如果不存在）
    if ! kubectl get namespace "$TEST_NAMESPACE" &> /dev/null; then
        kubectl create namespace "$TEST_NAMESPACE"
        log_success "创建命名空间: $TEST_NAMESPACE"
    else
        log_info "命名空间已存在: $TEST_NAMESPACE"
    fi
}

# 安装 CRD
install_crds() {
    log_info "安装 CRD..."
    
    cd "$PROJECT_ROOT"
    
    # 生成最新的 CRD
    make manifests
    
    # 安装 CRD
    kubectl apply -f config/crd/bases/
    
    # 等待 CRD 就绪
    log_info "等待 CRD 就绪..."
    kubectl wait --for condition=established --timeout=60s crd/etcdclusters.etcd.etcd.io
    kubectl wait --for condition=established --timeout=60s crd/etcdbackups.etcd.etcd.io
    kubectl wait --for condition=established --timeout=60s crd/etcdrestores.etcd.etcd.io
    
    log_success "CRD 安装完成"
}

# 构建和加载镜像
build_and_load_image() {
    log_info "构建和加载 Operator 镜像..."
    
    cd "$PROJECT_ROOT"
    
    local image_name="etcd-operator:test"
    
    # 构建镜像
    make docker-build IMG="$image_name"
    
    # 加载镜像到 Kind 集群
    kind load docker-image "$image_name" --name "$CLUSTER_NAME"
    
    log_success "镜像构建和加载完成"
}

# 运行集成测试
run_integration_tests() {
    log_info "运行集成测试..."
    
    cd "$PROJECT_ROOT"
    
    # 设置环境变量
    export KUBEBUILDER_ASSETS="$PROJECT_ROOT/bin/k8s/1.28.3-$(go env GOOS)-$(go env GOARCH)"
    export TEST_NAMESPACE="$TEST_NAMESPACE"
    
    # 确保测试二进制文件存在
    if [[ ! -d "$KUBEBUILDER_ASSETS" ]]; then
        log_info "下载测试二进制文件..."
        make envtest
    fi
    
    # 运行集成测试
    local test_args=(
        "-v"
        "-timeout=30m"
        "-count=1"
        "./test/integration/..."
    )
    
    if command -v ginkgo &> /dev/null; then
        # 使用 Ginkgo 运行测试
        ginkgo -v --timeout=30m --fail-fast ./test/integration/
    else
        # 使用 go test 运行测试
        go test "${test_args[@]}"
    fi
    
    log_success "集成测试完成"
}

# 运行控制器测试
run_controller_tests() {
    log_info "运行控制器集成测试..."
    
    cd "$PROJECT_ROOT"
    
    # 部署 Operator 到测试集群
    log_info "部署 Operator 到测试集群..."
    
    # 创建 RBAC
    kubectl apply -f config/rbac/
    
    # 部署 Operator
    local operator_manifest="/tmp/operator-test.yaml"
    cat > "$operator_manifest" << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: etcd-operator-controller-manager
  namespace: $TEST_NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      control-plane: controller-manager
  template:
    metadata:
      labels:
        control-plane: controller-manager
    spec:
      containers:
      - name: manager
        image: etcd-operator:test
        imagePullPolicy: Never
        command:
        - /manager
        env:
        - name: WATCH_NAMESPACE
          value: "$TEST_NAMESPACE"
        resources:
          limits:
            cpu: 500m
            memory: 128Mi
          requests:
            cpu: 10m
            memory: 64Mi
      serviceAccountName: etcd-operator-controller-manager
      terminationGracePeriodSeconds: 10
EOF
    
    kubectl apply -f "$operator_manifest"
    
    # 等待 Operator 就绪
    kubectl wait --for=condition=available --timeout=300s deployment/etcd-operator-controller-manager -n "$TEST_NAMESPACE"
    
    log_success "Operator 部署完成"
    
    # 运行实际的集群测试
    test_etcd_cluster_lifecycle
    
    # 清理
    kubectl delete -f "$operator_manifest" || true
    rm -f "$operator_manifest"
}

# 测试 EtcdCluster 生命周期
test_etcd_cluster_lifecycle() {
    log_info "测试 EtcdCluster 生命周期..."
    
    local cluster_name="test-cluster-$(date +%s)"
    local cluster_manifest="/tmp/test-cluster.yaml"
    
    # 创建测试集群
    cat > "$cluster_manifest" << EOF
apiVersion: etcd.etcd.io/v1alpha1
kind: EtcdCluster
metadata:
  name: $cluster_name
  namespace: $TEST_NAMESPACE
spec:
  size: 3
  version: "3.5.9"
  repository: "quay.io/coreos/etcd"
  storage:
    size: "1Gi"
  security:
    tls:
      enabled: true
      autoTLS: true
EOF
    
    log_info "创建测试集群: $cluster_name"
    kubectl apply -f "$cluster_manifest"
    
    # 等待集群创建
    log_info "等待集群创建..."
    local timeout=300
    local elapsed=0
    while [[ $elapsed -lt $timeout ]]; do
        local phase=$(kubectl get etcdcluster "$cluster_name" -n "$TEST_NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
        if [[ "$phase" == "Running" ]]; then
            log_success "集群创建成功"
            break
        elif [[ "$phase" == "Failed" ]]; then
            log_error "集群创建失败"
            kubectl describe etcdcluster "$cluster_name" -n "$TEST_NAMESPACE"
            return 1
        fi
        
        sleep 10
        elapsed=$((elapsed + 10))
        log_info "等待中... ($elapsed/${timeout}s) 当前状态: $phase"
    done
    
    if [[ $elapsed -ge $timeout ]]; then
        log_error "集群创建超时"
        kubectl describe etcdcluster "$cluster_name" -n "$TEST_NAMESPACE"
        return 1
    fi
    
    # 验证资源创建
    log_info "验证资源创建..."
    kubectl get statefulset "$cluster_name" -n "$TEST_NAMESPACE"
    kubectl get service "$cluster_name-client" -n "$TEST_NAMESPACE"
    kubectl get service "$cluster_name-peer" -n "$TEST_NAMESPACE"
    kubectl get configmap "$cluster_name-config" -n "$TEST_NAMESPACE"
    
    # 测试扩缩容
    log_info "测试集群扩容..."
    kubectl patch etcdcluster "$cluster_name" -n "$TEST_NAMESPACE" --type='merge' -p='{"spec":{"size":5}}'
    
    # 等待扩容完成
    sleep 30
    local ready_replicas=$(kubectl get etcdcluster "$cluster_name" -n "$TEST_NAMESPACE" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
    log_info "当前就绪副本数: $ready_replicas"
    
    # 清理测试集群
    log_info "清理测试集群..."
    kubectl delete -f "$cluster_manifest" || true
    rm -f "$cluster_manifest"
    
    # 等待资源清理
    local cleanup_timeout=120
    local cleanup_elapsed=0
    while [[ $cleanup_elapsed -lt $cleanup_timeout ]]; do
        if ! kubectl get etcdcluster "$cluster_name" -n "$TEST_NAMESPACE" &> /dev/null; then
            log_success "集群清理完成"
            break
        fi
        sleep 5
        cleanup_elapsed=$((cleanup_elapsed + 5))
    done
    
    log_success "EtcdCluster 生命周期测试完成"
}

# 清理测试环境
cleanup_test_environment() {
    log_info "清理测试环境..."
    
    # 清理测试命名空间中的资源
    kubectl delete etcdclusters --all -n "$TEST_NAMESPACE" --timeout=60s || true
    kubectl delete etcdbackups --all -n "$TEST_NAMESPACE" --timeout=60s || true
    kubectl delete etcdrestores --all -n "$TEST_NAMESPACE" --timeout=60s || true
    
    # 等待资源清理
    sleep 10
    
    # 删除测试命名空间
    kubectl delete namespace "$TEST_NAMESPACE" --timeout=60s || true
    
    log_success "测试环境清理完成"
}

# 显示测试结果
show_test_results() {
    log_info "集成测试结果总结:"
    echo "  项目根目录: $PROJECT_ROOT"
    echo "  测试命名空间: $TEST_NAMESPACE"
    echo "  集群名称: $CLUSTER_NAME"
    
    # 显示集群状态
    log_info "当前集群状态:"
    kubectl get nodes || true
    kubectl get pods -A | grep etcd || true
}

# 主函数
main() {
    local skip_build=false
    local skip_controller=false
    local cleanup=true
    
    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-build)
                skip_build=true
                shift
                ;;
            --skip-controller)
                skip_controller=true
                shift
                ;;
            --no-cleanup)
                cleanup=false
                shift
                ;;
            -h|--help)
                echo "用法: $0 [选项]"
                echo "选项:"
                echo "  --skip-build         跳过镜像构建"
                echo "  --skip-controller    跳过控制器测试"
                echo "  --no-cleanup         不清理测试环境"
                echo "  -h, --help           显示帮助信息"
                exit 0
                ;;
            *)
                log_error "未知选项: $1"
                exit 1
                ;;
        esac
    done
    
    log_info "开始运行集成测试..."
    
    # 设置清理陷阱
    if [[ "$cleanup" == true ]]; then
        trap cleanup_test_environment EXIT
    fi
    
    check_environment
    setup_test_namespace
    install_crds
    
    if [[ "$skip_build" != true ]]; then
        build_and_load_image
    fi
    
    run_integration_tests
    
    if [[ "$skip_controller" != true ]]; then
        run_controller_tests
    fi
    
    show_test_results
    
    log_success "集成测试完成！"
}

# 脚本入口
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
