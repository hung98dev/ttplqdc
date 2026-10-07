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
	h := func(ctx context.Context, v View, r Route) error { return nil }
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
	err := reg.Dispatch(context.Background(), View{}, listener.Inbound{
		MessageID: 400, Payload: &protocolv1.C2SInventoryMutate{},
	})
	if !errors.As(err, &re) || re.Code != protocolv1.ErrorCode_ERROR_CODE_OPERATION_REJECTED {
		t.Fatalf("unregistered dispatch: %v", err)
	}
	// id 103 with a payload that is not C2SInteract → family unresolved →
	// not-allowed-in-state.
	err = reg.Dispatch(context.Background(), View{}, listener.Inbound{
		MessageID: 103, Payload: &protocolv1.C2SCharacterDetach{},
	})
	if !errors.As(err, &re) || re.Code != protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE {
		t.Fatalf("bad payload: %v", err)
	}
	// Registered handler receives the frame.
	reg = New()
	var got Route
	var gotView View
	if err := reg.Register(400, func(ctx context.Context,
		v View, r Route) error {
		got = r
		gotView = v
		return nil
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	in := listener.Inbound{MessageID: 400, Payload: &protocolv1.C2SInventoryMutate{}, ClientSeq: 7}
	if err := reg.Dispatch(context.Background(), View{SessionEpoch: 42}, in); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if got.Inbound.ClientSeq != 7 || got.Family != "inventory.mutate" || got.MsgID != 400 {
		t.Fatalf("handler got %+v", got)
	}
	if gotView.SessionEpoch != 42 {
		t.Fatalf("handler got view %+v", gotView)
	}
}

// TestNonDurableRouteTable: registered non-durable C2S ids (ADR-0082)
// are reported by Handles the same way durable intents are, resolve no
// family, and invoke their handler with the same session view; §7 ids
// and session-owned ids can't register, unregistered non-durable ids
// keep the silent-consume path (Handles false).
func TestNonDurableRouteTable(t *testing.T) {
	reg := New()
	h := func(ctx context.Context, v View, r Route) error { return nil }
	// 208 is unregistered: not handled, dispatch is a no-op for the
	// adapter (ErrNotDurableIntent signals silent consume).
	if reg.Handles(208) {
		t.Fatal("unregistered 208 reported handled")
	}
	if err := reg.Dispatch(context.Background(), View{}, listener.Inbound{
		MessageID: 208, Payload: &protocolv1.C2SRespawnRequest{},
	}); !errors.Is(err, ErrNotDurableIntent) {
		t.Fatalf("unregistered 208 dispatch: %v", err)
	}
	// §7 durable ids and session-owned ids are rejected.
	if err := reg.RegisterNonDurable(400, h); !errors.Is(err, ErrNotNonDurableID) {
		t.Fatalf("durable register: %v", err)
	}
	for _, id := range []uint32{1, 4, 6, 10, 11} {
		if err := reg.RegisterNonDurable(id, h); !errors.Is(err, ErrNotNonDurableID) {
			t.Fatalf("session id %d accepted", id)
		}
	}
	if err := reg.RegisterNonDurable(208, nil); err == nil {
		t.Fatal("nil handler accepted")
	}
	// Registered id dispatches through with an empty family and the
	// injected view.
	var got Route
	var gotView View
	if err := reg.RegisterNonDurable(208, func(ctx context.Context,
		v View, r Route) error {
		got = r
		gotView = v
		return nil
	}); err != nil {
		t.Fatalf("register 208: %v", err)
	}
	if err := reg.RegisterNonDurable(208, h); !errors.Is(err, ErrDuplicateID) {
		t.Fatalf("dup register: %v", err)
	}
	if !reg.Handles(208) {
		t.Fatal("registered 208 not handled")
	}
	in := listener.Inbound{MessageID: 208, Payload: &protocolv1.C2SRespawnRequest{}, ClientSeq: 9}
	if err := reg.Dispatch(context.Background(), View{SessionEpoch: 7}, in); err != nil {
		t.Fatalf("dispatch 208: %v", err)
	}
	if got.MsgID != 208 || got.Family != "" || got.Inbound.ClientSeq != 9 {
		t.Fatalf("handler got %+v", got)
	}
	if gotView.SessionEpoch != 7 {
		t.Fatalf("handler got view %+v", gotView)
	}
	// The durable path is unchanged.
	for _, id := range []uint32{12, 103, 400, 608, 708} {
		if !reg.Handles(id) {
			t.Fatalf("durable %d not handled", id)
		}
	}
	var re *RejectError
	if err := reg.Dispatch(context.Background(), View{}, listener.Inbound{
		MessageID: 400, Payload: &protocolv1.C2SInventoryMutate{},
	}); !errors.As(err, &re) || re.Code != protocolv1.ErrorCode_ERROR_CODE_OPERATION_REJECTED {
		t.Fatalf("unregistered durable dispatch: %v", err)
	}
}
