package guild

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
	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
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
	name := fmt.Sprintf("guild_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
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
		"guild_blessing_votes", "guild_ritual_cycle_members", "guild_ritual_cycles",
		"guild_progression", "guild_member_contributions", "guild_invites",
		"guild_applications", "guild_memberships", "guild_membership_history",
		"guild_cosmetic_selections", "guild_cosmetic_entitlements",
		"guild_storage_claims", "guilds", "character_attach_events",
		"character_activity", "audit_events", "character_currencies",
		"characters", "accounts",
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
	return char, account
}

// mkFunds gives a character a currency.common balance.
func mkFunds(t *testing.T, char id.UUID, balance int64) {
	t.Helper()
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO character_currencies (character_id, currency_id, balance)
		 VALUES ($1,'currency.common',$2)`, char[:], balance); err != nil {
		t.Fatalf("funds: %v", err)
	}
}

// attach records a real attach event + refresh activity.
func attach(t *testing.T, char id.UUID, at time.Time) {
	t.Helper()
	ctx := context.Background()
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO character_attach_events (character_id, session_epoch, attached_at)
		 VALUES ($1,$2,$3) ON CONFLICT (character_id, session_epoch) DO NOTHING`,
		char[:], time.Now().UnixNano(), at); err != nil {
		t.Fatalf("attach event: %v", err)
	}
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO character_activity (character_id, last_attached_at, last_detached_at, session_active)
		 VALUES ($1,$2,$2,true)
		 ON CONFLICT (character_id) DO UPDATE
		 SET last_attached_at = EXCLUDED.last_attached_at,
		     last_detached_at = EXCLUDED.last_detached_at, session_active = true`,
		char[:], at); err != nil {
		t.Fatalf("activity: %v", err)
	}
}

func detach(t *testing.T, char id.UUID, at time.Time) {
	t.Helper()
	if _, err := sharedPool.Exec(context.Background(),
		`UPDATE character_activity SET last_detached_at = $2, session_active = false
		 WHERE character_id = $1`, char[:], at); err != nil {
		t.Fatalf("detach: %v", err)
	}
}

// deps builds executor deps with a fixed clock.
func deps(now time.Time, guard WarRegistrationGuard) Deps {
	return Deps{
		Store: NewStore(sharedPool),
		Now:   func() time.Time { return now },
		Guard: guard,
	}
}

// runTx executes fn inside a real transaction.
func runTx(t *testing.T, fn func(tx pgx.Tx) error) {
	t.Helper()
	ctx := context.Background()
	if err := pgx.BeginTxFunc(ctx, sharedPool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return fn(tx)
	}); err != nil {
		t.Fatalf("tx: %v", err)
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

// result decodes the embedded 649.
func result(t *testing.T, out *journalv1.JournalOutcome) *protocolv1.S2CGuildResult {
	t.Helper()
	r := out.GetS2CGuildResult()
	if r == nil {
		t.Fatalf("no s2c_guild_result in outcome")
	}
	return r
}

func wantStatus(t *testing.T, out *journalv1.JournalOutcome, want protocolv1.ResultStatus) {
	t.Helper()
	r := result(t, out)
	if r.GetResult().GetStatus() != want {
		t.Fatalf("status = %v (%v), want %v", r.GetResult().GetStatus(),
			r.GetResult().GetErrorCode(), want)
	}
}

func wantError(t *testing.T, out *journalv1.JournalOutcome, want protocolv1.ErrorCode) {
	t.Helper()
	r := result(t, out)
	if r.GetResult().GetErrorCode() != want {
		t.Fatalf("error_code = %v, want %v", r.GetResult().GetErrorCode(), want)
	}
}

// recFor builds a client.<ID> record straight through the record
// builder (mirrors what edge enqueues).
func recFor(t *testing.T, family string, char, account id.UUID, req proto.Message,
	opID id.UUID, at time.Time) *journalv1.DurableCommandRecord {
	t.Helper()
	rec, err := ClientRecord(family, account, char, 1, 1, req, at)
	if err != nil {
		t.Fatalf("ClientRecord: %v", err)
	}
	return rec
}

// guildRow reads guilds for assertions.
func guildRow(t *testing.T, guildID id.UUID) GuildRow {
	t.Helper()
	s := NewStore(sharedPool)
	g, err := s.Guild(context.Background(), sharedPool, guildID)
	if err != nil {
		t.Fatalf("guild: %v", err)
	}
	return g
}

// memberRow reads guild_memberships for assertions.
func memberRow(t *testing.T, char id.UUID) *MemberRow {
	t.Helper()
	s := NewStore(sharedPool)
	m, err := s.Member(context.Background(), sharedPool, char)
	if err != nil {
		t.Fatalf("member: %v", err)
	}
	return m
}

// mkGuild seeds a guild + leader member + progression row directly
// (cheaper than driving 637 when the test is not about creation).
func mkGuild(t *testing.T, name string, leader id.UUID, level int, exp int64, at time.Time) id.UUID {
	t.Helper()
	ctx := context.Background()
	gid := id.NewV4()
	_, key, err := NormalizeGuildName(name)
	if err != nil {
		t.Fatalf("seed name: %v", err)
	}
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO guilds (guild_id, name, name_key, state, recruitment_mode,
		 leader_character_id, motd, guild_revision, guild_storage_revision,
		 guild_cosmetic_revision, created_at)
		 VALUES ($1,$2,$3,'ACTIVE','CLOSED',$4,'',0,0,0,$5)`,
		gid[:], name, key, leader[:], at); err != nil {
		t.Fatalf("guild row: %v", err)
	}
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO guild_progression (guild_id, guild_exp, guild_level, ritual_streak, revision)
		 VALUES ($1,$2,$3,0,0)`, gid[:], exp, level); err != nil {
		t.Fatalf("progression: %v", err)
	}
	return gid
}

// mkMember adds one member row + history interval.
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

// --- request builders -----------------------------------------------------

func createReq(t *testing.T, name string) *protocolv1.C2SGuildCreate {
	t.Helper()
	op := id.NewV7(time.Now().UTC())
	return &protocolv1.C2SGuildCreate{OperationId: op[:], GuildName: name}
}

func disbandReq(t *testing.T) *protocolv1.C2SGuildDisband {
	t.Helper()
	op := id.NewV7(time.Now().UTC())
	return &protocolv1.C2SGuildDisband{OperationId: op[:]}
}

func inviteReq(t *testing.T, target id.UUID) *protocolv1.C2SGuildInvite {
	t.Helper()
	op := id.NewV7(time.Now().UTC())
	return &protocolv1.C2SGuildInvite{OperationId: op[:], TargetCharacterId: target[:]}
}

func acceptReq(t *testing.T, guildID id.UUID) *protocolv1.C2SGuildAccept {
	t.Helper()
	op := id.NewV7(time.Now().UTC())
	return &protocolv1.C2SGuildAccept{OperationId: op[:], GuildId: guildID[:]}
}

func inviteCancelReq(t *testing.T, target id.UUID) *protocolv1.C2SGuildInviteCancel {
	t.Helper()
	op := id.NewV7(time.Now().UTC())
	return &protocolv1.C2SGuildInviteCancel{OperationId: op[:], TargetCharacterId: target[:]}
}

func applyReq(t *testing.T, guildID id.UUID) *protocolv1.C2SGuildApply {
	t.Helper()
	op := id.NewV7(time.Now().UTC())
	return &protocolv1.C2SGuildApply{OperationId: op[:], GuildId: guildID[:]}
}

func applyDecideReq(t *testing.T, applicant id.UUID, accept bool) *protocolv1.C2SGuildApplicationDecide {
	t.Helper()
	op := id.NewV7(time.Now().UTC())
	dec := protocolv1.GuildApplicationDecision_GUILD_APPLICATION_DECISION_REJECT
	if accept {
		dec = protocolv1.GuildApplicationDecision_GUILD_APPLICATION_DECISION_ACCEPT
	}
	return &protocolv1.C2SGuildApplicationDecide{
		OperationId: op[:], ApplicantCharacterId: applicant[:], Decision: dec}
}

func applyCancelReq(t *testing.T, guildID id.UUID) *protocolv1.C2SGuildApplicationCancel {
	t.Helper()
	op := id.NewV7(time.Now().UTC())
	return &protocolv1.C2SGuildApplicationCancel{OperationId: op[:], GuildId: guildID[:]}
}

func kickReq(t *testing.T, target id.UUID) *protocolv1.C2SGuildKick {
	t.Helper()
	op := id.NewV7(time.Now().UTC())
	return &protocolv1.C2SGuildKick{OperationId: op[:], TargetCharacterId: target[:]}
}

func roleReq(t *testing.T, target id.UUID, role string) *protocolv1.C2SGuildRoleUpdate {
	t.Helper()
	op := id.NewV7(time.Now().UTC())
	return &protocolv1.C2SGuildRoleUpdate{
		OperationId: op[:], TargetCharacterId: target[:], NewRole: role}
}

func transferReq(t *testing.T, target id.UUID) *protocolv1.C2SGuildLeaderTransfer {
	t.Helper()
	op := id.NewV7(time.Now().UTC())
	return &protocolv1.C2SGuildLeaderTransfer{OperationId: op[:], TargetCharacterId: target[:]}
}

func settingsReq(t *testing.T, mode protocolv1.GuildRecruitmentMode) *protocolv1.C2SGuildSettingsSet {
	t.Helper()
	op := id.NewV7(time.Now().UTC())
	return &protocolv1.C2SGuildSettingsSet{OperationId: op[:], RecruitmentMode: mode}
}

func statusSuccess() protocolv1.ResultStatus {
	return protocolv1.ResultStatus_RESULT_STATUS_SUCCESS
}

func errorTaken() protocolv1.ErrorCode {
	return protocolv1.ErrorCode_ERROR_CODE_GUILD_NAME_TAKEN
}

// --- tx/test scaffolding ---------------------------------------------------

// pgxTx aliases pgx.Tx so runTx closures read naturally.
type pgxTx = pgx.Tx

func testCtx() context.Context { return context.Background() }

// mkGuildRowMode flips a seeded guild's recruitment mode.
func mkGuildRowMode(t *testing.T, guildID id.UUID, mode string) {
	t.Helper()
	if _, err := sharedPool.Exec(context.Background(),
		`UPDATE guilds SET recruitment_mode = $2 WHERE guild_id = $1`,
		guildID[:], mode); err != nil {
		t.Fatalf("mode: %v", err)
	}
}

func v4b() []byte { u := id.NewV4(); return u[:] }
