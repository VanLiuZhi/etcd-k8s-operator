# ETCD Operator 三层测试架构详细指南

[![测试架构](https://img.shields.io/badge/测试架构-三层设计-green.svg)](https://github.com/your-org/etcd-k8s-operator)
[![技术栈](https://img.shields.io/badge/技术栈-Go测试生态-blue.svg)](https://github.com/your-org/etcd-k8s-operator)

> **文档状态**: 📋 技术指南 | **创建时间**: 2025-08-05 | **作者**: AI Assistant

## 📋 三层测试架构概述

### 🎯 测试金字塔设计

```
        ┌─────────────────────────┐
        │     E2E Tests (少)       │  ← 完整用户场景，真实环境
        │    慢，昂贵，全面         │
        └─────────────────────────┘
               ┌─────────────────────────────┐
               │   Integration Tests (中)     │  ← 组件集成，半真实环境
               │      中速，中等成本          │
               └─────────────────────────────┘
                      ┌─────────────────────────────────┐
                      │      Unit Tests (多)             │  ← 单个组件，Mock环境
                      │        快，便宜，精确            │
                      └─────────────────────────────────┘
```

### 📊 测试分层原则

#### 1. **70% 单元测试** - 快速反馈
- **目标**: 测试单个函数/方法的逻辑
- **环境**: 完全隔离，Mock所有外部依赖
- **执行时间**: < 1秒/测试
- **覆盖率**: 90%+

#### 2. **20% 集成测试** - 组件协作
- **目标**: 测试多个组件之间的协作
- **环境**: 部分真实环境 (真实etcd + 模拟K8s)
- **执行时间**: < 30秒/测试
- **覆盖率**: 80%+

#### 3. **10% 端到端测试** - 用户场景
- **目标**: 测试完整的用户使用场景
- **环境**: 完全真实环境 (真实K8s + 真实etcd)
- **执行时间**: < 5分钟/测试
- **覆盖率**: 主要用户路径

## 🛠️ 测试工具栈详解

### 1. **Go 原生测试框架** (基础)

#### 什么是Go原生测试
```go
// Go语言内置的测试框架，所有Go项目的基础
package service

import "testing"

func TestClusterService_CreateCluster(t *testing.T) {
    // 测试逻辑
    if result != expected {
        t.Errorf("期望 %v, 得到 %v", expected, result)
    }
}
```

#### 特点和用途
- ✅ **内置支持**: Go语言标准库，无需额外依赖
- ✅ **简单直接**: 基本的断言和测试运行
- ✅ **性能优秀**: 执行速度快，资源占用少
- 🎯 **适用场景**: 单元测试的基础框架

### 2. **Testify** (断言和Mock库)

#### 什么是Testify
```go
// 增强Go测试的断言和Mock功能
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestExample(t *testing.T) {
    // 更友好的断言
    assert.Equal(t, expected, actual)
    assert.NoError(t, err)
    assert.Contains(t, slice, item)
}
```

#### 核心功能
- 🎯 **丰富断言**: 提供各种便捷的断言方法
- 🎭 **Mock支持**: 创建Mock对象，模拟外部依赖
- 📊 **测试套件**: 组织复杂的测试场景
- 🎯 **适用场景**: 单元测试和集成测试的断言

#### Mock示例
```go
// 创建Mock对象
type MockEtcdClient struct {
    mock.Mock
}

func (m *MockEtcdClient) ListMembers(ctx context.Context) ([]Member, error) {
    args := m.Called(ctx)
    return args.Get(0).([]Member), args.Error(1)
}

// 在测试中使用
mockClient := &MockEtcdClient{}
mockClient.On("ListMembers", mock.Anything).Return(members, nil)
```

### 3. **Ginkgo & Gomega** (BDD测试框架)

#### 什么是Ginkgo
```go
// 行为驱动开发(BDD)风格的测试框架
import (
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

var _ = Describe("EtcdCluster", func() {
    Context("when creating a cluster", func() {
        It("should create all required resources", func() {
            // 测试逻辑
            Expect(result).To(Equal(expected))
        })
    })
})
```

#### 核心特点
- 📝 **可读性强**: 类似自然语言的测试描述
- 🏗️ **结构化**: Describe/Context/It 层次结构
- ⚡ **并发执行**: 支持并行测试执行
- 🎯 **适用场景**: 集成测试和E2E测试

#### BDD结构说明
```go
Describe("功能模块")     // 描述要测试的功能
  Context("在某种情况下")  // 描述测试的上下文/条件
    It("应该做某事")      // 描述期望的行为
      BeforeEach()      // 每个测试前的准备
      AfterEach()       // 每个测试后的清理
```

### 4. **Testcontainers** (容器化测试环境)

#### 什么是Testcontainers
```go
// 在测试中启动真实的Docker容器
import (
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/wait"
)

func setupEtcdContainer() (testcontainers.Container, error) {
    req := testcontainers.ContainerRequest{
        Image:        "quay.io/coreos/etcd:v3.5.21",
        ExposedPorts: []string{"2379/tcp"},
        Env: map[string]string{
            "ETCD_LISTEN_CLIENT_URLS": "http://0.0.0.0:2379",
        },
        WaitingFor: wait.ForLog("ready to serve client requests"),
    }
    return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
        ContainerRequest: req,
        Started:          true,
    })
}
```

#### 核心优势
- 🐳 **真实环境**: 使用真实的数据库/服务
- 🔄 **自动管理**: 自动启动和清理容器
- 🎯 **隔离性**: 每个测试独立的环境
- 🎯 **适用场景**: 集成测试中需要真实外部服务

### 5. **Controller-Runtime EnvTest** (K8s测试环境)

#### 什么是EnvTest
```go
// 模拟Kubernetes API Server的测试环境
import "sigs.k8s.io/controller-runtime/pkg/envtest"

testEnv := &envtest.Environment{
    CRDDirectoryPaths: []string{"config/crd/bases"},
    BinaryAssetsDirectory: "bin/k8s/1.28.3-linux-amd64",
}

cfg, err := testEnv.Start()  // 启动模拟的K8s API Server
```

#### 核心功能
- 🎭 **模拟K8s**: 启动真实的etcd和kube-apiserver
- 🚀 **快速启动**: 比完整K8s集群快很多
- 🎯 **控制器测试**: 专门为控制器测试设计
- 🎯 **适用场景**: 控制器逻辑的集成测试

## 🏗️ 三层测试架构实现

### 第1层: 单元测试 (Unit Tests)

#### 目录结构
```
test/unit/
├── service/           # 服务层单元测试
│   ├── cluster_test.go
│   ├── scaling_test.go
│   └── health_test.go
├── resource/          # 资源层单元测试
│   ├── statefulset_test.go
│   ├── service_test.go
│   └── configmap_test.go
├── client/            # 客户端层单元测试
│   ├── etcd_test.go
│   └── kubernetes_test.go
└── utils/             # 工具函数单元测试
    └── utils_test.go
```

#### 实现示例
```go
// test/unit/service/cluster_test.go
package service

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestClusterService_CreateCluster(t *testing.T) {
    // 1. 准备Mock对象
    mockStatefulSetManager := &MockStatefulSetManager{}
    mockServiceManager := &MockServiceManager{}
    
    // 2. 设置Mock期望
    mockStatefulSetManager.On("Create", mock.Anything, mock.Anything).Return(nil)
    mockServiceManager.On("CreateHeadlessService", mock.Anything, mock.Anything).Return(nil)
    
    // 3. 创建被测试对象
    service := NewClusterService(mockStatefulSetManager, mockServiceManager, ...)
    
    // 4. 执行测试
    err := service.CreateCluster(context.Background(), cluster)
    
    // 5. 验证结果
    assert.NoError(t, err)
    mockStatefulSetManager.AssertExpectations(t)
}
```

#### 特点
- ⚡ **执行速度**: < 100ms/测试
- 🎭 **完全隔离**: Mock所有外部依赖
- 🎯 **精确测试**: 只测试单个组件的逻辑
- 📊 **高覆盖率**: 目标90%+代码覆盖率

### 第2层: 集成测试 (Integration Tests)

#### 目录结构
```
test/integration/
├── controller/        # 控制器集成测试
│   ├── cluster_controller_test.go
│   └── scaling_controller_test.go
├── service/           # 服务层集成测试
│   ├── cluster_service_test.go
│   └── scaling_service_test.go
└── resource/          # 资源管理集成测试
    └── resource_manager_test.go
```

#### 实现示例
```go
// test/integration/service/cluster_service_test.go
package service

import (
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "github.com/testcontainers/testcontainers-go"
)

var _ = Describe("ClusterService Integration", func() {
    var (
        etcdContainer testcontainers.Container
        service       ClusterService
    )
    
    BeforeEach(func() {
        // 启动真实的etcd容器
        etcdContainer = startEtcdContainer()
        
        // 创建真实的etcd客户端
        etcdClient := createEtcdClient(etcdContainer)
        
        // 创建服务实例 (使用真实etcd + Mock K8s)
        service = NewClusterService(..., etcdClient, ...)
    })
    
    AfterEach(func() {
        etcdContainer.Terminate(context.Background())
    })
    
    It("should create cluster with real etcd", func() {
        err := service.CreateCluster(ctx, cluster)
        Expect(err).NotTo(HaveOccurred())
        
        // 验证etcd中的数据
        members, err := etcdClient.ListMembers(ctx)
        Expect(err).NotTo(HaveOccurred())
        Expect(members).To(HaveLen(3))
    })
})
```

#### 特点
- 🐳 **半真实环境**: 真实etcd + 模拟K8s
- ⏱️ **中等速度**: < 30秒/测试
- 🔗 **组件集成**: 测试多个组件协作
- 📊 **中等覆盖率**: 目标80%+

### 第3层: 端到端测试 (E2E Tests)

#### 目录结构
```
test/e2e/
├── lifecycle/         # 生命周期测试
│   ├── create_test.go
│   ├── update_test.go
│   └── delete_test.go
├── scaling/           # 扩缩容测试
│   ├── scale_up_test.go
│   ├── scale_down_test.go
│   └── scale_to_zero_test.go
├── recovery/          # 故障恢复测试
│   └── failure_recovery_test.go
└── performance/       # 性能测试
    └── load_test.go
```

#### 实现示例 (基于你的Kind环境)
```go
// test/e2e/scaling/scale_to_zero_test.go
package scaling

import (
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Scale to Zero E2E", func() {
    var (
        k8sClient client.Client
        cluster   *v1alpha1.EtcdCluster
        namespace string
    )
    
    BeforeEach(func() {
        // 使用你的Kind集群
        k8sClient = setupKindClient()
        namespace = createTestNamespace()
        
        // 创建测试集群
        cluster = createTestCluster(namespace, 1)
        Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
        
        // 等待集群就绪
        waitForClusterReady(k8sClient, cluster)
    })
    
    AfterEach(func() {
        cleanupTestResources(k8sClient, namespace)
    })
    
    It("should support complete scale-to-zero cycle", func() {
        By("scaling up to 3 nodes")
        updateClusterSize(k8sClient, cluster, 3)
        waitForClusterSize(k8sClient, cluster, 3)
        verifyEtcdHealth(k8sClient, cluster, 3)
        
        By("scaling down to 1 node")
        updateClusterSize(k8sClient, cluster, 1)
        waitForClusterSize(k8sClient, cluster, 1)
        verifyEtcdHealth(k8sClient, cluster, 1)
        
        By("scaling to zero (stop cluster)")
        updateClusterSize(k8sClient, cluster, 0)
        waitForClusterSize(k8sClient, cluster, 0)
        verifyPVCsPreserved(k8sClient, cluster)
        
        By("scaling back to 1 (restart cluster)")
        updateClusterSize(k8sClient, cluster, 1)
        waitForClusterSize(k8sClient, cluster, 1)
        verifyEtcdHealth(k8sClient, cluster, 1)
        verifyDataPersistence(k8sClient, cluster)
    })
})
```

#### 特点
- 🌍 **完全真实**: 真实K8s + 真实etcd
- ⏰ **较慢执行**: < 5分钟/测试
- 👤 **用户视角**: 测试完整用户场景
- 🎯 **关键路径**: 覆盖主要功能

## 🚀 基于你的环境的实施方案

### 🐳 利用你的Docker环境

#### 1. **单元测试**: 纯Go测试，不需要Docker
```bash
# 快速单元测试
go test ./test/unit/... -v

# 带覆盖率的单元测试
go test ./test/unit/... -coverprofile=unit.out
```

#### 2. **集成测试**: 使用Docker启动etcd
```bash
# testcontainers会自动使用你的Docker环境
go test ./test/integration/... -v
```

#### 3. **E2E测试**: 使用你的Kind集群
```bash
# 确保Kind集群运行
kind get clusters

# 运行E2E测试
go test ./test/e2e/... -v
```

### 🎯 测试执行策略

#### 开发阶段
```bash
# 1. 快速反馈 - 只运行单元测试
make test-unit-fast

# 2. 完整验证 - 运行所有单元测试
make test-unit

# 3. 集成验证 - 运行集成测试
make test-integration

# 4. 完整验证 - 运行E2E测试 (较慢)
make test-e2e
```

#### CI/CD流水线
```yaml
# 并行执行不同层次的测试
jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - run: make test-unit
  
  integration-tests:
    runs-on: ubuntu-latest
    services:
      docker:
        image: docker:dind
    steps:
      - run: make test-integration
  
  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: helm/kind-action@v1
      - run: make test-e2e
```

## 📊 测试覆盖率目标

### 分层覆盖率
- **单元测试**: 90%+ (服务层、资源层、客户端层)
- **集成测试**: 80%+ (组件间集成路径)
- **E2E测试**: 70%+ (主要用户场景)
- **总体覆盖率**: 85%+

### 质量门禁
```bash
# 覆盖率检查
go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//' | awk '{if($1<85) exit 1}'

# 测试必须通过
go test ./... -v

# 性能回归检查
go test -bench=. -benchmem ./test/performance/...
```

## 🎯 迁移路径

### 阶段1: 建立基础设施 (第2周前半)
1. 创建测试目录结构
2. 引入测试工具依赖
3. 创建测试工具包和Mock对象

### 阶段2: 实现单元测试 (第2周后半)
1. 为新的服务层编写单元测试
2. 为新的资源层编写单元测试
3. 建立Mock对象和测试夹具

### 阶段3: 实现集成测试 (第3周前半)
1. 使用testcontainers建立集成测试
2. 测试控制器与服务层集成
3. 测试服务层与真实etcd集成

### 阶段4: 实现E2E测试 (第3周后半)
1. 基于Kind集群实现E2E测试
2. 迁移Shell脚本功能到Go测试
3. 建立完整的用户场景测试

### 阶段5: 清理和优化 (第4周)
1. 移除Shell脚本测试
2. 优化测试执行性能
3. 建立CI/CD集成

## 🔧 实用工具和配置

### 📦 依赖管理

#### 添加测试依赖
```bash
# 基础测试工具
go get github.com/stretchr/testify@latest
go get github.com/onsi/ginkgo/v2@latest
go get github.com/onsi/gomega@latest

# 容器化测试
go get github.com/testcontainers/testcontainers-go@latest

# Kubernetes测试
go get sigs.k8s.io/controller-runtime/pkg/envtest@latest
```

#### Makefile配置
```makefile
# 测试相关目标
.PHONY: test-unit test-integration test-e2e test-all

test-unit:
	go test ./test/unit/... -v -race -coverprofile=unit.out

test-integration:
	go test ./test/integration/... -v -timeout=10m

test-e2e:
	go test ./test/e2e/... -v -timeout=30m

test-all: test-unit test-integration test-e2e

coverage-report:
	go tool cover -html=unit.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-watch:
	# 监控文件变化，自动运行测试
	find . -name "*.go" | entr -r make test-unit
```

### 🐳 Docker配置

#### 测试用的docker-compose.yml
```yaml
# test/docker-compose.test.yml
version: '3.8'
services:
  etcd:
    image: quay.io/coreos/etcd:v3.5.21
    environment:
      - ETCD_LISTEN_CLIENT_URLS=http://0.0.0.0:2379
      - ETCD_ADVERTISE_CLIENT_URLS=http://localhost:2379
      - ETCD_LISTEN_PEER_URLS=http://0.0.0.0:2380
      - ETCD_INITIAL_ADVERTISE_PEER_URLS=http://localhost:2380
      - ETCD_INITIAL_CLUSTER=etcd0=http://localhost:2380
      - ETCD_NAME=etcd0
      - ETCD_DATA_DIR=/etcd-data
    ports:
      - "2379:2379"
      - "2380:2380"
    volumes:
      - etcd-data:/etcd-data

volumes:
  etcd-data:
```

### 🎯 测试配置文件

#### 测试配置
```yaml
# test/config/test_config.yaml
test:
  timeouts:
    unit: "30s"
    integration: "5m"
    e2e: "15m"

  etcd:
    image: "quay.io/coreos/etcd:v3.5.21"
    client_port: 2379
    peer_port: 2380

  kubernetes:
    version: "1.28.3"
    namespace_prefix: "test-"

  coverage:
    threshold: 85
    exclude_patterns:
      - "test/"
      - "mock/"
      - "*.pb.go"
```

## 📋 最佳实践指南

### 🎯 单元测试最佳实践

#### 1. **测试命名规范**
```go
// 格式: Test[被测试的结构体]_[被测试的方法]_[测试场景]
func TestClusterService_CreateCluster_WithValidInput(t *testing.T) {}
func TestClusterService_CreateCluster_WithInvalidSize(t *testing.T) {}
func TestClusterService_CreateCluster_WhenResourceExists(t *testing.T) {}
```

#### 2. **测试结构模式**
```go
func TestClusterService_CreateCluster(t *testing.T) {
    // Arrange (准备)
    mockManager := &MockStatefulSetManager{}
    service := NewClusterService(mockManager, ...)
    cluster := createTestCluster()

    // Act (执行)
    err := service.CreateCluster(ctx, cluster)

    // Assert (断言)
    assert.NoError(t, err)
    mockManager.AssertExpectations(t)
}
```

#### 3. **表驱动测试**
```go
func TestValidateClusterSize(t *testing.T) {
    tests := []struct {
        name     string
        size     int32
        wantErr  bool
        errMsg   string
    }{
        {"valid odd size", 3, false, ""},
        {"valid single node", 1, false, ""},
        {"invalid even size", 2, true, "cluster size must be odd"},
        {"invalid zero size", 0, true, "cluster size must be positive"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateClusterSize(tt.size)
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### 🔗 集成测试最佳实践

#### 1. **测试套件模式**
```go
type ClusterServiceIntegrationSuite struct {
    suite.Suite
    etcdContainer testcontainers.Container
    etcdClient    EtcdClient
    service       ClusterService
}

func (s *ClusterServiceIntegrationSuite) SetupSuite() {
    // 启动etcd容器
    s.etcdContainer = s.startEtcdContainer()
    s.etcdClient = s.createEtcdClient()
    s.service = NewClusterService(..., s.etcdClient, ...)
}

func (s *ClusterServiceIntegrationSuite) TearDownSuite() {
    s.etcdContainer.Terminate(context.Background())
}

func TestClusterServiceIntegrationSuite(t *testing.T) {
    suite.Run(t, new(ClusterServiceIntegrationSuite))
}
```

#### 2. **容器管理**
```go
func (s *ClusterServiceIntegrationSuite) startEtcdContainer() testcontainers.Container {
    req := testcontainers.ContainerRequest{
        Image:        "quay.io/coreos/etcd:v3.5.21",
        ExposedPorts: []string{"2379/tcp"},
        Env: map[string]string{
            "ETCD_LISTEN_CLIENT_URLS":          "http://0.0.0.0:2379",
            "ETCD_ADVERTISE_CLIENT_URLS":       "http://localhost:2379",
            "ETCD_LISTEN_PEER_URLS":           "http://0.0.0.0:2380",
            "ETCD_INITIAL_ADVERTISE_PEER_URLS": "http://localhost:2380",
            "ETCD_INITIAL_CLUSTER":            "etcd0=http://localhost:2380",
            "ETCD_NAME":                       "etcd0",
        },
        WaitingFor: wait.ForLog("ready to serve client requests"),
    }

    container, err := testcontainers.GenericContainer(context.Background(),
        testcontainers.GenericContainerRequest{
            ContainerRequest: req,
            Started:          true,
        })
    s.Require().NoError(err)
    return container
}
```

### 🌍 E2E测试最佳实践

#### 1. **测试环境管理**
```go
var _ = BeforeSuite(func() {
    // 确保Kind集群可用
    ensureKindCluster()

    // 部署Operator
    deployOperator()

    // 等待Operator就绪
    waitForOperatorReady()
})

var _ = AfterSuite(func() {
    // 清理测试资源
    cleanupTestResources()
})
```

#### 2. **资源清理策略**
```go
func cleanupTestResources() {
    // 删除所有测试命名空间
    namespaces := getTestNamespaces()
    for _, ns := range namespaces {
        deleteNamespace(ns)
    }

    // 等待资源完全清理
    waitForResourceCleanup()
}

func createTestNamespace() string {
    namespace := fmt.Sprintf("test-%s", generateRandomString(8))
    createNamespace(namespace)

    // 注册清理函数
    DeferCleanup(func() {
        deleteNamespace(namespace)
    })

    return namespace
}
```

## 🚨 常见问题和解决方案

### 1. **测试执行慢**
```
问题: 集成测试和E2E测试执行时间过长
解决方案:
├── 并行执行测试
├── 使用测试缓存
├── 优化容器启动时间
└── 合理设置超时时间
```

### 2. **测试不稳定**
```
问题: 测试结果不一致，偶尔失败
解决方案:
├── 增加适当的等待时间
├── 使用Eventually而不是固定等待
├── 确保测试间的隔离性
└── 避免硬编码的时间依赖
```

### 3. **Mock对象复杂**
```
问题: Mock对象设置复杂，维护困难
解决方案:
├── 使用接口而不是具体类型
├── 创建测试工厂函数
├── 使用Builder模式创建测试数据
└── 抽取通用的Mock设置
```

### 4. **测试数据管理**
```
问题: 测试数据准备复杂，重复代码多
解决方案:
├── 创建测试数据工厂
├── 使用测试夹具(fixtures)
├── 参数化测试数据
└── 共享通用的测试工具
```

---

**总结**: 三层测试架构通过合理的工具选择和分层设计，既保证了测试的全面性，又控制了测试的执行成本。结合你现有的Docker和Kind环境，可以快速建立起完整的测试体系，为项目重构提供可靠的质量保障。
