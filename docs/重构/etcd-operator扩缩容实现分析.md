# etcd-operator 扩缩容实现详细分析

## 项目概述

etcd-operator 是一个 Kubernetes Operator，用于管理 etcd 集群的生命周期，包括创建、扩缩容、升级和备份恢复等操作。本文档详细分析其扩缩容的实现机制。

## 核心架构组件

### 1. Controller 层 (pkg/controller/)
- **informer.go**: 监听 EtcdCluster 资源变化
- **controller.go**: 处理集群事件，管理集群生命周期

### 2. Cluster 层 (pkg/cluster/)
- **cluster.go**: 集群管理核心逻辑
- **reconcile.go**: 协调逻辑，实现扩缩容的核心算法
- **member.go**: 成员管理

### 3. 状态管理 (pkg/apis/etcd/v1beta2/)
- **status.go**: 集群状态和条件管理

## 扩缩容触发机制

### 1. 事件监听
```go
// pkg/controller/informer.go
func (c *Controller) onUpdateEtcdClus(oldObj, newObj interface{}) {
    c.syncEtcdClus(newObj.(*api.EtcdCluster))
}
```

当用户修改 EtcdCluster 的 `spec.size` 字段时：
1. Kubernetes API Server 通知 informer
2. informer 调用 `onUpdateEtcdClus`
3. 触发 `syncEtcdClus` 处理集群更新

### 2. 事件处理流程
```go
// pkg/controller/controller.go
func (c *Controller) handleClusterEvent(event *Event) (bool, error) {
    switch event.Type {
    case kwatch.Modified:
        c.clusters[getNamespacedName(clus)].Update(clus)
    }
}
```

## 协调循环 (Reconciliation Loop)

### 1. 主循环结构
```go
// pkg/cluster/cluster.go
func (c *Cluster) run() {
    for {
        select {
        case event := <-c.eventCh:
            // 处理集群更新事件
        case <-time.After(reconcileInterval):
            // 定期协调循环 (默认每5秒)
            running, pending, err := c.pollPods()
            rerr = c.reconcile(running)
        }
    }
}
```

### 2. 协调逻辑核心
```go
// pkg/cluster/reconcile.go
func (c *Cluster) reconcile(pods []*v1.Pod) error {
    running := podsToMemberSet(pods, c.isSecureClient())
    if !running.IsEqual(c.members) || c.members.Size() != sp.Size {
        return c.reconcileMembers(running)
    }
}
```

## 扩缩容实现详解

### 1. 成员协调算法
```go
func (c *Cluster) reconcileMembers(running etcdutil.MemberSet) error {
    // 步骤1: 移除不属于集群的意外Pod
    unknownMembers := running.Diff(c.members)
    if unknownMembers.Size() > 0 {
        for _, m := range unknownMembers {
            c.removePod(m.Name)
        }
    }
    
    // 步骤2: 计算有效运行的成员
    L := running.Diff(unknownMembers)
    
    // 步骤3: 检查是否需要调整大小
    if L.Size() == c.members.Size() {
        return c.resize()
    }
    
    // 步骤4: 检查法定人数
    if L.Size() < c.members.Size()/2+1 {
        return ErrLostQuorum
    }
    
    // 步骤5: 移除死亡成员
    return c.removeDeadMember(c.members.Diff(L).PickOne())
}
```

### 2. 扩容实现 (Scale Up)
```go
func (c *Cluster) addOneMember() error {
    // 1. 设置扩容状态
    c.status.SetScalingUpCondition(c.members.Size(), c.cluster.Spec.Size)
    
    // 2. 创建etcd客户端连接
    etcdcli, err := clientv3.New(cfg)
    
    // 3. 生成新成员
    newMember := c.newMember()
    
    // 4. 向etcd集群添加成员
    resp, err := etcdcli.MemberAdd(ctx, []string{newMember.PeerURL()})
    newMember.ID = resp.Member.ID
    c.members.Add(newMember)
    
    // 5. 创建Kubernetes Pod
    if err := c.createPod(c.members, newMember, "existing"); err != nil {
        return err
    }
    
    // 6. 记录事件
    c.eventsCli.Create(k8sutil.NewMemberAddEvent(newMember.Name, c.cluster))
}
```

### 3. 缩容实现 (Scale Down)
```go
func (c *Cluster) removeOneMember() error {
    // 1. 设置缩容状态
    c.status.SetScalingDownCondition(c.members.Size(), c.cluster.Spec.Size)
    
    // 2. 选择要移除的成员
    return c.removeMember(c.members.PickOne())
}

func (c *Cluster) removeMember(toRemove *etcdutil.Member) error {
    // 1. 从etcd集群移除成员
    err = etcdutil.RemoveMember(c.members.ClientURLs(), c.tlsConfig, toRemove.ID)
    
    // 2. 从内存状态移除
    c.members.Remove(toRemove.Name)
    
    // 3. 删除Kubernetes Pod
    if err := c.removePod(toRemove.Name); err != nil {
        return err
    }
    
    // 4. 删除PVC (如果启用持久化)
    if c.isPodPVEnabled() {
        err = c.removePVC(k8sutil.PVCNameFromMember(toRemove.Name))
    }
    
    // 5. 记录事件
    c.eventsCli.Create(k8sutil.MemberRemoveEvent(toRemove.Name, c.cluster))
}
```

## 状态管理机制

### 1. 扩缩容状态跟踪
```go
// pkg/apis/etcd/v1beta2/status.go
func (cs *ClusterStatus) SetScalingUpCondition(from, to int) {
    c := newClusterCondition(ClusterConditionScaling, v1.ConditionTrue, 
                           "Scaling up", scalingMsg(from, to))
    cs.setClusterCondition(*c)
}

func (cs *ClusterStatus) SetScalingDownCondition(from, to int) {
    c := newClusterCondition(ClusterConditionScaling, v1.ConditionTrue, 
                           "Scaling down", scalingMsg(from, to))
    cs.setClusterCondition(*c)
}
```

### 2. 状态更新流程
```go
func (c *Cluster) updateCRStatus() error {
    if reflect.DeepEqual(c.cluster.Status, c.status) {
        return nil
    }
    
    newCluster := c.cluster
    newCluster.Status = c.status
    newCluster, err := c.config.EtcdCRCli.EtcdV1beta2().EtcdClusters(c.cluster.Namespace).Update(c.cluster)
}
```

## Pod 生命周期管理

### 1. Pod 创建
```go
func (c *Cluster) createPod(members etcdutil.MemberSet, m *etcdutil.Member, state string) error {
    // 1. 生成Pod规格
    pod := k8sutil.NewEtcdPod(m, members.PeerURLPairs(), c.cluster.Name, state, uuid.New(), c.cluster.Spec, c.cluster.AsOwner())
    
    // 2. 处理持久化存储
    if c.isPodPVEnabled() {
        pvc := k8sutil.NewEtcdPodPVC(m, *c.cluster.Spec.Pod.PersistentVolumeClaimSpec, c.cluster.Name, c.cluster.Namespace, c.cluster.AsOwner())
        c.config.KubeCli.CoreV1().PersistentVolumeClaims(c.cluster.Namespace).Create(pvc)
        k8sutil.AddEtcdVolumeToPod(pod, pvc)
    }
    
    // 3. 创建Pod
    _, err := c.config.KubeCli.CoreV1().Pods(c.cluster.Namespace).Create(pod)
    return err
}
```

### 2. Pod 删除
```go
func (c *Cluster) removePod(name string) error {
    opts := metav1.NewDeleteOptions(podTerminationGracePeriod)
    err := c.config.KubeCli.Core().Pods(ns).Delete(name, opts)
    return err
}
```

## 扩缩容场景分析

### 场景1: 从3个节点扩容到5个节点

**执行流程:**
1. 用户修改 `spec.size: 3` → `spec.size: 5`
2. Controller 接收到 Modified 事件
3. 协调循环检测到 `c.members.Size() < c.cluster.Spec.Size`
4. 调用 `c.addOneMember()` 添加第4个成员
5. 等待Pod启动并加入集群
6. 下一个协调循环再次调用 `c.addOneMember()` 添加第5个成员
7. 所有成员就绪后，清除 Scaling 条件，设置 Ready 条件

**关键代码执行路径:**
```
reconcile() → reconcileMembers() → resize() → addOneMember() → createPod()
```

### 场景2: 从5个节点缩容到3个节点

**执行流程:**
1. 用户修改 `spec.size: 5` → `spec.size: 3`
2. Controller 接收到 Modified 事件
3. 协调循环检测到 `c.members.Size() > c.cluster.Spec.Size`
4. 调用 `c.removeOneMember()` 移除一个成员
5. 等待成员从etcd集群和Kubernetes中完全移除
6. 下一个协调循环再次调用 `c.removeOneMember()` 移除另一个成员
7. 达到目标大小后，清除 Scaling 条件，设置 Ready 条件

**关键代码执行路径:**
```
reconcile() → reconcileMembers() → resize() → removeOneMember() → removeMember() → removePod()
```

## 安全性和可靠性保障

### 1. 法定人数保护
```go
if L.Size() < c.members.Size()/2+1 {
    return ErrLostQuorum
}
```

### 2. 逐个操作策略
- 扩容时一次只添加一个成员
- 缩容时一次只移除一个成员
- 确保集群在操作过程中始终可用

### 3. 状态一致性
- 先更新etcd集群成员关系
- 再操作Kubernetes资源
- 通过协调循环确保最终一致性

## 总结

etcd-operator 的扩缩容实现采用了经典的 Kubernetes Operator 模式，通过：

1. **事件驱动**: 监听资源变化触发操作
2. **协调循环**: 定期检查并修正状态偏差
3. **逐步操作**: 一次只变更一个成员，确保集群稳定性
4. **状态管理**: 完整的状态跟踪和条件报告
5. **安全保障**: 法定人数检查和错误处理

这种设计确保了 etcd 集群在扩缩容过程中的高可用性和数据安全性。

## 详细代码执行流程

### 扩容代码执行时序图

```
用户修改spec.size: 3→5
    ↓
Controller.onUpdateEtcdClus()
    ↓
Controller.syncEtcdClus()
    ↓
Controller.handleClusterEvent(Modified)
    ↓
Cluster.Update() → 发送事件到eventCh
    ↓
Cluster.run() 主循环接收事件
    ↓
Cluster.handleUpdateEvent()
    ↓
定时协调循环 (每5秒)
    ↓
Cluster.pollPods() → 获取当前运行的Pod
    ↓
Cluster.reconcile(running)
    ↓
检查: !running.IsEqual(c.members) || c.members.Size() != sp.Size
    ↓
Cluster.reconcileMembers(running)
    ↓
检查: L.Size() == c.members.Size() → 调用resize()
    ↓
检查: c.members.Size() < c.cluster.Spec.Size → 调用addOneMember()
    ↓
执行扩容操作:
  1. SetScalingUpCondition()
  2. 创建etcd客户端
  3. newMember()
  4. etcdcli.MemberAdd()
  5. createPod()
  6. 记录事件
    ↓
等待Pod启动 → 下一个协调循环
    ↓
重复上述流程直到达到目标大小
```

### 关键数据结构

#### 1. MemberSet 结构
```go
// pkg/util/etcdutil/member.go
type MemberSet map[string]*Member

type Member struct {
    Name         string  // Pod名称
    Namespace    string  // K8s命名空间
    ID           uint64  // etcd成员ID
    SecurePeer   bool    // 是否启用peer TLS
    SecureClient bool    // 是否启用client TLS
    ClusterDomain string // 集群域名
}
```

#### 2. 集群状态条件
```go
// pkg/apis/etcd/v1beta2/status.go
const (
    ClusterConditionAvailable  = "Available"   // 集群可用
    ClusterConditionRecovering = "Recovering"  // 灾难恢复中
    ClusterConditionScaling    = "Scaling"     // 扩缩容中
    ClusterConditionUpgrading  = "Upgrading"   // 升级中
)
```

### 错误处理和重试机制

#### 1. 协调循环错误处理
```go
// pkg/cluster/cluster.go
rerr = c.reconcile(running)
if rerr != nil {
    c.logger.Errorf("failed to reconcile: %v", rerr)
    break
}

if isFatalError(rerr) {
    c.status.SetReason(rerr.Error())
    c.logger.Errorf("cluster failed: %v", rerr)
    c.reportFailedStatus()
    return
}
```

#### 2. 成员操作失败处理
```go
// 扩容失败时的清理
if err := c.createPod(c.members, newMember, "existing"); err != nil {
    // 需要从etcd集群中移除已添加的成员
    etcdutil.RemoveMember(c.members.ClientURLs(), c.tlsConfig, newMember.ID)
    return fmt.Errorf("fail to create member's pod (%s): %v", newMember.Name, err)
}
```

### 监控和指标

#### 1. Prometheus 指标
```go
// pkg/cluster/cluster.go
var (
    reconcileHistogram = prometheus.NewHistogramVec(...)
    reconcileFailed = prometheus.NewCounterVec(...)
    clustersTotal = prometheus.NewGauge(...)
    clustersCreated = prometheus.NewCounter(...)
    clustersDeleted = prometheus.NewCounter(...)
    clustersModified = prometheus.NewCounter(...)
    clustersFailed = prometheus.NewCounter(...)
)
```

#### 2. 事件记录
```go
// pkg/util/k8sutil/events_util.go
func NewMemberAddEvent(memberName string, cl *api.EtcdCluster) *v1.Event {
    event := newClusterEvent(cl)
    event.Type = v1.EventTypeNormal
    event.Reason = "New Member Added"
    event.Message = fmt.Sprintf("New member %s added to cluster", memberName)
    return event
}
```

### 配置参数

#### 1. 重要常量
```go
// pkg/cluster/cluster.go
const (
    reconcileInterval = 5 * time.Second  // 协调循环间隔
    podTerminationGracePeriod = int64(5) // Pod终止宽限期
)

// pkg/util/constants/constants.go
const (
    DefaultDialTimeout = 5 * time.Second     // etcd连接超时
    DefaultRequestTimeout = 5 * time.Second // etcd请求超时
)
```

#### 2. 集群规格验证
```go
// pkg/apis/etcd/v1beta2/cluster.go
func (c *ClusterSpec) Validate() error {
    if c.Size <= 0 {
        return errors.New("cluster size must be positive")
    }
    if c.Size%2 == 0 {
        return errors.New("cluster size must be odd")
    }
    // 更多验证逻辑...
}
```

## 实际部署和测试建议

### 1. 扩容测试步骤
```bash
# 1. 创建初始集群
kubectl apply -f example-etcd-cluster.yaml

# 2. 观察集群状态
kubectl get etcdclusters -w
kubectl describe etcdcluster example-etcd-cluster

# 3. 修改集群大小
kubectl patch etcdcluster example-etcd-cluster -p '{"spec":{"size":5}}'

# 4. 监控扩容过程
kubectl get pods -l app=etcd -w
kubectl logs -f deployment/etcd-operator
```

### 2. 关键日志观察点
```
# 扩容开始
"Start reconciling"
"running members: [member1, member2, member3]"
"cluster membership: [member1, member2, member3]"

# 检测到需要扩容
"Current cluster size: 3, desired cluster size: 5"

# 添加新成员
"added member (example-etcd-cluster-xxx)"
"New member example-etcd-cluster-xxx added to cluster"

# 扩容完成
"Finish reconciling"
"Cluster available"
```

### 3. 故障排查要点
1. **Pod状态**: 检查Pod是否正常启动
2. **etcd日志**: 查看etcd容器日志确认成员加入
3. **网络连通性**: 确保Pod间网络通信正常
4. **存储**: 检查PVC创建和挂载状态
5. **资源限制**: 确认节点资源充足

## Pod变化时的具体代码执行分析

### 扩容场景：从3个Pod到5个Pod的详细执行

#### 第一次协调循环 (添加第4个成员)

**1. Pod状态检查**
```go
// pkg/cluster/cluster.go:235
running, pending, err := c.pollPods()
// 此时 running = [pod1, pod2, pod3], pending = []
```

**2. 成员状态对比**
```go
// pkg/cluster/reconcile.go:47
running := podsToMemberSet(pods, c.isSecureClient())
// running = {pod1: Member, pod2: Member, pod3: Member}
// c.members = {member1: Member, member2: Member, member3: Member}
// c.cluster.Spec.Size = 5
```

**3. 触发协调**
```go
// pkg/cluster/reconcile.go:48
if !running.IsEqual(c.members) || c.members.Size() != sp.Size {
    return c.reconcileMembers(running)
}
// 条件: c.members.Size() (3) != sp.Size (5) → true
```

**4. 成员协调逻辑**
```go
// pkg/cluster/reconcile.go:91
if L.Size() == c.members.Size() {
    return c.resize()
}
// L.Size() = 3, c.members.Size() = 3 → 调用resize()
```

**5. 扩容判断**
```go
// pkg/cluster/reconcile.go:109
if c.members.Size() < c.cluster.Spec.Size {
    return c.addOneMember()
}
// 3 < 5 → 调用addOneMember()
```

**6. 执行添加成员**
```go
// pkg/cluster/reconcile.go:117
c.status.SetScalingUpCondition(c.members.Size(), c.cluster.Spec.Size)
// 设置状态: "Scaling up from 3 to 5"

// pkg/cluster/reconcile.go:130
newMember := c.newMember()
// 生成新成员: example-etcd-cluster-abc123

// pkg/cluster/reconcile.go:132
resp, err := etcdcli.MemberAdd(ctx, []string{newMember.PeerURL()})
// 向etcd集群添加成员

// pkg/cluster/reconcile.go:140
if err := c.createPod(c.members, newMember, "existing"); err != nil
// 创建第4个Pod
```

#### 第二次协调循环 (等待第4个Pod启动)

**1. Pod状态检查**
```go
running, pending, err := c.pollPods()
// 此时可能 running = [pod1, pod2, pod3], pending = [pod4]
// 或者 running = [pod1, pod2, pod3, pod4], pending = []
```

**2. 处理Pending状态**
```go
// pkg/cluster/cluster.go:242
if len(pending) > 0 {
    c.logger.Infof("skip reconciliation: running (%v), pending (%v)",
                   k8sutil.GetPodNames(running), k8sutil.GetPodNames(pending))
    continue
}
// 如果Pod4还在pending，跳过本次协调
```

#### 第三次协调循环 (添加第5个成员)

**1. Pod状态检查**
```go
running, pending, err := c.pollPods()
// 此时 running = [pod1, pod2, pod3, pod4], pending = []
```

**2. 更新成员信息**
```go
// pkg/cluster/cluster.go:256
rerr = c.updateMembers(podsToMemberSet(running, c.isSecureClient()))
// 从etcd集群获取最新成员信息，更新c.members
```

**3. 再次触发扩容**
```go
// 重复上述流程，添加第5个成员
c.members.Size() = 4, c.cluster.Spec.Size = 5
// 4 < 5 → 再次调用addOneMember()
```

### 缩容场景：从5个Pod到3个Pod的详细执行

#### 第一次协调循环 (移除第5个成员)

**1. 缩容判断**
```go
// pkg/cluster/reconcile.go:113
return c.removeOneMember()
// c.members.Size() (5) > c.cluster.Spec.Size (3)
```

**2. 执行移除成员**
```go
// pkg/cluster/reconcile.go:152
c.status.SetScalingDownCondition(c.members.Size(), c.cluster.Spec.Size)
// 设置状态: "Scaling down from 5 to 3"

// pkg/cluster/reconcile.go:154
return c.removeMember(c.members.PickOne())
// 选择一个成员进行移除
```

**3. 成员移除流程**
```go
// pkg/cluster/reconcile.go:174
err = etcdutil.RemoveMember(c.members.ClientURLs(), c.tlsConfig, toRemove.ID)
// 从etcd集群移除成员

// pkg/cluster/reconcile.go:183
c.members.Remove(toRemove.Name)
// 从内存状态移除

// pkg/cluster/reconcile.go:188
if err := c.removePod(toRemove.Name); err != nil
// 删除Kubernetes Pod

// pkg/cluster/reconcile.go:192
if c.isPodPVEnabled() {
    err = c.removePVC(k8sutil.PVCNameFromMember(toRemove.Name))
}
// 删除PVC（如果启用）
```

### 关键状态转换

#### 扩容状态转换
```
Initial: Size=3, Condition=Available
    ↓ (用户修改spec.size=5)
Scaling: Size=3, Condition=Scaling(3→5)
    ↓ (添加第4个成员)
Scaling: Size=4, Condition=Scaling(3→5)
    ↓ (添加第5个成员)
Ready: Size=5, Condition=Available
```

#### 缩容状态转换
```
Initial: Size=5, Condition=Available
    ↓ (用户修改spec.size=3)
Scaling: Size=5, Condition=Scaling(5→3)
    ↓ (移除第5个成员)
Scaling: Size=4, Condition=Scaling(5→3)
    ↓ (移除第4个成员)
Ready: Size=3, Condition=Available
```

### 异常情况处理

#### 1. Pod创建失败
```go
// pkg/cluster/reconcile.go:140
if err := c.createPod(c.members, newMember, "existing"); err != nil {
    // 需要清理已添加到etcd的成员
    etcdutil.RemoveMember(c.members.ClientURLs(), c.tlsConfig, newMember.ID)
    c.members.Remove(newMember.Name)
    return fmt.Errorf("fail to create member's pod (%s): %v", newMember.Name, err)
}
```

#### 2. etcd成员添加失败
```go
// pkg/cluster/reconcile.go:134
if err != nil {
    return fmt.Errorf("fail to add new member (%s): %v", newMember.Name, err)
}
// 不需要清理，因为Pod还未创建
```

#### 3. 法定人数不足
```go
// pkg/cluster/reconcile.go:95
if L.Size() < c.members.Size()/2+1 {
    return ErrLostQuorum
}
// 停止所有操作，等待人工干预
```

### 性能优化考虑

#### 1. 批量操作限制
- 一次只操作一个成员，避免集群不稳定
- 等待Pod完全就绪后再进行下一个操作

#### 2. 协调频率控制
```go
// pkg/cluster/cluster.go:224
case <-time.After(reconcileInterval):
// 默认5秒间隔，避免过于频繁的检查
```

#### 3. 状态缓存
```go
// pkg/cluster/cluster.go:449
if reflect.DeepEqual(c.cluster.Status, c.status) {
    return nil
}
// 只有状态真正变化时才更新CR
```

## 关键工具函数详解

### 1. Pod状态轮询 (pollPods)
```go
// pkg/cluster/cluster.go:400
func (c *Cluster) pollPods() (running, pending []*v1.Pod, err error) {
    // 1. 获取集群所有Pod
    podList, err := c.config.KubeCli.Core().Pods(c.cluster.Namespace).List(k8sutil.ClusterListOpt(c.cluster.Name))

    for i := range podList.Items {
        pod := &podList.Items[i]

        // 2. 跳过正在删除的Pod
        if pod.DeletionTimestamp != nil {
            continue
        }

        // 3. 检查Pod所有者
        if pod.OwnerReferences[0].UID != c.cluster.UID {
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

### 2. Pod就绪状态检查
```go
// pkg/util/k8sutil/pod_util.go:125
func IsPodReady(pod *v1.Pod) bool {
    condition := getPodReadyCondition(&pod.Status)
    return condition != nil && condition.Status == v1.ConditionTrue
}

func getPodReadyCondition(status *v1.PodStatus) *v1.PodCondition {
    for i := range status.Conditions {
        if status.Conditions[i].Type == v1.PodReady {
            return &status.Conditions[i]
        }
    }
    return nil
}
```

### 3. Pod到成员集合转换
```go
// pkg/cluster/member.go:67
func podsToMemberSet(pods []*v1.Pod, sc bool) etcdutil.MemberSet {
    members := etcdutil.MemberSet{}
    for _, pod := range pods {
        m := &etcdutil.Member{
            Name: pod.Name,
            Namespace: pod.Namespace,
            SecureClient: sc
        }
        members.Add(m)
    }
    return members
}
```

### 4. 成员状态更新
```go
// pkg/cluster/cluster.go:433
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

## 完整扩缩容流程总结

### 扩容完整流程 (3→5节点)

```
阶段1: 事件触发
├── 用户执行: kubectl patch etcdcluster example --patch '{"spec":{"size":5}}'
├── API Server更新EtcdCluster资源
├── Informer检测到Modified事件
├── Controller.onUpdateEtcdClus()被调用
└── 事件发送到Cluster.eventCh

阶段2: 第一次协调 (添加第4个节点)
├── Cluster.run()接收事件
├── 定时协调循环触发 (5秒后)
├── pollPods(): running=[pod1,pod2,pod3], pending=[]
├── reconcile(): 检测到size不匹配 (3 != 5)
├── reconcileMembers(): L.Size() == c.members.Size() (3)
├── resize(): c.members.Size() (3) < c.cluster.Spec.Size (5)
├── addOneMember():
│   ├── SetScalingUpCondition(3, 5)
│   ├── newMember(): 生成example-etcd-cluster-abc123
│   ├── etcdcli.MemberAdd(): 向etcd集群添加成员
│   ├── createPod(): 创建Kubernetes Pod
│   └── 记录MemberAddEvent
└── 等待Pod启动

阶段3: 第二次协调 (等待第4个节点就绪)
├── pollPods(): running=[pod1,pod2,pod3], pending=[pod4] (Pod启动中)
├── 检测到pending Pod存在
├── 跳过本次协调: "skip reconciliation: pending pods"
└── 继续等待

阶段4: 第三次协调 (添加第5个节点)
├── pollPods(): running=[pod1,pod2,pod3,pod4], pending=[]
├── updateMembers(): 从etcd获取最新成员信息
├── reconcile(): 检测到size不匹配 (4 != 5)
├── addOneMember(): 重复阶段2流程，添加第5个成员
└── 创建第5个Pod

阶段5: 扩容完成
├── pollPods(): running=[pod1,pod2,pod3,pod4,pod5], pending=[]
├── reconcile(): c.members.Size() (5) == c.cluster.Spec.Size (5)
├── ClearCondition(ClusterConditionScaling)
├── SetReadyCondition()
└── 扩容完成
```

### 缩容完整流程 (5→3节点)

```
阶段1: 事件触发
├── 用户执行: kubectl patch etcdcluster example --patch '{"spec":{"size":3}}'
└── 触发Modified事件

阶段2: 第一次协调 (移除第5个节点)
├── reconcile(): 检测到size不匹配 (5 != 3)
├── resize(): c.members.Size() (5) > c.cluster.Spec.Size (3)
├── removeOneMember():
│   ├── SetScalingDownCondition(5, 3)
│   └── removeMember(c.members.PickOne()):
│       ├── etcdutil.RemoveMember(): 从etcd集群移除成员
│       ├── c.members.Remove(): 从内存状态移除
│       ├── removePod(): 删除Kubernetes Pod
│       ├── removePVC(): 删除PVC (如果启用)
│       └── 记录MemberRemoveEvent
└── 等待Pod终止

阶段3: 第二次协调 (移除第4个节点)
├── pollPods(): running=[pod1,pod2,pod3,pod4], pending=[]
├── reconcile(): 检测到size不匹配 (4 != 3)
├── removeOneMember(): 重复阶段2流程，移除第4个成员
└── 等待Pod终止

阶段4: 缩容完成
├── pollPods(): running=[pod1,pod2,pod3], pending=[]
├── reconcile(): c.members.Size() (3) == c.cluster.Spec.Size (3)
├── ClearCondition(ClusterConditionScaling)
├── SetReadyCondition()
└── 缩容完成
```

## 设计亮点和最佳实践

### 1. 渐进式操作
- **一次一个**: 每次只添加或移除一个成员，确保集群稳定性
- **等待就绪**: 等待新Pod完全启动后再进行下一个操作
- **状态检查**: 每次操作前都检查当前状态

### 2. 错误恢复机制
- **幂等操作**: 所有操作都是幂等的，可以安全重试
- **状态同步**: 定期从etcd集群同步成员信息
- **法定人数保护**: 防止操作导致集群失去法定人数

### 3. 可观测性
- **详细日志**: 记录每个操作步骤和状态变化
- **Kubernetes事件**: 通过Event API记录重要操作
- **状态条件**: 通过Condition API报告集群状态
- **Prometheus指标**: 提供监控和告警数据

### 4. 资源管理
- **OwnerReference**: 确保Pod被正确管理和清理
- **优雅终止**: 使用GracePeriod确保Pod优雅关闭
- **存储管理**: 正确处理PVC的创建和删除

这个详细分析展示了etcd-operator在Pod数量变化时的完整代码执行路径，为您开发新的operator提供了宝贵的实现参考。
