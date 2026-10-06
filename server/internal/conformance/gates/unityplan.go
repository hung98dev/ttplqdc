package gates

import "strings"

// UnityPlan is the JSON emitted by `verify -plan-unity` and consumed by the
// Unity job to decide which -runTests invocations to execute.
type UnityPlan struct {
	Scope         string `json:"scope"` // "full" | "none"
	EditMode      bool   `json:"editmode"`
	PlayMode      bool   `json:"playmode"`
	VisualReview  bool   `json:"visual_review"`
	Performance   bool   `json:"performance"`
	GraphicalLoad bool   `json:"graphical_load"`
}

// owner task per §3.3a graphical pass mode.
const (
	ownerEditMode      = "IMP-000"
	ownerPlayMode      = "IMP-065"
	ownerVisualReview  = "IMP-070"
	ownerPerformance   = "IMP-095"
	ownerGraphicalLoad = "IMP-067"
)

// PlanUnityModes resolves the run mode activation table: a mode is active iff
// scope=full, the PR is not status-only, and the mode's owner task is DONE on
// main or head. PlayMode additionally activates on any client-runtime or
// PlayMode-harness diff — owner-DONE activation alone can never verify the
// suite's own owner's fixes (a fix/IMP-065 runtime change shipped while
// playmode=false).
func PlanUnityModes(ctx RunContext) UnityPlan {
	on := func(owner string) bool {
		if ctx.UnityScope != "full" || ctx.StatusOnly {
			return false
		}
		return OwnerDone(ctx.MainIdx, ctx.HeadIdx, owner)
	}
	return UnityPlan{
		Scope:         ctx.UnityScope,
		EditMode:      on(ownerEditMode),
		PlayMode:      on(ownerPlayMode) || touchesClientRuntime(ctx.ChangedPaths),
		VisualReview:  on(ownerVisualReview),
		Performance:   on(ownerPerformance),
		GraphicalLoad: on(ownerGraphicalLoad),
	}
}

// clientRuntimePrefixes are diff surfaces the PlayMode suite verifies. Both
// are always scope=full and never status-only, so they need no further gating.
var clientRuntimePrefixes = []string{
	"client/Assets/Scripts/",
	"client/Assets/Tests/PlayMode/", // suite harness fixes must self-verify
}

// touchesClientRuntime reports whether the diff changes Unity client runtime
// code or the PlayMode harness itself.
func touchesClientRuntime(paths []string) bool {
	for _, p := range paths {
		for _, pre := range clientRuntimePrefixes {
			if strings.HasPrefix(p, pre) {
				return true
			}
		}
	}
	return false
}
