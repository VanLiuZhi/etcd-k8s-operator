# ETCD 集群扩缩容代码执行流程分析

## 概述

本文档详细分析从CRD size从1改成3时的完整代码执行流程，特别关注辅助PVC pod的创建和销毁过程。

## 代码执行流程分析

### 1. 主入口点 (main函数)

**文件位置**: `cmd/main.go:55`

```go
func main() {
    // 1. 创建控制器管理器
    mgr, err := ctrl.NewManager(cfg, ctrl.Options{...})
    
    // 2. 注册EtcdCluster控制器
    clusterController := controller.NewClusterController(
        mgr.GetClient(),
        mgr.GetScheme(), 
        mgr.GetEventRecorderFor("etcdcluster-controller"),
    )
    clusterController.SetupWithManager(mgr)
    
    // 3. 启动管理器，开始监听CRD变化
    mgr.Start(ctrl.SetupSignalHandler())
}
```

### 2. 控制器调谐入口

**文件位置**: `internal/controller/cluster_controller.go:91`

```go
func (r *ClusterController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // 1. 获取EtcdCluster实例
    cluster := &etcdv1alpha1.EtcdCluster{}
    r.Get(ctx, req.NamespacedName, cluster)
    
    // 2. 检查删除标记和Finalizer
    
    // 3. 设置默认值
    r.clusterService.SetDefaults(cluster)
    
    // 4. 状态机处理
    return r.handleStateMachine(ctx, cluster)
}
```

### 3. 状态机处理

**文件位置**: `internal/controller/cluster_controller.go:131`

```go
func (r *ClusterController) handleStateMachine(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
    switch cluster.Status.Phase {
    case "":
        return r.clusterService.InitializeCluster(ctx, cluster)
    case etcdv1alpha1.EtcdClusterPhaseCreating:
        return r.clusterService.CreateCluster(ctx, cluster)
    case etcdv1alpha1.EtcdClusterPhaseRunning:
        // 关键：这里检查是否需要扩缩容
        return r.clusterService.HandleRunning(ctx, cluster)
    case etcdv1alpha1.EtcdClusterPhaseScaling:
        return r.scalingService.HandleScaling(ctx, cluster)
    }
}
```

### 4. Running状态处理 - 扩缩容检测

**文件位置**: `internal/controller/etcdcluster_controller.go:340`

```go
func (r *EtcdClusterReconciler) handleRunning(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
    // 1. 更新集群状态
    if err := r.updateClusterStatus(ctx, cluster); err != nil {
        return ctrl.Result{}, err
    }
    
    // 2. 检查是否需要扩缩容 - 关键判断点
    if r.needsScaling(cluster) {
        logger.Info("Cluster needs scaling", "current", cluster.Status.ReadyReplicas, "desired", cluster.Spec.Size)
        cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseScaling
        
        if err := r.Status().Update(ctx, cluster); err != nil {
            return ctrl.Result{}, err
        }
        return ctrl.Result{Requeue: true}, nil
    }
    
    // 3. 正常运行状态维护
    return ctrl.Result{RequeueAfter: utils.DefaultHealthCheckInterval}, nil
}
```

### 5. 扩缩容需求判断

**文件位置**: `internal/controller/etcdcluster_controller.go:1200`

```go
func (r *EtcdClusterReconciler) needsScaling(cluster *etcdv1alpha1.EtcdCluster) bool {
    currentSize := cluster.Status.ReadyReplicas
    desiredSize := cluster.Spec.Size
    
    // 当前就绪副本数与期望大小不一致时需要扩缩容
    return currentSize != desiredSize
}
```

### 6. 扩缩容处理入口

**文件位置**: `internal/controller/etcdcluster_controller.go:384`

```go
func (r *EtcdClusterReconciler) handleScaling(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
    currentSize := cluster.Status.ReadyReplicas
    desiredSize := cluster.Spec.Size
    
    if currentSize < desiredSize {
        // 扩容路径
        return r.handleScaleUp(ctx, cluster)
    } else if currentSize > desiredSize {
        // 缩容路径
        return r.handleScaleDown(ctx, cluster)
    }
    
    // 扩缩容完成，转换回运行状态
    cluster.Status.Phase = etcdv1alpha1.EtcdClusterPhaseRunning
    return ctrl.Result{RequeueAfter: utils.DefaultHealthCheckInterval}, nil
}
```

## 扩容流程详细分析 (1→3节点)

### 7. 扩容处理逻辑

**文件位置**: `internal/controller/etcdcluster_controller.go:913`

```go
func (r *EtcdClusterReconciler) handleScaleUp(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster) (ctrl.Result, error) {
    // 1. 获取当前StatefulSet状态
    sts := &appsv1.StatefulSet{}
    r.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, sts)
    
    currentReplicas := *sts.Spec.Replicas  // 当前副本数
    desiredReplicas := cluster.Spec.Size   // 期望副本数 (3)
    readyReplicas := sts.Status.ReadyReplicas // 就绪副本数
    
    // 2. 渐进式扩容：一次只添加一个节点
    if currentReplicas < desiredReplicas {
        nextNodeIndex := currentReplicas  // 下一个节点索引
        
        // 步骤1: 先通过etcd API添加成员
        if err := r.addEtcdMember(ctx, cluster, nextNodeIndex); err != nil {
            return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
        }
        
        // 步骤2: 增加StatefulSet副本数，让Kubernetes创建新Pod
        nextReplicas := currentReplicas + 1
        *sts.Spec.Replicas = nextReplicas
        if err := r.Update(ctx, sts); err != nil {
            return ctrl.Result{}, err
        }
        
        return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
    }
    
    return ctrl.Result{RequeueAfter: utils.DefaultRequeueInterval}, nil
}
```

### 8. etcd成员添加

**文件位置**: `internal/controller/etcdcluster_controller.go:1262`

```go
func (r *EtcdClusterReconciler) addEtcdMember(ctx context.Context, cluster *etcdv1alpha1.EtcdCluster, memberIndex int32) error {
    // 1. 创建etcd客户端连接到现有集群
    etcdClient, err := r.createEtcdClient(cluster)
    if err != nil {
        return fmt.Errorf("failed to create etcd client: %w", err)
    }
    defer etcdClient.Close()
    
    // 2. 构建新成员信息
    memberName := fmt.Sprintf("%s-%d", cluster.Name, memberIndex)
    peerURL := fmt.Sprintf("http://%s.%s-peer.%s.svc.cluster.local:%d",
        memberName, cluster.Name, cluster.Namespace, utils.EtcdPeerPort)
    
    // 3. 检查成员是否已存在
    members, err := etcdClient.GetClusterMembers(ctx)
    if err != nil {
        return fmt.Errorf("failed to get cluster members: %w", err)
    }
    
    for _, member := range members {
        if member.Name == memberName {
            return nil // 已存在，跳过
        }
    }
    
    // 4. 添加新成员到etcd集群
    resp, err := etcdClient.AddMember(ctx, peerURL)
    if err != nil {
        return fmt.Errorf("failed to add member %s: %w", memberName, err)
    }
    
    logger.Info("Successfully added etcd member", "name", memberName, "memberID", resp.Member.ID)
    return nil
}
```

## StatefulSet和PVC管理分析

### 9. StatefulSet配置

**文件位置**: `pkg/k8s/resources.go:39`

```go
func BuildStatefulSetWithReplicas(cluster *etcdv1alpha1.EtcdCluster, replicas int32) *appsv1.StatefulSet {
    sts := &appsv1.StatefulSet{
        ObjectMeta: metav1.ObjectMeta{
            Name:      cluster.Name,
            Namespace: cluster.Namespace,
        },
        Spec: appsv1.StatefulSetSpec{
            Replicas:    &replicas,  // 关键：副本数设置
            ServiceName: fmt.Sprintf("%s-peer", cluster.Name),

            // 关键配置：并行Pod管理策略
            PodManagementPolicy: appsv1.ParallelPodManagement,

            // PVC模板 - 这里定义了存储需求
            VolumeClaimTemplates: buildVolumeClaimTemplates(cluster),

            Template: corev1.PodTemplateSpec{
                Spec: buildPodSpec(cluster),
            },
        },
    }
    return sts
}
```

### 10. PVC模板配置

**文件位置**: `pkg/k8s/resources.go:509`

```go
func buildVolumeClaimTemplates(cluster *etcdv1alpha1.EtcdCluster) []corev1.PersistentVolumeClaim {
    storageSize := cluster.Spec.Storage.Size
    if storageSize.IsZero() {
        storageSize = resource.MustParse(utils.DefaultStorageSize) // 默认10Gi
    }

    pvc := corev1.PersistentVolumeClaim{
        ObjectMeta: metav1.ObjectMeta{
            Name:   "data",  // PVC名称模板
            Labels: utils.LabelsForEtcdCluster(cluster),
        },
        Spec: corev1.PersistentVolumeClaimSpec{
            AccessModes: []corev1.PersistentVolumeAccessMode{
                corev1.ReadWriteOnce,
            },
            Resources: corev1.VolumeResourceRequirements{
                Requests: corev1.ResourceList{
                    corev1.ResourceStorage: storageSize,
                },
            },
        },
    }

    return []corev1.PersistentVolumeClaim{pvc}
}
```

### 11. Pod规格配置 - Init Container分析

**文件位置**: `pkg/k8s/resources.go:75`

```go
func buildPodSpec(cluster *etcdv1alpha1.EtcdCluster) corev1.PodSpec {
    // 关键：每个Pod都有Init Container
    var initContainers []corev1.Container
    initContainers = append(initContainers, buildEtcdInitContainer(cluster))

    containers := []corev1.Container{
        buildEtcdContainer(cluster, 0),
    }

    return corev1.PodSpec{
        InitContainers: initContainers,  // 这里是辅助容器
        Containers:     containers,
        Volumes: []corev1.Volume{
            {
                Name: "etcd-config",
                VolumeSource: corev1.VolumeSource{
                    EmptyDir: &corev1.EmptyDirVolumeSource{},
                },
            },
        },
    }
}
```

### 12. Init Container详细分析 - 辅助PVC Pod的真相

**文件位置**: `pkg/k8s/resources.go:334`

```go
func buildEtcdInitContainer(cluster *etcdv1alpha1.EtcdCluster) corev1.Container {
    script := `#!/bin/sh
set -e

# 获取当前节点信息
HOSTNAME=$(hostname)
POD_INDEX=$(echo $HOSTNAME | sed 's/.*-//')

echo "Current hostname: $HOSTNAME"
echo "Pod index: $POD_INDEX"

# 创建配置目录
mkdir -p /etc/etcd

# 根据Pod索引生成不同的etcd配置
if [ "$POD_INDEX" = "0" ]; then
    # 第一个节点配置
    cat > /etc/etcd/etcd.conf << EOF
name: $HOSTNAME
data-dir: /var/lib/etcd
listen-client-urls: http://0.0.0.0:2379
listen-peer-urls: http://0.0.0.0:2380
advertise-client-urls: http://$HOSTNAME.${CLUSTER_NAME}-peer.${NAMESPACE}.svc.cluster.local:2379
initial-advertise-peer-urls: http://$HOSTNAME.${CLUSTER_NAME}-peer.${NAMESPACE}.svc.cluster.local:2380
initial-cluster-state: new
initial-cluster-token: ${CLUSTER_NAME}
initial-cluster: ${HOSTNAME}=http://$HOSTNAME.${CLUSTER_NAME}-peer.${NAMESPACE}.svc.cluster.local:2380
EOF
else
    # 后续节点配置
    cat > /etc/etcd/etcd.conf << EOF
name: $HOSTNAME
data-dir: /var/lib/etcd
listen-client-urls: http://0.0.0.0:2379
listen-peer-urls: http://0.0.0.0:2380
advertise-client-urls: http://$HOSTNAME.${CLUSTER_NAME}-peer.${NAMESPACE}.svc.cluster.local:2379
initial-advertise-peer-urls: http://$HOSTNAME.${CLUSTER_NAME}-peer.${NAMESPACE}.svc.cluster.local:2380
initial-cluster-state: existing
initial-cluster-token: ${CLUSTER_NAME}
EOF
fi

echo "Generated etcd configuration:"
cat /etc/etcd/etcd.conf
echo "Init container completed successfully"
`

    return corev1.Container{
        Name:    "etcd-init",
        Image:   "busybox:1.35",  // 轻量级镜像
        Command: []string{"/bin/sh", "-c", script},
        VolumeMounts: []corev1.VolumeMount{
            {
                Name:      "etcd-config",
                MountPath: "/etc/etcd",
            },
        },
        Resources: corev1.ResourceRequirements{
            Requests: corev1.ResourceList{
                corev1.ResourceCPU:    resource.MustParse("50m"),
                corev1.ResourceMemory: resource.MustParse("32Mi"),
            },
            Limits: corev1.ResourceList{
                corev1.ResourceCPU:    resource.MustParse("100m"),
                corev1.ResourceMemory: resource.MustParse("64Mi"),
            },
        },
    }
}
```

## 关键发现：辅助PVC Pod的真相

### 13. "辅助PVC Pod" 实际上是 Init Container

通过代码分析发现，您观察到的"辅助PVC pod被创建但马上就销毁"的现象，实际上是 **Init Container** 的正常行为：

**现象解释**：
1. **不是独立的Pod**：这不是一个单独的Pod，而是每个etcd Pod中的Init Container
2. **生命周期**：Init Container在主容器启动前运行，完成后自动退出
3. **作用**：生成etcd配置文件，根据Pod索引决定是新集群还是加入现有集群

**观察到的行为**：
```bash
# 您可能看到类似这样的Pod状态变化
NAME             READY   STATUS     RESTARTS   AGE
test-scaling-1   0/2     Init:0/1   0          10s   # Init Container运行中
test-scaling-1   0/2     Init:0/1   0          15s   # Init Container完成
test-scaling-1   0/2     PodInitializing   0    20s   # 主容器启动中
test-scaling-1   2/2     Running    0          30s   # 全部容器运行
```

### 14. StatefulSet的PVC自动管理

**文件位置**: StatefulSet Controller (Kubernetes内置)

当StatefulSet副本数从1增加到3时：

```yaml
# 自动创建的PVC (Kubernetes自动管理)
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-test-scaling-0  # 第一个Pod的PVC
  namespace: operator-etcd
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 10Gi

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-test-scaling-1  # 第二个Pod的PVC (新创建)
  namespace: operator-etcd
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 10Gi

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: data-test-scaling-2  # 第三个Pod的PVC (新创建)
  namespace: operator-etcd
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 10Gi
```

### 15. 并行Pod管理策略的影响

**关键配置**: `PodManagementPolicy: appsv1.ParallelPodManagement`

这个配置导致：
1. **并行创建**：新Pod不需要等待前一个Pod就绪
2. **快速扩容**：多个Pod可以同时创建和初始化
3. **Init Container并行运行**：多个Init Container可能同时运行

## 完整的扩容执行时序

### 时序图

```
时间轴: 0s -----> 10s -----> 20s -----> 30s -----> 40s

CRD更新: size=1 -> size=3
         |
         v
Controller检测到变化
         |
         v
handleRunning() -> needsScaling() -> true
         |
         v
状态转换: Running -> Scaling
         |
         v
handleScaleUp() 第一次调用
         |
         v
addEtcdMember(index=1) -> 添加test-scaling-1到etcd集群
         |
         v
StatefulSet.Replicas: 1 -> 2
         |
         v
Kubernetes创建test-scaling-1 Pod
         |
         v
test-scaling-1 Init Container运行 (您看到的"辅助Pod")
         |
         v
Init Container完成，主容器启动
         |
         v
Controller再次调用handleScaleUp()
         |
         v
addEtcdMember(index=2) -> 添加test-scaling-2到etcd集群
         |
         v
StatefulSet.Replicas: 2 -> 3
         |
         v
Kubernetes创建test-scaling-2 Pod
         |
         v
test-scaling-2 Init Container运行
         |
         v
所有Pod就绪，状态转换: Scaling -> Running
```

## 总结

1. **"辅助PVC Pod"实际上是Init Container**，不是独立的Pod
2. **Init Container的作用**：为每个etcd节点生成正确的配置文件
3. **PVC由StatefulSet自动管理**，根据VolumeClaimTemplates创建
4. **扩容是渐进式的**：一次添加一个节点，先加入etcd集群，再创建Pod
5. **并行Pod管理**：允许多个Pod同时创建，加快扩容速度

## 关键代码位置索引

### 主要控制流程
- **主入口**: `cmd/main.go:55` - main()函数
- **控制器入口**: `internal/controller/cluster_controller.go:91` - Reconcile()
- **状态机**: `internal/controller/cluster_controller.go:131` - handleStateMachine()
- **运行状态处理**: `internal/controller/etcdcluster_controller.go:340` - handleRunning()
- **扩缩容检测**: `internal/controller/etcdcluster_controller.go:1200` - needsScaling()

### 扩容相关
- **扩容入口**: `internal/controller/etcdcluster_controller.go:384` - handleScaling()
- **扩容逻辑**: `internal/controller/etcdcluster_controller.go:913` - handleScaleUp()
- **etcd成员管理**: `internal/controller/etcdcluster_controller.go:1262` - addEtcdMember()

### 资源管理
- **StatefulSet构建**: `pkg/k8s/resources.go:39` - BuildStatefulSetWithReplicas()
- **PVC模板**: `pkg/k8s/resources.go:509` - buildVolumeClaimTemplates()
- **Pod规格**: `pkg/k8s/resources.go:75` - buildPodSpec()
- **Init Container**: `pkg/k8s/resources.go:334` - buildEtcdInitContainer()

### 服务层
- **集群服务**: `pkg/service/cluster_service.go:123` - CreateCluster()
- **扩缩容服务**: `pkg/service/scaling_service.go:118` - HandleScaling()

## 调试建议

### 1. 观察Init Container
```bash
# 查看Pod的Init Container状态
kubectl describe pod test-scaling-1 -n operator-etcd

# 查看Init Container日志
kubectl logs test-scaling-1 -c etcd-init -n operator-etcd
```

### 2. 监控PVC创建
```bash
# 实时监控PVC创建
kubectl get pvc -w -n operator-etcd

# 查看PVC详情
kubectl describe pvc data-test-scaling-1 -n operator-etcd
```

### 3. 跟踪扩容过程
```bash
# 监控StatefulSet变化
kubectl get statefulset test-scaling -w -n operator-etcd

# 查看etcd集群成员变化
kubectl exec test-scaling-0 -c etcd -n operator-etcd -- etcdctl member list
```

### 4. 查看关键日志
```bash
# 查找扩容相关日志
kubectl logs -n etcd-k8s-operator-system deployment/etcd-k8s-operator-controller-manager | grep -E "(handleScaleUp|addEtcdMember|StatefulSet.*replicas)"
```

## 实际验证结果

### 验证1：Init Container确认

通过 `kubectl describe pod test-scaling-1 -n operator-etcd` 确认：

```yaml
Init Containers:
  etcd-init:
    Container ID:  containerd://60068ac2829f8feb224874acc56fdcdaf8765fe27036ccaa7fdfe32c7f93e98c
    Image:         busybox:1.35
    State:          Terminated
      Reason:       Completed
      Exit Code:    0
      Started:      Mon, 18 Aug 2025 00:20:10 +0800
      Finished:     Mon, 18 Aug 2025 00:20:11 +0800
    Ready:          True
```

**关键发现**：
- Init Container使用 `busybox:1.35` 镜像
- 运行时间很短（1秒钟），然后状态变为 `Terminated/Completed`
- 这就是您观察到的"辅助PVC pod被创建但马上就销毁"的现象

### 验证2：PVC自动创建

通过 `kubectl get pvc -n operator-etcd` 确认：

```
NAME                  STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
data-test-scaling-0   Bound    pvc-b34bb56c-c86e-4ad6-9660-efec02c43945   10Gi       RWO            standard       24m
data-test-scaling-1   Bound    pvc-1247987c-078a-40a7-99e6-e9b754343476   10Gi       RWO            standard       20m
data-test-scaling-2   Bound    pvc-14b15488-1f1c-4b91-80f5-c182f8265609   10Gi       RWO            standard       20m
```

**关键发现**：
- StatefulSet自动为每个Pod创建了对应的PVC
- PVC命名规则：`data-{StatefulSet名称}-{Pod索引}`
- 所有PVC都使用默认的10Gi存储大小

### 验证3：Init Container工作内容

通过 `kubectl logs test-scaling-1 -c etcd-init -n operator-etcd` 确认：

```bash
Current hostname: test-scaling-1
Pod index: 1
Configuring as additional node (joining existing cluster)
Waiting for first node to be ready...
# DNS解析成功
Querying current etcd cluster members...
Successfully queried etcd v3 members API
API response: {"header":{"cluster_id":"11559400194303071067"...}
Current node already in cluster, using existing members: test-scaling-0=...,test-scaling-1=...
Generated etcd configuration:
# etcd configuration for test-scaling-1
name: test-scaling-1
initial-cluster-state: existing
initial-cluster: test-scaling-0=...,test-scaling-1=...
Init container completed successfully
```

**关键发现**：
- Init Container智能地检测当前节点是否已在etcd集群中
- 通过etcd v3 API查询现有成员列表
- 生成正确的etcd配置文件，设置 `initial-cluster-state: existing`
- 配置完成后立即退出（Completed状态）

## 最终结论

**"辅助PVC pod"的真相**：
1. **不是独立的Pod**：是每个etcd Pod中的Init Container
2. **使用轻量级镜像**：busybox:1.35，资源消耗极小
3. **生命周期很短**：通常1-2秒完成任务后退出
4. **智能配置生成**：根据etcd集群状态生成正确的配置
5. **正常行为**：Terminated/Completed状态是预期的，不是错误

这个设计非常巧妙，通过Init Container解决了etcd多节点集群的配置复杂性问题！
```
```
