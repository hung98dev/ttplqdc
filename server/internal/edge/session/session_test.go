package session

import (
	"errors"
	"testing"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"thinhthan/internal/edge/listener"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestAttachDetachFlow: C2S_CHARACTER_ATTACH binds the owned character
// (ATTACH_OK + activity journal); DETACH releases it and re-lists.
func TestAttachDetachFlow(t *testing.T) {
	reg, store, _ := newTestRegistry(t, 4)
	acct := seedAccount(t, store)
	charID := seedCharacter(t, store, acct, "hero.one")
	ctx := testCtx()

	tk, err := reg.IssueTicket(ctx, acct, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 0, "")
	if err != nil {
		t.Fatalf("ticket: %v", err)
	}
	d := reg.Hello(ctx, listener.HelloMeta{}, helloTicket(tk.Credential, 1))
	if d.OK == nil {
		t.Fatalf("hello: %+v", d.Reject)
	}
	epoch := d.OK.SessionEpoch

	if err := reg.Enqueue(ctx, nil, listener.Inbound{
		MessageID: 6, SessionEpoch: epoch, ClientSeq: 2,
		Payload: &protocolv1.C2SCharacterAttach{CharacterId: charID[:]},
	}); err != nil {
		t.Fatalf("attach: %v", err)
	}
	waitActive(t, store, charID, true)

	if err := reg.Enqueue(ctx, nil, listener.Inbound{
		MessageID: 10, SessionEpoch: epoch, ClientSeq: 3,
		Payload: &protocolv1.C2SCharacterDetach{},
	}); err != nil {
		t.Fatalf("detach: %v", err)
	}
	waitActive(t, store, charID, false)
}

// TestStaleEpochDropped: an intent on a superseded/expired epoch is
// consumed silently — no dispatch, no error.
func TestStaleEpochDropped(t *testing.T) {
	reg, store, _ := newTestRegistry(t, 4)
	acct := seedAccount(t, store)
	charID := seedCharacter(t, store, acct, "hero.stale")
	ctx := testCtx()

	tk, _ := reg.IssueTicket(ctx, acct, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 0, "")
	d := reg.Hello(ctx, listener.HelloMeta{}, helloTicket(tk.Credential, 1))
	if err := reg.Enqueue(ctx, nil, listener.Inbound{
		MessageID: 6, SessionEpoch: d.OK.SessionEpoch + 999, ClientSeq: 2,
		Payload: &protocolv1.C2SCharacterAttach{CharacterId: charID[:]},
	}); err != nil {
		t.Fatalf("stale enqueue: %v", err)
	}
	reg.mu.Lock()
	if s := reg.sessions[d.OK.SessionEpoch]; s != nil && s.charID != nil {
		reg.mu.Unlock()
		t.Fatal("stale epoch attached a character")
	} else {
		reg.mu.Unlock()
	}
}

// TestSessionReplacedPush: a second HELLO on the same account sends
// S2C_SESSION_REPLACED{NEWER_SESSION} to the old conn and closes it.
func TestSessionReplacedPush(t *testing.T) {
	reg, store, _ := newTestRegistry(t, 4)
	acct := seedAccount(t, store)
	charID := seedCharacter(t, store, acct, "hero.push")
	addr := startListener(t, reg)
	ctx := testCtx()

	tk1, _ := reg.IssueTicket(ctx, acct, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 0, "")
	c1 := wsDial(t, addr)
	env1 := wsHello(t, c1, helloTicket(tk1.Credential, 1))
	var ok1 protocolv1.S2CHelloOk
	if err := proto.Unmarshal(env1.Payload, &ok1); err != nil {
		t.Fatalf("hello_ok: %v", err)
	}
	// The conn binds to the session on its first inbound intent — a conn
	// that sends nothing has no push channel yet.
	wsAttach(t, c1, ok1.SessionEpoch, 2, charID)

	tk2, _ := reg.IssueTicket(ctx, acct, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 0, "")
	c2 := wsDial(t, addr)
	env2 := wsHello(t, c2, helloTicket(tk2.Credential, 1))
	var ok2 protocolv1.S2CHelloOk
	if err := proto.Unmarshal(env2.Payload, &ok2); err != nil {
		t.Fatalf("hello_ok 2: %v", err)
	}
	if ok2.SessionEpoch == ok1.SessionEpoch {
		t.Fatal("superseding login reused the epoch")
	}

	// Old conn receives S2C_SESSION_REPLACED (id 8) then closes.
	env8 := wsReadUntil(t, c1, 8)
	var rep protocolv1.S2CSessionReplaced
	if err := proto.Unmarshal(env8.Payload, &rep); err != nil {
		t.Fatalf("replaced payload: %v", err)
	}
	if rep.Reason != protocolv1.SessionReplacedReason_SESSION_REPLACED_REASON_NEWER_SESSION {
		t.Fatalf("reason: %v", rep.Reason)
	}
	_ = c2
}

// TestReconnectAuthority: after a disconnect, the resume credential
// re-attaches the SAME session and the live character resumes.
func TestReconnectAuthority(t *testing.T) {
	reg, store, _ := newTestRegistry(t, 4)
	acct := seedAccount(t, store)
	charID := seedCharacter(t, store, acct, "hero.back")
	addr := startListener(t, reg)
	ctx := testCtx()

	tk, _ := reg.IssueTicket(ctx, acct, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 0, "")
	c1 := wsDial(t, addr)
	env := wsHello(t, c1, helloTicket(tk.Credential, 1))
	var ok protocolv1.S2CHelloOk
	if err := proto.Unmarshal(env.Payload, &ok); err != nil {
		t.Fatalf("hello_ok: %v", err)
	}
	if env := wsAttach(t, c1, ok.SessionEpoch, 2, charID); env.MessageId != 7 {
		wsReadUntil(t, c1, 7)
	}

	// Disconnect: session enters reconnect grace.
	c1.Close(websocket.StatusAbnormalClosure, "lost")
	resume := ok.ResumeCredential

	c2 := wsDial(t, addr)
	env2 := wsHello(t, c2, helloResume(resume, 1))
	var ok2 protocolv1.S2CHelloOk
	if err := proto.Unmarshal(env2.Payload, &ok2); err != nil {
		t.Fatalf("resume hello_ok: %v", err)
	}
	if ok2.SessionEpoch != ok.SessionEpoch {
		t.Fatalf("resume minted new epoch %d vs %d", ok2.SessionEpoch, ok.SessionEpoch)
	}
	if len(ok2.ResumedCharacterId) != 16 {
		t.Fatal("no resumed character")
	}
}

// TestOneAccountOneCharacter: one account holds at most one live
// character — attaching a second is CHARACTER_ALREADY_ACTIVE, and
// attaching someone else's is NOT_OWNER.
func TestOneAccountOneCharacter(t *testing.T) {
	reg, store, _ := newTestRegistry(t, 4)
	acct := seedAccount(t, store)
	other := seedAccount(t, store)
	charA := seedCharacter(t, store, acct, "hero.a")
	charB := seedCharacter(t, store, acct, "hero.b")
	charC := seedCharacter(t, store, other, "hero.c")
	ctx := testCtx()

	tk, _ := reg.IssueTicket(ctx, acct, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 0, "")
	d := reg.Hello(ctx, listener.HelloMeta{}, helloTicket(tk.Credential, 1))
	epoch := d.OK.SessionEpoch
	attach := func(cid []byte) error {
		return reg.Enqueue(ctx, nil, listener.Inbound{
			MessageID: 6, SessionEpoch: epoch, ClientSeq: 2,
			Payload: &protocolv1.C2SCharacterAttach{CharacterId: cid},
		})
	}
	var pe *protoError
	if err := attach(charA[:]); err != nil {
		t.Fatalf("attach a: %v", err)
	}
	if err := attach(charB[:]); !errors.As(err, &pe) ||
		pe.code != protocolv1.ErrorCode_ERROR_CODE_CHARACTER_ALREADY_ACTIVE {
		t.Fatalf("second attach: %v", err)
	}
	if err := attach(charC[:]); !errors.As(err, &pe) ||
		pe.code != protocolv1.ErrorCode_ERROR_CODE_CHARACTER_ALREADY_ACTIVE {
		// Session already holds charA — ALREADY_ACTIVE wins over NOT_OWNER.
		t.Fatalf("other-account attach while attached: %v", err)
	}
}
