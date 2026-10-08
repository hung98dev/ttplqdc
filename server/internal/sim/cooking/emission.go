package cooking

import (
	"sync"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/sim/runtime"
)

// Settlement command identities (save_rules.md § Closed Producer
// Registry): sim emits intents only; durable/reward (kind "rest") and
// durable/beasts (kind "beast") apply them.
const (
	FamilyRestSettlement  = "sim.rest_settlement"
	FamilyBeastSettlement = "sim.beast_settlement"
	KindRest              = "rest"
	KindBeast             = "beast"
	// RestSourceKind is the claim-side source_type for rest-exp lines
	// (reward_claims.md closed enum — bonfire rest settles under
	// WORLD_EVENT).
	RestSourceKind = "WORLD_EVENT"
)

// Emission is the channel's settlement ledger: each queued
// DurableCommand carries its typed JournalRewardCommand payload until
// the edge emit adapter commits and drops it (the discovery.Emission
// contract — Payload/Drop per operation id).
type Emission struct {
	mu   sync.Mutex
	pend map[[16]byte]*journalv1.JournalRewardCommand
}

// NewEmission builds the ledger.
func NewEmission() *Emission {
	return &Emission{pend: map[[16]byte]*journalv1.JournalRewardCommand{}}
}

// emitRest queues one completed 10 s rest tick: REST settlement for
// the character's 1,000 rest EXP (durable/reward applies it against
// the 180/day cap in its own transaction).
func (c *Channel) emitRest(p *runtime.Partition, charID id.UUID, now uint64) {
	op := c.nextOp(FamilyRestSettlement, charID)
	slot := &journalv1.JournalRewardSlot{
		RewardSlot:      "rest.bonfire." + c.mapID + "." + charID.String(),
		CharacterExp:    RestExpPerTick,
		SourceType:      RestSourceKind,
		SourceReference: bonfireAnchorID(c.mapID),
	}
	c.queue(p, op, charID, FamilyRestSettlement,
		"bonfire.rest."+bonfireAnchorID(c.mapID),
		&journalv1.JournalRewardCommand{
			Source: c.journalSource(p, now),
			Kind:   KindRest,
			Slots:  []*journalv1.JournalRewardSlot{slot},
		})
}

// emitBond queues one completed bond interval: BEAST settlement for
// +1 companion bond (durable/beasts resolves the resting character's
// active companion and applies the 6/day cap; IMP-059 never picks the
// beast row itself).
func (c *Channel) emitBond(p *runtime.Partition, charID id.UUID, now uint64) {
	op := c.nextOp(FamilyBeastSettlement, charID)
	slot := &journalv1.JournalRewardSlot{
		RewardSlot:      "bond.bonfire." + c.mapID + "." + charID.String(),
		SourceType:      RestSourceKind,
		SourceReference: bonfireAnchorID(c.mapID),
	}
	c.queue(p, op, charID, FamilyBeastSettlement,
		"bonfire.bond."+bonfireAnchorID(c.mapID),
		&journalv1.JournalRewardCommand{
			Source: c.journalSource(p, now),
			Kind:   KindBeast,
			Slots:  []*journalv1.JournalRewardSlot{slot},
		})
}

// queue stages the typed payload and submits the flat durable command
// on the partition (EMIT_DURABLE_COMMANDS order next tick).
func (c *Channel) queue(p *runtime.Partition, op id.UUID, charID id.UUID,
	family, sourceEvent string, payload *journalv1.JournalRewardCommand) {
	c.emit.mu.Lock()
	c.emit.pend[op] = payload
	c.emit.mu.Unlock()
	_ = p.QueueCommand(runtime.DurableCommand{
		Kind:        runtime.CmdReward,
		Family:      family,
		OwnerKind:   runtime.OwnerCharacter,
		Owner:       charID,
		OperationID: op,
		SourceEvent: sourceEvent,
	})
}

// nextOp mints the deterministic settlement operation id —
// ServerJobOperationID makes a crash-retry emit the same id
// (idempotency: the consumer dedups on it).
func (c *Channel) nextOp(family string, charID id.UUID) id.UUID {
	c.seq++
	return id.ServerJobOperationID(family, c.mapID, charID.String(), seqName(c.seq))
}

func seqName(n uint64) string {
	const hexd = "0123456789abcdef"
	var b [16]byte
	for i := 15; i >= 0; i-- {
		b[i] = hexd[n&0xf]
		n >>= 4
	}
	return string(b[:])
}

// journalSource tags the settlement with the partition's sim provenance.
func (c *Channel) journalSource(p *runtime.Partition, now uint64) *journalv1.JournalSource {
	return &journalv1.JournalSource{
		MapId: c.mapID,
		Tick:  now,
	}
}

// Payload exposes a queued reward command for the emit adapter.
func (e *Emission) Payload(op [16]byte) (*journalv1.JournalRewardCommand, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	payload, ok := e.pend[op]
	return payload, ok
}

// Drop retires a queued command after the emit adapter's Submit
// succeeded.
func (e *Emission) Drop(op [16]byte) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.pend, op)
}
