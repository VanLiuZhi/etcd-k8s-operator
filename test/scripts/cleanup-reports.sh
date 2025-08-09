#!/bin/bash

# 测试报告清理脚本
# 用于清理根目录和test/report下的测试报告文件

set -e

echo "🧹 开始清理测试报告文件..."

# 获取脚本所在目录的父目录的父目录（即项目根目录）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
REPORT_DIR="$PROJECT_ROOT/test/report"

echo "📁 项目根目录: $PROJECT_ROOT"
echo "📁 报告目录: $REPORT_DIR"

# 清理根目录下的测试报告文件
echo "🗑️  清理根目录下的测试报告文件..."
cd "$PROJECT_ROOT"

# 删除覆盖率文件
rm -f *.out
echo "   ✅ 删除 *.out 文件"

# 删除HTML报告文件
rm -f *.html
echo "   ✅ 删除 *.html 文件"

# 删除其他测试相关文件
rm -f coverage.* test-*.xml junit-*.xml
echo "   ✅ 删除其他测试报告文件"

# 清理test/report目录
if [ -d "$REPORT_DIR" ]; then
    echo "🗑️  清理 test/report 目录..."
    rm -rf "$REPORT_DIR"/*
    echo "   ✅ 清理 test/report 目录完成"
else
    echo "📁 创建 test/report 目录..."
    mkdir -p "$REPORT_DIR"
fi

# 创建.gitkeep文件保持目录结构
touch "$REPORT_DIR/.gitkeep"

echo "✨ 测试报告清理完成！"
echo ""
echo "📋 清理内容："
echo "   - 根目录下的 *.out, *.html 文件"
echo "   - test/report 目录下的所有文件"
echo "   - 其他测试报告文件"
echo ""
echo "💡 使用方法："
echo "   chmod +x test/scripts/cleanup-reports.sh"
echo "   ./test/scripts/cleanup-reports.sh"
