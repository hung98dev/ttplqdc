package listener

import (
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	protocolv1 "thinhthan/internal/protocol/v1"
)

func enc(t *testing.T, m proto.Message) []byte {
	t.Helper()
	b, err := proto.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func delta(baseline, tick uint64) *protocolv1.S2CStateDelta {
	return &protocolv1.S2CStateDelta{BaselineId: baseline, ServerTick: tick}
}

func TestStateSupersede(t *testing.T) {
	now := time.Now()
	q := newOutboundQueue(256, 1<<20, func() time.Time { return now })

	// Two S2C_PARTY_STATE (607) for the same singleton key: the newer
	// supersedes; only one frame survives.
	old := enc(t, &protocolv1.S2CPartyState{})
	newer := enc(t, &protocolv1.S2CPartyState{})
	if err := q.offer(&outEntry{msgID: 607, payload: old, class: DeliveryReplaceableState, supKey: supKeyFor(607, old), estBytes: len(old) + envOverhead}); err != nil {
		t.Fatal(err)
	}
	if err := q.offer(&outEntry{msgID: 607, payload: newer, class: DeliveryReplaceableState, supKey: supKeyFor(607, newer), estBytes: len(newer) + envOverhead}); err != nil {
		t.Fatal(err)
	}
	if q.frames != 1 {
		t.Fatalf("supersede: want 1 queued frame, got %d", q.frames)
	}
	got := q.pop()
	if string(got.payload) != string(newer) {
		t.Fatalf("supersede kept the older payload")
	}
	if q.pop() != nil {
		t.Fatalf("supersede left a second frame")
	}

	// AUTHORITATIVE_EVENT frames never coalesce.
	a, b := enc(t, &protocolv1.S2CChatMessage{}), enc(t, &protocolv1.S2CChatMessage{})
	if err := q.offer(&outEntry{msgID: 601, payload: a, class: DeliveryAuthoritativeEvent, supKey: supKey{msgID: 601, key: 1}, estBytes: len(a) + envOverhead}); err != nil {
		t.Fatal(err)
	}
	if err := q.offer(&outEntry{msgID: 601, payload: b, class: DeliveryAuthoritativeEvent, supKey: supKey{msgID: 601, key: 2}, estBytes: len(b) + envOverhead}); err != nil {
		t.Fatal(err)
	}
	if q.frames != 2 {
		t.Fatalf("events must not supersede, got %d", q.frames)
	}
}

func TestDeltaMerge(t *testing.T) {
	now := time.Now()
	q := newOutboundQueue(256, 1<<20, func() time.Time { return now })

	d1 := &protocolv1.S2CStateDelta{
		BaselineId: 42, ServerTick: 100,
		SelfAck: &protocolv1.SelfAck{LastProcessedClientSeq: 5},
		Entities: []*protocolv1.EntityDelta{
			{EntityId: 7, Hp: proto.Int64(100)},
			{EntityId: 8, Hp: proto.Int64(50)},
		},
	}
	d2 := &protocolv1.S2CStateDelta{
		BaselineId: 42, ServerTick: 101,
		SelfAck: &protocolv1.SelfAck{LastProcessedClientSeq: 6},
		Entities: []*protocolv1.EntityDelta{
			{EntityId: 7, MaxHp: proto.Int64(30)},
			{EntityId: 9, Hp: proto.Int64(10)},
		},
	}
	p1, p2 := enc(t, d1), enc(t, d2)
	for _, p := range [][]byte{p1, p2} {
		if err := q.offer(&outEntry{msgID: 303, payload: p, class: DeliveryReplaceableState, supKey: supKeyFor(303, p), estBytes: len(p) + envOverhead}); err != nil {
			t.Fatal(err)
		}
	}
	if q.frames != 1 {
		t.Fatalf("same-baseline deltas must merge into one frame, got %d", q.frames)
	}
	got := q.pop()
	var merged protocolv1.S2CStateDelta
	if err := proto.Unmarshal(got.payload, &merged); err != nil {
		t.Fatal(err)
	}
	if merged.ServerTick != 101 || merged.BaselineId != 42 {
		t.Fatalf("merged tick/baseline wrong: %+v", &merged)
	}
	if merged.SelfAck == nil || merged.SelfAck.LastProcessedClientSeq != 6 {
		t.Fatalf("newest complete SelfAck must be retained: %+v", merged.SelfAck)
	}
	if len(merged.Entities) != 3 {
		t.Fatalf("want 3 entities after merge, got %d", len(merged.Entities))
	}
	var e7 *protocolv1.EntityDelta
	for _, e := range merged.Entities {
		if e.EntityId == 7 {
			e7 = e
		}
	}
	if e7 == nil || e7.Hp == nil || *e7.Hp != 100 || e7.MaxHp == nil || *e7.MaxHp != 30 {
		t.Fatalf("entity 7 must merge field-wise (absence unchanged): %+v", e7)
	}

	// Deltas for a different baseline do NOT merge with this one.
	d3 := enc(t, delta(43, 102))
	if err := q.offer(&outEntry{msgID: 303, payload: d3, class: DeliveryReplaceableState, supKey: supKeyFor(303, d3), estBytes: len(d3) + envOverhead}); err != nil {
		t.Fatal(err)
	}
	if q.frames != 1 {
		t.Fatalf("new baseline delta queues independently, got %d", q.frames)
	}
}

// TestSlowConsumerClose4008 covers protocol.md § Connection Backpressure:
// non-replaceable overflow closes, and >75 % fill for 5 s closes — both
// with WS 4008 SLOW_CONSUMER.
func TestSlowConsumerClose4008(t *testing.T) {
	if WSCloseSlowConsumer != 4008 {
		t.Fatalf("spec-pinned close code: got %d", WSCloseSlowConsumer)
	}
	now := time.Now()
	q := newOutboundQueue(8, 1<<20, func() time.Time { return now })
	payload := []byte("x")
	entry := func() *outEntry {
		return &outEntry{msgID: 601, payload: payload, class: DeliveryAuthoritativeEvent, supKey: supKey{msgID: 601, key: 0}, estBytes: len(payload) + envOverhead}
	}
	for i := 0; i < 8; i++ {
		e := entry()
		e.supKey.key = uint64(i)
		if err := q.offer(e); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.offer(entry()); err != ErrSlowConsumer {
		t.Fatalf("full queue + non-replaceable frame: want ErrSlowConsumer, got %v", err)
	}
	// Above 75 % continuously for 5 s.
	q2 := newOutboundQueue(8, 1<<20, func() time.Time { return now })
	for i := 0; i < 7; i++ { // 7/8 = 87.5 % > 75 %
		e := entry()
		e.supKey.key = uint64(i + 100)
		if err := q2.offer(e); err != nil {
			t.Fatal(err)
		}
	}
	if q2.checkSlowConsumer(now) {
		t.Fatalf("over-cap must persist 5 s before closing")
	}
	if !q2.checkSlowConsumer(now.Add(6 * time.Second)) {
		t.Fatalf("over-cap for 5 s must close with 4008")
	}
	// Draining resets the window.
	q2.pop()
	q2.pop()
	if q2.checkSlowConsumer(now.Add(10 * time.Second)) {
		t.Fatalf("below-cap must not trigger slow-consumer close")
	}
}
