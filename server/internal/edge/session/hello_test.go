package session

import (
	"testing"
	"time"

	"thinhthan/internal/edge/listener"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestHelloTicketSingleUse: a gameplay ticket redeems exactly once —
// presenting it again is AUTH_INVALID.
func TestHelloTicketSingleUse(t *testing.T) {
	reg, store, _ := newTestRegistry(t, 4)
	acct := seedAccount(t, store)
	ctx := testCtx()

	tk, err := reg.IssueTicket(ctx, acct, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 0, "")
	if err != nil {
		t.Fatalf("ticket: %v", err)
	}
	d1 := reg.Hello(ctx, listener.HelloMeta{}, helloTicket(tk.Credential, 1))
	if d1.OK == nil {
		t.Fatalf("hello: %+v", d1.Reject)
	}
	if d1.OK.SessionEpoch == 0 || d1.OK.ResumeCredential == "" {
		t.Fatalf("hello_ok shape: %+v", d1.OK)
	}
	d2 := reg.Hello(ctx, listener.HelloMeta{}, helloTicket(tk.Credential, 1))
	if d2.Reject == nil || d2.Reject.Code != protocolv1.ErrorCode_ERROR_CODE_AUTH_INVALID {
		t.Fatalf("reused ticket: %+v", d2)
	}
	// Expired ticket → AUTH_EXPIRED.
	reg2, store2, clk := newTestRegistry(t, 4)
	acct2 := seedAccount(t, store2)
	tk2, err := reg2.IssueTicket(ctx, acct2, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 0, "")
	if err != nil {
		t.Fatalf("ticket2: %v", err)
	}
	clk.Advance(11 * time.Minute)
	d3 := reg2.Hello(ctx, listener.HelloMeta{}, helloTicket(tk2.Credential, 1))
	if d3.Reject == nil || d3.Reject.Code != protocolv1.ErrorCode_ERROR_CODE_AUTH_EXPIRED {
		t.Fatalf("expired ticket: %+v", d3)
	}
}

// TestResumeCredentialRotates: presenting the issued resume credential
// re-attaches the same session and mints a fresh credential.
func TestResumeCredentialRotates(t *testing.T) {
	reg, store, _ := newTestRegistry(t, 4)
	acct := seedAccount(t, store)
	ctx := testCtx()

	tk, err := reg.IssueTicket(ctx, acct, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 0, "")
	if err != nil {
		t.Fatalf("ticket: %v", err)
	}
	d1 := reg.Hello(ctx, listener.HelloMeta{}, helloTicket(tk.Credential, 1))
	if d1.OK == nil {
		t.Fatalf("hello: %+v", d1.Reject)
	}
	d2 := reg.Hello(ctx, listener.HelloMeta{}, helloResume(d1.OK.ResumeCredential, 1))
	if d2.OK == nil {
		t.Fatalf("resume hello: %+v", d2.Reject)
	}
	if d2.OK.SessionEpoch != d1.OK.SessionEpoch {
		t.Fatalf("resume minted a new epoch: %d vs %d",
			d2.OK.SessionEpoch, d1.OK.SessionEpoch)
	}
	if d2.OK.ResumeCredential == d1.OK.ResumeCredential {
		t.Fatal("resume credential did not rotate")
	}
}

// TestHelloDeadline: a connection that never sends C2S_HELLO inside the
// handshake window is closed by the listener.
func TestHelloDeadline(t *testing.T) {
	reg, _, _ := newTestRegistry(t, 4)
	addr := startListenerWindow(t, reg, 100*time.Millisecond)
	c := wsDial(t, addr)
	defer c.Close(1000, "done")
	time.Sleep(400 * time.Millisecond)
	// The listener emits an id-3 deadline error frame, then closes; keep
	// reading until the connection actually goes down.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if env := wsReadOrErr(t, c); env == nil {
			return
		}
	}
	t.Fatal("connection still open past deadline")

}
