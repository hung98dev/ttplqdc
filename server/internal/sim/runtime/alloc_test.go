//go:build !race

package runtime

import (
	"testing"
	"time"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

const (
	steadyPlayers  = PlayersCap
	steadyMonsters = SpawnGroupCap
)

// steadyState is the HOT-001 fixture: the full 64-actor steady channel —
// 22 players + 42 spawn-group monsters — with every player's replication
// client past the baseline handshake.
type steadyState struct {
	p        *Partition
	players  [steadyPlayers]uint64
	monsters [steadyMonsters]uint64
	seq      uint64
}

func newSteadyState(tb testing.TB) *steadyState {
	tb.Helper()
	clk := &testClock{}
	cfg := PartitionConfig{
		MapID:           "map_hue_citadel",
		ChannelID:       7,
		InstanceID:      id.UUID{0x01},
		ContentRevision: testRevision,
		Seed:            0x5eed,
		Now:             clk.Now,
		Sleep:           func(time.Duration) {},
	}
	p, err := NewPartition(cfg, Ports{})
	if err != nil {
		tb.Fatalf("NewPartition: %v", err)
	}
	s := &steadyState{p: p}
	for i := range s.players {
		pid, err := p.Admit(ClassPlayer)
		if err != nil {
			tb.Fatalf("admit player %d: %v", i, err)
		}
		s.players[i] = pid
		e, _ := p.Entity(pid)
		e.Snap.Kind = protocolv1.EntityKind_ENTITY_KIND_PLAYER
		e.Snap.Level = 30
		_ = p.SetPosition(pid, int32(i*1000), 0)
	}
	for i := range s.monsters {
		mid, err := p.Admit(ClassSpawnGroup)
		if err != nil {
			tb.Fatalf("admit monster %d: %v", i, err)
		}
		s.monsters[i] = mid
		e, _ := p.Entity(mid)
		e.Snap.Kind = protocolv1.EntityKind_ENTITY_KIND_MONSTER
		e.Hostile = true
		_ = p.SetPosition(mid, int32(i*1400), 4000)
	}
	return s
}

// step submits one held intent per player and advances one tick — the
// steady per-tick load of HOT-001. Nothing here may allocate.
func (s *steadyState) step() {
	for _, pid := range s.players {
		s.seq++
		_ = s.p.SubmitIntent(Intent{
			Kind:      IntentMovementHeld,
			EntityID:  pid,
			ClientSeq: s.seq,
			Held:      protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_RIGHT,
		})
	}
	s.p.stepUntil(s.p.simTime + Step)
}

// warmup advances the fixture past the baseline handshake so the measured
// window runs the true steady state.
func (s *steadyState) warmup(ticks int) {
	for i := 0; i < ticks; i++ {
		if i == 1 {
			for _, pid := range s.players {
				_ = s.p.SubmitIntent(Intent{
					Kind:       IntentBaselineAck,
					EntityID:   pid,
					BaselineID: s.p.clients[slotOf(pid)].baselineID,
				})
			}
		}
		s.step()
	}
}

// TestSteadyStateFixtureShape verifies the fixture itself: full 64-actor
// channel and every player client attached and baseline-acked.
func TestSteadyStateFixtureShape(t *testing.T) {
	s := newSteadyState(t)
	s.warmup(4)
	if s.p.classN[ClassPlayer] != PlayersCap ||
		s.p.classN[ClassSpawnGroup] != SpawnGroupCap {
		t.Fatalf("entity counts: players=%d monsters=%d",
			s.p.classN[ClassPlayer], s.p.classN[ClassSpawnGroup])
	}
	for _, pid := range s.players {
		c := &s.p.clients[slotOf(pid)]
		if !c.attached {
			t.Fatalf("player %d client not attached", pid)
		}
		if !c.baselineAcked {
			t.Fatalf("player %d client not baseline-acked", pid)
		}
	}
	if s.p.TickN() != 4 {
		t.Fatalf("warmup ticks = %d, want 4", s.p.TickN())
	}
}

// TestAllocs_SteadyTick is HOT-001: a steady tick — mailbox drain, ingest,
// AOI recompute per viewer and replication build — allocates nothing.
func TestAllocs_SteadyTick(t *testing.T) {
	s := newSteadyState(t)
	s.warmup(200)
	got := testing.AllocsPerRun(1000, s.step)
	if got != 0 {
		t.Fatalf("HOT-001: steady tick allocates %v allocs/op, want 0", got)
	}
}

// BenchmarkSteadyTick is the goAlloc gate's measurement point — it must
// report 0 allocs/op at -benchtime=200x.
func BenchmarkSteadyTick(b *testing.B) {
	s := newSteadyState(b)
	s.warmup(200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.step()
	}
}
