package character

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/account"
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
	name := fmt.Sprintf("character_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
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

// runTx runs fn in a transaction: commit on nil error, rollback otherwise.
func runTx(t *testing.T, fn func(ctx context.Context, tx pgx.Tx) error) error {
	t.Helper()
	ctx := context.Background()
	tx, err := pool(t).Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(ctx, tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return nil
}

// n prefixes a test name with a unique run marker — name_key is globally
// unique across the shared test database.
func n(base string) string {
	return fmt.Sprintf("%s_%d", base, time.Now().UnixNano()%1_000_000_000)
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

// submitCreate drives the real durable path: build the record the edge
// builds, then TrustedReplay through the executor (admission + receipt +
// commit + retained outcome).
func submitCreate(t *testing.T, store *Store, idem *idempotency.Store,
	accountID id.UUID, name, classID string) *journalv1.JournalOutcome {
	t.Helper()
	ctx := context.Background()
	opID := id.NewV7(time.Now())
	req := &protocolv1.C2SCharacterCreate{
		OperationId:   opID[:],
		CharacterName: name,
		ClassId:       classID,
	}
	rec, err := CreateRecord(accountID, 1, req, time.Now())
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	var fp [32]byte
	copy(fp[:], rec.GetRequestFingerprint())
	out, err := idem.TrustedReplay(ctx, CreateFamily,
		idempotency.Owner{Kind: idempotency.OwnerAccount, ID: accountID}, opID, fp,
		func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
			return store.CreateExecutor(ctx, tx, rec)
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

func TestMaxThreeCharactersPerAccount(t *testing.T) {
	store := NewStore(pool(t))
	idem := idempotency.NewStore(pool(t))
	acct := mkAccount(t)
	for i, name := range []string{n("One"), n("Two"), n("Three")} {
		out := submitCreate(t, store, idem, acct, name, "class.kim")
		if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
			t.Fatalf("create %d status %v", i, out.GetStatus())
		}
	}
	rows, err := store.ListByAccount(context.Background(), nil, acct)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != MaxPerAccount {
		t.Fatalf("rows %d, want %d", len(rows), MaxPerAccount)
	}
}

func TestFourthCharacterRejected(t *testing.T) {
	store := NewStore(pool(t))
	idem := idempotency.NewStore(pool(t))
	acct := mkAccount(t)
	for _, name := range []string{n("One"), n("Two"), n("Three")} {
		submitCreate(t, store, idem, acct, name, "class.kim")
	}
	out := submitCreate(t, store, idem, acct, n("Four"), "class.kim")
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_ERROR ||
		out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_CHARACTER_SLOTS_FULL {
		t.Fatalf("4th create outcome %v %v", out.GetStatus(), out.GetErrorCode())
	}
	rows, err := store.ListByAccount(context.Background(), nil, acct)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != MaxPerAccount {
		t.Fatalf("rows %d after rejection", len(rows))
	}
}

// Characters are permanent: there is no delete path on the store, and
// the account FK RESTRICT refuses removing an owning account — the
// row outlives any account-level deletion attempt (character.md).
func TestCharacterPermanenceNoDeletion(t *testing.T) {
	store := NewStore(pool(t))
	idem := idempotency.NewStore(pool(t))
	acct := mkAccount(t)
	out := submitCreate(t, store, idem, acct, n("Perm"), "class.moc")
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("create: %v", out.GetStatus())
	}
	var charID id.UUID
	copy(charID[:], out.GetCreatedIds()[0].GetId())
	if _, err := pool(t).Exec(context.Background(),
		`DELETE FROM accounts WHERE account_id = $1`, acct.String()); err == nil {
		t.Fatal("account delete with owned character succeeded")
	}
	if _, err := store.Get(context.Background(), nil, charID); err != nil {
		t.Fatalf("character row missing after delete attempt: %v", err)
	}
}

func TestNormalizedNameKeyUniqueness(t *testing.T) {
	store := NewStore(pool(t))
	idem := idempotency.NewStore(pool(t))
	acct1, acct2, acct3 := mkAccount(t), mkAccount(t), mkAccount(t)
	base := n("Alice")
	out := submitCreate(t, store, idem, acct1, base, "class.kim")
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("create: %v", out.GetStatus())
	}
	// Folded key collision: different case resolves to the same name_key.
	out = submitCreate(t, store, idem, acct2, strings.ToUpper(base), "class.moc")
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_CHARACTER_NAME_TAKEN {
		t.Fatalf("fold collision code %v", out.GetErrorCode())
	}
	// Canonically equivalent NFD input resolves to the same key.
	out = submitCreate(t, store, idem, acct3, "Nguye\u0302\u0303n", "class.thuy")
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("nfd create: %v %v", out.GetStatus(), out.GetErrorCode())
	}
	out = submitCreate(t, store, idem, acct1, "Nguyễn", "class.hoa")
	if out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_CHARACTER_NAME_TAKEN {
		t.Fatalf("nfc collision code %v", out.GetErrorCode())
	}
}

func TestUpdatedAtMaintained(t *testing.T) {
	store := NewStore(pool(t))
	idem := idempotency.NewStore(pool(t))
	acct := mkAccount(t)
	out := submitCreate(t, store, idem, acct, n("Keeper"), "class.tho")
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		t.Fatalf("create: %v", out.GetStatus())
	}
	var charID id.UUID
	copy(charID[:], out.GetCreatedIds()[0].GetId())
	before, err := store.Get(context.Background(), nil, charID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	err = runTx(t, func(ctx context.Context, tx pgx.Tx) error {
		return store.UpdateProgression(ctx, tx, charID, 5, 1200, 2, 1)
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	after, err := store.Get(context.Background(), nil, charID)
	if err != nil {
		t.Fatalf("get after: %v", err)
	}
	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("updated_at %v did not advance past %v", after.UpdatedAt, before.UpdatedAt)
	}
	if !after.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("created_at changed %v -> %v", before.CreatedAt, after.CreatedAt)
	}
	if after.Level != 5 || after.MapID != before.MapID {
		t.Fatalf("unexpected row mutation %+v", after)
	}
}

func TestSuspendedAccountCannotCreate(t *testing.T) {
	store := NewStore(pool(t))
	idem := idempotency.NewStore(pool(t))
	acct := mkAccount(t)
	if _, err := pool(t).Exec(context.Background(),
		`UPDATE accounts SET status = $2 WHERE account_id = $1`,
		acct.String(), account.StatusSuspendedReconciliation); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	out := submitCreate(t, store, idem, acct, n("Frost"), "class.thuy")
	if out.GetStatus() != protocolv1.ResultStatus_RESULT_STATUS_ERROR ||
		out.GetErrorCode() != protocolv1.ErrorCode_ERROR_CODE_ACCOUNT_SUSPENDED {
		t.Fatalf("suspended create outcome %v %v", out.GetStatus(), out.GetErrorCode())
	}
	rows, err := store.ListByAccount(context.Background(), nil, acct)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("suspended account owns %d rows", len(rows))
	}
}
