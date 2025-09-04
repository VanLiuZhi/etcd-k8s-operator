# ETCD Operator 测试工作流

## 概述

本文档提供ETCD Operator的完整测试流程，重点验证etcd集群的正确性和成员状态。

## 测试目标

- 验证Operator部署和功能正常
- 确保etcd集群成员状态正确
- 验证集群内部通信和数据一致性
- 测试扩缩容和故障恢复

## 快速开始

### 1. 环境清理

```bash
# 清理现有资源
kubectl delete etcdcluster --all --all-namespaces
make uninstall
make undeploy
```

### 1.5. 离线测试准备（可选）

```bash
# 检查本地是否有所需基础镜像
docker images | grep -E "(golang:1.23.4|gcr.io/distroless/static:nonroot|quay.io/coreos/etcd:v3.5.21)"

# 如果需要离线测试，可以预先拉取镜像
docker pull golang:1.23.4
docker pull gcr.io/distroless/static:nonroot
docker pull quay.io/coreos/etcd:v3.5.21

# 验证镜像缓存
docker images | grep -E "(golang:1.23.4|gcr.io/distroless/static:nonroot|quay.io/coreos/etcd:v3.5.21)"
```

### 2. 部署Operator

```bash
# 部署CRD
make install

# 构建镜像（使用make而不是直接docker）
make docker-build IMG=etcd-operator:dev

# 部署Operator
make deploy IMG=etcd-operator:dev

# 验证部署
kubectl get pods -n etcd-system
```

**优化说明**：
- 使用`make docker-build`而不是直接使用docker，确保构建一致性
- 本地Docker缓存会自动复用已存在的镜像层，避免重复下载
- 如果本地已有所需基础镜像，构建过程完全离线进行

### 2.1. 构建优化选项

```bash
# 使用本地构建（无需网络）
make build
make run

# 或者使用Docker缓存优化构建
make docker-build IMG=etcd-operator:dev

# 查看构建缓存使用情况
docker system df
```

### 3. 创建测试集群

```bash
# 创建etcd集群
kubectl apply -f config/samples/etcd_v1alpha1_etcdcluster.yaml

# 检查基础状态
kubectl get etcdcluster etcdcluster-sample
kubectl get pods -l app.kubernetes.io/name=etcd
```

## 集群状态验证

### 1. 集群成员状态检查

```bash
# 检查集群成员状态
kubectl get etcdcluster etcdcluster-sample -o jsonpath='{.status.members}'

# 验证成员数量
kubectl get etcdcluster etcdcluster-sample -o jsonpath='{.status.members.ready}'
kubectl get etcdcluster etcdcluster-sample -o jsonpath='{.status.members.total}'
```

### 2. 集群健康检查

```bash
# 获取集群服务地址
SERVICE_NAME=$(kubectl get svc -l app.kubernetes.io/name=etcd -o jsonpath='{.items[0].metadata.name}')
NAMESPACE=$(kubectl get etcdcluster etcdcluster-sample -o jsonpath='{.metadata.namespace}')

# 检查集群健康状态
kubectl exec -it <etcd-pod-name> -- etcdctl endpoint health \
  --endpoints=http://localhost:2379

# 检查集群成员列表
kubectl exec -it <etcd-pod-name> -- etcdctl member list \
  --endpoints=http://localhost:2379
```

### 3. 数据一致性检查

```bash
# 在一个节点写入测试数据
kubectl exec -it <etcd-pod-name> -- etcdctl put /test/key "hello-world"

# 在其他节点验证数据
kubectl exec -it <etcd-pod-name-2> -- etcdctl get /test/key
kubectl exec -it <etcd-pod-name-3> -- etcdctl get /test/key

# 检查集群状态
kubectl exec -it <etcd-pod-name> -- etcdctl endpoint status \
  --endpoints=http://localhost:2379 --write-out=table
```

### 4. 集群通信验证

```bash
# 检查所有节点的endpoint状态
kubectl exec -it <etcd-pod-name> -- etcdctl endpoint status \
  --endpoints=http://localhost:2379,http://<pod-ip>:2379,http://<pod-ip>:2379 \
  --write-out=table

# 验证leader选举
kubectl exec -it <etcd-pod-name> -- etcdctl endpoint status \
  --endpoints=http://localhost:2379 --write-out=table | grep "isLeader"
```

## 扩缩容测试

### 1. 扩容测试

```bash
# 扩容到5个节点
kubectl patch etcdcluster etcdcluster-sample --type='merge' -p '{"spec":{"size":5}}'

# 观察新节点加入
kubectl get pods -l app.kubernetes.io/name=etcd -w

# 验证新成员状态
kubectl get etcdcluster etcdcluster-sample -o jsonpath='{.status.members}'

# 检查集群健康状态
kubectl exec -it <etcd-pod-name> -- etcdctl endpoint health \
  --endpoints=http://localhost:2379
```

### 2. 缩容测试

```bash
# 缩容到3个节点
kubectl patch etcdcluster etcdcluster-sample --type='merge' -p '{"spec":{"size":3}}'

# 观察节点移除
kubectl get pods -l app.kubernetes.io/name=etcd -w

# 验证成员状态
kubectl get etcdcluster etcdcluster-sample -o jsonpath='{.status.members}'

# 检查数据一致性
kubectl exec -it <etcd-pod-name> -- etcdctl get /test/key
```

## 故障恢复测试

### 1. 模拟节点故障

```bash
# 选择一个etcd Pod删除
kubectl delete pod <etcd-pod-name> --grace-period=0 --force

# 观察自动恢复
kubectl get pods -l app.kubernetes.io/name=etcd -w

# 检查集群状态
kubectl get etcdcluster etcdcluster-sample -o jsonpath='{.status.members}'
```

### 2. 验证恢复后功能

```bash
# 检查集群健康状态
kubectl exec -it <etcd-pod-name> -- etcdctl endpoint health \
  --endpoints=http://localhost:2379

# 验证数据完整性
kubectl exec -it <etcd-pod-name> -- etcdctl get /test/key

# 检查成员通信
kubectl exec -it <etcd-pod-name> -- etcdctl member list \
  --endpoints=http://localhost:2379
```

## 性能和稳定性测试

### 1. 压力测试

```bash
# 批量写入测试数据
for i in {1..1000}; do
  kubectl exec -it <etcd-pod-name> -- etcdctl put /test/key$i "value$i"
done

# 验证数据一致性
kubectl exec -it <etcd-pod-name> -- etcdctl get /test/key1 --prefix
kubectl exec -it <etcd-pod-name-2> -- etcdctl get /test/key1 --prefix
```

### 2. 长时间稳定性测试

```bash
# 监控集群状态
watch -n 5 'kubectl get etcdcluster etcdcluster-sample -o jsonpath="{.status.members.ready}/{.status.members.total}"'

# 检查资源使用情况
kubectl top pods -l app.kubernetes.io/name=etcd
```

## 清理环境

### 1. 删除测试资源

```bash
# 删除etcd集群
kubectl delete etcdcluster etcdcluster-sample

# 等待清理完成
kubectl get pods -l app.kubernetes.io/name=etcd
```

### 2. 卸载Operator

```bash
# 卸载Operator
make undeploy

# 卸载CRD
make uninstall

# 验证清理
kubectl get crd | grep etcd
```

## 验证检查清单

### 部署成功标准

- [ ] Operator Pod运行正常
- [ ] CRD安装成功
- [ ] etcd集群创建成功
- [ ] 所有Pod处于Running状态

### 集群健康标准

- [ ] 集群成员数量正确
- [ ] 所有成员状态为Ready
- [ ] 集群健康检查通过
- [ ] 数据一致性验证通过
- [ ] 成员间通信正常

### 扩缩容标准

- [ ] 扩容后成员数量正确
- [ ] 新成员加入集群成功
- [ ] 缩容后成员数量正确
- [ ] 数据保持一致性

### 故障恢复标准

- [ ] 故障节点自动恢复
- [ ] 集群状态保持健康
- [ ] 数据完整性保证
- [ ] 成员通信正常

## 故障排查

### 常用命令

```bash
# 查看Operator日志
kubectl logs -n etcd-system -l control-plane=controller-manager

# 查看etcd Pod日志
kubectl logs -l app.kubernetes.io/name=etcd

# 查看集群事件
kubectl describe etcdcluster etcdcluster-sample

# 查看Pod事件
kubectl describe pods -l app.kubernetes.io/name=etcd
```

### 常见问题

1. **集群无法形成**: 检查网络策略和Pod间通信
2. **成员状态异常**: 查看etcd日志确认选举状态
3. **数据不一致**: 检查quorum配置和网络延迟
4. **扩缩容失败**: 验证存储卷和资源限制
5. **镜像构建失败**: 
   - 检查本地是否有所需基础镜像：`docker images | grep -E "(golang:1.23.4|gcr.io/distroless/static:nonroot)"`
   - 如果本地没有镜像且网络有问题，请停止，告诉我，由我来解决
   - 确保Docker守护进程正在运行：`docker info`

### 离线测试优势

- **无需网络依赖**：所有镜像都在本地缓存中
- **快速构建**：Docker层缓存机制大大提高构建速度
- **一致的环境**：确保每次测试都在相同的环境中进行
- **节省带宽**：避免重复下载相同的基础镜像

---

**文档版本**: v2.0  
**创建时间**: 2025-09-03  
**适用范围**: ETCD Operator 重构版本