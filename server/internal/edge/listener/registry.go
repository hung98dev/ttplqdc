package listener

import (
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// DeliveryClass mirrors docs/05_network/messages.md § Delivery Classes.
type DeliveryClass int

const (
	DeliveryControl DeliveryClass = iota
	DeliveryReplaceableState
	DeliveryDiscreteIntent
	DeliveryAuthoritativeEvent
	DeliveryDurableResult
)

// registryRow is one messages.md registry line compiled into the listener.
type registryRow struct {
	id     uint32
	name   string
	goType string
	c2s    bool
}

// registryEntry is a row enriched with the wire facts the validation table
// and outbound queue need: payload type, delivery class, realtime-input
// membership and the rate bucket set.
type registryEntry struct {
	row      registryRow
	typ      protoreflect.MessageType // nil for S2C-only rows (never decoded inbound)
	class    DeliveryClass
	realtime bool
	buckets  []bucketSpec
}

// realtimeInput is the canonical set of docs/05_network/protocol.md § Phase
// Legality: silently dropped in DEAD / TRANSFER / PLACEMENT_PENDING.
var realtimeInput = map[uint32]bool{100: true, 101: true, 102: true, 108: true, 200: true, 201: true, 202: true}

var byID = buildRegistry()

func buildRegistry() map[uint32]*registryEntry {
	byID := make(map[uint32]*registryEntry, len(registryTable))
	for i := range registryTable {
		r := registryTable[i]
		e := &registryEntry{row: r, realtime: realtimeInput[r.id]}
		e.class = classOf(r)
		if r.c2s {
			mt, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName("thinhthan.v1." + r.goType))
			if err == nil {
				e.typ = mt
			}
			e.buckets = rateBuckets(r.id)
		}
		byID[r.id] = e
	}
	return byID
}

// classOf applies the default-class rules of messages.md § Delivery Classes.
func classOf(r registryRow) DeliveryClass {
	if r.id >= 1 && r.id <= 11 {
		return DeliveryControl // session IDs incl. heartbeat + S2C_ERROR
	}
	if r.c2s {
		if r.id == 100 {
			return DeliveryReplaceableState // C2S_INPUT_STATE
		}
		return DeliveryDiscreteIntent // C2S_MOVEMENT_EDGE + every other C2S
	}
	if strings.HasSuffix(r.name, "_RESULT") {
		return DeliveryDurableResult
	}
	if r.id == 300 || r.id == 303 || strings.HasSuffix(r.name, "_STATE") || r.name == "S2C_REWARD_CLAIM_DELTA" {
		return DeliveryReplaceableState // snapshots, state pushes, deltas
	}
	return DeliveryAuthoritativeEvent
}

// lookup returns the registry entry for a wire message_id (nil when the ID
// is not registered in messages.md — MESSAGE_UNKNOWN territory).
func lookup(id uint32) *registryEntry { return byID[id] }

// newPayload allocates a fresh payload message for a registered C2S id.
func (e *registryEntry) newPayload() proto.Message {
	if e.typ == nil {
		return nil
	}
	return e.typ.New().Interface()
}

// phaseAllowed reports whether a registered C2S id is dispatched in the
// connection phase per protocol.md § Phase Legality. Returns the outcome
// kind: dispatch, silent drop (realtime input in DEAD/TRANSFER/PENDING) or
// MESSAGE_NOT_ALLOWED_IN_STATE.
type phaseVerdict int

const (
	dispatchOK phaseVerdict = iota
	dropSilently
	notAllowed
)

// phaseSet is the dispatched C2S set for each phase (protocol.md § Phase
// Legality table). IN_WORLD's set is computed: every registered C2S except
// HELLO and CHARACTER_CREATE.
func phaseVerdictFor(e *registryEntry, st *ConnState) phaseVerdict {
	switch st.Phase {
	case PhasePreHello:
		if e.row.id == 1 {
			return dispatchOK
		}
		return notAllowed // handled as PROTOCOL_VIOLATION close by the caller
	case PhaseCharacterSelect:
		switch e.row.id {
		case 4, 6, 12:
			return dispatchOK
		}
	case PhaseInWorld:
		if e.row.c2s && e.row.id != 1 && e.row.id != 12 {
			return dispatchOK
		}
	case PhaseDead, PhaseTransfer, PhasePlacementPending:
		if e.realtime {
			return dropSilently
		}
		switch st.Phase {
		case PhaseDead:
			// IN_WORLD set minus realtime input.
			if e.row.c2s && e.row.id != 1 && e.row.id != 12 {
				return dispatchOK
			}
		case PhaseTransfer:
			switch e.row.id {
			case 4, 106, 306, 600:
				return dispatchOK
			}
		case PhasePlacementPending:
			switch e.row.id {
			case 4:
				return dispatchOK
			case 600:
				if st.Attached {
					return dispatchOK
				}
			}
		}
	}
	return notAllowed
}

// silence for go vet on unused import path when types change.
var _ = protocolv1.Envelope{}
