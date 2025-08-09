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