#!/bin/bash

# Agent 工具调用测试运行脚本

set -e

echo "🤖 Agent 工具调用测试"
echo "=========================================="

# 检查环境变量文件
if [ ! -f "../.env" ]; then
    echo "❌ 错误: 找不到 .env 文件"
    echo "请在项目根目录创建 .env 文件并配置以下变量:"
    echo "  SILICONFLOW_API_KEY=your_api_key"
    echo "  SILICONFLOW_BASE_URL=https://api.siliconflow.cn/v1"
    echo "  SILICONFLOW_MODEL=Qwen/Qwen2.5-7B-Instruct"
    exit 1
fi

# 检查 API Key
source ../.env
if [ -z "$SILICONFLOW_API_KEY" ]; then
    echo "❌ 错误: SILICONFLOW_API_KEY 未设置"
    exit 1
fi

echo "✅ 环境配置检查通过"
echo ""

# 显示菜单
show_menu() {
    echo "请选择运行模式:"
    echo "1) 交互式对话模式"
    echo "2) 运行所有自动化测试"
    echo "3) 运行计算器工具测试"
    echo "4) 运行天气查询测试"
    echo "5) 运行多工具测试"
    echo "6) 运行数据库查询测试"
    echo "7) 运行搜索工具测试"
    echo "8) 运行工具错误处理测试"
    echo "9) 运行状态管理测试"
    echo "10) 运行并行工具执行测试"
    echo "0) 退出"
    echo ""
    read -p "请输入选项 [0-10]: " choice
}

# 运行测试
run_test() {
    local test_name=$1
    echo ""
    echo "🚀 运行测试: $test_name"
    echo "------------------------------------------"
    go test -v -run "$test_name" -timeout 120s
}

# 主循环
while true; do
    show_menu
    
    case $choice in
        1)
            echo ""
            echo "🚀 启动交互式对话模式..."
            echo "------------------------------------------"
            go run main.go
            ;;
        2)
            echo ""
            echo "🚀 运行所有自动化测试..."
            echo "------------------------------------------"
            go test -v -run "TestRunAll" -timeout 300s
            ;;
        3)
            run_test "TestCalculatorTool"
            ;;
        4)
            run_test "TestWeatherTool"
            ;;
        5)
            run_test "TestMultipleTools"
            ;;
        6)
            run_test "TestDatabaseQuery"
            ;;
        7)
            run_test "TestSearchTool"
            ;;
        8)
            run_test "TestToolErrorHandling"
            ;;
        9)
            run_test "TestAgentStateManagement"
            ;;
        10)
            run_test "TestParallelToolExecution"
            ;;
        0)
            echo ""
            echo "👋 再见！"
            exit 0
            ;;
        *)
            echo "❌ 无效选项，请重新选择"
            ;;
    esac
    
    echo ""
    read -p "按 Enter 键继续..."
done
