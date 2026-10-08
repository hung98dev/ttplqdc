package combat

import protocolv1 "thinhthan/internal/protocol/v1"

// Just Guard rules of combat.md § Just Guard (ADR-0034):
//
//   - window: [hit_commit_ms-150, hit_commit_ms] on the authoritative
//     hit commit timestamp.
//   - trigger: a horizontal C2S_MOVEMENT_EDGE (PRESS/RELEASE/FLIP with a
//     LEFT/RIGHT direction) whose effective_edge_ms lands inside the
//     window AND no other edge in the preceding 400 ms; stale edges
//     (lag > RTT+80) never qualify. Held state never qualifies.
//   - streak: consecutive successes within 3.0 s escalate mitigation
//     40% -> 50% -> 60%; each success sets ICD 900 ms; any FAIL window
//     resets streak to 0 and ICD to 500 ms.
//   - window opportunity: on a connected hit, when the defender is not
//     hard-controlled (STUN/FREEZE/ROOT — AIRBORNE does not block) and
//     the ICD is ready, JUST_GUARD_WINDOW is emitted only when
//     streak >= 1 OR post_mitigation_damage >= 12% of defender MAX_HP.
//     A hit that opens no window is neither SUCCESS nor FAIL and
//     changes nothing.
const (
	jgWindowMs      = 150
	jgSpamMs        = 400
	jgStreakSpanMs  = 3000
	jgIcdBaseMs     = 500
	jgIcdSuccessMs  = 900
	jgHeavyRatioBP  = 1200 // 12% of MAX_HP, basis points
	jgMitigationBP0 = 4000
	jgMitigationBP1 = 5000
	jgMitigationBP2 = 6000
)

// justGuardState is the per-defender JG bookkeeping (runtime-only).
type justGuardState struct {
	streak        int
	lastSuccessMs int64
	icdUntilMs    int64
}

// streakTier returns the streak count still inside its 3.0 s window.
func (g *justGuardState) streakTier(commitMs int64) int {
	if g.streak <= 0 || commitMs-g.lastSuccessMs > jgStreakSpanMs {
		return 0
	}
	return g.streak
}

// mitigationBP returns the streak-tier mitigation in basis points.
func (g *justGuardState) mitigationBP(commitMs int64) int64 {
	switch {
	case g.streakTier(commitMs) >= 2:
		return jgMitigationBP2
	case g.streakTier(commitMs) == 1:
		return jgMitigationBP1
	default:
		return jgMitigationBP0
	}
}

// edgeStamp is one C2S_MOVEMENT_EDGE occurrence as fed to combat.
type edgeStamp struct {
	effectiveMs int64
	stale       bool
	horizontal  bool
}

// edgeHistory keeps recent edges for the 400 ms anti-spam check. 32
// entries covers any legal burst.
const edgeHistoryCap = 32

type edgeHistory struct {
	edges [edgeHistoryCap]edgeStamp
	n     int
}

func (h *edgeHistory) note(st edgeStamp) {
	if h.n < edgeHistoryCap {
		h.edges[h.n] = st
		h.n++
		return
	}
	copy(h.edges[:edgeHistoryCap-1], h.edges[1:])
	h.edges[edgeHistoryCap-1] = st
}

// jgVerdict is the outcome of evaluating one connected hit.
type jgVerdict uint8

const (
	jgNoWindow jgVerdict = iota // no window opened: neither success nor fail
	jgFail                      // window opened, no valid edge in time
	jgSuccess                   // window opened, valid edge in time
)

// evalJustGuard evaluates one connected hit on the defender. commitMs
// is the authoritative hit-commit timestamp; postMit the post-mitigation
// damage of the hit. Returns (windowOpened, verdict, mitigationBP).
func evalJustGuard(st *actorState, hardControlled bool, maxHP int64, commitMs, postMit int64) (bool, jgVerdict, int64) {
	if hardControlled {
		return false, jgNoWindow, 0
	}
	if commitMs < st.jg.icdUntilMs {
		return false, jgNoWindow, 0
	}
	heavy := postMit*10000 >= maxHP*jgHeavyRatioBP
	streakAlive := st.jg.streakTier(commitMs) >= 1
	if !streakAlive && !heavy {
		return false, jgNoWindow, 0
	}
	// Trigger scan: a horizontal non-stale edge inside
	// [commit-150, commit] with no other edge in the preceding 400 ms.
	lo := commitMs - jgWindowMs
	for i := 0; i < st.stamps.n; i++ {
		ed := st.stamps.edges[i]
		if ed.stale || !ed.horizontal {
			continue
		}
		if ed.effectiveMs < lo || ed.effectiveMs > commitMs {
			continue
		}
		spam := false
		for j := 0; j < st.stamps.n; j++ {
			if j == i {
				continue
			}
			o := st.stamps.edges[j]
			if o.effectiveMs > ed.effectiveMs-jgSpamMs && o.effectiveMs < ed.effectiveMs {
				spam = true
				break
			}
		}
		if spam {
			continue
		}
		bp := st.jg.mitigationBP(commitMs)
		st.jg.streak++
		st.jg.lastSuccessMs = commitMs
		st.jg.icdUntilMs = commitMs + jgIcdSuccessMs
		return true, jgSuccess, bp
	}
	st.jg.streak = 0
	st.jg.icdUntilMs = commitMs + jgIcdBaseMs
	return true, jgFail, 0
}

// horizontalEdge reports whether an edge qualifies for Just Guard:
// PRESS/RELEASE/FLIP with a LEFT/RIGHT direction — jump and drop edges
// carry UNSPECIFIED direction and never qualify.
func horizontalEdge(t protocolv1.MovementEdgeType, d protocolv1.Facing) bool {
	switch t {
	case protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_PRESS,
		protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_RELEASE,
		protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_FLIP:
	default:
		return false
	}
	return d == protocolv1.Facing_FACING_LEFT || d == protocolv1.Facing_FACING_RIGHT
}
