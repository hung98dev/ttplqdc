package schema

import (
	"fmt"
	"strings"
	"testing"
)

// insertReceipt builds one durable_command_receipts insert; overrides map
// replaces column values (keyed by column name, raw SQL fragment values).
func insertReceiptStmt(over map[string]string) string {
	cols := map[string]string{
		"operation_family":       "'audit.test'",
		"owner_kind":             "'ACCOUNT'",
		"owner_id":               "'" + "00000000-0000-4000-8006-0000000000aa" + "'",
		"operation_id":           "'" + "00000000-0000-4000-8006-0000000000bb" + "'",
		"request_fingerprint":    "'\\x" + strings.Repeat("00", 32) + "'",
		"admitted_at":            "NOW()",
		"issued_at":              "NOW()",
		"replay_until":           "NOW() + INTERVAL '180 days'",
		"state":                  "'ADMITTED'",
		"outcome_schema_version": "NULL",
		"outcome":                "NULL",
		"completed_at":           "NULL",
		"disposition_ack_at":     "NULL",
	}
	for k, v := range over {
		cols[k] = v
	}
	order := []string{
		"operation_family", "owner_kind", "owner_id", "operation_id",
		"request_fingerprint", "admitted_at", "issued_at", "replay_until",
		"state", "outcome_schema_version", "outcome", "completed_at",
		"disposition_ack_at",
	}
	var names, vals []string
	for _, c := range order {
		names = append(names, c)
		vals = append(vals, cols[c])
	}
	return fmt.Sprintf("INSERT INTO durable_command_receipts (%s) VALUES (%s)",
		strings.Join(names, ", "), strings.Join(vals, ", "))
}

// TestBossDefeatedRevisionRequiredAndUndefeatedRevisionNull: defeated copy
// rows require a 64-hex reward content revision; undefeated rows reject one.
func TestBossDefeatedRevisionRequiredAndUndefeatedRevisionNull(t *testing.T) {
	p := newProbe(t)
	charID := p.mkCharacter()
	gen := mustUUID(t)
	hex := strings.Repeat("a", 64)
	ins := func(defeated bool, eligible, rev string) error {
		return p.exec(
			`INSERT INTO boss_chest_eligibility
			   (character_id, public_boss_spawn_generation_id, boss_id, copy_map_id,
			    copy_channel_id, copy_defeated, eligible_until, reward_content_revision)
			 VALUES ($1, $2, 'boss.test', 'map.test', 1, $3, `+eligible+`, `+rev+`)`,
			charID, gen, defeated)
	}
	if err := ins(true, "NOW()", "'"+hex+"'"); err != nil {
		t.Fatalf("defeated+revision rejected: %v", err)
	}
	if err := ins(true, "NOW()", "NULL"); err == nil {
		t.Fatal("defeated row without revision accepted")
	}
	if err := ins(true, "NOW()", "'"+strings.ToUpper(hex)+"'"); err == nil {
		t.Fatal("defeated row with uppercase revision accepted")
	}
	if err := ins(true, "NULL", "'"+hex+"'"); err == nil {
		t.Fatal("defeated row without eligible_until accepted")
	}
	if err := ins(false, "NULL", "'"+hex+"'"); err == nil {
		t.Fatal("undefeated row with revision accepted")
	}
	// second character on same copy key: undefeated, null revision.
	charID2 := p.mkCharacter()
	if err := p.exec(
		`INSERT INTO boss_chest_eligibility
		   (character_id, public_boss_spawn_generation_id, boss_id, copy_map_id,
		    copy_channel_id, copy_defeated, eligible_until, reward_content_revision)
		 VALUES ($1, $2, 'boss.test', 'map.test', 1, FALSE, NULL, NULL)`,
		charID2, gen); err != nil {
		t.Fatalf("undefeated+null revision rejected: %v", err)
	}
}

// TestDurableCommandReceiptConstraintsAndIndexes: verbatim receipt CHECKs
// and the two partial indexes.
func TestDurableCommandReceiptConstraintsAndIndexes(t *testing.T) {
	p := newProbe(t)
	bad := []struct {
		name string
		over map[string]string
	}{
		{"owner_kind world", map[string]string{"owner_kind": "'WORLD'"}},
		{"bad state", map[string]string{"state": "'BOGUS'"}},
		{"short fingerprint",
			map[string]string{"request_fingerprint": "'\\x" + strings.Repeat("00", 16) + "'"}},
		{"admitted with outcome",
			map[string]string{"outcome": "'\\x7b7d'", "outcome_schema_version": "1",
				"completed_at": "NOW()"}},
		{"committed null outcome",
			map[string]string{"state": "'COMMITTED'", "completed_at": "NOW()"}},
		{"replay_until mismatch",
			map[string]string{"replay_until": "NOW() + INTERVAL '90 days'"}},
		{"issued after admission+60s",
			map[string]string{"issued_at": "NOW() + INTERVAL '61 seconds'"}},
		{"admitted at/after horizon",
			map[string]string{"admitted_at": "NOW() + INTERVAL '180 days'"}},
	}
	for _, b := range bad {
		if err := p.exec(insertReceiptStmt(b.over)); err == nil {
			t.Fatalf("%s: accepted, want CHECK violation", b.name)
		}
	}
	if err := p.exec(insertReceiptStmt(map[string]string{})); err != nil {
		t.Fatalf("valid ADMITTED receipt rejected: %v", err)
	}
	open := indexDef(t, "durable_command_receipts_open_idx")
	if !strings.Contains(open, "WHERE") ||
		!strings.Contains(open, "disposition_ack_at IS NULL") {
		t.Fatalf("open index missing predicate: %s", open)
	}
	purge := indexDef(t, "durable_command_receipts_purge_idx")
	if !strings.Contains(purge, "WHERE") ||
		!strings.Contains(purge, "disposition_ack_at IS NOT NULL") {
		t.Fatalf("purge index missing predicate: %s", purge)
	}
}

// TestTerminalReceiptNullOutcomeVersionRejected: terminal states require
// outcome_schema_version = 1 + outcome bytes + completed_at.
func TestTerminalReceiptNullOutcomeVersionRejected(t *testing.T) {
	p := newProbe(t)
	base := map[string]string{
		"state":                  "'REJECTED'",
		"completed_at":           "NOW()",
		"outcome":                "'\\x7b7d'",
		"outcome_schema_version": "1",
	}
	if err := p.exec(insertReceiptStmt(base)); err != nil {
		t.Fatalf("valid REJECTED rejected: %v", err)
	}
	for _, over := range []map[string]string{
		{"state": "'REJECTED'", "completed_at": "NOW()",
			"outcome": "'\\x7b7d'", "outcome_schema_version": "NULL"},
		{"state": "'REJECTED'", "completed_at": "NOW()",
			"outcome": "'\\x7b7d'", "outcome_schema_version": "2"},
		{"state": "'REJECTED'", "completed_at": "NULL",
			"outcome": "'\\x7b7d'", "outcome_schema_version": "1"},
		{"state": "'COMMITTED'", "completed_at": "NOW()",
			"outcome": "NULL", "outcome_schema_version": "1"},
	} {
		if err := p.exec(insertReceiptStmt(over)); err == nil {
			t.Fatalf("terminal row %v accepted", over)
		}
	}
}

// TestContentRevisionHex64Check: content_revision CHAR(64) accepts lowercase
// 64-hex and rejects anything else.
func TestContentRevisionHex64Check(t *testing.T) {
	p := newProbe(t)
	hex := strings.Repeat("b", 64)
	if err := p.exec(
		`INSERT INTO item_instances (item_instance_id, item_id, quantity, effective_binding,
		                            content_revision, created_at)
		 VALUES ($1, 'item.test', 1, 'UNBOUND', $2, NOW())`,
		mustUUID(t), hex); err != nil {
		t.Fatalf("lowercase hex64 rejected: %v", err)
	}
	for _, bad := range []string{strings.ToUpper(hex), hex[:63], "zz" + hex[2:]} {
		if err := p.exec(
			`INSERT INTO item_instances (item_instance_id, item_id, quantity, effective_binding,
			                            content_revision, created_at)
			 VALUES ($1, 'item.test', 1, 'UNBOUND', $2, NOW())`,
			mustUUID(t), bad); err == nil {
			t.Fatalf("content_revision %q accepted", bad)
		}
	}
}

// TestOperationReplayUntilAndPurgeIndex: replay_until NOT NULL + the
// replay_until index exists for the retention sweep.
func TestOperationReplayUntilAndPurgeIndex(t *testing.T) {
	p := newProbe(t)
	if err := p.exec(
		`INSERT INTO operations (operation_family, owner_kind, owner_id, operation_id,
		                         request_fingerprint, outcome, created_at, completed_at,
		                         replay_until)
		 VALUES ('audit.test', 'ACCOUNT', $1, $2, '\\x00', '{}', NOW(), NOW(), NULL)`,
		mustUUID(t), mustUUID(t)); err == nil {
		t.Fatal("NULL replay_until accepted")
	}
	def := indexDef(t, "operations_replay_until_idx")
	if !strings.Contains(def, "replay_until") {
		t.Fatalf("unexpected index def: %s", def)
	}
}
