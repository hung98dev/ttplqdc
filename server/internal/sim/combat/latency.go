package combat

// Latency-compensation model of combat.md § Just Guard (ADR-0038 §2,3):
//
//	sample            = server_receive_ms - client_mono_ms
//	base_offset       = min(sample) over the last 64 samples of the
//	                  session epoch
//	lag_ms            = sample - base_offset                 (>= 0)
//	compensation_ms   = min(lag_ms, 80)
//	effective_edge_ms = server_receive_ms - compensation_ms
//	RTT_estimate      = EWMA(alpha = 1/8) of heartbeat RTT; 200 ms
//	                  until the first heartbeat sample
//
// An edge with lag_ms > RTT_estimate + 80 is STALE_INPUT: it still
// applies to movement but never qualifies for Just Guard. The
// STALE_INPUT emission itself is movement's concern; this model only
// reports the verdict.
//
// The model is per-session-epoch; combat tracks one per player entity
// (a partition's player maps to one session).
const (
	latencyWindowSize  = 64
	latencyMaxCompMs   = 80
	latencyInitRttMs   = 200
	latencyEwmShift    = 3
	latencyFixedScale  = 256
	latencyStaleBoundM = 80
)

// LatencyModel tracks one session's samples.
type LatencyModel struct {
	samples [latencyWindowSize]int64
	n       int
	base    int64
	rttX256 int64
}

// NewLatencyModel returns a tracker at the protocol's 200 ms initial
// RTT estimate.
func NewLatencyModel() *LatencyModel {
	return &LatencyModel{rttX256: latencyInitRttMs * latencyFixedScale}
}

// NoteRtt folds one heartbeat round-trip sample into the EWMA.
func (m *LatencyModel) NoteRtt(rttMs int64) {
	m.rttX256 += (rttMs*latencyFixedScale - m.rttX256) >> latencyEwmShift
}

// RttMs reports the current RTT estimate.
func (m *LatencyModel) RttMs() int64 { return m.rttX256 / latencyFixedScale }

// record returns lag_ms for one (client_mono_ms, receive_ms) sample.
func (m *LatencyModel) record(clientMonoMs, receiveMs int64) (lag int64) {
	sample := receiveMs - clientMonoMs
	if m.n == 0 || sample < m.base {
		m.base = sample
	}
	m.samples[m.n%latencyWindowSize] = sample
	m.n++
	if m.n > latencyWindowSize {
		evicted := m.samples[(m.n-1)%latencyWindowSize]
		if evicted == m.base {
			m.base = m.samples[m.n%latencyWindowSize]
			for i := 1; i < latencyWindowSize; i++ {
				v := m.samples[(m.n+i)%latencyWindowSize]
				if v < m.base {
					m.base = v
				}
			}
		}
	}
	lag = sample - m.base
	if lag < 0 {
		lag = 0
	}
	return lag
}

// EffectiveEdgeMs applies the bounded compensation to a server receive
// stamp: effective = receive - min(lag, 80).
func (m *LatencyModel) EffectiveEdgeMs(clientMonoMs, receiveMs int64) int64 {
	lag := m.record(clientMonoMs, receiveMs)
	comp := lag
	if comp > latencyMaxCompMs {
		comp = latencyMaxCompMs
	}
	return receiveMs - comp
}

// Evaluate records the sample and reports the effective edge time plus
// the staleness verdict in one pass.
func (m *LatencyModel) Evaluate(clientMonoMs, receiveMs int64) (effectiveMs int64, stale bool) {
	lag := m.record(clientMonoMs, receiveMs)
	comp := lag
	if comp > latencyMaxCompMs {
		comp = latencyMaxCompMs
	}
	return receiveMs - comp, lag > m.RttMs()+latencyStaleBoundM
}
