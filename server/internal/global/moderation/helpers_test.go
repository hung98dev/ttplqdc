package moderation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
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
	name := fmt.Sprintf("moderation_tests_%d_%d", time.Now().UnixNano(), os.Getpid())
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
		"player_reports", "chat_messages", "characters",
	} {
		if _, err := pool.Exec(context.Background(),
			"DELETE FROM "+table); err != nil {
			t.Fatalf("wipe %s: %v", table, err)
		}
	}
	// Keep TOMBSTONE_ACCOUNT_ID — the erasure re-point target seeded by
	// the baseline migration (never erasable/purgeable).
	if _, err := pool.Exec(context.Background(),
		`DELETE FROM accounts
		  WHERE account_id <> '00000000-0000-0000-0000-000000000001'`); err != nil {
		t.Fatalf("wipe accounts: %v", err)
	}
}

// mkCharacter inserts one character and returns (accountID, charID).
func mkCharacter(t *testing.T) (id.UUID, id.UUID) {
	t.Helper()
	ctx := context.Background()
	accountID, char := id.NewV4(), id.NewV4()
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO accounts (account_id) VALUES ($1)`, accountID[:]); err != nil {
		t.Fatalf("account: %v", err)
	}
	name := fmt.Sprintf("c%x", char[:4])
	if _, err := sharedPool.Exec(ctx,
		`INSERT INTO characters (character_id, account_id, name, name_key, class_id, level, created_at)
		 VALUES ($1,$2,$3,$3,'class.kim',1,now())`,
		char[:], accountID[:], name); err != nil {
		t.Fatalf("character: %v", err)
	}
	return accountID, char
}

// mkReport inserts one OPEN player_reports row the way the
// durable/social C2S_REPORT_PLAYER exec writes it (insert path is
// owned there; IMP-094 reads case state only).
func mkReport(t *testing.T, accountID, reporter, target id.UUID,
	reason string, chatMessageID *id.UUID, notes *string,
	createdAt time.Time) id.UUID {
	t.Helper()
	reportID, opID := id.NewV4(), id.NewV4()
	if _, err := sharedPool.Exec(context.Background(),
		`INSERT INTO player_reports
		    (report_id, operation_id, reporter_account_id,
		     reporter_character_id, target_character_id, reason,
		     chat_message_id, reporter_notes, created_at, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'OPEN')`,
		reportID[:], opID[:], accountID[:], reporter[:], target[:],
		reason, chatMessageID, notes, createdAt); err != nil {
		t.Fatalf("report: %v", err)
	}
	return reportID
}
