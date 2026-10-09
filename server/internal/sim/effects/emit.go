package effects

// emit.go — the replication surface: S2CStatusEvent (205) emission
// through the partition OutboundPort and Snap.Statuses fill with the
// deterministic 16-slot eviction (creation seq) of
// status_effects.md § Replication.

import (
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/replication"
	"thinhthan/internal/sim/runtime"
)

// MsgS2CStatusEvent is the wire id of S2C_STATUS_EVENT.
const MsgS2CStatusEvent = 205

// FillStatuses writes the target's active instances into
// snap.Statuses (cap 16). When more than 16 instances are live the
// oldest creation seqs win — deterministic eviction.
func FillStatuses(snap *replication.EntitySnapshot, book *Book, now uint64) {
	var live []*Instance
	for _, i := range book.inst {
		if i.active(now) {
			live = append(live, i)
		}
	}
	// Eviction: oldest creation seq wins the 16 slots.
	for i := 1; i < len(live); i++ {
		for j := i; j > 0; j-- {
			if live[j].seq < live[j-1].seq {
				live[j-1], live[j] = live[j], live[j-1]
			}
		}
	}
	if len(live) > replication.MaxStatuses {
		live = live[:replication.MaxStatuses]
	}
	// Wire order: determinism order (source, effect, seq).
	for i := 1; i < len(live); i++ {
		for j := i; j > 0; j-- {
			x, y := live[j-1], live[j]
			if y.SourceID < x.SourceID ||
				(y.SourceID == x.SourceID && y.EffectID < x.EffectID) ||
				(y.SourceID == x.SourceID && y.EffectID == x.EffectID && y.seq < x.seq) {
				live[j-1], live[j] = live[j], live[j-1]
			}
		}
	}
	snap.StatusN = len(live)
	for i, inst := range live {
		snap.Statuses[i] = replication.StatusEntry{
			EffectID:       inst.EffectID,
			SourceEntityID: inst.SourceID,
			Stacks:         uint32(inst.Stacks),
			ExpiresAtTick:  inst.ExpiresAtTick,
		}
	}
}

// emitAll converts results into S2CStatusEvent frames through the
// injected OutboundPort. Only 205 kinds emit (applied, refreshed,
// stack change, expired, dispelled, consumed); damage/shield results
// are the caller's pipeline output.
func (s *System) emitAll(results []Result, now uint64) {
	for _, r := range results {
		var ev protocolv1.StatusEventKind
		switch r.Kind {
		case ResultApplied:
			ev = protocolv1.StatusEventKind_STATUS_EVENT_KIND_APPLIED
		case ResultRefreshed:
			ev = protocolv1.StatusEventKind_STATUS_EVENT_KIND_REFRESHED
		case ResultStackChanged:
			ev = protocolv1.StatusEventKind_STATUS_EVENT_KIND_STACK_CHANGED
		case ResultExpired:
			ev = protocolv1.StatusEventKind_STATUS_EVENT_KIND_EXPIRED
		case ResultDispelled:
			ev = protocolv1.StatusEventKind_STATUS_EVENT_KIND_DISPELLED
		case ResultConsumed:
			ev = protocolv1.StatusEventKind_STATUS_EVENT_KIND_CONSUMED
		default:
			continue
		}
		t, _ := TemplateByID(r.EffectID)
		kind := ""
		if t != nil {
			kind = statusKindLabel(t)
		}
		_ = s.Outbound.Enqueue(runtime.Outbound{
			To:        r.TargetID,
			MessageID: MsgS2CStatusEvent,
			Class:     replication.DeliveryAuthoritativeEvent,
			Msg: &protocolv1.S2CStatusEvent{
				TargetEntityId: r.TargetID,
				SourceEntityId: r.SourceID,
				EffectId:       r.EffectID,
				StatusKind:     kind,
				Event:          ev,
				Stacks:         uint32(r.Stacks),
				ExpiresAtTick:  r.ExpiresAt,
				ServerTick:     now,
			},
		})
	}
}

// statusKindLabel maps template tags to the wire status_kind string
// (presentation groups: control, slow, dot, buff, debuff).
func statusKindLabel(t *Template) string {
	switch {
	case t.Tags&controlTags != 0 || t.Tags&TagControl != 0:
		return "control"
	case t.Tags&TagSlow != 0 || t.Tags&TagChill != 0:
		return "slow"
	case t.Tags&TagDoT != 0:
		return "dot"
	case t.Tags&TagPositive != 0:
		return "buff"
	case t.Tags&TagNegative != 0:
		return "debuff"
	}
	return "status"
}
