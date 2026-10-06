//go:build !race

package movement

import (
	"testing"
	"time"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
)

// The steady fixture mirrors HOT-001's channel load: 22 players + 42
// spawn-group monsters. Everyone runs into the blocking wall at x=60000
// so the measured window is fully converged — grounded runners whose
// positions have stopped changing (no landings, penetrations or grid
// churn), which is what the zero-alloc gate measures.
func newAllocFixture(tb testing.TB) *testRig {
	clk := &manualClock{}
	p, err := runtime.NewPartition(runtime.PartitionConfig{
		MapID:           "map_test_flat",
		ChannelID:       7,
		InstanceID:      id.UUID{0x02},
		ContentRevision: testRevision,
		Seed:            0x5eed,
		Now:             clk.Now,
		Sleep:           func(time.Duration) {},
	}, runtime.Ports{})
	if err != nil {
		tb.Fatalf("NewPartition: %v", err)
	}
	r := &testRig{p: p, clk: clk, seqs: make(map[uint64]uint64)}
	sys := New(Config{World: flatWorld()})
	r.s = sys
	p.RegisterSystem(runtime.PhaseMovement, sys.Step)

	for i := 0; i < runtime.PlayersCap; i++ {
		pid, err := p.Admit(runtime.ClassPlayer)
		if err != nil {
			tb.Fatalf("admit player %d: %v", i, err)
		}
		e, _ := p.Entity(pid)
		e.Snap.Kind = protocolv1.EntityKind_ENTITY_KIND_PLAYER
		// Left of the wall, running right.
		x := int32(50000 + i*100)
		_ = p.SetPosition(pid, x, 0)
		e.Checkpoint.X, e.Checkpoint.Y = x, 0
		e.Checkpoint.HeldHorizontalIntent =
			protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_RIGHT
	}
	for i := 0; i < runtime.SpawnGroupCap; i++ {
		mid, err := p.Admit(runtime.ClassSpawnGroup)
		if err != nil {
			tb.Fatalf("admit monster %d: %v", i, err)
		}
		e, _ := p.Entity(mid)
		e.Snap.Kind = protocolv1.EntityKind_ENTITY_KIND_MONSTER
		x := int32(2000 + i*300)
		_ = p.SetPosition(mid, x, 0)
		e.Checkpoint.X, e.Checkpoint.Y = x, 0
		e.Checkpoint.HeldHorizontalIntent =
			protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_RIGHT
	}
	return r
}

// stepOnce drives one movement phase in place — the measured op. No
// mailbox drain and no goroutine loop, matching the steady-state pattern.
func (r *testRig) stepOnce() {
	tc := &runtime.TickContext{Tick: r.p.TickN() + 1}
	r.s.Step(r.p, tc)
}

// TestAllocs_MovementTick is the HOT-00x counterpart for the movement
// phase: integrating the full 64-actor channel costs zero allocations.
func TestAllocs_MovementTick(t *testing.T) {
	r := newAllocFixture(t)
	// Warmup: players run into the wall and stop; grid + entity state
	// fully converged before measurement.
	for i := 0; i < 300; i++ {
		r.stepOnce()
	}
	got := testing.AllocsPerRun(1000, r.stepOnce)
	if got != 0 {
		t.Fatalf("movement tick allocates %v allocs/op, want 0", got)
	}
}

// BenchmarkMovementTick is the ns/op observation point for the phase.
func BenchmarkMovementTick(b *testing.B) {
	r := newAllocFixture(b)
	for i := 0; i < 300; i++ {
		r.stepOnce()
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.stepOnce()
	}
}
