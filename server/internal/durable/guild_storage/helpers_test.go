package guild_storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/guild"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/inventory"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/schema"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/testing/pgtest"
)

var (
	sharedPool *pgxpool.Pool
	setupErr   error
)

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
	switch {
	case errors.Is(err, pgtest.ErrUnavailable):
		fmt.Fprintf(os.Stderr, "pgtest ensure: %v\n", err)
		os.Exit(m.Run())
	case err != nil:
		setupErr = err
		os.Exit(m.Run())
	}
	name := fmt.Sprintf("guild_storage_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
	dsn, cleanup, err := srv.NewDB(ctx, name)
	if err != nil {
		setupErr = err
		os.Exit(m.Run())
	}
	if err := schema.Migrate(ctx, dsn, schema.MigrationsDir(testRepoRoot()), "up"); err != nil {
		setupErr = err
	} else if p, err := pgxpool.New(ctx, dsn); err != nil {
		setupErr = err
	} else {
		sharedPool = p
	}
	code := m.Run()
	if sharedPool != nil {
		sharedPool.Close()
	}
	if cleanup != nil {
		cleanup()
	}
	srv.Close()
	os.Exit(code)
}

func requirePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if setupErr != nil {
		t.Skipf("pg setup: %v", setupErr)
	}
	if sharedPool == nil {
		t.Skip("pg pool unavailable")
	}
	return sharedPool
}

func wipe(t *testing.T) {
	t.Helper()
	pool := requirePool(t)
	for _, table := range []string{
		"guild_storage_audit", "guild_storage_claims", "item_locations",
		"item_instances", "guild_memberships", "guild_membership_history",
		"guild_progression", "guilds", "economy_character_daily_rollups",
		"character_inventories", "characters", "accounts",
	} {
		if _, err := pool.Exec(context.Background(),
			"DELETE FROM "+table); err != nil {
			t.Fatalf("wipe %s: %v", table, err)
		}
	}
}

// mkChar inserts one character and returns (char, account).
func mkChar(t *testing.T, level int32) (id.UUID, id.UUID) {
	t.Helper()
	ctx := context.Background()
	char, account := id.NewV4(), id.NewV4()
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO accounts (account_id) VALUES ($1)`, account[:]); err != nil {
		t.Fatalf("account: %v", err)
	}
	name := fmt.Sprintf("c%x", char[:4])
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id, level, created_at)
		 VALUES ($1,$2,$3,$4,'class.kim',$5,$6)`,
		char[:], account[:], name, name, level,
		time.Now().UTC().Add(-60*24*time.Hour)); err != nil {
		t.Fatalf("character: %v", err)
	}
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO character_inventories (character_id, capacity) VALUES ($1,60)`, char[:]); err != nil {
		t.Fatalf("inventory: %v", err)
	}
	return char, account
}

// mkGuild seeds a guild + leader membership + progression row.
func mkGuild(t *testing.T, name string, leader id.UUID, level int, at time.Time) id.UUID {
	t.Helper()
	ctx := context.Background()
	gid := id.NewV4()
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO guilds (guild_id, name, name_key, state, recruitment_mode,
		 leader_character_id, motd, guild_revision, guild_storage_revision,
		 guild_cosmetic_revision, created_at)
		 VALUES ($1,$2,$3,'ACTIVE','CLOSED',$4,'',0,0,0,$5)`,
		gid[:], name, name, leader[:], at); err != nil {
		t.Fatalf("guild row: %v", err)
	}
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO guild_progression (guild_id, guild_exp, guild_level, ritual_streak, revision)
		 VALUES ($1,0,$2,0,0)`, gid[:], level); err != nil {
		t.Fatalf("progression: %v", err)
	}
	mkMember(t, gid, leader, guild.RoleLeader, at)
	return gid
}

// mkMember adds one membership row.
func mkMember(t *testing.T, guildID, char id.UUID, role string, joinedAt time.Time) id.UUID {
	t.Helper()
	memID := id.NewV4()
	ctx := context.Background()
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO guild_membership_history (membership_id, guild_id, character_id, joined_at)
		 VALUES ($1,$2,$3,$4)`, memID[:], guildID[:], char[:], joinedAt); err != nil {
		t.Fatalf("history: %v", err)
	}
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO guild_memberships (character_id, guild_id, role, joined_at, membership_id)
		 VALUES ($1,$2,$3,$4,$5)`, char[:], guildID[:], role, joinedAt, memID[:]); err != nil {
		t.Fatalf("membership: %v", err)
	}
	return memID
}

func dropMember(t *testing.T, char id.UUID) {
	t.Helper()
	if _, err := sharedPool.Exec(context.Background(),
		`DELETE FROM guild_memberships WHERE character_id=$1`, char[:]); err != nil {
		t.Fatalf("drop member: %v", err)
	}
	if _, err := sharedPool.Exec(context.Background(),
		`DELETE FROM guild_membership_history WHERE character_id=$1`, char[:]); err != nil {
		t.Fatalf("drop history: %v", err)
	}
}

// mkItem seeds one UNBOUND instance in the character's inventory.
func mkItem(t *testing.T, charID id.UUID, itemID string, qty, slot int) id.UUID {
	t.Helper()
	inst := id.NewV4()
	ctx := context.Background()
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO item_instances
		 (item_instance_id, item_id, quantity, effective_binding, enhancement_level,
		  item_state, content_revision, created_at)
		 VALUES ($1,$2,$3,'UNBOUND',0,'{}'::jsonb,NULL,NOW())`,
		inst, itemID, qty); err != nil {
		t.Fatalf("instance: %v", err)
	}
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO item_locations (item_instance_id, location_kind, character_id, slot, updated_at)
		 VALUES ($1,'CHARACTER_INVENTORY',$2,$3,NOW())`,
		inst, charID, fmt.Sprintf("inv.%d", slot)); err != nil {
		t.Fatalf("location: %v", err)
	}
	return inst
}

// mkStorageItem seeds one instance directly inside guild storage.
func mkStorageItem(t *testing.T, guildID id.UUID, section, itemID string, qty,
	slot int, depositor, depAcct id.UUID) id.UUID {
	t.Helper()
	inst := id.NewV4()
	ctx := context.Background()
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO item_instances
		 (item_instance_id, item_id, quantity, effective_binding, enhancement_level,
		  item_state, content_revision, created_at)
		 VALUES ($1,$2,$3,'UNBOUND',0,'{}'::jsonb,NULL,NOW())`,
		inst, itemID, qty); err != nil {
		t.Fatalf("instance: %v", err)
	}
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO item_locations
		 (item_instance_id, location_kind, guild_id, slot,
		  depositor_character_id, depositor_account_id, updated_at)
		 VALUES ($1,'GUILD_STORAGE',$2,$3,$4,$5,NOW())`,
		inst, guildID, fmt.Sprintf("%s.%d", section, slot), depositor, depAcct); err != nil {
		t.Fatalf("location: %v", err)
	}
	return inst
}

// deps builds executor deps with a fixed clock.
func deps(now time.Time) Deps {
	return Deps{
		Store: New(sharedPool),
		Guild: guild.NewStore(sharedPool),
		Items: items.New(sharedPool),
		Inv:   inventory.NewStore(sharedPool),
		Now:   func() time.Time { return now },
	}
}

// runExec runs one executor inside a tx and decodes the JournalOutcome.
func runExec(t *testing.T, ex func(context.Context, pgx.Tx, *journalv1.DurableCommandRecord) (idempotency.Outcome, error),
	rec *journalv1.DurableCommandRecord) *journalv1.JournalOutcome {
	t.Helper()
	ctx := context.Background()
	var out *journalv1.JournalOutcome
	if err := pgx.BeginTxFunc(ctx, sharedPool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		o, err := ex(ctx, tx, rec)
		if err != nil {
			return err
		}
		out = &journalv1.JournalOutcome{}
		return protojson.Unmarshal(o.Payload, out)
	}); err != nil {
		t.Fatalf("exec: %v", err)
	}
	return out
}

func wantStatus(t *testing.T, out *journalv1.JournalOutcome, want protocolv1.ResultStatus) {
	t.Helper()
	r := out.GetS2CGuildResult()
	if r == nil {
		t.Fatalf("no s2c_guild_result in outcome")
	}
	if r.GetResult().GetStatus() != want {
		t.Fatalf("status = %v (%v), want %v", r.GetResult().GetStatus(),
			r.GetResult().GetErrorCode(), want)
	}
}

func wantError(t *testing.T, out *journalv1.JournalOutcome, want protocolv1.ErrorCode) {
	t.Helper()
	r := out.GetS2CGuildResult()
	if r == nil {
		t.Fatalf("no s2c_guild_result in outcome")
	}
	if r.GetResult().GetErrorCode() != want {
		t.Fatalf("error_code = %v, want %v", r.GetResult().GetErrorCode(), want)
	}
}

// --- query helpers ---------------------------------------------------------

func storageCount(t *testing.T, guildID id.UUID, section string) int {
	t.Helper()
	var n int
	if err := sharedPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM item_locations
		  WHERE guild_id=$1 AND location_kind='GUILD_STORAGE' AND slot LIKE $2`,
		guildID, section+".%").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func claimState(t *testing.T, claimID id.UUID) string {
	t.Helper()
	var s string
	if err := sharedPool.QueryRow(context.Background(),
		`SELECT state FROM guild_storage_claims WHERE claim_id=$1`, claimID).Scan(&s); err != nil {
		t.Fatalf("claim state: %v", err)
	}
	return s
}

func auditCount(t *testing.T, guildID id.UUID, action string) int {
	t.Helper()
	var n int
	if err := sharedPool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM guild_storage_audit WHERE guild_id=$1 AND action=$2`,
		guildID, action).Scan(&n); err != nil {
		t.Fatalf("audit: %v", err)
	}
	return n
}

func partnerCount(t *testing.T, char, partner id.UUID) int64 {
	t.Helper()
	var n int64
	if err := sharedPool.QueryRow(context.Background(),
		`SELECT COALESCE((item_partner_counts->>$2)::bigint,0)
		   FROM economy_character_daily_rollups
		  WHERE character_id=$1 AND utc_day=$3`,
		char, partner.String(), utcDayStart(now)).Scan(&n); err != nil {
		t.Fatalf("partner counts: %v", err)
	}
	return n
}

func storageRevision(t *testing.T, guildID id.UUID) uint64 {
	t.Helper()
	var rev uint64
	if err := sharedPool.QueryRow(context.Background(),
		`SELECT guild_storage_revision FROM guilds WHERE guild_id=$1`, guildID).Scan(&rev); err != nil {
		t.Fatalf("revision: %v", err)
	}
	return rev
}

func itemLocation(t *testing.T, instanceID id.UUID) string {
	t.Helper()
	var k string
	if err := sharedPool.QueryRow(context.Background(),
		`SELECT location_kind FROM item_locations WHERE item_instance_id=$1`,
		instanceID).Scan(&k); err != nil {
		t.Fatalf("location: %v", err)
	}
	return k
}
