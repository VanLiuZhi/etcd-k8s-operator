# etcd-operator CRD 功能分析报告

## 项目架构概述

etcd-operator 项目确实不是通过 kubebuilder 构建的，而是采用了早期的 Kubernetes Operator 开发模式，使用原生的 client-go 和 code-generator 工具。该项目包含三个独立的 operator：

1. **etcd-operator** - 主要的集群管理器
2. **etcd-backup-operator** - 备份管理器  
3. **etcd-restore-operator** - 恢复管理器

## CRD 定义分析

### 1. EtcdCluster (主要资源)

**API 版本**: `etcd.database.coreos.com/v1beta2`  
**Kind**: `EtcdCluster`  
**复数形式**: `etcdclusters`

#### 核心功能字段

```yaml
apiVersion: "etcd.database.coreos.com/v1beta2"
kind: "EtcdCluster"
metadata:
  name: "example-etcd-cluster"
spec:
  # 基础配置
  size: 3                           # 集群大小 (1-7)
  version: "3.2.13"                 # etcd版本
  repository: "quay.io/coreos/etcd" # 镜像仓库
  paused: false                     # 暂停控制
  
  # Pod策略配置
  pod:
    labels: {}                      # 自定义标签
    annotations: {}                 # 自定义注解
    nodeSelector: {}                # 节点选择器
    tolerations: []                 # 污点容忍
    affinity: {}                    # 亲和性规则
    resources: {}                   # 资源限制
    securityContext: {}             # 安全上下文
    etcdEnv: []                     # etcd环境变量
    persistentVolumeClaimSpec: {}   # PVC规格
    DNSTimeoutInSecond: 0           # DNS超时
    clusterDomain: ".cluster.local" # 集群域名
    
  # TLS安全配置
  TLS:
    static:
      member:
        peerSecret: "etcd-peer-tls"     # peer TLS密钥
        serverSecret: "etcd-server-tls" # server TLS密钥
      operatorSecret: "etcd-client-tls" # operator客户端TLS密钥
```

#### 状态字段

```yaml
status:
  phase: "Running"              # 集群阶段: Creating/Running/Failed
  reason: ""                    # 失败原因
  controlPaused: false          # 控制是否暂停
  size: 3                       # 当前大小
  serviceName: ""               # 服务名称
  clientPort: 2379              # 客户端端口
  currentVersion: "3.2.13"      # 当前版本
  targetVersion: ""             # 目标版本(升级时)
  
  # 条件状态
  conditions:
    - type: "Available"         # 可用状态
      status: "True"
      lastUpdateTime: ""
      lastTransitionTime: ""
      reason: "Cluster available"
      message: ""
      
  # 成员状态
  members:
    ready: ["pod1", "pod2", "pod3"]     # 就绪成员
    unready: []                         # 未就绪成员
```

### 2. EtcdBackup (备份资源)

**API 版本**: `etcd.database.coreos.com/v1beta2`  
**Kind**: `EtcdBackup`  
**复数形式**: `etcdbackups`

#### 功能特性

```yaml
apiVersion: "etcd.database.coreos.com/v1beta2"
kind: "EtcdBackup"
metadata:
  name: "example-backup"
spec:
  # etcd连接配置
  etcdEndpoints: ["http://etcd-cluster:2379"]
  clientTLSSecret: "etcd-client-tls"    # TLS客户端密钥
  
  # 存储类型 (支持4种)
  storageType: "S3"  # S3/ABS/GCS/OSS
  
  # 备份策略
  backupPolicy:
    timeoutInSecond: 60               # 备份超时
    backupIntervalInSecond: 3600      # 备份间隔 (0=一次性)
    maxBackups: 5                     # 最大备份数
    
  # 存储源配置 (根据storageType选择)
  s3:
    path: "mybucket/etcd.backup"      # S3路径
    awsSecret: "aws-credentials"      # AWS凭证
    endpoint: ""                      # 自定义端点
    forcePathStyle: false             # 强制路径样式
    
  abs:
    path: "container/etcd.backup"     # Azure路径
    absSecret: "azure-credentials"    # Azure凭证
    
  gcs:
    path: "bucket/etcd.backup"        # GCS路径
    gcpSecret: "gcp-credentials"      # GCP凭证
    
  oss:
    path: "bucket/etcd.backup"        # OSS路径
    ossSecret: "oss-credentials"      # OSS凭证
    endpoint: "http://oss-cn-hangzhou.aliyuncs.com"
```

#### 状态字段

```yaml
status:
  succeeded: true                     # 备份是否成功
  reason: ""                          # 失败原因
  etcdVersion: "3.2.13"              # etcd版本
  etcdRevision: 12345                 # etcd修订版本
  lastSuccessDate: "2023-01-01T00:00:00Z"  # 最后成功时间
```

### 3. EtcdRestore (恢复资源)

**API 版本**: `etcd.database.coreos.com/v1beta2`  
**Kind**: `EtcdRestore`  
**复数形式**: `etcdrestores`

#### 功能特性

```yaml
apiVersion: "etcd.database.coreos.com/v1beta2"
kind: "EtcdRestore"
metadata:
  name: "example-etcd-cluster"  # 必须与目标集群名相同
spec:
  # 目标集群引用
  etcdCluster:
    name: "example-etcd-cluster"      # 要恢复的集群名
    
  # 备份存储类型
  backupStorageType: "S3"             # S3/ABS/GCS/OSS
  
  # 恢复源配置 (与备份配置类似)
  s3:
    path: "mybucket/etcd.backup"
    awsSecret: "aws-credentials"
    endpoint: ""
    forcePathStyle: false
    
  abs:
    path: "container/etcd.backup"
    absSecret: "azure-credentials"
    
  gcs:
    path: "bucket/etcd.backup"
    gcpSecret: "gcp-credentials"
    
  oss:
    path: "bucket/etcd.backup"
    ossSecret: "oss-credentials"
    endpoint: "http://oss-cn-hangzhou.aliyuncs.com"
```

#### 状态字段

```yaml
status:
  phase: "Completed"                  # 恢复阶段
  reason: ""                          # 失败原因
```

## 支持的存储后端

### 1. Amazon S3
- **标识**: `BackupStorageTypeS3`
- **配置**: AWS凭证文件 (credentials/config)
- **特性**: 支持自定义端点、路径样式

### 2. Azure Blob Storage (ABS)
- **标识**: `BackupStorageTypeABS`
- **配置**: 存储账户和密钥
- **特性**: 支持多云环境

### 3. Google Cloud Storage (GCS)
- **标识**: `BackupStorageTypeGCS`
- **配置**: 访问令牌或JSON凭证
- **特性**: 支持默认应用凭证

### 4. 阿里云对象存储 (OSS)
- **标识**: `BackupStorageTypeOSS`
- **配置**: AccessKey ID/Secret
- **特性**: 支持自定义端点

## 核心功能总结

### 1. 集群生命周期管理
- ✅ **创建集群**: 自动创建etcd集群
- ✅ **扩缩容**: 动态调整集群大小 (1-7节点)
- ✅ **版本升级**: 滚动升级etcd版本
- ✅ **暂停控制**: 临时停止operator控制
- ✅ **集群删除**: 清理所有相关资源

### 2. 高可用和安全
- ✅ **TLS加密**: 支持peer和client TLS
- ✅ **静态证书**: 使用预生成的证书
- ✅ **网络策略**: 节点选择器和亲和性
- ✅ **资源管理**: CPU/内存限制和请求
- ✅ **安全上下文**: Pod安全策略

### 3. 存储和持久化
- ✅ **持久化卷**: 支持PVC持久化存储
- ✅ **存储类**: 自定义存储类
- ✅ **数据目录**: 可配置数据目录

### 4. 备份和恢复
- ✅ **多存储后端**: S3/ABS/GCS/OSS
- ✅ **定期备份**: 可配置备份间隔
- ✅ **备份保留**: 自动清理旧备份
- ✅ **灾难恢复**: 从备份完全恢复集群
- ✅ **跨集群恢复**: 从一个集群恢复到另一个集群

### 5. 监控和可观测性
- ✅ **状态报告**: 详细的集群状态
- ✅ **条件跟踪**: 扩缩容/升级/恢复状态
- ✅ **事件记录**: Kubernetes事件集成
- ✅ **健康检查**: etcd健康状态监控
- ✅ **成员状态**: 个别成员就绪状态

### 6. 运维友好特性
- ✅ **自定义配置**: etcd环境变量配置
- ✅ **Pod策略**: 标签、注解、容忍度
- ✅ **DNS配置**: 自定义集群域名
- ✅ **镜像管理**: 自定义镜像仓库
- ✅ **资源控制**: 细粒度资源配置

## 架构优势

1. **模块化设计**: 三个独立operator，职责清晰
2. **存储灵活性**: 支持4种主流云存储
3. **安全完备**: 全面的TLS和安全配置
4. **运维友好**: 丰富的配置选项和状态报告
5. **云原生**: 完全基于Kubernetes原语

## 局限性

1. **版本较老**: 基于v1beta2 API，未使用最新的Operator SDK
2. **复杂性**: 三个operator需要分别部署和管理
3. **文档**: 相比kubebuilder项目，文档和工具支持较少
4. **维护状态**: 项目已不再积极维护

这个分析为您提供了etcd-operator的完整功能图谱，可以作为开发新operator的重要参考。
