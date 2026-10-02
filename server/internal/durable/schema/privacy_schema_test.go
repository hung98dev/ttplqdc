package schema

import (
	"strings"
	"testing"
)

// TestReportEvidenceSchemaAndIndependentPurge: player_reports keeps the
// reporter/target rows with the canonical reason enum and carries the
// standalone created_at purge index (reports purge independently of chat).
func TestReportEvidenceSchemaAndIndependentPurge(t *testing.T) {
	p := newProbe(t)
	reporter := p.mkCharacter()
	target := p.mkCharacter()
	good := `INSERT INTO player_reports
			   (report_id, operation_id, reporter_account_id, reporter_character_id,
			    target_character_id, reason, created_at)
			 VALUES ($1, $2, $3, $4, $5, 'SPAM', NOW())`
	var acct any
	p.val(&acct, `SELECT account_id::text FROM characters WHERE character_id=$1`, reporter)
	if err := p.exec(good, mustUUID(t), mustUUID(t), acct.(string), reporter, target); err != nil {
		t.Fatalf("valid report rejected: %v", err)
	}
	if err := p.exec(
		`INSERT INTO player_reports
		   (report_id, operation_id, reporter_account_id, reporter_character_id,
		    target_character_id, reason, created_at)
		 VALUES ($1, $2, $3, $4, $5, 'BOGUS', NOW())`,
		mustUUID(t), mustUUID(t), acct.(string), reporter, target); err == nil {
		t.Fatal("bad reason accepted")
	}
	// RESOLVED must set resolved_at.
	if err := p.exec(
		`INSERT INTO player_reports
		   (report_id, operation_id, reporter_account_id, reporter_character_id,
		    target_character_id, reason, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, 'CHEATING', 'RESOLVED', NOW())`,
		mustUUID(t), mustUUID(t), acct.(string), reporter, target); err == nil {
		t.Fatal("RESOLVED without resolved_at accepted")
	}
	for _, idx := range []string{"player_reports_created_idx"} {
		def := indexDef(t, idx)
		if !strings.Contains(def, "created_at") {
			t.Fatalf("unexpected purge index: %s", def)
		}
	}
}

// TestDisabledOperatorCredentialAndRetentionConstraints: ACTIVE operators
// hold both credentials; DISABLED operators hold neither and carry
// disabled_at, with the canonical partial purge index on disabled_at.
func TestDisabledOperatorCredentialAndRetentionConstraints(t *testing.T) {
	p := newProbe(t)
	ins := func(status, pw, totp, disabled string) error {
		return p.exec(
			`INSERT INTO operators (operator_id, login_key, password_hash,
			                        totp_secret_encrypted, role, status,
			                        created_at, disabled_at)
			 VALUES ($1, $2, `+pw+`, `+totp+`, 'SUPPORT', `+status+`,
			         NOW(), `+disabled+`)`,
			mustUUID(t), "op_"+rand16str())
	}
	if err := ins("'ACTIVE'", "'hash'", "'\\x00'", "NULL"); err != nil {
		t.Fatalf("valid ACTIVE rejected: %v", err)
	}
	if err := ins("'DISABLED'", "NULL", "NULL", "NOW()"); err != nil {
		t.Fatalf("valid DISABLED rejected: %v", err)
	}
	bad := []struct{ name, status, pw, totp, disabled string }{
		{"active no password", "'ACTIVE'", "NULL", "'\\x00'", "NULL"},
		{"active no totp", "'ACTIVE'", "'hash'", "NULL", "NULL"},
		{"active with disabled_at", "'ACTIVE'", "'hash'", "'\\x00'", "NOW()"},
		{"disabled keeps password", "'DISABLED'", "'hash'", "NULL", "NOW()"},
		{"disabled without stamp", "'DISABLED'", "NULL", "NULL", "NULL"},
	}
	for _, b := range bad {
		if err := ins(b.status, b.pw, b.totp, b.disabled); err == nil {
			t.Fatalf("%s accepted", b.name)
		}
	}
	def := indexDef(t, "operators_disabled_idx")
	if !strings.Contains(def, "WHERE") || !strings.Contains(def, "DISABLED") {
		t.Fatalf("disabled index missing predicate: %s", def)
	}
}

// TestErasureIntentCompletionConstraintAndPurgeIndex: the completion fence
// (account cleared exactly when completed) plus the completed-intent
// partial index used by the erasure-deadline sweep.
func TestErasureIntentCompletionConstraintAndPurgeIndex(t *testing.T) {
	p := newProbe(t)
	def := indexDef(t, "erasure_intents_completed_idx")
	if !strings.Contains(def, "WHERE") ||
		!strings.Contains(def, "completed_at IS NOT NULL") {
		t.Fatalf("completed index missing predicate: %s", def)
	}
	pending := indexDef(t, "erasure_intents_pending_prepared_idx")
	if !strings.Contains(pending, "completed_at IS NULL") {
		t.Fatalf("pending index missing predicate: %s", pending)
	}
	// account_id cleared at completion is enforced by table CHECK; a row
	// with both set or both null violates it.
	acct := p.mkAccount()
	hash := "'\\x" + strings.Repeat("cd", 32) + "'"
	if err := p.exec(
		`INSERT INTO erasure_intents
		   (operation_id, account_id, account_id_hash, prepared_at, completed_at)
		 VALUES ($1, $2, `+hash+`, NOW(), NOW())`,
		mustUUID(t), acct); err == nil {
		t.Fatal("completed intent retaining account_id accepted")
	}
}
