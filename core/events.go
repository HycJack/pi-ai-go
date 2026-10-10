/*
 * 功能说明：事件流和流式事件类型定义
 * 
 * 解决的问题：
 * 1. 需要异步流式传输 LLM 响应
 * 2. 需要支持生产者-消费者模式的并发安全
 * 3. 需要定义统一的流式事件类型（文本、思考、工具调用等）
 * 4. 需要支持取消操作和错误处理
 * 
 * 解决方案：
 * 1. 实现 EventStream 泛型结构，支持并发安全的推送和消费
 * 2. 使用 channel 和 mutex 实现线程安全
 * 3. 定义 AssistantMessageEvent 接口和具体事件类型
 * 4. 提供 ForEach 方法支持 context 取消
 * 
 * 应用场景：
 * - 所有 AI 提供者使用 EventStream 返回流式响应
 * - Agent 层通过 ForEach 消费事件流
 * - 支持 SSE 和 WebSocket 传输
 */
package core

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// ErrStreamStopped is returned by Push after the consumer has stopped reading
// (typically because ForEach was cancelled). It signals the producer to abandon
// further work. Pushes after End/Error are ignored and return nil, matching PiG's
// semantics.
// || Push 在消费者停止读取（通常是 ForEach 被取消）后返回的错误，用于通知
// || 生产者放弃后续工作。End/Error 之后的 Push 被忽略并返回 nil（对齐 PiG）。
var ErrStreamStopped = errors.New("event stream stopped")

// EventStream is an async event stream for streaming LLM responses. It never
// drops events: Push appends to an unbounded queue and wakes a blocked
// consumer, mirroring PiG's AssistantMessageEventStream (lossless under
// backpressure). A single terminal result is set by End or Error; Result and
// iteration both observe it. Cancellation only releases blocked waiters and
// does not leak goroutines.
// || 异步事件流，用于流式传输 LLM 响应。事件永不丢弃：Push 追加到无界队列并
// || 唤醒阻塞的消费者（对齐 PiG 的无界队列语义）。End/Error 设置单一终止结果，
// || Result 与迭代都返回该结果。取消只释放等待者，不泄漏 goroutine。
type EventStream[T any, R any] struct {
	mu         sync.Mutex
	queue      []T           // 无界事件队列
	notify     chan struct{} // Push/终止时关闭并重建，用于唤醒消费者
	done       chan struct{} // End/Error 时关闭，标志终止
	result     R             // 最终结果
	err        error         // 最终错误
	terminated bool          // End/Error 是否已调用
	stopped    bool          // 消费者是否已停止读取
}

// streamEvt is the internal event type yielded by Events().
// || Events() 产生的内部事件类型
type streamEvt[T any] struct {
	value T
	err   error
	done  bool
}

// NewEventStream creates a new EventStream.
// || 创建新的 EventStream
func NewEventStream[T any, R any]() *EventStream[T, R] {
	return &EventStream[T, R]{
		notify: make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// Push appends an event to the stream. It never drops events: the event is
// queued and a blocked consumer is woken. Push returns nil on success, nil
// after End/Error (events following termination are ignored, matching PiG),
// and ErrStreamStopped after the consumer stopped reading.
// || 向流追加事件，永不丢弃：入队并唤醒阻塞的消费者。成功返回 nil；
// || End/Error 之后返回 nil（终止后的事件被忽略，对齐 PiG）；
// || 消费者停止读取后返回 ErrStreamStopped。
func (s *EventStream[T, R]) Push(event T) error {
	s.mu.Lock()
	if s.terminated {
		s.mu.Unlock()
		return nil
	}
	if s.stopped {
		s.mu.Unlock()
		return ErrStreamStopped
	}
	s.queue = append(s.queue, event)
	close(s.notify)
	s.notify = make(chan struct{})
	s.mu.Unlock()
	return nil
}

// End signals successful completion with a result. A subsequent End/Error is a
// no-op; Push after End is ignored.
// || 发送成功完成信号并附带结果。之后的 End/Error 为 no-op；End 之后的 Push 被忽略。
func (s *EventStream[T, R]) End(result R) {
	s.mu.Lock()
	if s.terminated {
		s.mu.Unlock()
		return
	}
	s.terminated = true
	s.result = result
	close(s.notify)
	s.mu.Unlock()
	close(s.done)
}

// Error signals an error and terminates the stream. A subsequent End/Error is a
// no-op; Push after Error is ignored.
// || 发送错误信号并终止流。之后的 End/Error 为 no-op；Error 之后的 Push 被忽略。
func (s *EventStream[T, R]) Error(err error) {
	s.mu.Lock()
	if s.terminated {
		s.mu.Unlock()
		return
	}
	s.terminated = true
	s.err = err
	close(s.notify)
	s.mu.Unlock()
	close(s.done)
}

// Stop signals the producer to stop sending events. It is called by the
// consumer path when iteration is cancelled; subsequent Push calls return
// ErrStreamStopped. Stop does not terminate the stream, so Result keeps
// blocking until End/Error.
// || 通知生产者停止发送事件。消费者取消迭代时调用；之后的 Push 返回
// || ErrStreamStopped。Stop 不终止流，Result 会一直阻塞到 End/Error。
func (s *EventStream[T, R]) Stop() {
	s.mu.Lock()
	if !s.stopped && !s.terminated {
		s.stopped = true
		close(s.notify)
	}
	s.mu.Unlock()
}

// Result waits for the stream to terminate and returns the final result.
// || 等待流终止并返回最终结果
func (s *EventStream[T, R]) Result() (R, error) {
	<-s.done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result, s.err
}

// Events returns a channel that yields stream events followed by a terminal
// marker. It is an alternative consumption path to ForEach; use one or the
// other, not both concurrently.
// || 返回产生流事件的 channel，随后产生一个终止标记。是 ForEach 之外的另一种
// || 消费方式；两者选其一，不要并发使用。
func (s *EventStream[T, R]) Events() <-chan streamEvt[T] {
	ch := make(chan streamEvt[T])
	go func() {
		_, err := s.ForEach(context.Background(), func(e T) error {
			ch <- streamEvt[T]{value: e}
			return nil
		})
		ch <- streamEvt[T]{err: err, done: true}
		close(ch)
	}()
	return ch
}

// ForEach iterates over all events in the stream, calling fn for each one.
// It returns the terminal result once the queue is drained and the stream is
// terminated, or ctx.Err() if ctx is cancelled. Cancelling ctx stops the stream
// but does not terminate it; the producer observes ErrStreamStopped on future
// Push calls.
// || 遍历流中的所有事件，对每个事件调用 fn。当队列排空且流已终止时返回最终
// || 结果；ctx 取消时返回 ctx.Err()。取消会停止流但不终止它，生产者后续 Push
// || 会收到 ErrStreamStopped。
// 参数：
//   ctx - 上下文（支持取消）
//   fn - 事件处理函数
// 返回：
//   最终结果和错误
func (s *EventStream[T, R]) ForEach(ctx context.Context, fn func(T) error) (R, error) {
	var zeroR R
	s.mu.Lock()
	for {
		// Drain the queue first so every event enqueued before termination is
		// delivered in order.
		if len(s.queue) > 0 {
			event := s.queue[0]
			s.queue = s.queue[1:]
			s.mu.Unlock()
			if err := fn(event); err != nil {
				s.Stop()
				return zeroR, err
			}
			s.mu.Lock()
			continue
		}
		// Queue empty + terminated: return the single terminal result.
		if s.terminated {
			r, e := s.result, s.err
			s.mu.Unlock()
			return r, e
		}
		if s.stopped {
			s.mu.Unlock()
			return zeroR, context.Canceled
		}
		notify := s.notify
		done := s.done
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			s.Stop()
			return zeroR, ctx.Err()
		case <-notify:
		case <-done:
		}
		s.mu.Lock()
	}
}

// --- Streaming Events ---
// || --- 流式事件类型 ---

// AssistantMessageEvent is the interface for all streaming events.
// || 所有流式事件的接口
type AssistantMessageEvent interface {
	eventTag()
}

// EventStart signals the start of a streaming response.
// || 表示流式响应开始
type EventStart struct {
	Type      string        `json:"type"`      // 类型：start
	API       KnownAPI      `json:"api"`       // API 协议
	Provider  KnownProvider `json:"provider"`  // 提供者
	Model     string        `json:"model"`     // 模型名称
	Timestamp time.Time     `json:"timestamp"` // 时间戳
}

func (EventStart) eventTag() {}

// EventTextStart signals the start of a text block.
// || 表示文本块开始
type EventTextStart struct {
	Type string `json:"type"` // 类型：text_start
}

func (EventTextStart) eventTag() {}

// EventTextDelta represents a text streaming delta.
// || 表示文本流式增量
type EventTextDelta struct {
	Type  string `json:"type"`  // 类型：text_delta
	Delta string `json:"delta"` // 增量文本
}

func (EventTextDelta) eventTag() {}

// EventTextEnd signals the end of a text block.
// || 表示文本块结束
type EventTextEnd struct {
	Type          string `json:"type"`                    // 类型：text_end
	TextSignature string `json:"textSignature,omitempty"` // 文本签名（用于 Anthropic）
}

func (EventTextEnd) eventTag() {}

// EventThinkingStart signals the start of a thinking block.
// || 表示思考块开始
type EventThinkingStart struct {
	Type string `json:"type"` // 类型：thinking_start
}

func (EventThinkingStart) eventTag() {}

// EventThinkingDelta represents a thinking streaming delta.
// || 表示思考流式增量
type EventThinkingDelta struct {
	Type  string `json:"type"`  // 类型：thinking_delta
	Delta string `json:"delta"` // 增量思考内容
}

func (EventThinkingDelta) eventTag() {}

// EventThinkingEnd signals the end of a thinking block.
// || 表示思考块结束
type EventThinkingEnd struct {
	Type              string `json:"type"`                    // 类型：thinking_end
	ThinkingSignature string `json:"thinkingSignature,omitempty"` // 思考签名
}

func (EventThinkingEnd) eventTag() {}

// EventToolCallStart signals the start of a tool call.
// || 表示工具调用开始
type EventToolCallStart struct {
	Type string `json:"type"` // 类型：tool_call_start
	ID   string `json:"id"`   // 工具调用 ID
	Name string `json:"name"` // 工具名称
}

func (EventToolCallStart) eventTag() {}

// EventToolCallDelta represents a tool call arguments delta.
// || 表示工具调用参数增量
type EventToolCallDelta struct {
	Type           string `json:"type"`           // 类型：tool_call_delta
	ID             string `json:"id"`             // 工具调用 ID
	ArgumentsDelta string `json:"argumentsDelta"` // 参数增量
}

func (EventToolCallDelta) eventTag() {}

// EventToolCallEnd signals the end of a tool call.
// || 表示工具调用结束
type EventToolCallEnd struct {
	Type      string          `json:"type"`      // 类型：tool_call_end
	ID        string          `json:"id"`        // 工具调用 ID
	Arguments json.RawMessage `json:"arguments"` // 完整参数（JSON）
}

func (EventToolCallEnd) eventTag() {}

// EventDone signals successful completion.
// || 表示成功完成
type EventDone struct {
	Type    string           `json:"type"`    // 类型：done
	Message AssistantMessage `json:"message"` // 最终消息
}

func (EventDone) eventTag() {}

// EventError signals an error.
// || 表示错误
type EventError struct {
	Type  string `json:"type"`  // 类型：error
	Error error  `json:"error"` // 错误信息
}

func (EventError) eventTag() {}

// AssistantMessageEventStream is a type alias for the event stream.
// || 事件流的类型别名
type AssistantMessageEventStream = EventStream[AssistantMessageEvent, AssistantMessage]

// CalculateCost computes the cost of a request from per-million-token rates.
// || 根据每百万 token 的费率计算请求费用
// 参数：
//   model - 模型信息（包含定价）
//   usage - token 使用统计
// 返回：
//   费用明细
func CalculateCost(model Model, usage Usage) CostBreakdown {
	// 计算各项费用：token 数 * 单价 / 1,000,000
	inputCost := float64(usage.Input) * model.Cost.Input / 1_000_000
	outputCost := float64(usage.Output) * model.Cost.Output / 1_000_000
	cacheReadCost := float64(usage.CacheRead) * model.Cost.CacheRead / 1_000_000
	cacheWriteCost := float64(usage.CacheWrite) * model.Cost.CacheWrite / 1_000_000

	return CostBreakdown{
		Input:      inputCost,
		Output:     outputCost,
		CacheRead:  cacheReadCost,
		CacheWrite: cacheWriteCost,
		Total:      inputCost + outputCost + cacheReadCost + cacheWriteCost,
	}
}
