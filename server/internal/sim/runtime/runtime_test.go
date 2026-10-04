package runtime

import (
	"bytes"
	"context"
	"testing"
	"time"

	"thinhthan/internal/core/id"
	observability "thinhthan/internal/observability/core"
	protocolv1 "thinhthan/internal/protocol/v1"

	"google.golang.org/protobuf/proto"
)

// testClock is the manually advanced partition clock.
type testClock struct{ now time.Duration }

func (c *testClock) Now() time.Duration { return c.now }

const testRevision = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// testConfig builds a partition config pinned to a fixed content revision
// and manual clock.
func testConfig(clk *testClock) PartitionConfig {
	return PartitionConfig{
		MapID:           "map_hue_citadel",
		ChannelID:       7,
		InstanceID:      id.UUID{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x10, 0x32, 0x54, 0x76, 0x98, 0xba, 0xdc, 0xfe},
		ContentRevision: testRevision,
		Seed:            0x5eed,
		Now:             clk.Now,
		Sleep:           func(time.Duration) {},
		Metrics:         observability.NewRegistry(nil),
	}
}

func newTestPartition(t *testing.T) *Partition {
	t.Helper()
	p, err := NewPartition(testConfig(&testClock{}), Ports{})
	if err != nil {
		t.Fatalf("NewPartition: %v", err)
	}
	return p
}

// TestFixedStepTickOrder pins the twelve-phase order of realtime_loop.md:
// every tick executes all phases in the mandated sequence.
func TestFixedStepTickOrder(t *testing.T) {
	p := newTestPartition(t)
	var got [phaseCount]PhaseID
	n := 0
	for ph := PhaseID(0); ph < phaseCount; ph++ {
		ph := ph
		p.RegisterSystem(ph, func(*Partition, *TickContext) {
			got[n] = ph
			n++
		})
	}
	p.stepUntil(Step)
	if n != int(phaseCount) {
		t.Fatalf("ran %d phases, want %d", n, phaseCount)
	}
	if got != PhaseOrder() {
		t.Fatalf("phase order = %v, want %v", got, PhaseOrder())
	}
	if p.TickN() != 1 || p.SimTime() != Step {
		t.Fatalf("after one step: tick=%d simTime=%v", p.TickN(), p.SimTime())
	}
}

// TestSingleOwnerMailbox exercises the single-owner handoff: cross-goroutine
// submission through the mailbox, apply in receive order inside the tick,
// held-state coalescing and per-character caps/rejects.
func TestSingleOwnerMailbox(t *testing.T) {
	p := newTestPartition(t)
	pid, err := p.Admit(ClassPlayer)
	if err != nil {
		t.Fatalf("Admit player: %v", err)
	}
	e, err := p.Entity(pid)
	if err != nil {
		t.Fatalf("Entity: %v", err)
	}

	// Submission from another goroutine lands in the mailbox; the owning
	// tick applies it. Three held values coalesce to the newest.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := uint64(1); i <= 3; i++ {
			held := protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_LEFT
			if i == 3 {
				held = protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_RIGHT
			}
			_ = p.SubmitIntent(Intent{Kind: IntentMovementHeld, EntityID: pid, ClientSeq: i, Held: held})
		}
		_ = p.SubmitIntent(Intent{Kind: IntentMovementEdge, EntityID: pid, ClientSeq: 4, Edge: 0x21})
	}()
	<-done
	p.stepUntil(Step)

	if e.Checkpoint.HeldHorizontalIntent != protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_RIGHT {
		t.Fatalf("held coalesce kept %v, want RIGHT", e.Checkpoint.HeldHorizontalIntent)
	}
	if e.pendingEdge != 0x21 || e.lastClientSeq != 4 {
		t.Fatalf("edge=%x seq=%d, want 0x21/4", e.pendingEdge, e.lastClientSeq)
	}

	// Stale resends and out-of-order intents never regress the applied seq.
	_ = p.SubmitIntent(Intent{Kind: IntentMovementEdge, EntityID: pid, ClientSeq: 2, Edge: 0x77})
	_ = p.SubmitIntent(Intent{Kind: IntentMovementEdge, EntityID: pid, ClientSeq: 5, Edge: 0x33})
	_ = p.SubmitIntent(Intent{Kind: IntentMovementEdge, EntityID: pid, ClientSeq: 4, Edge: 0x44})
	p.stepUntil(2 * Step)
	if e.pendingEdge != 0x33 {
		t.Fatalf("edge = %x, want 0x33 (stale resends rejected)", e.pendingEdge)
	}
	if e.lastClientSeq != 5 {
		t.Fatalf("lastClientSeq = %d, want 5", e.lastClientSeq)
	}
	if p.rejectedN != 2 ||
		p.rejected[0].Code != RejectStaleSeq || p.rejected[1].Code != RejectStaleSeq {
		t.Fatalf("rejects = %v", p.rejected[:p.rejectedN])
	}

	// The per-character discrete queue rejects past its cap of 8.
	for i := uint64(10); i < 10+DiscreteCap+1; i++ {
		_ = p.SubmitIntent(Intent{Kind: IntentAction, EntityID: pid, ClientSeq: i})
	}
	p.stepUntil(3 * Step)
	last := p.rejected[p.rejectedN-1]
	if last.Code != RejectDiscreteFull {
		t.Fatalf("discrete overflow reject = %v, want RejectDiscreteFull", last.Code)
	}

	// Non-player entities reject with RejectNotPlayer.
	mid, err := p.Admit(ClassSpawnGroup)
	if err != nil {
		t.Fatalf("Admit monster: %v", err)
	}
	_ = p.SubmitIntent(Intent{Kind: IntentAction, EntityID: mid, ClientSeq: 1})
	p.stepUntil(4 * Step)
	found := false
	for i := 0; i < p.rejectedN; i++ {
		if p.rejected[i].Code == RejectNotPlayer {
			found = true
		}
	}
	if !found {
		t.Fatal("monster intent not rejected as RejectNotPlayer")
	}
}

// TestTickOverrunAccounting pins the bounded catch-up rule: at most
// MaxCatchUpTicks run per scheduling slice; deeper lag is skipped forward
// and accounted as overrun.
func TestTickOverrunAccounting(t *testing.T) {
	p := newTestPartition(t)

	// Inside the cap both owed ticks run.
	p.stepUntil(2 * Step)
	if p.TickN() != 2 || p.SimTime() != 2*Step {
		t.Fatalf("tick=%d simTime=%v", p.TickN(), p.SimTime())
	}

	// 10 owed ticks beyond simTime: only MaxCatchUpTicks run, the
	// remainder resyncs simTime forward.
	p.stepUntil(12 * Step)
	if p.TickN() != 2+MaxCatchUpTicks {
		t.Fatalf("tick=%d, want %d (catch-up cap)", p.TickN(), 2+MaxCatchUpTicks)
	}
	if p.SimTime() != 12*Step {
		t.Fatalf("simTime=%v, want %v (overrun skip)", p.SimTime(), 12*Step)
	}
}

// recOutbound marshals each emitted message at enqueue time — required
// because message storage is borrowed from the client's pooled Builder.
type recOutbound struct{ msgs [][]byte }

func (r *recOutbound) Enqueue(o Outbound) error {
	raw, err := proto.Marshal(o.Msg)
	if err != nil {
		return err
	}
	r.msgs = append(r.msgs, raw)
	return nil
}

type recDurable struct{ cmds []DurableCommand }

func (r *recDurable) Emit(_ context.Context, c DurableCommand) error {
	r.cmds = append(r.cmds, c)
	return nil
}

// replayScript runs a fixed interaction sequence against a fresh partition
// and returns the emitted replication bytes plus durable commands.
func replayScript(t *testing.T) ([][]byte, []DurableCommand) {
	t.Helper()
	rec := &recOutbound{}
	dur := &recDurable{}
	p, err := NewPartition(testConfig(&testClock{}), Ports{Outbound: rec, Durable: dur})
	if err != nil {
		t.Fatalf("NewPartition: %v", err)
	}

	p1, err := p.Admit(ClassPlayer)
	if err != nil {
		t.Fatalf("admit p1: %v", err)
	}
	p2, err := p.Admit(ClassPlayer)
	if err != nil {
		t.Fatalf("admit p2: %v", err)
	}
	m1, err := p.Admit(ClassSpawnGroup)
	if err != nil {
		t.Fatalf("admit m1: %v", err)
	}
	e1, _ := p.Entity(p1)
	e1.Snap.Kind = protocolv1.EntityKind_ENTITY_KIND_PLAYER
	e1.Snap.DisplayName = "p1"
	e2, _ := p.Entity(p2)
	e2.Snap.Kind = protocolv1.EntityKind_ENTITY_KIND_PLAYER
	em, _ := p.Entity(m1)
	em.Snap.Kind = protocolv1.EntityKind_ENTITY_KIND_MONSTER
	em.Hostile = true
	_ = p.SetPosition(p1, 0, 0)
	_ = p.SetPosition(p2, 2000, 0)
	_ = p.SetPosition(m1, 1500, 0)

	seq := uint64(0)
	held := func(pid uint64, v protocolv1.HeldHorizontalIntent) {
		seq++
		_ = p.SubmitIntent(Intent{Kind: IntentMovementHeld, EntityID: pid, ClientSeq: seq, Held: v})
	}
	edge := func(pid uint64, v uint8) {
		seq++
		_ = p.SubmitIntent(Intent{Kind: IntentMovementEdge, EntityID: pid, ClientSeq: seq, Edge: v})
	}

	for tick := 0; tick < 12; tick++ {
		switch tick {
		case 1:
			// Both players acknowledge their baselines; deltas start.
			_ = p.SubmitIntent(Intent{Kind: IntentBaselineAck, EntityID: p1,
				BaselineID: p.clients[slotOf(p1)].baselineID})
			_ = p.SubmitIntent(Intent{Kind: IntentBaselineAck, EntityID: p2,
				BaselineID: p.clients[slotOf(p2)].baselineID})
		case 3:
			held(p1, protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_RIGHT)
			edge(p1, 0x01)
		case 5:
			// Client resync: result + fresh baseline + delta gate reset.
			_ = p.SubmitIntent(Intent{Kind: IntentBaselineResync, EntityID: p1,
				RequestID: 9, Reason: protocolv1.ResyncReason_RESYNC_REASON_INVALID_DELTA})
		case 6:
			// Monster leaves both players' AOI: despawn LEFT_AOI.
			_ = p.SetPosition(m1, 60000, 0)
		case 8:
			held(p2, protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_LEFT)
		case 10:
			_ = p.QueueCommand(DurableCommand{
				Kind:        CmdCheckpoint,
				Family:      "checkpoint",
				OwnerKind:   OwnerCharacter,
				Owner:       id.UUID{0xaa},
				OperationID: id.UUID{0xbb},
				SourceEvent: "checkpoint_tick10",
			})
		}
		p.stepUntil(p.simTime + Step)
	}
	return rec.msgs, dur.cmds
}

// TestSeededReplayDeterminism pins determinism: the same seed, content
// revision and intent sequence must produce byte-identical replication and
// durable output on a fresh partition.
func TestSeededReplayDeterminism(t *testing.T) {
	a, da := replayScript(t)
	b, db := replayScript(t)
	if len(a) == 0 {
		t.Fatal("replication emitted nothing — script broken")
	}
	if len(a) != len(b) {
		t.Fatalf("message counts differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if !bytes.Equal(a[i], b[i]) {
			t.Fatalf("message %d differs between seeded replays", i)
		}
	}
	if len(da) != len(db) {
		t.Fatalf("durable counts differ: %d vs %d", len(da), len(db))
	}
	for i := range da {
		if da[i] != db[i] {
			t.Fatalf("durable command %d differs between replays", i)
		}
	}
}
