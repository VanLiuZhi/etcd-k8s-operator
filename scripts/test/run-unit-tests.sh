#!/bin/bash

# ETCD Operator 单元测试执行脚本

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
COVERAGE_DIR="$PROJECT_ROOT/coverage"
COVERAGE_FILE="$COVERAGE_DIR/coverage.out"
COVERAGE_HTML="$COVERAGE_DIR/coverage.html"

# 创建覆盖率目录
setup_coverage_dir() {
    log_info "设置覆盖率目录..."
    mkdir -p "$COVERAGE_DIR"
}

# 运行代码格式检查
run_fmt_check() {
    log_info "运行代码格式检查..."
    
    cd "$PROJECT_ROOT"
    
    # 检查代码格式
    local unformatted_files=$(gofmt -l . | grep -v vendor/ || true)
    if [[ -n "$unformatted_files" ]]; then
        log_error "以下文件格式不正确:"
        echo "$unformatted_files"
        log_info "运行 'make fmt' 修复格式问题"
        return 1
    fi
    
    log_success "代码格式检查通过"
}

# 运行静态分析
run_lint() {
    log_info "运行静态分析..."
    
    cd "$PROJECT_ROOT"
    
    # 检查是否安装了 golangci-lint
    if ! command -v golangci-lint &> /dev/null; then
        log_warning "golangci-lint 未安装，跳过静态分析"
        return 0
    fi
    
    # 运行 golangci-lint
    if golangci-lint run --timeout=5m; then
        log_success "静态分析通过"
    else
        log_error "静态分析发现问题"
        return 1
    fi
}

# 运行单元测试
run_unit_tests() {
    log_info "运行单元测试..."
    
    cd "$PROJECT_ROOT"
    
    # 查找所有测试包
    local test_packages=$(go list ./... | grep -v /test/integration | grep -v /test/e2e)
    
    if [[ -z "$test_packages" ]]; then
        log_warning "未找到单元测试包"
        return 0
    fi
    
    log_info "测试包:"
    echo "$test_packages" | sed 's/^/  /'
    
    # 运行测试并生成覆盖率报告
    local test_args=(
        "-v"                           # 详细输出
        "-race"                        # 竞态检测
        "-coverprofile=$COVERAGE_FILE" # 覆盖率文件
        "-covermode=atomic"            # 覆盖率模式
        "-timeout=10m"                 # 测试超时
    )
    
    if go test "${test_args[@]}" $test_packages; then
        log_success "单元测试通过"
    else
        log_error "单元测试失败"
        return 1
    fi
}

# 生成覆盖率报告
generate_coverage_report() {
    log_info "生成覆盖率报告..."
    
    if [[ ! -f "$COVERAGE_FILE" ]]; then
        log_warning "覆盖率文件不存在，跳过报告生成"
        return 0
    fi
    
    cd "$PROJECT_ROOT"
    
    # 生成 HTML 覆盖率报告
    go tool cover -html="$COVERAGE_FILE" -o "$COVERAGE_HTML"
    
    # 显示覆盖率统计
    local coverage_percent=$(go tool cover -func="$COVERAGE_FILE" | tail -1 | awk '{print $3}')
    log_info "总体覆盖率: $coverage_percent"
    
    # 显示详细覆盖率
    log_info "详细覆盖率报告:"
    go tool cover -func="$COVERAGE_FILE" | grep -v "total:" | while read line; do
        echo "  $line"
    done
    
    log_success "覆盖率报告生成完成: $COVERAGE_HTML"
}

# 运行基准测试
run_benchmarks() {
    log_info "运行基准测试..."
    
    cd "$PROJECT_ROOT"
    
    # 查找基准测试
    local bench_packages=$(go list ./... | xargs -I {} sh -c 'go test -list "Benchmark.*" {} 2>/dev/null | grep -q "Benchmark" && echo {}' || true)
    
    if [[ -z "$bench_packages" ]]; then
        log_info "未找到基准测试，跳过"
        return 0
    fi
    
    log_info "基准测试包:"
    echo "$bench_packages" | sed 's/^/  /'
    
    # 运行基准测试
    local bench_args=(
        "-bench=."
        "-benchmem"
        "-timeout=10m"
    )
    
    if go test "${bench_args[@]}" $bench_packages; then
        log_success "基准测试完成"
    else
        log_warning "基准测试出现问题"
    fi
}

# 检查测试覆盖率阈值
check_coverage_threshold() {
    local threshold=${1:-50}
    
    log_info "检查覆盖率阈值 (>= ${threshold}%)..."
    
    if [[ ! -f "$COVERAGE_FILE" ]]; then
        log_warning "覆盖率文件不存在，跳过阈值检查"
        return 0
    fi
    
    local coverage_percent=$(go tool cover -func="$COVERAGE_FILE" | tail -1 | awk '{print $3}' | sed 's/%//')
    
    if (( $(echo "$coverage_percent >= $threshold" | bc -l) )); then
        log_success "覆盖率 ${coverage_percent}% 达到阈值要求"
    else
        log_error "覆盖率 ${coverage_percent}% 未达到阈值要求 (>= ${threshold}%)"
        return 1
    fi
}

# 清理测试文件
cleanup_test_files() {
    log_info "清理测试文件..."
    
    cd "$PROJECT_ROOT"
    
    # 清理测试缓存
    go clean -testcache
    
    # 清理临时文件
    find . -name "*.test" -delete 2>/dev/null || true
    find . -name "*.prof" -delete 2>/dev/null || true
    
    log_success "测试文件清理完成"
}

# 显示测试总结
show_test_summary() {
    log_info "测试总结:"
    echo "  项目根目录: $PROJECT_ROOT"
    echo "  覆盖率文件: $COVERAGE_FILE"
    echo "  覆盖率报告: $COVERAGE_HTML"
    
    if [[ -f "$COVERAGE_FILE" ]]; then
        local coverage_percent=$(go tool cover -func="$COVERAGE_FILE" | tail -1 | awk '{print $3}')
        echo "  总体覆盖率: $coverage_percent"
    fi
}

# 主函数
main() {
    local skip_lint=false
    local skip_bench=false
    local coverage_threshold=50
    local cleanup=false
    
    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-lint)
                skip_lint=true
                shift
                ;;
            --skip-bench)
                skip_bench=true
                shift
                ;;
            --coverage-threshold)
                coverage_threshold="$2"
                shift 2
                ;;
            --cleanup)
                cleanup=true
                shift
                ;;
            -h|--help)
                echo "用法: $0 [选项]"
                echo "选项:"
                echo "  --skip-lint              跳过静态分析"
                echo "  --skip-bench             跳过基准测试"
                echo "  --coverage-threshold N   设置覆盖率阈值 (默认: 50)"
                echo "  --cleanup                测试后清理文件"
                echo "  -h, --help               显示帮助信息"
                exit 0
                ;;
            *)
                log_error "未知选项: $1"
                exit 1
                ;;
        esac
    done
    
    log_info "开始运行单元测试..."
    
    setup_coverage_dir
    
    # 运行代码质量检查
    run_fmt_check
    
    if [[ "$skip_lint" != true ]]; then
        run_lint
    fi
    
    # 运行单元测试
    run_unit_tests
    
    # 生成覆盖率报告
    generate_coverage_report
    
    # 检查覆盖率阈值
    check_coverage_threshold "$coverage_threshold"
    
    # 运行基准测试
    if [[ "$skip_bench" != true ]]; then
        run_benchmarks
    fi
    
    # 清理文件
    if [[ "$cleanup" == true ]]; then
        cleanup_test_files
    fi
    
    # 显示总结
    show_test_summary
    
    log_success "单元测试完成！"
}

# 脚本入口
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
