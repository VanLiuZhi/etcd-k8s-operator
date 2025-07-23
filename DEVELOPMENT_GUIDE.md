# ETCD Operator 开发指南

## 🚀 快速开始

### 环境要求
- **Go**: 1.22.3+
- **Docker**: 20.10+
- **Kubernetes**: 1.22+ (推荐使用 Kind)
- **Kubebuilder**: 4.0.0+

### 项目设置
```bash
# 1. 克隆项目
git clone <repository-url>
cd etcd-k8s-operator

# 2. 安装依赖
go mod tidy

# 3. 安装开发工具
make install-tools

# 4. 验证环境
make verify-env
```

### 本地开发环境
```bash
# 创建 Kind 集群
make kind-create

# 安装 CRD
make install

# 运行 operator (在集群外)
make run

# 部署示例集群
kubectl apply -f config/samples/etcd_v1alpha1_etcdcluster.yaml
```

## 📁 项目结构

```
etcd-k8s-operator/
├── api/v1alpha1/              # CRD 类型定义
│   ├── etcdcluster_types.go   # EtcdCluster CRD
│   ├── etcdbackup_types.go    # EtcdBackup CRD
│   └── etcdrestore_types.go   # EtcdRestore CRD
├── internal/controller/       # 控制器实现
│   ├── etcdcluster_controller.go
│   ├── etcdbackup_controller.go
│   └── etcdrestore_controller.go
├── pkg/                       # 业务逻辑包
│   ├── etcd/                  # etcd 客户端和工具
│   ├── k8s/                   # Kubernetes 工具
│   └── utils/                 # 通用工具
├── config/                    # Kubernetes 配置
│   ├── crd/                   # CRD 定义
│   ├── samples/               # 示例资源
│   └── rbac/                  # RBAC 配置
├── test/                      # 测试代码
│   ├── e2e/                   # 端到端测试
│   ├── integration/           # 集成测试
│   └── utils/                 # 测试工具
└── docs/                      # 文档
```

## 🔧 开发工作流

### 1. 功能开发流程
```bash
# 1. 创建功能分支
git checkout -b feature/your-feature

# 2. 开发和测试
make test
make build

# 3. 代码检查
make lint
make fmt

# 4. 集成测试
make test-integration

# 5. 提交代码
git commit -m "feat: add your feature"
git push origin feature/your-feature
```

### 2. CRD 修改流程
```bash
# 1. 修改 api/v1alpha1/*_types.go
# 2. 生成代码
make generate

# 3. 生成 manifests
make manifests

# 4. 更新 CRD
make install

# 5. 测试变更
kubectl apply -f config/samples/
```

### 3. 控制器开发流程
```bash
# 1. 实现控制器逻辑
# 2. 添加单元测试
make test

# 3. 本地测试
make run

# 4. 集成测试
make test-integration

# 5. 端到端测试
make test-e2e
```

## 📝 代码规范

### Go 代码规范
```go
// 1. 包注释
// Package controller implements the etcd cluster controller.
package controller

// 2. 结构体注释
// EtcdClusterReconciler reconciles a EtcdCluster object
type EtcdClusterReconciler struct {
    client.Client
    Scheme *runtime.Scheme
}

// 3. 函数注释
// Reconcile handles the reconciliation of EtcdCluster resources
func (r *EtcdClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 实现逻辑
}

// 4. 错误处理
if err != nil {
    return ctrl.Result{}, fmt.Errorf("failed to create StatefulSet: %w", err)
}
```

### 命名规范
- **包名**: 小写，简短，有意义 (`etcd`, `k8s`, `utils`)
- **类型名**: 大驼峰 (`EtcdCluster`, `BackupSpec`)
- **函数名**: 大驼峰 (公开) / 小驼峰 (私有)
- **变量名**: 小驼峰 (`clusterName`, `backupSize`)
- **常量名**: 大写下划线 (`DEFAULT_SIZE`, `MAX_RETRIES`)

### 错误处理规范
```go
// 1. 使用 fmt.Errorf 包装错误
return fmt.Errorf("failed to get cluster %s: %w", name, err)

// 2. 定义自定义错误类型
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation failed for field %s: %s", e.Field, e.Message)
}

// 3. 错误检查模式
if err != nil {
    log.Error(err, "Failed to reconcile cluster")
    return ctrl.Result{RequeueAfter: time.Minute}, err
}
```

## 🧪 测试指南

### 单元测试
```go
func TestEtcdClusterReconciler_Reconcile(t *testing.T) {
    tests := []struct {
        name    string
        cluster *etcdv1alpha1.EtcdCluster
        want    ctrl.Result
        wantErr bool
    }{
        {
            name: "create new cluster",
            cluster: &etcdv1alpha1.EtcdCluster{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      "test-cluster",
                    Namespace: "default",
                },
                Spec: etcdv1alpha1.EtcdClusterSpec{
                    Size: 3,
                },
            },
            want:    ctrl.Result{RequeueAfter: time.Second * 30},
            wantErr: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 测试逻辑
        })
    }
}
```

### 集成测试
```go
func TestEtcdClusterIntegration(t *testing.T) {
    // 使用 envtest 创建测试环境
    testEnv := &envtest.Environment{
        CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
    }
    
    cfg, err := testEnv.Start()
    require.NoError(t, err)
    defer testEnv.Stop()
    
    // 测试逻辑
}
```

### 端到端测试
```bash
# 运行 E2E 测试
make test-e2e

# 运行特定测试
go test ./test/e2e -v -run TestEtcdClusterLifecycle
```

## 🔍 调试指南

### 本地调试
```bash
# 1. 启用详细日志
export LOG_LEVEL=debug
make run

# 2. 使用 delve 调试器
dlv debug ./cmd/main.go

# 3. 查看资源状态
kubectl get etcdcluster -o yaml
kubectl describe etcdcluster my-cluster
kubectl logs -l app.kubernetes.io/name=etcd-k8s-operator
```

### 常见问题排查
```bash
# 1. 检查 CRD 是否正确安装
kubectl get crd etcdclusters.etcd.etcd.io

# 2. 检查 RBAC 权限
kubectl auth can-i create etcdclusters --as=system:serviceaccount:etcd-system:etcd-operator

# 3. 检查控制器日志
kubectl logs -n etcd-system deployment/etcd-operator-controller-manager

# 4. 检查事件
kubectl get events --sort-by=.metadata.creationTimestamp
```

## 📦 构建和部署

### 本地构建
```bash
# 构建二进制文件
make build

# 构建 Docker 镜像
make docker-build IMG=my-registry/etcd-operator:latest

# 推送镜像
make docker-push IMG=my-registry/etcd-operator:latest
```

### 部署到集群
```bash
# 部署到 Kind 集群
make deploy IMG=my-registry/etcd-operator:latest

# 部署到远程集群
kubectl apply -f dist/install.yaml
```

### Helm 部署
```bash
# 安装 Helm Chart
helm install etcd-operator ./deploy/helm/etcd-operator

# 升级
helm upgrade etcd-operator ./deploy/helm/etcd-operator

# 卸载
helm uninstall etcd-operator
```

## 🔄 CI/CD 流程

### GitHub Actions
```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v3
      with:
        go-version: 1.22.3
    - run: make test
    - run: make build
    - run: make test-e2e
```

### 发布流程
```bash
# 1. 创建发布分支
git checkout -b release/v0.1.0

# 2. 更新版本号
# 3. 创建 Git 标签
git tag v0.1.0

# 4. 推送标签
git push origin v0.1.0

# 5. 创建 GitHub Release
```

## 📚 有用的命令

### Makefile 目标
```bash
make help           # 显示所有可用命令
make build          # 构建项目
make test           # 运行测试
make lint           # 代码检查
make fmt            # 代码格式化
make generate       # 生成代码
make manifests      # 生成 manifests
make install        # 安装 CRD
make deploy         # 部署到集群
make undeploy       # 从集群卸载
make kind-create    # 创建 Kind 集群
make kind-delete    # 删除 Kind 集群
```

### kubectl 命令
```bash
# 查看 etcd 集群
kubectl get etcd
kubectl describe etcd my-cluster

# 查看备份
kubectl get etcdbackup
kubectl describe etcdbackup my-backup

# 查看日志
kubectl logs -l app.kubernetes.io/name=etcd-k8s-operator -f

# 端口转发
kubectl port-forward svc/my-cluster-client 2379:2379
```

---

**文档版本**: v1.0 | **最后更新**: 2025-07-21
