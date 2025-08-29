# CLAUDE.md

此文件为 Claude Code (claude.ai/code) 在此仓库中工作提供指导。

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

# 回答要求
任何时刻，使用中文回答
- 不要管重构问题