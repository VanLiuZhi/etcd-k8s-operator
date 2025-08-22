#!/bin/bash

# ETCD Operator 测试环境设置脚本
# 适配 Mac + OrbStack 环境

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

# 检查必需的工具
check_prerequisites() {
    log_info "检查必需的工具..."
    
    local missing_tools=()
    
    # 检查 Go
    if ! command -v go &> /dev/null; then
        missing_tools+=("go")
    else
        local go_version=$(go version | awk '{print $3}' | sed 's/go//')
        log_info "Go 版本: $go_version"
    fi
    
    # 检查 Docker (OrbStack)
    if ! command -v docker &> /dev/null; then
        missing_tools+=("docker")
    else
        log_info "Docker 版本: $(docker --version)"
    fi
    
    # 检查 kubectl
    if ! command -v kubectl &> /dev/null; then
        missing_tools+=("kubectl")
    else
        log_info "kubectl 版本: $(kubectl version --client --short 2>/dev/null || echo 'N/A')"
    fi
    
    # 检查 kind
    if ! command -v kind &> /dev/null; then
        missing_tools+=("kind")
    else
        log_info "Kind 版本: $(kind version)"
    fi
    
    if [ ${#missing_tools[@]} -ne 0 ]; then
        log_error "缺少必需的工具: ${missing_tools[*]}"
        log_info "请安装缺少的工具后重试"
        exit 1
    fi
    
    log_success "所有必需的工具都已安装"
}

# 检查 OrbStack 状态
check_orbstack() {
    log_info "检查 OrbStack 状态..."
    
    # 检查 Docker 是否运行
    if ! docker info &> /dev/null; then
        log_error "Docker 未运行，请启动 OrbStack"
        exit 1
    fi
    
    # 检查是否在 OrbStack 环境中
    if docker info 2>/dev/null | grep -q "orbstack"; then
        log_success "检测到 OrbStack 环境"
    else
        log_warning "未检测到 OrbStack，但 Docker 正在运行"
    fi
}

# 设置 Go 环境
setup_go_env() {
    log_info "设置 Go 环境..."
    
    # 设置 Go 代理（针对中国用户）
    if [[ "${GOPROXY:-}" == "" ]]; then
        export GOPROXY=https://goproxy.cn,direct
        log_info "设置 GOPROXY=$GOPROXY"
    fi
    
    # 确保 Go 模块缓存目录存在
    local go_cache_dir=$(go env GOCACHE)
    if [[ ! -d "$go_cache_dir" ]]; then
        mkdir -p "$go_cache_dir"
    fi
    
    log_success "Go 环境设置完成"
}

# 安装测试工具
install_test_tools() {
    log_info "安装测试工具..."
    
    # 安装 ginkgo
    if ! command -v ginkgo &> /dev/null; then
        log_info "安装 Ginkgo..."
        go install github.com/onsi/ginkgo/v2/ginkgo@latest
    fi
    
    # 安装 golangci-lint
    if ! command -v golangci-lint &> /dev/null; then
        log_info "安装 golangci-lint..."
        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.54.2
    fi
    
    # 安装 controller-gen
    if ! command -v controller-gen &> /dev/null; then
        log_info "安装 controller-gen..."
        go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
    fi
    
    log_success "测试工具安装完成"
}

# 创建 Kind 集群
create_kind_cluster() {
    local cluster_name=${1:-"etcd-operator-test"}
    
    log_info "创建 Kind 集群: $cluster_name"
    
    # 检查集群是否已存在
    if kind get clusters | grep -q "^$cluster_name$"; then
        log_warning "集群 $cluster_name 已存在，跳过创建"
        return 0
    fi
    
    # 创建 Kind 配置文件
    local kind_config="/tmp/kind-config.yaml"
    cat > "$kind_config" << EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: $cluster_name
nodes:
- role: control-plane
  image: kindest/node:v1.28.0
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
- role: worker
  image: kindest/node:v1.28.0
- role: worker
  image: kindest/node:v1.28.0
EOF
    
    # 创建集群
    if kind create cluster --config="$kind_config"; then
        log_success "Kind 集群 $cluster_name 创建成功"
    else
        log_error "Kind 集群创建失败"
        exit 1
    fi
    
    # 等待集群就绪
    log_info "等待集群就绪..."
    kubectl wait --for=condition=Ready nodes --all --timeout=300s
    
    # 清理临时配置文件
    rm -f "$kind_config"
}

# 设置 kubectl 上下文
setup_kubectl_context() {
    local cluster_name=${1:-"etcd-operator-test"}
    
    log_info "设置 kubectl 上下文..."
    
    # 切换到 Kind 集群上下文
    kubectl config use-context "kind-$cluster_name"
    
    # 验证连接
    if kubectl cluster-info &> /dev/null; then
        log_success "kubectl 上下文设置成功"
        kubectl get nodes
    else
        log_error "无法连接到 Kubernetes 集群"
        exit 1
    fi
}

# 安装测试依赖
install_test_dependencies() {
    log_info "安装测试依赖..."
    
    # 进入项目目录
    cd "$(dirname "$0")/../.."
    
    # 下载 Go 模块依赖
    go mod download
    
    # 安装 testify
    go get github.com/stretchr/testify@v1.9.0
    
    # 整理依赖
    go mod tidy
    
    log_success "测试依赖安装完成"
}

# 构建项目
build_project() {
    log_info "构建项目..."
    
    cd "$(dirname "$0")/../.."
    
    # 生成代码
    make generate
    
    # 生成 manifests
    make manifests
    
    # 构建项目
    make build
    
    log_success "项目构建完成"
}

# 主函数
main() {
    local cluster_name=${1:-"etcd-operator-test"}
    
    log_info "开始设置 ETCD Operator 测试环境..."
    log_info "集群名称: $cluster_name"
    
    check_prerequisites
    check_orbstack
    setup_go_env
    install_test_tools
    create_kind_cluster "$cluster_name"
    setup_kubectl_context "$cluster_name"
    install_test_dependencies
    build_project
    
    log_success "测试环境设置完成！"
    log_info "可以使用以下命令运行测试:"
    log_info "  make test                    # 运行单元测试"
    log_info "  make test-integration        # 运行集成测试"
    log_info "  make test-e2e               # 运行端到端测试"
    log_info "  scripts/test/test-all.sh    # 运行所有测试"
}

# 脚本入口
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
