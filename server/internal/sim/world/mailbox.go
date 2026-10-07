package world

import "thinhthan/internal/sim/runtime"

// MailboxCap bounds one partition/director mailbox. Overflow rejects —
// commands fail fast, consults get an admission error (ADR-0083).
const MailboxCap = 256

// Entry is one mailbox item: exactly one of Cmd/Con/Fn is set. Entries drain
// FIFO across both kinds inside the partition tick (ADR-0083: consults
// interleave with commands in receipt order). Fn is a raw tick-work slot
// for producers that must run inside the tick without a wire command
// (tests, one-shot migrations) — it executes before the entry's
// command/consult dispatch.
type Entry struct {
	Cmd *Command
	Con *Consult
	Fn  func(*runtime.Partition, *runtime.TickContext)
}

// Mailbox is the bounded FIFO intake of one partition (or the director).
// Producers are edge handlers and the world runtime; the single consumer
// is the owning goroutine's drain system.
type Mailbox struct {
	ch chan Entry
}

// NewMailbox constructs an empty mailbox.
func NewMailbox() *Mailbox {
	return &Mailbox{ch: make(chan Entry, MailboxCap)}
}

// Post enqueues one entry or returns ErrMailboxFull on overflow.
func (m *Mailbox) Post(e Entry) error {
	select {
	case m.ch <- e:
		return nil
	default:
		return ErrMailboxFull
	}
}

// Take removes the oldest entry; ok=false when empty.
func (m *Mailbox) Take() (Entry, bool) {
	select {
	case e := <-m.ch:
		return e, true
	default:
		return Entry{}, false
	}
}
