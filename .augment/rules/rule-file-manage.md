---
type: "always_apply"
---

# 文件管理规则

## 📁 目录结构规范

### 🚫 禁止在根目录创建文件
**严格禁止**在项目根目录下随意创建文件，除了以下预定义的标准文件：

#### ✅ 允许的根目录文件
```
etcd-k8s-operator/
├── README.md                    # 项目主要说明文档
├── Makefile                     # 构建和管理脚本
├── Dockerfile                   # 容器镜像构建文件
├── go.mod                       # Go 模块定义
├── go.sum                       # Go 依赖校验
├── PROJECT                      # Kubebuilder 项目配置
├── .gitignore                   # Git 忽略规则
├── .dockerignore               # Docker 忽略规则
└── LICENSE                      # 开源许可证
```

#### 🚫 禁止的根目录文件类型
- 测试配置文件 (如 `test-*.yaml`)
- 临时文档文件 (如 `TEMP_*.md`)
- 个人笔记文件 (如 `notes.txt`)
- 实验性脚本 (如 `experiment.sh`)
- 备份文件 (如 `*.bak`, `*.tmp`)

### 📂 标准目录结构

```
etcd-k8s-operator/
├── .augment/                    # Augment 配置和规则
│   └── rules/                   # 项目规则定义
├── api/                         # Kubernetes API 定义
│   └── v1alpha1/               # API 版本
├── bin/                         # 编译后的二进制文件
├── cmd/                         # 应用程序入口点
├── config/                      # Kubernetes 配置文件
│   ├── crd/                    # CRD 定义
│   ├── rbac/                   # RBAC 配置
│   └── samples/                # 示例配置
├── docs/                        # 项目文档
│   ├── design/                 # 技术设计文档
│   └── project-manage/         # 项目管理文档
├── hack/                        # 构建和开发脚本
├── internal/                    # 内部代码包
│   └── controller/             # 控制器实现
├── pkg/                         # 可重用的代码包
│   ├── etcd/                   # etcd 客户端封装
│   ├── k8s/                    # Kubernetes 资源管理
│   └── utils/                  # 工具函数
├── test/                        # 测试代码和配置
│   ├── e2e/                    # 端到端测试
│   ├── integration/            # 集成测试
│   └── testdata/               # 测试数据
└── deploy/                      # 部署配置和脚本
    ├── operator/               # Operator 部署配置
    └── examples/               # 部署示例
```

## 📋 文件创建规则

### 🎯 按功能分类存放

#### 📄 文档文件
- **位置**: `docs/` 目录下
- **分类**:
  - `docs/design/` - 技术设计文档
  - `docs/project-manage/` - 项目管理文档
- **命名**: 使用大写字母和下划线，如 `TECHNICAL_SPECIFICATION.md`

#### 🧪 测试文件
- **位置**: `test/` 目录下
- **分类**:
  - `test/e2e/` - 端到端测试
  - `test/integration/` - 集成测试
  - `test/testdata/` - 测试数据和配置
- **命名**: 使用小写字母和连字符，如 `scaling-test.yaml`

#### ⚙️ 配置文件
- **位置**: `config/` 或 `deploy/` 目录下
- **分类**:
  - `config/samples/` - CRD 示例配置
  - `deploy/examples/` - 部署示例
- **命名**: 使用小写字母和连字符，如 `etcd-cluster-sample.yaml`

#### 🔧 脚本文件
- **位置**: `hack/` 目录下
- **命名**: 使用小写字母和连字符，如 `setup-test-env.sh`
- **权限**: 确保可执行权限 (`chmod +x`)

#### 📦 代码文件
- **位置**: `pkg/`, `internal/`, `cmd/` 目录下
- **命名**: 遵循 Go 语言命名规范
- **包结构**: 按功能模块组织

## 🔍 文件命名规范

### 📝 文档文件命名
- **格式**: `UPPERCASE_WITH_UNDERSCORES.md`
- **示例**: 
  - `PROJECT_MASTER.md`
  - `TECHNICAL_SPECIFICATION.md`
  - `DEVELOPMENT_GUIDE.md`

### ⚙️ 配置文件命名
- **格式**: `lowercase-with-hyphens.yaml`
- **示例**:
  - `etcd-cluster-sample.yaml`
  - `test-scaling-scenarios.yaml`
  - `rbac-config.yaml`

### 🧪 测试文件命名
- **Go 测试**: `*_test.go`
- **测试配置**: `test-*.yaml`
- **测试数据**: `testdata-*.json`

### 🔧 脚本文件命名
- **格式**: `lowercase-with-hyphens.sh`
- **示例**:
  - `setup-test-env.sh`
  - `build-and-deploy.sh`
  - `cleanup-resources.sh`

## ✅ 文件创建检查清单

在创建新文件之前，请检查：

### 📍 位置检查
- [ ] 文件是否放在正确的目录下？
- [ ] 是否避免了在根目录创建文件？
- [ ] 目录结构是否符合项目规范？

### 📛 命名检查
- [ ] 文件名是否符合命名规范？
- [ ] 文件名是否具有描述性？
- [ ] 是否避免了特殊字符和空格？

### 🎯 功能检查
- [ ] 文件用途是否明确？
- [ ] 是否有重复功能的文件？
- [ ] 文件是否需要版本控制？

### 📚 文档检查
- [ ] 是否需要更新相关文档？
- [ ] 是否需要添加使用说明？
- [ ] 是否需要更新 README.md？

## 🚨 违规处理

### ⚠️ 常见违规行为
1. **根目录文件污染**: 在根目录创建非标准文件
2. **命名不规范**: 使用不符合规范的文件名
3. **目录混乱**: 文件放置在错误的目录
4. **重复文件**: 创建功能重复的文件

### 🔧 修复措施
1. **立即移动**: 将违规文件移动到正确位置
2. **重命名**: 修正不规范的文件名
3. **清理重复**: 删除或合并重复文件
4. **更新引用**: 修正所有相关引用

