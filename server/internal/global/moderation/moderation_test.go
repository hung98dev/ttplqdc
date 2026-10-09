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

// TestReportCreatesCase: a submitted report persists as an OPEN case
// carrying every registered evidence field — reporter ownership, the
// optional chat_message_id reference and bounded notes.
func TestReportCreatesCase(t *testing.T) {
	wipe(t)
	pool := requirePool(t)
	reports := NewReports(pool)
	reporterAcct, reporter := mkCharacter(t)
	_, target := mkCharacter(t)
	now := time.Now().UTC()
	notes := "ke spam kenh the gioi"
	chatRef := id.NewV4()

	// The referenced chat message must exist (evidence pointer).
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO chat_messages
		    (message_id, sender_account_id, sender_character_id,
		     channel, content, created_at)
		 VALUES ($1,$2,$3,'WORLD','mua vang re',now())`,
		chatRef[:], reporterAcct[:], reporter[:]); err != nil {
		t.Fatalf("chat seed: %v", err)
	}
	reportID := mkReport(t, reporterAcct, reporter, target,
		"SPAM", &chatRef, &notes, now)

	c, ok, err := reports.CaseByID(context.Background(), nil, reportID)
	if err != nil || !ok {
		t.Fatalf("case lookup: %v", err)
	}
	if c.Status != "OPEN" || c.ReporterAccountID != reporterAcct ||
		c.ReporterCharacterID != reporter || c.TargetCharacterID != target {
		t.Fatal("case identity mismatch")
	}
	if c.Reason != "SPAM" {
		t.Fatalf("reason = %q", c.Reason)
	}
	if c.ChatMessageID == nil || *c.ChatMessageID != chatRef {
		t.Fatal("chat evidence reference lost")
	}
	if c.ReporterNotes == nil || *c.ReporterNotes != notes {
		t.Fatal("reporter notes lost")
	}
	open, err := reports.OpenForCharacter(context.Background(), nil, reporter)
	if err != nil || len(open) != 1 {
		t.Fatalf("open cases = %d err=%v", len(open), err)
	}
}

// TestReportEvidenceSurvivesReporterErasureWithoutBroadDisclosure —
// PRIV-004: when the reporter's account is erased (account and the
// account's chat_messages rows deleted), the report retains every
// registered evidence field and its original reporter UUID, still with
// no account FK requirement, and the player-facing export only ever
// surfaces the reporter-own projection — never operator or joined
// evidence.
func TestReportEvidenceSurvivesReporterErasureWithoutBroadDisclosure(t *testing.T) {
	wipe(t)
	pool := requirePool(t)
	reports := NewReports(pool)
	reporterAcct, reporter := mkCharacter(t)
	_, target := mkCharacter(t)
	now := time.Now().UTC()
	notes := "evidence note"
	chatRef := id.NewV4()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO chat_messages
		    (message_id, sender_account_id, sender_character_id,
		     channel, content, created_at)
		 VALUES ($1,$2,$3,'WORLD','spam text',now())`,
		chatRef[:], reporterAcct[:], reporter[:]); err != nil {
		t.Fatalf("chat seed: %v", err)
	}
	reportID := mkReport(t, reporterAcct, reporter, target,
		"HARASSMENT", &chatRef, &notes, now)

	// Reporter erasure: the canonical actions are the account's
	// chat_messages rows deleted (15-day action IMP-056 runs) and the
	// character re-pointed to TOMBSTONE_ACCOUNT_ID — characters persist
	// (data_model.md: character FKs remain valid because characters
	// persist after player erasure).
	tombstone := id.UUID{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	if _, err := pool.Exec(context.Background(),
		`DELETE FROM chat_messages WHERE sender_account_id = $1`,
		reporterAcct[:]); err != nil {
		t.Fatalf("erase chat: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`UPDATE characters SET account_id = $1 WHERE character_id = $2`,
		tombstone[:], reporter[:]); err != nil {
		t.Fatalf("re-point character: %v", err)
	}
	if _, err := pool.Exec(context.Background(),
		`DELETE FROM accounts WHERE account_id = $1`,
		reporterAcct[:]); err != nil {
		t.Fatalf("erase account: %v", err)
	}

	// The case survives with every evidence field — reporter UUID
	// intact without an account FK.
	c, ok, err := reports.CaseByID(context.Background(), nil, reportID)
	if err != nil || !ok {
		t.Fatalf("case lost on reporter erasure: %v", err)
	}
	if c.ReporterAccountID != reporterAcct ||
		c.ReporterCharacterID != reporter ||
		c.TargetCharacterID != target || c.Reason != "HARASSMENT" {
		t.Fatal("evidence identity mutated")
	}
	if c.ChatMessageID == nil || *c.ChatMessageID != chatRef {
		t.Fatal("evidence pointer lost")
	}
	if c.ReporterNotes == nil || *c.ReporterNotes != notes {
		t.Fatal("evidence notes lost")
	}
	if c.ResolvedAt != nil || c.Status != "OPEN" {
		t.Fatal("case state mutated")
	}

	// Export only surfaces the reporter-own projection — the target's
	// account sees nothing; operator attribution is absent by shape.
	targetAcct := accountOf(t, target)
	exports, err := reports.ExportForAccount(context.Background(), nil,
		targetAcct)
	if err != nil || len(exports) != 0 {
		t.Fatalf("target received report evidence: %d", len(exports))
	}
}

func accountOf(t *testing.T, char id.UUID) id.UUID {
	t.Helper()
	var acc []byte
	if err := sharedPool.QueryRow(context.Background(),
		`SELECT account_id FROM characters WHERE character_id = $1`,
		char[:]).Scan(&acc); err != nil {
		t.Fatalf("accountOf: %v", err)
	}
	var u id.UUID
	copy(u[:], acc)
	return u
}
