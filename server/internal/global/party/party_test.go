package party

import (
	"testing"
	"time"

	"thinhthan/internal/core/id"
	v1 "thinhthan/internal/protocol/v1"
)

var t0 = time.Unix(1_700_000_000, 0)

func op() id.UUID { return id.NewV4() }

func TestMaxFivePartyMembership(t *testing.T) {
	online := map[id.UUID]bool{}
	svc := New(Options{
		Now:    func() time.Time { return t0 },
		Online: func(c id.UUID) bool { return online[c] },
	})
	leader := id.NewV4()
	for i := 0; i < MaxMembers-1; i++ {
		target := id.NewV4()
		online[target] = true
		if r := svc.Invite(op(), leader, target); r.Status != v1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("invite %d: %v", i, r.ErrorCode)
		}
		p := svc.Party(leader)
		if r := svc.Accept(op(), target, p.ID, leader); r.Status != v1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("accept %d: %v", i, r.ErrorCode)
		}
	}
	p := svc.Party(leader)
	if len(p.Members) != MaxMembers {
		t.Fatalf("members %d", len(p.Members))
	}
	sixth := id.NewV4()
	online[sixth] = true
	if r := svc.Invite(op(), leader, sixth); r.Status != v1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("invite sixth: %v", r.ErrorCode)
	}
	if r := svc.Accept(op(), sixth, p.ID, leader); r.ErrorCode != v1.ErrorCode_ERROR_CODE_CAPACITY_FULL {
		t.Fatalf("sixth accept: %v", r.ErrorCode)
	}
	if len(svc.Party(leader).Members) != MaxMembers {
		t.Fatal("capacity exceeded")
	}
}

func TestLeaderPromotionAndTransfer(t *testing.T) {
	online := map[id.UUID]bool{}
	svc := New(Options{
		Now:    func() time.Time { return t0 },
		Online: func(c id.UUID) bool { return online[c] },
	})
	leader, m1, m2 := id.NewV4(), id.NewV4(), id.NewV4()
	online[m1], online[m2] = true, true
	for _, target := range []id.UUID{m1, m2} {
		if r := svc.Invite(op(), leader, target); r.Status != v1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("invite: %v", r.ErrorCode)
		}
		if r := svc.Accept(op(), target, svc.Party(leader).ID, leader); r.Status != v1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("accept: %v", r.ErrorCode)
		}
	}
	p := svc.Party(leader)
	if r := svc.TransferLeader(op(), leader, m2); r.Status != v1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("transfer: %v", r.ErrorCode)
	}
	if p.Leader != m2 {
		t.Fatalf("leader %v", p.Leader)
	}
	// Non-leader transfer rejected.
	if r := svc.TransferLeader(op(), leader, m1); r.ErrorCode != v1.ErrorCode_ERROR_CODE_PERMISSION_DENIED {
		t.Fatalf("non-leader transfer: %v", r.ErrorCode)
	}
	// New leader leaves -> lowest join_sequence re-inherits (seq 1).
	if r := svc.Leave(op(), m2); r.Status != v1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("leave: %v", r.ErrorCode)
	}
	if p.Leader != leader {
		t.Fatalf("promoted leader %v", p.Leader)
	}
	if r := svc.Leave(op(), leader); r.Status != v1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("leader leave: %v", r.ErrorCode)
	}
	if p.Leader != m1 {
		t.Fatalf("promoted leader %v", p.Leader)
	}
	// Final leave disbands.
	if r := svc.Leave(op(), m1); r.Status != v1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("final leave: %v", r.ErrorCode)
	}
	if svc.Party(m1) != nil {
		t.Fatal("party must disband on final leave")
	}
}

func TestDisconnectTimeoutGracePeriod(t *testing.T) {
	clock := t0
	online := map[id.UUID]bool{}
	svc := New(Options{
		Now:    func() time.Time { return clock },
		Online: func(c id.UUID) bool { return online[c] },
	})
	leader, m1 := id.NewV4(), id.NewV4()
	online[leader], online[m1] = true, true
	svc.Invite(op(), leader, m1)
	p := svc.Party(leader)
	svc.Accept(op(), m1, p.ID, leader)

	svc.Detach(leader)
	online[leader] = false
	// Under grace: no transfer.
	svc.Sweep(clock.Add(LeaderGrace - time.Second))
	if p.Leader != leader {
		t.Fatal("transferred inside grace")
	}
	// Past grace: lowest-seq ONLINE member takes over.
	svc.Sweep(clock.Add(LeaderGrace + time.Second))
	if p.Leader != m1 {
		t.Fatalf("leader after grace %v", p.Leader)
	}
}

func TestInviteWhilePartylessCreatesParty(t *testing.T) {
	online := map[id.UUID]bool{}
	target := id.NewV4()
	online[target] = true
	svc := New(Options{
		Now:    func() time.Time { return t0 },
		Online: func(c id.UUID) bool { return online[c] },
	})
	inviter := id.NewV4()
	r := svc.Invite(op(), inviter, target)
	if r.Status != v1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("invite: %v", r.ErrorCode)
	}
	p := svc.Party(inviter)
	if p == nil || !p.Active || p.Leader != inviter {
		t.Fatalf("party %+v", p)
	}
	if len(p.Members) != 1 || p.Members[0].JoinSequence != 1 {
		t.Fatalf("members %+v", p.Members)
	}
}

// ADR-0060: the partyless invite creates exactly one party and carries
// its party_id in 603.
func TestPartylessInviteCreatesParty(t *testing.T) {
	online := map[id.UUID]bool{}
	target := id.NewV4()
	online[target] = true
	svc := New(Options{
		Now:    func() time.Time { return t0 },
		Online: func(c id.UUID) bool { return online[c] },
	})
	inviter := id.NewV4()
	r := svc.Invite(op(), inviter, target)
	if r.PartyID.IsNil() {
		t.Fatal("result must carry party_id")
	}
	var inviteEv *Event
	stateEv := 0
	for i := range svc.Outbox {
		if svc.Outbox[i].Kind == EventPartyInvite {
			inviteEv = &svc.Outbox[i]
		}
		if svc.Outbox[i].Kind == EventPartyState {
			stateEv++
		}
	}
	if inviteEv == nil || inviteEv.Invite == nil {
		t.Fatal("603 fanout missing")
	}
	if got := inviteEv.Invite.PartyId; len(got) != 16 || id.UUID(got[0:16]) != r.PartyID {
		t.Fatal("603 party_id mismatch")
	}
	if len(inviteEv.Targets) != 1 || inviteEv.Targets[0] != target {
		t.Fatal("603 must address the target only")
	}
	if stateEv == 0 {
		t.Fatal("607 roster push missing")
	}
	// A second invite by the same (now leader) char creates no new party.
	other := id.NewV4()
	online[other] = true
	r2 := svc.Invite(op(), inviter, other)
	if r2.PartyID != r.PartyID {
		t.Fatal("second invite must reuse the existing party")
	}
}
