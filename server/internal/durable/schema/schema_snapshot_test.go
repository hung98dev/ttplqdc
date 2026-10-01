package schema

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/testing/pgtest"
)

// One baseline-applied database per package run; probe inserts run in
// rolled-back transactions so tests stay isolated.
var sharedPool *pgxpool.Pool

func testRepoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd + "/../../../.."
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	srv, err := pgtest.Ensure(ctx)
	if err == nil {
		defer srv.Close()
		name := "schema_tests_" + time.Now().Format("20060102150405")
		dsn, cleanup, e2 := srv.NewDB(ctx, name)
		if e2 != nil {
			fmt.Fprintf(os.Stderr, "pgtest newdb: %v\n", e2)
		} else {
			defer cleanup()
			if e3 := Migrate(ctx, dsn, MigrationsDir(testRepoRoot()), "up"); e3 != nil {
				fmt.Fprintf(os.Stderr, "pgtest migrate: %v\n", e3)
			} else if p, e4 := pgxpool.New(ctx, dsn); e4 == nil {
				defer p.Close()
				sharedPool = p
			}
		}
	} else {
		fmt.Fprintf(os.Stderr, "pgtest ensure: %v\n", err)
	}
	os.Exit(m.Run())
}

func pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if sharedPool == nil {
		t.Skip("DEFERRED(local-missing): no postgres")
	}
	return sharedPool
}

// probe is one test's transaction: fixture inserts persist for the test,
// probe statements run inside savepoints that always roll back, and the
// whole transaction rolls back when the test ends.
type probe struct {
	t  *testing.T
	tx pgx.Tx
}

func newProbe(t *testing.T) *probe {
	t.Helper()
	tx, err := pool(t).Begin(context.Background())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	p := &probe{t: t, tx: tx}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return p
}

// exec runs stmt in a savepoint that always rolls back; the returned error
// is the statement error (nil on success).
func (p *probe) exec(stmt string, args ...any) error {
	p.t.Helper()
	ntx, err := p.tx.Begin(context.Background())
	if err != nil {
		p.t.Fatalf("savepoint: %v", err)
	}
	defer ntx.Rollback(context.Background())
	_, err = ntx.Exec(context.Background(), stmt, args...)
	return err
}

// must runs stmt in the parent transaction (persists until test rollback).
func (p *probe) must(stmt string, args ...any) {
	p.t.Helper()
	if _, err := p.tx.Exec(context.Background(), stmt, args...); err != nil {
		p.t.Fatalf("exec %q: %v", stmt, err)
	}
}

func (p *probe) val(dst *any, stmt string, args ...any) {
	p.t.Helper()
	if err := p.tx.QueryRow(context.Background(), stmt, args...).Scan(dst); err != nil {
		p.t.Fatalf("query %q: %v", stmt, err)
	}
}

func queryVal(t *testing.T, dst *any, stmt string, args ...any) {
	t.Helper()
	if err := pool(t).QueryRow(context.Background(), stmt, args...).Scan(dst); err != nil {
		t.Fatalf("query %q: %v", stmt, err)
	}
}

func indexDef(t *testing.T, indexName string) string {
	t.Helper()
	var def any
	if err := pool(t).QueryRow(context.Background(),
		`SELECT indexdef FROM pg_indexes WHERE schemaname='public' AND indexname=$1`,
		indexName).Scan(&def); err == pgx.ErrNoRows {
		t.Fatalf("index %s not found", indexName)
	} else if err != nil {
		t.Fatalf("indexdef %s: %v", indexName, err)
	}
	return def.(string)
}

func liveCatalog(t *testing.T) *LiveCatalog {
	t.Helper()
	cat, err := LoadCatalog(context.Background(), sharedPool.Config().ConnString())
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	return cat
}

// colType returns e.g. "varchar(256)" for table.column or "".
func colType(cat *LiveCatalog, table, col string) string {
	for _, c := range cat.Columns[table] {
		if strings.HasPrefix(c, col+" ") {
			return c[len(col)+1:]
		}
	}
	return ""
}

func rand16str() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func mustUUID(t *testing.T) string {
	t.Helper()
	id := rand16str()
	return fmt.Sprintf("00000000-0000-4000-8005-%012s", id[len(id)-12:])
}

// Fixture helpers (insert into the probe's transaction) -------------------

func (p *probe) mkAccount() string {
	p.t.Helper()
	id := rand16str()
	uuid := fmt.Sprintf("00000000-0000-4000-8000-%012s", id[len(id)-12:])
	p.must(`INSERT INTO accounts (account_id, status) VALUES ($1, 'ACTIVE')`, uuid)
	return uuid
}

func (p *probe) mkCharacter() string {
	p.t.Helper()
	acct := p.mkAccount()
	id := rand16str()
	uuid := fmt.Sprintf("00000000-0000-4000-8001-%012s", id[len(id)-12:])
	p.must(
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id)
		 VALUES ($1, $2, $3, $4, 'class.kim')`,
		uuid, acct, "char_"+id[len(id)-8:], "k_"+id)
	return uuid
}

func (p *probe) mkItem() string {
	p.t.Helper()
	id := rand16str()
	uuid := fmt.Sprintf("00000000-0000-4000-8002-%012s", id[len(id)-12:])
	p.must(
		`INSERT INTO item_instances (item_instance_id, item_id, quantity, effective_binding, created_at)
		 VALUES ($1, 'item.test', 1, 'UNBOUND', NOW())`, uuid)
	return uuid
}

func (p *probe) mkGuild() string {
	p.t.Helper()
	id := rand16str()
	uuid := fmt.Sprintf("00000000-0000-4000-8003-%012s", id[len(id)-12:])
	p.must(
		`INSERT INTO guilds (guild_id, name, name_key, state, recruitment_mode,
		                    guild_revision, guild_storage_revision, created_at)
		 VALUES ($1, $2, $3, 'DISBANDING', 'CLOSED', 0, 0, NOW())`,
		uuid, "g"+id[len(id)-8:], "gk_"+id)
	return uuid
}

func (p *probe) mkRewardClaim() string {
	p.t.Helper()
	charID := p.mkCharacter()
	uuid := mustUUID(p.t)
	p.must(
		`INSERT INTO reward_claims (reward_claim_id, owner_character_id, source_type, source_reference,
		                            reward_slot, claim_kind, state, created_at, updated_at, revision)
		 VALUES ($1, $2, 'QUEST', 'quest.test', 'slot.a', 'SINGLE', 'PENDING', NOW(), NOW(), 0)`,
		uuid, charID)
	return uuid
}

// Tests ------------------------------------------------------------------

// TestBaselineApplyDownApply: migration applies, tears down to zero public
// tables, and re-applies cleanly on a scratch database.
func TestBaselineApplyDownApply(t *testing.T) {
	ctx := context.Background()
	srv, err := pgtest.Ensure(ctx)
	if err != nil {
		t.Skipf("DEFERRED(local-missing): %v", err)
	}
	defer srv.Close()
	dsn, cleanup, err := srv.NewDB(ctx, "schema_applydown_"+randSuffix())
	if err != nil {
		t.Fatalf("newdb: %v", err)
	}
	defer cleanup()
	dir := MigrationsDir(testRepoRoot())

	count := func() int {
		c, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatalf("connect: %v", err)
		}
		defer c.Close(ctx)
		var n int
		if err := c.QueryRow(ctx,
			`SELECT count(*) FROM pg_tables WHERE schemaname='public' AND tablename <> 'schema_migrations'`).
			Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}

	if err := Migrate(ctx, dsn, dir, "up"); err != nil {
		t.Fatalf("apply up: %v", err)
	}
	if n := count(); n < 80 {
		t.Fatalf("after up: %d tables, want >=80", n)
	}
	if err := Migrate(ctx, dsn, dir, "down"); err != nil {
		t.Fatalf("apply down: %v", err)
	}
	if n := count(); n != 0 {
		t.Fatalf("after down: %d tables remain", n)
	}
	if err := Migrate(ctx, dsn, dir, "up"); err != nil {
		t.Fatalf("re-apply up: %v", err)
	}
	if n := count(); n < 80 {
		t.Fatalf("after re-apply: %d tables, want >=80", n)
	}
}

// TestPerConstraintSnapshot: the authored DDL parses into the full catalog
// expectation set and every derived expectation holds on the live catalog.
func TestPerConstraintSnapshot(t *testing.T) {
	live := liveCatalog(t)
	parsed, err := ParseMigration(filepath.Join(
		MigrationsDir(testRepoRoot()), "000001_baseline_schema.up.sql"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tables := 0
	for _, e := range parsed.Entries {
		if e.Kind == "table" {
			tables++
		}
	}
	if tables < 80 {
		t.Fatalf("parser found %d tables, want >=80", tables)
	}
	if viols := parsed.Verify(live); len(viols) != 0 {
		for _, v := range viols {
			t.Logf("violation: %s", v)
		}
		t.Fatalf("%d catalog violations", len(viols))
	}
}

// TestMigrationsImmutable: the migrations directory holds only NNNNNN
// pairs plus the exempt snapshot, and every numbered file has its pair.
func TestMigrationsImmutable(t *testing.T) {
	dir := MigrationsDir(testRepoRoot())
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	ups, downs := map[string]bool{}, map[string]bool{}
	for _, e := range ents {
		name := e.Name()
		switch {
		case name == "schema_snapshot.sql":
		case strings.HasSuffix(name, ".up.sql"):
			base := strings.TrimSuffix(name, ".up.sql")
			if len(base) < 8 || base[:6] != "000001" && base[:6] != "000002" && base[:6] != "000003" {
				if len(base) < 6 || base[6] != '_' {
					t.Fatalf("bad migration name %s", name)
				}
			}
			ups[base] = true
		case strings.HasSuffix(name, ".down.sql"):
			downs[strings.TrimSuffix(name, ".down.sql")] = true
		default:
			t.Fatalf("unexpected file in migrations dir: %s", name)
		}
	}
	for name := range ups {
		if !downs[name] {
			t.Fatalf("missing down pair for %s", name)
		}
	}
	for name := range downs {
		if !ups[name] {
			t.Fatalf("missing up pair for %s", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "schema_snapshot.sql")); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
}

// TestBaselineAdr0065Tables: ADR-0065 inventory tables exist with the keyed
// constraints the packet enumerates.
func TestBaselineAdr0065Tables(t *testing.T) {
	cat := liveCatalog(t)
	want := []string{
		"accounts", "account_login_history", "account_password_credentials",
		"characters", "iap_notification_dedup", "iap_provider_cursors",
		"reward_claims", "reward_claim_lines", "reward_claim_contributions",
		"auction_listings", "auction_proceeds", "guilds", "guild_memberships",
		"audit_events", "operations", "durable_command_receipts",
	}
	for _, tbl := range want {
		if !cat.Tables[tbl] {
			t.Fatalf("missing table %s", tbl)
		}
	}
	for _, tbl := range []string{"characters", "guilds"} {
		if typ := colType(cat, tbl, "name_key"); typ != "character varying(256)" {
			t.Fatalf("%s.name_key type %q", tbl, typ)
		}
		found := false
		for _, c := range cat.Constraints[tbl] {
			if c.contype == "u" && strings.Contains(c.def, "name_key") {
				found = true
			}
		}
		if !found && !hasUniqueIndex(cat.Indexes[tbl], "(name_key)") {
			t.Fatalf("%s.name_key lacks UNIQUE", tbl)
		}
	}
	if def := indexDef(t, "auction_listings_expiry_idx"); !strings.Contains(
		def, "(expires_at)") || !strings.Contains(def, "'ACTIVE'") {
		t.Fatalf("auction_listings_expiry_idx wrong: %s", def)
	}
}

// TestTombstoneAccountSeeded: the reserved tombstone owner row exists.
func TestTombstoneAccountSeeded(t *testing.T) {
	var status any
	queryVal(t, &status,
		`SELECT status FROM accounts WHERE account_id='00000000-0000-0000-0000-000000000001'`)
	if status.(string) != "TOMBSTONE_ERASED" {
		t.Fatalf("tombstone status %v", status)
	}
}

// TestSeasonTrackIndexExcludesTombstone: the live-season partial unique
// index on account_iap_entitlements excludes the tombstone.
func TestSeasonTrackIndexExcludesTombstone(t *testing.T) {
	def := indexDef(t, "account_iap_entitlements_live_season_idx")
	if !strings.Contains(def, "00000000-0000-0000-0000-000000000001") {
		t.Fatalf("index lacks tombstone exclusion: %s", def)
	}
	if !strings.Contains(def, "WHERE") {
		t.Fatalf("index is not partial: %s", def)
	}
	p := newProbe(t)
	// Tombstone-scoped duplicate season tracks must not collide.
	insert := `INSERT INTO account_iap_entitlements
		   (entitlement_id, account_id, product_id, entitlement_type, season_number,
		    claim_deadline_at, grant_state, platform, platform_receipt, created_at)
		 VALUES
		   ($1, $2, 'season.track.1', 'ACCOUNT_SCOPED_ACCESS', 1, NOW() + INTERVAL '30 days',
		    'GRANTED', 'GOOGLE_PLAY', $3, NOW()),
		   ($4, $2, 'season.track.1', 'ACCOUNT_SCOPED_ACCESS', 1, NOW() + INTERVAL '30 days',
		    'GRANTED', 'STEAM', $5, NOW())`
	err := p.exec(insert,
		mustUUID(t), "00000000-0000-0000-0000-000000000001", "rcpt_"+rand16str(),
		mustUUID(t), "rcpt2_"+rand16str())
	if err != nil {
		t.Fatalf("tombstone duplicate season insert should pass: %v", err)
	}
	// ...but two live tracks for a real account still collide.
	acct := p.mkAccount()
	err = p.exec(insert,
		mustUUID(t), acct, "r_"+rand16str(), mustUUID(t), "r2_"+rand16str())
	if err == nil {
		t.Fatal("duplicate live season track accepted")
	}
}

// TestEntitlementClaimFksDeferrable: the composite FKs on
// account_entitlement_claims are DEFERRABLE INITIALLY IMMEDIATE.
func TestEntitlementClaimFksDeferrable(t *testing.T) {
	var n any
	queryVal(t, &n,
		`SELECT count(*) FROM pg_constraint c JOIN pg_class t ON c.conrelid=t.oid
		 WHERE t.relname='account_entitlement_claims' AND c.contype='f'
		   AND c.condeferrable AND NOT c.condeferred`)
	if n.(int64) != 2 {
		t.Fatalf("want 2 deferrable-initially-immediate FKs, got %v", n)
	}
}

// TestBaselineAdr0060Tables: ADR-0060 surface — cosmetic entitlements,
// souls, beast food daily cap, IAP entitlement rejection columns.
func TestBaselineAdr0060Tables(t *testing.T) {
	cat := liveCatalog(t)
	for _, tbl := range []string{
		"account_iap_entitlements", "account_cosmetic_entitlements",
		"character_cosmetic_entitlements", "character_cosmetic_equips",
		"character_souls", "character_beast_food_daily", "account_entitlement_claims",
	} {
		if !cat.Tables[tbl] {
			t.Fatalf("missing ADR-0060 table %s", tbl)
		}
	}
	for _, col := range []string{"grant_state", "reject_reason", "platform", "platform_receipt"} {
		if colType(cat, "account_iap_entitlements", col) == "" {
			t.Fatalf("account_iap_entitlements missing %s", col)
		}
	}
	if colType(cat, "account_cosmetic_entitlements", "first_equipped_at") == "" {
		t.Fatal("account_cosmetic_entitlements missing first_equipped_at")
	}
	p := newProbe(t)
	if err := p.exec(
		`INSERT INTO character_beast_food_daily (character_id, utc_date, food_points_gained)
		 VALUES ($1, CURRENT_DATE, 21)`, p.mkCharacter()); err == nil {
		t.Fatal("food_points_gained=21 accepted, want CHECK violation")
	}
	if err := p.exec(
		`INSERT INTO character_beast_food_daily (character_id, utc_date, food_points_gained)
		 VALUES ($1, CURRENT_DATE, 20)`, p.mkCharacter()); err != nil {
		t.Fatalf("food_points_gained=20 rejected: %v", err)
	}
}

// TestNoStoredRefundScore: the refund-consumed score is derived-only — no
// persisted column may carry it.
func TestNoStoredRefundScore(t *testing.T) {
	var n any
	queryVal(t, &n,
		`SELECT count(*) FROM information_schema.columns
		 WHERE table_schema='public' AND column_name='iap_refund_consumed_score'`)
	if n.(int64) != 0 {
		t.Fatal("iap_refund_consumed_score column exists")
	}
}

// TestBaselinePublicBossSchedules: ADR-0061 state CHECK — OPEN requires the
// generation/opened pair; SCHEDULED requires next_spawn_at.
func TestBaselinePublicBossSchedules(t *testing.T) {
	p := newProbe(t)
	if err := p.exec(
		`INSERT INTO public_boss_schedules (boss_id, state, public_boss_spawn_generation_id,
		                                    opened_at, next_spawn_at, revision)
		 VALUES ('boss.test.open', 'OPEN', $1, NOW(), NULL, 0)`, mustUUID(t)); err != nil {
		t.Fatalf("valid OPEN row rejected: %v", err)
	}
	if err := p.exec(
		`INSERT INTO public_boss_schedules (boss_id, state, next_spawn_at, revision)
		 VALUES ('boss.test.sched', 'SCHEDULED', NOW(), 0)`); err != nil {
		t.Fatalf("valid SCHEDULED row rejected: %v", err)
	}
	bad := []struct{ name, stmt string }{
		{"open-all-null",
			`INSERT INTO public_boss_schedules (boss_id, state, revision)
			 VALUES ('boss.test.bad1', 'OPEN', 0)`},
		{"scheduled-all-null",
			`INSERT INTO public_boss_schedules (boss_id, state, revision)
			 VALUES ('boss.test.bad2', 'SCHEDULED', 0)`},
		{"open-without-generation",
			`INSERT INTO public_boss_schedules (boss_id, state, opened_at, revision)
			 VALUES ('boss.test.bad3', 'OPEN', NOW(), 0)`},
	}
	for _, b := range bad {
		if err := p.exec(b.stmt); err == nil {
			t.Fatalf("invalid %s row accepted", b.name)
		}
	}
}
