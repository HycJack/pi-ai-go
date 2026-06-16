package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	core "pi-ai-go/core"
)

func main() {
	fmt.Println("=== 超时错误日志区分示例 ===\n")

	// 1. Agent 运行超时
	agentTimeout := core.WrapTimeout(core.TimeoutSourceAgent, 3*time.Minute, nil)
	fmt.Printf("Agent 运行超时:\n  错误: %v\n  来源: %s\n  时长: %v\n\n",
		agentTimeout,
		getTimeoutSource(agentTimeout),
		getTimeoutDuration(agentTimeout))

	// 2. HTTP 请求超时
	httpTimeout := core.WrapHTTPTimeout(core.ProviderOpenAI, 5*time.Minute, nil)
	fmt.Printf("HTTP 请求超时:\n  错误: %v\n  来源: %s\n  时长: %v\n  提供者: %s\n\n",
		httpTimeout,
		getTimeoutSource(httpTimeout),
		getTimeoutDuration(httpTimeout),
		getTimeoutProvider(httpTimeout))

	// 3. 工具执行超时
	toolTimeout := core.WrapToolTimeout("bash", 30*time.Second, nil)
	fmt.Printf("工具执行超时:\n  错误: %v\n  来源: %s\n  时长: %v\n  工具: %s\n\n",
		toolTimeout,
		getTimeoutSource(toolTimeout),
		getTimeoutDuration(toolTimeout),
		getTimeoutToolName(toolTimeout))

	// 4. 带自定义原因的超时
	customTimeout := core.WrapTimeout(core.TimeoutSourceAgent, 1*time.Minute, errors.New("database connection failed"))
	fmt.Printf("带自定义原因的超时:\n  错误: %v\n  来源: %s\n  原因: %v\n\n",
		customTimeout,
		getTimeoutSource(customTimeout),
		getTimeoutCause(customTimeout))

	// 5. 错误类型判断示例
	fmt.Println("=== 错误类型判断 ===")
	testErrors := []error{
		agentTimeout,
		httpTimeout,
		toolTimeout,
		context.DeadlineExceeded,
		core.ErrTimeout,
	}

	for i, err := range testErrors {
		fmt.Printf("\n错误 %d: %v\n", i+1, err)
		fmt.Printf("  errors.Is(err, context.DeadlineExceeded): %v\n", errors.Is(err, context.DeadlineExceeded))
		fmt.Printf("  errors.Is(err, core.ErrTimeout): %v\n", errors.Is(err, core.ErrTimeout))
		fmt.Printf("  core.IsRetryableError(err): %v\n", core.IsRetryableError(err))

		var timeoutErr *core.TimeoutError
		if errors.As(err, &timeoutErr) {
			fmt.Printf("  可解析为 *TimeoutError: 是\n")
			fmt.Printf("  - Source: %s\n", timeoutErr.Source)
			fmt.Printf("  - Duration: %v\n", timeoutErr.Duration)
			if timeoutErr.Provider != "" {
				fmt.Printf("  - Provider: %s\n", timeoutErr.Provider)
			}
			if timeoutErr.ToolName != "" {
				fmt.Printf("  - ToolName: %s\n", timeoutErr.ToolName)
			}
		} else {
			fmt.Printf("  可解析为 *TimeoutError: 否\n")
		}
	}
}

// 辅助函数：从错误中提取超时来源
func getTimeoutSource(err error) string {
	var timeoutErr *core.TimeoutError
	if errors.As(err, &timeoutErr) {
		return string(timeoutErr.Source)
	}
	return "unknown"
}

// 辅助函数：从错误中提取超时时长
func getTimeoutDuration(err error) time.Duration {
	var timeoutErr *core.TimeoutError
	if errors.As(err, &timeoutErr) {
		return timeoutErr.Duration
	}
	return 0
}

// 辅助函数：从错误中提取提供者
func getTimeoutProvider(err error) string {
	var timeoutErr *core.TimeoutError
	if errors.As(err, &timeoutErr) {
		return string(timeoutErr.Provider)
	}
	return ""
}

// 辅助函数：从错误中提取工具名称
func getTimeoutToolName(err error) string {
	var timeoutErr *core.TimeoutError
	if errors.As(err, &timeoutErr) {
		return timeoutErr.ToolName
	}
	return ""
}

// 辅助函数：从错误中提取原因
func getTimeoutCause(err error) error {
	var timeoutErr *core.TimeoutError
	if errors.As(err, &timeoutErr) {
		return timeoutErr.Cause
	}
	return nil
}