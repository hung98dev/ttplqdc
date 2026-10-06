package movement

import protocolv1 "thinhthan/internal/protocol/v1"

// EdgeByte is the single-octet discrete-intent encoding carried inside
// runtime.Intent.Edge. It folds the three discrete movement messages —
// C2S_JUMP (101), C2S_DROP_THROUGH (102) and C2S_MOVEMENT_EDGE (108) —
// into one flat value so enqueue costs zero allocations (runtime/input.go).
// Discrete edges are never coalesced (ADR-0038); the per-character queue
// cap is runtime.DiscreteCap.
const (
	EdgeJump uint8 = 0x01 // C2S_JUMP
	EdgeDrop uint8 = 0x02 // C2S_DROP_THROUGH

	EdgePressLeft    uint8 = 0x11 // MOVEMENT_EDGE_TYPE_PRESS  + FACING_LEFT
	EdgePressRight   uint8 = 0x12 // MOVEMENT_EDGE_TYPE_PRESS  + FACING_RIGHT
	EdgeReleaseLeft  uint8 = 0x13 // MOVEMENT_EDGE_TYPE_RELEASE + FACING_LEFT
	EdgeReleaseRight uint8 = 0x14 // MOVEMENT_EDGE_TYPE_RELEASE + FACING_RIGHT
	EdgeFlipLeft     uint8 = 0x15 // MOVEMENT_EDGE_TYPE_FLIP   + FACING_LEFT
	EdgeFlipRight    uint8 = 0x16 // MOVEMENT_EDGE_TYPE_FLIP   + FACING_RIGHT
)

// EncodeEdge maps a (type, direction) pair from the wire enums onto its
// EdgeByte. ok=false for UNSPECIFIED values or a JUMP/DROP-shaped call —
// use EdgeJump/EdgeDrop literals for those.
func EncodeEdge(t protocolv1.MovementEdgeType, dir protocolv1.Facing) (uint8, bool) {
	base, ok := edgeBase(t)
	if !ok {
		return 0, false
	}
	switch dir {
	case protocolv1.Facing_FACING_LEFT:
		return base, true
	case protocolv1.Facing_FACING_RIGHT:
		return base + 1, true
	}
	return 0, false
}

// DecodeEdge splits an EdgeByte back into (kind, direction). JUMP and DROP
// decode with FACING_UNSPECIFIED direction. ok=false on a reserved byte.
func DecodeEdge(b uint8) (protocolv1.MovementEdgeType, protocolv1.Facing, bool) {
	switch b {
	case EdgePressLeft:
		return protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_PRESS, protocolv1.Facing_FACING_LEFT, true
	case EdgePressRight:
		return protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_PRESS, protocolv1.Facing_FACING_RIGHT, true
	case EdgeReleaseLeft:
		return protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_RELEASE, protocolv1.Facing_FACING_LEFT, true
	case EdgeReleaseRight:
		return protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_RELEASE, protocolv1.Facing_FACING_RIGHT, true
	case EdgeFlipLeft:
		return protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_FLIP, protocolv1.Facing_FACING_LEFT, true
	case EdgeFlipRight:
		return protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_FLIP, protocolv1.Facing_FACING_RIGHT, true
	}
	return protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_UNSPECIFIED, protocolv1.Facing_FACING_UNSPECIFIED, false
}

func edgeBase(t protocolv1.MovementEdgeType) (uint8, bool) {
	switch t {
	case protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_PRESS:
		return EdgePressLeft, true
	case protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_RELEASE:
		return EdgeReleaseLeft, true
	case protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_FLIP:
		return EdgeFlipLeft, true
	}
	return 0, false
}

// EdgeEvent is one applied movement edge exported for downstream systems
// (IMP-014 Just Guard consumes effective_edge_ms timelines). ClientMonoMs
// is advisory metadata carried for the lag model — it is never used for
// authoritative sim timing (contract Inputs; F-02 wires the real feed).
type EdgeEvent struct {
	EntityID     uint64
	Type         protocolv1.MovementEdgeType
	Dir          protocolv1.Facing
	Tick         uint64
	ClientMonoMs int64
}
