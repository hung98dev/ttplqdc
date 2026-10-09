package party

import (
	"context"

	"thinhthan/internal/core/id"
	runtime "thinhthan/internal/global/runtime"
)

// Family routes party commands through the global mailbox.
const Family = "party"

// Host adapts Service to the global runtime: Restore is a no-op
// (ephemeral state per party.md § Disconnect); OnCommand is a no-op —
// mutation happens inside each Command's Execute on the writer
// goroutine.
type Host struct {
	Svc *Service
}

// NewHost wraps svc for runtime.RegisterHost(h, party.Family).
func NewHost(svc *Service) *Host { return &Host{Svc: svc} }

func (h *Host) Name() string { return Family }

func (h *Host) Restore(context.Context, *runtime.DurableView) error { return nil }

func (h *Host) OnCommand(context.Context, runtime.Command) error { return nil }

// Command carries one party op through the mailbox. Read Out after the
// Submit result channel resolves.
type Command struct {
	Svc     *Service
	Out     *Result
	execute func(svc *Service) Result
}

func (c *Command) Family() string { return Family }

func (c *Command) Execute(context.Context, *runtime.Runtime) error {
	res := c.execute(c.Svc)
	if c.Out != nil {
		*c.Out = res
	}
	return nil
}

// Command constructors — one per request id.
func CmdInvite(svc *Service, out *Result, opID, inviter, target id.UUID) *Command {
	return &Command{Svc: svc, Out: out, execute: func(s *Service) Result {
		return s.Invite(opID, inviter, target)
	}}
}

func CmdAccept(svc *Service, out *Result, opID, char, partyID, inviter id.UUID) *Command {
	return &Command{Svc: svc, Out: out, execute: func(s *Service) Result {
		return s.Accept(opID, char, partyID, inviter)
	}}
}

func CmdDecline(svc *Service, out *Result, opID, char, partyID, inviter id.UUID) *Command {
	return &Command{Svc: svc, Out: out, execute: func(s *Service) Result {
		return s.Decline(opID, char, partyID, inviter)
	}}
}

func CmdCancelInvite(svc *Service, out *Result, opID, inviter, target id.UUID) *Command {
	return &Command{Svc: svc, Out: out, execute: func(s *Service) Result {
		return s.CancelInvite(opID, inviter, target)
	}}
}

func CmdLeave(svc *Service, out *Result, opID, char id.UUID) *Command {
	return &Command{Svc: svc, Out: out, execute: func(s *Service) Result {
		return s.Leave(opID, char)
	}}
}

func CmdKick(svc *Service, out *Result, opID, leader, target id.UUID) *Command {
	return &Command{Svc: svc, Out: out, execute: func(s *Service) Result {
		return s.Kick(opID, leader, target)
	}}
}

func CmdTransferLeader(svc *Service, out *Result, opID, leader, target id.UUID) *Command {
	return &Command{Svc: svc, Out: out, execute: func(s *Service) Result {
		return s.TransferLeader(opID, leader, target)
	}}
}

func CmdBoardPost(svc *Service, out *Result, opID, poster id.UUID, dungeonID string, size uint32, note string) *Command {
	return &Command{Svc: svc, Out: out, execute: func(s *Service) Result {
		return s.BoardPost(opID, poster, dungeonID, size, note)
	}}
}

func CmdBoardCancel(svc *Service, out *Result, opID, poster id.UUID) *Command {
	return &Command{Svc: svc, Out: out, execute: func(s *Service) Result {
		return s.BoardCancel(opID, poster)
	}}
}
