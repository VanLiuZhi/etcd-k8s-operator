#!/bin/bash

# ETCD Operator 完整测试执行脚本

set -euo pipefail

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
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

log_step() {
    echo -e "${PURPLE}[STEP]${NC} $1"
}

# 配置变量
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CLUSTER_NAME="etcd-operator-test"
START_TIME=$(date +%s)

# 测试结果跟踪 (兼容 bash 3.2)
TEST_RESULTS_SETUP=""
TEST_RESULTS_UNIT=""
TEST_RESULTS_INTEGRATION=""
TEST_RESULTS_E2E=""
TEST_PHASES=("setup" "unit" "integration" "e2e")

# 显示横幅
show_banner() {
    echo -e "${PURPLE}"
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║                    ETCD Operator 测试套件                    ║"
    echo "║                                                              ║"
    echo "║  完整的测试流程：环境设置 → 单元测试 → 集成测试 → 端到端测试    ║"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

# 显示测试配置
show_test_configuration() {
    log_info "测试配置:"
    echo "  项目根目录: $PROJECT_ROOT"
    echo "  脚本目录: $SCRIPT_DIR"
    echo "  集群名称: $CLUSTER_NAME"
    echo "  开始时间: $(date)"
    echo ""
}

# 检查脚本权限
check_script_permissions() {
    log_info "检查脚本权限..."
    
    local scripts=(
        "$SCRIPT_DIR/setup-test-env.sh"
        "$SCRIPT_DIR/run-unit-tests.sh"
        "$SCRIPT_DIR/run-integration-tests.sh"
        "$SCRIPT_DIR/run-e2e-tests.sh"
    )
    
    for script in "${scripts[@]}"; do
        if [[ ! -f "$script" ]]; then
            log_error "脚本不存在: $script"
            exit 1
        fi
        
        if [[ ! -x "$script" ]]; then
            log_info "设置脚本执行权限: $script"
            chmod +x "$script"
        fi
    done
    
    log_success "脚本权限检查完成"
}

# 设置测试结果
set_test_result() {
    local phase=$1
    local result=$2

    case $phase in
        "setup")
            TEST_RESULTS_SETUP="$result"
            ;;
        "unit")
            TEST_RESULTS_UNIT="$result"
            ;;
        "integration")
            TEST_RESULTS_INTEGRATION="$result"
            ;;
        "e2e")
            TEST_RESULTS_E2E="$result"
            ;;
    esac
}

# 获取测试结果
get_test_result() {
    local phase=$1

    case $phase in
        "setup")
            echo "$TEST_RESULTS_SETUP"
            ;;
        "unit")
            echo "$TEST_RESULTS_UNIT"
            ;;
        "integration")
            echo "$TEST_RESULTS_INTEGRATION"
            ;;
        "e2e")
            echo "$TEST_RESULTS_E2E"
            ;;
    esac
}

# 运行测试阶段
run_test_phase() {
    local phase=$1
    local description=$2
    local script=$3
    shift 3
    local args=("$@")

    log_step "阶段 $phase: $description"
    echo "----------------------------------------"

    local phase_start=$(date +%s)

    if "$script" "${args[@]}"; then
        local phase_end=$(date +%s)
        local phase_duration=$((phase_end - phase_start))
        set_test_result "$phase" "SUCCESS:${phase_duration}s"
        log_success "阶段 $phase 完成 (耗时: ${phase_duration}s)"
    else
        local phase_end=$(date +%s)
        local phase_duration=$((phase_end - phase_start))
        set_test_result "$phase" "FAILED:${phase_duration}s"
        log_error "阶段 $phase 失败 (耗时: ${phase_duration}s)"
        return 1
    fi

    echo ""
}

# 运行环境设置
run_setup_phase() {
    run_test_phase "setup" "环境设置" "$SCRIPT_DIR/setup-test-env.sh" "$CLUSTER_NAME"
}

# 运行单元测试
run_unit_tests_phase() {
    local args=()
    
    # 根据参数决定是否跳过某些检查
    if [[ "${SKIP_LINT:-false}" == "true" ]]; then
        args+=("--skip-lint")
    fi
    
    if [[ "${SKIP_BENCH:-false}" == "true" ]]; then
        args+=("--skip-bench")
    fi
    
    if [[ -n "${COVERAGE_THRESHOLD:-}" ]]; then
        args+=("--coverage-threshold" "$COVERAGE_THRESHOLD")
    fi
    
    run_test_phase "unit" "单元测试" "$SCRIPT_DIR/run-unit-tests.sh" "${args[@]}"
}

# 运行集成测试
run_integration_tests_phase() {
    local args=()
    
    if [[ "${SKIP_BUILD:-false}" == "true" ]]; then
        args+=("--skip-build")
    fi
    
    if [[ "${SKIP_CONTROLLER:-false}" == "true" ]]; then
        args+=("--skip-controller")
    fi
    
    run_test_phase "integration" "集成测试" "$SCRIPT_DIR/run-integration-tests.sh" "${args[@]}"
}

# 运行端到端测试
run_e2e_tests_phase() {
    run_test_phase "e2e" "端到端测试" "$SCRIPT_DIR/run-e2e-tests.sh"
}

# 显示测试结果总结
show_test_summary() {
    local end_time=$(date +%s)
    local total_duration=$((end_time - START_TIME))
    
    echo -e "${PURPLE}"
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║                        测试结果总结                          ║"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    
    log_info "测试结果:"

    local all_passed=true
    for phase in "${TEST_PHASES[@]}"; do
        local result=$(get_test_result "$phase")
        if [[ -n "$result" ]]; then
            local status="${result%%:*}"
            local duration="${result##*:}"

            if [[ "$status" == "SUCCESS" ]]; then
                echo -e "  ${GREEN}✓${NC} $phase: 成功 ($duration)"
            else
                echo -e "  ${RED}✗${NC} $phase: 失败 ($duration)"
                all_passed=false
            fi
        else
            echo -e "  ${YELLOW}○${NC} $phase: 跳过"
        fi
    done
    
    echo ""
    log_info "总体统计:"
    echo "  总耗时: ${total_duration}s ($(date -u -d @${total_duration} +%H:%M:%S))"
    echo "  开始时间: $(date -d @${START_TIME})"
    echo "  结束时间: $(date -d @${end_time})"
    
    if [[ "$all_passed" == true ]]; then
        echo -e "\n${GREEN}🎉 所有测试通过！${NC}"
        return 0
    else
        echo -e "\n${RED}❌ 部分测试失败！${NC}"
        return 1
    fi
}

# 清理函数
cleanup_on_exit() {
    local exit_code=$?
    
    if [[ $exit_code -ne 0 ]]; then
        log_error "测试过程中发生错误，正在清理..."
    fi
    
    # 显示测试结果（即使失败也显示）
    show_test_summary || true
    
    # 如果设置了清理标志，清理测试环境
    if [[ "${CLEANUP_ON_EXIT:-true}" == "true" ]]; then
        log_info "清理测试环境..."
        "$SCRIPT_DIR/cleanup-test-env.sh" "$CLUSTER_NAME" || true
    fi
    
    exit $exit_code
}

# 显示帮助信息
show_help() {
    echo "ETCD Operator 完整测试套件"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  --skip-setup              跳过环境设置"
    echo "  --skip-unit               跳过单元测试"
    echo "  --skip-integration        跳过集成测试"
    echo "  --skip-e2e                跳过端到端测试"
    echo "  --skip-lint               跳过代码检查"
    echo "  --skip-bench              跳过基准测试"
    echo "  --skip-build              跳过镜像构建"
    echo "  --skip-controller         跳过控制器测试"
    echo "  --coverage-threshold N    设置覆盖率阈值 (默认: 50)"
    echo "  --cluster-name NAME       设置集群名称 (默认: etcd-operator-test)"
    echo "  --no-cleanup              测试完成后不清理环境"
    echo "  --fast                    快速模式 (跳过非关键测试)"
    echo "  -h, --help                显示帮助信息"
    echo ""
    echo "环境变量:"
    echo "  SKIP_LINT=true            跳过代码检查"
    echo "  SKIP_BENCH=true           跳过基准测试"
    echo "  SKIP_BUILD=true           跳过镜像构建"
    echo "  SKIP_CONTROLLER=true      跳过控制器测试"
    echo "  COVERAGE_THRESHOLD=N      设置覆盖率阈值"
    echo "  CLEANUP_ON_EXIT=false     禁用退出时清理"
    echo ""
    echo "示例:"
    echo "  $0                        # 运行所有测试"
    echo "  $0 --fast                 # 快速测试"
    echo "  $0 --skip-e2e             # 跳过端到端测试"
    echo "  $0 --coverage-threshold 80 # 设置80%覆盖率要求"
}

# 主函数
main() {
    local skip_setup=false
    local skip_unit=false
    local skip_integration=false
    local skip_e2e=false
    local fast_mode=false
    
    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            --skip-setup)
                skip_setup=true
                shift
                ;;
            --skip-unit)
                skip_unit=true
                shift
                ;;
            --skip-integration)
                skip_integration=true
                shift
                ;;
            --skip-e2e)
                skip_e2e=true
                shift
                ;;
            --skip-lint)
                export SKIP_LINT=true
                shift
                ;;
            --skip-bench)
                export SKIP_BENCH=true
                shift
                ;;
            --skip-build)
                export SKIP_BUILD=true
                shift
                ;;
            --skip-controller)
                export SKIP_CONTROLLER=true
                shift
                ;;
            --coverage-threshold)
                export COVERAGE_THRESHOLD="$2"
                shift 2
                ;;
            --cluster-name)
                CLUSTER_NAME="$2"
                shift 2
                ;;
            --no-cleanup)
                export CLEANUP_ON_EXIT=false
                shift
                ;;
            --fast)
                fast_mode=true
                export SKIP_LINT=true
                export SKIP_BENCH=true
                export SKIP_BUILD=true
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log_error "未知选项: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    # 设置退出清理
    trap cleanup_on_exit EXIT
    
    # 显示横幅和配置
    show_banner
    show_test_configuration
    
    # 检查脚本权限
    check_script_permissions
    
    # 运行测试阶段
    if [[ "$skip_setup" != true ]]; then
        run_setup_phase
    else
        log_warning "跳过环境设置阶段"
    fi
    
    if [[ "$skip_unit" != true ]]; then
        run_unit_tests_phase
    else
        log_warning "跳过单元测试阶段"
    fi
    
    if [[ "$skip_integration" != true ]]; then
        run_integration_tests_phase
    else
        log_warning "跳过集成测试阶段"
    fi
    
    if [[ "$skip_e2e" != true ]]; then
        run_e2e_tests_phase
    else
        log_warning "跳过端到端测试阶段"
    fi
    
    log_success "所有测试阶段完成！"
}

# 脚本入口
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
