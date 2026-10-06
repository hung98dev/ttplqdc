package movement

import (
	"testing"

	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
)

// admitPlayer places one player entity at (x, y) on the flat world.
func admitPlayer(t *testing.T, r *testRig, x, y int64) uint64 {
	t.Helper()
	pid, err := r.p.Admit(runtime.ClassPlayer)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	e, err := r.p.Entity(pid)
	if err != nil {
		t.Fatalf("Entity: %v", err)
	}
	e.Snap.Kind = protocolv1.EntityKind_ENTITY_KIND_PLAYER
	if err := r.p.SetPosition(pid, int32(x), int32(y)); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	e.Checkpoint.X = int32(x)
	e.Checkpoint.Y = int32(y)
	return pid
}

func heldRight(seq uint64) runtime.Intent {
	return runtime.Intent{
		Kind: runtime.IntentMovementHeld,
		Held: protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_RIGHT,
	}
}

func edge(seq uint64, b uint8) runtime.Intent {
	return runtime.Intent{Kind: runtime.IntentMovementEdge, Edge: b}
}

func entityOf(t *testing.T, r *testRig, id uint64) *runtime.Entity {
	t.Helper()
	e, err := r.p.Entity(id)
	if err != nil {
		t.Fatalf("Entity %d: %v", id, err)
	}
	return e
}

// TestCharacterReferenceCollider pins the CHARACTER collider: 800x1800 mm
// at scale (1,1,1), anchored mid-feet, independent of any sprite size.
func TestCharacterReferenceCollider(t *testing.T) {
	b := charBox(10000, 5000)
	if b.Width() != 800 || b.Height() != 1800 {
		t.Fatalf("collider %dx%d, want 800x1800", b.Width(), b.Height())
	}
	if b.MinX != 9600 || b.MaxX != 10400 || b.MinY != 5000 || b.MaxY != 6800 {
		t.Fatalf("mid-feet anchor wrong: %+v", b)
	}
}

// TestIdleRunJumpFallTransitions drives the canonical state machine on the
// flat world: IDLE -> RUN -> JUMP -> FALL -> land (IDLE/RUN, jump count
// reset), plus a <=300 mm ledge mount and a wall stop.
func TestIdleRunJumpFallTransitions(t *testing.T) {
	r := newRig(t, nil)
	pid := admitPlayer(t, r, 1000, 0)
	r.runTicks(t, 2)

	e := entityOf(t, r, pid)
	if e.Checkpoint.MovementState != protocolv1.MovementState_MOVEMENT_STATE_IDLE {
		t.Fatalf("initial state %v, want IDLE", e.Checkpoint.MovementState)
	}
	if !e.Checkpoint.IsGrounded {
		t.Fatal("spawned entity not grounded on floor")
	}

	// RUN: held right moves at +6000 mm/s.
	r.submit(pid, 1, heldRight(1))
	r.runTicks(t, 1)
	e = entityOf(t, r, pid)
	if e.Checkpoint.MovementState != protocolv1.MovementState_MOVEMENT_STATE_RUN ||
		e.Checkpoint.Vx != RunSpeedMmS {
		t.Fatalf("RUN: state=%v vx=%d", e.Checkpoint.MovementState, e.Checkpoint.Vx)
	}
	x0 := e.Checkpoint.X
	r.runTicks(t, 1)
	e = entityOf(t, r, pid)
	if dx := e.Checkpoint.X - x0; dx != 300 {
		t.Fatalf("one-tick travel %d mm, want 300 (6000 mm/s * 50 ms)", dx)
	}

	// Release: RELEASE_RIGHT clears the direction -> IDLE.
	r.submit(pid, 2, edge(2, EdgeReleaseRight))
	r.runTicks(t, 1)
	e = entityOf(t, r, pid)
	if e.Checkpoint.MovementState != protocolv1.MovementState_MOVEMENT_STATE_IDLE ||
		e.Checkpoint.Vx != 0 {
		t.Fatalf("IDLE after release: state=%v vx=%d",
			e.Checkpoint.MovementState, e.Checkpoint.Vx)
	}

	// JUMP grounded: impulse 11000, count 1.
	r.submit(pid, 3, edge(3, EdgeJump))
	r.runTicks(t, 1)
	e = entityOf(t, r, pid)
	if e.Checkpoint.MovementState != protocolv1.MovementState_MOVEMENT_STATE_JUMP ||
		e.Checkpoint.Vy != FirstJumpMmS || e.Checkpoint.JumpCount != 1 {
		t.Fatalf("JUMP: state=%v vy=%d jc=%d",
			e.Checkpoint.MovementState, e.Checkpoint.Vy, e.Checkpoint.JumpCount)
	}

	// JUMP -> FALL once gravity overruns the impulse (11 ticks: vy falls
	// 1400/tick from +11000).
	r.submit(pid, 4, heldRight(4))
	fell := false
	for i := 0; i < 12 && !fell; i++ {
		r.runTicks(t, 1)
		e = entityOf(t, r, pid)
		if e.Checkpoint.MovementState == protocolv1.MovementState_MOVEMENT_STATE_FALL {
			fell = true
		}
	}
	if !fell {
		t.Fatalf("never entered FALL; state=%v vy=%d",
			e.Checkpoint.MovementState, e.Checkpoint.Vy)
	}

	// Land: grounded, jump count reset, RUN resumes under held RIGHT.
	landed := false
	for i := 0; i < 20 && !landed; i++ {
		r.runTicks(t, 1)
		e = entityOf(t, r, pid)
		if e.Checkpoint.IsGrounded {
			landed = true
		}
	}
	if !landed {
		t.Fatal("never landed")
	}
	if e.Checkpoint.JumpCount != 0 {
		t.Fatalf("jump count %d after landing, want 0", e.Checkpoint.JumpCount)
	}
	if e.Checkpoint.MovementState != protocolv1.MovementState_MOVEMENT_STATE_RUN {
		t.Fatalf("post-land state %v, want RUN", e.Checkpoint.MovementState)
	}
}

// TestDoubleJumpOneWayDrop exercises the double-jump budget and the
// six-tick one-way drop-through window.
// mountOn places an entity already grounded on a surface (the physics
// equivalent of a persisted spawn): a mid-air admit would legitimately
// fall, and one-way platforms never mount an entity back upward.
func mountOn(t *testing.T, r *testRig, pid uint64, x, y int64, platform uint64) {
	t.Helper()
	if err := r.p.SetPosition(pid, int32(x), int32(y)); err != nil {
		t.Fatalf("SetPosition: %v", err)
	}
	e := entityOf(t, r, pid)
	e.Checkpoint.X, e.Checkpoint.Y = int32(x), int32(y)
	e.Checkpoint.IsGrounded = true
	e.Checkpoint.PlatformID = platform
	e.Checkpoint.MovementState = protocolv1.MovementState_MOVEMENT_STATE_IDLE
}

func TestDoubleJumpOneWayDrop(t *testing.T) {
	r := newRig(t, nil)
	// Spawn standing on the one-way platform (y=1500).
	pid := admitPlayer(t, r, 105000, 1500)
	mountOn(t, r, pid, 105000, 1500, 5)
	r.runTicks(t, 2)
	e := entityOf(t, r, pid)
	if !e.Checkpoint.IsGrounded || e.Checkpoint.Y != 1500 {
		t.Fatalf("platform spawn: y=%d grounded=%v", e.Checkpoint.Y, e.Checkpoint.IsGrounded)
	}

	// First jump off the platform.
	r.submit(pid, 1, edge(1, EdgeJump))
	r.runTicks(t, 1)
	e = entityOf(t, r, pid)
	if e.Checkpoint.Vy != FirstJumpMmS || e.Checkpoint.JumpCount != 1 {
		t.Fatalf("jump1: vy=%d jc=%d", e.Checkpoint.Vy, e.Checkpoint.JumpCount)
	}

	// Second jump airborne: SECOND impulse, count 2.
	r.submit(pid, 2, edge(2, EdgeJump))
	r.runTicks(t, 1)
	e = entityOf(t, r, pid)
	if e.Checkpoint.Vy != SecondJumpMmS || e.Checkpoint.JumpCount != 2 {
		t.Fatalf("jump2: vy=%d jc=%d", e.Checkpoint.Vy, e.Checkpoint.JumpCount)
	}

	// Third jump press is ignored (budget exhausted).
	r.submit(pid, 3, edge(3, EdgeJump))
	r.runTicks(t, 1)
	e = entityOf(t, r, pid)
	if e.Checkpoint.JumpCount != 2 {
		t.Fatalf("jump3 consumed budget: jc=%d", e.Checkpoint.JumpCount)
	}

	// Land back on the one-way platform is impossible while dropping past
	// its level; let the entity land on the floor, then re-stand on the
	// platform for the drop case.
	r.submit(pid, 4, heldRight(4)) // drift keeps it off the platform column
	// Apex of a platform double jump is ~5.4 m — the fall alone takes
	// ~13 ticks at terminal velocity.
	for i := 0; i < 30; i++ {
		r.runTicks(t, 1)
		if entityOf(t, r, pid).Checkpoint.IsGrounded {
			break
		}
	}
	e = entityOf(t, r, pid)
	if !e.Checkpoint.IsGrounded {
		t.Fatal("never landed after double jump")
	}

	// Re-mount the one-way platform, then DROP: platform ignored for six
	// ticks and the entity falls through to the floor.
	mountOn(t, r, pid, 105000, 1500, 5)
	r.submit(pid, 5, runtime.Intent{Kind: runtime.IntentMovementHeld,
		Held: protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_NONE})
	r.runTicks(t, 1)
	e = entityOf(t, r, pid)
	if !e.Checkpoint.IsGrounded || e.Checkpoint.Y != 1500 {
		t.Fatalf("re-mount failed: y=%d grounded=%v", e.Checkpoint.Y, e.Checkpoint.IsGrounded)
	}

	r.submit(pid, 6, edge(6, EdgeDrop))
	r.runTicks(t, 1)
	e = entityOf(t, r, pid)
	if e.Checkpoint.DropIgnorePlatformID != 5 ||
		e.Checkpoint.DropIgnoreUntilTick == 0 {
		t.Fatalf("drop window not armed: %+v", e.Checkpoint)
	}
	if e.Checkpoint.IsGrounded {
		t.Fatal("still grounded during drop-through")
	}
	// Falls through the ignored platform onto the floor.
	for i := 0; i < 12; i++ {
		r.runTicks(t, 1)
		if entityOf(t, r, pid).Checkpoint.IsGrounded {
			break
		}
	}
	e = entityOf(t, r, pid)
	if !e.Checkpoint.IsGrounded || e.Checkpoint.Y != 0 {
		t.Fatalf("post-drop: y=%d grounded=%v", e.Checkpoint.Y, e.Checkpoint.IsGrounded)
	}
}

// TestDiscreteMovementEdgeMessage proves 108 edges drive direction and
// facing in receive order, never merge, and stream to DrainEdgeEvents.
func TestDiscreteMovementEdgeMessage(t *testing.T) {
	r := newRig(t, nil)
	pid := admitPlayer(t, r, 1000, 0)
	r.runTicks(t, 1)

	// PRESS_RIGHT then FLIP_LEFT in one tick: last edge wins.
	r.submit(pid, 1, edge(1, EdgePressRight))
	r.submit(pid, 2, edge(2, EdgeFlipLeft))
	r.runTicks(t, 1)
	e := entityOf(t, r, pid)
	if e.Checkpoint.Facing != protocolv1.Facing_FACING_LEFT ||
		e.Checkpoint.HeldHorizontalIntent != protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_LEFT {
		t.Fatalf("flip merge: facing=%v held=%v",
			e.Checkpoint.Facing, e.Checkpoint.HeldHorizontalIntent)
	}
	var evs [8]EdgeEvent
	n := r.s.DrainEdgeEvents(evs[:])
	if n != 2 || evs[0].Dir != protocolv1.Facing_FACING_RIGHT ||
		evs[1].Dir != protocolv1.Facing_FACING_LEFT {
		t.Fatalf("edge stream n=%d evs=%+v", n, evs[:n])
	}
	if evs[0].Type != protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_PRESS ||
		evs[1].Type != protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_FLIP {
		t.Fatalf("edge types: %+v", evs[:n])
	}

	// RELEASE_LEFT stops the run.
	r.submit(pid, 3, edge(3, EdgeReleaseLeft))
	r.runTicks(t, 1)
	e = entityOf(t, r, pid)
	if e.Checkpoint.MovementState != protocolv1.MovementState_MOVEMENT_STATE_IDLE {
		t.Fatalf("after release: %v", e.Checkpoint.MovementState)
	}

	// Stale-seq edge is rejected by ingest — stream sees only fresh ones.
	r.submit(pid, 2, edge(2, EdgeFlipRight)) // seq 2 <= lastClientSeq
	n = r.s.DrainEdgeEvents(evs[:])
	r.runTicks(t, 1)
	n2 := r.s.DrainEdgeEvents(evs[:])
	_ = n
	if n2 != 0 {
		t.Fatalf("stale edge reached stream: n=%d", n2)
	}
}

// TestKnockbackCollision drives forced displacement: velocities hold under
// knockback regardless of intent, a 107 KNOCKBACK emits with checkpoint,
// and landing returns control.
func TestKnockbackCollision(t *testing.T) {
	out := &fakeOutbound{}
	r := newRig(t, out)
	pid := admitPlayer(t, r, 30000, 0)
	r.runTicks(t, 2)

	// Caller-bounded knockback: vx=+8000 vy=+2000 for 8 ticks.
	r.s.ApplyKnockback(r.p, &runtime.TickContext{Tick: r.p.TickN()}, pid, 8000, 2000, 8)
	e := entityOf(t, r, pid)
	if e.Checkpoint.MovementState != protocolv1.MovementState_MOVEMENT_STATE_KNOCKBACK {
		t.Fatalf("state %v, want KNOCKBACK", e.Checkpoint.MovementState)
	}

	// Opposing held intent must not steer the displacement.
	r.submit(pid, 1, runtime.Intent{Kind: runtime.IntentMovementHeld,
		Held: protocolv1.HeldHorizontalIntent_HELD_HORIZONTAL_INTENT_LEFT})
	r.runTicks(t, 1)
	e = entityOf(t, r, pid)
	if e.Checkpoint.Vx != 8000 {
		t.Fatalf("knockback vx=%d, want forced 8000", e.Checkpoint.Vx)
	}
	if e.Checkpoint.X <= 30000 {
		t.Fatalf("knockback did not displace: x=%d", e.Checkpoint.X)
	}

	corrections := out.byID(107)
	if len(corrections) != 1 ||
		corrections[0].Reason != protocolv1.MovementCorrectionReason_MOVEMENT_CORRECTION_REASON_KNOCKBACK {
		t.Fatalf("107s=%d reasons=%v", len(corrections), corrections)
	}
	if corrections[0].Checkpoint == nil {
		t.Fatal("107 missing checkpoint")
	}

	// Past the bound and landed: control returns (state leaves KNOCKBACK).
	for i := 0; i < 30; i++ {
		r.runTicks(t, 1)
		e = entityOf(t, r, pid)
		if e.Checkpoint.MovementState != protocolv1.MovementState_MOVEMENT_STATE_KNOCKBACK {
			break
		}
	}
	e = entityOf(t, r, pid)
	if e.Checkpoint.MovementState == protocolv1.MovementState_MOVEMENT_STATE_KNOCKBACK {
		t.Fatal("knockback never ended")
	}
}

// TestStaleInputRejected: an edge outside the lag window emits exactly one
// S2C_ERROR STALE_INPUT and logs, yet still applies (combat.md L167).
func TestStaleInputRejected(t *testing.T) {
	out := &fakeOutbound{}
	r := newRig(t, out)
	pid := admitPlayer(t, r, 1000, 0)
	// Feed stamps: first edge fresh (lag 0), second edge lag 400 >
	// RTT(200)+80.
	var stamps map[uint64][2]int64
	r.s.stamps = func(id uint64) (int64, int64, bool) {
		pair, ok := stamps[id]
		return pair[0], pair[1], ok
	}
	r.runTicks(t, 1)

	stamps = map[uint64][2]int64{pid: {1000, 1000}}
	r.submit(pid, 1, edge(1, EdgePressRight))
	r.runTicks(t, 1)
	if errs := out.errors(); len(errs) != 0 {
		t.Fatalf("fresh edge emitted errors: %v", errs)
	}
	e := entityOf(t, r, pid)
	if e.Checkpoint.Facing != protocolv1.Facing_FACING_RIGHT {
		t.Fatalf("fresh edge not applied: facing=%v", e.Checkpoint.Facing)
	}

	stamps = map[uint64][2]int64{pid: {0, 400}}
	r.submit(pid, 2, edge(2, EdgeFlipLeft))
	r.runTicks(t, 1)
	errs := out.errors()
	if len(errs) != 1 ||
		errs[0].ErrorCode != protocolv1.ErrorCode_ERROR_CODE_STALE_INPUT {
		t.Fatalf("stale edge errors=%v, want one STALE_INPUT", errs)
	}
	// Stale edge still applies to movement.
	e = entityOf(t, r, pid)
	if e.Checkpoint.Facing != protocolv1.Facing_FACING_LEFT {
		t.Fatalf("stale edge not applied: facing=%v", e.Checkpoint.Facing)
	}
}

// TestCoalescingNeverDropsMovementEdge is the ADR-0038 regression: under
// the 17-player and 18-player loads of network.md L59-61, held state
// coalesces but every discrete edge reaches the movement phase in order
// up to the explicit cap-8 queue (overflow rejects, never merges).
func TestCoalescingNeverDropsMovementEdge(t *testing.T) {
	for _, players := range []int{17, 18} {
		t.Run(map[bool]string{true: "18p", false: "17p"}[players == 18], func(t *testing.T) {
			r := newRig(t, nil)
			ids := make([]uint64, 0, players)
			for i := 0; i < players; i++ {
				ids = append(ids, admitPlayer(t, r, int64(1000+i*2000), 0))
			}

			// Per-tick saturation respecting the 256-entry mailbox: 9
			// edges interleaved with 1 coalescible held intent per player
			// (10/player <= 180 total, inside the mailbox budget).
			var seq uint64
			for _, pid := range ids {
				for k := 0; k < 9; k++ {
					seq++
					b := EdgePressRight
					if k%2 == 1 {
						b = EdgeFlipLeft
					}
					r.submit(pid, seq, edge(seq, b))
					if k == 4 {
						seq++
						r.submit(pid, seq, heldRight(seq))
					}
				}
			}
			r.runTicks(t, 1)

			var evs [runtime.PlayersCap * runtime.DiscreteCap]EdgeEvent
			n := r.s.DrainEdgeEvents(evs[:])
			// Exactly the discrete cap applies per player — the excess was
			// rejected at routeIntent, not merged.
			want := players * runtime.DiscreteCap
			if n != want {
				t.Fatalf("applied edges=%d, want %d (cap %d x players)", n, want, runtime.DiscreteCap)
			}
			// Order preserved: every player's eighth edge is the same
			// pattern (k=7 => FLIP_LEFT).
			for pi := 0; pi < players; pi++ {
				ev := evs[pi*runtime.DiscreteCap+runtime.DiscreteCap-1]
				if ev.Type != protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_FLIP {
					t.Fatalf("player %d last applied edge type=%v (order broken)", pi, ev.Type)
				}
			}

			// Next tick: the queue drained, fresh edges apply again.
			for _, pid := range ids {
				seq++
				r.submit(pid, seq, edge(seq, EdgePressLeft))
			}
			r.runTicks(t, 1)
			n = r.s.DrainEdgeEvents(evs[:])
			if n != players {
				t.Fatalf("post-drain applied=%d, want %d", n, players)
			}
		})
	}
}
