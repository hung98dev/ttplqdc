package guild

import (
	"context"

	"time"

	"thinhthan/internal/core/id"
	durableguild "thinhthan/internal/durable/guild"
	runtime "thinhthan/internal/global/runtime"
)

// Family routes guild commands through the global mailbox.
const Family = "guild"

// Host adapts Service + Gathering to the global runtime: Restore is a
// no-op (gathering anchors and surge dedupe are ephemeral; durable
// state lives in durable/guild); OnCommand is a no-op — mutation
// happens inside each Command's Execute on the writer goroutine.
type Host struct {
	Svc *Service
	Gat *Gathering
}

// NewHost wraps the service for runtime.RegisterHost(h, guild.Family).
func NewHost(svc *Service, gat *Gathering) *Host { return &Host{Svc: svc, Gat: gat} }

func (h *Host) Name() string { return Family }

func (h *Host) Restore(context.Context, *runtime.DurableView) error { return nil }

func (h *Host) OnCommand(context.Context, runtime.Command) error { return nil }

// Command carries one guild op through the mailbox. Read Out after
// the Submit result channel resolves.
type Command struct {
	Svc     *Service
	Gat     *Gathering
	Out     *Result
	execute func(svc *Service, gat *Gathering) Result
}

func (c *Command) Family() string { return Family }

func (c *Command) Execute(ctx context.Context, _ *runtime.Runtime) error {
	res := c.execute(c.Svc, c.Gat)
	if c.Out != nil {
		*c.Out = res
	}
	return nil
}

// Result is the typed outcome of one host command — kept generic so
// the edge translates it into the durable enqueue (or a rejection
// before any durable command exists).
type Result struct {
	RequestMessageID uint32
	OperationID      id.UUID
	OK               bool
	Err              error
	GuildID          id.UUID
}

// CmdGuildEvent submits a typed SourceEvent to the sink.
func CmdGuildEvent(svc *Service, out *Result, ev durableguild.SourceEvent) *Command {
	return &Command{Svc: svc, Out: out, execute: func(s *Service, _ *Gathering) Result {
		err := s.Apply(context.Background(), ev)
		return Result{OK: err == nil, Err: err}
	}}
}

// CmdRestStart anchors a bonfire rest session.
func CmdRestStart(svc *Service, gat *Gathering, out *Result,
	char id.UUID, bonfireID, mapInstance string, at time.Time) *Command {
	return &Command{Svc: svc, Gat: gat, Out: out, execute: func(_ *Service, g *Gathering) Result {
		err := g.RestStart(context.Background(), char, bonfireID, mapInstance, at)
		return Result{OK: err == nil, Err: err}
	}}
}

// CmdRestStop drops a rest anchor.
func CmdRestStop(svc *Service, gat *Gathering, out *Result, char id.UUID) *Command {
	return &Command{Svc: svc, Gat: gat, Out: out, execute: func(_ *Service, g *Gathering) Result {
		g.RestStop(char)
		return Result{OK: true}
	}}
}
