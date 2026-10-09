package guild

import (
	"context"
	"testing"
	"time"

	"thinhthan/internal/core/id"
	durableguild "thinhthan/internal/durable/guild"
)

// mapView resolves each character to (guild, membership).
type mapView struct{ m map[id.UUID][2]id.UUID }

func (v mapView) GuildOf(_ context.Context, c id.UUID) (id.UUID, id.UUID, error) {
	p, ok := v.m[c]
	if !ok {
		return id.UUID{}, id.UUID{}, nil
	}
	return p[0], p[1], nil
}

// capSink captures applied events.
type capSink struct{ evs []durableguild.SourceEvent }

func (s *capSink) Apply(_ context.Context, ev durableguild.SourceEvent) error {
	s.evs = append(s.evs, ev)
	return nil
}

func mkGatheringFixture(n int) (mapView, *Gathering, id.UUID, []id.UUID) {
	gid := id.NewV4()
	v := mapView{m: map[id.UUID][2]id.UUID{}}
	chars := make([]id.UUID, 0, n)
	for i := 0; i < n; i++ {
		c := id.NewV4()
		memID := id.NewV4()
		v.m[c] = [2]id.UUID{gid, memID}
		chars = append(chars, c)
	}
	return v, NewGathering(v, nil), gid, chars
}

// TestGatheringAlignedSlotWholeSlotRequired: the aligned 300s UTC slot
// pays only when >=5 members stay anchored for the WHOLE slot — a late
// joiner (membership after slot start) disqualifies the grant.
func TestGatheringAlignedSlotWholeSlotRequired(t *testing.T) {
	ctx := context.Background()
	_, g, _, chars := mkGatheringFixture(5)
	slot := SlotStart(time.Now().UTC().Truncate(time.Minute))
	for _, c := range chars {
		if err := g.RestStart(ctx, c, "bonfire:a", "mi:1", slot.Add(-time.Minute)); err != nil {
			t.Fatalf("anchor: %v", err)
		}
	}
	sink := &capSink{}
	// All five members present for the whole slot -> grant.
	since := map[id.UUID]time.Time{}
	for _, c := range chars {
		since[c] = slot.Add(-time.Hour)
	}
	ev, err := g.CloseSlot(ctx, slot, since, sink)
	if err != nil || ev == nil {
		t.Fatalf("close: %v ev=%v", err, ev)
	}
	if ev.Kind != durableguild.SourceGuildActivity || ev.Element != durableguild.ElementTho {
		t.Fatalf("event: %+v", ev)
	}
	if ev.SlotStart == nil || !ev.SlotStart.Equal(slot) {
		t.Fatalf("slot start: %v", ev.SlotStart)
	}
	if len(sink.evs) != 1 {
		t.Fatalf("emissions = %d", len(sink.evs))
	}
	// A member whose membership starts mid-slot disqualifies the whole
	// grant (whole-slot rule).
	_, g2, _, chars2 := mkGatheringFixture(5)
	for _, c := range chars2 {
		_ = g2.RestStart(ctx, c, "bonfire:a", "mi:1", slot.Add(-time.Minute))
	}
	since2 := map[id.UUID]time.Time{}
	for i, c := range chars2 {
		since2[c] = slot.Add(-time.Hour)
		if i == 4 {
			since2[c] = slot.Add(time.Minute) // joined mid-slot
		}
	}
	ev2, err := g2.CloseSlot(ctx, slot, since2, &capSink{})
	if err != nil {
		t.Fatalf("close2: %v", err)
	}
	if ev2 != nil {
		t.Fatalf("mid-slot joiner must disqualify, got %+v", ev2)
	}
}

// TestGatheringFirstSlotPerDayOnly: only the first qualifying slot in
// each UTC day pays a grant per guild.
func TestGatheringFirstSlotPerDayOnly(t *testing.T) {
	ctx := context.Background()
	_, g, _, chars := mkGatheringFixture(5)
	slot := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	for _, c := range chars {
		_ = g.RestStart(ctx, c, "bonfire:a", "mi:1", slot.Add(-time.Minute))
	}
	since := map[id.UUID]time.Time{}
	for _, c := range chars {
		since[c] = slot.Add(-time.Hour)
	}
	sink := &capSink{}
	if ev, err := g.CloseSlot(ctx, slot, since, sink); err != nil || ev == nil {
		t.Fatalf("first slot must pay: %v", err)
	}
	// Same day, later slot -> no second grant.
	ev2, err := g.CloseSlot(ctx, slot.Add(2*time.Hour), since, sink)
	if err != nil || ev2 != nil {
		t.Fatalf("second slot same day must not pay: %v", err)
	}
	// Next UTC day pays again.
	ev3, err := g.CloseSlot(ctx, slot.Add(26*time.Hour), since, sink)
	if err != nil || ev3 == nil {
		t.Fatalf("next-day slot must pay: %v", err)
	}
	if len(sink.evs) != 2 {
		t.Fatalf("emissions = %d, want 2", len(sink.evs))
	}
}

// TestSurgeGuildGrantKeyedByChainId: Spirit Surge grants dedupe on
// (guild_id, chain_id) — replaying the same chain never double-grants
// (ADR-0062).
func TestSurgeGuildGrantKeyedByChainId(t *testing.T) {
	ctx := context.Background()
	v := mapView{m: map[id.UUID][2]id.UUID{}}
	gid := id.NewV4()
	credits := make([]durableguild.Credit, 0, 3)
	for i := 0; i < 3; i++ {
		c, mem := id.NewV4(), id.NewV4()
		v.m[c] = [2]id.UUID{gid, mem}
		credits = append(credits, durableguild.Credit{CharacterID: c, MembershipID: mem, Amount: 5})
	}
	g := NewGathering(v, nil)
	sink := &capSink{}
	at := time.Now().UTC()
	for i := 0; i < 3; i++ {
		if err := g.SurgeGrant(ctx, gid, "chain:9", durableguild.ElementKim,
			credits, at, sink); err != nil {
			t.Fatalf("surge %d: %v", i, err)
		}
	}
	if len(sink.evs) != 1 {
		t.Fatalf("chain replay granted %d times, want 1", len(sink.evs))
	}
	if sink.evs[0].ChainID != "chain:9" || sink.evs[0].Kind != durableguild.SourceWorldEvent {
		t.Fatalf("event: %+v", sink.evs[0])
	}
	// A different chain on the same guild grants again.
	if err := g.SurgeGrant(ctx, gid, "chain:10", durableguild.ElementKim,
		credits, at, sink); err != nil {
		t.Fatalf("surge2: %v", err)
	}
	if len(sink.evs) != 2 {
		t.Fatalf("new chain must grant: %d", len(sink.evs))
	}
}
