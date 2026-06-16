// Package keypool 提供多个 API key 的轮询与故障转移。
//
// 策略：
//  1. 启动时按输入顺序轮询
//  2. 调用成功时推进游标
//  3. 调用失败时：
//     - 401/403 → 标记为 cooldown 到 cooldown 结束
//     - 429 → 标记为 rate-limited
//     - 5xx/网络错误 → 标记为 cooldown
//  4. 选择下一个 key 时跳过 cooldown 的
//  5. 全部 cooldown 时返回错误
//
// 线程安全。
package keypool

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// KeyStatus 标识 key 状态。
type KeyStatus string

const (
	StatusAvailable   KeyStatus = "available"
	StatusCooldown    KeyStatus = "cooldown"
	StatusRateLimited KeyStatus = "rate_limited"
)

// FailureKind 失败原因分类。
type FailureKind string

const (
	FailureAuth    FailureKind = "auth"    // 401/403 - 永久/长期
	FailureRate    FailureKind = "rate"    // 429
	FailureServer  FailureKind = "server"  // 5xx
	FailureNetwork FailureKind = "network" // 超时/连接失败
	FailureUnknown FailureKind = "unknown" // 其它
)

// ErrNoAvailableKey 全部 key 不可用。
var ErrNoAvailableKey = errors.New("keypool: no available key")

// Settings 轮询配置。
type Settings struct {
	// Cooldown 失败后的冷却时间
	Cooldown time.Duration
	// RateLimitCooldown 限流后的冷却时间（通常更长）
	RateLimitCooldown time.Duration
	// CycleReset 轮询周期：超过这个时间没有 Next 调用，认为是新周期
	// 触发 cursor 重置（从头轮询）以及所有 cooldown key 重新评估
	CycleReset time.Duration
}

// DefaultSettings 默认设置。
func DefaultSettings() Settings {
	return Settings{
		Cooldown:          60 * time.Second,
		RateLimitCooldown: 120 * time.Second,
		CycleReset:        10 * time.Second,
	}
}

// keyState 单个 key 的状态。
type keyState struct {
	key         string
	status      KeyStatus
	cooldownEnd time.Time
	modifiedAt  time.Time // 最后修改时间（status/cooldownEnd 变化时更新）
	failCount   int
}

// Pool key 池。
type Pool struct {
	mu         sync.Mutex
	keys       []*keyState
	cursor     int       // 当前指向
	lastNextAt time.Time // 上次 Next 调用时间
	settings   Settings
}

// New 从 key 列表创建池。
// 允许空字符串，会被忽略。
func New(keys []string, settings Settings) *Pool {
	p := &Pool{
		keys:     make([]*keyState, 0, len(keys)),
		settings: settings,
	}
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		p.keys = append(p.keys, &keyState{key: k, status: StatusAvailable})
	}
	return p
}

// Size 返回 key 数量。
func (p *Pool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.keys)
}

// Next 选择下一个可用 key。
// 优先：当前 cursor 位置（如果可用），否则轮询找到下一个可用的。
// 找不到可用 key 时返回 ErrNoAvailableKey。
//
// "根据修改时间判断重新轮询"：
//   - 如果距上次 Next 超过 CycleReset，认为是新周期，重置 cursor 从头开始
//   - 如果整个池子已 N 秒没有活动（modifiedAt 都早于 now-CycleReset），
//     强制重新评估所有 key 的状态，把已过 cooldown 的恢复为 available
func (p *Pool) Next() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.keys) == 0 {
		return "", ErrNoAvailableKey
	}

	now := time.Now()

	// 周期重置：距上次 Next 超过 CycleReset → 重新从头轮询
	// 这种情况说明程序"空闲"了一段时间，所有 key 的 cooldown 应该都已经过期
	if !p.lastNextAt.IsZero() && now.Sub(p.lastNextAt) > p.settings.CycleReset {
		p.cursor = 0
	}

	// 清理已过期的 cooldown（基于 cooldownEnd 修改时间）
	for _, k := range p.keys {
		if k.status == StatusCooldown || k.status == StatusRateLimited {
			if now.After(k.cooldownEnd) {
				k.status = StatusAvailable
				k.failCount = 0
				k.modifiedAt = now
			}
		}
	}

	// 从 cursor 开始找第一个 available
	for i := 0; i < len(p.keys); i++ {
		idx := (p.cursor + i) % len(p.keys)
		k := p.keys[idx]
		if k.status == StatusAvailable {
			p.cursor = (idx + 1) % len(p.keys)
			p.lastNextAt = now
			return k.key, nil
		}
	}

	// 所有 key 都不可用
	p.lastNextAt = now
	return "", ErrNoAvailableKey
}

// MarkSuccess 标记当前 key 调用成功，重置失败计数。
func (p *Pool) MarkSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.keys) == 0 {
		return
	}
	// cursor 已经推进到下一个，当前 key 是上一个
	idx := (p.cursor - 1 + len(p.keys)) % len(p.keys)
	k := p.keys[idx]
	k.failCount = 0
	k.modifiedAt = time.Now()
}

// MarkFailed 标记上一个 Next() 返回的 key 调用失败。
// kind 决定冷却策略。
func (p *Pool) MarkFailed(kind FailureKind) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.keys) == 0 {
		return
	}
	idx := (p.cursor - 1 + len(p.keys)) % len(p.keys)
	k := p.keys[idx]
	p.markKeyFailed(k, kind, time.Now())
}

// MarkFailedByKey 标记指定 key 失败（用于外部直接传 key 调用的场景）。
func (p *Pool) MarkFailedByKey(key string, kind FailureKind) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, k := range p.keys {
		if k.key != key {
			continue
		}
		p.markKeyFailed(k, kind, time.Now())
		return
	}
}

// markKeyFailed 内部辅助：标记单个 key 失败（调用方需持锁）。
func (p *Pool) markKeyFailed(k *keyState, kind FailureKind, now time.Time) {
	var cooldown time.Duration
	switch kind {
	case FailureRate:
		cooldown = p.settings.RateLimitCooldown
		k.status = StatusRateLimited
	case FailureAuth, FailureServer, FailureNetwork, FailureUnknown:
		cooldown = p.settings.Cooldown
		k.status = StatusCooldown
	default:
		cooldown = p.settings.Cooldown
		k.status = StatusCooldown
	}
	k.cooldownEnd = now.Add(cooldown)
	k.modifiedAt = now
	k.failCount++
}

// MarkSuccessByKey 重置指定 key 的失败计数。
func (p *Pool) MarkSuccessByKey(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, k := range p.keys {
		if k.key == key {
			k.failCount = 0
			k.modifiedAt = time.Now()
		}
	}
}

// Status 返回所有 key 的状态（用于调试）。
type KeySnapshot struct {
	Index      int
	Key        string // 隐藏中间部分
	Status     KeyStatus
	FailCount  int
	CooldownIn time.Duration
}

func (p *Pool) Status() []KeySnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()

	out := make([]KeySnapshot, len(p.keys))
	now := time.Now()
	for i, k := range p.keys {
		var cd time.Duration
		if k.status == StatusCooldown || k.status == StatusRateLimited {
			cd = k.cooldownEnd.Sub(now)
			if cd < 0 {
				cd = 0
			}
		}
		out[i] = KeySnapshot{
			Index:      i,
			Key:        maskKey(k.key),
			Status:     k.status,
			FailCount:  k.failCount,
			CooldownIn: cd,
		}
	}
	return out
}

// maskKey 隐藏 key 中间部分（用于显示）。
func maskKey(key string) string {
	if len(key) < 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// CategorizeError 从 error 推断 FailureKind。
// 基于错误信息中常见的关键字。
// 这是一个启发式实现，简单场景下够用。
func CategorizeError(err error) FailureKind {
	if err == nil {
		return FailureUnknown
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "invalid api key") || strings.Contains(msg, "authentication"):
		return FailureAuth
	case strings.Contains(msg, "403") || strings.Contains(msg, "forbidden") || strings.Contains(msg, "permission"):
		return FailureAuth
	case strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") || strings.Contains(msg, "rate_limit") || strings.Contains(msg, "quota"):
		return FailureRate
	case strings.Contains(msg, "500") || strings.Contains(msg, "502") || strings.Contains(msg, "503") || strings.Contains(msg, "504") || strings.Contains(msg, "internal server") || strings.Contains(msg, "bad gateway") || strings.Contains(msg, "service unavailable"):
		return FailureServer
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") || strings.Contains(msg, "connection refused") || strings.Contains(msg, "eof") || strings.Contains(msg, "network") || strings.Contains(msg, "dial"):
		return FailureNetwork
	default:
		return FailureUnknown
	}
}

// String 友好输出。
func (s KeySnapshot) String() string {
	if s.Status == StatusAvailable {
		return fmt.Sprintf("[%d] %s available", s.Index, s.Key)
	}
	return fmt.Sprintf("[%d] %s %s (fails=%d, cooldown=%s)", s.Index, s.Key, s.Status, s.FailCount, s.CooldownIn.Round(time.Second))
}
