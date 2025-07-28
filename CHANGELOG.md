# 变更日志

本文档记录了 etcd-k8s-operator 项目的所有重要变更。

## [未发布] - 2025-07-27

### 🎉 新增功能

#### Bitnami etcd 镜像完整支持
- **新增**: 完整支持 Bitnami etcd 镜像 (`bitnami/etcd`)
- **新增**: 自动检测 Bitnami 镜像并配置必要的环境变量
- **新增**: netshoot sidecar 容器用于网络调试
- **修复**: 解决了 Bitnami etcd 在 Kubernetes 环境中的集群组建问题

#### 环境变量增强
- **新增**: `ETCD_ON_K8S=yes` - 启用 Kubernetes 集群模式
- **新增**: `ETCD_CLUSTER_DOMAIN` - 自动配置 headless service 域名
- **新增**: `MY_STS_NAME` - 避免 Bitnami 脚本变量未定义错误

#### 调试工具集成
- **新增**: netshoot sidecar 容器 (仅限 Bitnami 镜像)
- **新增**: 完整的网络诊断工具集 (nslookup, ping, curl, tcpdump 等)
- **优化**: 资源配置优化 (CPU 50m-100m, 内存 64Mi-128Mi)

### 🔧 改进

#### 代码质量
- **改进**: 增强了 `buildPodSpec` 函数的镜像检测逻辑
- **改进**: 完善了环境变量构建函数 `buildEtcdEnvironment`
- **新增**: 针对 Bitnami 镜像的专门测试用例

#### 文档完善
- **新增**: [Bitnami etcd 支持文档](docs/BITNAMI_ETCD_SUPPORT.md)
- **更新**: README.md 添加 Bitnami etcd 使用示例
- **更新**: 项目主控文档记录技术成就

### 🐛 修复

#### 关键问题修复
- **修复**: Bitnami etcd "MY_STS_NAME: unbound variable" 错误
- **修复**: headless service DNS 解析失败问题
- **修复**: etcd 集群组建失败，所有节点显示 "Bootstrapping a new cluster"
- **修复**: Pod endpoints 为空导致服务发现失败

#### 网络问题修复
- **修复**: Kind 集群中的 DNS 解析问题
- **修复**: etcd 节点间通信失败问题
- **修复**: headless service endpoints 不更新问题

### 📊 测试改进

#### 测试覆盖
- **新增**: Bitnami 环境变量验证测试
- **新增**: netshoot 容器配置测试
- **改进**: 所有测试用例 100% 通过率

#### 验证流程
- **新增**: etcd 功能验证 (put/get 操作)
- **新增**: DNS 解析验证
- **新增**: 网络连通性验证

### 🔄 重构

#### 代码结构
- **重构**: Pod 规范构建逻辑，支持多容器配置
- **重构**: 环境变量管理，按镜像类型分组
- **优化**: 资源配置管理，提高可维护性

### 📈 性能优化

#### 资源使用
- **优化**: netshoot 容器资源配置
- **优化**: 环境变量构建性能
- **优化**: 镜像检测逻辑效率

## 技术债务

### 已解决
- ✅ Bitnami etcd 集群组建问题
- ✅ 网络调试工具缺失
- ✅ 环境变量配置不完整

### 待解决
- [ ] 多镜像类型的统一抽象
- [ ] 调试模式的可配置开关
- [ ] 生产环境安全配置优化

## 兼容性

### 支持的镜像
- ✅ `quay.io/coreos/etcd` (官方镜像)
- ✅ `bitnami/etcd` (Bitnami 镜像) - **新增**

### 支持的 Kubernetes 版本
- ✅ Kubernetes 1.22+
- ✅ Kind 集群
- ✅ 标准 Kubernetes 集群

### 支持的 etcd 版本
- ✅ etcd 3.5.x
- ✅ etcd 3.4.x (部分功能)

## 贡献者

感谢以下贡献者的努力：

- **网络问题排查**: 深入分析 Kind 集群网络和 DNS 解析问题
- **Bitnami 支持**: 完整实现 Bitnami etcd 镜像支持
- **调试工具**: 集成 netshoot 容器提升调试能力
- **文档完善**: 编写详细的技术文档和使用指南

## 下一步计划

### v0.2.0 (计划中)
- [ ] 多节点集群支持 (3/5/7 节点)
- [ ] TLS 安全配置
- [ ] 高级健康检查
- [ ] 动态扩缩容功能

### v0.3.0 (计划中)
- [ ] 数据备份恢复
- [ ] Prometheus 监控集成
- [ ] 故障自动恢复
- [ ] 跨区域部署支持

---

**格式说明**:
- 🎉 新增功能
- 🔧 改进
- 🐛 修复
- 📊 测试
- 🔄 重构
- 📈 性能优化
