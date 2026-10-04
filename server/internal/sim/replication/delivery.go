package replication

import (
	protocolv1 "thinhthan/internal/protocol/v1"

	"google.golang.org/protobuf/proto"
)

// Message ids from docs/05_network/messages.md.
const (
	MsgBaseline       uint32 = 300 // S2C_WORLD_BASELINE
	MsgEntitySpawn    uint32 = 301 // S2C_ENTITY_SPAWN
	MsgEntityDespawn  uint32 = 302 // S2C_ENTITY_DESPAWN
	MsgStateDelta     uint32 = 303 // S2C_STATE_DELTA
	MsgBaselineAck    uint32 = 306 // C2S_BASELINE_ACK
	MsgBaselineResync uint32 = 307 // C2S_BASELINE_RESYNC_REQUEST
	MsgResyncResult   uint32 = 308 // S2C_BASELINE_RESYNC_RESULT
)

// DeliveryClass is the queue class of docs/05_network/messages.md's delivery
// table. The edge queue enforces per-class behavior; the sim only supplies
// the classification.
type DeliveryClass int

const (
	// DeliveryControl covers session, heartbeat and error traffic; never
	// coalesced.
	DeliveryControl DeliveryClass = iota
	// DeliveryReplaceableState covers input state, snapshots and deltas;
	// queue entries supersede by (message_id, entity-or-state key) with
	// field-wise merge.
	DeliveryReplaceableState
	// DeliveryDiscreteIntent covers C2S intent; never coalesced.
	DeliveryDiscreteIntent
	// DeliveryAuthoritativeEvent covers ordered S2C events; never
	// coalesced.
	DeliveryAuthoritativeEvent
	// DeliveryDurableResult covers S2C_*_RESULT replies; never dropped.
	DeliveryDurableResult
)

// MessageID returns the wire message id for a replication message. It
// reports false for messages outside the replication family.
func MessageID(m proto.Message) (uint32, bool) {
	switch m.(type) {
	case *protocolv1.S2CWorldBaseline:
		return MsgBaseline, true
	case *protocolv1.S2CEntitySpawn:
		return MsgEntitySpawn, true
	case *protocolv1.S2CEntityDespawn:
		return MsgEntityDespawn, true
	case *protocolv1.S2CStateDelta:
		return MsgStateDelta, true
	case *protocolv1.C2SBaselineAck:
		return MsgBaselineAck, true
	case *protocolv1.C2SBaselineResyncRequest:
		return MsgBaselineResync, true
	case *protocolv1.S2CBaselineResyncResult:
		return MsgResyncResult, true
	}
	return 0, false
}

// ClassOf returns the delivery class the edge queue applies to m.
func ClassOf(m proto.Message) DeliveryClass {
	switch m.(type) {
	case *protocolv1.S2CWorldBaseline, *protocolv1.S2CStateDelta:
		return DeliveryReplaceableState
	case *protocolv1.S2CEntitySpawn, *protocolv1.S2CEntityDespawn:
		return DeliveryAuthoritativeEvent
	case *protocolv1.C2SBaselineAck:
		return DeliveryControl
	case *protocolv1.C2SBaselineResyncRequest:
		return DeliveryDiscreteIntent
	case *protocolv1.S2CBaselineResyncResult:
		return DeliveryDurableResult
	}
	return DeliveryControl
}

// SupersedeKey returns the queue coalescing key for m: (message_id,
// entity-or-state key). It reports ok=false for classes that never
// coalesce — authoritative events (spawn/despawn), baselines (each new
// baseline_id supersedes nothing: the client must see it), control and
// durable results — so only a state delta yields a key, which callers scope
// per replication client.
func SupersedeKey(m proto.Message) (messageID uint32, stateKey uint64, ok bool) {
	switch t := m.(type) {
	case *protocolv1.S2CStateDelta:
		// One pending delta per client supersedes; baseline_id keeps a
		// stale pre-resync delta from merging into the new baseline's
		// stream.
		return MsgStateDelta, t.GetBaselineId(), true
	}
	return 0, 0, false
}
