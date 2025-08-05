# ETCD Operator 测试框架重构文档

[![测试状态](https://img.shields.io/badge/测试状态-设计中-yellow.svg)](https://github.com/your-org/etcd-k8s-operator)
[![框架版本](https://img.shields.io/badge/框架版本-v2.0-blue.svg)](https://github.com/your-org/etcd-k8s-operator)

> **框架状态**: 🚧 设计中 | **创建时间**: 2025-08-05 | **设计者**: AI Assistant

## 📋 测试框架概述

### 🎯 重构目标
- **统一测试框架**: 完全基于Go测试框架，移除Shell脚本依赖
- **分层测试策略**: 单元测试 → 集成测试 → 端到端测试
- **高测试覆盖率**: 目标达到80%以上的代码覆盖率
- **自动化测试**: CI/CD集成，自动化测试执行

### 🚨 当前测试问题

#### 现有测试架构问题
```
当前测试架构 (混乱不堪):
├── Go 测试框架
│   ├── test/e2e/e2e_test.go
│   ├── test/integration/etcdcluster_test.go
│   └── internal/controller/*_test.go
├── Shell 脚本测试
│   ├── scripts/test/run-unit-tests.sh
│   ├── scripts/test/run-integration-tests.sh
│   ├── scripts/test/run-e2e-tests.sh
│   └── test/scripts/test-scale-to-zero-simple.sh
└── 手动测试配置
    ├── test/testdata/*.yaml
    └── config/samples/*.yaml
```

**主要问题**:
- ❌ **测试方式混乱**: Go测试与Shell脚本混用
- ❌ **环境依赖复杂**: 依赖外部脚本设置环境
- ❌ **测试覆盖不足**: 扩缩容功能测试不理想
- ❌ **维护成本高**: 多套测试体系，维护困难

## 🏗️ 新测试框架设计

### 📐 三层测试架构

```
新测试框架 (清晰分层):

┌─────────────────────────────────────┐
│        端到端测试 (E2E Tests)         │  ← 完整用户场景测试
├─────────────────────────────────────┤
│        集成测试 (Integration Tests)   │  ← 组件间集成测试
├─────────────────────────────────────┤
│        单元测试 (Unit Tests)          │  ← 单个组件测试
└─────────────────────────────────────┘
```

### 🎯 各层测试定义

#### 1. 单元测试 (Unit Tests)
**目标**: 测试单个组件的功能，隔离外部依赖

```go
// 测试范围
├── 服务层单元测试 (pkg/service/*_test.go)
├── 资源层单元测试 (pkg/resource/*_test.go)
├── 客户端层单元测试 (pkg/client/*_test.go)
└── 工具函数单元测试 (pkg/utils/*_test.go)
```

**特点**:
- 🎯 **快速执行**: 每个测试 < 100ms
- 🎭 **Mock依赖**: 使用Mock隔离外部依赖
- 📊 **高覆盖率**: 目标覆盖率 90%+
- 🔄 **可重复**: 测试结果稳定可重复

#### 2. 集成测试 (Integration Tests)
**目标**: 测试组件间的集成，使用真实的外部依赖

```go
// 测试范围
├── 控制器集成测试 (test/integration/controller/*_test.go)
├── 服务层集成测试 (test/integration/service/*_test.go)
├── 资源管理集成测试 (test/integration/resource/*_test.go)
└── 客户端集成测试 (test/integration/client/*_test.go)
```

**特点**:
- 🐳 **容器化环境**: 使用testcontainers创建隔离环境
- 🔗 **真实依赖**: 使用真实的etcd和Kubernetes
- 📋 **场景覆盖**: 覆盖主要业务流程
- ⏱️ **适中执行时间**: 每个测试 < 30s

#### 3. 端到端测试 (E2E Tests)
**目标**: 测试完整的用户场景，验证整体功能

```go
// 测试范围
├── 集群生命周期测试 (test/e2e/lifecycle/*_test.go)
├── 扩缩容功能测试 (test/e2e/scaling/*_test.go)
├── 故障恢复测试 (test/e2e/recovery/*_test.go)
└── 性能压力测试 (test/e2e/performance/*_test.go)
```

**特点**:
- 🎭 **真实环境**: 使用Kind集群模拟真实环境
- 👤 **用户视角**: 从用户角度验证功能
- 🔄 **完整流程**: 测试完整的操作流程
- ⏰ **较长执行时间**: 每个测试 < 5min

## 🛠️ 测试工具栈

### 📦 核心测试工具

#### 1. **Go原生测试框架**
```go
// 基础测试框架
import (
    "testing"
    "context"
    "time"
)

func TestClusterService_CreateCluster(t *testing.T) {
    // 测试实现
}
```

#### 2. **Testify - 断言和Mock**
```go
// 断言库
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/suite"
)

// 使用断言
assert.Equal(t, expected, actual)
assert.NoError(t, err)

// 使用Mock
mockClient := &MockEtcdClient{}
mockClient.On("ListMembers", mock.Anything).Return(members, nil)
```

#### 3. **Ginkgo & Gomega - BDD测试**
```go
// BDD风格测试 (用于E2E测试)
import (
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

var _ = Describe("EtcdCluster", func() {
    Context("when creating a cluster", func() {
        It("should create all required resources", func() {
            // 测试实现
        })
    })
})
```

#### 4. **Testcontainers - 容器化测试环境**
```go
// 容器化测试环境
import (
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)

func setupEtcdContainer(ctx context.Context) (testcontainers.Container, error) {
    req := testcontainers.ContainerRequest{
        Image:        "quay.io/coreos/etcd:v3.5.21",
        ExposedPorts: []string{"2379/tcp"},
        WaitingFor:   wait.ForLog("ready to serve client requests"),
    }
    return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
}
```

#### 5. **Controller-Runtime测试工具**
```go
// Kubernetes控制器测试
import (
    "sigs.k8s.io/controller-runtime/pkg/envtest"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

var testEnv *envtest.Environment
var k8sClient client.Client
```

## 📁 新测试目录结构

### 🗂️ 重构后的测试结构

```
test/
├── unit/                          # 单元测试
│   ├── service/                   # 服务层单元测试
│   │   ├── cluster_test.go
│   │   ├── scaling_test.go
│   │   └── health_test.go
│   ├── resource/                  # 资源层单元测试
│   │   ├── statefulset_test.go
│   │   ├── service_test.go
│   │   └── configmap_test.go
│   ├── client/                    # 客户端层单元测试
│   │   ├── etcd_test.go
│   │   └── kubernetes_test.go
│   └── utils/                     # 工具函数单元测试
│       └── utils_test.go
├── integration/                   # 集成测试
│   ├── controller/                # 控制器集成测试
│   │   ├── cluster_controller_test.go
│   │   └── scaling_controller_test.go
│   ├── service/                   # 服务层集成测试
│   │   ├── cluster_service_test.go
│   │   └── scaling_service_test.go
│   └── resource/                  # 资源管理集成测试
│       └── resource_manager_test.go
├── e2e/                          # 端到端测试
│   ├── lifecycle/                # 生命周期测试
│   │   ├── create_test.go
│   │   ├── update_test.go
│   │   └── delete_test.go
│   ├── scaling/                  # 扩缩容测试
│   │   ├── scale_up_test.go
│   │   ├── scale_down_test.go
│   │   └── scale_to_zero_test.go
│   ├── recovery/                 # 故障恢复测试
│   │   └── failure_recovery_test.go
│   └── performance/              # 性能测试
│       └── load_test.go
├── testdata/                     # 测试数据
│   ├── clusters/                 # 集群配置
│   ├── manifests/                # K8s清单
│   └── fixtures/                 # 测试夹具
├── utils/                        # 测试工具
│   ├── test_utils.go            # 通用测试工具
│   ├── mock_clients.go          # Mock客户端
│   ├── test_env.go              # 测试环境设置
│   └── assertions.go            # 自定义断言
└── config/                       # 测试配置
    ├── test_config.yaml         # 测试配置
    └── kind_config.yaml         # Kind集群配置
```

## 🧪 测试实现示例

### 1. 单元测试示例

```go
// pkg/service/cluster_service_test.go
package service

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
    "github.com/your-org/etcd-k8s-operator/test/utils"
)

func TestClusterService_CreateCluster(t *testing.T) {
    // 准备测试数据
    cluster := utils.NewTestCluster("test-cluster", "default", 3)
    
    // 创建Mock对象
    mockStatefulSetManager := &utils.MockStatefulSetManager{}
    mockServiceManager := &utils.MockServiceManager{}
    mockConfigMapManager := &utils.MockConfigMapManager{}
    
    // 设置Mock期望
    mockStatefulSetManager.On("Create", mock.Anything, cluster).Return(nil)
    mockServiceManager.On("CreateHeadlessService", mock.Anything, cluster).Return(nil)
    mockServiceManager.On("CreateClientService", mock.Anything, cluster).Return(nil)
    mockConfigMapManager.On("Create", mock.Anything, cluster).Return(nil)
    
    // 创建服务实例
    service := NewClusterService(
        mockStatefulSetManager,
        mockServiceManager,
        mockConfigMapManager,
        nil, // etcdClient not needed for this test
        utils.NewTestLogger(),
    )
    
    // 执行测试
    err := service.CreateCluster(context.Background(), cluster)
    
    // 验证结果
    assert.NoError(t, err)
    mockStatefulSetManager.AssertExpectations(t)
    mockServiceManager.AssertExpectations(t)
    mockConfigMapManager.AssertExpectations(t)
}
```

### 2. 集成测试示例

```go
// test/integration/service/cluster_service_test.go
package service

import (
    "context"
    "testing"
    "github.com/stretchr/testify/suite"
    "github.com/testcontainers/testcontainers-go"
    "sigs.k8s.io/controller-runtime/pkg/envtest"
    "github.com/your-org/etcd-k8s-operator/pkg/service"
    "github.com/your-org/etcd-k8s-operator/test/utils"
)

type ClusterServiceIntegrationSuite struct {
    suite.Suite
    testEnv     *envtest.Environment
    etcdContainer testcontainers.Container
    service     service.ClusterService
}

func (suite *ClusterServiceIntegrationSuite) SetupSuite() {
    // 设置测试环境
    suite.testEnv = utils.SetupTestEnvironment()
    
    // 启动etcd容器
    var err error
    suite.etcdContainer, err = utils.StartEtcdContainer(context.Background())
    suite.Require().NoError(err)
    
    // 创建服务实例
    suite.service = utils.NewTestClusterService(suite.testEnv.Config)
}

func (suite *ClusterServiceIntegrationSuite) TearDownSuite() {
    // 清理资源
    suite.etcdContainer.Terminate(context.Background())
    suite.testEnv.Stop()
}

func (suite *ClusterServiceIntegrationSuite) TestCreateCluster() {
    // 准备测试数据
    cluster := utils.NewTestCluster("integration-test", "default", 3)
    
    // 执行测试
    err := suite.service.CreateCluster(context.Background(), cluster)
    suite.NoError(err)
    
    // 验证结果
    status, err := suite.service.GetClusterStatus(context.Background(), cluster)
    suite.NoError(err)
    suite.Equal("Creating", status.Phase)
}

func TestClusterServiceIntegrationSuite(t *testing.T) {
    suite.Run(t, new(ClusterServiceIntegrationSuite))
}
```

### 3. E2E测试示例

```go
// test/e2e/scaling/scale_up_test.go
package scaling

import (
    "context"
    "time"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "github.com/your-org/etcd-k8s-operator/api/v1alpha1"
    "github.com/your-org/etcd-k8s-operator/test/utils"
)

var _ = Describe("EtcdCluster Scale Up", func() {
    var (
        ctx     context.Context
        cluster *v1alpha1.EtcdCluster
    )
    
    BeforeEach(func() {
        ctx = context.Background()
        cluster = utils.NewTestCluster("scale-up-test", testNamespace, 1)
        
        // 创建初始集群
        Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
        
        // 等待集群就绪
        Eventually(func() bool {
            return utils.IsClusterReady(ctx, k8sClient, cluster)
        }, 5*time.Minute, 10*time.Second).Should(BeTrue())
    })
    
    AfterEach(func() {
        // 清理资源
        utils.CleanupCluster(ctx, k8sClient, cluster)
    })
    
    Context("when scaling from 1 to 3 nodes", func() {
        It("should successfully add 2 new nodes", func() {
            // 更新集群规模
            cluster.Spec.Size = 3
            Expect(k8sClient.Update(ctx, cluster)).To(Succeed())
            
            // 验证扩容过程
            Eventually(func() int32 {
                status := utils.GetClusterStatus(ctx, k8sClient, cluster)
                return status.ReadyReplicas
            }, 10*time.Minute, 30*time.Second).Should(Equal(int32(3)))
            
            // 验证etcd集群状态
            etcdStatus := utils.GetEtcdClusterStatus(ctx, cluster)
            Expect(etcdStatus.Members).To(HaveLen(3))
            Expect(etcdStatus.IsHealthy).To(BeTrue())
        })
    })
})
```

## 🚀 测试执行策略

### 📋 测试执行计划

#### 1. **本地开发测试**
```bash
# 快速单元测试
make test-unit-fast

# 完整单元测试
make test-unit

# 集成测试
make test-integration

# 端到端测试
make test-e2e
```

#### 2. **CI/CD测试流水线**
```yaml
# .github/workflows/test.yml
name: Test Pipeline
on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
      - run: make test-unit
      - run: make coverage-report
  
  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
      - run: make test-integration
  
  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v3
      - run: make test-e2e
```

### 📊 测试覆盖率目标

#### 分层覆盖率目标
- **单元测试**: 90%+ (服务层、资源层、客户端层)
- **集成测试**: 80%+ (组件间集成)
- **端到端测试**: 70%+ (用户场景)
- **总体覆盖率**: 80%+

#### 覆盖率监控
```bash
# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# 覆盖率检查
go tool cover -func=coverage.out | grep total
```

## 🎯 迁移计划

### 📅 测试框架迁移时间表

#### 第1周: 基础设施建设
- [ ] 创建新的测试目录结构
- [ ] 建立测试工具包和Mock对象
- [ ] 配置测试环境和CI/CD

#### 第2-3周: 测试迁移
- [ ] 迁移现有单元测试
- [ ] 创建新的集成测试
- [ ] 重写端到端测试

#### 第4周: 优化和清理
- [ ] 移除Shell脚本测试
- [ ] 优化测试性能
- [ ] 完善测试文档

### ✅ 成功标准
- ✅ 完全移除Shell脚本依赖
- ✅ 测试覆盖率达到80%+
- ✅ 测试执行时间减少50%+
- ✅ CI/CD集成完成

---

**下一步**: 开始创建测试工具包和Mock对象
