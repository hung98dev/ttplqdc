// Command verify is the repo's canonical verifier entrypoint. It evaluates
// the Q0-Q6 gate registry for the current OS/phase, writes a
// verify-report-v1 JSON, and provides the support modes CI consumes:
// -plan-unity (mode plan JSON) and -merge (evidence manifest merge).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"thinhthan/internal/conformance/gates"
	"thinhthan/internal/stackpin"
)

func main() {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	root := fs.String("repo-root", "", "repo checkout root (default: nearest ancestor with .git)")
	phase := fs.String("phase", "all", "pre-unity | unity | all")
	preReport := fs.String("pre-report", "", "verify-pre-unity-v1 report to replay in the unity phase")
	unityDir := fs.String("unity-results-dir", "", "directory holding the Unity job's test results")
	reportOut := fs.String("report-out", "", "path for the emitted verify-report JSON")
	planUnity := fs.String("plan-unity", "", "emit the unity mode plan JSON to this path ('-' = stdout)")
	merge := fs.Bool("merge", false, "merge -linux/-windows verify reports into an evidence manifest")
	linux := fs.String("linux", "", "verify-report-linux.json path")
	windows := fs.String("windows", "", "verify-report-windows.json path")
	out := fs.String("out", "", "evidence manifest output path")
	localDefer := fs.Bool("local-defer-missing", false, "defer locally-missing gates (forbidden under CI)")
	printPin := fs.String("print-pin", "", "print one stackpin pin value and exit")
	_ = fs.Parse(os.Args[1:])

	if *printPin != "" {
		if v, ok := pinValue(*printPin); ok {
			fmt.Println(v)
			return
		}
		fmt.Fprintln(os.Stderr, "unknown pin:", *printPin)
		os.Exit(2)
	}
	if *merge {
		code := runMerge(*linux, *windows, *out)
		os.Exit(code)
	}
	if *localDefer && os.Getenv("CI") != "" {
		fmt.Fprintln(os.Stderr, "-local-defer-missing is forbidden under CI")
		os.Exit(2)
	}

	r, err := absRoot(*root)
	fatal(err)
	ctx := buildContext(r, *localDefer)

	if *planUnity != "" {
		plan := gates.PlanUnityModes(ctx)
		b, _ := json.Marshal(plan)
		if *planUnity == "-" {
			fmt.Println(string(b))
		} else if err := os.WriteFile(*planUnity, append(b, '\n'), 0o644); err != nil {
			fatal(err)
		}
		return
	}

	runner := &gates.Runner{Root: r, Ctx: ctx, UnityDir: *unityDir, APICheck: ghAPICheck(r)}
	if added, err := addedFiles(r); err == nil {
		runner.AddedPaths = added
	}

	var rep *gates.Report
	switch *phase {
	case "all":
		rep, err = runner.Run("")
	case "pre-unity", "unity":
		if *phase == "unity" {
			pr, perr := gates.LoadPreReport(*preReport, gitHead(r))
			if perr != nil {
				fmt.Fprintf(os.Stderr, "pre-report unusable (%v); running all gates\n", perr)
				runner.PreReport = nil
				rep, err = runner.Run("")
			} else {
				runner.PreReport = pr
				rep, err = runner.Run(gates.PhaseUnity)
				// Full report = pre rows + unity rows, schema verify-report-v1.
				rep.Schema = gates.SchemaReport
			}
		} else {
			rep, err = runner.Run(gates.PhasePreUnity)
		}
	default:
		fatal(fmt.Errorf("unknown phase %q", *phase))
	}
	fatal(err)

	if *reportOut != "" {
		fatal(gates.WriteReport(*reportOut, rep))
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(rep.Gates)
	fmt.Fprintln(os.Stderr, "conclusion:", rep.Conclusion)
	if rep.Conclusion != "PASSED" {
		os.Exit(1)
	}
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "verify:", err)
		os.Exit(1)
	}
}

func absRoot(root string) (string, error) {
	if root != "" {
		return filepath.Abs(root)
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if fi, err := os.Stat(filepath.Join(dir, ".git")); err == nil && fi.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .git ancestor found; pass -repo-root")
		}
		dir = parent
	}
}

func gitHead(root string) string {
	out, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// addedFiles lists files added by origin/main...HEAD (Q6.evidence scope).
func addedFiles(root string) ([]string, error) {
	base, err := mergeBase(root)
	if err != nil {
		return nil, err
	}
	out, err := exec.Command("git", "-C", root, "diff", "--name-only", "--diff-filter=A", base+"...HEAD").Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, l := range strings.Split(string(out), "\n") {
		if l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

// ghAPICheck returns the GitHub-API ci_run_id/run_attempt verifier via gh;
// failure surfaces as an error inside the gate.
func ghAPICheck(root string) gates.APICheckFunc {
	return func(runID, attempt string) (string, string, error) {
		out, err := exec.Command("gh", "api", fmt.Sprintf("repos/{owner}/{repo}/actions/runs/%s", runID),
			"--jq", "{workflow: .path, conclusion: .conclusion, attempt: .run_attempt}").Output()
		if err != nil {
			return "", "", fmt.Errorf("gh api runs/%s: %w", runID, err)
		}
		var v struct {
			Workflow   string `json:"workflow"`
			Conclusion string `json:"conclusion"`
			Attempt    int    `json:"attempt"`
		}
		if err := json.Unmarshal(out, &v); err != nil {
			return "", "", err
		}
		wf := filepath.Base(v.Workflow)
		if strconv.Itoa(v.Attempt) != attempt && attempt != "" && v.Attempt != 0 {
			return wf, v.Conclusion, fmt.Errorf("attempt mismatch: api=%d manifest=%s", v.Attempt, attempt)
		}
		return wf, v.Conclusion, nil
	}
}

// buildContext resolves the RunContext from the environment + git state.
func buildContext(root string, localDefer bool) gates.RunContext {
	ctx := gates.RunContext{InCI: os.Getenv("CI") != "", LocalDeferMissing: localDefer}
	switch runtime.GOOS {
	case "windows":
		ctx.OSTarget = "windows"
	default:
		ctx.OSTarget = "linux"
	}
	ctx.EventName = os.Getenv("GITHUB_EVENT_NAME")
	ctx.HeadBranch = os.Getenv("GITHUB_HEAD_REF")
	if ctx.HeadBranch == "" {
		ctx.HeadBranch = os.Getenv("GITHUB_REF_NAME")
	}
	if ctx.HeadBranch == "" {
		out, err := exec.Command("git", "-C", root, "branch", "--show-current").Output()
		if err == nil {
			ctx.HeadBranch = strings.TrimSpace(string(out))
		}
	}

	// Task queue: head parse always; main parse best-effort (empty = unknown,
	// which fails closed for owner-done checks).
	headSrc, err := os.ReadFile(filepath.Join(root, "docs", "10_implementation", "task_queue.md"))
	if err == nil {
		ctx.HeadIdx, _ = gates.ParseQueue(string(headSrc))
	}
	mainSrc, err := gates.GitShowMain(root, "docs/10_implementation/task_queue.md", "origin/main")
	if err == nil {
		ctx.MainIdx, _ = gates.ParseQueue(mainSrc)
	}
	if ctx.HeadIdx == nil {
		ctx.HeadIdx = gates.PacketIndex{}
	}
	if ctx.MainIdx == nil {
		ctx.MainIdx = gates.PacketIndex{}
	}

	// Status-only + unity scope need the PR diff. On non-PR events scope=full.
	var changes []gates.FileChange
	var paths []string
	var diffErr error
	if ctx.EventName == "pull_request" || ctx.EventName == "pull_request_target" {
		changes, paths, diffErr = prDiff(root)
		if diffErr == nil {
			role, rerr := gates.RoleForBranch(ctx.HeadBranch)
			if rerr == nil {
				cd := gates.ClassifyControlDiff(role, changes)
				ctx.StatusOnly = cd.StatusOnly
			}
		}
	}
	ctx.UnityScope = gates.ResolveUnityScope(ctx.EventName, ctx.HeadBranch, paths, diffErr)
	return ctx
}

// prDiff returns per-file added/removed lines for origin/main...HEAD.
func prDiff(root string) ([]gates.FileChange, []string, error) {
	base, err := mergeBase(root)
	if err != nil {
		return nil, nil, err
	}
	out, err := exec.Command("git", "-C", root, "diff", "-U0", base+"...HEAD").Output()
	if err != nil {
		return nil, nil, err
	}
	return parseDiff(string(out))
}

func mergeBase(root string) (string, error) {
	out, err := exec.Command("git", "-C", root, "merge-base", "origin/main", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// parseDiff extracts file changes from a -U0 unified diff.
func parseDiff(diff string) ([]gates.FileChange, []string, error) {
	var changes []gates.FileChange
	var paths []string
	var cur *gates.FileChange
	for _, l := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(l, "+++ b/"):
			p := strings.TrimPrefix(l, "+++ b/")
			changes = append(changes, gates.FileChange{Path: p})
			paths = append(paths, p)
			cur = &changes[len(changes)-1]
		case cur != nil && strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++"):
			cur.Added = append(cur.Added, l[1:])
		case cur != nil && strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---"):
			cur.Removed = append(cur.Removed, l[1:])
		}
	}
	return changes, paths, nil
}

// pinValue exposes stackpin entries to the workflow (-print-pin <key>).
func pinValue(key string) (string, bool) {
	plat := "linux"
	if runtime.GOOS == "windows" {
		plat = "windows"
	}
	switch key {
	case "go-version":
		return stackpin.GoVersion, true
	case "protoc-version":
		return stackpin.ProtocVersion, true
	case "staticcheck-version":
		return stackpin.StaticcheckVersion, true
	case "unity-editor-version":
		return stackpin.UnityEditorVersion, true
	case "unity-editor-changeset":
		return stackpin.UnityEditorChangeset, true
	case "unity-installer-url":
		return stackpin.UnityWindowsInstallers["editor"].URL, true
	case "unity-installer-sha256":
		return stackpin.UnityWindowsInstallers["editor"].SHA256, true
	case "unity-il2cpp-url":
		return stackpin.UnityWindowsInstallers["il2cpp"].URL, true
	case "unity-il2cpp-sha256":
		return stackpin.UnityWindowsInstallers["il2cpp"].SHA256, true
	case "edb-url":
		return stackpin.EDBPostgresZip.URL, true
	case "edb-sha256":
		return stackpin.EDBPostgresZip.SHA256, true
	case "cacert-url":
		return stackpin.CacertDownloadURL, true
	case "cacert-sha256":
		return stackpin.CacertSHA256, true
	case "gcloud-url":
		return stackpin.GcloudPin.URL, true
	case "gcloud-sha256":
		return stackpin.GcloudPin.SHA256, true
	}
	if strings.HasPrefix(key, "cli-") {
		return cliPin(strings.TrimPrefix(key, "cli-"), plat)
	}
	return "", false
}

// cliPin handles keys cli-<name>-url / cli-<name>-sha256 / cli-<name>-version.
func cliPin(key, plat string) (string, bool) {
	for _, t := range stackpin.CliTools {
		switch key {
		case t.Name + "-url":
			if plat == "windows" {
				return t.ReleaseURL + t.WindowsAsset, true
			}
			return t.ReleaseURL + t.LinuxAsset, true
		case t.Name + "-sha256":
			if plat == "windows" {
				return t.WindowsSHA256, true
			}
			return t.LinuxSHA256, true
		case t.Name + "-asset":
			if plat == "windows" {
				return t.WindowsAsset, true
			}
			return t.LinuxAsset, true
		case t.Name + "-version":
			return t.Version, true
		}
	}
	return "", false
}

// runMerge executes -merge: read both reports, write the evidence manifest.
func runMerge(linuxPath, windowsPath, outPath string) int {
	m, errs := gates.MergeReports(linuxPath, windowsPath)
	m.TaskID = taskIDFromBranch()
	if outPath == "" {
		outPath = "artifacts/evidence/manifest.json"
	}
	if err := gates.WriteManifest(outPath, m); err != nil {
		fmt.Fprintln(os.Stderr, "verify merge:", err)
		return 1
	}
	for _, e := range errs {
		fmt.Fprintln(os.Stderr, "verify merge:", e)
	}
	if m.Result != "PASSED" {
		return 1
	}
	return 0
}

// taskIDFromBranch extracts IMP-NNN from the head branch, else "none".
func taskIDFromBranch() string {
	b := os.Getenv("GITHUB_HEAD_REF")
	if b == "" {
		b = os.Getenv("GITHUB_REF_NAME")
	}
	re := regexp.MustCompile(`IMP-\d+`)
	if m := re.FindString(b); m != "" {
		return m
	}
	return "none"
}

var _ = sort.Strings       // keep import
var _ = context.Background // keep import
