package gates

import (
	"bufio"
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"thinhthan/internal/conformance/style"
)

// Registry is the canonical, ordered gate list. IMP-000 evaluates its own
// gates live; gates owned by later tasks emit SKIP(owner-not-done) rows until
// their owner reaches DONE on main or head.
var Registry = []GateSpec{
	{ID: "Q0.absent_paths", Owner: "IMP-000", Phase: PhasePreUnity},
	{ID: "Q0.ci", Owner: "IMP-000", Phase: PhasePreUnity},
	{ID: "Q0.claims", Owner: "IMP-083", Phase: PhasePreUnity},
	{ID: "Q0.control.diff", Owner: "IMP-083", Phase: PhasePreUnity},
	{ID: "Q0.dag", Owner: "IMP-083", Phase: PhasePreUnity},
	{ID: "Q0.ratchet", Owner: "IMP-068", Phase: PhasePreUnity},
	{ID: "Q0.req.coverage", Owner: "IMP-083", Phase: PhasePreUnity},
	{ID: "Q1.forbidden_deps", Owner: "IMP-000", Phase: PhasePreUnity},
	{ID: "Q1.pins", Owner: "IMP-000", Phase: PhasePreUnity},
	{ID: "Q1.unity.editor", Owner: "IMP-000", OS: "windows", Phase: PhaseUnity},
	{ID: "Q2.codegen", Owner: "IMP-061", Phase: PhasePreUnity},
	{ID: "Q3.go.alloc", Owner: "IMP-000", Phase: PhasePreUnity},
	{ID: "Q3.go.bench", Owner: "IMP-000", Phase: PhasePreUnity},
	{ID: "Q3.go.race", Owner: "IMP-000", OS: "linux", Phase: PhasePreUnity},
	{ID: "Q3.go.test", Owner: "IMP-000", Phase: PhasePreUnity},
	{ID: "Q3.unity.compile", Owner: "IMP-000", OS: "windows", Phase: PhaseUnity},
	{ID: "Q3.unity.editmode", Owner: "IMP-000", OS: "windows", Phase: PhaseUnity},
	{ID: "Q3.unity.loadgraphics", Owner: "IMP-067", OS: "windows", Phase: PhaseUnity},
	{ID: "Q3.unity.performance", Owner: "IMP-095", OS: "windows", Phase: PhaseUnity},
	{ID: "Q3.unity.playmode", Owner: "IMP-065", OS: "windows", Phase: PhaseUnity},
	{ID: "Q3.unity.visualreview", Owner: "IMP-070", OS: "windows", Phase: PhaseUnity},
	{ID: "Q4.api_fence", Owner: "IMP-083", Phase: PhasePreUnity},
	{ID: "Q4.arch", Owner: "IMP-083", Phase: PhasePreUnity},
	{ID: "Q4.canonical", Owner: "IMP-083", Phase: PhasePreUnity},
	{ID: "Q4.cscrsp", Owner: "IMP-000", Phase: PhasePreUnity},
	{ID: "Q4.dotfiles", Owner: "IMP-000", Phase: PhasePreUnity},
	{ID: "Q4.go.static", Owner: "IMP-000", Phase: PhasePreUnity},
	{ID: "Q4.style", Owner: "IMP-000", Phase: PhasePreUnity},
	{ID: "Q5.content_activation", Owner: "IMP-004", Phase: PhasePreUnity},
	{ID: "Q5.content_compile", Owner: "IMP-003", Phase: PhasePreUnity},
	{ID: "Q5.migrations", Owner: "IMP-005", Phase: PhasePreUnity},
	{ID: "Q5.schema", Owner: "IMP-005", Phase: PhasePreUnity},
	{ID: "Q6.clean_tree", Owner: "IMP-000", Phase: PhasePreUnity},
	{ID: "Q6.evidence", Owner: "IMP-000", Phase: PhasePreUnity},
	{ID: "Q6.ratchet", Owner: "IMP-068", Phase: PhasePreUnity},
}

// APICheckFunc confirms a ci_run_id/run_attempt belongs to workflow
// verify.yml with conclusion success (Q6.evidence, added manifests only).
type APICheckFunc func(runID, attempt string) (workflow, conclusion string, err error)

// Runner executes the gate registry for one phase.
type Runner struct {
	Root       string
	Ctx        RunContext
	Commands   []CommandRecord
	Bench      []BenchRecord
	Summary    TestSummary
	UnityDir   string // unity test-results dir (phase=unity)
	PreReport  *Report
	AddedPaths []string     // files added by the PR diff (Q6.evidence)
	APICheck   APICheckFunc // nil = skip API validation (local runs)
	benchDone  bool         // Q3.go.alloc and Q3.go.bench share one benchmark run
}

// Run evaluates the registry and returns a populated Report (not yet written).
func (r *Runner) Run(phase string) (*Report, error) {
	rep := &Report{Schema: SchemaReport, OS: r.Ctx.OSTarget, Phase: phase}
	if phase == PhasePreUnity {
		rep.Schema = SchemaPreReport
	}
	rep.HeadSHA = gitHeadSHA(r.Root)
	rep.SourceTreeHash, _ = SourceTreeHash(r.Root)

	var rows []GateRow
	if phase == PhaseUnity && r.PreReport != nil {
		rows = append(rows, r.PreReport.Gates...)
	}
	for _, spec := range Registry {
		if phase != "" && spec.Phase != phase {
			continue
		}
		row := GateRow{ID: spec.ID, OS: r.Ctx.OSTarget, Owner: spec.Owner}
		v := gateVerdict(spec, r.Ctx)
		switch {
		case v == "absent":
			continue
		case v != "":
			row.Result = ResultSkip
			row.Reason = v
		default:
			fmt.Fprintf(os.Stderr, "verify: evaluating %s\n", spec.ID)
			row = r.evaluate(spec)
			fmt.Fprintf(os.Stderr, "verify: %s -> %s\n", spec.ID, row.Result)
		}
		rows = append(rows, row)
	}
	rep.Gates = rows
	rep.Commands = r.Commands
	rep.Bench = r.Bench
	rep.TestSummary = r.Summary
	if AnyFail(rows) {
		rep.Conclusion = "FAILED"
	} else {
		rep.Conclusion = "PASSED"
	}
	return rep, nil
}

// LoadPreReport reads a verify-pre-unity-v1 report for the unity phase. A
// missing/invalid/foreign-HEAD file fails closed: the caller must run the
// pre-unity gates itself rather than trusting stale rows.
func LoadPreReport(path, headSHA string) (*Report, error) {
	r, err := ReadReport(path, SchemaPreReport)
	if err != nil {
		return nil, err
	}
	if r.HeadSHA != headSHA {
		return nil, fmt.Errorf("pre-report head_sha %s != HEAD %s", r.HeadSHA, headSHA)
	}
	return r, nil
}

func (r *Runner) evaluate(spec GateSpec) GateRow {
	row := GateRow{ID: spec.ID, OS: r.Ctx.OSTarget, Owner: spec.Owner}
	var details []string
	var evalErr error
	missing := false
	switch spec.ID {
	case "Q0.absent_paths":
		tracked, err := gitLsFiles(r.Root)
		if err != nil {
			evalErr = err
			break
		}
		details = CheckAbsentPaths(r.Ctx.HeadIdx, tracked)
	case "Q0.ci":
		details = CheckWorkflowLint(r.Root)
	case "Q1.pins":
		details = append(CheckPins(r.Root), checkProtobufDLL(r.Root)...)
	case "Q1.forbidden_deps":
		details = append(CheckForbiddenDeps(r.Root), CheckForbiddenImports(r.Root)...)
	case "Q1.unity.editor":
		details, missing = r.checkUnityEditor()
	case "Q3.go.test":
		details = r.goTest()
	case "Q3.go.race":
		details = r.goRace()
	case "Q3.go.alloc":
		details = r.goAlloc()
	case "Q3.go.bench":
		details = r.goBench() // report-only; never fails on numbers
	case "Q3.unity.compile":
		details, missing = r.unityCompile()
	case "Q3.unity.editmode":
		details, missing = r.unityEditMode()
	case "Q4.style":
		details = append(r.goFmtVet(), style.CheckCSharpTree(r.Root)...)
	case "Q4.dotfiles":
		details = checkDotfiles(r.Root)
	case "Q4.cscrsp":
		details = append(checkCscRsp(r.Root), checkAsmdefs(r.Root)...)
	case "Q4.go.static":
		details, missing = r.goStatic()
	case "Q6.clean_tree":
		details = r.cleanTree()
	case "Q6.evidence":
		treeHash, err := SourceTreeHash(r.Root)
		if err != nil {
			evalErr = err
			break
		}
		details = CheckEvidenceGate(r.Root, gitHeadSHA(r.Root), treeHash, r.AddedPaths, r.APICheck)
		if r.Ctx.EventName == "push" {
			// Merged-head invariant: every DONE packet on main must carry its
			// evidence manifest (post-merge guard; IMP-068 owns the ratchet).
			details = append(details, CheckMergedHeadManifests(r.Root, r.Ctx.HeadIdx)...)
		}
	default:
		evalErr = fmt.Errorf("gate %s has no evaluator", spec.ID)
	}
	switch {
	case evalErr != nil:
		row.Result = ResultFail
		row.Reason = evalErr.Error()
	case missing && r.Ctx.LocalDeferMissing && !r.Ctx.InCI:
		row.Result = ResultDeferred
		row.Reason = DeferLocalMissing
	case missing:
		row.Result = ResultFail
		row.Reason = "required inputs missing"
	case len(details) > 0:
		row.Result = ResultFail
		row.Reason = strings.Join(details, "; ")
	default:
		row.Result = ResultPass
	}
	if row.Result == ResultFail && len(row.Reason) > 2000 {
		row.Reason = row.Reason[:2000]
	}
	return row
}

// --- subprocess helpers ---------------------------------------------------

// gateCmdTimeout bounds every subprocess a gate spawns — an unbounded child
// turns into a silent multi-hour job stall instead of a named gate failure.
const gateCmdTimeout = 15 * time.Minute

func (r *Runner) runCmd(ctx context.Context, dir string, argv ...string) (string, int) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, gateCmdTimeout)
	defer cancel()
	fmt.Fprintf(os.Stderr, "verify: $ %s\n", strings.Join(argv, " "))
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	r.Commands = append(r.Commands, CommandRecord{
		Argv: argv, ExitCode: code, WallSeconds: time.Since(start).Seconds(),
	})
	return string(out), code
}

func gitLsFiles(root string) ([]string, error) {
	out, err := exec.Command("git", "-C", root, "ls-files").Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, l := range strings.Split(string(out), "\n") {
		if l != "" {
			files = append(files, l)
		}
	}
	sort.Strings(files)
	return files, nil
}

func gitHeadSHA(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// GitShowMain returns a file's contents at the main ref, "" when absent.
func GitShowMain(root, relpath, ref string) (string, error) {
	if ref == "" {
		ref = "origin/main"
	}
	out, err := exec.Command("git", "-C", root, "show", ref+":"+relpath).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// --- IMP-000-owned gate evaluators ---------------------------------------

func (r *Runner) goTest() []string {
	out, code := r.runCmd(context.Background(), r.Root, "go", "-C", "server", "build", "./...")
	if code != 0 {
		return []string{"go build: " + tail(out)}
	}
	out, code = r.runCmd(context.Background(), r.Root, "go", "-C", "server", "test", "-v", "./...")
	r.countGoTestOutput(out)
	if code != 0 {
		return []string{"go test: " + tail(out)}
	}
	return nil
}

// countGoTestOutput folds `--- PASS/FAIL/SKIP:` lines into TestSummary.
func (r *Runner) countGoTestOutput(out string) {
	for _, l := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(l, "--- PASS:"):
			r.Summary.Total++
			r.Summary.Passed++
		case strings.HasPrefix(l, "--- FAIL:"):
			r.Summary.Total++
			r.Summary.Failed++
		case strings.HasPrefix(l, "--- SKIP:"):
			r.Summary.Total++
			r.Summary.Skipped++
		}
	}
}

func (r *Runner) goRace() []string {
	// -race scope: the hot-path trees only (ADR-0078). Trees that do not yet
	// exist are skipped; if none exist the gate is vacuous.
	var dirs []string
	for _, d := range []string{"sim", "edge", "durable", "global"} {
		if fi, err := os.Stat(filepath.Join(r.Root, "server", "internal", d)); err == nil && fi.IsDir() {
			dirs = append(dirs, "./internal/"+d+"/...")
		}
	}
	if len(dirs) == 0 {
		return nil
	}
	argv := append([]string{"go", "-C", "server", "test", "-race"}, dirs...)
	out, code := r.runCmd(context.Background(), r.Root, argv...)
	if code != 0 {
		return []string{"go test -race: " + tail(out)}
	}
	return nil
}

var benchRe = regexp.MustCompile(`^Benchmark(\S+)\s+\d+\s+([\d.]+)\s+ns/op\s+(\d+)\s+B/op\s+(\d+)\s+allocs/op`)

// hotPathTrees are the packages whose benchmarks carry exact alloc gates.
var hotPathTrees = []string{"sim", "edge", "durable", "global"}

func (r *Runner) goBench() []string {
	var dirs []string
	for _, d := range hotPathTrees {
		if fi, err := os.Stat(filepath.Join(r.Root, "server", "internal", d)); err == nil && fi.IsDir() {
			dirs = append(dirs, "./internal/"+d+"/...")
		}
	}
	if len(dirs) == 0 {
		return nil
	}
	argv := append([]string{"go", "-C", "server", "test", "-run", "^$", "-bench", ".", "-benchmem", "-benchtime=200x"}, dirs...)
	out, code := r.runCmd(context.Background(), r.Root, argv...)
	r.benchDone = true
	if code != 0 {
		return []string{"go bench: " + tail(out)}
	}
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "ok \t") || strings.HasPrefix(l, "ok ") {
			continue
		}
		if m := benchRe.FindStringSubmatch(l); m != nil {
			ns, _ := strconv.ParseFloat(m[2], 64)
			b, _ := strconv.ParseInt(m[3], 10, 64)
			ao, _ := strconv.ParseInt(m[4], 10, 64)
			r.Bench = append(r.Bench, BenchRecord{Name: m[1], NsPerOp: ns, AllocB: b, AllocOps: ao})
		}
	}
	return nil
}

// goAlloc: hot-path benchmarks must report 0 allocs/op (capacity.md exact
// gates). Vacuous when no such benchmarks exist yet.
func (r *Runner) goAlloc() []string {
	if !r.benchDone {
		r.goBench() // reuse one benchmark run for both gates
	}
	var errs []string
	for _, b := range r.Bench {
		if b.AllocOps > 0 {
			errs = append(errs, fmt.Sprintf("%s allocs/op=%d (hot-path gate requires 0)", b.Name, b.AllocOps))
		}
	}
	return errs
}

func (r *Runner) goFmtVet() []string {
	var errs []string
	out, code := r.runCmd(context.Background(), r.Root, "gofmt", "-l", "server/")
	if code != 0 {
		errs = append(errs, "gofmt failed: "+tail(out))
	} else if strings.TrimSpace(out) != "" {
		errs = append(errs, "gofmt dirty: "+strings.TrimSpace(out))
	}
	out, code = r.runCmd(context.Background(), r.Root, "go", "-C", "server", "vet", "./...")
	if code != 0 {
		errs = append(errs, "go vet: "+tail(out))
	}
	return errs
}

// goStatic runs the pinned staticcheck (installed in CI; local-missing
// deferral when absent).
func (r *Runner) goStatic() (errs []string, missing bool) {
	sc, err := exec.LookPath("staticcheck")
	if err != nil {
		return nil, true
	}
	out, code := r.runCmd(context.Background(), filepath.Join(r.Root, "server"), sc, "./...")
	if code != 0 {
		return []string{"staticcheck: " + tail(out)}, false
	}
	return nil, false
}

// cleanTree (Q6): `git status --porcelain` must be empty at the end of a run
// — no verification-created drift.
func (r *Runner) cleanTree() []string {
	out, err := exec.Command("git", "-C", r.Root, "status", "--porcelain").Output()
	if err != nil {
		return []string{"git status: " + err.Error()}
	}
	if strings.TrimSpace(string(out)) != "" {
		return []string{"working tree dirty: " + tail(string(out))}
	}
	return nil
}

// --- unity-phase evaluators ------------------------------------------------

// unityResultsDir files: editmode-results.xml + materialize log + editor-version.txt.
func (r *Runner) checkUnityEditor() (errs []string, missing bool) {
	b, err := os.ReadFile(filepath.Join(r.UnityDir, "editor-version.txt"))
	if err != nil {
		return nil, true
	}
	if !strings.Contains(string(b), "6000.6.1f1") {
		return []string{"editor-version.txt does not record the pinned editor"}, false
	}
	return nil, false
}

var (
	unityErrRe  = regexp.MustCompile(`error CS\d+`)
	unityWarnRe = regexp.MustCompile(`warning CS\d+`)
)

// unityCompile fails on any `error CS`/`Scripts have compiler errors` line or
// any `warning CS` under Assets/ in the Unity editor logs (zero-warning rule).
func (r *Runner) unityCompile() (errs []string, missing bool) {
	var logs []string
	_ = filepath.Walk(r.UnityDir, func(p string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() && (strings.HasSuffix(fi.Name(), ".log") || strings.HasSuffix(fi.Name(), ".txt")) {
			logs = append(logs, p)
		}
		return nil
	})
	if len(logs) == 0 {
		return nil, true
	}
	for _, lp := range logs {
		f, err := os.Open(lp)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			l := sc.Text()
			if unityErrRe.MatchString(l) || strings.Contains(l, "Scripts have compiler errors") {
				errs = append(errs, fmt.Sprintf("%s: %s", filepath.Base(lp), strings.TrimSpace(l)[:120]))
			} else if unityWarnRe.MatchString(l) && strings.Contains(l, "Assets") {
				errs = append(errs, fmt.Sprintf("%s: %s", filepath.Base(lp), strings.TrimSpace(l)[:120]))
			}
		}
		f.Close()
	}
	sort.Strings(errs)
	return errs, false
}

// nunitRun is the minimal NUnit3 XML projection we check.
type nunitRun struct {
	XMLName xml.Name `xml:"test-run"`
	Failed  int      `xml:"failed,attr"`
	Passed  int      `xml:"passed,attr"`
	Total   int      `xml:"total,attr"`
}

// unityEditMode parses editmode-results.xml (NUnit3) from the Unity results
// dir and requires the editor's completion line; any failure/error is FAIL,
// absent file is fail-closed. The spec contract is the "Test run completed.
// Exiting with code 0" log line **plus** a Passed XML — XML alone is not
// enough because a killed editor can leave a stale passing file behind.
func (r *Runner) unityEditMode() (errs []string, missing bool) {
	var files []string
	var logs []string
	_ = filepath.Walk(r.UnityDir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return err
		}
		if strings.HasSuffix(fi.Name(), ".xml") && strings.Contains(fi.Name(), "editmode") {
			files = append(files, p)
		}
		if strings.Contains(fi.Name(), "editmode") && strings.HasSuffix(fi.Name(), ".log") {
			logs = append(logs, p)
		}
		return nil
	})
	if len(files) == 0 {
		return nil, true
	}
	completed := false
	for _, lp := range logs {
		b, err := os.ReadFile(lp)
		if err != nil {
			continue
		}
		if strings.Contains(string(b), "Test run completed. Exiting with code 0") {
			completed = true
		}
	}
	if !completed {
		errs = append(errs, "no editmode log carries the 'Test run completed. Exiting with code 0' completion line")
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			errs = append(errs, f+": "+err.Error())
			continue
		}
		var run nunitRun
		if err := xml.Unmarshal(b, &run); err != nil {
			errs = append(errs, f+": invalid NUnit XML: "+err.Error())
			continue
		}
		r.Summary.Total += run.Total
		r.Summary.Passed += run.Passed
		r.Summary.Failed += run.Failed
		r.Summary.Skipped += run.Total - run.Passed - run.Failed
		if run.Failed > 0 {
			errs = append(errs, fmt.Sprintf("%s: %d failed test(s)", filepath.Base(f), run.Failed))
		}
		if run.Total == 0 {
			errs = append(errs, filepath.Base(f)+": zero tests executed")
		}
	}
	return errs, false
}

// checkDotfiles (Q4.dotfiles): the dotfiles carry the exact required lines.
func checkDotfiles(root string) []string {
	var errs []string
	req := map[string][]string{
		".gitattributes": {
			"* text=auto eol=lf",
			"filter=lfs diff=lfs merge=lfs -text",
			"merge=unityyamlmerge",
		},
		".editorconfig": {
			"root = true",
			"end_of_line = lf",
			"insert_final_newline = true",
			"csharp_new_line_before_open_brace = all",
			"csharp_style_namespace_declarations = block_scoped",
		},
	}
	for file, wants := range req {
		b, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			errs = append(errs, file+" unreadable")
			continue
		}
		s := string(b)
		for _, w := range wants {
			if !strings.Contains(s, w) {
				errs = append(errs, fmt.Sprintf("%s missing %q", file, w))
			}
		}
	}
	return errs
}

func tail(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > 30 {
		lines = lines[len(lines)-30:]
	}
	return strings.Join(lines, "\n")
}
