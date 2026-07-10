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
	FailureAuth    FailureKind = "auth"
	FailureRate    FailureKind = "rate"
	FailureServer  FailureKind = "server"
	FailureNetwork FailureKind = "network"
	FailureUnknown FailureKind = "unknown"
)

// ErrNoAvailableKey 全部 key 不可用。
var ErrNoAvailableKey = errors.New("keypool: no available key")

// Settings 轮询配置。
type Settings struct {
	Cooldown          time.Duration
	RateLimitCooldown time.Duration
	CycleReset        time.Duration
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
	modifiedAt  time.Time
	failCount   int
}

// Pool key 池。
type Pool struct {
	mu         sync.Mutex
	keys       []*keyState
	cursor     int
	lastNextAt time.Time
	settings   Settings
}

// New 从 key 列表创建池。
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
func (p *Pool) Next() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.keys) == 0 {
		return "", ErrNoAvailableKey
	}

	now := time.Now()

	if !p.lastNextAt.IsZero() && now.Sub(p.lastNextAt) > p.settings.CycleReset {
		p.cursor = 0
	}

	for _, k := range p.keys {
		if k.status == StatusCooldown || k.status == StatusRateLimited {
			if now.After(k.cooldownEnd) {
				k.status = StatusAvailable
				k.failCount = 0
				k.modifiedAt = now
			}
		}
	}

	for i := 0; i < len(p.keys); i++ {
		idx := (p.cursor + i) % len(p.keys)
		k := p.keys[idx]
		if k.status == StatusAvailable {
			p.cursor = (idx + 1) % len(p.keys)
			p.lastNextAt = now
			return k.key, nil
		}
	}

	p.lastNextAt = now
	return "", ErrNoAvailableKey
}

// MarkSuccess 标记当前 key 调用成功。
func (p *Pool) MarkSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.keys) == 0 {
		return
	}
	idx := (p.cursor - 1 + len(p.keys)) % len(p.keys)
	k := p.keys[idx]
	k.failCount = 0
	k.modifiedAt = time.Now()
}

// MarkFailed 标记上一个 Next() 返回的 key 调用失败。
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

// markKeyFailed 内部辅助。
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

// KeySnapshot key 快照（用于显示）。
type KeySnapshot struct {
	Index      int
	Key        string
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

func maskKey(key string) string {
	if len(key) < 8 {
		return "****"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// CategorizeError 从 error 推断 FailureKind。
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

func (s KeySnapshot) String() string {
	if s.Status == StatusAvailable {
		return fmt.Sprintf("[%d] %s available", s.Index, s.Key)
	}
	return fmt.Sprintf("[%d] %s %s (fails=%d, cooldown=%s)", s.Index, s.Key, s.Status, s.FailCount, s.CooldownIn.Round(time.Second))
}
