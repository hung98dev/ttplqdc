package session

import (
	"errors"
	"testing"
	"time"

	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"

	"thinhthan/internal/durable/account"
	"thinhthan/internal/edge/listener"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestSupersedingTicketReattachesLiveCharacter: a fresh ticket+HELLO on
// the same account supersedes the live conn and hands the attached
// character straight to the new session (resumed_character_id set).
func TestSupersedingTicketReattachesLiveCharacter(t *testing.T) {
	reg, store, _ := newTestRegistry(t, 4)
	acct := seedAccount(t, store)
	charID := seedCharacter(t, store, acct, "hero.takeover")
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
	wsEnv(t, c1, 6, ok1.SessionEpoch, 2,
		&protocolv1.C2SCharacterAttach{CharacterId: charID[:]})
	envA := wsReadUntil(t, c1, 7)
	var aok protocolv1.S2CCharacterAttachOk
	if err := proto.Unmarshal(envA.Payload, &aok); err != nil {
		t.Fatalf("attach_ok: %v", err)
	}

	// Second login: supersede — old conn gets id 8, new session resumes
	// the character with no CHARACTER_LIST.
	tk2, _ := reg.IssueTicket(ctx, acct, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 0, "")
	c2 := wsDial(t, addr)
	env2 := wsHello(t, c2, helloTicket(tk2.Credential, 1))
	var ok2 protocolv1.S2CHelloOk
	if err := proto.Unmarshal(env2.Payload, &ok2); err != nil {
		t.Fatalf("hello_ok 2: %v", err)
	}
	if len(ok2.ResumedCharacterId) != 16 {
		t.Fatal("live character not re-attached")
	}
	env8 := wsReadUntil(t, c1, 8)
	var rep protocolv1.S2CSessionReplaced
	if err := proto.Unmarshal(env8.Payload, &rep); err != nil {
		t.Fatalf("replaced: %v", err)
	}
	if rep.Reason != protocolv1.SessionReplacedReason_SESSION_REPLACED_REASON_NEWER_SESSION {
		t.Fatalf("reason %v", rep.Reason)
	}
	_ = c2
}

// TestResumeCredentialRotationEvery300s: the registry emits
// S2C_RESUME_CREDENTIAL (id 16) each rotation interval on a bound conn.
func TestResumeCredentialRotationEvery300s(t *testing.T) {
	reg, store, clk := newTestRegistry(t, 4)
	acct := seedAccount(t, store)
	addr := startListener(t, reg)
	ctx := testCtx()

	tk, _ := reg.IssueTicket(ctx, acct, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 0, "")
	c := wsDial(t, addr)
	env := wsHello(t, c, helloTicket(tk.Credential, 1))
	var ok protocolv1.S2CHelloOk
	if err := proto.Unmarshal(env.Payload, &ok); err != nil {
		t.Fatalf("hello_ok: %v", err)
	}
	// Bind the conn (any inbound intent). The bogus char id is rejected
	// with NOT_OWNER — its id-3 reply doubles as the read barrier proving
	// the inbound was delivered and the conn bound.
	wsEnv(t, c, 6, ok.SessionEpoch, 2,
		&protocolv1.C2SCharacterAttach{CharacterId: make([]byte, 16)})
	wsReadUntil(t, c, 3)
	clk.Advance(ResumeRotateEvery)
	env16 := wsReadUntil(t, c, 16)
	var rc protocolv1.S2CResumeCredential
	if err := proto.Unmarshal(env16.Payload, &rc); err != nil {
		t.Fatalf("resume cred: %v", err)
	}
	if rc.ResumeCredential == "" || rc.ResumeCredential == ok.ResumeCredential {
		t.Fatalf("no rotation: %q", rc.ResumeCredential)
	}
}

// TestPredecessorCredentialInvalidAfterNewestUsed: presenting the newest
// resume credential invalidates its predecessor.
func TestPredecessorCredentialInvalidAfterNewestUsed(t *testing.T) {
	reg, store, clk := newTestRegistry(t, 4)
	acct := seedAccount(t, store)
	addr := startListener(t, reg)
	ctx := testCtx()

	tk, _ := reg.IssueTicket(ctx, acct, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 0, "")
	c := wsDial(t, addr)
	env := wsHello(t, c, helloTicket(tk.Credential, 1))
	var ok protocolv1.S2CHelloOk
	if err := proto.Unmarshal(env.Payload, &ok); err != nil {
		t.Fatalf("hello_ok: %v", err)
	}
	// Bind + rotate once → cred2 current, ok's cred predecessor.
	wsEnv(t, c, 6, ok.SessionEpoch, 2,
		&protocolv1.C2SCharacterAttach{CharacterId: make([]byte, 16)})
	wsReadUntil(t, c, 3)
	clk.Advance(ResumeRotateEvery)
	env16 := wsReadUntil(t, c, 16)
	var rc protocolv1.S2CResumeCredential
	if err := proto.Unmarshal(env16.Payload, &rc); err != nil {
		t.Fatalf("rotation: %v", err)
	}
	prev := ok.ResumeCredential
	newest := rc.ResumeCredential

	// Present the newest → predecessor invalidated.
	c2 := wsDial(t, addr)
	env2 := wsHello(t, c2, helloResume(newest, 1))
	if env2.MessageId != 2 {
		t.Fatalf("newest resume rejected: id %d", env2.MessageId)
	}
	// Present the predecessor → AUTH_INVALID.
	c3 := wsDial(t, addr)
	env3 := wsHello(t, c3, helloResume(prev, 1))
	var errMsg protocolv1.S2CError
	if env3.MessageId != 3 {
		t.Fatalf("predecessor accepted: id %d", env3.MessageId)
	}
	if err := proto.Unmarshal(env3.Payload, &errMsg); err != nil {
		t.Fatalf("err payload: %v", err)
	}
	if errMsg.ErrorCode != protocolv1.ErrorCode_ERROR_CODE_AUTH_INVALID {
		t.Fatalf("predecessor reject code: %v", errMsg.ErrorCode)
	}
}

// TestTicketBypassesQueueDuringGrace: inside reconnect grace the account
// bypasses a saturated queue on the ticket path.
func TestTicketBypassesQueueDuringGrace(t *testing.T) {
	reg, store, _ := newTestRegistry(t, 1)
	acct := seedAccount(t, store)
	other := seedAccount(t, store)
	charID := seedCharacter(t, store, acct, "hero.grace")
	addr := startListener(t, reg)
	ctx := testCtx()
	plat := protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS

	tk, _ := reg.IssueTicket(ctx, acct, 1, plat, 0, "")
	c1 := wsDial(t, addr)
	env := wsHello(t, c1, helloTicket(tk.Credential, 1))
	var ok protocolv1.S2CHelloOk
	if err := proto.Unmarshal(env.Payload, &ok); err != nil {
		t.Fatalf("hello_ok: %v", err)
	}
	wsAttach(t, c1, ok.SessionEpoch, 2, charID)
	wsReadUntil(t, c1, 7)

	// Drop the conn: session enters grace, slot still held → others queue.
	c1.Close(websocket.StatusAbnormalClosure, "lost")
	deadline := time.Now().Add(3 * time.Second)
	for {
		reg.mu.Lock()
		s := reg.byAccount[acct]
		grace := s != nil && s.graceTimer != nil
		reg.mu.Unlock()
		if grace {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("grace never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	tkO, _ := reg.IssueTicket(ctx, other, 1, plat, 0, "")
	if tkO.Credential != "" {
		t.Fatal("queued account unexpectedly admitted")
	}
	// The grace account bypasses the queue on the ticket path.
	tk2, err := reg.IssueTicket(ctx, acct, 1, plat, 0, "")
	if err != nil || tk2.Credential == "" || tk2.QueuePosition != 0 {
		t.Fatalf("grace bypass: %+v err=%v", tk2, err)
	}
}

// TestAttachNeverServerOverloaded: a session holding a slot always
// attaches, however deep the login queue behind it.
func TestAttachNeverServerOverloaded(t *testing.T) {
	reg, store, _ := newTestRegistry(t, 1)
	acct := seedAccount(t, store)
	queued := seedAccount(t, store)
	charID := seedCharacter(t, store, acct, "hero.attach")
	ctx := testCtx()
	plat := protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS

	tk, _ := reg.IssueTicket(ctx, acct, 1, plat, 0, "")
	d := reg.Hello(ctx, listener.HelloMeta{}, helloTicket(tk.Credential, 1))
	if d.OK == nil {
		t.Fatalf("hello: %+v", d.Reject)
	}
	// Saturate: another account waits on the (full) capacity.
	tkQ, _ := reg.IssueTicket(ctx, queued, 1, plat, 0, "")
	if tkQ.Credential != "" {
		t.Fatal("setup: queue not saturated")
	}
	// Attach ignores the queue entirely.
	if err := reg.Enqueue(ctx, nil, listener.Inbound{
		MessageID: 6, SessionEpoch: d.OK.SessionEpoch, ClientSeq: 2,
		Payload: &protocolv1.C2SCharacterAttach{CharacterId: charID[:]},
	}); err != nil {
		t.Fatalf("attach under load: %v", err)
	}
	reg.mu.Lock()
	s := reg.sessions[d.OK.SessionEpoch]
	attached := s != nil && s.charID != nil
	reg.mu.Unlock()
	if !attached {
		t.Fatal("attach not recorded")
	}
}

// TestPhaseLegalitySilentDrop: inbound frames that must be consumed
// without reply are: stale-epoch frames (superseded session), and
// non-durable ids on a live session — both return nil and mutate
// nothing.
func TestPhaseLegalitySilentDrop(t *testing.T) {
	reg, store, _ := newTestRegistry(t, 4)
	acct := seedAccount(t, store)
	ctx := testCtx()

	tk, _ := reg.IssueTicket(ctx, acct, 1,
		protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS, 0, "")
	d := reg.Hello(ctx, listener.HelloMeta{}, helloTicket(tk.Credential, 1))
	// Stale epoch → silent drop.
	if err := reg.Enqueue(ctx, nil, listener.Inbound{
		MessageID: 6, SessionEpoch: 999999, ClientSeq: 2,
		Payload: &protocolv1.C2SCharacterAttach{CharacterId: make([]byte, 16)},
	}); err != nil {
		t.Fatalf("stale epoch: %v", err)
	}
	// Non-durable id on a live epoch → silent consume.
	if err := reg.Enqueue(ctx, nil, listener.Inbound{
		MessageID: 9999, SessionEpoch: d.OK.SessionEpoch, ClientSeq: 2,
		Payload: &protocolv1.C2SCharacterDetach{},
	}); err != nil {
		t.Fatalf("non-durable id: %v", err)
	}
	reg.mu.Lock()
	s := reg.sessions[d.OK.SessionEpoch]
	alive := s != nil && s.charID == nil
	reg.mu.Unlock()
	if !alive {
		t.Fatal("silent drops mutated the session")
	}
}

// TestDetachRejections: DETACH without an attached character is
// MESSAGE_NOT_ALLOWED_IN_STATE; attach on a PENDING_DELETION account is
// ACCOUNT_PENDING_DELETION; attach of another account's character is
// NOT_OWNER.
func TestDetachRejections(t *testing.T) {
	reg, store, _ := newTestRegistry(t, 4)
	acct := seedAccount(t, store)
	other := seedAccount(t, store)
	charForeign := seedCharacter(t, store, other, "hero.foreign")
	ctx := testCtx()
	plat := protocolv1.ClientPlatform_CLIENT_PLATFORM_WINDOWS

	tk, _ := reg.IssueTicket(ctx, acct, 1, plat, 0, "")
	d := reg.Hello(ctx, listener.HelloMeta{}, helloTicket(tk.Credential, 1))
	epoch := d.OK.SessionEpoch
	var pe *protoError

	// Detach with nothing attached.
	err := reg.Enqueue(ctx, nil, listener.Inbound{
		MessageID: 10, SessionEpoch: epoch, ClientSeq: 2,
		Payload: &protocolv1.C2SCharacterDetach{},
	})
	if !errors.As(err, &pe) || pe.code != protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE {
		t.Fatalf("empty detach: %v", err)
	}
	// Attach of a character owned by another account.
	err = reg.Enqueue(ctx, nil, listener.Inbound{
		MessageID: 6, SessionEpoch: epoch, ClientSeq: 3,
		Payload: &protocolv1.C2SCharacterAttach{CharacterId: charForeign[:]},
	})
	if !errors.As(err, &pe) || pe.code != protocolv1.ErrorCode_ERROR_CODE_NOT_OWNER {
		t.Fatalf("foreign attach: %v", err)
	}

	// PENDING_DELETION account cannot attach at all.
	acct2 := seedAccount(t, store)
	char2 := seedCharacter(t, store, acct2, "hero.doomed")
	if err := store.SetStatus(ctx, nil, acct2, account.StatusPendingDeletion); err != nil {
		t.Fatalf("status: %v", err)
	}
	tk2, _ := reg.IssueTicket(ctx, acct2, 1, plat, 0, "")
	d2 := reg.Hello(ctx, listener.HelloMeta{}, helloTicket(tk2.Credential, 1))
	if d2.OK == nil {
		t.Fatalf("pending-deletion hello: %+v", d2.Reject)
	}
	err = reg.Enqueue(ctx, nil, listener.Inbound{
		MessageID: 6, SessionEpoch: d2.OK.SessionEpoch, ClientSeq: 2,
		Payload: &protocolv1.C2SCharacterAttach{CharacterId: char2[:]},
	})
	if !errors.As(err, &pe) || pe.code != protocolv1.ErrorCode_ERROR_CODE_ACCOUNT_PENDING_DELETION {
		t.Fatalf("pending-deletion attach: %v", err)
	}
}
