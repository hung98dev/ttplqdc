package runtime

import (
	"errors"
	"time"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// Queue caps from docs/04_architecture/concurrency.md.
const (
	// MailboxCap is the partition command-queue cap.
	MailboxCap = 256
	// DiscreteCap is the per-character discrete-intent queue cap; discrete
	// intents are never coalesced, so overflow rejects.
	DiscreteCap = 8
	// ResultsCap is the durable-result drain budget per tick.
	ResultsCap = 64
	// RejectCap bounds the per-tick rejected-intent log.
	RejectCap = 64
)

// IntentKind selects the delivery and coalescing treatment an input
// receives (messages.md delivery classes; ADR-0038).
type IntentKind int

const (
	// IntentMovementHeld is replaceable input state: only the newest
	// pending held value matters, so it coalesces into a one-slot
	// per-character queue.
	IntentMovementHeld IntentKind = iota
	// IntentMovementEdge is C2S_MOVEMENT_EDGE: discrete intent, never
	// coalesced (ADR-0038).
	IntentMovementEdge
	// IntentAction is generic discrete gameplay intent.
	IntentAction
	// IntentBaselineAck is C2S_BASELINE_ACK (306).
	IntentBaselineAck
	// IntentBaselineResync is C2S_BASELINE_RESYNC_REQUEST (307).
	IntentBaselineResync
)

// Intent is one validated, typed input crossing into the partition mailbox.
// The edge constructs it; payloads stay flat values so enqueue costs zero
// allocations.
type Intent struct {
	Kind      IntentKind
	EntityID  uint64
	ClientSeq uint64

	Held       protocolv1.HeldHorizontalIntent // IntentMovementHeld
	Edge       uint8                           // IntentMovementEdge
	BaselineID uint64                          // IntentBaselineAck
	RequestID  uint64                          // IntentBaselineResync
	Reason     protocolv1.ResyncReason         // IntentBaselineResync

	// QueuedAt stamps mailbox entry for the queue-wait metric; the
	// partition sets it, callers leave it zero.
	QueuedAt time.Duration
}

// RejectCode explains a dropped input after it left the mailbox.
type RejectCode uint8

const (
	RejectNotPlayer    RejectCode = iota + 1 // target entity missing or not a player
	RejectStaleSeq                           // client_seq not newer than last processed
	RejectDiscreteFull                       // per-character discrete queue at cap
)

// Rejected is one input dropped at ingest, surfaced for observability.
type Rejected struct {
	Intent Intent
	Code   RejectCode
}

var (
	// ErrOverload reports the partition mailbox at cap; the caller
	// retries or degrades.
	ErrOverload = errors.New("runtime: partition command queue overload")
	// ErrNotStarted reports inputs or ticks before the partition start
	// gate (durable state load) completed.
	ErrNotStarted = errors.New("runtime: partition not started")
)

// inbox is the per-player second-stage queue: a one-slot coalescing cell
// for held input state plus an ordered discrete queue at cap 8.
type inbox struct {
	hasHeld   bool
	held      protocolv1.HeldHorizontalIntent
	heldSeq   uint64
	discrete  [DiscreteCap]Intent
	discreteN int
}

// SubmitIntent enqueues a typed input. It is safe to call from outside the
// owning goroutine: the mailbox is the only input handoff, and every state
// mutation happens inside tick on the owner goroutine.
func (p *Partition) SubmitIntent(i Intent) error {
	i.QueuedAt = p.cfg.Now()
	select {
	case p.mailbox <- i:
		return nil
	default:
		return ErrOverload
	}
}

// Enqueue satisfies IntentInPort.
func (p *Partition) Enqueue(i Intent) error { return p.SubmitIntent(i) }

// routeIntent moves one mailbox entry into its per-character inbox or into
// control state. Runs inside tick only.
func (p *Partition) routeIntent(i Intent, rejected *[RejectCap]Rejected, rejectedN *int) {
	reject := func(code RejectCode) {
		if *rejectedN < RejectCap {
			rejected[*rejectedN] = Rejected{Intent: i, Code: code}
			*rejectedN++
		}
	}
	e := p.entityByID(i.EntityID)
	if e == nil || e.Class != ClassPlayer {
		reject(RejectNotPlayer)
		return
	}
	c := &p.clients[e.Slot]

	switch i.Kind {
	case IntentBaselineAck:
		if i.BaselineID == c.baselineID {
			c.baselineAcked = true
		}
		return
	case IntentBaselineResync:
		c.resyncPending = true
		c.resyncReq = i.RequestID
		c.resyncReason = i.Reason
		return
	}

	// Client seq dedup runs against the greatest seq already applied or
	// still queued this tick, so an out-of-order resend inside one
	// mailbox drain can never regress lastClientSeq.
	ib := &p.inboxes[e.Slot]
	maxSeq := e.lastClientSeq
	if ib.hasHeld && ib.heldSeq > maxSeq {
		maxSeq = ib.heldSeq
	}
	for k := 0; k < ib.discreteN; k++ {
		if ib.discrete[k].ClientSeq > maxSeq {
			maxSeq = ib.discrete[k].ClientSeq
		}
	}
	if i.ClientSeq <= maxSeq {
		reject(RejectStaleSeq)
		return
	}
	switch i.Kind {
	case IntentMovementHeld:
		// Coalesce slot: newest pending held value wins.
		ib.hasHeld = true
		ib.held = i.Held
		ib.heldSeq = i.ClientSeq
	case IntentMovementEdge, IntentAction:
		if ib.discreteN >= DiscreteCap {
			reject(RejectDiscreteFull)
			return
		}
		ib.discrete[ib.discreteN] = i
		ib.discreteN++
	}
}

// applyInboxes applies each player's pending inputs in mailbox-receive
// order within the tick — INGEST_PLAYER_COMMANDS.
func (p *Partition) applyInboxes() {
	for slot := 0; slot < PlayersCap; slot++ {
		if !p.used[slot] {
			continue
		}
		e := &p.ent[slot]
		ib := &p.inboxes[slot]
		maxSeq := e.lastClientSeq
		if ib.hasHeld {
			e.Checkpoint.HeldHorizontalIntent = ib.held
			e.hasHeld = true
			if ib.heldSeq > maxSeq {
				maxSeq = ib.heldSeq
			}
			ib.hasHeld = false
		}
		// Edges are tick-scoped inputs: this tick's consumer drains them
		// before the next ingest resets the queue.
		e.pendingEdgeN = 0
		for k := 0; k < ib.discreteN; k++ {
			d := &ib.discrete[k]
			if d.Kind == IntentMovementEdge {
				e.pendingEdges[e.pendingEdgeN] = d.Edge
				e.pendingEdgeN++
			}
			if d.ClientSeq > maxSeq {
				maxSeq = d.ClientSeq
			}
		}
		e.lastClientSeq = maxSeq
		ib.discreteN = 0
	}
}
