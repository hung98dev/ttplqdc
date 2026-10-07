package inventory

import (
	"fmt"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// OpKind is the C2S_INVENTORY_MUTATE op enum mirrored into the durable
// domain. Wire values map 1:1 (protocolv1.InventoryOp).
type OpKind byte

const (
	OpMove    OpKind = 1
	OpSplit   OpKind = 2
	OpMerge   OpKind = 3
	OpSort    OpKind = 4
	OpDiscard OpKind = 5
	OpUse     OpKind = 6
)

// opKindOf maps the wire enum; UNSPECIFIED/unknown is an invalid request.
func opKindOf(op protocolv1.InventoryOp) (OpKind, bool) {
	switch op {
	case protocolv1.InventoryOp_INVENTORY_OP_MOVE:
		return OpMove, true
	case protocolv1.InventoryOp_INVENTORY_OP_SPLIT:
		return OpSplit, true
	case protocolv1.InventoryOp_INVENTORY_OP_MERGE:
		return OpMerge, true
	case protocolv1.InventoryOp_INVENTORY_OP_SORT:
		return OpSort, true
	case protocolv1.InventoryOp_INVENTORY_OP_DISCARD:
		return OpDiscard, true
	case protocolv1.InventoryOp_INVENTORY_OP_USE:
		return OpUse, true
	}
	return 0, false
}

// MutateOp is the validated request view for one inventory.mutate
// record. FromSlot is deliberately absent: the source slot is the
// instance's current committed slot (messages.md §400).
type MutateOp struct {
	Op         OpKind
	InstanceID id.UUID
	ToSlot     uint32
	Quantity   uint32
}

// validate enforces request-shape invariants per op before any row
// lookup; violations are deterministic INVALID_STATE verdicts.
func (o MutateOp) validate() error {
	switch o.Op {
	case OpMove:
		if o.Quantity != 0 {
			return fmt.Errorf("%w: move quantity must be 0", ErrInvalidState)
		}
	case OpSplit:
		if o.Quantity == 0 {
			return fmt.Errorf("%w: split quantity must be >0", ErrInvalidState)
		}
	case OpMerge:
		// quantity ignored: merge always transfers min(source, remaining)
	case OpSort:
		// whole-inventory op: instance/slot/quantity ignored
	case OpDiscard:
		// quantity 0 = whole stack; else partial units
	case OpUse:
		if o.Quantity != 1 {
			return fmt.Errorf("%w: use quantity must be 1", ErrInvalidState)
		}
	default:
		return fmt.Errorf("%w: unknown op", ErrInvalidState)
	}
	return nil
}
