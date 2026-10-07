package progression

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
	"thinhthan/internal/durable/currency"
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

func repoRoot() string {
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
	name := fmt.Sprintf("progression_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
	dsn, cleanup, err := srv.NewDB(ctx, name)
	if err != nil {
		setupErr = err
		os.Exit(m.Run())
	}
	if err := schema.Migrate(ctx, dsn, schema.MigrationsDir(repoRoot()), "up"); err != nil {
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
	cleanup()
	srv.Close()
	os.Exit(code)
}

func pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if sharedPool == nil {
		if setupErr != nil {
			t.Fatalf("postgres provisioning failed: %v", setupErr)
		}
		t.Fatal("postgres pool unavailable")
	}
	return sharedPool
}

func mkAccount(t *testing.T) id.UUID {
	t.Helper()
	acct := id.NewV4()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO accounts (account_id) VALUES ($1)`, acct.String()); err != nil {
		t.Fatalf("account: %v", err)
	}
	return acct
}

// mkCharacter seeds one characters row with explicit progression columns.
func mkCharacter(t *testing.T, acct id.UUID, classID string, level int32,
	skillPts, potPts int32) id.UUID {
	t.Helper()
	char := id.NewV4()
	key := "progchar" + char.String()[:12]
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO characters
		 (character_id, account_id, name, name_key, class_id, level,
		  unspent_skill_points, unspent_potential_points)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		char.String(), acct.String(), key, key, classID, level,
		skillPts, potPts); err != nil {
		t.Fatalf("character: %v", err)
	}
	return char
}

func seedSkill(t *testing.T, char id.UUID, skillID string, level int32) {
	t.Helper()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_skill_levels (character_id, skill_id, level)
		 VALUES ($1,$2,$3)`, char.String(), skillID, level); err != nil {
		t.Fatalf("seed skill: %v", err)
	}
}

func seedAlloc(t *testing.T, char id.UUID, potentialID string, points int32) {
	t.Helper()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_potential_allocations
		 (character_id, potential_id, allocated_points) VALUES ($1,$2,$3)`,
		char.String(), potentialID, points); err != nil {
		t.Fatalf("seed alloc: %v", err)
	}
}

func seedBalance(t *testing.T, char id.UUID, balance int64) {
	t.Helper()
	if _, err := pool(t).Exec(context.Background(),
		`INSERT INTO character_currencies (character_id, currency_id, balance)
		 VALUES ($1,$2,$3)`, char.String(), string(currency.Common), balance); err != nil {
		t.Fatalf("seed balance: %v", err)
	}
}

func charCols(t *testing.T, char id.UUID) (skillPts, potPts int32, rev uint64) {
	t.Helper()
	if err := pool(t).QueryRow(context.Background(),
		`SELECT unspent_skill_points, unspent_potential_points, progression_revision
		 FROM characters WHERE character_id = $1`, char.String()).
		Scan(&skillPts, &potPts, &rev); err != nil {
		t.Fatalf("charCols: %v", err)
	}
	return skillPts, potPts, rev
}

func skillLevel(t *testing.T, char id.UUID, skillID string) (int32, bool) {
	t.Helper()
	var lvl int32
	err := pool(t).QueryRow(context.Background(),
		`SELECT level FROM character_skill_levels
		 WHERE character_id=$1 AND skill_id=$2`, char.String(), skillID).Scan(&lvl)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		t.Fatalf("skillLevel: %v", err)
	}
	return lvl, true
}

func allocPoints(t *testing.T, char id.UUID, pid string) (int32, bool) {
	t.Helper()
	var pts int32
	err := pool(t).QueryRow(context.Background(),
		`SELECT allocated_points FROM character_potential_allocations
		 WHERE character_id=$1 AND potential_id=$2`, char.String(), pid).Scan(&pts)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		t.Fatalf("allocPoints: %v", err)
	}
	return pts, true
}

func balance(t *testing.T, char id.UUID) int64 {
	t.Helper()
	var bal int64
	err := pool(t).QueryRow(context.Background(),
		`SELECT balance FROM character_currencies
		 WHERE character_id=$1 AND currency_id='currency.common'`,
		char.String()).Scan(&bal)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0
	}
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	return bal
}

// opBytes mints a fresh UUIDv7 operation_id as a byte slice.
func opBytes() []byte {
	op := id.NewV7(time.Now())
	return op[:]
}

func spatial() *journalv1.JournalSource {
	return &journalv1.JournalSource{MapId: "map.lang_da.dinh_lang", OccurredAtMs: 1}
}

// replay drives the durable path the queue executor runs: TrustedReplay
// under the receipt lock invoking the exported family executor.
func replay(t *testing.T, store *Store, idem *idempotency.Store,
	rec *journalv1.DurableCommandRecord) *journalv1.JournalOutcome {
	t.Helper()
	ctx := context.Background()
	var opID id.UUID
	copy(opID[:], rec.GetOperationId())
	var charID id.UUID
	copy(charID[:], rec.GetOwnerId())
	var fp [32]byte
	copy(fp[:], rec.GetRequestFingerprint())
	ex := Executors(store)[rec.GetOperationFamily()]
	if ex == nil {
		t.Fatalf("no executor for family %q", rec.GetOperationFamily())
	}
	out, err := idem.TrustedReplay(ctx, rec.GetOperationFamily(),
		idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: charID}, opID, fp,
		func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
			return ex(ctx, tx, rec)
		})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	outcome := &journalv1.JournalOutcome{}
	if err := protojson.Unmarshal(out.Payload, outcome); err != nil {
		t.Fatalf("outcome decode: %v", err)
	}
	return outcome
}

func resultOf(outcome *journalv1.JournalOutcome) *protocolv1.S2CProgressionMutateResult {
	return outcome.GetS2CProgressionMutateResult()
}

func TestSkillUpgradeCommitAndRejections(t *testing.T) {
	store := NewStore(pool(t))
	idem := idempotency.NewStore(pool(t))
	acct := mkAccount(t)
	char := mkCharacter(t, acct, "class.kim", 25, 2, 0)
	const skill = "skill.kim.active.xuyen_phong"
	seedSkill(t, char, skill, 3)

	opID := id.NewV7(time.Now())
	rec, err := SkillUpgradeRecord(acct, char, 1, 7,
		&protocolv1.C2SSkillUpgrade{
			OperationId:   opID[:],
			SkillId:       skill,
			ExpectedLevel: 3,
		}, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	out := replay(t, store, idem, rec)
	res := resultOf(out)
	if res == nil || res.GetRequestMessageId() != 511 ||
		res.GetResult().GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("upgrade outcome: %+v", out)
	}
	if lvl, ok := skillLevel(t, char, skill); !ok || lvl != 4 {
		t.Fatalf("skill level %d ok=%v, want 4", lvl, ok)
	}
	if pts, _, rev := charCols(t, char); pts != 1 || rev != 1 {
		t.Fatalf("pts %d rev %d, want 1/1", pts, rev)
	}

	reject := func(req *protocolv1.C2SSkillUpgrade) protocolv1.ErrorCode {
		r, err := SkillUpgradeRecord(acct, char, 1, 7, req, time.Now())
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		o := replay(t, store, idem, r)
		return o.GetErrorCode()
	}
	if code := reject(&protocolv1.C2SSkillUpgrade{
		OperationId: opBytes(), SkillId: skill, ExpectedLevel: 99,
	}); code != protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT {
		t.Fatalf("expected_level mismatch code %v", code)
	}
	seedSkill(t, char, "skill.kim.passive.kiem_tam", 6)
	if code := reject(&protocolv1.C2SSkillUpgrade{
		OperationId: opBytes(), SkillId: "skill.kim.passive.kiem_tam",
	}); code != protocolv1.ErrorCode_ERROR_CODE_SKILL_MAX_LEVEL {
		t.Fatalf("passive max code %v", code)
	}
	// Level-25 char has not reached the Lv45 signature; a cross-class id
	// is never learnable.
	if code := reject(&protocolv1.C2SSkillUpgrade{
		OperationId: opBytes(), SkillId: "skill.kim.active.nhat_kiem_dinh_hon",
	}); code != protocolv1.ErrorCode_ERROR_CODE_SKILL_NOT_LEARNED {
		t.Fatalf("unlearned code %v", code)
	}
	if code := reject(&protocolv1.C2SSkillUpgrade{
		OperationId: opBytes(), SkillId: "skill.moc.basic.linh_diep",
	}); code != protocolv1.ErrorCode_ERROR_CODE_SKILL_NOT_LEARNED {
		t.Fatalf("cross-class code %v", code)
	}
	// An auto-learned skill with no persisted row upgrades from the
	// implicit level 1 (skills.md — milestone learning is automatic).
	op := id.NewV7(time.Now())
	rec, err = SkillUpgradeRecord(acct, char, 1, 7,
		&protocolv1.C2SSkillUpgrade{OperationId: op[:],
			SkillId: "skill.kim.basic.kiem_thuc"}, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	out = replay(t, store, idem, rec)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("implicit-learned upgrade outcome %v", out.GetErrorCode())
	}
	if lvl, ok := skillLevel(t, char, "skill.kim.basic.kiem_thuc"); !ok || lvl != 2 {
		t.Fatalf("implicit-learned level %d ok=%v, want 2", lvl, ok)
	}
	// Drain the last point, then verify insufficient.
	if _, err := pool(t).Exec(context.Background(),
		`UPDATE characters SET unspent_skill_points = 0 WHERE character_id = $1`,
		char.String()); err != nil {
		t.Fatal(err)
	}
	if code := reject(&protocolv1.C2SSkillUpgrade{
		OperationId: opBytes(), SkillId: skill,
	}); code != protocolv1.ErrorCode_ERROR_CODE_SKILL_POINTS_INSUFFICIENT {
		t.Fatalf("insufficient code %v", code)
	}
}

func TestAllocateAllOrNothingDurable(t *testing.T) {
	store := NewStore(pool(t))
	idem := idempotency.NewStore(pool(t))
	acct := mkAccount(t)
	// earned = 10 → per-stat cap = 6.
	char := mkCharacter(t, acct, "class.thuy", 20, 0, 10)

	alloc := func(d *protocolv1.PotentialDelta) *journalv1.JournalOutcome {
		op := id.NewV7(time.Now())
		rec, err := AllocateRecord(acct, char, 1, 7,
			&protocolv1.C2SPotentialAllocate{OperationId: op[:], Deltas: d},
			time.Now())
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		return replay(t, store, idem, rec)
	}

	out := alloc(&protocolv1.PotentialDelta{Str: 7})
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_POTENTIAL_CAP_EXCEEDED {
		t.Fatalf("cap code %v", out.GetErrorCode())
	}
	if _, ok := allocPoints(t, char, "STR"); ok {
		t.Fatal("cap-exceeded request left a row behind")
	}
	out = alloc(&protocolv1.PotentialDelta{Str: 6, Vit: 4})
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("alloc outcome %v", out.GetErrorCode())
	}
	if pts, _, rev := charCols(t, char); pts != 0 || rev != 1 {
		t.Fatalf("unspent %d rev %d, want 0/1", pts, rev)
	}
	out = alloc(&protocolv1.PotentialDelta{Str: 1})
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_POTENTIAL_POINTS_INSUFFICIENT {
		t.Fatalf("insufficient code %v", out.GetErrorCode())
	}
	if out.GetS2CProgressionMutateResult().GetRequestMessageId() != 512 {
		t.Fatalf("request_message_id %d", out.GetS2CProgressionMutateResult().GetRequestMessageId())
	}
}

func TestRespecAtomicDurable(t *testing.T) {
	store := NewStore(pool(t))
	idem := idempotency.NewStore(pool(t))
	acct := mkAccount(t)
	char := mkCharacter(t, acct, "class.moc", 30, 0, 0)
	const s1, s2 = "skill.moc.active.moc_bo", "skill.moc.passive.duoc_tinh"
	seedSkill(t, char, s1, 5)
	seedSkill(t, char, s2, 3)
	seedAlloc(t, char, "VIT", 10)
	seedAlloc(t, char, "AGI", 7)
	// Level 30 price = 25·30² = 22500 — one point short.
	seedBalance(t, char, 22499)

	respec := func(kind protocolv1.RespecKind) *journalv1.JournalOutcome {
		op := id.NewV7(time.Now())
		rec, err := RespecRecord(acct, char, 1, 7,
			&protocolv1.C2SRespec{OperationId: op[:], NpcId: "npc.lang_da.nguoi_dan_duong", Kind: kind},
			spatial(), time.Now())
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		return replay(t, store, idem, rec)
	}

	out := respec(protocolv1.RespecKind_RESPEC_KIND_SKILL)
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_CURRENCY {
		t.Fatalf("insufficient respec code %v", out.GetErrorCode())
	}
	if lvl, _ := skillLevel(t, char, s1); lvl != 5 {
		t.Fatalf("rejected respec mutated skill to %d", lvl)
	}
	if bal := balance(t, char); bal != 22499 {
		t.Fatalf("rejected respec debited balance to %d", bal)
	}
	if _, _, rev := charCols(t, char); rev != 0 {
		t.Fatalf("rejected respec bumped revision to %d", rev)
	}

	if _, err := pool(t).Exec(context.Background(),
		`UPDATE character_currencies SET balance = 50000
		 WHERE character_id=$1 AND currency_id='currency.common'`, char.String()); err != nil {
		t.Fatal(err)
	}
	out = respec(protocolv1.RespecKind_RESPEC_KIND_SKILL)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("skill respec outcome %v", out.GetErrorCode())
	}
	if lvl, _ := skillLevel(t, char, s1); lvl != 1 {
		t.Fatalf("s1 level %d, want 1", lvl)
	}
	if lvl, _ := skillLevel(t, char, s2); lvl != 1 {
		t.Fatalf("s2 level %d, want 1", lvl)
	}
	if pts, _, rev := charCols(t, char); pts != 6 || rev != 1 {
		t.Fatalf("refunded pts %d rev %d, want 6/1", pts, rev)
	}
	if bal := balance(t, char); bal != 50000-22500 {
		t.Fatalf("balance %d, want 27500", bal)
	}

	out = respec(protocolv1.RespecKind_RESPEC_KIND_POTENTIAL)
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("potential respec outcome %v", out.GetErrorCode())
	}
	if _, ok := allocPoints(t, char, "VIT"); ok {
		t.Fatal("VIT row not cleared by potential respec")
	}
	if _, pts, rev := charCols(t, char); pts != 17 || rev != 2 {
		t.Fatalf("potential refund %d rev %d, want 17/2", pts, rev)
	}
	if bal := balance(t, char); bal != 50000-2*22500 {
		t.Fatalf("balance %d, want 5000", bal)
	}
}

func TestProgressionOpsReplayIdempotent(t *testing.T) {
	store := NewStore(pool(t))
	idem := idempotency.NewStore(pool(t))
	acct := mkAccount(t)
	char := mkCharacter(t, acct, "class.kim", 10, 1, 0)
	const skill = "skill.kim.basic.kiem_thuc"
	seedSkill(t, char, skill, 1)

	opID := id.NewV7(time.Now())
	rec, err := SkillUpgradeRecord(acct, char, 1, 7,
		&protocolv1.C2SSkillUpgrade{OperationId: opID[:], SkillId: skill}, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	first := replay(t, store, idem, rec)
	second := replay(t, store, idem, rec)
	if first.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS ||
		second.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("replay statuses %v/%v", first.GetStatus(), second.GetStatus())
	}
	if lvl, _ := skillLevel(t, char, skill); lvl != 2 {
		t.Fatalf("skill level %d — replay double-applied", lvl)
	}
	if pts, _, rev := charCols(t, char); pts != 0 || rev != 1 {
		t.Fatalf("pts %d rev %d — replay double-charged", pts, rev)
	}
}

func TestRecordBuilderValidation(t *testing.T) {
	acct, char := id.NewV4(), id.NewV4()
	bad := make([]byte, 15)
	if _, err := SkillUpgradeRecord(acct, char, 1, 7,
		&protocolv1.C2SSkillUpgrade{OperationId: bad, SkillId: "skill.kim.basic.x"},
		time.Now()); !errors.Is(err, ErrMalformedRecord) {
		t.Fatalf("short opID err %v", err)
	}
	op := id.NewV7(time.Now())
	if _, err := RespecRecord(acct, char, 1, 7,
		&protocolv1.C2SRespec{OperationId: op[:], NpcId: "npc.x",
			Kind: protocolv1.RespecKind_RESPEC_KIND_SKILL},
		nil, time.Now()); !errors.Is(err, ErrMalformedRecord) {
		t.Fatalf("nil spatial err %v", err)
	}
	rec, err := AllocateRecord(acct, char, 1, 7,
		&protocolv1.C2SPotentialAllocate{OperationId: op[:],
			Deltas: &protocolv1.PotentialDelta{Str: 1}}, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if rec.GetOperationFamily() != AllocateFamily ||
		rec.GetClient().GetC2SPotentialAllocate() == nil ||
		len(rec.GetRequestFingerprint()) != 32 {
		t.Fatalf("record shape %+v", rec)
	}
	if rec.GetOwnerKind() != journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER ||
		rec.GetCommandType() != journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT {
		t.Fatalf("owner/type %+v", rec)
	}
}
