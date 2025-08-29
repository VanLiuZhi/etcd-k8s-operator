# ETCD Kubernetes Operator 测试工作流指导文档

## 📋 文档概述

本文档旨在为 ETCD Kubernetes Operator 项目的测试工作提供完整的指导，包括测试环境准备、各类测试执行流程、结果分析和故障排除等内容。通过遵循本文档，开发人员和测试人员可以高效地进行项目测试，确保代码质量和功能正确性。

## 🎯 测试目标

1. **确保功能正确性** - 验证所有功能按预期工作
2. **保证代码质量** - 通过测试发现潜在问题和缺陷
3. **提高可靠性** - 确保在各种场景下系统稳定运行
4. **支持持续集成** - 建立自动化测试流程，支持CI/CD
5. **降低维护成本** - 通过完善的测试体系减少后期维护工作量

## 📚 文档结构

- 测试环境准备
- 单元测试执行指南
- 集成测试执行指南
- 端到端测试执行指南
- 测试结果分析和报告
- 常见问题和故障排除

## 🛠️ 测试环境准备

### 系统要求

在开始测试之前，请确保您的系统满足以下要求：

1. **操作系统**: Linux/macOS/Windows (推荐使用Linux或macOS)
2. **Go版本**: 1.23.4 或更高版本
3. **Docker**: 20.10 或更高版本
4. **Kubernetes**: 1.28 或更高版本
5. **Kind**: 0.17 或更高版本 (用于本地测试集群)
6. **kubectl**: 与Kubernetes集群版本匹配

### 开发环境设置

#### 1. 克隆代码仓库

```bash
git clone https://github.com/etcd-lz/etcd-k8s-operator.git
cd etcd-k8s-operator
```

#### 2. 安装Go依赖

```bash
make deps
```

#### 3. 安装测试工具

```bash
# 安装测试所需的工具
make test-setup
```

### Kind集群设置

#### 1. 创建测试集群

```bash
# 创建Kind测试集群
make kind-create
```

#### 2. 验证集群状态

```bash
# 检查集群节点状态
kubectl get nodes

# 检查集群组件状态
kubectl get pods -n kube-system
```

#### 3. 部署CRD

```bash
# 安装自定义资源定义
make install
```

### 测试数据准备

#### 1. 准备测试用的EtcdCluster资源

```bash
# 应用示例EtcdCluster资源
kubectl apply -f config/samples/etcd_v1alpha1_etcdcluster.yaml
```

#### 2. 验证资源创建

```bash
# 检查EtcdCluster资源状态
kubectl get etcdcluster

# 查看详细信息
kubectl describe etcdcluster etcdcluster-sample
```

### 环境变量配置

在执行测试之前，可能需要设置以下环境变量：

```bash
# 设置测试相关的环境变量
export TEST_NAMESPACE=default
export TEST_TIMEOUT=300s
export TEST_VERBOSE=true
```

## 🧪 单元测试执行指南

单元测试是对代码中最小可测试单元（如函数、方法）进行验证的测试方法。在ETCD Kubernetes Operator项目中，单元测试主要用于验证各个组件的独立功能。

### 运行单元测试

#### 1. 运行所有单元测试

```bash
# 运行所有单元测试
make test-unit
```

#### 2. 运行特定包的单元测试

```bash
# 运行特定包的测试
go test ./pkg/cluster/... -v

# 运行控制器包的测试
go test ./internal/controller/... -v
```

#### 3. 运行带有覆盖率检查的测试

```bash
# 运行测试并生成覆盖率报告
make test-unit-coverage
```

### 单元测试结构

#### 1. 测试文件命名规范

单元测试文件应遵循Go语言的命名规范，以 `_test.go` 结尾：

```
controller_test.go
cluster_test.go
reconcile_test.go
```

#### 2. 测试函数命名规范

测试函数应以 `Test` 开头，后跟被测试函数的名称：

```go
func TestClusterCreate(t *testing.T) {
    // 测试Cluster.Create函数
}

func TestReconcileMembers(t *testing.T) {
    // 测试reconcileMembers函数
}
```

### Mock和测试工具

#### 1. 使用Fake Client

项目使用controller-runtime提供的fake client进行测试：

```go
import (
    "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestExample(t *testing.T) {
    // 创建fake client
    fakeClient := fake.NewClientBuilder().Build()
    
    // 使用fake client进行测试
    // ...
}
```

#### 2. 使用Test Environment

对于需要更复杂测试环境的场景，可以使用test environment：

```bash
# 启动测试环境
make test-env-up

# 运行测试
go test ./... -tags=integration

# 清理测试环境
make test-env-down
```

### 单元测试最佳实践

1. **测试独立性**: 每个测试应独立运行，不依赖其他测试的执行结果
2. **测试覆盖**: 确保关键逻辑路径都有对应的测试用例
3. **测试数据**: 使用清晰、有意义的测试数据
4. **断言明确**: 使用明确的断言来验证预期结果
5. **错误处理**: 测试错误处理路径和边界条件

## 🔧 集成测试执行指南

集成测试用于验证多个组件之间的交互是否正常工作。在ETCD Kubernetes Operator项目中，集成测试主要用于验证Operator与Kubernetes API、etcd集群以及其他外部系统的交互。

### 运行集成测试

#### 1. 运行所有集成测试

```bash
# 运行所有集成测试
make test-integration
```

#### 2. 运行特定集成测试

```bash
# 运行特定的集成测试
go test ./test/integration/... -tags=integration -v
```

#### 3. 运行带有环境准备的集成测试

```bash
# 启动测试环境并运行集成测试
make test-integration-with-env
```

### 集成测试环境

#### 1. EnvTest环境

项目使用controller-runtime的envtest工具来创建测试环境：

```go
import (
    "sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestIntegrationExample(t *testing.T) {
    testEnv := &envtest.Environment{
        CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
    }
    
    cfg, err := testEnv.Start()
    if err != nil {
        t.Fatal(err)
    }
    defer testEnv.Stop()
    
    // 在测试环境中运行测试
    // ...
}
```

#### 2. 测试控制平面

EnvTest会启动一个本地的Kubernetes控制平面（API Server和etcd），用于运行集成测试：

- API Server运行在随机端口
- 使用真实的Kubernetes API
- 支持CRD和自定义资源

### 集成测试结构

#### 1. 测试文件组织

集成测试文件通常放在 `test/integration` 目录下：

```
test/
└── integration/
    ├── cluster/
    │   ├── cluster_integration_test.go
    │   └── suite_test.go
    ├── controller/
    │   ├── controller_integration_test.go
    │   └── suite_test.go
    └── k8s/
        ├── k8s_integration_test.go
        └── suite_test.go
```

#### 2. 测试套件

集成测试通常使用测试套件来管理共享的测试环境：

```go
import (
    "testing"
    "github.com/onsi/ginkgo/v2"
    "github.com/onsi/gomega"
)

func TestIntegration(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Integration Suite")
}
```

### 集成测试最佳实践

1. **环境复用**: 在测试套件中复用测试环境，避免重复启动和停止
2. **资源清理**: 确保测试结束后清理创建的资源
3. **并发安全**: 集成测试可能并发运行，确保测试之间不相互干扰
4. **超时控制**: 设置合理的测试超时时间
5. **日志记录**: 记录详细的测试日志以便调试

## 🌐 端到端测试执行指南

端到端测试（E2E测试）用于验证整个系统在真实环境中的行为是否符合预期。在ETCD Kubernetes Operator项目中，端到端测试主要用于验证Operator在真实的Kubernetes集群中管理etcd集群的完整功能。

### 运行端到端测试

#### 1. 运行所有端到端测试

```bash
# 运行所有端到端测试
make test-e2e
```

#### 2. 运行特定端到端测试

```bash
# 运行特定的端到端测试
go test ./test/e2e/... -tags=e2e -v
```

#### 3. 运行快速端到端测试

```bash
# 运行快速端到端测试（跳过耗时较长的测试）
make test-e2e-fast
```

### 端到端测试环境

#### 1. Kind集群环境

端到端测试使用Kind（Kubernetes in Docker）创建真实的Kubernetes集群：

```bash
# 创建端到端测试集群
make kind-create-e2e

# 删除端到端测试集群
make kind-delete-e2e
```

#### 2. 测试资源配置

端到端测试会自动部署以下资源：

- EtcdCluster CRD
- Operator部署
- RBAC权限配置
- 测试用的EtcdCluster实例

#### 3. 测试数据清理

测试结束后，会自动清理创建的资源：

```bash
# 清理测试数据
make test-e2e-cleanup
```

### 端到端测试结构

#### 1. 测试文件组织

端到端测试文件通常放在 `test/e2e` 目录下：

```
test/
└── e2e/
    ├── cluster/
    │   ├── cluster_creation_test.go
    │   ├── cluster_scaling_test.go
    │   └── cluster_deletion_test.go
    ├── operator/
    │   ├── operator_deployment_test.go
    │   └── operator_upgrade_test.go
    └── suite_test.go
```

#### 2. 测试框架

端到端测试使用Ginkgo和Gomega作为测试框架：

```go
import (
    "github.com/onsi/ginkgo/v2"
    "github.com/onsi/gomega"
)

var _ = Describe("EtcdCluster", func() {
    Context("when creating a new cluster", func() {
        It("should create etcd pods successfully", func() {
            // 测试逻辑
            Expect(pods).To(HaveLen(3))
        })
    })
})
```

### 端到端测试场景

#### 1. 集群创建测试

验证Operator能否成功创建etcd集群：

```bash
# 运行集群创建测试
go test ./test/e2e/cluster/cluster_creation_test.go -tags=e2e
```

#### 2. 集群扩缩容测试

验证Operator能否正确处理集群扩缩容：

```bash
# 运行集群扩缩容测试
go test ./test/e2e/cluster/cluster_scaling_test.go -tags=e2e
```

#### 3. 集群故障恢复测试

验证Operator能否在节点故障时恢复集群：

```bash
# 运行集群故障恢复测试
go test ./test/e2e/cluster/cluster_recovery_test.go -tags=e2e
```

### 端到端测试最佳实践

1. **测试隔离**: 每个测试应使用独立的命名空间和资源
2. **资源清理**: 确保测试结束后清理所有创建的资源
3. **超时设置**: 为长时间运行的操作设置合理的超时时间
4. **状态验证**: 验证系统状态而不仅仅是执行结果
5. **日志收集**: 收集测试过程中产生的日志以便调试

## 📊 测试结果分析和报告

测试结果分析是测试流程中的重要环节，通过分析测试结果可以了解代码质量、发现潜在问题并持续改进。

### 测试结果查看

#### 1. 控制台输出

测试运行时会直接在控制台输出结果：

```bash
# 运行测试并查看结果
make test

# 示例输出
ok      github.com/etcd-lz/etcd-k8s-operator/pkg/cluster    0.542s
ok      github.com/etcd-lz/etcd-k8s-operator/internal/controller 1.234s
FAIL    github.com/etcd-lz/etcd-k8s-operator/pkg/etcd [build failed]
```

#### 2. 详细测试日志

使用 `-v` 参数查看详细测试日志：

```bash
# 查看详细测试日志
go test ./... -v

# 示例输出
=== RUN   TestClusterCreate
--- PASS: TestClusterCreate (0.05s)
=== RUN   TestClusterScaleUp
--- FAIL: TestClusterScaleUp (0.12s)
    cluster_test.go:123: Expected 3 members, got 2
```

### 测试覆盖率分析

#### 1. 生成覆盖率报告

```bash
# 运行测试并生成覆盖率报告
make test-coverage

# 或者手动运行
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

#### 2. 查看覆盖率详情

```bash
# 查看文本格式的覆盖率报告
go tool cover -func=coverage.out

# 在浏览器中查看可视化报告
go tool cover -html=coverage.out
```

#### 3. 覆盖率阈值检查

项目设置了最低覆盖率要求：

```bash
# 检查覆盖率是否达到阈值
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//' | awk '{if ($1 < 80) exit 1}'
```

### 测试报告生成

#### 1. JUnit格式报告

CI/CD系统通常需要JUnit格式的测试报告：

```bash
# 生成JUnit格式报告
go install github.com/jstemmer/go-junit-report/v2@latest
go test ./... -v 2>&1 | go-junit-report > test-report.xml
```

#### 2. HTML格式报告

生成美观的HTML测试报告：

```bash
# 生成HTML测试报告
go install github.com/onsi/ginkgo/v2/ginkgo@latest
ginkgo -r --junit-report=report.xml --cover --coverprofile=coverage.out
```

### 测试结果解读

#### 1. 测试状态说明

- **PASS**: 测试通过，所有断言都满足
- **FAIL**: 测试失败，至少有一个断言不满足
- **SKIP**: 测试被跳过，通常由于条件不满足
- **BUILD FAILED**: 编译失败，代码存在语法错误

#### 2. 常见失败原因

1. **断言失败**: 实际结果与预期结果不匹配
2. **超时**: 测试运行时间超过设定的超时时间
3. **panic**: 代码中发生了未处理的异常
4. **资源不足**: 测试环境资源不足导致测试失败

#### 3. 覆盖率指标解读

- **语句覆盖率**: 代码中被执行的语句比例
- **分支覆盖率**: 代码中被执行的分支比例
- **函数覆盖率**: 代码中被调用的函数比例

### 持续集成中的测试

#### 1. CI流水线集成

在GitHub Actions中集成测试：

```yaml
# .github/workflows/test.yml
- name: Run tests
  run: make test
  
- name: Check coverage
  run: make test-coverage-check
  
- name: Generate report
  run: make test-report
```

#### 2. 测试门禁

设置测试门禁确保代码质量：

- 所有测试必须通过
- 覆盖率不能低于阈值
- 不能引入新的失败测试

## ❓ 常见问题和故障排除

在测试过程中可能会遇到各种问题，本节列出了一些常见问题及其解决方案。

### 环境相关问题

#### 1. Kind集群创建失败

**问题现象**：
```bash
ERROR: failed to create cluster: failed to generate kubeadm config
```

**解决方案**：
```bash
# 检查Docker是否运行
docker info

# 重启Docker服务
sudo systemctl restart docker

# 清理现有集群
make kind-delete

# 重新创建集群
make kind-create
```

#### 2. CRD部署失败

**问题现象**：
```bash
Error from server (Invalid): error when creating "config/crd/bases/k8s.etcd.lz_etcdclusters.yaml": 
CustomResourceDefinition.apiextensions.k8s.io "etcdclusters.k8s.etcd.lz" is invalid
```

**解决方案**：
```bash
# 删除现有的CRD
kubectl delete crd etcdclusters.k8s.etcd.lz

# 重新部署CRD
make install

# 验证部署
kubectl get crd etcdclusters.k8s.etcd.lz
```

### 测试执行问题

#### 1. 测试超时

**问题现象**：
```bash
panic: test timed out after 10m0s
```

**解决方案**：
```bash
# 增加测试超时时间
go test ./... -timeout=30m

# 或者在Makefile中调整超时设置
make test TIMEOUT=30m
```

#### 2. 测试环境资源不足

**问题现象**：
```bash
failed to start the controlplane. retried 5 times: fork/exec /usr/local/kubebuilder/bin/etcd: cannot allocate memory
```

**解决方案**：
```bash
# 增加Docker资源限制
# 在Docker Desktop中: Preferences -> Resources -> Increase Memory/CPU

# 减少并行测试数量
go test ./... -p 1

# 清理测试环境
make test-cleanup
```

### 依赖相关问题

#### 1. Go模块依赖问题

**问题现象**：
```bash
go: github.com/etcd-lz/etcd-k8s-operator@v0.0.0: parsing go.mod:
module declares its path as: github.com/your-org/etcd-k8s-operator
        but was required as: github.com/etcd-lz/etcd-k8s-operator
```

**解决方案**：
```bash
# 清理模块缓存
go clean -modcache

# 重新下载依赖
go mod download

# 验证模块路径
go mod verify
```

#### 2. 工具链版本不兼容

**问题现象**：
```bash
../../../go/pkg/mod/sigs.k8s.io/controller-runtime@v0.18.2/pkg/builder/options.go:54:15: undefined: strings.Cut
```

**解决方案**：
```bash
# 检查Go版本
go version

# 确保使用正确的Go版本（1.23.4+）
# 如果需要升级Go版本，请参考官方文档进行升级
```

### 测试代码问题

#### 1. 测试数据竞争

**问题现象**：
```bash
WARNING: DATA RACE
Write at 0x00c000123456 by goroutine 12:
```

**解决方案**：
```bash
# 运行带竞态检测的测试
go test ./... -race

# 修复代码中的竞态条件
# 1. 使用互斥锁保护共享资源
# 2. 使用原子操作
# 3. 避免在多个goroutine中修改同一变量
```

#### 2. Mock对象不匹配

**问题现象**：
```bash
missing call(s) to *mock.MockClient.Get(is anything, is equal to default/etcd-cluster-sample, is anything)
```

**解决方案**：
```bash
// 确保Mock调用的参数匹配
mockClient.EXPECT().Get(
    gomock.Any(), // context
    client.ObjectKey{Name: "etcd-cluster-sample", Namespace: "default"}, // object key
    gomock.Any(), // object
).DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
    // 实现mock逻辑
    return nil
})
```

### 调试技巧

#### 1. 增加日志输出

```bash
# 启用详细日志
export LOG_LEVEL=debug
make test

# 或者在测试中增加日志
func TestExample(t *testing.T) {
    t.Log("Debug information")
    // 测试代码
}
```

#### 2. 使用调试器

```bash
# 使用delve调试器
dlv test ./pkg/cluster -- -test.run TestClusterCreate

# 在代码中设置断点
func TestClusterCreate(t *testing.T) {
    // 在需要调试的地方添加
    // runtime.Breakpoint()
}
```

#### 3. 检查测试覆盖率

```bash
# 生成覆盖率报告
go test ./... -coverprofile=coverage.out

# 查看未覆盖的代码
go tool cover -html=coverage.out -o coverage.html
```

### 性能优化建议

1. **并行测试**: 合理使用`t.Parallel()`提高测试执行效率
2. **资源复用**: 在测试套件中复用昂贵的资源（如测试环境）
3. **测试数据**: 使用最小化的测试数据减少测试时间
4. **缓存清理**: 及时清理测试过程中产生的缓存和临时文件

### 联系支持

如果遇到无法解决的问题，请通过以下方式寻求帮助：

1. **查看文档**: 检查相关组件的官方文档
2. **搜索Issues**: 在GitHub Issues中搜索类似问题
3. **提交Issue**: 如果确认是bug，请提交详细的Issue报告
4. **社区支持**: 在相关社区或论坛寻求帮助

---
**文档版本**: v1.0
**最后更新**: 2025-08-28
**维护者**: ETCD Kubernetes Operator Team