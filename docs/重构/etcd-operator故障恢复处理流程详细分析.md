# etcd-operator 故障恢复处理流程详细分析

## 项目概述

etcd-operator 具备完善的故障检测和恢复机制，能够处理从单个成员故障到整个集群灾难性故障的各种场景。本文档详细分析其故障恢复的实现机制和处理流程。

## 核心架构组件

### 1. 故障检测层 (pkg/cluster/)
- **cluster.go**: 主协调循环，负责故障检测和状态监控
- **reconcile.go**: 故障处理逻辑，实现成员恢复算法
- **member.go**: 成员状态管理和同步

### 2. 备份恢复层 (pkg/controller/backup-operator/, pkg/controller/restore-operator/)
- **backup-operator**: 定期备份etcd数据
- **restore-operator**: 灾难恢复时重建集群

### 3. 状态管理层 (pkg/apis/etcd/v1beta2/)
- **status.go**: 集群状态和故障条件管理

## 故障检测机制

### 1. Pod状态监控
```go
// pkg/cluster/cluster.go:232
running, pending, err := c.pollPods()
if err != nil {
    c.logger.Errorf("fail to poll pods: %v", err)
    reconcileFailed.WithLabelValues("failed to poll pods").Inc()
    continue
}

// 检查所有Pod是否死亡
if len(running) == 0 {
    c.logger.Warningf("all etcd pods are dead.")
    break
}
```

**检测内容：**
- Pod运行状态 (Running/Pending/Failed)
- Pod删除时间戳检查
- Pod所有者引用验证

### 2. etcd成员状态同步
```go
// pkg/cluster/member.go:28
func (c *Cluster) updateMembers(known etcdutil.MemberSet) error {
    resp, err := etcdutil.ListMembers(known.ClientURLs(), c.tlsConfig)
    if err != nil {
        return err
    }
    
    members := etcdutil.MemberSet{}
    for _, m := range resp.Members {
        name, err := getMemberName(m, c.cluster.GetName())
        if err != nil {
            return errors.Wrap(err, "get member name failed")
        }
        
        members[name] = &etcdutil.Member{
            Name:         name,
            Namespace:    c.cluster.Namespace,
            ID:           m.ID,
            SecurePeer:   c.isSecurePeer(),
            SecureClient: c.isSecureClient(),
        }
    }
    c.members = members
    return nil
}
```

**同步机制：**
- 从etcd集群获取实际成员列表
- 与Kubernetes Pod状态对比
- 识别不一致状态

### 3. 故障类型识别
```go
// pkg/cluster/reconcile.go:79
func (c *Cluster) reconcileMembers(running etcdutil.MemberSet) error {
    // 步骤1: 识别意外Pod
    unknownMembers := running.Diff(c.members)
    
    // 步骤2: 计算有效运行成员
    L := running.Diff(unknownMembers)
    
    // 步骤3: 检查法定人数
    if L.Size() < c.members.Size()/2+1 {
        return ErrLostQuorum
    }
    
    // 步骤4: 处理死亡成员
    return c.removeDeadMember(c.members.Diff(L).PickOne())
}
```

## 故障恢复处理流程

### 1. 单成员故障恢复

#### 故障检测流程
```
定时协调循环 (每5秒)
    ↓
pollPods() → 获取Pod状态
    ↓
updateMembers() → 同步etcd成员状态
    ↓
reconcileMembers() → 对比状态差异
    ↓
检测到死亡成员 → 触发恢复流程
```

#### 成员恢复算法
```go
// pkg/cluster/reconcile.go:99
c.logger.Infof("removing one dead member")
return c.removeDeadMember(c.members.Diff(L).PickOne())

func (c *Cluster) removeDeadMember(toRemove *etcdutil.Member) error {
    // 1. 从etcd集群移除死亡成员
    err := etcdutil.RemoveMember(c.members.ClientURLs(), c.tlsConfig, toRemove.ID)
    if err != nil {
        switch err {
        case rpctypes.ErrMemberNotFound:
            // 成员已经不在etcd集群中
        default:
            return err
        }
    }
    
    // 2. 从内存状态移除
    c.members.Remove(toRemove.Name)
    
    // 3. 删除Kubernetes资源
    if err := c.removePod(toRemove.Name); err != nil {
        return err
    }
    
    // 4. 清理PVC (如果启用持久化)
    if c.isPodPVEnabled() {
        err = c.removePVC(k8sutil.PVCNameFromMember(toRemove.Name))
    }
    
    return nil
}
```

#### 成员替换流程
```go
// 下一个协调循环会检测到成员数量不足
func (c *Cluster) resize() error {
    if c.members.Size() < c.cluster.Spec.Size {
        return c.addOneMember()
    }
    return nil
}

func (c *Cluster) addOneMember() error {
    // 1. 设置恢复状态
    c.status.SetScalingUpCondition(c.members.Size(), c.cluster.Spec.Size)
    
    // 2. 创建新成员
    newMember := c.newMember()
    
    // 3. 向etcd集群添加成员
    resp, err := etcdcli.MemberAdd(ctx, []string{newMember.PeerURL()})
    newMember.ID = resp.Member.ID
    c.members.Add(newMember)
    
    // 4. 创建替换Pod
    return c.createPod(c.members, newMember, "existing")
}
```

### 2. 法定人数丢失处理

#### 法定人数检查
```go
// pkg/cluster/reconcile.go:95
if L.Size() < c.members.Size()/2+1 {
    return ErrLostQuorum
}
```

**法定人数规则：**
- 3节点集群：至少需要2个节点
- 5节点集群：至少需要3个节点
- 7节点集群：至少需要4个节点

#### 法定人数丢失处理
```go
// pkg/cluster/cluster.go:272
if rerr != nil {
    reconcileFailed.WithLabelValues(rerr.Error()).Inc()
}

if isFatalError(rerr) {
    c.status.SetReason(rerr.Error())
    c.logger.Errorf("cluster failed: %v", rerr)
    c.reportFailedStatus()
    return
}
```

**处理策略：**
- 停止所有自动恢复操作
- 设置集群状态为Failed
- 等待人工干预或灾难恢复

### 3. 灾难恢复机制

#### 灾难恢复触发条件
- 所有etcd Pod死亡
- 法定人数永久丢失
- 数据损坏无法启动

#### 备份创建流程
```go
// pkg/controller/backup-operator/s3_backup.go:52
func (bm *BackupManager) SaveSnap(ctx context.Context, s3Path string) (int64, string, error) {
    // 1. 选择最高版本的etcd成员
    etcdcli, rev, err := bm.etcdClientWithMaxRevision(ctx)
    if err != nil {
        return 0, "", fmt.Errorf("create etcd client failed: %v", err)
    }
    defer etcdcli.Close()
    
    // 2. 获取etcd版本信息
    resp, err := etcdcli.Status(ctx, etcdcli.Endpoints()[0])
    if err != nil {
        return 0, "", fmt.Errorf("failed to retrieve etcd version: %v", err)
    }
    
    // 3. 创建快照
    rc, err := etcdcli.Snapshot(ctx)
    if err != nil {
        return 0, "", fmt.Errorf("failed to receive snapshot: %v", err)
    }
    
    // 4. 保存到存储
    return bm.writer.Write(ctx, s3Path, rc)
}
```

#### 灾难恢复流程
```go
// pkg/controller/restore-operator/sync.go:129
func (r *Restore) prepareSeed(er *api.EtcdRestore) error {
    // 1. 获取原集群配置
    ecRef := er.Spec.EtcdCluster
    ec, err := r.etcdCRCli.EtcdV1beta2().EtcdClusters(r.namespace).Get(ecRef.Name, metav1.GetOptions{})
    
    // 2. 删除原集群资源
    err = r.etcdCRCli.EtcdV1beta2().EtcdClusters(r.namespace).Delete(ecRef.Name, &metav1.DeleteOptions{})
    r.deleteClusterResourcesCompletely(ecRef.Name)
    
    // 3. 创建新集群CR (暂停状态)
    ec = &api.EtcdCluster{
        ObjectMeta: metav1.ObjectMeta{
            Name:            clusterName,
            Labels:          ec.ObjectMeta.Labels,
            Annotations:     ec.ObjectMeta.Annotations,
            OwnerReferences: ec.ObjectMeta.OwnerReferences,
        },
        Spec: ec.Spec,
    }
    ec.Spec.Paused = true
    ec.Status.Phase = api.ClusterPhaseRunning
    
    // 4. 创建种子成员
    err = r.createSeedMember(ec, r.mySvcAddr, clusterName, ec.AsOwner())
    
    // 5. 启用集群管理
    ec.Spec.Paused = false
    _, err = r.etcdCRCli.EtcdV1beta2().EtcdClusters(r.namespace).Update(ec)
    
    return nil
}
```

#### 种子成员创建
```go
// pkg/controller/restore-operator/sync.go:213
func (r *Restore) createSeedMember(ec *api.EtcdCluster, svcAddr, clusterName string, owner metav1.OwnerReference) error {
    // 1. 创建成员定义
    m := &etcdutil.Member{
        Name:         k8sutil.UniqueMemberName(clusterName),
        Namespace:    r.namespace,
        SecurePeer:   ec.Spec.TLS.IsSecurePeer(),
        SecureClient: ec.Spec.TLS.IsSecureClient(),
    }
    
    // 2. 构建备份URL
    ms := etcdutil.NewMemberSet(m)
    backupURL := backupapi.BackupURLForRestore("http", svcAddr, clusterName)
    
    // 3. 创建种子Pod (包含恢复初始化容器)
    ec.SetDefaults()
    pod := k8sutil.NewSeedMemberPod(clusterName, ms, m, ec.Spec, owner, backupURL)
    
    // 4. 部署种子Pod
    _, err := r.kubecli.Core().Pods(r.namespace).Create(pod)
    return err
}
```

## 故障恢复状态管理

### 1. 集群状态条件
```go
// pkg/apis/etcd/v1beta2/status.go
const (
    ClusterConditionAvailable  = "Available"   // 集群正常可用
    ClusterConditionRecovering = "Recovering"  // 故障恢复中
    ClusterConditionScaling    = "Scaling"     // 成员调整中
    ClusterConditionUpgrading  = "Upgrading"   // 版本升级中
)
```

### 2. 恢复状态设置
```go
// 设置恢复状态
func (cs *ClusterStatus) SetRecoveringCondition(reason string) {
    c := newClusterCondition(ClusterConditionRecovering, v1.ConditionTrue, 
                           "Cluster recovering", reason)
    cs.setClusterCondition(*c)
}

// 清除恢复状态
func (cs *ClusterStatus) SetReadyCondition() {
    c := newClusterCondition(ClusterConditionAvailable, v1.ConditionTrue, 
                           "Cluster available", "")
    cs.setClusterCondition(*c)
}
```

### 3. 故障状态报告
```go
// pkg/cluster/cluster.go:458
func (c *Cluster) reportFailedStatus() {
    c.logger.Info("cluster failed. Reporting failed reason...")
    
    retryInterval := 5 * time.Second
    f := func() (bool, error) {
        c.status.SetPhase(api.ClusterPhaseFailed)
        err := c.updateCRStatus()
        if err == nil || k8sutil.IsKubernetesResourceNotFoundError(err) {
            return true, nil
        }
        
        if !apierrors.IsConflict(err) {
            c.logger.Warningf("retry report status in %v: fail to update: %v", retryInterval, err)
            return false, nil
        }
        
        return false, nil
    }
    
    retryutil.Retry(5*time.Minute, retryInterval, f)
}
```

## 错误处理和重试机制

### 1. 协调循环错误处理
```go
// pkg/cluster/cluster.go:259
rerr = c.reconcile(running)
if rerr != nil {
    c.logger.Errorf("failed to reconcile: %v", rerr)
    break
}

// 致命错误处理
if isFatalError(rerr) {
    c.status.SetReason(rerr.Error())
    c.logger.Errorf("cluster failed: %v", rerr)
    c.reportFailedStatus()
    return
}
```

### 2. 成员操作失败处理
```go
// 成员添加失败时的清理
if err := c.createPod(c.members, newMember, "existing"); err != nil {
    // 需要从etcd集群中移除已添加的成员
    etcdutil.RemoveMember(c.members.ClientURLs(), c.tlsConfig, newMember.ID)
    c.members.Remove(newMember.Name)
    return fmt.Errorf("fail to create member's pod (%s): %v", newMember.Name, err)
}
```

### 3. 网络分区处理
```go
// etcd客户端连接失败处理
func (c *Cluster) updateMembers(known etcdutil.MemberSet) error {
    resp, err := etcdutil.ListMembers(known.ClientURLs(), c.tlsConfig)
    if err != nil {
        // 网络分区或etcd不可用
        return err
    }
    // 继续处理...
}
```

## 故障恢复场景分析

### 场景1: 单个Pod故障
**故障现象：**
- 1个Pod处于Failed状态
- etcd集群仍有法定人数

**恢复流程：**
```
检测Pod故障
    ↓
removeDeadMember() → 从etcd移除死亡成员
    ↓
removePod() → 删除失败的Pod
    ↓
下一个协调循环检测成员不足
    ↓
addOneMember() → 创建新成员
    ↓
createPod() → 部署新Pod
    ↓
等待Pod就绪 → 恢复完成
```

### 场景2: 网络分区故障
**故障现象：**
- Pod运行正常但etcd成员列表获取失败
- 网络连接中断

**处理策略：**
- 暂停协调操作
- 等待网络恢复
- 重新同步成员状态

### 场景3: 存储故障
**故障现象：**
- PVC挂载失败
- 数据目录损坏

**恢复流程：**
- 检测存储问题
- 重新创建PVC
- 从备份恢复数据（如果需要）

### 场景4: 整集群灾难
**故障现象：**
- 所有Pod死亡
- 法定人数完全丢失

**恢复流程：**
```
创建EtcdRestore CR
    ↓
restore-operator检测恢复请求
    ↓
prepareSeed() → 准备种子成员
    ↓
deleteClusterResourcesCompletely() → 清理旧资源
    ↓
createSeedMember() → 创建种子Pod
    ↓
种子Pod从备份恢复数据
    ↓
启用集群管理 (Paused=false)
    ↓
etcd-operator接管并扩展到目标大小
```

## 监控和可观测性

### 1. Prometheus指标
```go
// pkg/cluster/cluster.go
var (
    reconcileHistogram = prometheus.NewHistogramVec(...)
    reconcileFailed = prometheus.NewCounterVec(...)
    clustersTotal = prometheus.NewGauge(...)
    clustersFailed = prometheus.NewCounter(...)
)
```

### 2. 事件记录
```go
// 故障事件记录
c.eventsCli.Create(k8sutil.MemberFailedEvent(memberName, reason, c.cluster))

// 恢复事件记录  
c.eventsCli.Create(k8sutil.MemberRecoveredEvent(memberName, c.cluster))
```

### 3. 详细日志
```go
c.logger.Errorf("failed to reconcile: %v", rerr)
c.logger.Infof("removing one dead member")
c.logger.Warningf("all etcd pods are dead.")
```

## 总结

etcd-operator 的故障恢复机制采用了多层次的设计：

1. **主动监控**: 定期检查Pod和etcd成员状态
2. **渐进恢复**: 一次只处理一个故障成员
3. **法定人数保护**: 防止操作导致集群不可用
4. **灾难恢复**: 通过备份重建整个集群
5. **状态管理**: 完整的故障状态跟踪和报告
6. **可观测性**: 详细的日志、事件和指标

这种设计确保了 etcd 集群在各种故障场景下的高可用性和数据安全性。

## 详细代码执行流程

### 单成员故障恢复时序图

```
故障发生: Pod-2 崩溃
    ↓
定时协调循环 (5秒后)
    ↓
Cluster.pollPods()
├── 获取Pod列表: [Pod-1:Running, Pod-3:Running]
├── 检查OwnerReference
└── 返回: running=[Pod-1, Pod-3], pending=[]
    ↓
Cluster.updateMembers(podsToMemberSet(running))
├── 调用etcdutil.ListMembers()
├── 从etcd获取: [Member-1, Member-2, Member-3]
└── 更新c.members = {Member-1, Member-2, Member-3}
    ↓
Cluster.reconcile(running)
    ↓
Cluster.reconcileMembers(running)
├── unknownMembers = running.Diff(c.members) = {}
├── L = running.Diff(unknownMembers) = {Pod-1, Pod-3}
├── L.Size() (2) != c.members.Size() (3) → 继续处理
├── L.Size() (2) >= c.members.Size()/2+1 (2) → 法定人数充足
└── 调用removeDeadMember(c.members.Diff(L).PickOne())
    ↓
Cluster.removeDeadMember(Member-2)
├── etcdutil.RemoveMember() → 从etcd集群移除Member-2
├── c.members.Remove("Member-2") → 内存状态更新
├── c.removePod("Pod-2") → 删除Kubernetes Pod
└── 清理PVC (如果启用)
    ↓
下一个协调循环 (5秒后)
    ↓
Cluster.reconcile()
├── c.members.Size() (2) < c.cluster.Spec.Size (3)
└── 调用addOneMember()
    ↓
Cluster.addOneMember()
├── c.status.SetScalingUpCondition(2, 3)
├── newMember = c.newMember() → 生成Pod-4
├── etcdcli.MemberAdd() → 向etcd添加Member-4
├── c.members.Add(newMember)
└── c.createPod() → 创建Pod-4
    ↓
等待Pod-4启动完成
    ↓
故障恢复完成: 集群恢复到3个健康成员
```

### 法定人数丢失处理流程

```
灾难场景: 3节点集群中2个Pod同时故障
    ↓
Cluster.pollPods()
└── 返回: running=[Pod-1], pending=[]
    ↓
Cluster.reconcileMembers(running)
├── L = running = {Pod-1}
├── L.Size() (1) < c.members.Size()/2+1 (2)
└── 返回 ErrLostQuorum
    ↓
协调循环错误处理
├── c.logger.Errorf("failed to reconcile: lost quorum")
├── reconcileFailed.WithLabelValues("lost quorum").Inc()
├── isFatalError(ErrLostQuorum) → true
├── c.status.SetReason("lost quorum")
├── c.status.SetPhase(api.ClusterPhaseFailed)
└── c.reportFailedStatus()
    ↓
集群标记为Failed状态
├── 停止所有自动恢复操作
├── 等待人工干预
└── 或触发灾难恢复流程
```

### 灾难恢复详细执行流程

#### 阶段1: 备份准备
```
用户创建EtcdRestore CR
    ↓
restore-operator检测到新CR
    ↓
Restore.handleCR(etcdRestore)
├── 验证CR名称与集群名称一致
├── 检查CR状态 (避免重复处理)
└── 调用prepareSeed()
```

#### 阶段2: 集群重建
```go
// pkg/controller/restore-operator/sync.go:129
Restore.prepareSeed(etcdRestore)
    ↓
步骤1: 获取原集群配置
├── ecRef := er.Spec.EtcdCluster
├── ec, err := r.etcdCRCli.EtcdV1beta2().EtcdClusters().Get(ecRef.Name)
└── 验证集群规格: ec.Spec.Validate()
    ↓
步骤2: 清理原集群资源
├── r.etcdCRCli.EtcdV1beta2().EtcdClusters().Delete(ecRef.Name)
└── r.deleteClusterResourcesCompletely(ecRef.Name)
    ↓
步骤3: 创建新集群CR
├── 复制原集群的metadata和spec
├── 设置ec.Spec.Paused = true (暂停operator管理)
├── 设置ec.Status.Phase = api.ClusterPhaseRunning
└── r.etcdCRCli.EtcdV1beta2().EtcdClusters().Create(ec)
    ↓
步骤4: 创建种子成员
├── 生成唯一成员名: k8sutil.UniqueMemberName(clusterName)
├── 构建备份URL: backupapi.BackupURLForRestore()
├── 创建种子Pod: k8sutil.NewSeedMemberPod()
└── r.kubecli.Core().Pods().Create(pod)
    ↓
步骤5: 启用集群管理
├── 重新获取集群CR
├── 设置ec.Spec.Paused = false
└── r.etcdCRCli.EtcdV1beta2().EtcdClusters().Update(ec)
```

#### 阶段3: 种子成员启动
```
种子Pod启动
    ↓
InitContainer执行恢复
├── 从备份服务下载快照
├── 使用etcdctl snapshot restore
├── 准备数据目录
└── 设置初始集群配置
    ↓
etcd主容器启动
├── 以单成员模式启动
├── 加载恢复的数据
└── 等待operator接管
    ↓
etcd-operator检测到集群
├── 发现Paused=false
├── 开始正常协调循环
└── 扩展到目标成员数量
```

## 关键工具函数详解

### 1. Pod状态轮询实现
```go
// pkg/cluster/cluster.go:394
func (c *Cluster) pollPods() (running, pending []*v1.Pod, err error) {
    // 1. 获取集群所有Pod
    podList, err := c.config.KubeCli.Core().Pods(c.cluster.Namespace).List(
        k8sutil.ClusterListOpt(c.cluster.Name))
    if err != nil {
        return nil, nil, fmt.Errorf("failed to list running pods: %v", err)
    }

    for i := range podList.Items {
        pod := &podList.Items[i]

        // 2. 跳过正在删除的Pod (避免k8s bug)
        if pod.DeletionTimestamp != nil {
            continue
        }

        // 3. 验证Pod所有者
        if len(pod.OwnerReferences) < 1 {
            c.logger.Warningf("pollPods: ignore pod %v: no owner", pod.Name)
            continue
        }
        if pod.OwnerReferences[0].UID != c.cluster.UID {
            c.logger.Warningf("pollPods: ignore pod %v: owner (%v) is not %v",
                pod.Name, pod.OwnerReferences[0].UID, c.cluster.UID)
            continue
        }

        // 4. 根据Pod状态分类
        switch pod.Status.Phase {
        case v1.PodRunning:
            running = append(running, pod)
        case v1.PodPending:
            pending = append(pending, pod)
        }
    }

    return running, pending, nil
}
```

### 2. etcd成员列表同步
```go
// pkg/util/etcdutil/etcdutil.go
func ListMembers(clientURLs []string, tlsConfig *tls.Config) (*clientv3.MemberListResponse, error) {
    cfg := clientv3.Config{
        Endpoints:   clientURLs,
        DialTimeout: constants.DefaultDialTimeout,
        TLS:         tlsConfig,
    }

    etcdcli, err := clientv3.New(cfg)
    if err != nil {
        return nil, err
    }
    defer etcdcli.Close()

    ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
    defer cancel()

    return etcdcli.MemberList(ctx)
}
```

### 3. 死亡成员移除实现
```go
// pkg/cluster/reconcile.go
func (c *Cluster) removeDeadMember(toRemove *etcdutil.Member) error {
    c.logger.Infof("removing dead member %q", toRemove.Name)

    // 1. 从etcd集群移除成员
    err := etcdutil.RemoveMember(c.members.ClientURLs(), c.tlsConfig, toRemove.ID)
    if err != nil {
        switch err {
        case rpctypes.ErrMemberNotFound:
            // 成员已经不在etcd集群中，继续清理Kubernetes资源
            c.logger.Infof("etcd member %q not found, continue cleanup", toRemove.Name)
        default:
            return err
        }
    }

    // 2. 从内存状态移除
    c.members.Remove(toRemove.Name)

    // 3. 删除Kubernetes Pod
    if err := c.removePod(toRemove.Name); err != nil {
        return err
    }

    // 4. 清理PVC (如果启用持久化存储)
    if c.isPodPVEnabled() {
        pvcName := k8sutil.PVCNameFromMember(toRemove.Name)
        err := c.removePVC(pvcName)
        if err != nil {
            c.logger.Errorf("failed to remove PVC %s: %v", pvcName, err)
            // 不返回错误，继续处理
        }
    }

    // 5. 记录事件
    c.eventsCli.Create(k8sutil.MemberRemoveEvent(toRemove.Name, c.cluster))

    c.logger.Infof("removed dead member %q", toRemove.Name)
    return nil
}
```

### 4. 备份快照创建
```go
// pkg/backup/backup_manager.go:52
func (bm *BackupManager) SaveSnap(ctx context.Context, s3Path string) (int64, string, error) {
    // 1. 选择具有最高版本的etcd成员
    etcdcli, rev, err := bm.etcdClientWithMaxRevision(ctx)
    if err != nil {
        return 0, "", fmt.Errorf("create etcd client failed: %v", err)
    }
    defer etcdcli.Close()

    // 2. 获取etcd版本信息
    resp, err := etcdcli.Status(ctx, etcdcli.Endpoints()[0])
    if err != nil {
        return 0, "", fmt.Errorf("failed to retrieve etcd version: %v", err)
    }

    // 3. 创建快照
    rc, err := etcdcli.Snapshot(ctx)
    if err != nil {
        return 0, "", fmt.Errorf("failed to receive snapshot: %v", err)
    }
    defer rc.Close()

    // 4. 写入存储
    size, err := bm.writer.Write(ctx, s3Path, rc)
    if err != nil {
        return 0, "", fmt.Errorf("failed to write snapshot: %v", err)
    }

    bm.logger.Infof("saved snapshot to %s (size: %d bytes, revision: %d)",
                    s3Path, size, rev)

    return rev, resp.Version, nil
}
```

## 故障恢复性能优化

### 1. 协调循环优化
```go
// pkg/cluster/cluster.go:221
case <-time.After(reconcileInterval):
    start := time.Now()

    // 暂停控制检查
    if c.cluster.Spec.Paused {
        c.status.PauseControl()
        c.logger.Infof("control is paused, skipping reconciliation")
        continue
    }

    // 性能监控
    defer func() {
        reconcileHistogram.WithLabelValues(c.name()).Observe(time.Since(start).Seconds())
    }()
```

### 2. 批量状态更新
```go
// pkg/cluster/cluster.go:264
c.updateMemberStatus(running)
if err := c.updateCRStatus(); err != nil {
    c.logger.Warningf("periodic update CR status failed: %v", err)
}

func (c *Cluster) updateMemberStatus(running []*v1.Pod) {
    var unready []string
    var ready []string

    for _, pod := range running {
        if k8sutil.IsPodReady(pod) {
            ready = append(ready, pod.Name)
        } else {
            unready = append(unready, pod.Name)
        }
    }

    c.status.Members.Ready = ready
    c.status.Members.Unready = unready
}
```

### 3. 错误重试策略
```go
// pkg/util/retryutil/retryutil.go
func Retry(maxRetries int, interval time.Duration, f func() (bool, error)) error {
    for i := 0; i < maxRetries; i++ {
        done, err := f()
        if done {
            return err
        }

        if i < maxRetries-1 {
            time.Sleep(interval)
        }
    }
    return fmt.Errorf("max retries (%d) exceeded", maxRetries)
}
```

## 实际故障场景测试

### 测试场景1: 模拟Pod崩溃
```bash
# 1. 创建3节点etcd集群
kubectl apply -f example-etcd-cluster.yaml

# 2. 观察集群状态
kubectl get etcdclusters -w
kubectl get pods -l app=etcd -w

# 3. 模拟Pod故障
kubectl delete pod example-etcd-cluster-xxx --force --grace-period=0

# 4. 观察恢复过程
kubectl logs -f deployment/etcd-operator
kubectl describe etcdcluster example-etcd-cluster
```

### 测试场景2: 模拟网络分区
```bash
# 1. 使用网络策略隔离Pod
kubectl apply -f network-partition-policy.yaml

# 2. 观察operator行为
kubectl logs -f deployment/etcd-operator

# 3. 恢复网络连接
kubectl delete -f network-partition-policy.yaml

# 4. 验证集群恢复
etcdctl --endpoints=http://example-etcd-cluster:2379 member list
```

### 测试场景3: 灾难恢复测试
```bash
# 1. 创建备份
kubectl apply -f example-backup.yaml

# 2. 模拟灾难 (删除所有Pod)
kubectl delete pods -l app=etcd --force --grace-period=0

# 3. 执行恢复
kubectl apply -f example-restore.yaml

# 4. 验证数据完整性
etcdctl --endpoints=http://example-etcd-cluster:2379 get --prefix ""
```

## 关键日志观察点

### 正常恢复日志
```
time="2023-xx-xx" level=info msg="removing one dead member"
time="2023-xx-xx" level=info msg="removed dead member \"example-etcd-cluster-xxx\""
time="2023-xx-xx" level=info msg="added member (example-etcd-cluster-yyy)"
time="2023-xx-xx" level=info msg="Cluster available"
```

### 法定人数丢失日志
```
time="2023-xx-xx" level=error msg="failed to reconcile: lost quorum"
time="2023-xx-xx" level=error msg="cluster failed: lost quorum"
time="2023-xx-xx" level=info msg="cluster failed. Reporting failed reason..."
```

### 灾难恢复日志
```
time="2023-xx-xx" level=info msg="all etcd pods are dead."
time="2023-xx-xx" level=info msg="starting restore process"
time="2023-xx-xx" level=info msg="created seed member for disaster recovery"
time="2023-xx-xx" level=info msg="cluster restored from backup"
```

## 故障排查指南

### 1. 成员恢复失败
**可能原因：**
- 网络连接问题
- 资源不足
- 存储问题

**排查步骤：**
```bash
# 检查Pod状态
kubectl describe pod <pod-name>

# 检查etcd日志
kubectl logs <pod-name> -c etcd

# 检查网络连通性
kubectl exec <pod-name> -- nslookup <service-name>

# 检查存储
kubectl get pvc
kubectl describe pvc <pvc-name>
```

### 2. 备份恢复失败
**可能原因：**
- 备份文件损坏
- 存储访问权限问题
- 版本不兼容

**排查步骤：**
```bash
# 检查备份operator日志
kubectl logs -f deployment/etcd-backup-operator

# 检查恢复operator日志
kubectl logs -f deployment/etcd-restore-operator

# 验证备份文件
aws s3 ls s3://bucket/path/

# 检查权限配置
kubectl get secret <aws-secret> -o yaml
```

### 3. 集群状态异常
**排查步骤：**
```bash
# 检查集群状态
kubectl describe etcdcluster <cluster-name>

# 检查operator日志
kubectl logs -f deployment/etcd-operator

# 检查etcd集群健康状态
etcdctl --endpoints=<endpoints> endpoint health

# 检查成员列表
etcdctl --endpoints=<endpoints> member list
```

这个详细分析展示了etcd-operator在各种故障场景下的完整处理流程，为开发新的operator提供了宝贵的故障恢复机制参考。
