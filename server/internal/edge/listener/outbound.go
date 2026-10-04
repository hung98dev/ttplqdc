package listener

import (
	"errors"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// outEntry is one queued outbound frame. Payload holds the already-encoded
// inner message; env meta (msgid/epoch/corr) rides along. Entries are
// per-message — the write loop encodes the envelope at dequeue so server_seq
// is assigned in wire order.
type outEntry struct {
	msgID         uint32
	correlationID uint64
	payload       []byte // encoded inner message (owned by the queue)
	class         DeliveryClass
	// supKey identifies the supersede group for REPLACEABLE_STATE frames:
	// (message_id, key). For S2C_STATE_DELTA the key is the delta's
	// baseline_id; for every other replaceable frame it is 0 (singleton per
	// message_id).
	supKey   supKey
	enqueued time.Time
	estBytes int // encoded-envelope size estimate for byte accounting
}

type supKey struct {
	msgID uint32
	key   uint64
}

// outboundQueue is the per-connection send queue of protocol.md
// § Connection Backpressure: at most `frames` queued frames or `bytes`
// pending bytes; REPLACEABLE_STATE frames supersede an earlier frame with
// the same key; unsent S2C_STATE_DLTAs merge field-wise; a non-replaceable
// frame that cannot fit — or >75 % fill for 5 s — closes the connection
// with WS 4008 SLOW_CONSUMER.
type outboundQueue struct {
	entries    []*outEntry
	supIndex   map[supKey]int // supersede group → queue position
	frames     int
	bytes      int
	maxFrames  int
	maxBytes   int
	overSince  time.Time // first moment fill exceeded 75 %, zero otherwise
	now        func() time.Time
	mergeQueue int // count of frames merged into an earlier entry (metric)
}

func newOutboundQueue(maxFrames, maxBytes int, now func() time.Time) *outboundQueue {
	return &outboundQueue{
		supIndex:  make(map[supKey]int),
		maxFrames: maxFrames,
		maxBytes:  maxBytes,
		now:       now,
	}
}

func (q *outboundQueue) clock() time.Time {
	if q.now != nil {
		return q.now()
	}
	return time.Now()
}

func (q *outboundQueue) overCap() bool {
	return q.frames > 3*q.maxFrames/4 || q.bytes > 3*q.maxBytes/4
}

// offer inserts one encoded payload into the queue. Returns
// ErrSlowConsumer when a non-replaceable frame cannot fit; the caller then
// closes the connection with WS 4008.
func (q *outboundQueue) offer(e *outEntry) error {
	if e.class == DeliveryReplaceableState {
		if at, ok := q.supIndex[e.supKey]; ok {
			old := q.entries[at]
			if e.msgID == 303 {
				// S2C_STATE_DELTA merges field-wise with the unsent older
				// delta rather than replacing it (protocol.md §174 —
				// newer wins per field, absence unchanged).
				merged, err := mergeStateDelta(old.payload, e.payload)
				if err != nil {
					return err
				}
				prev := old.estBytes
				old.payload = merged
				old.estBytes = len(merged) + envOverhead
				q.bytes += old.estBytes - prev
				q.mergeQueue++
			} else {
				// Newer frame with the same key supersedes: the queued
				// entry is replaced wholesale and keeps its position.
				q.bytes += e.estBytes - old.estBytes
				q.entries[at] = e
			}
			return nil
		}
	}
	if q.frames+1 > q.maxFrames || q.bytes+e.estBytes > q.maxBytes {
		// Non-replaceable (or non-supersedable) frame that cannot fit.
		return ErrSlowConsumer
	}
	q.supIndex[e.supKey] = q.frames
	q.entries = append(q.entries, e)
	q.frames++
	q.bytes += e.estBytes
	e.enqueued = q.clock()
	return nil
}

// ErrSlowConsumer reports a frame that could not be queued after
// superseding — the connection closes with WS 4008 (protocol.md
// § Connection Backpressure).
var ErrSlowConsumer = errors.New("edge/listener: slow consumer")

// pop removes the head entry.
func (q *outboundQueue) pop() *outEntry {
	if q.frames == 0 {
		return nil
	}
	e := q.entries[0]
	q.entries = q.entries[1:]
	q.frames--
	q.bytes -= e.estBytes
	delete(q.supIndex, e.supKey)
	// supIndex positions shifted — rebuild the index (supersede lookups are
	// rare relative to pops; a full rebuild keeps the code dead simple).
	q.supIndex = make(map[supKey]int, q.frames)
	for i, en := range q.entries {
		q.supIndex[en.supKey] = i
	}
	return e
}

// checkSlowConsumer applies the "above 75 % continuously for 5 s" rule and
// reports whether the connection must close with WS 4008.
func (q *outboundQueue) checkSlowConsumer(now time.Time) bool {
	if !q.overCap() {
		q.overSince = time.Time{}
		return false
	}
	if q.overSince.IsZero() {
		q.overSince = now
		return false
	}
	return now.Sub(q.overSince) > 5*time.Second
}

const envOverhead = 32 // envelope fields + frame header estimate

// supKeyFor computes the supersede key of a queued frame: (message_id,
// baseline_id) for S2C_STATE_DELTA — read straight off the wire bytes,
// no decode — and (message_id, 0) for every other replaceable frame.
func supKeyFor(msgID uint32, payload []byte) supKey {
	k := supKey{msgID: msgID}
	if msgID != 303 {
		return k
	}
	b := payload
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			break
		}
		b = b[n:]
		if num == 1 && typ == protowire.VarintType {
			v, m := protowire.ConsumeVarint(b)
			if m > 0 {
				k.key = v
			}
			return k
		}
		m := protowire.ConsumeFieldValue(num, typ, b)
		if m < 0 {
			break
		}
		b = b[m:]
	}
	return k
}

// mergeStateDelta merges `newer` into `older` field-wise per protocol.md
// §174: optional and self-private fields merge field-wise (newer wins,
// absence unchanged), the newest complete SelfAck is retained, explicitly
// present list wrappers replace, and the merge never falls back to a whole
// snapshot.
func mergeStateDelta(older, newer []byte) ([]byte, error) {
	var od, nd protocolv1.S2CStateDelta
	if err := proto.Unmarshal(older, &od); err != nil {
		return nil, err
	}
	if err := proto.Unmarshal(newer, &nd); err != nil {
		return nil, err
	}
	mergeDeltaInto(&od, &nd)
	return proto.Marshal(&od)
}

// mergeDeltaInto merges src into dst field-wise (proto3 optional pointer
// fields: newer wins where present, absent fields keep the old value).
// Entity deltas merge per entity_id — a delta for an entity already queued
// merges into that queued entry field-wise; new entities append (server
// order preserved).
func mergeDeltaInto(dst, src *protocolv1.S2CStateDelta) {
	dst.BaselineId = src.BaselineId
	dst.ServerTick = src.ServerTick
	if src.SelfAck != nil {
		dst.SelfAck = src.SelfAck // newest complete SelfAck retained
	}
	if src.SelfPrivate != nil {
		if dst.SelfPrivate == nil {
			dst.SelfPrivate = src.SelfPrivate
		} else {
			proto.Merge(dst.SelfPrivate, src.SelfPrivate)
		}
	}
	if len(src.Entities) == 0 {
		return
	}
	pos := make(map[uint64]*protocolv1.EntityDelta, len(dst.Entities))
	for _, e := range dst.Entities {
		pos[e.EntityId] = e
	}
	for _, ne := range src.Entities {
		if old := pos[ne.EntityId]; old != nil {
			proto.Merge(old, ne) // field-wise, newer wins
		} else {
			dst.Entities = append(dst.Entities, ne)
			pos[ne.EntityId] = ne
		}
	}
}
