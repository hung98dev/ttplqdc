package lockorder

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/schema"
	"thinhthan/internal/testing/pgtest"
)

var sharedDSN string

func repoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd + "/../../../.."
}

// uuid builds a deterministic UUID from one byte value.
func uuid(b byte) id.UUID {
	var u id.UUID
	for i := range u {
		u[i] = b
	}
	return u
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	srv, err := pgtest.Ensure(ctx)
	switch {
	case errors.Is(err, pgtest.ErrUnavailable):
		fmt.Fprintf(os.Stderr, "pgtest ensure: %v\n", err)
		os.Exit(m.Run())
	case err != nil:
		fmt.Fprintf(os.Stderr, "pgtest ensure: %v\n", err)
		os.Exit(m.Run())
	}
	var rb [8]byte
	if _, rerr := crand.Read(rb[:]); rerr != nil {
		panic(rerr)
	}
	name := "lockorder_tests_" + hex.EncodeToString(rb[:])
	dsn, cleanup, err := srv.NewDB(ctx, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgtest newdb: %v\n", err)
		cleanup()
		srv.Close()
		os.Exit(m.Run())
	}
	if err := schema.Migrate(ctx, dsn, schema.MigrationsDir(repoRoot()), "up"); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
	} else {
		sharedDSN = dsn
	}
	code := m.Run()
	cleanup()
	srv.Close()
	os.Exit(code)
}

func TestCanonicalOrder(t *testing.T) {
	a, b, c := uuid(0x0a), uuid(0x0b), uuid(0x0c)
	locks := []Lock{
		RowLock("character_inventories", a),
		RowLock("characters", c),
		RowLock("character_currencies", b),
		RowLock("characters", a),
		RowLock("character_currencies", a),
		AdvisoryLock("durable_command_receipts", []byte{0x01}),
		RowLock("characters", b),
		RowLock("accounts", a),
	}
	if err := SortLocks(locks); err != nil {
		t.Fatalf("SortLocks: %v", err)
	}
	want := []struct {
		table string
		key   id.UUID
	}{
		{"accounts", a},
		{"characters", a},
		{"characters", b},
		{"characters", c},
		{"character_currencies", a},
		{"character_currencies", b},
		{"character_inventories", a},
	}
	if len(locks) != len(want)+1 {
		t.Fatalf("len(locks)=%d, want %d", len(locks), len(want)+1)
	}
	// receipt tier (2.5) sits after priority 2, before value aggregates.
	if locks[4].Table != "durable_command_receipts" {
		t.Fatalf("receipt lock misplaced: %+v", locks[4])
	}
	sorted := append(locks[:4:4], locks[5:]...)
	for i, w := range want {
		if sorted[i].Table != w.table {
			t.Fatalf("locks[%d].Table = %q, want %q", i, sorted[i].Table, w.table)
		}
		if string(sorted[i].Key) != string(w.key[:]) {
			t.Fatalf("locks[%d].Key = %x, want %x", i, sorted[i].Key, w.key)
		}
	}
	// within-priority listed order: item_instances before item_locations.
	dup := []Lock{RowLock("item_locations", a), RowLock("item_instances", b)}
	if err := SortLocks(dup); err != nil {
		t.Fatalf("SortLocks: %v", err)
	}
	if dup[0].Table != "item_instances" {
		t.Fatalf("within-priority order broken: %+v", dup)
	}
	// priority-18 marker-first exception is encoded by the table.
	p18 := []Lock{
		RowLockText("world_consequence_relics", "relic.x"),
		RowLockText("region_di_tich_markers", "region.y"),
	}
	if err := SortLocks(p18); err != nil {
		t.Fatalf("SortLocks: %v", err)
	}
	if p18[0].Table != "region_di_tich_markers" {
		t.Fatalf("p18 order broken: %+v", p18)
	}
	// guild-storage dual placement: item_locations at priority 13.
	gs := []Lock{RowLock("character_currencies", a), GuildStorageLock(b)}
	if err := SortLocks(gs); err != nil {
		t.Fatalf("SortLocks: %v", err)
	}
	if gs[0].Table != "character_currencies" || gs[1].priority != GuildStorage {
		t.Fatalf("guild-storage placement broken: %+v", gs)
	}
	// PriorityOf resolves first occurrence; item_locations = priority 5.
	if p, ok := PriorityOf("item_locations"); !ok || p != Items {
		t.Fatalf("PriorityOf(item_locations) = %d,%v want 50,true", p, ok)
	}
	if got := TablesAt(Characters); len(got) != 5 || got[0] != "characters" {
		t.Fatalf("TablesAt(Characters) = %v", got)
	}
}

func TestLockOrderMatchesDatabaseMd(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(), "docs", "06_data", "database.md"))
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	got, err := ParseDatabaseMd(string(raw))
	if err != nil {
		t.Fatalf("ParseDatabaseMd: %v", err)
	}
	if len(got) != len(CanonicalOrder) {
		t.Fatalf("parsed %d entries, registry has %d: %v", len(got), len(CanonicalOrder), got)
	}
	for i, e := range got {
		want := CanonicalOrder[i]
		if e.Priority != want.Priority {
			t.Fatalf("entry %d priority %d, want %d", i, e.Priority, want.Priority)
		}
		if len(e.Tables) != len(want.Tables) {
			t.Fatalf("priority %d parsed tables %v, want %v", e.Priority, e.Tables, want.Tables)
		}
		for j, tb := range e.Tables {
			if tb != want.Tables[j] {
				t.Fatalf("priority %d table %d = %q, want %q", e.Priority, j, tb, want.Tables[j])
			}
		}
	}
}

func TestReceiptBeforeValueAggregates(t *testing.T) {
	acct, c1, c2 := uuid(0xaa), uuid(0x01), uuid(0x02)
	op1, op2 := uuid(0x11), uuid(0x12)
	locks := AccountCharacterSet(acct, []id.UUID{c2, c1})
	locks = append(locks,
		ReceiptKeys(Receipt{Owner: c1, Family: "trade.send", Operation: op1},
			Receipt{Owner: c2, Family: "trade.send", Operation: op2})...,
	)
	locks = append(locks, RowLock("character_currencies", c1), RowLock("character_inventories", c2))
	if err := CheckOrder(locks); err != nil {
		t.Fatalf("helper output not canonical: %v", err)
	}
	// shuffle value-aggregate + receipt rows out of order and resort.
	mixed := append([]Lock{locks[len(locks)-1], locks[len(locks)-2]}, locks[:len(locks)-2]...)
	if err := SortLocks(mixed); err != nil {
		t.Fatalf("SortLocks: %v", err)
	}
	// every receipt lock must sit after all priority-2 and before p3+.
	var lastChar, firstReceipt, firstValue int = -1, -1, -1
	for i, l := range mixed {
		p, _ := l.effectivePriority()
		switch {
		case p <= Characters:
			lastChar = i
		case p == Receipts && firstReceipt < 0:
			firstReceipt = i
		case p >= Currencies && firstValue < 0:
			firstValue = i
		}
	}
	if !(lastChar < firstReceipt && firstReceipt < firstValue) {
		t.Fatalf("receipt placement wrong: lastChar=%d firstReceipt=%d firstValue=%d", lastChar, firstReceipt, firstValue)
	}
}

func TestMultiownerReceiptKeysSortedBeforeMutation(t *testing.T) {
	o1, o2 := uuid(0x01), uuid(0x02)
	opA, opB, opC := uuid(0xa0), uuid(0xb0), uuid(0xc0)
	locks := ReceiptKeys(
		Receipt{Owner: o2, Family: "inventory.mutate", Operation: opB},
		Receipt{Owner: o1, Family: "trade.send", Operation: opC},
		Receipt{Owner: o1, Family: "auction.buy", Operation: opB},
		Receipt{Owner: o1, Family: "auction.buy", Operation: opA},
		Receipt{Owner: o2, Family: "auction.buy", Operation: opA},
	)
	if len(locks) != 5 {
		t.Fatalf("ReceiptKeys produced %d locks", len(locks))
	}
	if err := CheckOrder(locks); err != nil {
		t.Fatalf("receipt keys not canonical: %v", err)
	}
	type triple struct {
		owner, family, op string
	}
	got := make([]triple, 0, len(locks))
	for _, l := range locks {
		// decode owner16 | family | 0x00 | op16
		k := l.Key
		got = append(got, triple{
			owner:  string(k[:16]),
			family: string(k[16 : len(k)-17]),
			op:     string(k[len(k)-16:]),
		})
	}
	want := []triple{
		{string(o1[:]), "auction.buy", string(opA[:])},
		{string(o1[:]), "auction.buy", string(opB[:])},
		{string(o1[:]), "trade.send", string(opC[:])},
		{string(o2[:]), "auction.buy", string(opA[:])},
		{string(o2[:]), "inventory.mutate", string(opB[:])},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("receipt %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestOutOfOrderRejected(t *testing.T) {
	a := uuid(0x01)
	locks := []Lock{
		RowLock("character_currencies", a),
		RowLock("characters", a),
	}
	err := Acquire(context.Background(), nil, locks...)
	var oo *OutOfOrderError
	if !errors.As(err, &oo) {
		t.Fatalf("Acquire out-of-order err = %v, want *OutOfOrderError", err)
	}
	if !strings.Contains(oo.Error(), "characters") || !strings.Contains(oo.Error(), "character_currencies") {
		t.Fatalf("OutOfOrderError content: %v", oo)
	}
	// Same-lock duplicate keys (equal order) are not an inversion.
	dup := []Lock{RowLock("characters", a), RowLock("characters", a)}
	if err := CheckOrder(dup); err != nil {
		t.Fatalf("CheckOrder duplicate: %v", err)
	}
	// Unknown tables reject before any statement.
	err = Acquire(context.Background(), nil, RowLock("nonexistent_table", a))
	if err == nil || errors.As(err, &oo) {
		t.Fatalf("unknown table err = %v, want non-OutOfOrder error", err)
	}
}

// TestNoDeadlockConcurrentTransfers runs 1,000 concurrent transfer-style
// transactions against seeded multi-owner rows on PostgreSQL 18.6 —
// every txn locks the full canonical set through Acquire, mutates, and
// inserts its operations row last. Any deadlock fails the test.
func TestNoDeadlockConcurrentTransfers(t *testing.T) {
	if sharedDSN == "" {
		t.Skip("DEFERRED(local-missing): no migrated test database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, sharedDSN)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	acct1, acct2 := uuid(0xa1), uuid(0xa2)
	chars := []id.UUID{uuid(0x31), uuid(0x32), uuid(0x33), uuid(0x34)}
	for i, a := range []id.UUID{acct1, acct2} {
		if _, err := pool.Exec(ctx, `INSERT INTO accounts (account_id) VALUES ($1)`, a.String()); err != nil {
			t.Fatalf("seed account %d: %v", i, err)
		}
	}
	for i, c := range chars {
		acct := acct1
		if i >= 2 {
			acct = acct2
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO characters (character_id, account_id, name, name_key, class_id) VALUES ($1,$2,$3,$4,'class.kim')`,
			c.String(), acct.String(), "lockchar"+c.String()[:8], "lockchar"+c.String()[:8]); err != nil {
			t.Fatalf("seed character %d: %v", i, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO character_currencies (character_id, currency_id, balance) VALUES ($1,'currency.common',1000)`, c.String()); err != nil {
			t.Fatalf("seed currency %d: %v", i, err)
		}
		if _, err := pool.Exec(ctx,
			`INSERT INTO character_inventories (character_id, capacity) VALUES ($1,60)`, c.String()); err != nil {
			t.Fatalf("seed inventory %d: %v", i, err)
		}
	}

	const iterations = 1000
	const workers = 8
	var deadlocks, otherErrs, commits atomic.Int64
	var wg sync.WaitGroup
	work := make(chan int, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				a := chars[i%len(chars)]
				b := chars[(i/len(chars))%len(chars)]
				if a == b {
					b = chars[(i+1)%len(chars)]
				}
				if err := transfer(ctx, pool, a, b, i); err != nil {
					var pgErr *pgconn.PgError
					if errors.As(err, &pgErr) && pgErr.Code == "40P01" {
						deadlocks.Add(1)
					} else {
						otherErrs.Add(1)
						t.Errorf("iteration %d: %v", i, err)
					}
					continue
				}
				commits.Add(1)
			}
		}()
	}
	for i := 0; i < iterations; i++ {
		work <- i
	}
	close(work)
	wg.Wait()

	if deadlocks.Load() != 0 {
		t.Fatalf("deadlocks detected: %d", deadlocks.Load())
	}
	if otherErrs.Load() != 0 {
		t.Fatalf("non-deadlock errors: %d", otherErrs.Load())
	}
	if commits.Load() != iterations {
		t.Fatalf("commits = %d, want %d", commits.Load(), iterations)
	}
	var sum int64
	if err := pool.QueryRow(ctx, `SELECT COALESCE(SUM(balance),0) FROM character_currencies`).Scan(&sum); err != nil {
		t.Fatalf("sum: %v", err)
	}
	if sum != int64(len(chars))*1000 {
		t.Fatalf("balance conservation broken: %d", sum)
	}
}

// transfer is one canonical multi-aggregate transaction: the complete
// account/character-style lock set (characters p2, currencies p3,
// inventories p4) + priority-2.5 receipt keys, then the mutation, then
// the operations row inserted last.
func transfer(ctx context.Context, pool *pgxpool.Pool, a, b id.UUID, seq int) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	op := id.NewV4()
	locks := []Lock{
		RowLock("characters", a), RowLock("characters", b),
		RowLock("character_currencies", a), RowLock("character_currencies", b),
		RowLock("character_inventories", a), RowLock("character_inventories", b),
	}
	locks = append(locks, ReceiptKeys(
		Receipt{Owner: a, Family: "trade.direct", Operation: op},
		Receipt{Owner: b, Family: "trade.direct", Operation: op},
	)...)
	locks = append(locks, OperationInsertLock("trade.direct", a, op))
	if err := SortLocks(locks); err != nil {
		return err
	}
	if err := Acquire(ctx, tx, locks...); err != nil {
		return err
	}
	amt := int64((seq % 7) + 1)
	if _, err := tx.Exec(ctx,
		`UPDATE character_currencies SET balance = balance - $2 WHERE character_id = $1 AND currency_id = 'currency.common'`,
		a.String(), amt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE character_currencies SET balance = balance + $2 WHERE character_id = $1 AND currency_id = 'currency.common'`,
		b.String(), amt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE character_inventories SET revision = revision + 1 WHERE character_id = ANY($1::uuid[])`,
		[]string{a.String(), b.String()}); err != nil {
		return err
	}
	fp := make([]byte, 32)
	now := time.Now().UTC()
	// operations rows are inserted last in the same transaction.
	if _, err := tx.Exec(ctx,
		`INSERT INTO operations (operation_family, owner_kind, owner_id, operation_id, request_fingerprint, created_at, completed_at, replay_until)
		 VALUES ('trade.direct','CHARACTER',$1,$2,$3,$4,$4,$5)`,
		a.String(), op.String(), fp, now, now.AddDate(0, 0, 180)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
