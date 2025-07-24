# ETCD Operator 测试指南

## 📋 概述

本文档详细介绍了 ETCD Kubernetes Operator 的完整测试策略、测试环境设置和测试执行过程。我们的测试架构采用分层设计，确保代码质量和功能可靠性。

## 🏗️ 测试架构

### 测试金字塔

```
    ┌─────────────────┐
    │   E2E Tests     │  ← 端到端测试 (少量，高价值)
    │   (5-10个)      │    真实环境，完整流程
    ├─────────────────┤
    │ Integration     │  ← 集成测试 (中等数量)
    │ Tests (20-30个) │    组件协作，API 交互
    ├─────────────────┤
    │  Unit Tests     │  ← 单元测试 (大量，快速)
    │   (100+个)      │    函数级别，逻辑验证
    └─────────────────┘
```

### 测试分类

| 测试类型 | 目的 | 工具 | 执行时间 | 覆盖范围 |
|---------|------|------|---------|----------|
| **单元测试** | 验证函数和方法逻辑 | testify + Go test | < 30秒 | 代码逻辑 |
| **集成测试** | 验证组件间协作 | Ginkgo + envtest | < 5分钟 | 控制器交互 |
| **端到端测试** | 验证完整用户场景 | Ginkgo + Kind | < 15分钟 | 完整流程 |

## 🛠️ 环境要求

### 系统要求

- **操作系统**: macOS (已针对 OrbStack 优化)
- **Go**: 1.22.0+
- **Docker**: 通过 OrbStack 提供
- **Kubernetes**: Kind 集群 (v1.28.0)
- **内存**: 至少 8GB 可用内存
- **磁盘**: 至少 10GB 可用空间

### 必需工具

```bash
# 核心工具
go version          # Go 1.22.0+
docker --version    # Docker (OrbStack)
kubectl version     # Kubernetes CLI
kind version        # Kind for local clusters

# 测试工具
ginkgo version      # BDD 测试框架
golangci-lint --version  # 代码检查工具
```

## 🚀 快速开始

### 1. 环境设置

```bash
# 设置完整测试环境
make test-setup

# 或者手动设置
scripts/test/setup-test-env.sh
```

### 2. 运行测试

```bash
# 运行所有测试
make test-all

# 运行特定类型的测试
make test-unit          # 单元测试
make test-integration   # 集成测试
make test-e2e          # 端到端测试

# 快速测试模式
make test-fast
```

### 3. 清理环境

```bash
# 清理测试环境
make test-cleanup

# 或者手动清理
scripts/test/cleanup-test-env.sh
```

## 📝 详细测试说明

### 单元测试

**目的**: 验证单个函数和方法的逻辑正确性

**特点**:
- 快速执行 (< 30秒)
- 无外部依赖
- 高覆盖率要求 (≥ 50%)
- 使用 Mock 对象

**执行**:
```bash
# 运行单元测试
scripts/test/run-unit-tests.sh

# 带选项运行
scripts/test/run-unit-tests.sh --coverage-threshold 80 --skip-bench
```

**测试内容**:
- 资源构建器 (`pkg/k8s/resources_test.go`)
- 工具函数 (`pkg/utils/labels_test.go`)
- 控制器逻辑 (`internal/controller/etcdcluster_controller_unit_test.go`)

### 集成测试

**目的**: 验证组件间的协作和 API 交互

**特点**:
- 使用 envtest 环境
- 真实的 Kubernetes API
- 控制器完整运行
- 中等执行时间 (< 5分钟)

**执行**:
```bash
# 运行集成测试
scripts/test/run-integration-tests.sh

# 跳过镜像构建
scripts/test/run-integration-tests.sh --skip-build
```

**测试内容**:
- EtcdCluster 创建和删除
- 资源管理 (StatefulSet, Service, ConfigMap)
- 状态转换和条件管理
- 基础扩缩容功能

### 端到端测试

**目的**: 验证完整的用户场景和系统行为

**特点**:
- 真实 Kind 集群
- 完整 Operator 部署
- 端到端用户流程
- 较长执行时间 (< 15分钟)

**执行**:
```bash
# 运行端到端测试
scripts/test/run-e2e-tests.sh

# 保留环境不清理
scripts/test/run-e2e-tests.sh --no-cleanup
```

**测试场景**:
1. **基础集群生命周期**: 创建 → 运行 → 删除
2. **集群扩缩容**: 3节点 → 5节点 → 3节点
3. **故障恢复**: Pod 删除后自动恢复
4. **数据持久化**: 重启后数据保持

## 🔧 测试配置

### 配置文件

测试配置存储在 `.testconfig` 文件中：

```bash
# 集群配置
CLUSTER_NAME=etcd-operator-test
TEST_NAMESPACE=etcd-operator-test

# 超时配置
UNIT_TEST_TIMEOUT=600
INTEGRATION_TEST_TIMEOUT=1800
E2E_TEST_TIMEOUT=3600

# 覆盖率要求
COVERAGE_THRESHOLD=50
```

### 环境变量

```bash
# 跳过特定检查
export SKIP_LINT=true
export SKIP_BENCH=true
export SKIP_BUILD=true

# 设置覆盖率阈值
export COVERAGE_THRESHOLD=80

# 禁用清理
export CLEANUP_ON_EXIT=false
```

## 📊 测试报告

### 覆盖率报告

```bash
# 生成覆盖率报告
make test-unit

# 查看报告
open coverage/coverage.html
```

### 测试结果

测试完成后会显示详细的结果总结：

```
╔══════════════════════════════════════════════════════════════╗
║                        测试结果总结                          ║
╚══════════════════════════════════════════════════════════════╝

测试结果:
  ✓ setup: 成功 (45s)
  ✓ unit: 成功 (23s)
  ✓ integration: 成功 (156s)
  ✓ e2e: 成功 (387s)

总体统计:
  总耗时: 611s (00:10:11)
  覆盖率: 67.8%

🎉 所有测试通过！
```

## 🐛 故障排除

### 常见问题

#### 1. Kind 集群创建失败

```bash
# 检查 Docker 状态
docker info

# 重新创建集群
kind delete cluster --name etcd-operator-test
scripts/test/setup-test-env.sh
```

#### 2. 测试超时

```bash
# 增加超时时间
export UNIT_TEST_TIMEOUT=1200
export INTEGRATION_TEST_TIMEOUT=3600
```

#### 3. 覆盖率不足

```bash
# 降低覆盖率要求
scripts/test/run-unit-tests.sh --coverage-threshold 30
```

#### 4. 资源不足

```bash
# 清理系统资源
docker system prune -f
scripts/test/cleanup-test-env.sh --force
```

### 调试技巧

#### 1. 详细日志

```bash
# 启用详细日志
export LOG_LEVEL=debug
export TEST_LOG_LEVEL=debug
```

#### 2. 保留测试环境

```bash
# 测试失败后保留环境
scripts/test/test-all.sh --no-cleanup
```

#### 3. 单独运行测试

```bash
# 只运行特定测试
go test -v ./pkg/k8s -run TestStatefulSetBuilder
```

## 📚 最佳实践

### 编写测试

1. **单元测试**:
   - 使用 testify/suite 组织测试
   - Mock 外部依赖
   - 测试边界条件
   - 保持测试独立

2. **集成测试**:
   - 使用真实的 Kubernetes API
   - 测试完整的控制器逻辑
   - 验证资源创建和状态
   - 清理测试资源

3. **端到端测试**:
   - 模拟真实用户场景
   - 测试完整的生命周期
   - 包含故障场景
   - 验证数据持久化

### 性能优化

1. **并行执行**:
   ```bash
   # 并行运行测试
   go test -parallel 4 ./...
   ginkgo -p --nodes 4
   ```

2. **缓存利用**:
   ```bash
   # 利用 Go 模块缓存
   export GOPROXY=https://goproxy.cn,direct
   ```

3. **资源限制**:
   ```bash
   # 限制测试资源使用
   export TEST_MEMORY_LIMIT=2Gi
   export TEST_CPU_LIMIT=1000m
   ```

## 🔄 CI/CD 集成

### GitHub Actions

```yaml
name: Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v3
      with:
        go-version: 1.22
    - name: Run tests
      run: make test-all
```

### 本地 Pre-commit

```bash
# 设置 pre-commit 钩子
cp scripts/ci/pre-commit.sh .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

## 📈 测试指标

### 目标指标

- **单元测试覆盖率**: ≥ 80%
- **集成测试通过率**: 100%
- **端到端测试通过率**: 100%
- **测试执行时间**: < 15分钟 (完整套件)

### 监控指标

- 测试执行时间趋势
- 覆盖率变化趋势
- 失败率统计
- 资源使用情况

---

**文档版本**: v1.0  
**最后更新**: 2025-07-21  
**维护者**: ETCD Operator 开发团队
