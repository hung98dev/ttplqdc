package discovery

import (
	"sync"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/sim/runtime"
)

// Kind is the JournalRewardCommand.kind for first-discovery settlements.
const Kind = "discovery"

// Family is the registry family (save_rules.md § REWARD closed set).
const Family = "sim.discovery_settlement"

// SourceKind is the reward_claims source_type for first-discovery
// grants (messages.md §506).
const SourceKind = "DISCOVERY"

// RewardKey is the once-only grant identity
// reward.discovery.<map_id>.<character_id>.
func RewardKey(mapID string, characterID id.UUID) string {
	return "reward.discovery." + mapID + "." + characterID.String()
}

// Emission is one queued sim.discovery_settlement command plus the
// reward payload the emit adapter persists.
type Emission struct {
	mu   sync.Mutex
	pend map[[16]byte]*journalv1.JournalRewardCommand
}

// NewEmission builds the emission ledger.
func NewEmission() *Emission {
	return &Emission{pend: map[[16]byte]*journalv1.JournalRewardCommand{}}
}

// Queue builds the reward command and queues the flat
// sim.discovery_settlement durable command on the partition; emission
// happens in EMIT_DURABLE_COMMANDS order at the partition's next tick —
// the same contract as the checkpoint ledger. Callers invoke Queue only
// for a first-observed entry on a map with a discovery slot.
func (e *Emission) Queue(p *runtime.Partition, characterID id.UUID, mapID string,
	op id.UUID, src *journalv1.JournalSource) error {
	payload := &journalv1.JournalRewardCommand{
		Source:      src,
		Kind:        Kind,
		DiscoveryId: &mapID,
		Slots: []*journalv1.JournalRewardSlot{
			{RewardSlot: RewardKey(mapID, characterID)},
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
		SourceEvent: "discovery.first." + mapID,
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
