package trusted

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const repoRoot = "../../../../"

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func verifyYML(t *testing.T) string {
	t.Helper()
	return readRepoFile(t, ".github/workflows/verify.yml")
}

func TestVerifierBuiltFromBase(t *testing.T) {
	if d := VerifierBuiltFromBase(verifyYML(t)); len(d) != 0 {
		t.Fatalf("verify.yml must build the verifier from base and run it on the head: %v", d)
	}
}

func TestOwnerSetupEvidenceSchema(t *testing.T) {
	valid := OwnerSetupEvidence{
		Repo:          OwnerSetupRepo{FullName: "hung98dev/ttplqdc", Visibility: "public", DefaultBranch: "main"},
		SecretNames:   append([]string{"OTHER"}, requiredSecretNames...),
		VariableNames: []string{"AUTO_MERGE_FROZEN", "OTHER"},
		Apps: []OwnerSetupApp{
			{Slug: "thinhthan-policy-reviewer", AppID: 11,
				Permissions: map[string]string{"checks": "write", "metadata": "read"}},
			{Slug: "thinhthan-merge-guard", AppID: 22,
				Permissions: map[string]string{
					"contents": "write", "pull_requests": "write",
					"issues": "write", "variables": "write",
				}},
		},
		Rulesets: 1,
	}
	if d := ValidateOwnerSetup(valid); len(d) != 0 {
		t.Fatalf("valid evidence rejected: %v", d)
	}
	bad := valid
	bad.SecretNames = []string{"UNITY_LICENSE"}
	bad.Apps = valid.Apps[:1]
	bad.Rulesets = 0
	d := ValidateOwnerSetup(bad)
	if len(d) < 3 {
		t.Fatalf("expected missing-secret/app/ruleset details, got %v", d)
	}
}

func TestGuardOpensRevertPr(t *testing.T) {
	jobs := []JobResult{
		{Name: "Q0-Q6 verify (Linux)", Conclusion: "failure"},
		{Name: "Q0-Q6 verify (Windows)", Conclusion: "success"},
	}
	if got := ClassifyGuard(jobs, "imp/IMP-001-x"); got != ActionRevertPR {
		t.Fatalf("non-infra failure on a normal merge must open a revert PR, got %q", got)
	}
}

func TestRevertOrInfraFailureFreezes(t *testing.T) {
	// A failing revert must freeze, not produce another revert.
	failRevert := []JobResult{{Name: "j", Conclusion: "failure"}}
	if got := ClassifyGuard(failRevert, "revert/abc123"); got != ActionFreeze {
		t.Fatalf("failing revert merge must freeze, got %q", got)
	}
	// An infrastructure failure (no job verdict) freezes.
	infra := []JobResult{{Name: "j", Conclusion: "cancelled"}}
	if got := ClassifyGuard(infra, "imp/IMP-001-x"); got != ActionFreeze {
		t.Fatalf("infrastructure failure must freeze, got %q", got)
	}
	// Mixed: a real failure plus infra noise still triggers the revert path.
	mixed := []JobResult{
		{Name: "a", Conclusion: "failure"},
		{Name: "b", Conclusion: "cancelled"},
	}
	if got := ClassifyGuard(mixed, "imp/IMP-001-x"); got != ActionRevertPR {
		t.Fatalf("a real failure among infra noise must open a revert PR, got %q", got)
	}
}

const blockersFixture = `# Known Implementation Blockers

## Open Blockers

### ` + "`BLK-009`" + ` — open example
opened_by: devin-imp-001   opened_at: 2026-10-01T00:00:00Z
evidence: x
owning spec / system: x
options:
  1. a
  2. b
blocks: <IMP-009>

### ` + "`OPS-001`" + ` — ops entry (Gate D, not Gate A)
opened_by: devin-imp-001   opened_at: 2026-10-01T00:00:00Z
evidence: x
owning spec / system: x
blocks: ALL
issue: x

## Resolved Blockers

### ` + "`BLK-002`" + ` — resolved example — RESOLVED
opened_by: devin-imp-071   opened_at: 2026-10-02T08:38:13Z
evidence: x
`

func TestOpenBlkFailsGateA(t *testing.T) {
	got := OpenBlockers(blockersFixture)
	if len(got) != 1 || got[0] != "BLK-009" {
		t.Fatalf("OpenBlockers = %v, want [BLK-009] (OPS and resolved excluded)", got)
	}
	// The real register currently has no open BLK entries — Gate A is green.
	if d := OpenBlockers(readRepoFile(t, "docs/10_implementation/known_blockers.md")); len(d) != 0 {
		t.Fatalf("Gate A must be green on main: %v", d)
	}
}

func TestForkPrFailsBeforeCheckout(t *testing.T) {
	if d := ForkGuardPrecedesCheckout(verifyYML(t)); len(d) != 0 {
		t.Fatalf("every job must fork-guard before checkout and secrets: %v", d)
	}
	bad := "jobs:\n  j:\n    steps:\n      - uses: actions/checkout@v1\n      - name: Fork guard\n        run: true\n"
	if d := ForkGuardPrecedesCheckout(bad); len(d) == 0 {
		t.Fatal("checkout before Fork guard must fail")
	}
}

func TestBothOsJobsRequired(t *testing.T) {
	if d := RequiredJobsPresent(verifyYML(t)); len(d) != 0 {
		t.Fatalf("both required OS jobs must exist: %v", d)
	}
}

func TestSkipOnlyWhileOwnerNotDone(t *testing.T) {
	runner := readRepoFile(t, "server/internal/conformance/gates/runner.go")
	activation := readRepoFile(t, "server/internal/conformance/gates/activation.go")
	if d := SkipOnlyWhileOwnerNotDone(runner, activation); len(d) != 0 {
		t.Fatalf("ratchet gates must be registered IMP-068-owned and skip until DONE: %v", d)
	}
}

func TestTwoStepTriggerCutover(t *testing.T) {
	// The trusted cutover is two commits on two PRs: this impl PR adds
	// pull_request_target alongside pull_request; imp/IMP-068-done removes
	// pull_request. In both states the trusted trigger must be present.
	if !HasTrustedTrigger(verifyYML(t)) {
		t.Fatal("verify.yml must declare pull_request_target")
	}
}

func TestGuardSkipsForkCheckOnPush(t *testing.T) {
	if d := ForkGuardPREventsOnly(verifyYML(t)); len(d) != 0 {
		t.Fatalf("fork guard must fire only on pull_request events: %v", d)
	}
}

func TestFreezeBlocksAllButRevertAndOps(t *testing.T) {
	if d := FreezeBlocksAllButRevertAndOps(verifyYML(t)); len(d) != 0 {
		t.Fatalf("freeze must fail all but revert/ and ops/ branches: %v", d)
	}
	guard := readRepoFile(t, ".github/workflows/post_merge_guard.yml")
	if strings.Contains(guard, "- name: Fork guard") {
		t.Fatal("post-merge guard must not run the PR head fork check")
	}
}

func TestGuardClearsFreezeOnOpsResolution(t *testing.T) {
	if d := GuardClearsFreezeOnOps(readRepoFile(t, ".github/workflows/post_merge_guard.yml")); len(d) != 0 {
		t.Fatalf("guard must clear AUTO_MERGE_FROZEN on the resolving ops/ merge: %v", d)
	}
	// Non-ops green push does not clear.
	jobs := []JobResult{{Name: "j", Conclusion: "success"}}
	if got := ClassifyGuard(jobs, "imp/IMP-001-x"); got != ActionNone {
		t.Fatalf("green non-ops push must not clear freeze, got %q", got)
	}
	if got := ClassifyGuard(jobs, "ops/fix-freeze"); got != ActionClearFreeze {
		t.Fatalf("green ops/ merge push must clear freeze, got %q", got)
	}
}

func TestPolicyReviewIsAppCheckRun(t *testing.T) {
	data := []byte(`{"total_count":2,"check_runs":[` +
		`{"name":"Q0-Q6 verify (Linux)","app":{"id":68,"slug":"github-actions"}},` +
		`{"name":"policy-review","app":{"id":973,"slug":"thinhthan-policy-reviewer"}}]}`)
	slug, err := CheckRunsFromApp(data, "policy-review")
	if err != nil {
		t.Fatalf("policy-review check run must resolve: %v", err)
	}
	if slug != "thinhthan-policy-reviewer" {
		t.Fatalf("policy-review app slug = %q, want thinhthan-policy-reviewer", slug)
	}
	actions := []byte(`{"check_runs":[{"name":"policy-review","app":{"id":68,"slug":"github-actions"}}]}`)
	if _, err := CheckRunsFromApp(actions, "policy-review"); err == nil {
		t.Fatal("policy-review from github-actions (a workflow job) must fail")
	}
}

func TestAgentTokenAndAppPermissionEvidence(t *testing.T) {
	// The evidence schema must carry both Apps with the ADR-0072 permission
	// sets; a reviewer App without checks:write fails validation.
	ev := OwnerSetupEvidence{
		Repo:          OwnerSetupRepo{Visibility: "public", DefaultBranch: "main"},
		SecretNames:   requiredSecretNames,
		VariableNames: requiredVariableNames,
		Apps: []OwnerSetupApp{
			{Slug: "thinhthan-policy-reviewer", AppID: 973,
				Permissions: map[string]string{"checks": "read", "metadata": "read"}},
			{Slug: "thinhthan-merge-guard", AppID: 974,
				Permissions: map[string]string{
					"contents": "write", "pull_requests": "write",
					"issues": "write", "variables": "write",
				}},
		},
		Rulesets: 2,
	}
	d := ValidateOwnerSetup(ev)
	if len(d) == 0 || !strings.Contains(strings.Join(d, " "), "checks") {
		t.Fatalf("reviewer App with checks=read must fail evidence validation: %v", d)
	}
}
