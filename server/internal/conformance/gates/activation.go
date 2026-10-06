package gates

import "strings"

// Result is the gate row verdict.
type Result string

const (
	ResultPass     Result = "PASS"
	ResultFail     Result = "FAIL"
	ResultSkip     Result = "SKIP"
	ResultDeferred Result = "DEFERRED"
)

// Canonical skip/defer reasons — the only legal values.
const (
	SkipOwnerNotDone   = "owner-not-done"
	SkipStatusOnly     = "status-only"
	SkipNoClientChange = "no-client-change"
	SkipWindowsOnly    = "windows-only"
	DeferLocalMissing  = "local-missing"
)

// GateSpec is one registered gate row.
type GateSpec struct {
	ID    string // e.g. "Q0.absent_paths"
	Owner string // owning IMP task
	OS    string // "", "windows", "linux"
	Phase string // PhasePreUnity or PhaseUnity
}

const (
	PhasePreUnity = "pre-unity"
	PhaseUnity    = "unity"
)

// OwnerDone reports whether a gate's owner task is DONE either on origin/main
// or in the current PR head (activation per ADR-0068: a -done PR runs its own
// gates for the first time).
func OwnerDone(mainIdx, headIdx PacketIndex, owner string) bool {
	isDone := func(idx PacketIndex) bool {
		p, ok := idx[owner]
		return ok && p.Status == "DONE"
	}
	return isDone(mainIdx) || isDone(headIdx)
}

// GateRequired reports whether the gate must evaluate (vs SKIP/DEFERRED) in
// this run context.
func GateRequired(spec GateSpec, ctx RunContext) bool {
	return gateVerdict(spec, ctx) == ""
}

// RunContext carries everything activation/scope decisions need.
type RunContext struct {
	OSTarget          string // "linux" | "windows"
	EventName         string // github.event_name
	HeadBranch        string // PR head ref ("" on push)
	StatusOnly        bool
	UnityScope        string   // "full" | "none" — from ResolveUnityScope
	ChangedPaths      []string // PR diff paths (empty on non-PR events)
	LocalDeferMissing bool
	InCI              bool
	MainIdx, HeadIdx  PacketIndex
}

// gateVerdict returns "" when the gate must run, else the skip reason code.
func gateVerdict(spec GateSpec, ctx RunContext) string {
	if spec.OS != "" && spec.OS != ctx.OSTarget {
		// Windows-scoped gates report SKIP(windows-only) on Linux; Linux-scoped
		// gates are simply absent from Windows reports (no canonical reason).
		if spec.OS == "windows" {
			return SkipWindowsOnly
		}
		return "absent" // not emitted
	}
	if !OwnerDone(ctx.MainIdx, ctx.HeadIdx, spec.Owner) {
		return SkipOwnerNotDone
	}
	if ctx.StatusOnly && !strings.HasPrefix(spec.ID, "Q0.") {
		return SkipStatusOnly
	}
	if spec.Phase == PhaseUnity && ctx.UnityScope == "none" {
		return SkipNoClientChange
	}
	return ""
}
