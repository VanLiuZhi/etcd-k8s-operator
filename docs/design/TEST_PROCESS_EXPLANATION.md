# ETCD Operator 测试过程详细说明

## 📋 概述

本文档详细说明了 ETCD Kubernetes Operator 的完整测试过程，包括每个测试阶段的具体步骤、预期结果和故障排除方法。在开始测试之前，请仔细阅读本文档以了解整个测试流程。

## 🏗️ 测试架构回顾

我们的测试系统采用四层架构：

```
┌─────────────────────────────────────────────────────────────┐
│                    完整测试流程                              │
├─────────────────────────────────────────────────────────────┤
│  1. 环境设置    │  2. 单元测试    │  3. 集成测试    │  4. E2E测试  │
│  (setup)       │  (unit)        │  (integration) │  (e2e)     │
│                │                │                │            │
│  • 工具检查     │  • 代码格式     │  • envtest环境  │  • Kind集群 │
│  • Kind集群     │  • 静态分析     │  • 控制器测试   │  • 真实场景 │
│  • 依赖安装     │  • 单元测试     │  • API交互      │  • 完整流程 │
│  • 项目构建     │  • 覆盖率报告   │  • 资源管理     │  • 故障恢复 │
└─────────────────────────────────────────────────────────────┘
```

## 🚀 测试执行步骤

### 阶段 1: 环境设置 (setup)

**目的**: 准备完整的测试环境，确保所有依赖工具和集群就绪。

**执行命令**:
```bash
scripts/test/setup-test-env.sh
```

**详细步骤**:

1. **工具检查** (30秒)
   ```bash
   # 检查必需工具
   ✓ Go 1.22.3
   ✓ Docker (OrbStack)
   ✓ kubectl
   ✓ kind
   ```

2. **OrbStack 状态验证** (10秒)
   ```bash
   # 验证 Docker 环境
   docker info | grep orbstack
   ```

3. **Go 环境配置** (15秒)
   ```bash
   # 设置代理和缓存
   export GOPROXY=https://goproxy.cn,direct
   ```

4. **测试工具安装** (60秒)
   ```bash
   # 安装测试依赖
   go install github.com/onsi/ginkgo/v2/ginkgo@latest
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   ```

5. **Kind 集群创建** (120秒)
   ```bash
   # 创建 3 节点集群
   kind create cluster --name etcd-operator-test
   ```

6. **项目构建** (45秒)
   ```bash
   # 生成代码和构建
   make generate && make manifests && make build
   ```

**预期结果**:
- ✅ 所有工具安装完成
- ✅ Kind 集群运行正常
- ✅ 项目构建成功
- ✅ kubectl 可以连接集群

**常见问题**:
- **Docker 未运行**: 启动 OrbStack
- **网络问题**: 检查代理设置
- **权限问题**: 确保脚本有执行权限

### 阶段 2: 单元测试 (unit)

**目的**: 验证单个函数和方法的逻辑正确性，确保代码质量。

**执行命令**:
```bash
scripts/test/run-unit-tests.sh
```

**详细步骤**:

1. **代码格式检查** (5秒)
   ```bash
   # 检查代码格式
   gofmt -l . | grep -v vendor/
   ```

2. **静态代码分析** (30秒)
   ```bash
   # 运行 golangci-lint
   golangci-lint run --timeout=5m
   ```

3. **单元测试执行** (45秒)
   ```bash
   # 运行所有单元测试
   go test -v -race -coverprofile=coverage.out ./pkg/... ./internal/...
   ```

4. **覆盖率报告生成** (10秒)
   ```bash
   # 生成 HTML 覆盖率报告
   go tool cover -html=coverage.out -o coverage.html
   ```

5. **基准测试** (可选, 30秒)
   ```bash
   # 运行性能基准测试
   go test -bench=. -benchmem ./...
   ```

**测试内容**:
- **资源构建器测试** (`pkg/k8s/resources_test.go`)
  - StatefulSet 构建逻辑
  - Service 配置验证
  - ConfigMap 生成测试
  
- **工具函数测试** (`pkg/utils/labels_test.go`)
  - 标签生成逻辑
  - 注解合并功能
  - 选择器构建测试

- **控制器逻辑测试** (`internal/controller/etcdcluster_controller_unit_test.go`)
  - Reconcile 方法测试
  - 状态机转换验证
  - 错误处理测试

**预期结果**:
- ✅ 代码格式检查通过
- ✅ 静态分析无问题
- ✅ 所有单元测试通过
- ✅ 覆盖率 ≥ 50%

**覆盖率目标**:
```
pkg/k8s/resources.go:     85.2%
pkg/utils/labels.go:      92.1%
internal/controller/:     67.8%
总体覆盖率:               73.4%
```

### 阶段 3: 集成测试 (integration)

**目的**: 验证组件间协作和 Kubernetes API 交互，测试控制器完整功能。

**执行命令**:
```bash
scripts/test/run-integration-tests.sh
```

**详细步骤**:

1. **环境验证** (10秒)
   ```bash
   # 检查 kubectl 连接
   kubectl cluster-info
   ```

2. **测试命名空间设置** (5秒)
   ```bash
   # 创建测试命名空间
   kubectl create namespace etcd-operator-test
   ```

3. **CRD 安装** (30秒)
   ```bash
   # 安装自定义资源定义
   kubectl apply -f config/crd/bases/
   ```

4. **镜像构建和加载** (90秒)
   ```bash
   # 构建 Operator 镜像
   make docker-build IMG=etcd-operator:test
   kind load docker-image etcd-operator:test
   ```

5. **集成测试执行** (180秒)
   ```bash
   # 运行 envtest 集成测试
   ginkgo -v --timeout=30m ./test/integration/
   ```

6. **控制器测试** (120秒)
   ```bash
   # 部署 Operator 并测试
   kubectl apply -f config/rbac/
   kubectl apply -f test-operator-deployment.yaml
   ```

**测试场景**:

1. **EtcdCluster 创建测试**
   ```yaml
   # 创建测试集群
   apiVersion: etcd.etcd.io/v1alpha1
   kind: EtcdCluster
   metadata:
     name: test-cluster
   spec:
     size: 3
     version: "3.5.9"
   ```

2. **资源验证测试**
   - StatefulSet 创建和配置
   - Service 端口和选择器
   - ConfigMap 配置内容
   - PVC 存储配置

3. **状态管理测试**
   - 集群状态转换
   - 条件设置和更新
   - 错误状态处理

4. **基础扩缩容测试**
   - 从 3 节点扩容到 5 节点
   - StatefulSet 副本数更新
   - 状态同步验证

**预期结果**:
- ✅ CRD 安装成功
- ✅ 控制器正常运行
- ✅ 资源创建正确
- ✅ 状态管理正常
- ✅ 基础扩缩容功能正常

### 阶段 4: 端到端测试 (e2e)

**目的**: 在真实环境中验证完整的用户场景和系统行为。

**执行命令**:
```bash
scripts/test/run-e2e-tests.sh
```

**详细步骤**:

1. **环境检查** (15秒)
   ```bash
   # 验证 Kind 集群状态
   kind get clusters
   kubectl get nodes
   ```

2. **完整 Operator 部署** (120秒)
   ```bash
   # 构建和部署最新版本
   make docker-build IMG=etcd-operator:e2e
   make deploy IMG=etcd-operator:e2e
   ```

3. **端到端测试场景执行** (600秒)

**测试场景详解**:

#### 场景 1: 基础集群生命周期 (180秒)

```bash
# 1. 创建集群
kubectl apply -f - <<EOF
apiVersion: etcd.etcd.io/v1alpha1
kind: EtcdCluster
metadata:
  name: e2e-basic
  namespace: etcd-operator-e2e
spec:
  size: 3
  version: "3.5.9"
  storage:
    size: "2Gi"
  security:
    tls:
      enabled: true
      autoTLS: true
EOF

# 2. 等待集群就绪
kubectl wait --for=condition=Ready etcdcluster/e2e-basic --timeout=300s

# 3. 验证资源创建
kubectl get statefulset e2e-basic
kubectl get service e2e-basic-client
kubectl get pods -l etcd.etcd.io/cluster=e2e-basic

# 4. 删除集群
kubectl delete etcdcluster e2e-basic

# 5. 验证资源清理
kubectl wait --for=delete etcdcluster/e2e-basic --timeout=180s
```

#### 场景 2: 集群扩缩容 (240秒)

```bash
# 1. 创建 3 节点集群
kubectl apply -f e2e-scaling-cluster.yaml

# 2. 等待初始集群就绪
kubectl wait --for=condition=Ready etcdcluster/e2e-scaling --timeout=300s

# 3. 扩容到 5 节点
kubectl patch etcdcluster e2e-scaling --type='merge' -p='{"spec":{"size":5}}'

# 4. 等待扩容完成
kubectl wait --for=jsonpath='{.status.readyReplicas}'=5 etcdcluster/e2e-scaling --timeout=300s

# 5. 缩容到 3 节点
kubectl patch etcdcluster e2e-scaling --type='merge' -p='{"spec":{"size":3}}'

# 6. 等待缩容完成
kubectl wait --for=jsonpath='{.status.readyReplicas}'=3 etcdcluster/e2e-scaling --timeout=300s
```

#### 场景 3: 故障恢复 (180秒)

```bash
# 1. 创建集群
kubectl apply -f e2e-recovery-cluster.yaml
kubectl wait --for=condition=Ready etcdcluster/e2e-recovery --timeout=300s

# 2. 模拟 Pod 故障
kubectl delete pod e2e-recovery-0

# 3. 等待自动恢复
sleep 60
kubectl wait --for=condition=Ready etcdcluster/e2e-recovery --timeout=300s

# 4. 验证集群功能
kubectl exec e2e-recovery-0 -- etcdctl endpoint health
```

#### 场景 4: 数据持久化 (240秒)

```bash
# 1. 创建集群
kubectl apply -f e2e-persistence-cluster.yaml
kubectl wait --for=condition=Ready etcdcluster/e2e-persistence --timeout=300s

# 2. 写入测试数据
kubectl exec e2e-persistence-0 -- etcdctl put test-key "test-value-$(date +%s)"
kubectl exec e2e-persistence-0 -- etcdctl put persistent-key "persistent-value"

# 3. 重启集群 (删除所有 Pod)
kubectl delete pods -l etcd.etcd.io/cluster=e2e-persistence

# 4. 等待集群恢复
kubectl wait --for=condition=Ready etcdcluster/e2e-persistence --timeout=300s

# 5. 验证数据持久化
VALUE=$(kubectl exec e2e-persistence-0 -- etcdctl get persistent-key --print-value-only)
if [[ "$VALUE" == "persistent-value" ]]; then
    echo "✅ 数据持久化验证成功"
else
    echo "❌ 数据持久化验证失败"
fi
```

**预期结果**:
- ✅ 基础生命周期正常
- ✅ 扩缩容功能正常
- ✅ 故障自动恢复
- ✅ 数据持久化正常

## 📊 测试结果解读

### 成功示例

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
  开始时间: 2025-07-21 14:30:15
  结束时间: 2025-07-21 14:40:26

🎉 所有测试通过！
```

### 失败示例

```
测试结果:
  ✓ setup: 成功 (45s)
  ✓ unit: 成功 (23s)
  ✗ integration: 失败 (89s)
  ○ e2e: 跳过

❌ 部分测试失败！

故障排除建议:
1. 检查集成测试日志
2. 验证 Kind 集群状态
3. 重新运行失败的测试阶段
```

## 🔧 故障排除指南

### 常见问题和解决方案

#### 1. 环境设置失败

**问题**: Kind 集群创建失败
```bash
ERROR: failed to create cluster: failed to ensure docker
```

**解决方案**:
```bash
# 检查 Docker 状态
docker info

# 重启 OrbStack
# 清理旧集群
kind delete cluster --name etcd-operator-test

# 重新创建
scripts/test/setup-test-env.sh
```

#### 2. 单元测试失败

**问题**: 覆盖率不足
```bash
ERROR: 覆盖率 45.2% 未达到阈值要求 (>= 50%)
```

**解决方案**:
```bash
# 降低覆盖率要求
scripts/test/run-unit-tests.sh --coverage-threshold 40

# 或者添加更多测试用例
```

#### 3. 集成测试超时

**问题**: 控制器部署超时
```bash
ERROR: 等待 Operator 就绪超时
```

**解决方案**:
```bash
# 检查集群资源
kubectl get nodes
kubectl top nodes

# 检查 Operator 日志
kubectl logs -n etcd-k8s-operator-system deployment/etcd-k8s-operator-controller-manager

# 重新运行测试
scripts/test/run-integration-tests.sh --skip-build
```

#### 4. 端到端测试失败

**问题**: 集群创建失败
```bash
ERROR: 集群创建失败: test-cluster
```

**解决方案**:
```bash
# 检查 CRD 状态
kubectl get crd etcdclusters.etcd.etcd.io

# 检查 Operator 状态
kubectl get pods -n etcd-k8s-operator-system

# 查看详细错误
kubectl describe etcdcluster test-cluster
```

## 📈 性能基准

### 预期执行时间

| 阶段 | 最小时间 | 平均时间 | 最大时间 |
|------|---------|---------|---------|
| 环境设置 | 30s | 45s | 90s |
| 单元测试 | 15s | 25s | 60s |
| 集成测试 | 120s | 180s | 300s |
| 端到端测试 | 300s | 450s | 600s |
| **总计** | **465s** | **700s** | **1050s** |

### 资源使用

- **内存**: 2-4GB (峰值)
- **CPU**: 2-4 核心
- **磁盘**: 5-10GB (临时文件)
- **网络**: 100MB (镜像下载)

## 🎯 下一步行动

测试完成后，根据结果采取相应行动：

### 如果所有测试通过 ✅
1. 提交代码到版本控制
2. 创建 Pull Request
3. 更新项目文档
4. 准备下一个开发迭代

### 如果部分测试失败 ❌
1. 分析失败原因
2. 修复发现的问题
3. 重新运行相关测试
4. 更新测试用例（如需要）

### 持续改进 🔄
1. 分析测试执行时间
2. 优化慢速测试
3. 增加测试覆盖率
4. 完善故障排除文档

---

**文档版本**: v1.0  
**最后更新**: 2025-07-21  
**维护者**: ETCD Operator 开发团队

现在您已经了解了完整的测试过程，可以开始执行测试了！建议从 `make test-setup` 开始。
