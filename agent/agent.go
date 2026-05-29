package agent

import (
	"context"
	"sync"

	piai "pi-ai-go"
)

// AgentState holds the agent's mutable state.
type AgentState struct {
	Model        piai.Model
	SystemPrompt string
	Messages     []piai.Message
	Tools        []AgentTool
	ToolExecution ToolExecutionMode

	// Options forwarded to AgentLoopConfig
	ConvertToLlm       func([]piai.Message) []piai.Message
	TransformContext    func([]piai.Message) []piai.Message
	GetApiKey          func() string
	ShouldStopAfterTurn func(piai.AssistantMessage, []piai.ToolResultMessage) bool
	PrepareNextTurn     func(*AgentLoopConfig, piai.AssistantMessage, []piai.ToolResultMessage, []piai.Message)
	BeforeToolCall      func(BeforeToolCallContext) *ToolCallBlock
	AfterToolCall       func(AfterToolCallContext) *ToolCallOverride
	StreamFn            StreamFn
	SimpleStreamOptions piai.SimpleStreamOptions
}

// AgentOptions configures a new Agent.
type AgentOptions struct {
	InitialState *AgentState
}

// Agent is a stateful wrapper around the agent loop.
type Agent struct {
	mu          sync.RWMutex
	state       AgentState
	subscribers []func(AgentEvent)
	steering    []piai.Message
	followUp    []piai.Message
	cancel      context.CancelFunc
	streamWg    sync.WaitGroup // tracks processStream goroutine completion
}

// New creates a new Agent.
func New(opts AgentOptions) *Agent {
	a := &Agent{}
	if opts.InitialState != nil {
		a.state = *opts.InitialState
	}
	if a.state.Messages == nil {
		a.state.Messages = make([]piai.Message, 0)
	}
	return a
}

// State returns a copy of the agent's current state.
func (a *Agent) State() AgentState {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state
}

// SetTools updates the agent's tools.
func (a *Agent) SetTools(tools []AgentTool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.Tools = tools
}

// SetModel updates the agent's model.
func (a *Agent) SetModel(model piai.Model) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.Model = model
}

// SetSystemPrompt updates the system prompt.
func (a *Agent) SetSystemPrompt(prompt string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state.SystemPrompt = prompt
}

// Messages returns the current message history.
func (a *Agent) Messages() []piai.Message {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state.Messages
}

// Subscribe registers a listener for agent events.
func (a *Agent) Subscribe(fn func(AgentEvent)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.subscribers = append(a.subscribers, fn)
}

// Steering injects messages that will be processed in the current turn.
func (a *Agent) Steering(msgs ...piai.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.steering = append(a.steering, msgs...)
}

// FollowUp injects messages that will be processed after the current turn.
func (a *Agent) FollowUp(msgs ...piai.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.followUp = append(a.followUp, msgs...)
}

// Abort cancels the current run.
func (a *Agent) Abort() {
	a.mu.Lock()
	cancel := a.cancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Run starts a new agent run with the given prompts.
func (a *Agent) Run(ctx context.Context, prompts ...piai.Message) ([]piai.Message, error) {
	a.mu.Lock()
	// Append prompts to messages
	a.state.Messages = append(a.state.Messages, prompts...)

	// Create cancellable context
	runCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	// Build config
	config := a.buildConfig()

	// Copy steering/followUp and clear
	steering := a.steering
	a.steering = nil
	followUp := a.followUp
	a.followUp = nil
	a.mu.Unlock()

	// Override getSteering/getFollowUp to use our captured queues
	config.GetSteeringMessages = func() []piai.Message {
		a.mu.Lock()
		msgs := steering
		steering = nil
		a.mu.Unlock()
		return msgs
	}
	config.GetFollowUpMessages = func() []piai.Message {
		a.mu.Lock()
		msgs := followUp
		followUp = nil
		a.mu.Unlock()
		return msgs
	}

	// Run
	stream := AgentLoop(runCtx, prompts, config)
	a.processStream(stream)

	result, err := stream.Result()
	if err != nil {
		a.streamWg.Wait()
		return nil, err
	}

	// Wait for processStream to finish updating state before overwriting
	a.streamWg.Wait()

	a.mu.Lock()
	a.state.Messages = result
	a.cancel = nil
	a.mu.Unlock()

	return result, nil
}

// RunContinue resumes the agent from its current message history.
func (a *Agent) RunContinue(ctx context.Context) ([]piai.Message, error) {
	a.mu.Lock()

	runCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	config := a.buildConfig()

	steering := a.steering
	a.steering = nil
	followUp := a.followUp
	a.followUp = nil

	messages := make([]piai.Message, len(a.state.Messages))
	copy(messages, a.state.Messages)
	a.mu.Unlock()

	config.GetSteeringMessages = func() []piai.Message {
		a.mu.Lock()
		msgs := steering
		steering = nil
		a.mu.Unlock()
		return msgs
	}
	config.GetFollowUpMessages = func() []piai.Message {
		a.mu.Lock()
		msgs := followUp
		followUp = nil
		a.mu.Unlock()
		return msgs
	}

	stream := AgentLoopContinue(runCtx, config, messages)
	a.processStream(stream)

	result, err := stream.Result()
	if err != nil {
		a.streamWg.Wait()
		return nil, err
	}

	// Wait for processStream to finish updating state before overwriting
	a.streamWg.Wait()

	a.mu.Lock()
	a.state.Messages = result
	a.cancel = nil
	a.mu.Unlock()

	return result, nil
}

// buildConfig creates an AgentLoopConfig from the agent's state.
func (a *Agent) buildConfig() AgentLoopConfig {
	return AgentLoopConfig{
		SimpleStreamOptions: a.state.SimpleStreamOptions,
		Model:               a.state.Model,
		SystemPrompt:        a.state.SystemPrompt,
		Tools:               a.state.Tools,
		ToolExecution:       a.state.ToolExecution,
		ConvertToLlm:        a.state.ConvertToLlm,
		TransformContext:     a.state.TransformContext,
		GetApiKey:           a.state.GetApiKey,
		ShouldStopAfterTurn: a.state.ShouldStopAfterTurn,
		PrepareNextTurn:     a.state.PrepareNextTurn,
		BeforeToolCall:      a.state.BeforeToolCall,
		AfterToolCall:       a.state.AfterToolCall,
		StreamFn:            a.state.StreamFn,
	}
}

// processStream subscribes to the event stream and forwards events to subscribers.
func (a *Agent) processStream(stream *AgentEventStream) {
	a.mu.RLock()
	subs := make([]func(AgentEvent), len(a.subscribers))
	copy(subs, a.subscribers)
	a.mu.RUnlock()

	a.streamWg.Add(1)

	// Process events in a goroutine
	go func() {
		defer a.streamWg.Done()
		stream.ForEach(context.Background(), func(evt AgentEvent) error {
			// Update state based on events
			a.mu.Lock()
			switch e := evt.(type) {
			case EventMessageEnd:
				a.state.Messages = append(a.state.Messages, e.Message)
			}
			a.mu.Unlock()

			// Forward to subscribers
			for _, fn := range subs {
				fn(evt)
			}
			return nil
		})
	}()
}
