package social

import (
	"testing"

	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestChatSendResultAlways: every C2S_CHAT_SEND — success or rejected —
// produces exactly one S2C_CHAT_SEND_RESULT (655); the contract embeds
// operation_id, status, error_code, and chat_message_id on SUCCESS.
func TestChatSendResultAlways(t *testing.T) {
	e := newEnv(t)
	acct := e.seedAccount(t)
	a := e.seedCharacter(t, acct, 12)
	e.setActive(t, a, true)
	e.hub.zones[a] = "zone.a"
	c, epoch := e.dial(t, acct)
	attach(t, c, epoch, a)

	send := func(seq uint64, channel protocolv1.ChatChannel, target, text string) *protocolv1.S2CChatSendResult {
		var targetBytes []byte
		if target != "" {
			tid := e.seedCharacter(t, e.seedAccount(t), 12)
			targetBytes = tid[:]
		}
		op := opBytes()
		wsEnv(t, c, 600, epoch, seq, &protocolv1.C2SChatSend{
			OperationId:       op,
			Channel:           channel,
			TargetCharacterId: targetBytes,
			MessageText:       text,
		})
		env := wsReadUntil(t, c, 655)
		res := &protocolv1.S2CChatSendResult{}
		decode(t, env, 655, res)
		if string(res.GetResult().GetOperationId()) != string(op) {
			t.Fatal("operation_id not echoed")
		}
		return res
	}

	ok := send(3, protocolv1.ChatChannel_CHAT_CHANNEL_LOCAL, "", "hi")
	if ok.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS ||
		len(ok.GetChatMessageId()) != 16 {
		t.Fatalf("local send: %v id=%d", ok.GetResult().GetStatus(), len(ok.GetChatMessageId()))
	}
	bad := send(4, protocolv1.ChatChannel_CHAT_CHANNEL_LOCAL, "", "  \x00 ")
	if bad.GetResult().GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_CHAT_TEXT_INVALID {
		t.Fatalf("bad text code=%v", bad.GetResult().GetErrorCode())
	}
	if len(bad.GetChatMessageId()) != 0 {
		t.Fatal("failed send carries message id")
	}
}

// TestSocialResultPerRequest: each social durable request delivers one
// 654 whose operation_id + request_message_id + target echo the input.
func TestSocialResultPerRequest(t *testing.T) {
	e := newEnv(t)
	aAcct, bAcct := e.seedAccount(t), e.seedAccount(t)
	a := e.seedCharacter(t, aAcct, 5)
	b := e.seedCharacter(t, bAcct, 5)
	c, epoch := e.dial(t, aAcct)
	attach(t, c, epoch, a)

	op := opBytes()
	wsEnv(t, c, 611, epoch, 3, &protocolv1.C2SFriendRequest{
		OperationId: op, TargetCharacterId: b[:]})
	env := wsReadUntil(t, c, 654)
	res := &protocolv1.S2CSocialResult{}
	decode(t, env, 654, res)
	if string(res.GetResult().GetOperationId()) != string(op) ||
		res.GetRequestMessageId() != 611 ||
		string(res.GetTargetCharacterId()) != string(b[:]) {
		t.Fatalf("654 shape: %v", res)
	}
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("status=%v code=%v", res.GetResult().GetStatus(), res.GetResult().GetErrorCode())
	}
	// Decline on the target side: second session for b.
	c2, epoch2 := e.dial(t, bAcct)
	attach(t, c2, epoch2, b)
	op2 := opBytes()
	wsEnv(t, c2, 614, epoch2, 3, &protocolv1.C2SFriendDecline{
		OperationId: op2, RequesterCharacterId: a[:]})
	env2 := wsReadUntil(t, c2, 654)
	res2 := &protocolv1.S2CSocialResult{}
	decode(t, env2, 654, res2)
	if res2.GetRequestMessageId() != 614 ||
		res2.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("decline result: %v", res2)
	}
}

// TestFriendStateSnapshotThenDelta: attach-time EmitSocialState pushes
// full_snapshot=true; a subsequent request produces a delta 616
// (full_snapshot=false) carrying the complete request lists.
func TestFriendStateSnapshotThenDelta(t *testing.T) {
	e := newEnv(t)
	aAcct, bAcct := e.seedAccount(t), e.seedAccount(t)
	a := e.seedCharacter(t, aAcct, 5)
	b := e.seedCharacter(t, bAcct, 5)
	e.setActive(t, a, true)
	e.hub.zones[a] = "z"

	// Attach path: EmitSocialState → 616 full snapshot to a.
	if err := e.svc.EmitSocialState(t.Context(), a); err != nil {
		t.Fatal(err)
	}
	snaps := e.hub.seen(a, 616)
	if len(snaps) != 1 {
		t.Fatalf("snapshots=%d", len(snaps))
	}
	full := snaps[0].(*protocolv1.S2CFriendState)
	if !full.GetFullSnapshot() {
		t.Fatal("attach snapshot not full_snapshot")
	}

	// b requests a → delta 616 with the incoming request on a.
	c, epoch := e.dial(t, bAcct)
	attach(t, c, epoch, b)
	wsEnv(t, c, 611, epoch, 3, &protocolv1.C2SFriendRequest{
		OperationId: opBytes(), TargetCharacterId: a[:]})
	if env := wsReadUntil(t, c, 654); env.GetMessageId() != 654 {
		t.Fatal("no 654")
	}
	// The target side sees the incoming-request delta (616
	// full_snapshot=false) carrying the complete request lists; the
	// requester sees the 612 delivery.
	msgs := e.hub.waitSeen(t, a, 616, 2) // snapshot + delta
	var tgtDelta *protocolv1.S2CFriendState
	for _, m := range msgs {
		d := m.(*protocolv1.S2CFriendState)
		if !d.GetFullSnapshot() {
			tgtDelta = d
		}
	}
	if tgtDelta == nil || len(tgtDelta.GetIncomingRequests()) == 0 {
		t.Fatal("no incoming-request delta to target")
	}
	if got := tgtDelta.GetIncomingRequests()[0].GetRequesterCharacterId(); string(got) != string(b[:]) {
		t.Fatal("incoming requester mismatch")
	}
	if push := e.hub.waitSeen(t, a, 612, 1); len(push) != 1 {
		t.Fatalf("612 deliveries=%d", len(push))
	}
}

// TestBlockStateFullSnapshot: block add emits a full 619 REPLACEABLE_STATE
// to the blocker carrying the blocked entry.
func TestBlockStateFullSnapshot(t *testing.T) {
	e := newEnv(t)
	aAcct, bAcct := e.seedAccount(t), e.seedAccount(t)
	a := e.seedCharacter(t, aAcct, 5)
	b := e.seedCharacter(t, bAcct, 5)
	c, epoch := e.dial(t, aAcct)
	attach(t, c, epoch, a)

	wsEnv(t, c, 617, epoch, 3, &protocolv1.C2SBlockAdd{
		OperationId: opBytes(), TargetCharacterId: b[:]})
	env := wsReadUntil(t, c, 654)
	res := &protocolv1.S2CSocialResult{}
	decode(t, env, 654, res)
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("block add: %v", res.GetResult())
	}
	blocks := e.hub.waitSeen(t, a, 619, 1)
	if len(blocks) != 1 {
		t.Fatalf("619 pushes=%d", len(blocks))
	}
	snap := blocks[0].(*protocolv1.S2CBlockState)
	if len(snap.GetBlocked()) != 1 ||
		string(snap.GetBlocked()[0].GetBlockedCharacterId()) != string(b[:]) {
		t.Fatalf("block snapshot: %v", snap)
	}
}

// TestReportResultShape: 632 delivers 633 with operation_id, status,
// error_code, report_id on SUCCESS.
func TestReportResultShape(t *testing.T) {
	e := newEnv(t)
	aAcct, bAcct := e.seedAccount(t), e.seedAccount(t)
	a := e.seedCharacter(t, aAcct, 5)
	b := e.seedCharacter(t, bAcct, 5)
	c, epoch := e.dial(t, aAcct)
	attach(t, c, epoch, a)

	op := opBytes()
	wsEnv(t, c, 632, epoch, 3, &protocolv1.C2SReportPlayer{
		OperationId:       op,
		TargetCharacterId: b[:],
		Reason:            protocolv1.ReportReason_REPORT_REASON_SPAM,
	})
	env := wsReadUntil(t, c, 633)
	res := &protocolv1.S2CReportPlayerResult{}
	decode(t, env, 633, res)
	if string(res.GetResult().GetOperationId()) != string(op) ||
		res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS ||
		len(res.GetReportId()) != 16 {
		t.Fatalf("633 shape: %v", res)
	}
}

// TestChatMessageIdOnWire: a successful send mints one 16-byte
// chat_message_id echoed by both the 601 fanout and the 655 result.
func TestChatMessageIdOnWire(t *testing.T) {
	e := newEnv(t)
	acct := e.seedAccount(t)
	a := e.seedCharacter(t, acct, 12)
	e.setActive(t, a, true)
	e.hub.zones[a] = "zone.a"
	c, epoch := e.dial(t, acct)
	attach(t, c, epoch, a)

	op := opBytes()
	wsEnv(t, c, 600, epoch, 3, &protocolv1.C2SChatSend{
		OperationId: op,
		Channel:     protocolv1.ChatChannel_CHAT_CHANNEL_LOCAL,
		MessageText: "hi",
	})
	env := wsReadUntil(t, c, 655)
	res := &protocolv1.S2CChatSendResult{}
	decode(t, env, 655, res)
	if res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("status=%v", res.GetResult().GetStatus())
	}
	msgs := e.hub.waitSeen(t, a, 601, 1)
	if len(msgs) != 1 {
		t.Fatalf("601 deliveries=%d", len(msgs))
	}
	m := msgs[0].(*protocolv1.S2CChatMessage)
	if string(m.GetChatMessageId()) != string(res.GetChatMessageId()) {
		t.Fatal("601/655 id mismatch")
	}
	if m.GetSenderName() == "" || m.GetMessageText() != "hi" {
		t.Fatalf("601 payload: %v", m)
	}
}
