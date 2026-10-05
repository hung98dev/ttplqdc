package session

import (
	"testing"
	"time"

	"thinhthan/internal/core/id"
	"thinhthan/internal/edge/listener"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestFifoAdmission: at capacity the queue admits strictly FIFO — the
// head only gains a slot once the holder releases, and positions reflect
// arrival order.
func TestFifoAdmission(t *testing.T) {
	reg, store, clk := newTestRegistry(t, 1)
	a := seedAccount(t, store)
	b := seedAccount(t, store)
	c := seedAccount(t, store)
	ctx := testCtx()
	plat := protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS

	tkA, err := reg.IssueTicket(ctx, a, id.UUID{}, 1, plat, 0, "")
	if err != nil || tkA.Credential == "" {
		t.Fatalf("A ticket: %v", err)
	}
	tkB, err := reg.IssueTicket(ctx, b, id.UUID{}, 1, plat, 0, "")
	if err != nil {
		t.Fatalf("B ticket: %v", err)
	}
	if tkB.Credential != "" || tkB.QueuePosition != 1 || tkB.RetryAfterMs != 5000 {
		t.Fatalf("B must be head of queue: %+v", tkB)
	}
	tkC, err := reg.IssueTicket(ctx, c, id.UUID{}, 1, plat, 0, "")
	if err != nil {
		t.Fatalf("C ticket: %v", err)
	}
	if tkC.QueuePosition != 2 {
		t.Fatalf("C position: %+v", tkC)
	}
	// B re-requests while the slot is still held → stays queued.
	tkB2, err := reg.IssueTicket(ctx, b, id.UUID{}, 1, plat, 0, "")
	if err != nil || tkB2.Credential != "" || tkB2.QueuePosition != 1 {
		t.Fatalf("B head re-request: %+v", tkB2)
	}
	// A's HELLO consumes its slot into the session.
	d := reg.Hello(ctx, listener.HelloMeta{}, helloTicket(tkA.Credential, 1))
	if d.OK == nil {
		t.Fatalf("A hello: %+v", d.Reject)
	}
	// A logs out (release) → B's next request admits.
	reg.mu.Lock()
	s := reg.byAccount[a]
	reg.releaseLocked(s)
	reg.mu.Unlock()
	tkB3, err := reg.IssueTicket(ctx, b, id.UUID{}, 1, plat, 0, "")
	if err != nil || tkB3.Credential == "" {
		t.Fatalf("B admit after release: %+v err=%v", tkB3, err)
	}
	// C still queued behind.
	tkC2, err := reg.IssueTicket(ctx, c, id.UUID{}, 1, plat, 0, "")
	if err != nil || tkC2.Credential != "" || tkC2.QueuePosition != 1 {
		t.Fatalf("C after B admitted: %+v", tkC2)
	}
	_ = clk
}

// TestReconnectBypassesQueue: an account with a live character (or one
// inside reconnect grace) skips the queue on the ticket path.
func TestReconnectBypassesQueue(t *testing.T) {
	reg, store, _ := newTestRegistry(t, 1)
	a := seedAccount(t, store)
	b := seedAccount(t, store)
	charA := seedCharacter(t, store, a, "hero.live")
	ctx := testCtx()
	plat := protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS

	tkA, _ := reg.IssueTicket(ctx, a, id.UUID{}, 1, plat, 0, "")
	d := reg.Hello(ctx, listener.HelloMeta{}, helloTicket(tkA.Credential, 1))
	if err := reg.Enqueue(ctx, nil, listener.Inbound{
		MessageID: 6, SessionEpoch: d.OK.SessionEpoch, ClientSeq: 2,
		Payload: &protocolv1.C2SCharacterAttach{CharacterId: charA[:]},
	}); err != nil {
		t.Fatalf("attach: %v", err)
	}
	// B queues behind the full capacity.
	tkB, _ := reg.IssueTicket(ctx, b, id.UUID{}, 1, plat, 0, "")
	if tkB.Credential != "" {
		t.Fatal("B unexpectedly admitted")
	}
	// A requests a ticket while holding a live character → bypass.
	tkA2, err := reg.IssueTicket(ctx, a, id.UUID{}, 1, plat, 0, "")
	if err != nil || tkA2.Credential == "" || tkA2.QueuePosition != 0 {
		t.Fatalf("reconnect bypass: %+v err=%v", tkA2, err)
	}
}

// TestAdmissionWindowExpiry: the queue head must re-request within the
// 60 s admission window; a stale head is evicted and the next requester
// takes its place.
func TestAdmissionWindowExpiry(t *testing.T) {
	reg, store, clk := newTestRegistry(t, 1)
	a := seedAccount(t, store)
	b := seedAccount(t, store)
	c := seedAccount(t, store)
	ctx := testCtx()
	plat := protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS

	tkA, _ := reg.IssueTicket(ctx, a, id.UUID{}, 1, plat, 0, "")
	if tkA.Credential == "" {
		t.Fatal("setup: A must hold the slot")
	}
	if _, err := reg.IssueTicket(ctx, b, id.UUID{}, 1, plat, 0, ""); err != nil {
		t.Fatalf("B queue: %v", err)
	}
	// B never re-requests. After the window lapses C takes the head.
	clk.Advance(61 * time.Second)
	tkC, err := reg.IssueTicket(ctx, c, id.UUID{}, 1, plat, 0, "")
	if err != nil || tkC.Credential != "" || tkC.QueuePosition != 1 {
		t.Fatalf("C head after B stale: %+v err=%v", tkC, err)
	}
	// B re-requesting now queues behind C.
	tkB2, err := reg.IssueTicket(ctx, b, id.UUID{}, 1, plat, 0, "")
	if err != nil || tkB2.Credential != "" || tkB2.QueuePosition != 2 {
		t.Fatalf("B after eviction: %+v", tkB2)
	}
}
