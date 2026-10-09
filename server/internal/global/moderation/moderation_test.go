package moderation

import (
	"context"
	"testing"
	"time"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// TestAutomatedFilterRejectsNoSanction: the automated classifier may
// reject a send as spam — and that is ALL it does. Rejection writes no
// restriction, report or account state (social.md: punitive sanctions
// require explicit moderation action).
func TestAutomatedFilterRejectsNoSanction(t *testing.T) {
	f := NewFilter()
	restrictions := NewRestrictions()
	svc := New(restrictions, f, nil)
	sender := id.NewV4()
	now := time.Now()

	// Normal traffic admits freely.
	if code, ok := svc.CheckSend(context.Background(), sender,
		protocolv1.ChatChannel_CHAT_CHANNEL_WORLD, id.UUID{}, "chao"); !ok {
		t.Fatalf("normal send rejected: %v", code)
	}
	// Identical text repeated past the bound rejects as spam.
	var lastCode protocolv1.ErrorCode
	var ok bool
	for i := 0; i < repeatLimit; i++ {
		lastCode, ok = svc.CheckSend(context.Background(), sender,
			protocolv1.ChatChannel_CHAT_CHANNEL_WORLD, id.UUID{},
			"mua vang re")
	}
	if ok || lastCode != protocolv1.ErrorCode_ERROR_CODE_CHAT_TEXT_INVALID {
		t.Fatalf("spam send not rejected (ok=%v code=%v)", ok, lastCode)
	}
	// The rejection never issued a mute — the same sender still sends
	// different text.
	if restrictions.Restricted(sender,
		protocolv1.ChatChannel_CHAT_CHANNEL_WORLD, now) {
		t.Fatal("filter issued a sanction")
	}
	if code, ok := svc.CheckSend(context.Background(), sender,
		protocolv1.ChatChannel_CHAT_CHANNEL_WORLD, id.UUID{},
		"khac roi"); !ok {
		t.Fatalf("post-rejection send sanctioned: %v", code)
	}
}

// TestRestrictionEnforcedOnSend: an operator-issued MUTED restriction
// blocks sends on the muted channel subset only, expires at the
// server-authoritative deadline, and leaves other channels open.
func TestRestrictionEnforcedOnSend(t *testing.T) {
	restrictions := NewRestrictions()
	svc := New(restrictions, NewFilter(), nil)
	sender := id.NewV4()
	now := time.Now()
	until := now.Add(time.Hour)

	svc.Mute(sender, []protocolv1.ChatChannel{
		protocolv1.ChatChannel_CHAT_CHANNEL_WORLD,
	}, until)

	if code, ok := svc.CheckSend(context.Background(), sender,
		protocolv1.ChatChannel_CHAT_CHANNEL_WORLD, id.UUID{}, "x"); ok {
		t.Fatal("muted WORLD send admitted")
	} else if code != protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED {
		t.Fatalf("mute code = %v, want PERMISSION_DENIED", code)
	}
	// Subset: GUILD stays open while WORLD is muted.
	if _, ok := svc.CheckSend(context.Background(), sender,
		protocolv1.ChatChannel_CHAT_CHANNEL_GUILD, id.UUID{}, "x"); !ok {
		t.Fatal("channel subset not honored — GUILD blocked")
	}

	// Server-authoritative expiry: after `until` the mute is a NONE.
	future := func() time.Time { return until.Add(time.Second) }
	svcExpired := New(restrictions, NewFilter(), future)
	if _, ok := svcExpired.CheckSend(context.Background(), sender,
		protocolv1.ChatChannel_CHAT_CHANNEL_WORLD, id.UUID{}, "x"); !ok {
		t.Fatal("expired mute still restricting")
	}

	// An empty channel set mutes every player-authored channel.
	svc.Mute(sender, nil, time.Time{})
	if _, ok := svc.CheckSend(context.Background(), sender,
		protocolv1.ChatChannel_CHAT_CHANNEL_WHISPER, id.UUID{},
		"x"); ok {
		t.Fatal("blanket mute not enforced")
	}
	svc.Lift(sender)
	if _, ok := svc.CheckSend(context.Background(), sender,
		protocolv1.ChatChannel_CHAT_CHANNEL_WHISPER, id.UUID{},
		"x"); !ok {
		t.Fatal("lifted restriction still blocking")
	}
}
