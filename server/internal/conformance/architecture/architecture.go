// Package architecture implements the Q4 architecture gates of
// docs/10_implementation/architecture_conformance.md §4 rules 1-11 (import
// fences, one production main, generated boundaries, schema prohibitions,
// asmdef conformance, test placement) plus rules 13-14 (client API fence and
// canonical implementations, client_fence.go). Exported Check* entry points
// return violation details ([]string, empty = pass) for the gates.Runner
// evaluator cases wired by the IMP-000 gatefix.
package architecture

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	serverRoot     = "server"
	layoutFile     = "docs/10_implementation/repository_layout.md"
	asmdefExt      = ".asmdef"
	migrationsDir  = "server/migrations"
	protocolDir    = "client/Assets/Scripts/Protocol/"
	observabilityD = "server/internal/observability/"
)

// allowedToolMains are the exact non-production main packages (rule 6).
var allowedToolMains = map[string]bool{
	"server/cmd/server":   true,
	"server/cmd/compiler": true,
	"server/cmd/verify":   true,
	"server/cmd/migrate":  true,
	"server/internal/conformance/caching/cmd/cachemerge": true,
}

// sqlOwners are the packages allowed to import database/sql or pgx (rule 2).
var sqlOwners = []string{
	"server/internal/durable/",
	"server/internal/stackpin/",
	"server/internal/conformance/",
	"server/internal/testing/",
	"server/cmd/migrate/",
}

// forbiddenModulePrefixes implement rule 1: imports matching any of these are
// rejected in first-party Go sources (transitive module presence in go.sum is
// fine — the check is on imports).
var forbiddenModulePrefixes = []string{
	"github.com/gin-gonic/gin",
	"github.com/go-chi/",
	"github.com/labstack/echo",
	"github.com/gofiber/fiber",
	"github.com/gorilla/",
	"github.com/redis/",
	"github.com/go-redis/",
	"github.com/segmentio/kafka",
	"github.com/confluentinc/",
	"github.com/Shopify/sarama",
	"github.com/nats-io/",
	"gorm.io/",
	"github.com/jmoiron/sqlx",
	"go.uber.org/zap",
	"github.com/sirupsen/logrus",
	"github.com/rs/zerolog",
	"google.golang.org/grpc",
	"go.opentelemetry.io/proto/otlp/trace/grpc",
	"go.opentelemetry.io/proto/otlp/metrics/grpc",
	"go.opentelemetry.io/proto/otlp/logs/grpc",
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc",
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc",
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc",
}

// schemaForbidden implements rule 8.
var schemaForbidden = []*regexp.Regexp{
	regexp.MustCompile(`\bglobal_leader_lease\b`),
	regexp.MustCompile(`\bitem_instances\b.*\bdurability\b|\bdurability\b.*\bitem_instances\b`),
}

// goFile is a parsed first-party Go source file.
type goFile struct {
	rel     string // repo-relative path
	pkg     string
	isMain  bool
	imports []string
}

// goImports builds path -> []imports for a supplied file list (fixture-aware:
// files may live anywhere under root, including _testdata dirs).
func scanGoTree(root, dir string, skipTestdata bool) ([]goFile, error) {
	var out []goFile
	base := path.Join(root, dir)
	err := filepath.Walk(base, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skipTestdata && (info.Name() == "testdata" || info.Name() == "_testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, p, nil, parser.ImportsOnly)
		if perr != nil {
			return fmt.Errorf("parse %s: %w", rel, perr)
		}
		gf := goFile{rel: filepath.ToSlash(rel), pkg: f.Name.Name, isMain: f.Name.Name == "main"}
		for _, im := range f.Imports {
			gf.imports = append(gf.imports, strings.Trim(im.Path.Value, `"`))
		}
		out = append(out, gf)
		return nil
	})
	return out, err
}

func hasImport(g goFile, prefixes ...string) (string, bool) {
	for _, im := range g.imports {
		for _, pre := range prefixes {
			if im == pre || strings.HasPrefix(im, pre) {
				return im, true
			}
		}
	}
	return "", false
}

// isFixtureDir names test-fixture trees any repo-wide walk must skip:
// mutation corpora live under _testdata/testdata and would self-flag.
func isFixtureDir(name string) bool {
	return name == "testdata" || name == "_testdata" || name == "vendor"
}

func under(p, prefix string) bool { return strings.HasPrefix(p, prefix) }

// CheckImportDirection enforces dependency-direction rules 2-5 and 10 on the
// Go import graph (SQL ownership, sim/durable/protocol isolation,
// observability no-imports-back). Fixture-aware variant: checkImports.
func CheckImportDirection(root string) []string {
	files, err := scanGoTree(root, serverRoot, true)
	if err != nil {
		return []string{"scan go files: " + err.Error()}
	}
	return checkImports(files)
}

func checkImports(files []goFile) []string {
	var out []string
	sqlImports := []string{"database/sql", "github.com/jackc/pgx"}
	for _, f := range files {
		inSim := under(f.rel, "server/internal/sim/")
		inDur := under(f.rel, "server/internal/durable/")
		inProto := under(f.rel, "server/internal/protocol/") || under(f.rel, "server/internal/durable/journal/")
		inObs := under(f.rel, observabilityD)
		sqlAllowed := false
		for _, o := range sqlOwners {
			if under(f.rel, o) {
				sqlAllowed = true
				break
			}
		}
		if !sqlAllowed {
			if im, ok := hasImport(f, sqlImports...); ok {
				out = append(out, fmt.Sprintf("%s imports %s outside SQL owners (durable/stackpin/conformance/testing/migrate)", f.rel, im))
			}
		}
		if inSim {
			if im, ok := hasImport(f, append(append([]string{}, sqlImports...),
				"thinhthan/internal/edge", "thinhthan/server/migrations")...); ok {
				out = append(out, fmt.Sprintf("%s: sim imports %s (SQL/edge forbidden)", f.rel, im))
			}
		}
		if inDur && !strings.Contains(f.rel, "/journal/") {
			if im, ok := hasImport(f, "thinhthan/internal/sim"); ok {
				out = append(out, fmt.Sprintf("%s: durable imports %s (sim forbidden)", f.rel, im))
			}
		}
		if inProto {
			if im, ok := hasImport(f, "thinhthan/internal/sim", "thinhthan/internal/edge",
				"thinhthan/internal/global", "thinhthan/internal/observability"); ok {
				out = append(out, fmt.Sprintf("%s: generated protocol imports domain/runtime %s", f.rel, im))
			}
		}
		if inObs {
			if im, ok := hasImport(f, "thinhthan/internal/sim", "thinhthan/internal/edge",
				"thinhthan/internal/durable", "thinhthan/internal/global"); ok {
				out = append(out, fmt.Sprintf("%s: observability imports %s (no imports back)", f.rel, im))
			}
		}
	}
	return out
}

// CheckOneProductionMain enforces rule 6: only server/cmd/server is a
// production main; the exact tool-main exceptions are whitelisted.
func CheckOneProductionMain(root string) []string {
	files, err := scanGoTree(root, serverRoot, true)
	if err != nil {
		return []string{"scan go files: " + err.Error()}
	}
	return checkMains(files)
}

func checkMains(files []goFile) []string {
	var out []string
	for _, f := range files {
		if !f.isMain {
			continue
		}
		dir := path.Dir(f.rel)
		if !allowedToolMains[dir] {
			out = append(out, fmt.Sprintf("%s: package main outside allowed production/tool mains", f.rel))
		}
	}
	return out
}

// CheckForbiddenDeps enforces rule 1 (forbidden imports incl. first-party
// gRPC and legacy math/rand) and rule 8 (schema prohibitions).
func CheckForbiddenDeps(root string) []string {
	files, err := scanGoTree(root, serverRoot, true)
	if err != nil {
		return []string{"scan go files: " + err.Error()}
	}
	var out []string
	out = append(out, checkForbiddenImports(files)...)
	out = append(out, checkSchema(root)...)
	return out
}

func checkForbiddenImports(files []goFile) []string {
	var out []string
	for _, f := range files {
		for _, im := range f.imports {
			if im == "math/rand" {
				out = append(out, fmt.Sprintf("%s imports legacy math/rand (use math/rand/v2 or crypto/rand)", f.rel))
				continue
			}
			for _, pre := range forbiddenModulePrefixes {
				if im == pre || strings.HasPrefix(im, pre) {
					out = append(out, fmt.Sprintf("%s imports forbidden dependency %s", f.rel, im))
					break
				}
			}
		}
	}
	return out
}

func checkSchema(root string) []string {
	var out []string
	_ = filepath.Walk(path.Join(root, migrationsDir), func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if isFixtureDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".sql") {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		for i, line := range strings.Split(string(b), "\n") {
			for _, re := range schemaForbidden {
				if re.MatchString(line) {
					out = append(out, fmt.Sprintf("%s:%d schema prohibition: %s", rel, i+1, strings.TrimSpace(line)))
				}
			}
		}
		return nil
	})
	return out
}

// CheckGeneratedBoundary enforces rules 7 (each .pb.go references its proto
// source), 9 (asmdef conformance: acyclic, IMP-000 table parity, ThinhThan.App
// only referenced by Tests.PlayMode, ThinhThan.Protocol with no project refs)
// and 11 (test placement).
func CheckGeneratedBoundary(root string) []string {
	var out []string
	out = append(out, checkPbGo(root)...)
	out = append(out, checkAsmdefs(root)...)
	out = append(out, checkTestPlacement(root)...)
	return out
}

var pbSourceRe = regexp.MustCompile(`//\s*source:\s*(\S+\.proto)`)

func checkPbGo(root string) []string {
	var out []string
	_ = filepath.Walk(path.Join(root, serverRoot), func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if isFixtureDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".pb.go") {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		m := pbSourceRe.FindSubmatch(b)
		if m == nil {
			out = append(out, fmt.Sprintf("%s: .pb.go has no `// source:` proto reference", rel))
			return nil
		}
		src := string(m[1])
		if !strings.HasPrefix(src, "proto/") {
			src = path.Join("proto", src)
		}
		if st, serr := os.Stat(path.Join(root, src)); serr != nil || st.IsDir() {
			out = append(out, fmt.Sprintf("%s: referenced proto source %s missing", rel, src))
		}
		return nil
	})
	return out
}

// asmdef is the subset of an .asmdef file the fence needs.
type asmdef struct {
	Name       string   `json:"name"`
	References []string `json:"references"`
	file       string
}

var asmdefTableRowRe = regexp.MustCompile("`" + `(ThinhThan\.[A-Za-z.]+)` + "`" + `\s*\|`)

// mandatoryAssemblies parses repository_layout.md § Mandatory Assemblies for
// the canonical name set (IMP-000-owned reference graph).
func mandatoryAssemblies(root string) ([]string, error) {
	b, err := os.ReadFile(path.Join(root, layoutFile))
	if err != nil {
		return nil, err
	}
	var names []string
	inTable := false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "## ") {
			inTable = strings.Contains(line, "Mandatory Assemblies")
			continue
		}
		if inTable && strings.HasPrefix(strings.TrimSpace(line), "|") {
			if m := asmdefTableRowRe.FindStringSubmatch(line); m != nil {
				names = append(names, m[1])
			}
		}
	}
	return names, nil
}

func checkAsmdefs(root string) []string {
	var out []string
	var defs []asmdef
	_ = filepath.Walk(path.Join(root, "client"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, asmdefExt) {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		var a asmdef
		if jerr := json.Unmarshal(b, &a); jerr != nil {
			rel, _ := filepath.Rel(root, p)
			out = append(out, fmt.Sprintf("%s: invalid asmdef JSON", rel))
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		a.file = rel
		defs = append(defs, a)
		return nil
	})
	return append(out, checkAsmdefSet(defs, root)...)
}

func checkAsmdefSet(defs []asmdef, root string) []string {
	var out []string
	names := map[string]asmdef{}
	for _, d := range defs {
		if names[d.Name].file != "" {
			out = append(out, fmt.Sprintf("duplicate asmdef name %s (%s, %s)", d.Name, names[d.Name].file, d.file))
		}
		names[d.Name] = d
	}
	table, err := mandatoryAssemblies(root)
	if err != nil {
		out = append(out, "repository_layout.md: "+err.Error())
	} else {
		want := map[string]bool{}
		for _, n := range table {
			want[n] = true
			if names[n].file == "" {
				out = append(out, fmt.Sprintf("mandatory assembly %s has no .asmdef", n))
			}
		}
		for _, d := range defs {
			if !want[d.Name] {
				out = append(out, fmt.Sprintf("%s: asmdef %s not in § Mandatory Assemblies table", d.file, d.Name))
			}
		}
	}
	// Reference resolution + acyclicity over ThinhThan.* references.
	color := map[string]int{}
	var stack []string
	var visit func(n string) bool
	visit = func(n string) bool {
		color[n] = 1
		stack = append(stack, n)
		for _, r := range names[n].References {
			r = strings.TrimPrefix(r, "GUID:")
			if _, ok := names[r]; !ok {
				continue
			}
			if color[r] == 1 {
				out = append(out, "asmdef reference cycle: "+strings.Join(append(stack, r), " -> "))
				return false
			}
			if color[r] == 0 && !visit(r) {
				return false
			}
		}
		stack = stack[:len(stack)-1]
		color[n] = 2
		return true
	}
	for _, d := range defs {
		if color[d.Name] == 0 {
			visit(d.Name)
		}
	}
	for _, d := range defs {
		for _, r := range d.References {
			r = strings.TrimPrefix(r, "GUID:")
			if strings.HasPrefix(r, "ThinhThan.") && names[r].file == "" {
				out = append(out, fmt.Sprintf("%s references unknown project assembly %s", d.file, r))
			}
			if r == "ThinhThan.App" && d.Name != "ThinhThan.Tests.PlayMode" {
				out = append(out, fmt.Sprintf("%s references ThinhThan.App (only ThinhThan.Tests.PlayMode may)", d.file))
			}
		}
		if d.Name == "ThinhThan.Protocol" {
			for _, r := range d.References {
				if strings.HasPrefix(strings.TrimPrefix(r, "GUID:"), "ThinhThan.") {
					out = append(out, fmt.Sprintf("%s: ThinhThan.Protocol references project assembly %s", d.file, r))
				}
			}
		}
	}
	return out
}

// checkTestPlacement enforces rule 11: Go tests live in the package they
// test (a _test.go's package is its dir's package or that package + "_test";
// a dir of only test files is itself a package); Unity tests live under
// client/Assets/Tests/{EditMode|PlayMode}/.
func checkTestPlacement(root string) []string {
	var out []string
	// Go: package consistency per dir.
	type dirPkgs struct {
		src   map[string]bool
		tests map[string][]string // test package name -> files
	}
	dirs := map[string]*dirPkgs{}
	_ = filepath.Walk(path.Join(root, serverRoot), func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if isFixtureDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, p, nil, parser.PackageClauseOnly)
		if perr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		dir := path.Dir(rel)
		d := dirs[dir]
		if d == nil {
			d = &dirPkgs{src: map[string]bool{}, tests: map[string][]string{}}
			dirs[dir] = d
		}
		if strings.HasSuffix(rel, "_test.go") {
			d.tests[f.Name.Name] = append(d.tests[f.Name.Name], rel)
		} else {
			d.src[f.Name.Name] = true
		}
		return nil
	})
	for dir, d := range dirs {
		for tpkg, files := range d.tests {
			ok := len(d.src) == 0 // test-only dir: the tests are the package
			for src := range d.src {
				if tpkg == src || tpkg == src+"_test" {
					ok = true
				}
			}
			if !ok {
				for _, f := range files {
					out = append(out, fmt.Sprintf("%s: Go test package %s outside tested package(s) in %s", f, tpkg, dir))
				}
			}
		}
	}
	// Unity: C# tests live only under client/Assets/Tests/{EditMode|PlayMode}/.
	_ = filepath.Walk(path.Join(root, "client"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".cs") {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "client/Assets/Tests/") {
			if !strings.HasPrefix(rel, "client/Assets/Tests/EditMode/") &&
				!strings.HasPrefix(rel, "client/Assets/Tests/PlayMode/") {
				out = append(out, fmt.Sprintf("%s: Unity test outside Tests/{EditMode|PlayMode}/", rel))
			}
			return nil
		}
		if strings.HasSuffix(rel, "Tests.cs") || strings.HasSuffix(rel, "Test.cs") {
			out = append(out, fmt.Sprintf("%s: test-named file outside client/Assets/Tests/", rel))
		}
		return nil
	})
	return out
}

// CheckArch is the aggregate Q4.arch evaluator entry (rules 1-11).
func CheckArch(root string) []string {
	var out []string
	out = append(out, CheckImportDirection(root)...)
	out = append(out, CheckOneProductionMain(root)...)
	out = append(out, CheckForbiddenDeps(root)...)
	out = append(out, CheckGeneratedBoundary(root)...)
	return out
}
