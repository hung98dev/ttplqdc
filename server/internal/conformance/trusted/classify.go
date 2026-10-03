package trusted

import "strings"

// GuardAction is the post-merge guard's verdict for one push to main
// (audit_gates.md § Gate D / post-merge guard):
//   - RevertPR: a non-infrastructure failure — the merge-guard App opens
//     revert/<sha> and dependents are set BLOCKED.
//   - Freeze: a failing revert or an infrastructure failure — AUTO_MERGE_FROZEN
//     is set and an ops-blocked issue is opened.
//   - ClearFreeze: a green push that merged the resolving ops/ PR clears
//     AUTO_MERGE_FROZEN.
type GuardAction string

const (
	ActionNone        GuardAction = "none"
	ActionRevertPR    GuardAction = "revert-pr"
	ActionFreeze      GuardAction = "freeze"
	ActionClearFreeze GuardAction = "clear-freeze"
)

// JobResult is one job's conclusion inside a guard run.
type JobResult struct {
	Name       string
	Conclusion string
}

// InfraConclusion reports a runner/platform failure, not a verdict on the
// code — these freeze auto-merge instead of opening a revert.
func InfraConclusion(c string) bool {
	switch c {
	case "cancelled", "timed_out", "action_required", "startup_failure", "stale":
		return true
	}
	return false
}

// Failed reports a job that ran to a failing verdict.
func (j JobResult) Failed() bool { return j.Conclusion == "failure" }

// ClassifyGuard maps a guard run's job conclusions plus the ref of the PR the
// pushed commit merged ("" for a direct push) onto a GuardAction.
func ClassifyGuard(jobs []JobResult, mergedRef string) GuardAction {
	failed, infra := false, false
	for _, j := range jobs {
		if j.Failed() {
			failed = true
		}
		if InfraConclusion(j.Conclusion) {
			infra = true
		}
	}
	if failed || infra {
		// A revert that itself fails must not produce another revert.
		if strings.HasPrefix(mergedRef, "revert/") {
			return ActionFreeze
		}
		// A real verification failure is the revert trigger even when other
		// jobs also hit infrastructure failures.
		if failed {
			return ActionRevertPR
		}
		return ActionFreeze
	}
	if strings.HasPrefix(mergedRef, "ops/") {
		return ActionClearFreeze
	}
	return ActionNone
}
