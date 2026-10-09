package chat

import (
	"context"
	"testing"
	"time"

	"thinhthan/internal/core/id"
)

// TestReportCreatesCase: a submitted report persists as an OPEN case
// carrying every registered evidence field — reporter ownership, the
// optional chat_message_id reference and bounded notes (social.md §
// Reports / data_model.md § player_reports).
func TestReportCreatesCase(t *testing.T) {
	wipe(t)
	pool := requirePool(t)
	reports := NewReports(pool)
	reporterAcct, reporter := mkSender(t)
	_, target := mkSender(t)
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
// PRIV-004: when the reporter is erased (account chat_messages rows
// deleted per the 15-day action, character re-pointed to
// TOMBSTONE_ACCOUNT_ID, account row removed), the report retains every
// registered evidence field and its original reporter UUID with no
// account FK requirement, and the player-facing export only ever
// surfaces the reporter-own projection — never target or operator data.
func TestReportEvidenceSurvivesReporterErasureWithoutBroadDisclosure(t *testing.T) {
	wipe(t)
	pool := requirePool(t)
	reports := NewReports(pool)
	reporterAcct, reporter := mkSender(t)
	targetAcct, target := mkSender(t)
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

	// Reporter erasure: canonical actions — the account's chat rows
	// deleted (IMP-056's 15-day action) and the character re-pointed
	// to TOMBSTONE_ACCOUNT_ID (characters persist; data_model.md).
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
	exports, err := reports.ExportForAccount(context.Background(), nil,
		targetAcct)
	if err != nil || len(exports) != 0 {
		t.Fatalf("target received report evidence: %d", len(exports))
	}
}
