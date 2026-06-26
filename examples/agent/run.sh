#!/bin/bash
# examples/agent 运行脚本（Linux / macOS / WSL / Git Bash）
#
# 用法：
#   ./run.sh                       # 默认 medium reasoning
#   ./run.sh -reasoning high       # 指定思维链级别
#   ./run.sh -query "计算 1+1"     # 单次提问
#   ./run.sh -v                    # 显示详细日志

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 加载 .env（如果存在）
if [ -f .env ]; then
    set -a
    # shellcheck disable=SC1091
    source .env
    set +a
fi

# 检查 API Key
if [ -z "${LLM_API_KEY:-}" ] && [ -z "${XIAOMI_API_KEY:-}" ] && [ -z "${SILICONFLOW_API_KEY:-}" ]; then
    echo -e "\033[31m❌ 错误: 请先在 .env 设置 LLM_API_KEY\033[0m"
    echo -e "\033[33m  参考 .env.example\033[0m"
    exit 1
fi

echo -e "\033[32m✅ 启动 Agent...\033[0m"
exec go run . "$@"
