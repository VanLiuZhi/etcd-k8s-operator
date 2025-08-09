#!/bin/bash

# 测试报告生成脚本
# 统一生成各种测试报告到test/report目录

set -e

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REPORT_DIR="$PROJECT_ROOT/test/report"

echo "📊 开始生成测试报告..."
echo "📁 项目根目录: $PROJECT_ROOT"
echo "📁 报告目录: $REPORT_DIR"

# 确保报告目录存在
mkdir -p "$REPORT_DIR"

cd "$PROJECT_ROOT"

# 1. 单元测试报告
echo ""
echo "🧪 生成单元测试报告..."
go test -v -coverprofile="$REPORT_DIR/unit-coverage.out" \
    -coverpkg=./pkg/service/...,./pkg/resource/...,./internal/controller/... \
    ./test/unit/cluster_test.go > "$REPORT_DIR/unit-test.log" 2>&1

if [ $? -eq 0 ]; then
    echo "   ✅ 单元测试通过"
    
    # 生成HTML覆盖率报告
    go tool cover -html="$REPORT_DIR/unit-coverage.out" -o "$REPORT_DIR/unit-coverage.html"
    echo "   ✅ 单元测试覆盖率报告: test/report/unit-coverage.html"
    
    # 生成覆盖率统计
    go tool cover -func="$REPORT_DIR/unit-coverage.out" > "$REPORT_DIR/unit-coverage-summary.txt"
    echo "   ✅ 单元测试覆盖率统计: test/report/unit-coverage-summary.txt"
    
    # 显示覆盖率摘要
    echo "   📈 覆盖率摘要:"
    tail -1 "$REPORT_DIR/unit-coverage-summary.txt" | awk '{print "      总覆盖率: " $3}'
else
    echo "   ❌ 单元测试失败，查看日志: test/report/unit-test.log"
fi

# 2. 集成测试报告（如果存在）
if [ -f "./test/integration/integration_test.go" ]; then
    echo ""
    echo "🔗 生成集成测试报告..."
    # 只运行我们新创建的集成测试，避免envtest问题
    go test -v -timeout=10m \
        ./test/integration/integration_test.go \
        ./test/integration/helpers.go \
        ./test/integration/config.go \
        ./test/integration/simple_test.go > "$REPORT_DIR/integration-test.log" 2>&1

    if [ $? -eq 0 ]; then
        echo "   ✅ 集成测试通过"
    else
        echo "   ❌ 集成测试失败，查看日志: test/report/integration-test.log"
    fi
fi

# 3. 生成测试摘要报告
echo ""
echo "📋 生成测试摘要报告..."
cat > "$REPORT_DIR/test-summary.md" << EOF
# 测试报告摘要

生成时间: $(date '+%Y-%m-%d %H:%M:%S')

## 单元测试结果

EOF

if [ -f "$REPORT_DIR/unit-coverage-summary.txt" ]; then
    echo "### 覆盖率统计" >> "$REPORT_DIR/test-summary.md"
    echo '```' >> "$REPORT_DIR/test-summary.md"
    cat "$REPORT_DIR/unit-coverage-summary.txt" >> "$REPORT_DIR/test-summary.md"
    echo '```' >> "$REPORT_DIR/test-summary.md"
fi

cat >> "$REPORT_DIR/test-summary.md" << EOF

## 报告文件

- [单元测试覆盖率HTML报告](unit-coverage.html)
- [单元测试覆盖率统计](unit-coverage-summary.txt)
- [单元测试日志](unit-test.log)

## 使用说明

1. 打开 \`unit-coverage.html\` 查看详细的代码覆盖率
2. 查看 \`unit-coverage-summary.txt\` 了解覆盖率统计
3. 如有测试失败，查看对应的日志文件

EOF

echo "   ✅ 测试摘要报告: test/report/test-summary.md"

echo ""
echo "✨ 测试报告生成完成！"
echo ""
echo "📁 报告文件位置: test/report/"
echo "🌐 打开HTML报告: open test/report/unit-coverage.html"
