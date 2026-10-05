package router

import (
	"context"
	"errors"
	"testing"

	"thinhthan/internal/edge/listener"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestHandlerRegistryUniqueIds: every durable-intent id accepts exactly
// one handler; re-registering or registering a non-durable id fails.
func TestHandlerRegistryUniqueIds(t *testing.T) {
	reg := New()
	h := func(ctx context.Context, c *listener.Conn, r Route) error { return nil }
	if err := reg.Register(12, h); err != nil {
		t.Fatalf("register 12: %v", err)
	}
	if err := reg.Register(12, h); !errors.Is(err, ErrDuplicateID) {
		t.Fatalf("dup register: %v", err)
	}
	if err := reg.Register(1, h); !errors.Is(err, ErrNotDurableIntent) {
		t.Fatalf("non-durable register: %v", err)
	}
	if err := reg.Register(400, nil); err == nil {
		t.Fatal("nil handler accepted")
	}
	// A spot-check of the closed §7 table.
	for _, id := range []uint32{12, 103, 104, 109, 400, 426, 500, 509, 608,
		632, 637, 645, 648, 656, 708, 730, 742} {
		if !IsDurableIntent(id) {
			t.Fatalf("id %d missing from durable table", id)
		}
	}
	for _, id := range []uint32{1, 3, 5, 100, 200, 300, 600, 700, 743, 999} {
		if IsDurableIntent(id) {
			t.Fatalf("id %d wrongly durable", id)
		}
	}
	// Interact kinds route to interaction.<kind lowercase> — unknown
	// kinds are rejected, not guessed.
	fam, ok := FamilyFor(103, &protocolv1.C2SInteract{
		InteractKind: protocolv1.InteractKind_INTERACT_KIND_BONFIRE_REST})
	if !ok || fam != "interaction.bonfire_rest" {
		t.Fatalf("interact family: %q %v", fam, ok)
	}
	if _, ok := FamilyFor(103, &protocolv1.C2SInteract{
		InteractKind: protocolv1.InteractKind_INTERACT_KIND_UNSPECIFIED}); ok {
		t.Fatal("unspecified interact kind routed")
	}
	// Social ids route to client.<ID>.
	if fam, ok := FamilyFor(608, nil); !ok || fam != "client.608" {
		t.Fatalf("social family: %q %v", fam, ok)
	}
}

// TestUnknownDurableIntentRejected: a durable-intent id with no
// registered handler is rejected OPERATION_REJECTED (never silently
// consumed); a durable id whose payload can't resolve its family is
// MESSAGE_NOT_ALLOWED_IN_STATE.
func TestUnknownDurableIntentRejected(t *testing.T) {
	reg := New()
	var re *RejectError
	err := reg.Dispatch(context.Background(), nil, listener.Inbound{
		MessageID: 400, Payload: &protocolv1.C2SInventoryMutate{},
	})
	if !errors.As(err, &re) || re.Code != protocolv1.ErrorCode_ERROR_CODE_OPERATION_REJECTED {
		t.Fatalf("unregistered dispatch: %v", err)
	}
	// id 103 with a payload that is not C2SInteract → family unresolved →
	// not-allowed-in-state.
	err = reg.Dispatch(context.Background(), nil, listener.Inbound{
		MessageID: 103, Payload: &protocolv1.C2SCharacterDetach{},
	})
	if !errors.As(err, &re) || re.Code != protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE {
		t.Fatalf("bad payload: %v", err)
	}
	// Registered handler receives the frame.
	reg = New()
	var got Route
	if err := reg.Register(400, func(ctx context.Context,
		c *listener.Conn, r Route) error {
		got = r
		return nil
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	in := listener.Inbound{MessageID: 400, Payload: &protocolv1.C2SInventoryMutate{}, ClientSeq: 7}
	if err := reg.Dispatch(context.Background(), nil, in); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if got.Inbound.ClientSeq != 7 || got.Family != "inventory.mutate" || got.MsgID != 400 {
		t.Fatalf("handler got %+v", got)
	}
}
