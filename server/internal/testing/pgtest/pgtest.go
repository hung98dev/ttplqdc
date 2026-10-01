// Package pgtest hands tests a real PostgreSQL 18.6. When
// THINHTHAN_TEST_PG_DSN is set it is used verbatim (Linux CI service
// container, a local pinned container); otherwise the pinned EDB Windows
// binaries are downloaded, hash-verified, initdb'd and started on a random
// port (Windows job), or the pinned postgres:18.6 image is started through
// Docker (local Linux). Nothing is mocked.
package pgtest

import (
	"archive/zip"
	"context"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/stackpin"
)

// EnvDSN is the environment variable carrying a preset test DSN.
const EnvDSN = "THINHTHAN_TEST_PG_DSN"

// Server is a reachable PostgreSQL 18.6 endpoint for tests.
type Server struct {
	dsn     string
	pgDump  []string // argv prefix producing a `pg_dump` for this server
	cleanup func()
}

// DSN returns the server-level DSN (default database).
func (s *Server) DSN() string { return s.dsn }

// PgDumpArgv returns the argv prefix that runs pg_dump against this server
// (e.g. {"pg_dump"} or {"docker","exec","<id>","pg_dump"}), or nil when no
// pg_dump is resolvable.
func (s *Server) PgDumpArgv() []string { return s.pgDump }

// DumpSchema returns `pg_dump --schema-only` output for dbName on this
// server, normalized like migrations/schema_snapshot.sql (no \restrict /
// dump-version comments).
func (s *Server) DumpSchema(ctx context.Context, dbName string) (string, error) {
	if len(s.pgDump) == 0 {
		return "", errors.New("pgtest: no pg_dump resolvable for this server")
	}
	u, err := pgx.ParseConfig(s.dsn)
	if err != nil {
		return "", err
	}
	argv := append(append([]string{}, s.pgDump...),
		"--schema-only", "--no-owner", "--no-privileges",
		"-h", u.Host, "-p", fmt.Sprint(u.Port), "-U", u.User, "-d", dbName)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+u.Password)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pgtest: pg_dump: %w", err)
	}
	return NormalizeDump(string(out)), nil
}

// NormalizeDump strips volatile pg_dump lines so two dumps of identical
// schema compare byte-equal (\restrict tokens, dump headers).
func NormalizeDump(dump string) string {
	var b strings.Builder
	for _, line := range strings.Split(dump, "\n") {
		switch {
		case strings.HasPrefix(line, `\restrict`), strings.HasPrefix(line, `\unrestrict`):
		case strings.HasPrefix(line, "-- Dumped"):
		case line == "--":
		default:
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// NewDB creates a fresh empty database on the server and returns its DSN.
// Caller drops it via the returned cleanup.
func (s *Server) NewDB(ctx context.Context, name string) (dsn string, cleanup func(), err error) {
	u, err := pgx.ParseConfig(s.dsn)
	if err != nil {
		return "", nil, err
	}
	conn, err := pgx.ConnectConfig(ctx, u)
	if err != nil {
		return "", nil, err
	}
	if _, err := conn.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize()); err != nil {
		conn.Close(ctx)
		return "", nil, fmt.Errorf("pgtest: create db: %w", err)
	}
	conn.Close(ctx)
	// Note: ConnConfig.Copy().ConnString() echoes the original DSN string
	// verbatim (it does not reflect a mutated Database field), so the
	// scratch DSN is built by rewriting the URL path.
	pu, err := url.Parse(s.dsn)
	if err != nil || pu.Scheme == "" {
		return "", nil, fmt.Errorf("pgtest: scratch dsn rewrite: %w", err)
	}
	pu.Path = "/" + name
	return pu.String(), func() {
		c2, e := pgx.ConnectConfig(context.Background(), u)
		if e == nil {
			c2.Exec(context.Background(),
				`DROP DATABASE IF EXISTS `+pgx.Identifier{name}.Sanitize()+` WITH (FORCE)`)
			c2.Close(context.Background())
		}
	}, nil
}

// Ensure resolves a PostgreSQL 18.6 server: preset DSN, Docker container
// running the pinned image (or one it can start), or EDB binaries on Windows.
// Returns ErrUnavailable when no path exists on this machine.
func Ensure(ctx context.Context) (*Server, error) {
	if dsn := strings.TrimSpace(os.Getenv(EnvDSN)); dsn != "" {
		s := &Server{dsn: dsn}
		s.pgDump = resolvePgDump(ctx, dsn)
		return s, nil
	}
	if runtime.GOOS == "windows" {
		return ensureEDB(ctx)
	}
	if cid, err := dockerPostgres(ctx); err == nil {
		// A container running the pinned image already exists; exec pg_dump in it.
		host, port, user, pass, db, err := dockerContainerEndpoint(ctx, cid)
		if err != nil {
			return nil, err
		}
		return &Server{
			dsn:    fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, db),
			pgDump: []string{"docker", "exec", "-e", "PGPASSWORD=" + pass, cid, "pg_dump"},
		}, nil
	}
	if cid, err := dockerRunPostgres(ctx); err == nil {
		s := &Server{}
		host, port, err := dockerMappedPort(ctx, cid)
		if err != nil {
			exec.Command("docker", "rm", "-f", cid).Run()
			return nil, err
		}
		s.dsn = fmt.Sprintf("postgres://postgres:postgres@%s:%s/postgres?sslmode=disable", host, port)
		s.pgDump = []string{"docker", "exec", "-e", "PGPASSWORD=postgres", cid, "pg_dump"}
		s.cleanup = func() { exec.Command("docker", "rm", "-f", cid).Run() }
		if err := waitReady(ctx, s.dsn, 60*time.Second); err != nil {
			s.Close()
			return nil, err
		}
		return s, nil
	}
	return nil, ErrUnavailable
}

// ErrUnavailable marks "no PostgreSQL path on this machine" (no preset DSN,
// no Docker, not Windows) — verify.ps1 -LocalDeferMissing treats this as
// DEFERRED(local-missing), not a failure.
var ErrUnavailable = errors.New("pgtest: no postgres available (local-missing)")

// Close releases a bootstrapped server (preset DSNs are no-ops).
func (s *Server) Close() {
	if s.cleanup != nil {
		s.cleanup()
	}
}

// DSN is the test-facing entry: preset env, else bootstrap, else t.Skip.
func DSN(t testing.TB) string {
	t.Helper()
	srv, err := Ensure(context.Background())
	if errors.Is(err, ErrUnavailable) {
		t.Skip("DEFERRED(local-missing): " + err.Error())
	}
	if err != nil {
		t.Fatalf("pgtest: %v", err)
	}
	t.Cleanup(srv.Close)
	if err := waitReady(context.Background(), srv.DSN(), 30*time.Second); err != nil {
		t.Fatalf("pgtest: server not ready: %v", err)
	}
	return srv.DSN()
}

// FreshDB returns a DSN for a new empty database created on the resolved
// server; it is dropped when the test ends.
func FreshDB(t testing.TB) string {
	t.Helper()
	srv, err := Ensure(context.Background())
	if errors.Is(err, ErrUnavailable) {
		t.Skip("DEFERRED(local-missing): " + err.Error())
	}
	if err != nil {
		t.Fatalf("pgtest: %v", err)
	}
	t.Cleanup(srv.Close)
	name := "pgtest_" + hex.EncodeToString(rand16())
	dsn, cleanup, err := srv.NewDB(context.Background(), name)
	if err != nil {
		t.Fatalf("pgtest: new db: %v", err)
	}
	t.Cleanup(cleanup)
	return dsn
}

// FreshServer is FreshDB's sibling for code paths that need the Server
// object (pg_dump resolution) instead of a bare DSN.
func FreshServer(t testing.TB) *Server {
	t.Helper()
	srv, err := Ensure(context.Background())
	if errors.Is(err, ErrUnavailable) {
		t.Skip("DEFERRED(local-missing): " + err.Error())
	}
	if err != nil {
		t.Fatalf("pgtest: %v", err)
	}
	t.Cleanup(srv.Close)
	return srv
}

func rand16() []byte {
	var b [16]byte
	if _, err := crand.Read(b[:]); err != nil {
		panic(err)
	}
	return b[:]
}

// resolvePgDump finds a pg_dump >= server version for a preset DSN: PATH
// first, then a Docker container running the pinned image.
func resolvePgDump(ctx context.Context, dsn string) []string {
	if path, err := exec.LookPath("pg_dump"); err == nil {
		out, err := exec.CommandContext(ctx, path, "--version").Output()
		if err == nil && strings.Contains(string(out), "pg_dump (PostgreSQL) 18") {
			return []string{path}
		}
	}
	if cid, err := dockerPostgres(ctx); err == nil {
		return []string{"docker", "exec", cid, "pg_dump"}
	}
	if runtime.GOOS == "windows" {
		if bin := edbBinDir(); bin != "" {
			return []string{filepath.Join(bin, "pg_dump.exe")}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Docker path (local Linux / Linux CI where the pinned image is running)
// ---------------------------------------------------------------------------

func dockerPostgres(ctx context.Context) (containerID string, err error) {
	out, err := exec.CommandContext(ctx, "docker", "ps", "-q",
		"--filter", "ancestor="+stackpin.PostgresLinuxImage).Output()
	if err == nil && strings.TrimSpace(string(out)) != "" {
		return strings.Fields(string(out))[0], nil
	}
	// Any local postgres:18.6 container (local agents name their own, e.g.
	// pg-imp061); require the pinned digest family 18.6.
	out, err = exec.CommandContext(ctx, "docker", "ps", "--format", "{{.ID}} {{.Image}}").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.HasPrefix(f[1], "postgres:18.6") {
			return f[0], nil
		}
	}
	return "", errors.New("pgtest: no running postgres:18.6 container")
}

func dockerRunPostgres(ctx context.Context) (containerID string, err error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return "", err
	}
	out, err := exec.CommandContext(ctx, "docker", "run", "-d", "--rm",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-p", "127.0.0.1::5432",
		stackpin.PostgresLinuxImage).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func dockerMappedPort(ctx context.Context, cid string) (string, string, error) {
	out, err := exec.CommandContext(ctx, "docker", "port", cid, "5432/tcp").Output()
	if err != nil {
		return "", "", err
	}
	// `docker port` prints one line per bind address
	// ("0.0.0.0:15061\n[::]:15061"); take the IPv4 line.
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.TrimSpace(line)
		if f == "" {
			continue
		}
		i := strings.LastIndex(f, ":")
		if i < 0 {
			continue
		}
		host, port := f[:i], f[i+1:]
		if strings.HasPrefix(host, "[") {
			continue
		}
		return host, port, nil
	}
	return "", "", fmt.Errorf("pgtest: unparsable docker port %q", strings.TrimSpace(string(out)))
}

func dockerContainerEndpoint(ctx context.Context, cid string) (host, port, user, pass, db string, err error) {
	host, port, err = dockerMappedPort(ctx, cid)
	if err != nil {
		return
	}
	out, err := exec.CommandContext(ctx, "docker", "exec", cid, "env").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			kv := strings.SplitN(line, "=", 2)
			if len(kv) != 2 {
				continue
			}
			switch kv[0] {
			case "POSTGRES_USER":
				user = kv[1]
			case "POSTGRES_PASSWORD":
				pass = kv[1]
			case "POSTGRES_DB":
				db = kv[1]
			}
		}
	}
	if user == "" {
		user = "postgres"
	}
	if db == "" {
		db = "postgres"
	}
	return host, port, user, pass, db, nil
}

// ---------------------------------------------------------------------------
// EDB binaries path (Windows job / local Windows)
// ---------------------------------------------------------------------------

var edbDir = func() string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return filepath.Join(d, "thinhthan", "pg18")
	}
	d, _ := os.UserCacheDir()
	return filepath.Join(d, "thinhthan", "pg18")
}

var edbRoot = filepath.Join(edbDir(), "pgsql")

func edbBinDir() string {
	bin := filepath.Join(edbRoot, "bin")
	if _, err := os.Stat(filepath.Join(bin, "postgres.exe")); err == nil {
		return bin
	}
	return ""
}

func ensureEDB(ctx context.Context) (*Server, error) {
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("pgtest: EDB binaries are windows-only (GOOS=%s)", runtime.GOOS)
	}
	bin := edbBinDir()
	if bin == "" {
		var err error
		if bin, err = fetchEDB(ctx); err != nil {
			return nil, err
		}
	}
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	dataDir, err := os.MkdirTemp("", "pgtest-data-")
	if err != nil {
		return nil, err
	}
	exe := func(name string) string { return filepath.Join(bin, name+".exe") }
	if out, err := exec.CommandContext(ctx, exe("initdb"),
		"-D", dataDir, "-U", "postgres", "-A", "trust", "-E", "UTF8", "--no-sync").CombinedOutput(); err != nil {
		os.RemoveAll(dataDir)
		return nil, fmt.Errorf("pgtest: initdb: %w\n%s", err, out)
	}
	logFile := filepath.Join(dataDir, "server.log")
	// pg_ctl spawns postgres.exe, which inherits piped stdio and keeps the
	// write end open after pg_ctl itself exits — capturing output through a
	// pipe (CombinedOutput/Output) hangs Wait forever. Redirect pg_ctl's own
	// output to a file instead (same pattern as scripts/verify.ps1).
	ctlLog := filepath.Join(dataDir, "pg_ctl-out.log")
	f, err := os.Create(ctlLog)
	if err != nil {
		os.RemoveAll(dataDir)
		return nil, fmt.Errorf("pgtest: pg_ctl log: %w", err)
	}
	cmd := exec.CommandContext(ctx, exe("pg_ctl"),
		"-D", dataDir, "-l", logFile, "-o",
		fmt.Sprintf("-p %d -h 127.0.0.1", port),
		"-w", "start")
	cmd.Stdout, cmd.Stderr = f, f
	startErr := cmd.Run()
	out, _ := os.ReadFile(ctlLog)
	f.Close()
	if startErr != nil {
		os.RemoveAll(dataDir)
		return nil, fmt.Errorf("pgtest: pg_ctl start: %w\n%s", startErr, out)
	}
	s := &Server{
		dsn:    fmt.Sprintf("postgres://postgres@127.0.0.1:%d/postgres?sslmode=disable", port),
		pgDump: []string{exe("pg_dump")},
	}
	s.cleanup = func() {
		exec.Command(exe("pg_ctl"), "-D", dataDir, "-m", "fast", "-w", "stop").Run()
		os.RemoveAll(dataDir)
	}
	if err := waitReady(ctx, s.dsn, 30*time.Second); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func fetchEDB(ctx context.Context) (bin string, err error) {
	root := filepath.Dir(edbRoot)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	zipPath := filepath.Join(root, "postgres.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, stackpin.EDBPostgresZip.URL, nil)
	if err != nil {
		f.Close()
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		f.Close()
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		f.Close()
		return "", fmt.Errorf("pgtest: EDB download status %d", resp.StatusCode)
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), resp.Body)
	f.Close()
	if err != nil {
		return "", err
	}
	if n != stackpin.EDBPostgresZip.Size {
		return "", fmt.Errorf("pgtest: EDB size %d != pinned %d", n, stackpin.EDBPostgresZip.Size)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != stackpin.EDBPostgresZip.SHA256 {
		return "", fmt.Errorf("pgtest: EDB sha256 %s != pinned %s", got, stackpin.EDBPostgresZip.SHA256)
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		target := filepath.Join(root, zf.Name)
		if !strings.HasPrefix(target, filepath.Clean(root)+string(os.PathSeparator)) {
			return "", fmt.Errorf("pgtest: zip slip %q", zf.Name)
		}
		if zf.FileInfo().IsDir() {
			os.MkdirAll(target, 0o755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		rc, err := zf.Open()
		if err != nil {
			return "", err
		}
		w, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			rc.Close()
			return "", err
		}
		if _, err := io.Copy(w, rc); err != nil {
			w.Close()
			rc.Close()
			return "", err
		}
		w.Close()
		rc.Close()
	}
	os.Remove(zipPath)
	if bin := edbBinDir(); bin != "" {
		return bin, nil
	}
	return "", errors.New("pgtest: EDB zip did not contain pgsql/bin")
}

// ---------------------------------------------------------------------------

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func waitReady(ctx context.Context, dsn string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		conn, err := pgx.Connect(ctx, dsn)
		if err == nil {
			conn.Close(ctx)
			return nil
		}
		last = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return fmt.Errorf("pgtest: postgres not ready after %s: %w", timeout, last)
}
