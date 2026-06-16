package keypool

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestNewFiltersEmpty(t *testing.T) {
	p := New([]string{"a", "", "  ", "b"}, DefaultSettings())
	if p.Size() != 2 {
		t.Errorf("Size = %d, want 2", p.Size())
	}
}

func TestNextRoundRobin(t *testing.T) {
	p := New([]string{"a", "b", "c"}, DefaultSettings())

	// 连续 Next 应该按顺序返回 a, b, c, a, ...
	want := []string{"a", "b", "c", "a", "b"}
	for i, w := range want {
		got, err := p.Next()
		if err != nil {
			t.Fatalf("Next #%d error: %v", i, err)
		}
		if got != w {
			t.Errorf("Next #%d = %q, want %q", i, got, w)
		}
	}
}

func TestNextEmpty(t *testing.T) {
	p := New([]string{}, DefaultSettings())
	_, err := p.Next()
	if !errors.Is(err, ErrNoAvailableKey) {
		t.Errorf("err = %v, want ErrNoAvailableKey", err)
	}
}

func TestNextSkipsCooldown(t *testing.T) {
	settings := Settings{
		Cooldown:          10 * time.Second,
		RateLimitCooldown: 10 * time.Second,
	}
	p := New([]string{"a", "b", "c"}, settings)

	// 取 a，标记失败 -> a 进入 cooldown
	k1, _ := p.Next()
	if k1 != "a" {
		t.Fatalf("first Next = %q, want a", k1)
	}
	p.MarkFailed(FailureAuth)

	// 再取应该跳到 b
	k2, _ := p.Next()
	if k2 != "b" {
		t.Errorf("second Next = %q, want b (skip a in cooldown)", k2)
	}

	// 再取应该到 c
	k3, _ := p.Next()
	if k3 != "c" {
		t.Errorf("third Next = %q, want c", k3)
	}

	// 再取 - a 还在 cooldown，循环回到 b
	k4, _ := p.Next()
	if k4 != "b" {
		t.Errorf("fourth Next = %q, want b (a still in cooldown)", k4)
	}
}

func TestMarkSuccessResets(t *testing.T) {
	settings := Settings{Cooldown: 10 * time.Second, RateLimitCooldown: 10 * time.Second}
	p := New([]string{"a", "b"}, settings)

	k1, _ := p.Next()
	if k1 != "a" {
		t.Fatalf("first = %q, want a", k1)
	}
	p.MarkFailed(FailureAuth)

	// 此时 a 失败，会跳过
	k2, _ := p.Next()
	if k2 != "b" {
		t.Errorf("second = %q, want b", k2)
	}

	// 标记 a 成功 - 模拟 key 恢复了（手动重置 cooldown）
	// 实际场景：cooldown 结束后会自动恢复
	// 这里测试 MarkSuccess 不会清 cooldown
	p.MarkSuccessByKey("a")

	// a 还在 cooldown
	k3, _ := p.Next()
	if k3 != "b" {
		t.Errorf("after success, Next = %q, want b (a still in cooldown)", k3)
	}
}

func TestCooldownExpires(t *testing.T) {
	settings := Settings{
		Cooldown:          50 * time.Millisecond,
		RateLimitCooldown: 100 * time.Millisecond,
	}
	p := New([]string{"a", "b"}, settings)

	k1, _ := p.Next()
	if k1 != "a" {
		t.Fatal("want a")
	}
	p.MarkFailed(FailureAuth)

	// 立即取，跳到 b
	k2, _ := p.Next()
	if k2 != "b" {
		t.Errorf("want b, got %q", k2)
	}

	// 等待 cooldown 过期
	time.Sleep(80 * time.Millisecond)

	// 此时 Next 内部会清理 cooldown，a 应该可用
	k3, _ := p.Next()
	if k3 != "a" {
		t.Errorf("after cooldown expires, Next = %q, want a (recovered)", k3)
	}
}

func TestAllInCooldown(t *testing.T) {
	settings := Settings{Cooldown: 1 * time.Hour, RateLimitCooldown: 1 * time.Hour}
	p := New([]string{"a", "b"}, settings)

	// 全部 fail
	_, _ = p.Next()
	p.MarkFailed(FailureAuth)
	_, _ = p.Next()
	p.MarkFailed(FailureAuth)

	// 此时全部 cooldown
	_, err := p.Next()
	if !errors.Is(err, ErrNoAvailableKey) {
		t.Errorf("err = %v, want ErrNoAvailableKey", err)
	}
}

func TestCategorizeError(t *testing.T) {
	tests := []struct {
		err  error
		want FailureKind
	}{
		{nil, FailureUnknown},
		{errors.New("401 unauthorized"), FailureAuth},
		{errors.New("invalid api key"), FailureAuth},
		{errors.New("403 forbidden"), FailureAuth},
		{errors.New("429 rate limit exceeded"), FailureRate},
		{errors.New("rate_limit hit"), FailureRate},
		{errors.New("500 internal server error"), FailureServer},
		{errors.New("502 bad gateway"), FailureServer},
		{errors.New("503 service unavailable"), FailureServer},
		{errors.New("context deadline exceeded"), FailureNetwork},
		{errors.New("connection refused"), FailureNetwork},
		{errors.New("eof"), FailureNetwork},
		{errors.New("random error"), FailureUnknown},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.err), func(t *testing.T) {
			got := CategorizeError(tt.err)
			if got != tt.want {
				t.Errorf("CategorizeError(%v) = %s, want %s", tt.err, got, tt.want)
			}
		})
	}
}

func TestStatus(t *testing.T) {
	settings := Settings{Cooldown: 1 * time.Hour, RateLimitCooldown: 1 * time.Hour}
	p := New([]string{"aaaaaaaa", "bbbbbbbb"}, settings)

	_, _ = p.Next() // a
	p.MarkFailed(FailureAuth)
	_, _ = p.Next() // b
	p.MarkFailed(FailureRate)

	snap := p.Status()
	if len(snap) != 2 {
		t.Fatalf("len = %d, want 2", len(snap))
	}

	if snap[0].Status != StatusCooldown {
		t.Errorf("snap[0] = %s, want cooldown", snap[0].Status)
	}
	if snap[1].Status != StatusRateLimited {
		t.Errorf("snap[1] = %s, want rate_limited", snap[1].Status)
	}

	// 验证 key 已被 mask
	if snap[0].Key != "aaaa...aaaa" {
		t.Errorf("snap[0].Key = %q, want aaaa...aaaa", snap[0].Key)
	}
}

func TestSnapshotString(t *testing.T) {
	s := KeySnapshot{Index: 0, Key: "abcd", Status: StatusAvailable, FailCount: 0}
	got := s.String()
	want := "[0] abcd available"
	if got != want {
		t.Errorf("String = %q, want %q", got, want)
	}
}

func TestConcurrentSafe(t *testing.T) {
	p := New([]string{"a", "b", "c"}, DefaultSettings())

	done := make(chan struct{}, 100)
	for i := 0; i < 100; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_, _ = p.Next()
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

// TestCycleReset 验证：超过 CycleReset 时间没有 Next 调用，
// 下次 Next 会重置 cursor 从头开始轮询（"根据修改时间重新轮询"）。
func TestCycleReset(t *testing.T) {
	settings := Settings{
		Cooldown:          1 * time.Hour, // cooldown 设长，让 a 仍处于 cooldown
		RateLimitCooldown: 1 * time.Hour,
		CycleReset:        100 * time.Millisecond,
	}
	p := New([]string{"a", "b", "c"}, settings)

	// 第一轮：a, b, c（每次 cursor +1，最后回到 0）
	for _, want := range []string{"a", "b", "c"} {
		got, _ := p.Next()
		if got != want {
			t.Errorf("first cycle: got %q, want %q", got, want)
		}
	}

	// 标记 a 失败（直接用 MarkFailedByKey，模拟某个 key 出错）
	p.MarkFailedByKey("a", FailureAuth)

	// 当前 cursor=0，下个 Next 应该从 0 开始扫：a 在 cooldown → 跳到 b
	k, _ := p.Next()
	if k != "b" {
		t.Errorf("after a fail: got %q, want b", k)
	}

	// 等待 CycleReset 时间过去
	time.Sleep(150 * time.Millisecond)

	// 下次 Next 触发周期重置，cursor 回到 0
	// a 还在 cooldown（1小时），所以应该返回 b（跳过 a）
	got, err := p.Next()
	if err != nil {
		t.Fatalf("Next after cycle reset: %v", err)
	}
	if got != "b" {
		t.Errorf("after cycle reset, Next = %q, want b (cursor reset to 0, a still in cooldown)", got)
	}

	// 验证 cursor 确实被重置：再 Next 应该是 c
	next, _ := p.Next()
	if next != "c" {
		t.Errorf("subsequent Next = %q, want c (cursor should be sequential after reset)", next)
	}
}

// TestModifiedAt 验证 modifiedAt 在状态变化时更新。
func TestModifiedAt(t *testing.T) {
	settings := Settings{Cooldown: 1 * time.Hour, RateLimitCooldown: 1 * time.Hour, CycleReset: 10 * time.Second}
	p := New([]string{"a", "b"}, settings)

	// 初始 modifiedAt 是 zero
	_, _ = p.Next() // a

	// 标记 a 失败
	p.MarkFailedByKey("a", FailureAuth)

	// 检查 a 的 modifiedAt 已更新
	p.mu.Lock()
	aModified := p.keys[0].modifiedAt
	p.mu.Unlock()

	if aModified.IsZero() {
		t.Error("a.modifiedAt should be updated after MarkFailed")
	}
	if time.Since(aModified) > 1*time.Second {
		t.Errorf("a.modifiedAt too old: %s", time.Since(aModified))
	}
}
