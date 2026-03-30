package adapter

import (
	"context"
	"os/exec"
)

// Executor abstracts command execution for testability.
type Executor interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ShellExecutor runs commands via os/exec.
type ShellExecutor struct{}

func (ShellExecutor) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}
