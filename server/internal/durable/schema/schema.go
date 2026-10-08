// Package schema owns the launch baseline schema assertions: it applies the
// numbered migration pairs through golang-migrate, parses the authored DDL
// into a per-constraint catalog (tables/columns/keys/checks/FKs/indexes)
// and verifies the live catalog matches — the Q5.migrations and Q5.schema
// evaluators (runner.go binds the exported Check* functions).
package schema

import (
	"context"
	crand "crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	pgxv5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"

	"thinhthan/internal/testing/pgtest"
)

// MigrationsDir returns repoRoot/server/migrations.
func MigrationsDir(repoRoot string) string {
	return filepath.Join(repoRoot, "server", "migrations")
}

// SnapshotPath returns repoRoot/server/migrations/schema_snapshot.sql — the
// sole exempt non-numbered file (physical_schema_contract.md §6).
func SnapshotPath(repoRoot string) string {
	return filepath.Join(MigrationsDir(repoRoot), "schema_snapshot.sql")
}

// Migrate applies the migration pairs in dir against dsn.
// direction: "up" (all), "down" (all), "up1"/"down1" (single step).
func Migrate(ctx context.Context, dsn, dir, direction string) error {
	m, err := openMigrator(dsn, dir)
	if err != nil {
		return err
	}
	defer m.Close()
	var merr error
	switch direction {
	case "up":
		merr = m.Up()
	case "down":
		merr = m.Down()
	case "up1":
		merr = m.Steps(1)
	case "down1":
		merr = m.Steps(-1)
	default:
		return fmt.Errorf("schema: unknown direction %q", direction)
	}
	if merr != nil && !errors.Is(merr, migrate.ErrNoChange) {
		return fmt.Errorf("schema: migrate %s: %w", direction, merr)
	}
	return ctxErr(ctx)
}

func ctxErr(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func openMigrator(dsn, dir string) (*migrate.Migrate, error) {
	src, err := iofs.New(os.DirFS(dir), ".")
	if err != nil {
		return nil, fmt.Errorf("schema: source: %w", err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	drv, err := pgxv5.WithInstance(db, &pgxv5.Config{MigrationsTable: "schema_migrations"})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("schema: db driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", drv)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("schema: migrate: %w", err)
	}
	return m, nil
}

// MigrateSteps moves n steps (positive = up, negative = down).
func MigrateSteps(ctx context.Context, dsn, dir string, n int) error {
	m, err := openMigrator(dsn, dir)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Steps(n); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return ctxErr(ctx)
}

// MigrateForce sets the version without running migrations (dirty repair).
func MigrateForce(ctx context.Context, dsn, dir string, v int) error {
	m, err := openMigrator(dsn, dir)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Force(v); err != nil {
		return err
	}
	return ctxErr(ctx)
}

// MigrateVersion reports the current migration version and dirty flag.
func MigrateVersion(ctx context.Context, dsn, dir string) (uint, bool, error) {
	m, err := openMigrator(dsn, dir)
	if err != nil {
		return 0, false, err
	}
	defer m.Close()
	v, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return v, dirty, ctxErr(ctx)
}

// CheckMigrations is the Q5.migrations evaluator: on a scratch database it
// runs up → down → up, verifies the round-trip empties the catalog, diffs a
// schema-only pg_dump byte-for-byte against schema_snapshot.sql (normalized
// for volatile lines), then drops the scratch database.
// Returns (details, missing); missing=true when no PostgreSQL/pg_dump path
// exists on this machine (verify.ps1 -LocalDeferMissing → DEFERRED).
func CheckMigrations(ctx context.Context, srv *pgtest.Server, repoRoot string) (details []string, missing bool) {
	if srv == nil {
		return nil, true
	}
	scratch := "q5_migrations_" + randSuffix()
	dsn, cleanup, err := srv.NewDB(ctx, scratch)
	if err != nil {
		return []string{fmt.Sprintf("create scratch db: %v", err)}, false
	}
	defer cleanup()

	dir := MigrationsDir(repoRoot)
	if err := Migrate(ctx, dsn, dir, "up"); err != nil {
		return []string{fmt.Sprintf("apply up: %v", err)}, false
	}
	// Round-trip: down must empty the public schema, re-up must restore it.
	if err := Migrate(ctx, dsn, dir, "down"); err != nil {
		return []string{fmt.Sprintf("apply down: %v", err)}, false
	}
	if n, err := countPublicTables(ctx, dsn); err != nil {
		return []string{fmt.Sprintf("count after down: %v", err)}, false
	} else if n != 0 {
		details = append(details, fmt.Sprintf("down left %d tables in public schema", n))
	}
	if err := Migrate(ctx, dsn, dir, "up"); err != nil {
		return append(details, fmt.Sprintf("re-apply up: %v", err)), false
	}
	if len(srv.PgDumpArgv()) == 0 {
		return append(details, "no pg_dump resolvable for snapshot diff"), true
	}
	dump, err := srv.DumpSchema(ctx, scratch)
	if err != nil {
		return append(details, fmt.Sprintf("pg_dump: %v", err)), false
	}
	want, err := os.ReadFile(SnapshotPath(repoRoot))
	if err != nil {
		return append(details, fmt.Sprintf("read snapshot: %v", err)), false
	}
	if diff := diffDump(pgtest.NormalizeDump(string(want)), dump); diff != "" {
		details = append(details, "snapshot diff: "+diff)
	}
	return details, false
}

// CheckSchema is the Q5.schema evaluator: applies the baseline on a scratch
// database and asserts the parsed per-constraint catalog — every table,
// column, primary/unique key, CHECK expression, foreign key and index
// (including partial-index predicates) — exists in the live pg_catalog.
func CheckSchema(ctx context.Context, srv *pgtest.Server, repoRoot string) (details []string, missing bool) {
	if srv == nil {
		return nil, true
	}
	cat, err := ParseMigration(filepath.Join(MigrationsDir(repoRoot), "000001_baseline_schema.up.sql"))
	if err != nil {
		return []string{fmt.Sprintf("parse migration: %v", err)}, false
	}
	scratch := "q5_schema_" + randSuffix()
	dsn, cleanup, err := srv.NewDB(ctx, scratch)
	if err != nil {
		return []string{fmt.Sprintf("create scratch db: %v", err)}, false
	}
	defer cleanup()
	if err := Migrate(ctx, dsn, MigrationsDir(repoRoot), "up"); err != nil {
		return []string{fmt.Sprintf("apply up: %v", err)}, false
	}
	live, err := LoadCatalog(ctx, dsn)
	if err != nil {
		return []string{fmt.Sprintf("load catalog: %v", err)}, false
	}
	return cat.Verify(live), false
}

// ---------------------------------------------------------------------------
// Catalog: parsed expectations + live assertion
// ---------------------------------------------------------------------------

// CatalogEntry is one asserted schema object.
type CatalogEntry struct {
	Kind  string // table | column | pk | unique | check | fk | index
	Table string
	Name  string // column/constraint/index name ("" for anonymous check)
	Def   string // normalized definition (check expr / index def / fk target)
}

// Catalog is the expected schema surface parsed from the migration DDL.
type Catalog struct {
	Entries []CatalogEntry
}

var (
	reCreateTable = regexp.MustCompile(`(?is)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)\s*\(`)
	reCreateIndex = regexp.MustCompile(`(?is)^\s*CREATE\s+(UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)\s+ON\s+([a-z_][a-z0-9_]*)\s*\(`)
	reIdent       = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
	reConNamed    = regexp.MustCompile(`(?is)^\s*CONSTRAINT\s+([a-z_][a-z0-9_]*)\s+(.*)$`)
)

// ParseMigration reads a baseline *.up.sql file into a Catalog. The parser
// handles the authored DDL subset: CREATE TABLE bodies and CREATE INDEX
// statements; statements are split at top-level semicolons.
func ParseMigration(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := &Catalog{}
	for _, stmt := range splitStatements(string(data)) {
		stmt = stripComments(stmt)
		if m := reCreateTable.FindStringSubmatch(stmt); m != nil {
			parseTable(c, m[1], stmt)
			continue
		}
		if m := reCreateIndex.FindStringSubmatch(stmt); m != nil {
			unique := strings.TrimSpace(m[1]) != ""
			parseIndex(c, unique, m[2], m[3], stmt)
			continue
		}
	}
	return c, nil
}

func splitStatements(sqlText string) []string {
	var out []string
	depth := 0
	var cur strings.Builder
	inStr := false
	for i := 0; i < len(sqlText); i++ {
		ch := sqlText[i]
		switch {
		case inStr:
			cur.WriteByte(ch)
			if ch == '\'' {
				inStr = false
			}
		case ch == '\'':
			inStr = true
			cur.WriteByte(ch)
		case ch == '-' && i+1 < len(sqlText) && sqlText[i+1] == '-':
			// Line comments are dropped here: parens/semicolons inside them
			// must not drive the splitter.
			for i < len(sqlText) && sqlText[i] != '\n' {
				i++
			}
			if i < len(sqlText) {
				cur.WriteByte(sqlText[i])
			}
		case ch == '(':
			depth++
			cur.WriteByte(ch)
		case ch == ')':
			depth--
			cur.WriteByte(ch)
		case ch == ';' && depth == 0:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(ch)
		}
	}
	if strings.TrimSpace(cur.String()) != "" {
		out = append(out, cur.String())
	}
	return out
}

func stripComments(stmt string) string {
	var b strings.Builder
	for _, line := range strings.Split(stmt, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

func parseTable(c *Catalog, table, stmt string) {
	c.Entries = append(c.Entries, CatalogEntry{Kind: "table", Table: table})
	body := stmt[strings.Index(stmt, "(")+1:]
	body = strings.TrimSuffix(strings.TrimSpace(body), ")")
	for _, part := range splitTopLevel(body) {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) == 0 {
			continue
		}
		head := strings.ToUpper(fields[0])
		switch head {
		case "PRIMARY":
			c.Entries = append(c.Entries, CatalogEntry{Kind: "pk", Table: table,
				Def: innerParens(part)})
		case "UNIQUE":
			c.Entries = append(c.Entries, CatalogEntry{Kind: "unique", Table: table,
				Def: innerParens(part)})
		case "FOREIGN":
			c.Entries = append(c.Entries, CatalogEntry{Kind: "fk", Table: table,
				Def: normalizeWS(part)})
		case "CHECK":
			c.Entries = append(c.Entries, CatalogEntry{Kind: "check", Table: table,
				Def: normalizeCheck(innerParens(part))})
		case "CONSTRAINT":
			if m := reConNamed.FindStringSubmatch(part); m != nil {
				rest := m[2]
				kind := "check"
				up := strings.ToUpper(rest)
				switch {
				case strings.HasPrefix(up, "CHECK"):
					kind = "check"
					rest = innerParens(rest)
				case strings.HasPrefix(up, "PRIMARY"):
					kind = "pk"
					rest = innerParens(rest)
				case strings.HasPrefix(up, "UNIQUE"):
					kind = "unique"
					rest = innerParens(rest)
				case strings.HasPrefix(up, "FOREIGN"):
					kind = "fk"
				}
				c.Entries = append(c.Entries, CatalogEntry{Kind: kind, Table: table,
					Name: m[1], Def: normalizeCheck(rest)})
			}
		default:
			if reIdent.MatchString(fields[0]) {
				col := fields[0]
				c.Entries = append(c.Entries, CatalogEntry{Kind: "column", Table: table,
					Name: col, Def: columnType(fields[1:])})
				rest := part[len(col):]
				if strings.Contains(strings.ToUpper(rest), "NOT NULL") {
					c.Entries = append(c.Entries, CatalogEntry{Kind: "notnull", Table: table, Name: col})
				}
				for _, chk := range extractChecks(rest) {
					c.Entries = append(c.Entries, CatalogEntry{Kind: "check", Table: table,
						Def: normalizeCheck(chk)})
				}
				if strings.Contains(strings.ToUpper(rest), "UNIQUE") {
					c.Entries = append(c.Entries, CatalogEntry{Kind: "unique", Table: table, Def: col})
				}
				if strings.Contains(strings.ToUpper(rest), "PRIMARY KEY") {
					c.Entries = append(c.Entries, CatalogEntry{Kind: "pk", Table: table, Def: col})
				}
				if i := strings.Index(strings.ToUpper(rest), "REFERENCES "); i >= 0 {
					c.Entries = append(c.Entries, CatalogEntry{Kind: "fk", Table: table,
						Def: normalizeWS(rest[i:])})
				}
			}
		}
	}
}

func parseIndex(c *Catalog, unique bool, name, table, stmt string) {
	where := ""
	up := strings.ToUpper(stmt)
	if i := strings.Index(up, " WHERE "); i >= 0 {
		where = normalizeWS(stmt[i+7:])
	}
	kind := "index"
	if unique {
		kind = "uniqueindex"
	}
	c.Entries = append(c.Entries, CatalogEntry{Kind: kind, Table: table, Name: name, Def: where})
}

// splitTopLevel splits a CREATE TABLE body on commas at paren depth 0,
// skipping quoted strings.
func splitTopLevel(body string) []string {
	var out []string
	depth := 0
	inStr := false
	var cur strings.Builder
	for i := 0; i < len(body); i++ {
		ch := body[i]
		switch {
		case inStr:
			cur.WriteByte(ch)
			if ch == '\'' {
				inStr = false
			}
		case ch == '\'':
			inStr = true
			cur.WriteByte(ch)
		case ch == '(':
			depth++
			cur.WriteByte(ch)
		case ch == ')':
			depth--
			cur.WriteByte(ch)
		case ch == ',' && depth == 0:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(ch)
		}
	}
	out = append(out, cur.String())
	return out
}

func innerParens(s string) string {
	i := strings.Index(s, "(")
	if i < 0 {
		return s
	}
	depth := 0
	for j := i; j < len(s); j++ {
		switch s[j] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[i+1 : j]
			}
		}
	}
	return s[i+1:]
}

func extractChecks(rest string) []string {
	var out []string
	up := strings.ToUpper(rest)
	for off := 0; ; {
		i := strings.Index(up[off:], "CHECK")
		if i < 0 {
			return out
		}
		i += off
		j := i + len("CHECK")
		for j < len(rest) && (rest[j] == ' ' || rest[j] == '\n' || rest[j] == '\t') {
			j++
		}
		if j < len(rest) && rest[j] == '(' {
			out = append(out, innerParens(rest[j:]))
			off = j + len(innerParens(rest[j:])) + 2
		} else {
			off = j
		}
		if off >= len(rest) {
			return out
		}
	}
}

var wsRe = regexp.MustCompile(`\s+`)

func normalizeWS(s string) string { return wsRe.ReplaceAllString(strings.TrimSpace(s), " ") }

// normalizeCheck canonicalizes a CHECK expression for comparison with
// pg_get_constraintdef output.
func normalizeCheck(expr string) string {
	e := normalizeWS(expr)
	e = strings.ReplaceAll(e, "((", "((")
	e = strings.TrimSpace(e)
	return e
}

// columnType returns the first type token(s) of a column definition.
func columnType(rest []string) string {
	var t []string
	for _, w := range rest {
		up := strings.ToUpper(w)
		switch up {
		case "NOT", "NULL", "DEFAULT", "CHECK", "REFERENCES", "PRIMARY", "UNIQUE", "CONSTRAINT", "COLLATE", "GENERATED":
			return strings.Join(t, " ")
		}
		t = append(t, w)
	}
	return strings.Join(t, " ")
}

// ---------------------------------------------------------------------------
// Live catalog
// ---------------------------------------------------------------------------

// LiveCatalog is the actual schema surface read from pg_catalog.
type LiveCatalog struct {
	Columns     map[string][]string          // table -> "name type"
	Constraints map[string][]constraintInfo  // table -> constraints
	Indexes     map[string]map[string]string // table -> index name -> def
	Tables      map[string]bool
}

type constraintInfo struct {
	name    string
	contype string // p u c f
	def     string
}

// LoadCatalog reads tables/columns/constraints/indexes of schema public.
func LoadCatalog(ctx context.Context, dsn string) (*LiveCatalog, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)
	lc := &LiveCatalog{
		Columns:     map[string][]string{},
		Constraints: map[string][]constraintInfo{},
		Indexes:     map[string]map[string]string{},
		Tables:      map[string]bool{},
	}
	rows, err := conn.Query(ctx,
		`SELECT table_name, column_name,
		        data_type || coalesce('(' || character_maximum_length || ')','')
		   FROM information_schema.columns
		  WHERE table_schema='public' ORDER BY table_name, ordinal_position`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t, col, typ string
		if err := rows.Scan(&t, &col, &typ); err != nil {
			rows.Close()
			return nil, err
		}
		lc.Tables[t] = true
		lc.Columns[t] = append(lc.Columns[t], col+" "+typ)
	}
	rows.Close()

	rows, err = conn.Query(ctx,
		`SELECT con.conrelid::regclass::text, con.conname, con.contype,
		        pg_get_constraintdef(con.oid)
		   FROM pg_constraint con
		   JOIN pg_namespace n ON n.oid = con.connamespace
		  WHERE n.nspname='public' ORDER BY 1, 2`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t, name, ct, def string
		if err := rows.Scan(&t, &name, &ct, &def); err != nil {
			rows.Close()
			return nil, err
		}
		lc.Constraints[t] = append(lc.Constraints[t], constraintInfo{name, ct, def})
	}
	rows.Close()

	rows, err = conn.Query(ctx,
		`SELECT tablename, indexname, indexdef FROM pg_indexes
		  WHERE schemaname='public' ORDER BY 1, 2`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t, name, def string
		if err := rows.Scan(&t, &name, &def); err != nil {
			rows.Close()
			return nil, err
		}
		if lc.Indexes[t] == nil {
			lc.Indexes[t] = map[string]string{}
		}
		lc.Indexes[t][name] = def
	}
	rows.Close()
	return lc, nil
}

// Verify asserts every parsed catalog entry exists in the live database and
// that no unexpected tables/constraints exist (per-constraint snapshot).
func (c *Catalog) Verify(live *LiveCatalog) []string {
	var details []string
	wantTables := map[string]bool{}
	for _, e := range c.Entries {
		if e.Kind == "table" {
			wantTables[e.Table] = true
		}
	}
	for t := range live.Tables {
		if !wantTables[t] && t != "schema_migrations" {
			details = append(details, "unexpected table "+t)
		}
	}
	for _, e := range c.Entries {
		switch e.Kind {
		case "table":
			if !live.Tables[e.Table] {
				details = append(details, "missing table "+e.Table)
			}
		case "column":
			if !hasColumn(live.Columns[e.Table], e.Name) {
				details = append(details, fmt.Sprintf("missing column %s.%s", e.Table, e.Name))
			}
		case "notnull":
			if !hasColumn(live.Columns[e.Table], e.Name) {
				continue
			}
		case "pk":
			if !hasConstraint(live.Constraints[e.Table], "p", e.Name, e.Def) {
				details = append(details, fmt.Sprintf("missing pk on %s (%s)", e.Table, e.Name+e.Def))
			}
		case "unique":
			if !hasConstraint(live.Constraints[e.Table], "u", e.Name, e.Def) &&
				!hasUniqueIndex(live.Indexes[e.Table], e.Def) {
				details = append(details, fmt.Sprintf("missing unique on %s (%s)", e.Table, e.Name+e.Def))
			}
		case "fk":
			if !hasFK(live.Constraints[e.Table], e.Def) {
				details = append(details, fmt.Sprintf("missing fk on %s: %s", e.Table, e.Def))
			}
		case "check":
			if !hasCheck(live.Constraints[e.Table], e.Name, e.Def) {
				details = append(details, fmt.Sprintf("missing check on %s (%s): %s", e.Table, e.Name, e.Def))
			}
		case "index", "uniqueindex":
			idef, ok := live.Indexes[e.Table][e.Name]
			if !ok {
				details = append(details, fmt.Sprintf("missing index %s on %s", e.Name, e.Table))
			} else if e.Def != "" && !indexPredicateMatches(idef, e.Def) {
				details = append(details, fmt.Sprintf("index %s missing predicate %q", e.Name, e.Def))
			}
		}
	}
	return details
}

func hasColumn(cols []string, name string) bool {
	for _, c := range cols {
		if strings.HasPrefix(c, name+" ") {
			return true
		}
	}
	return false
}

func hasConstraint(list []constraintInfo, contype, name, def string) bool {
	defCols := constraintCols(def)
	for _, c := range list {
		if c.contype != contype {
			continue
		}
		if name != "" && c.name == name {
			return true
		}
		if defCols != "" && strings.Contains(normalizeWS(c.def), defCols) {
			return true
		}
	}
	return false
}

func hasUniqueIndex(indexes map[string]string, def string) bool {
	cols := constraintCols(def)
	for _, idef := range indexes {
		if strings.Contains(idef, "UNIQUE") && cols != "" && strings.Contains(idef, cols) {
			return true
		}
	}
	return false
}

func hasFK(list []constraintInfo, def string) bool {
	// parsed def: "FOREIGN KEY (a,b) REFERENCES t(x,y) ON DELETE ..."
	var refs string
	up := strings.ToUpper(def)
	if i := strings.Index(up, "REFERENCES "); i >= 0 {
		refs = normalizeWS(def[i+len("REFERENCES "):])
	} else {
		refs = normalizeWS(def)
	}
	for _, c := range list {
		if c.contype != "f" {
			continue
		}
		target := normalizeWS(c.def)
		// c.def looks like: FOREIGN KEY (cols) REFERENCES reftable(col) ...
		if i := strings.Index(strings.ToUpper(target), "REFERENCES "); i >= 0 {
			target = normalizeWS(target[i+len("REFERENCES "):])
		}
		if fkTargetMatches(refs, target) {
			return true
		}
	}
	return false
}

func fkTargetMatches(want, got string) bool {
	wi := strings.Index(want, "(")
	gi := strings.Index(got, "(")
	if wi < 0 || gi < 0 {
		return false
	}
	wname := strings.TrimPrefix(strings.TrimSpace(want[:wi]), "public.")
	gname := strings.TrimPrefix(strings.TrimSpace(got[:gi]), "public.")
	wsuf := strings.TrimSpace(want[wi:])
	gsuf := strings.TrimSpace(got[gi:])
	// pg's deparse elides INITIALLY IMMEDIATE (the default).
	wsuf = strings.ReplaceAll(wsuf, " INITIALLY IMMEDIATE", "")
	gsuf = strings.ReplaceAll(gsuf, " INITIALLY IMMEDIATE", "")
	return strings.EqualFold(wname, gname) && wsuf == gsuf
}

// indexPredicateMatches compares a parsed partial-index WHERE clause
// against the live indexdef, both reduced through canonCheck.
func indexPredicateMatches(indexdef, parsedPredicate string) bool {
	pred := indexdef
	if i := strings.Index(strings.ToUpper(pred), " WHERE "); i >= 0 {
		pred = pred[i+len(" WHERE "):]
	}
	return canonCheck(pred) == canonCheck(parsedPredicate)
}

var (
	// pg type names (multi-word first: alternation is leftmost-first).
	reCast = regexp.MustCompile(
		`::(?:timestamp with time zone|double precision|character varying|` +
			`timestamptz|smallint|bigint|integer|boolean|timestamp|varchar|` +
			`numeric|decimal|jsonb|bytea|interval|uuid|text|date|real|float8|` +
			`float4|int8|int4|int2|int|char|bpchar|bit|time|json|inet)` +
			`(?:\s*\(\d+(?:\s*,\s*\d+)*\))?(?:\s*\[\s*\])*`)
	reAny = regexp.MustCompile(
		`\(?([a-zA-Z_][a-zA-Z0-9_.]*)\)?\s*=\s*ANY\s*\(+\s*ARRAY\s*\[([^\]]*)\]\s*\)+`)
	reBoolKw       = regexp.MustCompile(`\b(TRUE|FALSE)\b`)
	reInListParens = regexp.MustCompile(`\bIN\s*\(\s*\(`)
	reNotAll       = regexp.MustCompile(
		`\(?([a-zA-Z_][a-zA-Z0-9_.]*)\)?\s*<>\s*ALL\s*\(+\s*ARRAY\s*\[([^\]]*)\]\s*\)+`)
	reNotDistinct = regexp.MustCompile(
		`NOT\s*\(\s*([a-zA-Z_][a-zA-Z0-9_.]*)\s+IS\s+DISTINCT\s+FROM\s+([a-zA-Z_][a-zA-Z0-9_.]*)\s*\)`)
	reFlatBetween = regexp.MustCompile(
		`([a-zA-Z_][a-zA-Z0-9_.()]*(?: [a-zA-Z_][a-zA-Z0-9_.()]*)?) >= (\S+) AND ([a-zA-Z_][a-zA-Z0-9_.()]*(?: [a-zA-Z_][a-zA-Z0-9_.()]*)?) <= (\S+)`)
	reFlatSingleIn = regexp.MustCompile(
		`\b([a-zA-Z_][a-zA-Z0-9_.()]*) IN ('[^']*')(\s+[^\s']|\s*$)`)
	reInterval = regexp.MustCompile(
		`(?i)\bINTERVAL\s+'(\d+)\s+(seconds?|minutes?|hours?|days?)'`)
	reOuterCheck = regexp.MustCompile(`^\s*CHECK\b`)
)

// canonCheck renders a CHECK expression (authored DDL or pg's deparse) into
// a canonical token stream: casts stripped, `x = ANY (ARRAY[..])` rewritten
// to `x IN (..)`, intervals to their deparsed literal form, punctuation
// dropped. Probe tests cover constraint semantics; this is the mechanical
// "did every authored CHECK make it into the catalog" assertion.
func canonCheck(s string) string {
	s = reOuterCheck.ReplaceAllString(s, "")
	s = reCast.ReplaceAllString(s, "")
	s = reAny.ReplaceAllString(s, "$1 IN ($2)")
	s = reNotAll.ReplaceAllString(s, "$1 NOT IN ($2)")
	s = reNotDistinct.ReplaceAllString(s, "$1 IS NOT DISTINCT FROM $2")
	s = reInListParens.ReplaceAllString(s, "IN (")
	s = reInterval.ReplaceAllStringFunc(s, func(m string) string {
		parts := reInterval.FindStringSubmatch(m)
		n, _ := strconv.Atoi(parts[1])
		switch strings.ToLower(parts[2][:1]) {
		case "s":
			return fmt.Sprintf("'00:%02d:%02d'", n/60, n%60)
		case "m":
			return fmt.Sprintf("'%02d:%02d:00'", n/60, n%60)
		case "h":
			if n%24 == 0 {
				return fmt.Sprintf("'%d day%s'", n/24, plural(n/24))
			}
			return fmt.Sprintf("'%02d:00:00'", n)
		default: // days
			return fmt.Sprintf("'%d day%s'", n, plural(n))
		}
	})
	s = reBoolKw.ReplaceAllStringFunc(s, strings.ToLower)
	s = normalizeWS(strings.NewReplacer(
		"(", " ", ")", " ", ",", " ").Replace(s))
	// x >= a AND x <= b  ->  x BETWEEN a AND b (RE2 has no backrefs;
	// verify the two operands match in code; skip non-matches).
	for off := 0; off < len(s); {
		m := reFlatBetween.FindStringSubmatchIndex(s[off:])
		if m == nil {
			break
		}
		a := func(i int) int { return off + m[i] }
		if s[a(2):a(3)] != s[a(6):a(7)] {
			off = a(0) + 1
			continue
		}
		s = s[:a(0)] + s[a(2):a(3)] + " BETWEEN " + s[a(4):a(5)] +
			" AND " + s[a(8):a(9)] + s[a(1):]
	}
	s = reFlatSingleIn.ReplaceAllString(s, "$1 = $2$3")
	return normalizeWS(s)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// reInListParens drops the doubled parens pg emits around rewritten
// ANY-arrays: `IN ((...))` -> `IN (...)`.

func hasCheck(list []constraintInfo, name, expr string) bool {
	want := canonCheck(expr)
	for _, c := range list {
		if c.contype != "c" {
			continue
		}
		if name != "" && c.name == name {
			return true
		}
		if canonCheck(c.def) == want {
			return true
		}
	}
	return false
}

func constraintCols(def string) string {
	if i := strings.Index(def, "("); i >= 0 {
		inner := innerParens(def[i:])
		return "(" + normalizeWS(inner) + ")"
	}
	return def
}

func countPublicTables(ctx context.Context, dsn string) (int, error) {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return 0, err
	}
	defer conn.Close(ctx)
	var n int
	err = conn.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables
		  WHERE table_schema='public' AND table_name<>'schema_migrations'`).Scan(&n)
	return n, err
}

// diffDump returns "" when identical else the first differing line pair.
func diffDump(want, got string) string {
	wl := strings.Split(strings.TrimSpace(want), "\n")
	gl := strings.Split(strings.TrimSpace(got), "\n")
	if len(wl) != len(gl) {
		return fmt.Sprintf("line count %d != %d", len(wl), len(gl))
	}
	for i := range wl {
		if wl[i] != gl[i] {
			return fmt.Sprintf("line %d:\n- %s\n+ %s", i+1, wl[i], gl[i])
		}
	}
	return ""
}

// SortedTables lists expected table names (for tests asserting inventory).
func (c *Catalog) SortedTables() []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range c.Entries {
		if e.Kind == "table" && !seen[e.Table] {
			seen[e.Table] = true
			out = append(out, e.Table)
		}
	}
	sort.Strings(out)
	return out
}

var randAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

func randSuffix() string {
	var b [10]byte
	// crypto/rand avoids math/rand (forbidden by AGENTS.md).
	if _, err := crand.Read(b[:]); err != nil {
		panic(err)
	}
	for i := range b {
		b[i] = randAlphabet[int(b[i])%len(randAlphabet)]
	}
	return string(b[:])
}
