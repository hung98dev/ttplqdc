package currency

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/schema"
	"thinhthan/internal/testing/pgtest"
)

// sharedDSN is one baseline-applied database per package run (schema
// setup dominates; tests isolate by fresh random account/character IDs).
var sharedDSN string

// setupErr records a provisioning failure on an existing postgres path —
// distinct from "no postgres path" (pgtest.ErrUnavailable): the former
// fails tests loudly, only the latter defers local-missing.
var setupErr error

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
	name := fmt.Sprintf("currency_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
	dsn, cleanup, err := srv.NewDB(ctx, name)
	if err != nil {
		setupErr = err
		os.Exit(m.Run())
	}
	if err := schema.Migrate(ctx, dsn, schema.MigrationsDir(repoRoot()), "up"); err != nil {
		setupErr = err
	} else {
		sharedDSN = dsn
	}
	code := m.Run()
	cleanup()
	srv.Close()
	os.Exit(code)
}

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if sharedDSN == "" {
		if setupErr != nil {
			t.Fatalf("postgres provisioning failed: %v", setupErr)
		}
		t.Skip("DEFERRED(local-missing): no postgres")
	}
	pool, err := pgxpool.New(context.Background(), sharedDSN)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func seedAccount(t *testing.T, pool *pgxpool.Pool, acct id.UUID) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO accounts (account_id) VALUES ($1)`, acct.String()); err != nil {
		t.Fatalf("seed account: %v", err)
	}
}

func seedCharacter(t *testing.T, pool *pgxpool.Pool, acct, char id.UUID) {
	t.Helper()
	key := "curchar" + char.String()[:12]
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id)
		 VALUES ($1,$2,$3,$4,'class.kim')`,
		char.String(), acct.String(), key, key); err != nil {
		t.Fatalf("seed character: %v", err)
	}
}

func seedBalance(t *testing.T, pool *pgxpool.Pool, char id.UUID, cur ID, balance int64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO character_currencies (character_id, currency_id, balance)
		 VALUES ($1,$2,$3)`, char.String(), string(cur), balance); err != nil {
		t.Fatalf("seed balance: %v", err)
	}
}

// balanceOf returns (balance, rowExists).
func balanceOf(t *testing.T, pool *pgxpool.Pool, char id.UUID, cur ID) (int64, bool) {
	t.Helper()
	var bal int64
	err := pool.QueryRow(context.Background(),
		`SELECT balance FROM character_currencies
		 WHERE character_id=$1 AND currency_id=$2`,
		char.String(), string(cur)).Scan(&bal)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		t.Fatalf("balanceOf: %v", err)
	}
	return bal, true
}

func auditCount(t *testing.T, pool *pgxpool.Pool, opID id.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM audit_events WHERE operation_id=$1`, opID.String()).Scan(&n); err != nil {
		t.Fatalf("auditCount: %v", err)
	}
	return n
}

// jsonEqual compares JSONB outcome payloads semantically (key order and
// spacing are not preserved by JSONB storage).
func jsonEqual(a, b []byte) bool {
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		return false
	}
	ab, _ := json.Marshal(av)
	bb, _ := json.Marshal(bv)
	return string(ab) == string(bb)
}

func fp(parts ...string) [32]byte {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0x00})
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

type fixture struct {
	pool  *pgxpool.Pool
	acct1 id.UUID
	acct2 id.UUID
	char1 id.UUID // acct1
	char2 id.UUID // acct1 (same-account partner)
	char3 id.UUID // acct2 (cross-account target)
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := newPool(t)
	f := fixture{
		pool:  pool,
		acct1: id.NewV4(),
		acct2: id.NewV4(),
		char1: id.NewV4(),
		char2: id.NewV4(),
		char3: id.NewV4(),
	}
	seedAccount(t, pool, f.acct1)
	seedAccount(t, pool, f.acct2)
	seedCharacter(t, pool, f.acct1, f.char1)
	seedCharacter(t, pool, f.acct1, f.char2)
	seedCharacter(t, pool, f.acct2, f.char3)
	return f
}

func inTx(t *testing.T, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	t.Helper()
	return pgx.BeginTxFunc(context.Background(), pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		return fn(tx)
	})
}

// TestAtomicDebitCredit covers paired settlement semantics: debit and
// credit commit or neither does; cross-account common transfers settle
// both legs atomically; a committed operation retries to the identical
// outcome; a failing second leg rolls the whole transfer back.
func TestAtomicDebitCredit(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	store := idempotency.NewStore(f.pool)
	opID := id.NewV7(time.Now())
	fingerprint := fp("transfer", f.char1.String(), f.char3.String(), "400")

	seedBalance(t, f.pool, f.char1, Common, 1000)

	var res TransferResult
	out, err := store.Execute(ctx, "currency.transfer",
		idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: f.char1},
		opID, fingerprint, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
			var err error
			res, err = DoTransfer(ctx, tx, Transfer{
				FromCharacterID: f.char1,
				ToCharacterID:   f.char3,
				CurrencyID:      Common,
				Amount:          400,
				OperationID:     opID,
				ReasonCode:      "TRADE_SETTLEMENT",
				SourceRef:       "trade.test.1",
				Actor:           ActorPlayer,
			})
			if err != nil {
				return idempotency.Outcome{}, err
			}
			b, _ := json.Marshal(res)
			return idempotency.Outcome{SchemaVersion: 1, Payload: b}, nil
		})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if res.From != (Result{Before: 1000, After: 600}) || res.To != (Result{Before: 0, After: 400}) {
		t.Fatalf("transfer result %+v", res)
	}
	if bal, _ := balanceOf(t, f.pool, f.char1, Common); bal != 600 {
		t.Fatalf("from balance %d", bal)
	}
	if bal, ok := balanceOf(t, f.pool, f.char3, Common); !ok || bal != 400 {
		t.Fatalf("to balance %d exists=%v", bal, ok)
	}
	if n := auditCount(t, f.pool, opID); n != 2 {
		t.Fatalf("audit rows %d, want 2", n)
	}

	// Retry with the same operation ID + fingerprint replays the
	// committed outcome without re-running the mutation.
	replayed := false
	out2, err := store.Execute(ctx, "currency.transfer",
		idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: f.char1},
		opID, fingerprint, func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
			replayed = true
			return idempotency.Outcome{}, errors.New("must not re-execute")
		})
	if err != nil || replayed {
		t.Fatalf("retry: replayed=%v err=%v", replayed, err)
	}
	if !jsonEqual(out.Payload, out2.Payload) {
		t.Fatalf("retry outcome %q != committed %q", out2.Payload, out.Payload)
	}
	if bal, _ := balanceOf(t, f.pool, f.char3, Common); bal != 400 {
		t.Fatalf("balance moved on retry: %d", bal)
	}

	// Mid-transfer failure (cap breach on the credit leg) rolls the
	// debit back — no partial settlement, no audit rows.
	if _, err := f.pool.Exec(ctx,
		`UPDATE character_currencies SET balance=$3 WHERE character_id=$1 AND currency_id=$2`,
		f.char3.String(), string(Common), Caps[Common]-100); err != nil {
		t.Fatalf("cap setup: %v", err)
	}
	badOp := id.NewV7(time.Now())
	_, err = store.Execute(ctx, "currency.transfer",
		idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: f.char1},
		badOp, fp("transfer", "capfail"), func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
			_, err := DoTransfer(ctx, tx, Transfer{
				FromCharacterID: f.char1,
				ToCharacterID:   f.char3,
				CurrencyID:      Common,
				Amount:          500,
				OperationID:     badOp,
				ReasonCode:      "TRADE_SETTLEMENT",
				SourceRef:       "trade.test.capfail",
				Actor:           ActorPlayer,
			})
			return idempotency.Outcome{}, err
		})
	if !errors.Is(err, ErrCapExceeded) {
		t.Fatalf("want ErrCapExceeded, got %v", err)
	}
	if bal, _ := balanceOf(t, f.pool, f.char1, Common); bal != 600 {
		t.Fatalf("debit survived a rolled-back transfer: %d", bal)
	}
	if n := auditCount(t, f.pool, badOp); n != 0 {
		t.Fatalf("audit rows %d on rolled-back transfer", n)
	}
}

// TestTransferSameAccountRejected: direct common transfer is
// cross-account only — same-account characters and the same character
// both reject; non-common currencies are non-transferable.
func TestTransferSameAccountRejected(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	seedBalance(t, f.pool, f.char1, Common, 1000)

	cases := []struct {
		name string
		tr   Transfer
		want error
	}{
		{"same account", Transfer{FromCharacterID: f.char1, ToCharacterID: f.char2, CurrencyID: Common,
			Amount: 10, OperationID: id.NewV7(time.Now()), ReasonCode: "TRADE_SETTLEMENT", Actor: ActorPlayer}, ErrSameAccountTransfer},
		{"same character", Transfer{FromCharacterID: f.char1, ToCharacterID: f.char1, CurrencyID: Common,
			Amount: 10, OperationID: id.NewV7(time.Now()), ReasonCode: "TRADE_SETTLEMENT", Actor: ActorPlayer}, ErrSameAccountTransfer},
		{"bound currency", Transfer{FromCharacterID: f.char1, ToCharacterID: f.char3, CurrencyID: Bound,
			Amount: 10, OperationID: id.NewV7(time.Now()), ReasonCode: "TRADE_SETTLEMENT", Actor: ActorPlayer}, ErrNonTransferable},
		{"special currency", Transfer{FromCharacterID: f.char1, ToCharacterID: f.char3, CurrencyID: Special,
			Amount: 10, OperationID: id.NewV7(time.Now()), ReasonCode: "TRADE_SETTLEMENT", Actor: ActorPlayer}, ErrNonTransferable},
		{"zero amount", Transfer{FromCharacterID: f.char1, ToCharacterID: f.char3, CurrencyID: Common,
			Amount: 0, OperationID: id.NewV7(time.Now()), ReasonCode: "TRADE_SETTLEMENT", Actor: ActorPlayer}, ErrInvalidTransfer},
	}
	for _, tc := range cases {
		err := inTx(t, f.pool, func(tx pgx.Tx) error {
			_, err := DoTransfer(ctx, tx, tc.tr)
			return err
		})
		if !errors.Is(err, tc.want) {
			t.Fatalf("%s: want %v, got %v", tc.name, tc.want, err)
		}
	}
	if bal, _ := balanceOf(t, f.pool, f.char1, Common); bal != 1000 {
		t.Fatalf("rejected transfers moved balance: %d", bal)
	}
}

// TestCurrencyCapsEnforcement: canonical caps per currency, whole-credit
// failure on overflow, overdraft rejection, unknown currency, zero delta.
func TestCurrencyCapsEnforcement(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	char := id.NewV4()
	seedCharacter(t, f.pool, f.acct1, char)

	for _, cur := range []ID{Common, Bound, Special} {
		cap := Caps[cur]
		m := func(delta int64) Mutation {
			return Mutation{CharacterID: char, CurrencyID: cur, Delta: delta,
				OperationID: id.NewV7(time.Now()), ReasonCode: "TEST", Actor: ActorSystem}
		}
		// Credit to cap-1 (first insert), then +2 overflows → whole
		// credit fails; +1 lands exactly on the cap.
		err := inTx(t, f.pool, func(tx pgx.Tx) error {
			_, err := Credit(ctx, tx, m(cap-1))
			return err
		})
		if err != nil {
			t.Fatalf("%s credit cap-1: %v", cur, err)
		}
		err = inTx(t, f.pool, func(tx pgx.Tx) error {
			_, err := Credit(ctx, tx, m(2))
			return err
		})
		if !errors.Is(err, ErrCapExceeded) {
			t.Fatalf("%s cap+1: want ErrCapExceeded, got %v", cur, err)
		}
		if bal, _ := balanceOf(t, f.pool, char, cur); bal != cap-1 {
			t.Fatalf("%s: balance moved on failed credit: %d", cur, bal)
		}
		if err := inTx(t, f.pool, func(tx pgx.Tx) error {
			_, err := Credit(ctx, tx, m(1))
			return err
		}); err != nil {
			t.Fatalf("%s credit to cap: %v", cur, err)
		}
		if bal, _ := balanceOf(t, f.pool, char, cur); bal != cap {
			t.Fatalf("%s: balance %d != cap %d", cur, bal, cap)
		}
		// Overdraft rejects and leaves the balance untouched.
		if err := inTx(t, f.pool, func(tx pgx.Tx) error {
			_, err := Debit(ctx, tx, m(cap+1))
			return err
		}); !errors.Is(err, ErrInsufficientBalance) {
			t.Fatalf("%s overdraft: want ErrInsufficientBalance, got %v", cur, err)
		}
		if bal, _ := balanceOf(t, f.pool, char, cur); bal != cap {
			t.Fatalf("%s: balance moved on failed debit: %d", cur, bal)
		}
	}

	// Debit of a currency the character has no row for.
	charNoRow := id.NewV4()
	seedCharacter(t, f.pool, f.acct1, charNoRow)
	err := inTx(t, f.pool, func(tx pgx.Tx) error {
		_, err := Debit(ctx, tx, Mutation{CharacterID: charNoRow, CurrencyID: Common, Delta: 1,
			OperationID: id.NewV7(time.Now()), ReasonCode: "TEST", Actor: ActorSystem})
		return err
	})
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("no-row debit: want ErrInsufficientBalance, got %v", err)
	}

	// Unknown currency and zero delta.
	err = inTx(t, f.pool, func(tx pgx.Tx) error {
		_, err := Apply(ctx, tx, Mutation{CharacterID: char, CurrencyID: "currency.fake", Delta: 5,
			OperationID: id.NewV7(time.Now()), ReasonCode: "TEST", Actor: ActorSystem})
		return err
	})
	if !errors.Is(err, ErrUnknownCurrency) {
		t.Fatalf("unknown currency: want ErrUnknownCurrency, got %v", err)
	}
	err = inTx(t, f.pool, func(tx pgx.Tx) error {
		_, err := Apply(ctx, tx, Mutation{CharacterID: char, CurrencyID: Common, Delta: 0,
			OperationID: id.NewV7(time.Now()), ReasonCode: "TEST", Actor: ActorSystem})
		return err
	})
	if !errors.Is(err, ErrInvalidDelta) {
		t.Fatalf("zero delta: want ErrInvalidDelta, got %v", err)
	}
}

// TestConcurrentDebitRollback: concurrent debits serialize on the row
// lock — exactly the debits that fit commit; the rest see the updated
// balance and fail without overspend.
func TestConcurrentDebitRollback(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	char := id.NewV4()
	seedCharacter(t, f.pool, f.acct1, char)
	seedBalance(t, f.pool, char, Common, 1000)

	const workers = 8
	const amount = 300 // exactly 3 fit: 3*300=900 <= 1000 < 4*300
	var succeeded, failed atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := inTx(t, f.pool, func(tx pgx.Tx) error {
				_, err := Debit(ctx, tx, Mutation{
					CharacterID: char, CurrencyID: Common, Delta: amount,
					OperationID: id.NewV7(time.Now()), ReasonCode: "TEST",
					Actor: ActorSystem})
				return err
			})
			switch {
			case err == nil:
				succeeded.Add(1)
			case errors.Is(err, ErrInsufficientBalance):
				failed.Add(1)
			default:
				failed.Add(1)
				t.Errorf("unexpected debit error: %v", err)
			}
		}()
	}
	wg.Wait()
	if got := succeeded.Load(); got != 3 {
		t.Fatalf("succeeded=%d, want 3", got)
	}
	if got := failed.Load(); got != workers-3 {
		t.Fatalf("failed=%d, want %d", got, workers-3)
	}
	if bal, _ := balanceOf(t, f.pool, char, Common); bal != 1000-3*amount {
		t.Fatalf("final balance %d, want %d", bal, 1000-3*amount)
	}
}

// TestAuditLogBalance: every committed mutation writes one audit_events
// row carrying the verbatim mutation fields.
func TestAuditLogBalance(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	store := idempotency.NewStore(f.pool)
	opID := id.NewV7(time.Now())

	seedBalance(t, f.pool, f.char1, Common, 500)
	_, err := store.Execute(ctx, "currency.debit",
		idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: f.char1},
		opID, fp("debit", "200"), func(ctx context.Context, tx pgx.Tx) (idempotency.Outcome, error) {
			res, err := Debit(ctx, tx, Mutation{
				CharacterID: f.char1, CurrencyID: Common, Delta: 200,
				OperationID: opID, ReasonCode: "NPC_SERVICE",
				SourceRef: "shop.smith.buy", Actor: ActorPlayer})
			if err != nil {
				return idempotency.Outcome{}, err
			}
			b, _ := json.Marshal(res)
			return idempotency.Outcome{SchemaVersion: 1, Payload: b}, nil
		})
	if err != nil {
		t.Fatalf("debit: %v", err)
	}

	var actor, action, subject string
	var payload []byte
	err = f.pool.QueryRow(ctx,
		`SELECT actor_kind, action, subject_character_id, payload
		 FROM audit_events WHERE operation_id=$1`, opID.String()).
		Scan(&actor, &action, &subject, &payload)
	if err != nil {
		t.Fatalf("audit row missing: %v", err)
	}
	if actor != string(ActorPlayer) || action != "NPC_SERVICE" || subject != f.char1.String() {
		t.Fatalf("audit header actor=%q action=%q subject=%q", actor, action, subject)
	}
	var p struct {
		CurrencyID string `json:"currency_id"`
		Delta      int64  `json:"delta"`
		Before     int64  `json:"balance_before"`
		After      int64  `json:"balance_after"`
		SourceRef  string `json:"source_reference"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if p.CurrencyID != string(Common) || p.Delta != -200 || p.Before != 500 || p.After != 300 ||
		p.SourceRef != "shop.smith.buy" {
		t.Fatalf("audit payload %+v", p)
	}
}
