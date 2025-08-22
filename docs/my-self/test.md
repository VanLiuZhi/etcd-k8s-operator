## 测试目录

test/
├── unit/                    # 单元测试 (已完成)
│   ├── cluster_test.go     # 集中的单元测试
│   └── mocks/              # Mock对象
├── integration/            # 集成测试 (新开发)
│   ├── integration_test.go # 主要集成测试
│   ├── helpers.go          # 测试辅助工具
│   └── config.go           # 测试配置和场景
├── e2e/                    # E2E测试 (现有)
├── fixtures/               # 测试数据文件 (整理后)
│   ├── single-node-cluster.yaml
│   └── multinode-cluster.yaml
├── testdata/               # 测试数据 (现有)
├── utils/                  # 测试工具 (整理后)
├── scripts/                # 测试脚本 (新增)
│   ├── cleanup-reports.sh  # 清理脚本
│   └── generate-reports.sh # 报告生成脚本
└── report/                 # 测试报告目录 (新增)
    ├── unit-coverage.html
    ├── unit-coverage.out
    └── test-summary.md

# 当前工作目标：
1. 实现基于operator管理的etcd集群自动扩缩，故障恢复等功能。当前实现的代码问题非常多，准备完全重构
2. _Reference 目录下是 core/etcd-operator 项目，不要修改这个项目的任何代码，可以参考这个项目来修改代码
3. 当前项目的旧代码已经没有意义，包括文档等，都可以删除
4. 由于是重构，我已经验证过，core/etcd-operator的代码无法跑在 Kubernetes 1.28+ ，重构时候要修改代码，主要是k8s api部分

# 技术规范
etcd部署使用版本：
   version: "v3.5.21"
   repository: "quay.io/coreos/etcd"

- Go: 1.23.4+
- Docker: 20.10+
- Kubernetes: 1.28+ (推荐使用 Kind)
- Kubebuilder: 4.0.0+

kind集群已经具备，任何情况下禁止修改，要操作k8s，直接执行kubectl命令，如果遇到无法操作的情况，询问我

# 重构要求
按照我的要求一次只完成一个功能，不要完成过多功能，这样会导致重构步骤混乱，一次完成一个功能逐步推进
如果遇到问题，必须标注记录，解决问题，禁止绕过问题认为是成功了。如果解决有困难，暂停任务，询问我

# 规则要求
任何情况下，遵守.augment/rules 下规则文件



回到crd定义实现上，参考core/etcd-operator，先把etcd-operator也就是集群管理相关的crd定义实现，代码只保留crd定义相关的，删除其它全部代码，没有作用了。代码修改完成后，验证crd资源是否可以生成，并把crd安装到k8s中。创建todolist，开始


@/Users/liuzhi/GoProject/etcd-k8s-operator/docs/refactor/参考之前的分析结果，实现扩缩容代码。我看到cmd和controller还有代码，删除重新实现。要求实现的代码加上中文注释，特别要注意按照当前技术栈做出调整和修改，_Reference 目录下是 core/etcd-operator 项目，不要修改这个项目的任何代码，必须参考这个项目的核心逻辑来开发新代码，禁止自己实现新代码。代码实现后，能通过代码编译即可，等待我下一步指令。创建todolist，开始任务