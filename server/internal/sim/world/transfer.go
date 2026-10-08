package world

import (
	"time"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// transfer is one in-flight map transfer tracked by the director: the
// frozen→loading budget is enforced by Tick (TRANSFER_BUDGET_*).
type transfer struct {
	CharacterID id.UUID
	TransferID  id.UUID
	Kind        protocolv1.TransferReason
	SrcMap      string
	SrcChannel  uint32
	DstMap      string
	DstChannel  uint32
	DstSpawn    string
	FrozenAt    time.Time
	Budget      time.Duration
}

// beginTransfer registers the transfer and returns the prepare payload
// fields; callers emit S2C_TRANSFER_PREPARE on the source channel.
func (d *Director) beginTransfer(characterID, transferID id.UUID,
	kind protocolv1.TransferReason, dstMap string, dstChannel uint32,
	dstSpawn string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	m := d.members[characterID]
	budget := TransferBudgetWorld
	d.transfers[characterID] = &transfer{
		CharacterID: characterID,
		TransferID:  transferID,
		Kind:        kind,
		DstMap:      dstMap,
		DstChannel:  dstChannel,
		DstSpawn:    dstSpawn,
		FrozenAt:    d.now(),
		Budget:      budget,
	}
	if m != nil {
		d.transfers[characterID].SrcMap = m.MapID
		d.transfers[characterID].SrcChannel = m.Channel
	}
}

// transferFor returns the in-flight transfer, if any.
func (d *Director) transferFor(characterID id.UUID) (*transfer, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	t, ok := d.transfers[characterID]
	return t, ok
}

// completeTransfer drops the transfer once the destination admitted the
// player (the membership row already moved via Admitted).
func (d *Director) completeTransfer(characterID id.UUID) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.transfers, characterID)
}

// expiredTransfers returns transfers past their frozen budget — the
// runtime runs source recovery on each (checkpoint forced placement).
func (d *Director) expiredTransfers() []*transfer {
	now := d.now()
	d.mu.Lock()
	defer d.mu.Unlock()
	var out []*transfer
	for charID, t := range d.transfers {
		if now.Sub(t.FrozenAt) >= t.Budget {
			out = append(out, t)
			_ = charID
		}
	}
	return out
}
