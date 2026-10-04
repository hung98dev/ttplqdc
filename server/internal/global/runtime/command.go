package runtime

import (
	"context"
	"errors"
)

// Mailbox errors (concurrency.md § Backpressure: typed errors over
// blocking; a full mailbox never blocks a simulation tick).
var (
	ErrMailboxFull          = errors.New("global: mailbox full")
	ErrNotRunning           = errors.New("global: runtime not running")
	ErrShutdown             = errors.New("global: runtime shut down")
	ErrUnknownCommandFamily = errors.New("global: unknown command family")
	ErrHostRestore          = errors.New("global: host restore failed")
)

// Command is one typed Global mutation. It runs on the single writer
// goroutine; Execute must not block on external waits (bounded work only).
type Command interface {
	// Family routes the command to its registered host.
	Family() string
	// Execute applies the command on the writer goroutine.
	Execute(ctx context.Context, rt *Runtime) error
}

// Result resolves a submitted command's outcome on its channel.
type Result struct {
	Err error
}

// commandFunc adapts a plain function to Command for simple hosts.
type commandFunc struct {
	family string
	fn     func(ctx context.Context, rt *Runtime) error
}

// Func wraps fn as a Command for family.
func Func(family string, fn func(ctx context.Context, rt *Runtime) error) Command {
	return &commandFunc{family: family, fn: fn}
}

func (c *commandFunc) Family() string { return c.family }

func (c *commandFunc) Execute(ctx context.Context, rt *Runtime) error {
	return c.fn(ctx, rt)
}
