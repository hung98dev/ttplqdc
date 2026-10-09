package atlas

import (
	"sync"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/sim/runtime"
)

// Kind is the JournalRewardCommand.kind routed to the Atlas consumer
// (save_rules.md §7).
const Kind = "atlas"

// Family is the registry family for Atlas settlements.
const Family = "sim.atlas_settlement"

// SourceKind is the reward_claims source_type for Atlas tier grants
// (messages.md §506 ATLAS_TIER progression events).
const SourceKind = "ATLAS"

// Emission is the queued-commands ledger for Atlas settlements: one
// JournalRewardCommand per source event, keyed by the event's
// deterministic operation id — the durable op ledger fences replays, so
// identical events can never double-count counters or double-grant a
// tier bundle.
type Emission struct {
	mu   sync.Mutex
	pend map[[16]byte]*journalv1.JournalRewardCommand
}

// NewEmission builds the emission ledger.
func NewEmission() *Emission {
	return &Emission{pend: map[[16]byte]*journalv1.JournalRewardCommand{}}
}

// HandleEvent resolves the source event against the compiled roster and
// queues one sim.atlas_settlement command carrying every matched page's
// atlas_progress delta in a single slot. The partition's deterministic
// source-event name + operation id are the at-least-once dedupe
// identity — retries of the same observation reuse them, so the durable
// ledger returns the recorded outcome instead of re-applying.
func (e *Emission) HandleEvent(p *runtime.Partition, characterID id.UUID,
	ev SourceEvent, src *journalv1.JournalSource) error {
	pages := Resolve(ev)
	if len(pages) == 0 {
		return nil
	}
	name, op, err := p.SourceEvent()
	if err != nil {
		return err
	}
	progress := make([]*journalv1.JournalAtlas, 0, len(pages))
	for _, page := range pages {
		progress = append(progress, &journalv1.JournalAtlas{
			PageId: page.ID,
			Delta:  ev.Value,
		})
	}
	payload := &journalv1.JournalRewardCommand{
		Source: src,
		Kind:   Kind,
		Slots: []*journalv1.JournalRewardSlot{
			{
				RewardSlot:    name,
				AtlasProgress: progress,
			},
		},
	}
	e.mu.Lock()
	e.pend[op] = payload
	e.mu.Unlock()
	return p.QueueCommand(runtime.DurableCommand{
		Kind:        runtime.CmdReward,
		Family:      Family,
		OwnerKind:   runtime.OwnerCharacter,
		Owner:       characterID,
		OperationID: op,
		SourceEvent: name,
	})
}

// Payload exposes a queued reward command for the emit adapter.
func (e *Emission) Payload(op [16]byte) (*journalv1.JournalRewardCommand, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	payload, ok := e.pend[op]
	return payload, ok
}

// Drop retires a queued command after the emit adapter's Submit was
// accepted.
func (e *Emission) Drop(op [16]byte) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.pend, op)
}
