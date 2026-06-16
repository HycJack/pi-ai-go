package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	core "pi-ai-go/core"
)

func main() {
	fmt.Println("=== 重试进度回调示例 ===\n")

	// 示例 1: 带进度显示的重试
	fmt.Println("示例 1: 带进度显示的重试")
	attempt := 0
	cfg := core.DefaultRetryConfig()
	cfg.OnRetry = func(attemptNum int, nextDelay time.Duration, err error) {
		attempt = attemptNum
		fmt.Printf("  [重试 %d/%d] %v 后重试 (错误: %v)\n",
			attemptNum, cfg.MaxRetries, nextDelay.Round(time.Millisecond), err)
	}

	err := core.Retry(context.Background(), cfg, func() error {
		if attempt < 2 {
			return errors.New("temporary error")
		}
		return nil
	})

	if err != nil {
		fmt.Printf("  最终失败: %v\n\n", err)
	} else {
		fmt.Printf("  成功！\n\n")
	}

	// 示例 2: 限流重试（带 RetryAfter）
	fmt.Println("示例 2: 限流重试（带 RetryAfter）")
	cfg2 := core.DefaultRetryConfig()
	cfg2.OnRetry = func(attemptNum int, nextDelay time.Duration, err error) {
		fmt.Printf("  [重试 %d/%d] %v 后重试\n", attemptNum, cfg2.MaxRetries, nextDelay.Round(time.Millisecond))
	}

	attempt = 0
	err = core.Retry(context.Background(), cfg2, func() error {
		if attempt == 0 {
			attempt++
			return fmt.Errorf("%s: rate limit exceeded", core.ProviderOpenAI)
		}
		return nil
	})

	if err != nil {
		fmt.Printf("  最终失败: %v\n\n", err)
	} else {
		fmt.Printf("  成功！\n\n")
	}

	// 示例 3: 不可重试错误（不会触发 OnRetry）
	fmt.Println("示例 3: 不可重试错误（不会触发 OnRetry）")
	cfg3 := core.DefaultRetryConfig()
	cfg3.OnRetry = func(attemptNum int, nextDelay time.Duration, err error) {
		fmt.Printf("  [重试 %d/%d] %v 后重试\n", attemptNum, cfg3.MaxRetries, nextDelay.Round(time.Millisecond))
	}

	err = core.Retry(context.Background(), cfg3, func() error {
		return core.ErrAuth // 认证错误不可重试
	})

	if err != nil {
		fmt.Printf("  最终失败: %v (未触发重试)\n\n", err)
	} else {
		fmt.Printf("  成功！\n\n")
	}

	// 示例 4: 超时错误（不可重试）
	fmt.Println("示例 4: 超时错误（不可重试）")
	cfg4 := core.DefaultRetryConfig()
	cfg4.OnRetry = func(attemptNum int, nextDelay time.Duration, err error) {
		fmt.Printf("  [重试 %d/%d] %v 后重试\n", attemptNum, cfg4.MaxRetries, nextDelay.Round(time.Millisecond))
	}

	err = core.Retry(context.Background(), cfg4, func() error {
		return core.WrapTimeout(core.TimeoutSourceHTTP, 5*time.Minute, nil)
	})

	if err != nil {
		fmt.Printf("  最终失败: %v (未触发重试)\n\n", err)
	} else {
		fmt.Printf("  成功！\n\n")
	}

	// 示例 5: 在 Agent 中使用（模拟）
	fmt.Println("示例 5: 在 Agent 中使用（模拟）")
	cfg5 := core.DefaultRetryConfig()
	cfg5.OnRetry = func(attemptNum int, nextDelay time.Duration, err error) {
		// 在实际应用中，这里可以通过事件流发送进度信息
		fmt.Printf("  ⏳ LLM 请求失败，%v 后重试 (尝试 %d/%d)\n",
			nextDelay.Round(time.Millisecond), attemptNum, cfg5.MaxRetries)
	}

	attempt = 0
	err = core.Retry(context.Background(), cfg5, func() error {
		if attempt < 1 {
			attempt++
			return errors.New("server error 503")
		}
		return nil
	})

	if err != nil {
		fmt.Printf("  最终失败: %v\n", err)
	} else {
		fmt.Printf("  成功！\n")
	}
}
