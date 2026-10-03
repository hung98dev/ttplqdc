// Trusted-CI structural validators over workflow YAML text. They assert the
// audit_gates.md § Job Preconditions / Gates A-D contracts directly on the
// shipped .github/workflows files: fork guard before checkout and secrets,
// freeze allowlist, trigger cutover, verifier-built-from-base, and the
// post-merge guard's ops-resolution path.
package trusted

import "strings"

// HasTrustedTrigger reports whether the workflow's on: block declares
// pull_request_target — the only trusted trigger after the cutover.
func HasTrustedTrigger(content string) bool {
	on := sectionText(content, "on:", "concurrency:", "permissions:", "env:", "jobs:")
	return strings.Contains(on, "pull_request_target:")
}

// RequiredJobsPresent reports that both required OS jobs are declared by name.
func RequiredJobsPresent(content string) []string {
	var out []string
	for _, n := range []string{"Q0-Q6 verify (Linux)", "Q0-Q6 verify (Windows)"} {
		if !strings.Contains(content, "name: "+n) {
			out = append(out, "required job missing: "+n)
		}
	}
	return out
}

// VerifierBuiltFromBase asserts the Gate D shape: the workspace checkout is
// pinned to the PR head SHA (the tested source), the verifier source is a
// fresh main clone outside the tested tree, and gate invocations run the
// prebuilt binary rather than `go run` against tested source.
func VerifierBuiltFromBase(content string) []string {
	var out []string
	for _, want := range []string{
		"pull_request.head.sha",
		"verifier-src",
		"clone",
		"verify-bin/verify",
	} {
		if !strings.Contains(content, want) {
			out = append(out, "verifier structure missing: "+want)
		}
	}
	// `go run ./cmd/verify` inside the tested tree builds the PR's own judge —
	// that must never execute for gate or report paths again.
	for _, bad := range []string{
		"run ./cmd/verify",
		"-File scripts/verify.ps1",
	} {
		if strings.Contains(content, bad) {
			out = append(out, "untrusted verifier path remains: "+bad)
		}
	}
	return out
}

// ForkGuardPrecedesCheckout asserts every job runs its Fork guard step before
// the first actions/checkout and before the first secrets.* reference, so a
// fork PR fails before any code or credential is exposed.
func ForkGuardPrecedesCheckout(content string) []string {
	var out []string
	for name, block := range jobBlocks(content) {
		guard := strings.Index(block, "- name: Fork guard")
		checkout := strings.Index(block, "uses: actions/checkout")
		secret := strings.Index(block, "secrets.")
		if guard < 0 {
			out = append(out, name+": no Fork guard step")
			continue
		}
		if checkout >= 0 && guard > checkout {
			out = append(out, name+": checkout precedes Fork guard")
		}
		if secret >= 0 && guard > secret {
			out = append(out, name+": secrets referenced before Fork guard")
		}
	}
	return out
}

// ForkGuardPREventsOnly asserts the fork guard only fires on pull-request
// events — push/workflow_call runs must pass the step.
func ForkGuardPREventsOnly(content string) []string {
	var out []string
	for name, block := range jobBlocks(content) {
		i := strings.Index(block, "- name: Fork guard")
		if i < 0 {
			continue
		}
		seg := block[i:]
		if j := strings.Index(seg[1:], "- name:"); j >= 0 {
			seg = seg[:1+j]
		}
		if !strings.Contains(seg, "pull_request|pull_request_target") {
			out = append(out, name+": fork guard does not match pull_request events")
		}
		if strings.Contains(seg, "push") || strings.Contains(seg, "workflow_call") {
			out = append(out, name+": fork guard must not fire on push/workflow_call")
		}
	}
	return out
}

// FreezeBlocksAllButRevertAndOps asserts a Freeze-check step exists that
// fails every branch except revert/* and ops/* while AUTO_MERGE_FROZEN.
func FreezeBlocksAllButRevertAndOps(content string) []string {
	var out []string
	if !strings.Contains(content, "AUTO_MERGE_FROZEN") {
		out = append(out, "no AUTO_MERGE_FROZEN freeze check")
	}
	if !strings.Contains(content, "revert/*|ops/*") {
		out = append(out, "freeze allowlist revert/*|ops/* missing")
	}
	return out
}

// GuardClearsFreezeOnOps asserts the post-merge guard resolves an ops/ merge
// by writing AUTO_MERGE_FROZEN=false.
func GuardClearsFreezeOnOps(content string) []string {
	var out []string
	for _, want := range []string{"ops/", "AUTO_MERGE_FROZEN", "value=false"} {
		if !strings.Contains(content, want) {
			out = append(out, "guard clear-freeze path missing: "+want)
		}
	}
	return out
}

// SkipOnlyWhileOwnerNotDone asserts the gate-activation contract: IMP-068's
// ratchet gates are registered with Owner IMP-068 and the runner emits
// SKIP(owner-not-done) until the owner is DONE.
func SkipOnlyWhileOwnerNotDone(runnerSrc, activationSrc string) []string {
	var out []string
	for _, want := range []string{
		`ID: "Q0.ratchet", Owner: "IMP-068"`,
		`ID: "Q6.ratchet", Owner: "IMP-068"`,
	} {
		if !strings.Contains(runnerSrc, want) {
			out = append(out, "runner registry missing: "+want)
		}
	}
	if !strings.Contains(activationSrc, "owner-not-done") {
		out = append(out, "gate activation missing owner-not-done skip")
	}
	return out
}

// jobBlocks splits the jobs: section into name -> block text.
func jobBlocks(content string) map[string]string {
	out := map[string]string{}
	lines := strings.Split(content, "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "jobs:") {
			start = i
			break
		}
	}
	if start < 0 {
		return out
	}
	cur, curName := -1, ""
	for i := start + 1; i < len(lines); i++ {
		l := lines[i]
		if len(l) > 0 && l[0] != ' ' && l[0] != '\t' {
			break
		}
		if strings.HasPrefix(l, "  ") && !strings.HasPrefix(l, "   ") {
			if curName != "" {
				out[curName] = strings.Join(lines[cur:i], "\n")
			}
			curName = strings.TrimSuffix(strings.TrimSpace(l), ":")
			cur = i
		}
	}
	if curName != "" {
		out[curName] = strings.Join(lines[cur:], "\n")
	}
	return out
}

// sectionText extracts the text between a marker line and the first of the
// given terminator prefixes (at column 0).
func sectionText(content, marker string, terminators ...string) string {
	i := strings.Index(content, marker)
	if i < 0 {
		return ""
	}
	rest := content[i:]
	end := len(rest)
	for _, t := range terminators {
		if j := strings.Index(rest, "\n"+t); j > 0 && j < end {
			end = j
		}
	}
	return rest[:end]
}
