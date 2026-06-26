# examples/agent PowerShell 运行脚本（Windows）
#
# 用法：
#   .\run.ps1                    # 默认 medium reasoning
#   .\run.ps1 -Reasoning high    # 指定思维链级别
#   .\run.ps1 -Query "计算 1+1"  # 单次提问
#   .\run.ps1 -Verbose           # 显示详细日志

param(
    [ValidateSet("off","minimal","low","medium","high","xhigh")]
    [string]$Reasoning = "",
    [string]$Query = "",
    [switch]$Verbose
)

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

# 加载 .env（如果存在）
$envFile = Join-Path $scriptDir ".env"
if (Test-Path $envFile) {
    Get-Content $envFile | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        $eq = $line.IndexOf("=")
        if ($eq -lt 1) { return }
        $key = $line.Substring(0, $eq).Trim()
        $val = $line.Substring($eq + 1).Trim()
        if (-not (Test-Path "Env:$key")) {
            Set-Item -Path "Env:$key" -Value $val
        }
    }
}

# 检查 API Key
if (-not $env:LLM_API_KEY -and -not $env:XIAOMI_API_KEY -and -not $env:SILICONFLOW_API_KEY) {
    Write-Host "❌ 错误: 请先在 .env 设置 LLM_API_KEY" -ForegroundColor Red
    Write-Host "  参考 .env.example" -ForegroundColor Yellow
    exit 1
}

Write-Host "✅ 启动 Agent..." -ForegroundColor Green
if ($Reasoning) { $env:LLM_REASONING = $Reasoning }

$args = @()
if ($Verbose)  { $args += "-v" }
if ($Query)    { $args += @("-query", $Query) }
if ($Reasoning) { $args += @("-reasoning", $Reasoning) }

& go run . @args
