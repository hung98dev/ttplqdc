package social

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
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/schema"
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
	name := fmt.Sprintf("social_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
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

// wipe clears the social tables so tests are isolated.
func wipe(t *testing.T) {
	t.Helper()
	pool := requirePool(t)
	for _, table := range []string{
		"player_reports", "chat_messages", "blocks", "friend_requests",
		"friends", "characters", "accounts",
	} {
		if _, err := pool.Exec(context.Background(),
			"DELETE FROM "+table); err != nil {
			t.Fatalf("wipe %s: %v", table, err)
		}
	}
}

// mkCharacter inserts one character (level 1) and returns its id.
func mkCharacter(t *testing.T) id.UUID {
	t.Helper()
	return mkCharacterLevel(t, 1)
}

func mkCharacterLevel(t *testing.T, level int32) id.UUID {
	t.Helper()
	ctx := context.Background()
	char := id.NewV4()
	accountID := id.NewV4()
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO accounts (account_id) VALUES ($1)`,
		accountID[:]); err != nil {
		t.Fatalf("account: %v", err)
	}
	name := fmt.Sprintf("c%x", char[:4])
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id, level, created_at)
		 VALUES ($1,$2,$3,$3,'class.kim',$4,now())`,
		char[:], accountID[:], name, level); err != nil {
		t.Fatalf("character: %v", err)
	}
	return char
}

// accountOf returns the character's account id.
func accountOf(t *testing.T, char id.UUID) id.UUID {
	t.Helper()
	var acc id.UUID
	var b []byte
	if err := sharedPool.QueryRow(context.Background(),
		`SELECT account_id FROM characters WHERE character_id = $1`,
		char[:]).Scan(&b); err != nil {
		t.Fatalf("accountOf: %v", err)
	}
	copy(acc[:], b)
	return acc
}

// mkPending inserts one PENDING request; when backdated is true the row
// is written 8 days old so expires_at < now (schema pins expires_at =
// created_at + 7d).
func mkPending(t *testing.T, requester, target id.UUID, backdated bool) id.UUID {
	t.Helper()
	ctx := context.Background()
	reqID, opID := id.NewV4(), id.NewV4()
	created := time.Now().UTC()
	if backdated {
		created = created.Add(-8 * 24 * time.Hour)
	}
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO friend_requests
		   (friend_request_id, requester_character_id, target_character_id,
		    state, created_at, expires_at, create_operation_id)
		 VALUES ($1,$2,$3,'PENDING',$4,$5,$6)`,
		reqID[:], requester[:], target[:], created,
		created.Add(RequestTTL), opID[:]); err != nil {
		t.Fatalf("mkPending: %v", err)
	}
	return reqID
}

// mkFriends inserts one friends pair directly.
func mkFriends(t *testing.T, a, b id.UUID) {
	t.Helper()
	ctx := context.Background()
	lo, hi := lowHigh(a, b)
	opID := id.NewV4()
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO friends
		   (character_low_id, character_high_id, created_at, created_operation_id)
		 VALUES ($1,$2,now(),$3)`, lo[:], hi[:], opID[:]); err != nil {
		t.Fatalf("mkFriends: %v", err)
	}
}

// mkBlock inserts one directional block.
func mkBlock(t *testing.T, blocker, blocked id.UUID) {
	t.Helper()
	ctx := context.Background()
	opID := id.NewV4()
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO blocks
		   (blocker_character_id, blocked_character_id, created_at, operation_id)
		 VALUES ($1,$2,now(),$3)`, blocker[:], blocked[:], opID[:]); err != nil {
		t.Fatalf("mkBlock: %v", err)
	}
}

// exec drives one executor method inside a transaction.
func exec(t *testing.T, fn func(context.Context, pgx.Tx,
	*journalv1.DurableCommandRecord) (idempotency.Outcome, error),
	rec *journalv1.DurableCommandRecord) *journalv1.JournalOutcome {
	t.Helper()
	ctx := context.Background()
	tx, err := sharedPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)
	out, err := fn(ctx, tx, rec)
	if err != nil {
		t.Fatalf("executor: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	o := &journalv1.JournalOutcome{}
	if err := protojson.Unmarshal(out.Payload, o); err != nil {
		t.Fatalf("outcome decode: %v", err)
	}
	return o
}
