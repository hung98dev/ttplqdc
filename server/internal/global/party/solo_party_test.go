package party

import (
	"testing"
	"time"

	"thinhthan/internal/core/id"
	v1 "thinhthan/internal/protocol/v1"
)

// ADR-0062: a partyless invite creates a one-member ACTIVE party that
// persists after the invite declines/expires/cancels until the leader
// leaves.
func TestSoloPartyPersistsAfterInviteEnds(t *testing.T) {
	online := map[id.UUID]bool{}

	newSvc := func() *Service {
		return New(Options{
			Now:    func() time.Time { return t0 },
			Online: func(c id.UUID) bool { return online[c] },
		})
	}

	t.Run("declined", func(t *testing.T) {
		svc := newSvc()
		inviter, target := id.NewV4(), id.NewV4()
		online[target] = true
		svc.Invite(op(), inviter, target)
		p := svc.Party(inviter)
		if r := svc.Decline(op(), target, p.ID, inviter); r.Status != v1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("decline: %v", r.ErrorCode)
		}
		if svc.Party(inviter) == nil || !p.Active || len(p.Members) != 1 {
			t.Fatal("solo party must persist after decline")
		}
	})

	t.Run("expired", func(t *testing.T) {
		clock := t0
		svc := New(Options{
			Now:    func() time.Time { return clock },
			Online: func(c id.UUID) bool { return online[c] },
		})
		inviter, target := id.NewV4(), id.NewV4()
		online[target] = true
		svc.Invite(op(), inviter, target)
		svc.Sweep(clock.Add(InviteTTL + time.Second))
		p := svc.Party(inviter)
		if p == nil || !p.Active || len(p.Members) != 1 || p.Leader != inviter {
			t.Fatal("solo party must persist after expiry")
		}
	})

	t.Run("cancelled", func(t *testing.T) {
		svc := newSvc()
		inviter, target := id.NewV4(), id.NewV4()
		online[target] = true
		svc.Invite(op(), inviter, target)
		if r := svc.CancelInvite(op(), inviter, target); r.Status != v1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("cancel: %v", r.ErrorCode)
		}
		p := svc.Party(inviter)
		if p == nil || !p.Active || len(p.Members) != 1 {
			t.Fatal("solo party must persist after cancel")
		}
		// ... until the leader leaves.
		if r := svc.Leave(op(), inviter); r.Status != v1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("leave: %v", r.ErrorCode)
		}
		if svc.Party(inviter) != nil {
			t.Fatal("final leave must disband")
		}
	})
}
