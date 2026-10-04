package listener

import "time"

// bucketSpec is one token-bucket row of docs/07_security/rate_limits.md:
// sustained `rate` tokens per second with `burst` capacity.
type bucketSpec struct {
	key   string
	rate  float64
	burst float64
}

func (s bucketSpec) new() *bucket { return &bucket{spec: s, tokens: s.burst} }

type bucket struct {
	spec   bucketSpec
	tokens float64
	last   time.Time
}

// allow consumes one token when available at `now`.
func (b *bucket) allow(now time.Time) bool {
	if b.last.IsZero() {
		b.last = now
	}
	if d := now.Sub(b.last).Seconds(); d > 0 {
		b.tokens += d * b.spec.rate
		if b.tokens > b.spec.burst {
			b.tokens = b.spec.burst
		}
		b.last = now
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// rateBuckets maps a C2S message_id to its per-connection bucket set
// (rate_limits.md § Realtime Defaults, § Chat, § Per-Operation Sub-limits,
// read-request buckets). A message may carry several buckets (an aggregate
// durable-mutations ceiling plus a stricter per-operation sub-limit).
func rateBuckets(id uint32) []bucketSpec {
	const (
		sec = 1.0
		min = 60 * sec
	)
	switch id {
	case 100: // C2S_INPUT_STATE: sustained 30/s, burst 60
		return []bucketSpec{{key: "input_state", rate: 30 / sec, burst: 60}}
	case 108: // C2S_MOVEMENT_EDGE: 20/s burst 30
		return []bucketSpec{{key: "movement_edge", rate: 20 / sec, burst: 30}}
	case 101, 102, 200, 201, 202: // discrete combat/input
		return []bucketSpec{{key: "discrete_input", rate: 20 / sec, burst: 30}}
	case 4: // heartbeat: configured cadence + small tolerance
		return []bucketSpec{{key: "heartbeat", rate: 0.4, burst: 2}}
	case 738: // auction search: 1/s burst 5
		return []bucketSpec{{key: "auction_search", rate: 1 / sec, burst: 5}}
	case 307: // baseline resync read: 1 / 5 s burst 1
		return []bucketSpec{{key: "baseline_resync", rate: 0.2, burst: 1}}
	case 442: // soul list read: 5/s burst 5
		return []bucketSpec{{key: "soul_list", rate: 5 / sec, burst: 5}}
	case 516: // daily board read: 1/s burst 2
		return []bucketSpec{{key: "daily_board", rate: 1 / sec, burst: 2}}
	case 600: // chat: 5/10s burst + 20/60s sustained
		return []bucketSpec{
			{key: "chat_burst", rate: 5.0 / 10.0, burst: 5},
			{key: "chat_sustained", rate: 20.0 / 60.0, burst: 20},
		}
	case 103, 104, 106, 109, 111, 113, 114, 117:
		// interaction/portal family (incl. dungeon and channel ops): 10/s burst 20
		return []bucketSpec{{key: "interaction", rate: 10 / sec, burst: 20}}
	case 1, 6, 10, 12:
		// session handshake / character lifecycle ops
		return []bucketSpec{{key: "session_ops", rate: 2 / sec, burst: 4}}
	case 306:
		// baseline ack housekeeping
		return []bucketSpec{{key: "baseline_ack", rate: 10 / sec, burst: 20}}
	case 700: // direct trade invitation: sub-limit 3/60s + aggregate
		return []bucketSpec{
			{key: "durable_mutations", rate: 10 / sec, burst: 20},
			{key: "trade_invite", rate: 3 / min, burst: 3},
		}
	case 730: // auction listing creation: sub-limit 10/60min + aggregate
		return []bucketSpec{
			{key: "durable_mutations", rate: 10 / sec, burst: 20},
			{key: "auction_list", rate: 10 / (60 * min), burst: 10},
		}
	case 732: // auction purchase: sub-limit 20/60s + aggregate
		return []bucketSpec{
			{key: "durable_mutations", rate: 10 / sec, burst: 20},
			{key: "auction_buy", rate: 20 / min, burst: 20},
		}
	case 408: // reward claim: sub-limit 30/60s + aggregate
		return []bucketSpec{
			{key: "durable_mutations", rate: 10 / sec, burst: 20},
			{key: "reward_claim", rate: 30 / min, burst: 30},
		}
	case 602, 608, 611:
		// social invite (party/guild/friend outbound): sub-limit 10/60s + aggregate
		return []bucketSpec{
			{key: "durable_mutations", rate: 10 / sec, burst: 20},
			{key: "social_invite", rate: 10 / min, burst: 10},
		}
	case 439:
		// reward-claim list read (request_id echo-only): bounded read bucket
		return []bucketSpec{{key: "claim_list_read", rate: 5 / sec, burst: 5}}
	}
	// Remaining C2S messages (durable-operation and management families)
	// share the aggregate durable-mutations bucket.
	return []bucketSpec{{key: "durable_mutations", rate: 10 / sec, burst: 20}}
}

// rejectBudget counts non-closing protocol rejections in a sliding window:
// more than 20 in any 10 s closes the connection with PROTOCOL_VIOLATION
// (rate_limits.md § Protocol Reject Budget). Rate-limit rejections count
// toward their own buckets, never this budget.
type rejectBudget struct {
	times [21]time.Time
	n     int
	head  int
}

// record appends a rejection at `now` and reports whether the budget is
// exhausted (> 20 rejections inside `window`).
func (r *rejectBudget) record(now, windowStart time.Time) bool {
	r.times[r.head] = now
	r.head = (r.head + 1) % len(r.times)
	if r.n < len(r.times) {
		r.n++
	}
	kept := 0
	for i := 0; i < r.n; i++ {
		idx := (r.head - 1 - i + len(r.times)) % len(r.times)
		if !r.times[idx].Before(windowStart) {
			kept++
		}
	}
	return kept > 20
}
