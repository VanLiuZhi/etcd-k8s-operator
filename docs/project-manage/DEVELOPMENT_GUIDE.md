# ETCD Operator 开发指南

## 🛠️ 技术栈

- **Go**: 1.23.4+
- **Docker**: 20.10+
- **Kubernetes**: 1.28+ (推荐使用 Kind)
- **Kubebuilder**: 4.0.0+
- **ETCD**: v3.5.21

## 🚀 快速开始

### 环境设置
```bash
# 1. 克隆项目
git clone <repository-url>
cd etcd-k8s-operator

# 2. 安装依赖
go mod tidy

# 3. 生成代码
make generate manifests
```

### 本地开发
```bash
# 安装 CRD
make install

# 运行控制器
make run

# 部署示例
kubectl apply -f config/samples/etcd_v1alpha1_etcdcluster.yaml
```

## 📁 项目结构

```
etcd-k8s-operator/
├── api/v1alpha1/              # API定义 (k8s.etcd.lz)
│   └── etcdcluster_types.go   # EtcdCluster CRD
├── internal/controller/       # 控制器实现
│   └── etcdcluster_controller.go
├── pkg/                       # 可重用代码包
├── config/                    # Kubernetes配置
├── docs/                      # 项目文档
└── test/                      # 测试代码
```

## 🔧 开发流程

### 代码生成
```bash
# 生成deepcopy代码
make generate

# 生成CRD manifests
make manifests
```

### 测试
```bash
# 单元测试
make test

# 集成测试
make test-integration

# 端到端测试
make test-e2e
```

### 构建部署
```bash
# 构建镜像
make docker-build

# 部署到集群
make deploy

# 清理资源
make undeploy
```

## 📋 API说明

### EtcdCluster CRD
- **API Group**: `k8s.etcd.lz/v1alpha1`
- **功能**: etcd集群生命周期管理
- **核心字段**: size, version, repository, pod

### 集群状态
- `None`: 初始状态
- `Creating`: 创建中
- `Running`: 运行中
- `Failed`: 失败状态

## 🧪 测试部署

### 创建测试集群
```bash
# 应用CRD
kubectl apply -f config/crd/bases/k8s.etcd.lz_etcdclusters.yaml

# 创建示例集群
kubectl apply -f config/samples/etcd_v1alpha1_etcdcluster.yaml

# 查看状态
kubectl get etcdcluster -o wide
```

### 验证功能
```bash
# 查看Pod状态
kubectl get pods -l etcd_cluster=etcdcluster-sample

# 查看日志
kubectl logs -l etcd_cluster=etcdcluster-sample

# 删除集群
kubectl delete etcdcluster etcdcluster-sample
```
