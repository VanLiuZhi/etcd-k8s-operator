#!/bin/bash

# ETCD Operator 测试环境清理脚本

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

# 清理 Kind 集群
cleanup_kind_cluster() {
    local cluster_name=${1:-"etcd-operator-test"}
    
    log_info "清理 Kind 集群: $cluster_name"
    
    # 检查集群是否存在
    if kind get clusters | grep -q "^$cluster_name$"; then
        log_info "删除 Kind 集群: $cluster_name"
        if kind delete cluster --name "$cluster_name"; then
            log_success "Kind 集群删除成功: $cluster_name"
        else
            log_error "Kind 集群删除失败: $cluster_name"
            return 1
        fi
    else
        log_info "Kind 集群不存在，跳过删除: $cluster_name"
    fi
}

# 清理 Docker 镜像
cleanup_docker_images() {
    log_info "清理 Docker 镜像..."
    
    # 清理测试镜像
    local test_images=(
        "etcd-operator:test"
        "etcd-operator:e2e"
        "etcd-operator:latest"
    )
    
    for image in "${test_images[@]}"; do
        if docker images --format "table {{.Repository}}:{{.Tag}}" | grep -q "^$image$"; then
            log_info "删除镜像: $image"
            docker rmi "$image" || log_warning "无法删除镜像: $image"
        fi
    done
    
    # 清理悬空镜像
    local dangling_images=$(docker images -f "dangling=true" -q)
    if [[ -n "$dangling_images" ]]; then
        log_info "清理悬空镜像..."
        docker rmi $dangling_images || log_warning "清理悬空镜像时出现问题"
    fi
    
    log_success "Docker 镜像清理完成"
}

# 清理项目构建文件
cleanup_build_files() {
    log_info "清理项目构建文件..."
    
    cd "$(dirname "$0")/../.."
    
    # 清理构建产物
    local build_files=(
        "bin/manager"
        "bin/k8s"
        "coverage/coverage.out"
        "coverage/coverage.html"
        "dist/"
    )
    
    for file in "${build_files[@]}"; do
        if [[ -e "$file" ]]; then
            log_info "删除: $file"
            rm -rf "$file"
        fi
    done
    
    # 清理 Go 缓存
    log_info "清理 Go 缓存..."
    go clean -cache -testcache -modcache || log_warning "清理 Go 缓存时出现问题"
    
    log_success "构建文件清理完成"
}

# 清理临时文件
cleanup_temp_files() {
    log_info "清理临时文件..."
    
    # 清理 /tmp 中的测试文件
    local temp_patterns=(
        "/tmp/kind-config*.yaml"
        "/tmp/e2e-*.yaml"
        "/tmp/test-*.yaml"
        "/tmp/operator-*.yaml"
    )
    
    for pattern in "${temp_patterns[@]}"; do
        local files=$(ls $pattern 2>/dev/null || true)
        if [[ -n "$files" ]]; then
            log_info "删除临时文件: $pattern"
            rm -f $files
        fi
    done
    
    log_success "临时文件清理完成"
}

# 清理 kubectl 上下文
cleanup_kubectl_contexts() {
    local cluster_name=${1:-"etcd-operator-test"}
    
    log_info "清理 kubectl 上下文..."
    
    local context_name="kind-$cluster_name"
    
    # 检查上下文是否存在
    if kubectl config get-contexts -o name | grep -q "^$context_name$"; then
        log_info "删除 kubectl 上下文: $context_name"
        kubectl config delete-context "$context_name" || log_warning "无法删除上下文: $context_name"
    fi
    
    # 检查集群配置是否存在
    if kubectl config get-clusters | grep -q "^$context_name$"; then
        log_info "删除集群配置: $context_name"
        kubectl config delete-cluster "$context_name" || log_warning "无法删除集群配置: $context_name"
    fi
    
    # 检查用户配置是否存在
    if kubectl config get-users | grep -q "^$context_name$"; then
        log_info "删除用户配置: $context_name"
        kubectl config delete-user "$context_name" || log_warning "无法删除用户配置: $context_name"
    fi
    
    log_success "kubectl 上下文清理完成"
}

# 清理系统资源
cleanup_system_resources() {
    log_info "清理系统资源..."
    
    # 清理孤立的容器
    local orphaned_containers=$(docker ps -a --filter "label=io.x-k8s.kind.cluster" --format "{{.ID}}" 2>/dev/null || true)
    if [[ -n "$orphaned_containers" ]]; then
        log_info "清理孤立的容器..."
        docker rm -f $orphaned_containers || log_warning "清理孤立容器时出现问题"
    fi
    
    # 清理孤立的网络
    local orphaned_networks=$(docker network ls --filter "label=io.x-k8s.kind.cluster" --format "{{.ID}}" 2>/dev/null || true)
    if [[ -n "$orphaned_networks" ]]; then
        log_info "清理孤立的网络..."
        docker network rm $orphaned_networks || log_warning "清理孤立网络时出现问题"
    fi
    
    # 清理孤立的卷
    local orphaned_volumes=$(docker volume ls --filter "label=io.x-k8s.kind.cluster" --format "{{.Name}}" 2>/dev/null || true)
    if [[ -n "$orphaned_volumes" ]]; then
        log_info "清理孤立的卷..."
        docker volume rm $orphaned_volumes || log_warning "清理孤立卷时出现问题"
    fi
    
    log_success "系统资源清理完成"
}

# 验证清理结果
verify_cleanup() {
    local cluster_name=${1:-"etcd-operator-test"}
    
    log_info "验证清理结果..."
    
    local issues=()
    
    # 检查 Kind 集群
    if kind get clusters | grep -q "^$cluster_name$"; then
        issues+=("Kind 集群仍然存在: $cluster_name")
    fi
    
    # 检查 kubectl 上下文
    if kubectl config get-contexts -o name | grep -q "^kind-$cluster_name$"; then
        issues+=("kubectl 上下文仍然存在: kind-$cluster_name")
    fi
    
    # 检查测试镜像
    if docker images --format "table {{.Repository}}:{{.Tag}}" | grep -q "etcd-operator:test"; then
        issues+=("测试镜像仍然存在")
    fi
    
    if [[ ${#issues[@]} -eq 0 ]]; then
        log_success "清理验证通过"
        return 0
    else
        log_warning "清理验证发现问题:"
        for issue in "${issues[@]}"; do
            echo "  - $issue"
        done
        return 1
    fi
}

# 显示清理总结
show_cleanup_summary() {
    log_info "清理总结:"
    echo "  清理时间: $(date)"
    echo "  清理项目:"
    echo "    - Kind 集群"
    echo "    - Docker 镜像"
    echo "    - 构建文件"
    echo "    - 临时文件"
    echo "    - kubectl 上下文"
    echo "    - 系统资源"
}

# 主函数
main() {
    local cluster_name="etcd-operator-test"
    local skip_docker=false
    local skip_build=false
    local skip_system=false
    local force=false
    
    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --cluster-name)
                cluster_name="$2"
                shift 2
                ;;
            --skip-docker)
                skip_docker=true
                shift
                ;;
            --skip-build)
                skip_build=true
                shift
                ;;
            --skip-system)
                skip_system=true
                shift
                ;;
            --force)
                force=true
                shift
                ;;
            -h|--help)
                echo "用法: $0 [选项] [集群名称]"
                echo "选项:"
                echo "  --cluster-name NAME   指定集群名称 (默认: etcd-operator-test)"
                echo "  --skip-docker         跳过 Docker 镜像清理"
                echo "  --skip-build          跳过构建文件清理"
                echo "  --skip-system         跳过系统资源清理"
                echo "  --force               强制清理，不询问确认"
                echo "  -h, --help            显示帮助信息"
                exit 0
                ;;
            *)
                # 位置参数作为集群名称
                cluster_name="$1"
                shift
                ;;
        esac
    done
    
    log_info "开始清理 ETCD Operator 测试环境..."
    log_info "集群名称: $cluster_name"
    
    # 确认清理操作
    if [[ "$force" != true ]]; then
        echo -e "${YELLOW}警告: 此操作将清理所有测试相关的资源${NC}"
        echo "包括: Kind 集群、Docker 镜像、构建文件、临时文件等"
        read -p "确认继续? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "清理操作已取消"
            exit 0
        fi
    fi
    
    # 执行清理操作
    cleanup_kind_cluster "$cluster_name"
    cleanup_kubectl_contexts "$cluster_name"
    cleanup_temp_files
    
    if [[ "$skip_docker" != true ]]; then
        cleanup_docker_images
    fi
    
    if [[ "$skip_build" != true ]]; then
        cleanup_build_files
    fi
    
    if [[ "$skip_system" != true ]]; then
        cleanup_system_resources
    fi
    
    # 验证清理结果
    verify_cleanup "$cluster_name"
    
    # 显示总结
    show_cleanup_summary
    
    log_success "测试环境清理完成！"
}

# 脚本入口
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
