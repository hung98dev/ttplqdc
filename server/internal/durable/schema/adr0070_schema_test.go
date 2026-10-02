package schema

import (
	"fmt"
	"strings"
	"testing"
)

// TestErasureIntentStagingFence: pending intents keep the account pointer,
// completed ones are fenced (account cleared), hash is exactly 32 bytes and
// completion never precedes preparation. One pending intent per account.
func TestErasureIntentStagingFence(t *testing.T) {
	p := newProbe(t)
	acct := p.mkAccount()
	hash := "'\\x" + strings.Repeat("ab", 32) + "'"
	ins := func(op, account, completed, prepared string) error {
		return p.exec(`INSERT INTO erasure_intents
			   (operation_id, account_id, account_id_hash, prepared_at, completed_at)
			 VALUES (` + op + `, ` + account + `, ` + hash + `, ` + prepared + `, ` + completed + `)`)
	}
	// First pending intent persists (committed inside the probe tx).
	op := mustUUID(t)
	p.must(`INSERT INTO erasure_intents
		   (operation_id, account_id, account_id_hash, prepared_at, completed_at)
		 VALUES ($1, $2, `+hash+`, NOW(), NULL)`, op, acct)
	if err := ins("'"+mustUUID(t)+"'", "'"+acct+"'", "NULL", "NOW()"); err == nil {
		t.Fatal("second pending intent for same account accepted")
	}
	if err := ins("'"+mustUUID(t)+"'", "NULL", "NOW()", "NOW() - INTERVAL '1 hour'"); err != nil {
		t.Fatalf("completed intent rejected: %v", err)
	}
	bad := []struct {
		name                            string
		account, completed, prepared, h string
	}{
		{"pending without account", "NULL", "NULL", "NOW()", hash},
		{"completed keeps account", "'" + acct + "'", "NOW()", "NOW()", hash},
		{"completed before prepared", "NULL", "NOW() - INTERVAL '2 hours'", "NOW()", hash},
		{"hash too short", "NULL", "NOW()", "NOW()",
			"'\\x" + strings.Repeat("ab", 16) + "'"},
	}
	for _, b := range bad {
		stmt := `INSERT INTO erasure_intents
			   (operation_id, account_id, account_id_hash, prepared_at, completed_at)
			 VALUES ('` + mustUUID(t) + `', ` + b.account + `, ` + b.h + `, ` +
			b.prepared + `, ` + b.completed + `)`
		if err := p.exec(stmt); err == nil {
			t.Fatalf("%s accepted", b.name)
		}
	}
}

// TestRelicTypedColumnsAndIndexes: typed world_consequence_relics columns
// (60-minute expiry pinned to spawn) and both active-only partial indexes.
func TestRelicTypedColumnsAndIndexes(t *testing.T) {
	p := newProbe(t)
	ins := func(channel, expires string) error {
		return p.exec(
			`INSERT INTO world_consequence_relics
			   (map_id, channel_id, relic_id, source_id, relic_active, buff_effect_id,
			    spawned_at, expires_at)
			 VALUES ('map.test', ` + channel + `, 'relic.boss.test', 'boss.test', TRUE,
			         'buff.di_tich.test', NOW(), ` + expires + `)`)
	}
	if err := ins("1", "NOW() + INTERVAL '60 minutes'"); err != nil {
		t.Fatalf("valid relic rejected: %v", err)
	}
	if err := ins("0", "NOW() + INTERVAL '60 minutes'"); err == nil {
		t.Fatal("channel 0 accepted")
	}
	if err := ins("31", "NOW() + INTERVAL '60 minutes'"); err == nil {
		t.Fatal("channel 31 accepted")
	}
	if err := ins("2", "NOW() + INTERVAL '59 minutes'"); err == nil {
		t.Fatal("expires != spawned+60m accepted")
	}
	for _, idx := range []string{
		"world_consequence_relics_active_idx",
		"world_consequence_relics_expiry_idx",
	} {
		def := indexDef(t, idx)
		if !strings.Contains(def, "WHERE") || !strings.Contains(def, "relic_active") {
			t.Fatalf("%s not a partial active index: %s", idx, def)
		}
	}
	// typed marker columns + defeated-nulness fence.
	var n any
	queryVal(t, &n,
		`SELECT count(*) FROM pg_constraint c JOIN pg_class t ON c.conrelid=t.oid
		 WHERE t.relname='region_di_tich_markers' AND c.contype='c'
		   AND pg_get_constraintdef(c.oid) ILIKE '%last_defeated_utc%'`)
	if n.(int64) == 0 {
		t.Fatal("region_di_tich_markers lacks defeated-nullness CHECK")
	}
}

// TestRewardClaimLineKindCheck: line-kind discriminated union — ITEM lines
// carry item fields only; CURRENCY lines carry currency fields only.
func TestRewardClaimLineKindCheck(t *testing.T) {
	p := newProbe(t)
	claim := p.mkRewardClaim()
	hex := strings.Repeat("c", 64)
	good := `INSERT INTO reward_claim_lines
			   (reward_claim_id, line_no, line_kind, item_id, quantity,
			    effective_binding, content_revision)
			 VALUES ($1, 1, 'ITEM', 'item.test', 3, 'UNBOUND', $2)`
	if err := p.exec(good, claim, hex); err != nil {
		t.Fatalf("valid ITEM line rejected: %v", err)
	}
	goodC := `INSERT INTO reward_claim_lines
			   (reward_claim_id, line_no, line_kind, currency_id, amount)
			 VALUES ($1, 2, 'CURRENCY', 'currency.common', 100)`
	if err := p.exec(goodC, claim); err != nil {
		t.Fatalf("valid CURRENCY line rejected: %v", err)
	}
	bad := []string{
		`INSERT INTO reward_claim_lines (reward_claim_id, line_no, line_kind)
		   VALUES ('` + claim + `', 3, 'BOGUS')`,
		`INSERT INTO reward_claim_lines (reward_claim_id, line_no, line_kind, item_id)
		   VALUES ('` + claim + `', 4, 'ITEM', 'item.test')`,
		`INSERT INTO reward_claim_lines (reward_claim_id, line_no, line_kind,
		                                currency_id, amount, item_id)
		   VALUES ('` + claim + `', 5, 'CURRENCY', 'currency.common', 5, 'item.test')`,
	}
	for i, stmt := range bad {
		if err := p.exec(stmt); err == nil {
			t.Fatalf("bad line %d accepted", i)
		}
	}
}

// TestAuctionEndedAtCheck: (state='ACTIVE') == (ended_at IS NULL).
func TestAuctionEndedAtCheck(t *testing.T) {
	p := newProbe(t)
	charID := p.mkCharacter()
	acct := p.mkAccount()
	item := p.mkItem()
	ins := func(state, ended string) error {
		return p.exec(
			`INSERT INTO auction_listings
			   (listing_id, seller_character_id, seller_account_id, item_instance_id,
			    item_id, quantity, price_common, listing_fee_common, state,
			    listed_at, expires_at, ended_at, revision)
			 VALUES ($1, $2, $3, $4, 'item.test', 1, 100, 10, `+state+`,
			         NOW(), NOW() + INTERVAL '24 hours', `+ended+`, 0)`,
			mustUUID(t), charID, acct, item)
	}
	if err := ins("'ACTIVE'", "NULL"); err != nil {
		t.Fatalf("ACTIVE listing rejected: %v", err)
	}
	if err := ins("'ACTIVE'", "NOW()"); err == nil {
		t.Fatal("ACTIVE listing with ended_at accepted")
	}
	if err := ins("'SOLD'", "NULL"); err == nil {
		t.Fatal("SOLD listing without ended_at accepted")
	}
	if err := ins("'SOLD'", "NOW()"); err != nil {
		t.Fatalf("SOLD listing with ended_at rejected: %v", err)
	}
}

// TestGuildStorageAuditChecks: action/section CHECK enums and the receiver
// partial index.
func TestGuildStorageAuditChecks(t *testing.T) {
	p := newProbe(t)
	guild := p.mkGuild()
	charID := p.mkCharacter()
	ins := func(action, section string) error {
		return p.exec(
			`INSERT INTO guild_storage_audit
			   (audit_id, guild_id, operation_id, actor_character_id, action, section,
			    item_id, quantity, before_quantity, after_quantity, occurred_at)
			 VALUES ($1, $2, $3, $4, `+action+`, `+section+`,
			         'item.test', 1, 0, 1, NOW())`,
			mustUUID(t), guild, mustUUID(t), charID)
	}
	if err := ins("'DEPOSIT'", "'COMMON'"); err != nil {
		t.Fatalf("valid audit row rejected: %v", err)
	}
	if err := ins("'BOGUS'", "'COMMON'"); err == nil {
		t.Fatal("bad action accepted")
	}
	if err := ins("'DEPOSIT'", "'VAULT'"); err == nil {
		t.Fatal("bad section accepted")
	}
	def := indexDef(t, "guild_storage_audit_receiver_idx")
	if !strings.Contains(def, "WHERE") || !strings.Contains(def, "CLAIM_DELIVER") {
		t.Fatalf("receiver index missing predicate: %s", def)
	}
	// quantity must be > 0.
	if err := p.exec(
		fmt.Sprintf(`INSERT INTO guild_storage_audit
			   (audit_id, guild_id, operation_id, actor_character_id, action, section,
			    item_id, quantity, before_quantity, after_quantity, occurred_at)
			 VALUES ('%s', '%s', '%s', '%s', 'DEPOSIT', 'COMMON',
			         'item.test', 0, 0, 1, NOW())`,
			mustUUID(t), guild, mustUUID(t), charID)); err == nil {
		t.Fatal("quantity=0 accepted")
	}
}
