package inventory

import (
	"context"
	"errors"
	"testing"
	"time"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/edge/listener"
	"thinhthan/internal/edge/router"
	protocolv1 "thinhthan/internal/protocol/v1"
)

type spyConsult struct {
	tick   uint64
	ok     bool
	err    error
	called int
}

func (s *spyConsult) PartitionTick(characterID id.UUID) (uint64, bool, error) {
	s.called++
	return s.tick, s.ok, s.err
}

func rejectCode(err error) protocolv1.ErrorCode {
	var re *router.RejectError
	if errors.As(err, &re) {
		return re.Code
	}
	return protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED
}

func callMutate(s *Service, op protocolv1.InventoryOp) error {
	char := id.NewV4()
	opID := id.NewV7(time.Now())
	inst := id.NewV4()
	v := router.View{AccountID: id.NewV4(), SessionEpoch: 1,
		CharacterID: &char, OwnershipEpoch: 1}
	return s.mutate(context.Background(), v, router.Route{
		MsgID: 400,
		Inbound: listener.Inbound{Payload: &protocolv1.C2SInventoryMutate{
			OperationId:    opID[:],
			Op:             op,
			ItemInstanceId: inst[:],
			Quantity:       1,
		}},
	})
}

// USE admission requires a live PartitionTick consult (ADR-0083):
// unbound port, no-partition, and consult failure all reject
// INVALID_STATE — never a fabricated tick.
func TestUseAdmissionConsultFailures(t *testing.T) {
	q := &queue.Queue{}

	// Unbound consult.
	s := New(q, nil, nil)
	if code := rejectCode(callMutate(s, protocolv1.InventoryOp_INVENTORY_OP_USE)); code !=
		protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE {
		t.Fatalf("nil consults code %v", code)
	}

	// No live partition.
	sc := &spyConsult{ok: false}
	s = New(q, nil, sc)
	if code := rejectCode(callMutate(s, protocolv1.InventoryOp_INVENTORY_OP_USE)); code !=
		protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE || sc.called != 1 {
		t.Fatalf("no partition code %v calls %d", code, sc.called)
	}

	// Bounded-await consult error.
	sc = &spyConsult{ok: false, err: context.DeadlineExceeded}
	s = New(q, nil, sc)
	if code := rejectCode(callMutate(s, protocolv1.InventoryOp_INVENTORY_OP_USE)); code !=
		protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE {
		t.Fatalf("consult err code %v", code)
	}
}

// A bound consult admits past the consult gate: a zero-value queue then
// fails at Submit (SERVER_OVERLOADED), proving the consult was reached.
// MOVE skips the consult entirely.
func TestUseAdmissionConsultBound(t *testing.T) {
	q := &queue.Queue{}
	sc := &spyConsult{tick: 777, ok: true}
	s := New(q, nil, sc)

	if code := rejectCode(callMutate(s, protocolv1.InventoryOp_INVENTORY_OP_USE)); code !=
		protocolv1.ErrorCode_ERROR_CODE_SERVER_OVERLOADED || sc.called != 1 {
		t.Fatalf("bound consult code %v calls %d", code, sc.called)
	}

	sc.called = 0
	if code := rejectCode(callMutate(s, protocolv1.InventoryOp_INVENTORY_OP_MOVE)); sc.called != 0 ||
		code != protocolv1.ErrorCode_ERROR_CODE_SERVER_OVERLOADED {
		t.Fatalf("move consulted or code %v calls %d", code, sc.called)
	}
}
