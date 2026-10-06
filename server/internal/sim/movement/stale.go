package movement

import (
	"log/slog"

	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/replication"
	"thinhthan/internal/sim/runtime"
)

// Lag-model constants from combat.md § Latency compensation:
// the offset baseline is the minimum of the last 64 samples of the
// session epoch, lag compensation caps at 80 ms, and RTT starts at the
// 200 ms estimate until the first heartbeat sample.
const (
	staleWindowSize  = 64
	staleMaxCompMs   = 80
	staleInitRttMs   = 200
	staleEwmAlphaInv = 8   // EWMA alpha = 1/8, computed in fixed point
	staleEwmaShift   = 3   // log2(8)
	staleFixedScale  = 256 // fixed-point ms for the EWMA accumulator
)

// StaleTracker evaluates the anti-cheat staleness bound of ADR-0038 §3
// per session epoch: sample = receive_ms - client_mono_ms, base = min of
// the last 64 samples, lag = sample - base; an edge is stale when
// lag > RTT_estimate + 80. The edge still applies to movement — staleness
// only disqualifies it for Just Guard and emits S2C_ERROR STALE_INPUT.
//
// The tracker is intentionally feed-agnostic (F-02): the caller supplies
// (clientMonoMs, receiveMs) stamps once the runtime/edge surface carries
// them, and heartbeat RTT via NoteRtt. Until then evaluations stay off.
type StaleTracker struct {
	samples [staleWindowSize]int64
	n       int
	base    int64
	rttX256 int64 // RTT EWMA in 1/256 ms
}

// NewStaleTracker returns a tracker with the protocol's 200 ms initial
// RTT estimate.
func NewStaleTracker() *StaleTracker {
	return &StaleTracker{rttX256: staleInitRttMs * staleFixedScale}
}

// NoteRtt folds one heartbeat round-trip sample into the EWMA
// (alpha = 1/8, combat.md latency model).
func (t *StaleTracker) NoteRtt(rttMs int64) {
	if t.rttX256 == 0 {
		t.rttX256 = rttMs * staleFixedScale
		return
	}
	t.rttX256 += (rttMs*staleFixedScale - t.rttX256) >> staleEwmaShift
}

// EdgeStale records one edge's (client_mono_ms, receive_ms) pair and
// reports whether its lag exceeded RTT+80.
func (t *StaleTracker) EdgeStale(clientMonoMs, receiveMs int64) bool {
	sample := receiveMs - clientMonoMs
	if t.n == 0 || sample < t.base {
		t.base = sample
	}
	t.samples[t.n%staleWindowSize] = sample
	t.n++
	if t.n > staleWindowSize {
		// Recompute the sliding minimum when the evicted slot may have
		// held it — bounded 64-element scan, still allocation-free.
		evicted := t.samples[(t.n-1)%staleWindowSize]
		if evicted == t.base {
			t.base = t.samples[t.n%staleWindowSize]
			for i := 1; i < staleWindowSize; i++ {
				v := t.samples[(t.n+i)%staleWindowSize]
				if v < t.base {
					t.base = v
				}
			}
		}
	}
	lag := sample - t.base
	rtt := t.rttX256 / staleFixedScale
	return lag > rtt+staleMaxCompMs
}

// evalStale applies the staleness verdict to one edge: a stale edge emits
// S2C_ERROR STALE_INPUT to its owner and logs (op, revision, actor,
// session) per combat.md; the edge still applies.
func (s *System) evalStale(e *runtime.Entity, clientMonoMs, receiveMs int64, tc *runtime.TickContext) {
	if !s.stale.EdgeStale(clientMonoMs, receiveMs) {
		return
	}
	s.log.Warn("stale movement edge rejected",
		slog.String("op", "C2S_MOVEMENT_EDGE"),
		slog.String("revision", ""),
		slog.Uint64("actor_id", e.ID),
		slog.Uint64("tick", tc.Tick),
	)
	if s.outbound == nil {
		return
	}
	_ = s.outbound.Enqueue(runtime.Outbound{
		To:        e.ID,
		MessageID: 3, // S2C_ERROR
		Class:     replication.DeliveryControl,
		Msg: &protocolv1.S2CError{
			ErrorCode:    protocolv1.ErrorCode_ERROR_CODE_STALE_INPUT,
			Retryability: protocolv1.Retryability_RETRYABILITY_NEVER,
		},
	})
}
