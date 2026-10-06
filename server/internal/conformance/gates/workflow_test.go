package gates

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readWorkflow(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "verify.yml"))
	if err != nil {
		t.Fatalf("verify.yml: %v", err)
	}
	return string(b)
}

// jobSection returns the yaml text of one job (from "  <key>:" to the next
// indent-2 key or EOF).
func jobSection(t *testing.T, wf, key string) string {
	t.Helper()
	start := strings.Index(wf, "  "+key+":\n")
	if start < 0 {
		t.Fatalf("job %s not in verify.yml", key)
	}
	rest := wf[start:]
	// next sibling job key: a line at indent-2 ending in ':'
	re := regexp.MustCompile(`(?m)^  [A-Za-z_-]+:\s*$`)
	locs := re.FindAllStringIndex(rest[4:], -1)
	for _, l := range locs {
		if rest[4:][l[0]:l[1]] != key+":" {
			return rest[:4+l[0]]
		}
	}
	return rest
}

// stepNames lists the `- name:` entries of a job section in order.
func stepNames(section string) []string {
	re := regexp.MustCompile(`(?m)^\s+- name:\s*(.+?)\s*$`)
	var names []string
	for _, m := range re.FindAllStringSubmatch(section, -1) {
		names = append(names, m[1])
	}
	return names
}

// stepBody returns the text of one named step (through the next `- ` step or
// dedent to `  <key>:`).
func stepBody(section, name string) string {
	idx := strings.Index(section, "- name: "+name)
	if idx < 0 {
		return ""
	}
	rest := section[idx:]
	re := regexp.MustCompile(`(?m)^\s+- `)
	locs := re.FindAllStringIndex(rest[2:], -1)
	if len(locs) > 0 {
		return rest[:2+locs[0][0]]
	}
	return rest
}

func TestLinuxAndWindowsJobsRequired(t *testing.T) {
	wf := readWorkflow(t)
	for _, j := range []string{"unity-windows", "verify-windows", "verify-linux"} {
		jobSection(t, wf, j)
	}
	w := jobSection(t, wf, "verify-windows")
	if !strings.Contains(w, "runs-on: windows-2022") {
		t.Fatal("verify-windows must run on windows-2022")
	}
	l := jobSection(t, wf, "verify-linux")
	if !strings.Contains(l, "runs-on: ubuntu-24.04") {
		t.Fatal("verify-linux must run on ubuntu-24.04")
	}
}

func TestForkGuardIsFirstStep(t *testing.T) {
	wf := readWorkflow(t)
	for _, j := range []string{"unity-windows", "verify-windows", "verify-linux"} {
		names := stepNames(jobSection(t, wf, j))
		if len(names) < 3 || names[0] != "Fork guard" || names[1] != "Freeze check" {
			t.Fatalf("job %s first steps must be Fork guard/Freeze check: %v", j, names)
		}
	}
}

func TestNoJobLevelIfOnRequiredJobs(t *testing.T) {
	wf := readWorkflow(t)
	for _, j := range []string{"verify-windows", "verify-linux"} {
		for _, line := range strings.Split(jobSection(t, wf, j), "\n") {
			if strings.HasPrefix(line, "    if:") {
				t.Fatalf("job %s has a job-level if: %s", j, line)
			}
		}
	}
}

func TestSecretsOnlyAfterForkGuard(t *testing.T) {
	wf := readWorkflow(t)
	for _, j := range []string{"unity-windows", "verify-windows", "verify-linux"} {
		sec := jobSection(t, wf, j)
		guardIdx := strings.Index(sec, "- name: Fork guard")
		for i, line := range strings.Split(sec[:guardIdx], "\n") {
			if strings.Contains(line, "secrets.") {
				t.Fatalf("job %s line %d references secrets before Fork guard", j, i)
			}
		}
	}
}

func TestCliToolsFromPinnedReleaseAssets(t *testing.T) {
	wf := readWorkflow(t)
	for _, j := range []string{"verify-windows", "verify-linux"} {
		body := stepBody(jobSection(t, wf, j), "Install pinned CLI tools")
		for _, want := range []string{"curl -fsSL", "sha256sum -c", "7.6.6", "2.101.0", "1.8.2", "3.8.0", "GITHUB_PATH"} {
			if !strings.Contains(body, want) {
				t.Fatalf("job %s CLI install missing %q", j, want)
			}
		}
	}
}

func TestWindowsRenderPassUsesWarp(t *testing.T) {
	wf := readWorkflow(t)
	u := jobSection(t, wf, "unity-windows")
	for _, step := range []string{"Run Performance tests (D3D11 WARP)", "Run Visual Review (D3D11 WARP)"} {
		body := stepBody(u, step)
		if !strings.Contains(body, "-force-d3d11") {
			t.Fatalf("%s must force D3D11 (WARP)", step)
		}
		if strings.Contains(body, "-nographics") {
			t.Fatalf("%s must render (no -nographics)", step)
		}
	}
}

func TestVisualReviewArtifactUpload(t *testing.T) {
	wf := readWorkflow(t)
	u := jobSection(t, wf, "unity-windows")
	body := stepBody(u, "Upload visual-review")
	if !strings.Contains(body, "if: always()") || !strings.Contains(body, "artifacts/visual-review/") {
		t.Fatal("visual-review artifact must upload always()")
	}
}

func TestNoUnityOnLinuxJobs(t *testing.T) {
	wf := readWorkflow(t)
	l := jobSection(t, wf, "verify-linux")
	for _, bad := range []string{"Unity.exe", "-runTests", "unity-editor", "UNITY_", "git lfs pull"} {
		if strings.Contains(l, bad) {
			t.Fatalf("linux job must not contain %q", bad)
		}
	}
}

func TestPullRequestTriggerBeforeCutover(t *testing.T) {
	wf := readWorkflow(t)
	head := wf[:strings.Index(wf, "jobs:")]
	// Shape-tolerant across the IMP-068 cutover: pre-cutover verify.yml
	// triggers on pull_request, post-cutover on pull_request_target only.
	if !strings.Contains(head, "\n  pull_request:") && !strings.Contains(head, "\n  pull_request_target:") {
		t.Fatal("verify.yml must trigger on pull_request or pull_request_target")
	}
	if strings.Contains(head, "push:") {
		t.Fatal("push trigger is IMP-068's, not IMP-000's")
	}
}

func TestForkGuardOnlyOnPullRequestEvents(t *testing.T) {
	wf := readWorkflow(t)
	body := stepBody(jobSection(t, wf, "verify-windows"), "Fork guard")
	if !strings.Contains(body, "pull_request|pull_request_target") {
		t.Fatal("fork guard must only apply to pull_request* events")
	}
}

func TestForkGuardSkippedOnPush(t *testing.T) {
	wf := readWorkflow(t)
	body := stepBody(jobSection(t, wf, "verify-windows"), "Fork guard")
	// On push the case arm doesn't match, so the guard exits 0 — the external
	// check is inside the pull_request* arm only.
	caseStart := strings.Index(body, "pull_request|pull_request_target)")
	exitIdx := strings.Index(body, "exit 1")
	if caseStart < 0 || exitIdx < 0 || exitIdx < caseStart {
		t.Fatal("external-PR rejection must be inside the pull_request* case arm")
	}
}

func TestFreezeFailsExceptRevertAndOps(t *testing.T) {
	wf := readWorkflow(t)
	body := stepBody(jobSection(t, wf, "verify-windows"), "Freeze check")
	for _, want := range []string{"AUTO_MERGE_FROZEN", "revert/*", "ops/*", "exit 1"} {
		if !strings.Contains(body, want) {
			t.Fatalf("freeze check missing %q", want)
		}
	}
}

func TestUnityMaterializeRunsWhenUnityGatesSkip(t *testing.T) {
	wf := readWorkflow(t)
	body := stepBody(jobSection(t, wf, "unity-windows"), "Materialize client")
	cond := ""
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "if:") {
			cond = strings.TrimSpace(l)
		}
	}
	if !strings.Contains(cond, "scope") {
		t.Fatal("materialize must be gated on scope only")
	}
	for _, bad := range []string{"editmode", "playmode", "visual_review", "performance"} {
		if strings.Contains(cond, bad) {
			t.Fatalf("materialize must run even when Unity gates skip: %s", cond)
		}
	}
}

func TestMaterializedArtifactWindowsFailsJob(t *testing.T) {
	wf := readWorkflow(t)
	u := jobSection(t, wf, "unity-windows")
	drift := stepBody(u, "Drift check (unity-materialized)")
	if !strings.Contains(drift, "commit unity-materialized") || !strings.Contains(drift, "exit 1") {
		t.Fatal("drift must fail the job with 'commit unity-materialized'")
	}
	up := stepBody(u, "Upload unity-materialized-windows")
	if !strings.Contains(up, "name: unity-materialized-windows") {
		t.Fatal("must upload unity-materialized-windows on drift")
	}
}

func TestLicenceActivationRetriedFiveTimes(t *testing.T) {
	wf := readWorkflow(t)
	body := stepBody(jobSection(t, wf, "unity-windows"), "Activate Unity licence")
	if !strings.Contains(body, "1; $i -le 5") && !strings.Contains(body, "5 attempts") {
		t.Fatal("licence activation must retry 5 times")
	}
	if !strings.Contains(body, "Start-Sleep -Seconds 60") {
		t.Fatal("retries must wait 60s")
	}
}

func TestCheckoutLfsAndPinnedGitLfs(t *testing.T) {
	wf := readWorkflow(t)
	if !strings.Contains(wf, "GIT_LFS_SKIP_SMUDGE: '1'") {
		t.Fatal("env must set GIT_LFS_SKIP_SMUDGE=1")
	}
	count := strings.Count(wf, "lfs: false")
	if count < 3 {
		t.Fatalf("every checkout must use lfs: false (got %d)", count)
	}
	u := jobSection(t, wf, "unity-windows")
	if !strings.Contains(u, "git lfs pull") {
		t.Fatal("unity job must git lfs pull")
	}
	if !strings.Contains(wf, "git-lfs-") {
		t.Fatal("pinned git-lfs release asset required")
	}
}

func TestRaceOnLinuxJobOnly(t *testing.T) {
	var raceOS string
	for _, s := range Registry {
		if s.ID == "Q3.go.race" {
			raceOS = s.OS
		}
	}
	if raceOS != "linux" {
		t.Fatal("Q3.go.race must be linux-scoped")
	}
	wf := readWorkflow(t)
	if strings.Contains(jobSection(t, wf, "verify-windows"), "-race") ||
		strings.Contains(jobSection(t, wf, "unity-windows"), "-race") {
		t.Fatal("-race must not appear in windows jobs")
	}
	linux := jobSection(t, wf, "verify-linux")
	if !strings.Contains(linux, "verify.ps1") && !strings.Contains(linux, "VERIFY_BIN") {
		t.Fatal("linux job must run the verifier (which owns -race)")
	}
}

func TestEvidenceJobUsesPinnedDownloadArtifact(t *testing.T) {
	wf := readWorkflow(t)
	w := jobSection(t, wf, "verify-windows")
	if !strings.Contains(w, "actions/download-artifact@d3f86a106a0bac45b974a628896c90dbdf5c8093") {
		t.Fatal("download-artifact must use the pinned SHA")
	}
}

func TestWindowsRequiredJobJoinsUnityJob(t *testing.T) {
	wf := readWorkflow(t)
	w := jobSection(t, wf, "verify-windows")
	if !strings.Contains(w, "wait_job.sh") || !strings.Contains(w, "WAIT_JOB: Unity (Windows)") {
		t.Fatal("verify-windows must wait_job.sh on Unity (Windows)")
	}
}

func TestUnityTestModesFromVerifierPlan(t *testing.T) {
	wf := readWorkflow(t)
	u := jobSection(t, wf, "unity-windows")
	if !strings.Contains(u, "-plan-unity") {
		t.Fatal("unity job must derive modes from verify -plan-unity")
	}
	for _, k := range []string{"editmode", "playmode", "visual_review", "performance", "graphical_load"} {
		if !strings.Contains(u, k) {
			t.Fatalf("plan output %q not consumed", k)
		}
	}
}

func TestEvidenceBuiltInWindowsRequiredJob(t *testing.T) {
	wf := readWorkflow(t)
	w := jobSection(t, wf, "verify-windows")
	if !strings.Contains(w, "-MergeReports") && !strings.Contains(w, "-merge") {
		t.Fatal("verify-windows must merge evidence")
	}
	if !strings.Contains(w, "name: evidence") {
		t.Fatal("verify-windows must upload the evidence artifact")
	}
	for _, j := range []string{"evidence", "merge"} {
		if strings.Contains(wf, "  "+j+":") {
			t.Fatalf("no separate %s job allowed", j)
		}
	}
}

func TestLinuxJobIsUnityFree(t *testing.T) {
	wf := readWorkflow(t)
	l := jobSection(t, wf, "verify-linux")
	for _, bad := range []string{"unity", "Unity", "lfs pull", "wait_job", "MergeReports"} {
		if strings.Contains(l, bad) {
			t.Fatalf("linux job must be Unity-free, found %q", bad)
		}
	}
}

func TestWindowsXmlFailureCannotBecomePassOnRetry(t *testing.T) {
	wf := readWorkflow(t)
	w := jobSection(t, wf, "verify-windows")
	body := stepBody(w, "Unity phase verify")
	if strings.Contains(body, "continue-on-error") || strings.Contains(body, "-le 5") || strings.Contains(body, "retry") {
		t.Fatal("unity phase must not retry a failed XML verdict")
	}
}

func TestMissingOrMalformedWindowsXmlFailsClosed(t *testing.T) {
	// Missing results dir -> missing -> FAIL on CI.
	r := &Runner{Root: t.TempDir(), Ctx: RunContext{OSTarget: "windows", InCI: true}, UnityDir: t.TempDir()}
	row := r.evaluate(GateSpec{ID: "Q3.unity.editmode", OS: "windows", Owner: "IMP-000"})
	if row.Result != ResultFail {
		t.Fatalf("missing XML must fail closed, got %s", row.Result)
	}
	// Malformed XML -> FAIL.
	dir := t.TempDir()
	writeFile(t, dir, "editmode-results.xml", "<test-run total=")
	r2 := &Runner{Root: t.TempDir(), Ctx: RunContext{OSTarget: "windows", InCI: true}, UnityDir: dir}
	row = r2.evaluate(GateSpec{ID: "Q3.unity.editmode", OS: "windows", Owner: "IMP-000"})
	if row.Result != ResultFail {
		t.Fatalf("malformed XML must fail, got %s", row.Result)
	}
}

func TestPlayModeEvaluator(t *testing.T) {
	spec := GateSpec{ID: "Q3.unity.playmode", OS: "windows", Owner: "IMP-065"}
	newRunner := func(dir string) *Runner {
		return &Runner{Root: t.TempDir(), Ctx: RunContext{OSTarget: "windows", InCI: true}, UnityDir: dir}
	}
	// No evaluator regression: the gate must evaluate, not "gate has no
	// evaluator". Missing results -> missing -> FAIL on CI.
	row := newRunner(t.TempDir()).evaluate(spec)
	if row.Result != ResultFail || strings.Contains(row.Reason, "no evaluator") {
		t.Fatalf("missing playmode XML must fail closed, got %s (%s)", row.Result, row.Reason)
	}
	passXML := `<test-run total="4" passed="4" failed="0"></test-run>`
	// Passing XML alone is not enough — no completion line -> FAIL (stale file
	// from a killed editor).
	dir := t.TempDir()
	writeFile(t, dir, "playmode-results.xml", passXML)
	writeFile(t, dir, "playmode-editor.log", "batchmode exited early")
	row = newRunner(dir).evaluate(spec)
	if row.Result != ResultFail {
		t.Fatalf("no completion line must fail, got %s", row.Result)
	}
	// failed>0 -> FAIL even with the completion line.
	writeFile(t, dir, "playmode-results.xml", `<test-run total="4" passed="3" failed="1"></test-run>`)
	writeFile(t, dir, "playmode-editor.log", "Test run completed. Exiting with code 0")
	row = newRunner(dir).evaluate(spec)
	if row.Result != ResultFail {
		t.Fatalf("failed>0 must fail, got %s", row.Result)
	}
	// Passed XML + completion line -> PASS.
	writeFile(t, dir, "playmode-results.xml", passXML)
	row = newRunner(dir).evaluate(spec)
	if row.Result != ResultPass {
		t.Fatalf("passed suite must pass, got %s (%s)", row.Result, row.Reason)
	}
	// Malformed XML -> FAIL.
	dir2 := t.TempDir()
	writeFile(t, dir2, "playmode-results.xml", "<test-run total=")
	writeFile(t, dir2, "playmode-editor.log", "Test run completed. Exiting with code 0")
	row = newRunner(dir2).evaluate(spec)
	if row.Result != ResultFail {
		t.Fatalf("malformed XML must fail, got %s", row.Result)
	}
}

func TestRequiredJobsRunPreUnityPhase(t *testing.T) {
	wf := readWorkflow(t)
	wv := jobSection(t, wf, "verify-windows")
	if !strings.Contains(wv, "-Phase pre-unity") && !strings.Contains(wv, "-phase pre-unity") {
		t.Fatal("verify-windows must run the pre-unity phase")
	}
	if !strings.Contains(stepBody(jobSection(t, wf, "verify-windows"), "Pre-Unity verify"), "continue-on-error: true") {
		t.Fatal("pre-unity step must continue-on-error so the unity phase can still join")
	}
}

func TestPrRunCancellationGroup(t *testing.T) {
	wf := readWorkflow(t)
	// Event-scoped during the trigger-cutover window: pull_request (head
	// workflow) and pull_request_target (base workflow) runs must not cancel
	// each other. Accept either group form until pull_request is removed.
	if !strings.Contains(wf, "group: verify-${{ github.event_name }}-${{ github.event.pull_request.number || github.ref }}") &&
		!strings.Contains(wf, "group: verify-${{ github.event.pull_request.number || github.ref }}") {
		t.Fatal("concurrency group must key on PR number")
	}
	if !strings.Contains(wf, "cancel-in-progress: ${{ github.event_name == 'pull_request' || github.event_name == 'pull_request_target' }}") {
		t.Fatal("PR events must cancel in-progress runs; main pushes never")
	}
}
