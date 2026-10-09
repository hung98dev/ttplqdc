package party

import (
	"testing"
	"time"

	"thinhthan/internal/core/id"
	v1 "thinhthan/internal/protocol/v1"
)

// ADR-0064: every party request (602, 604..606, 620..622, 634, 635)
// yields exactly one S2C_PARTY_RESULT with operation_id, status,
// error_code, party_id.
func TestPartyResultPerRequest(t *testing.T) {
	online := map[id.UUID]bool{}
	t1, t2 := id.NewV4(), id.NewV4()
	online[t1], online[t2] = true, true
	svc := New(Options{
		Now:          func() time.Time { return t0 },
		Online:       func(c id.UUID) bool { return online[c] },
		InSafeAnchor: func(id.UUID) bool { return true },
	})
	leader := id.NewV4()

	type call struct {
		req uint32
		run func(id.UUID) Result
	}
	calls := []call{
		{ReqInvite, func(op id.UUID) Result { return svc.Invite(op, leader, t1) }},
		{ReqAccept, func(op id.UUID) Result {
			return svc.Accept(op, t1, svc.Party(leader).ID, leader)
		}},
		{ReqInvite, func(op id.UUID) Result { return svc.Invite(op, leader, t2) }},
		{ReqDecline, func(op id.UUID) Result {
			return svc.Decline(op, t2, svc.Party(leader).ID, leader)
		}},
		{ReqInvite, func(op id.UUID) Result { return svc.Invite(op, leader, t2) }},
		{ReqInviteCancel, func(op id.UUID) Result { return svc.CancelInvite(op, leader, t2) }},
		{ReqInvite, func(op id.UUID) Result { return svc.Invite(op, leader, t2) }},
		{ReqAccept, func(op id.UUID) Result {
			return svc.Accept(op, t2, svc.Party(leader).ID, leader)
		}},
		{ReqLeaderTransfer, func(op id.UUID) Result {
			return svc.TransferLeader(op, leader, t1)
		}},
		{ReqLeaderTransfer, func(op id.UUID) Result {
			return svc.TransferLeader(op, t1, leader)
		}},
		{ReqKick, func(op id.UUID) Result { return svc.Kick(op, leader, t2) }},
		{ReqBoardPost, func(op id.UUID) Result {
			return svc.BoardPost(op, leader, "dungeon.x", 3, "")
		}},
		{ReqBoardCancel, func(op id.UUID) Result { return svc.BoardCancel(op, leader) }},
		{ReqLeave, func(op id.UUID) Result { return svc.Leave(op, t1) }},
	}
	for i, c := range calls {
		opID := id.NewV4()
		res := c.run(opID)
		if res.RequestMessageID != c.req {
			t.Fatalf("call %d: request_message_id %d", i, res.RequestMessageID)
		}
		if res.OperationID != opID {
			t.Fatalf("call %d: operation_id dropped", i)
		}
		if res.Status != v1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("call %d: %v", i, res.ErrorCode)
		}
	}
}

func TestPartyResultErrorCodes(t *testing.T) {
	online := map[id.UUID]bool{}
	svc := New(Options{
		Now:    func() time.Time { return t0 },
		Online: func(c id.UUID) bool { return online[c] },
	})
	leader, member, stranger := id.NewV4(), id.NewV4(), id.NewV4()
	online[member] = true
	svc.Invite(op(), leader, member)
	svc.Accept(op(), member, svc.Party(leader).ID, leader)

	// Not-leader invite.
	if r := svc.Invite(op(), member, id.NewV4()); r.ErrorCode != v1.ErrorCode_ERROR_CODE_PERMISSION_DENIED {
		t.Fatalf("non-leader invite: %v", r.ErrorCode)
	}
	// Invite someone already in a party.
	if r := svc.Invite(op(), leader, member); r.ErrorCode != v1.ErrorCode_ERROR_CODE_TARGET_INVALID {
		t.Fatalf("in-party target: %v", r.ErrorCode)
	}
	// Offline target.
	if r := svc.Invite(op(), leader, id.NewV4()); r.ErrorCode != v1.ErrorCode_ERROR_CODE_TARGET_INVALID {
		t.Fatalf("offline target: %v", r.ErrorCode)
	}
	// Blocked target.
	svcBlocked := New(Options{
		Now:     func() time.Time { return t0 },
		Online:  func(c id.UUID) bool { return online[c] },
		Blocked: func(a, b id.UUID) bool { return b == stranger },
	})
	if r := svcBlocked.Invite(op(), leader, stranger); r.ErrorCode != v1.ErrorCode_ERROR_CODE_TARGET_BLOCKED {
		t.Fatalf("blocked: %v", r.ErrorCode)
	}
	// Accept with nothing pending.
	if r := svc.Accept(op(), stranger, id.NewV4(), leader); r.ErrorCode != v1.ErrorCode_ERROR_CODE_TARGET_INVALID {
		t.Fatalf("no invite: %v", r.ErrorCode)
	}
	// Leave while partyless.
	if r := svc.Leave(op(), stranger); r.ErrorCode != v1.ErrorCode_ERROR_CODE_TARGET_INVALID {
		t.Fatalf("partyless leave: %v", r.ErrorCode)
	}
	// Board post outside safe anchor.
	if r := svc.BoardPost(op(), leader, "dungeon.x", 3, ""); r.ErrorCode != v1.ErrorCode_ERROR_CODE_NOT_IN_SAFE_ANCHOR {
		t.Fatalf("no anchor: %v", r.ErrorCode)
	}
	// Result proto carries request id + status.
	res := svc.Leave(op(), stranger).Proto()
	if res.Result.Status != v1.ResultStatus_RESULT_STATUS_ERROR ||
		res.RequestMessageId != ReqLeave {
		t.Fatalf("proto %+v", res)
	}
}
