package pgtest

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestUsesPresetDsn: a preset THINHTHAN_TEST_PG_DSN is used verbatim — no
// bootstrap path runs.
func TestUsesPresetDsn(t *testing.T) {
	const dsn = "postgres://u:p@127.0.0.1:9/preset?sslmode=disable"
	t.Setenv(EnvDSN, dsn)
	srv, err := Ensure(context.Background())
	if err != nil {
		t.Fatalf("ensure with preset DSN: %v", err)
	}
	defer srv.Close()
	if srv.DSN() != dsn {
		t.Fatalf("DSN %q != preset %q", srv.DSN(), dsn)
	}
	if len(srv.PgDumpArgv()) == 0 {
		t.Fatal("no pg_dump path resolved for preset DSN")
	}
}

// TestStartsEdbBinariesWhenDsnUnset: with no preset DSN the harness
// bootstraps a server — EDB binaries on Windows, pinned docker image
// elsewhere — or reports ErrUnavailable when no path exists.
func TestStartsEdbBinariesWhenDsnUnset(t *testing.T) {
	t.Setenv(EnvDSN, "")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	srv, err := Ensure(ctx)
	if errors.Is(err, ErrUnavailable) {
		t.Skip("DEFERRED(local-missing): " + err.Error())
	}
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	defer srv.Close()
	if !strings.HasPrefix(srv.DSN(), "postgres://") {
		t.Fatalf("unexpected DSN %q", srv.DSN())
	}
	dsn, cleanup, err := srv.NewDB(ctx, "pgtest_smoke_"+time.Now().Format("150405"))
	if err != nil {
		t.Fatalf("newdb: %v", err)
	}
	defer cleanup()
	if !strings.Contains(dsn, "pgtest_smoke_") {
		t.Fatalf("unexpected scratch DSN %q", dsn)
	}
}
