package core

import (
	"context"
)

type execEnvKey struct{}

// WithExecutionEnv returns a new context with the ExecutionEnv attached.
func WithExecutionEnv(ctx context.Context, execEnv ExecutionEnv) context.Context {
	return context.WithValue(ctx, execEnvKey{}, execEnv)
}

// GetExecutionEnv retrieves the ExecutionEnv from the context.
// Returns nil if no ExecutionEnv is set or ctx is nil.
func GetExecutionEnv(ctx context.Context) ExecutionEnv {
	if ctx == nil {
		return nil
	}
	execEnv, _ := ctx.Value(execEnvKey{}).(ExecutionEnv)
	return execEnv
}