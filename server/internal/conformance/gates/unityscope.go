package gates

import "strings"

// UnityRelevantPatterns is the single source for scope=full detection in both
// the Unity job and the verifier (ADR-0073 / test-plan gate scope table).
// A changed path matching any pattern forces scope=full.
var UnityRelevantPatterns = []string{
	"client/",
	"proto/",
	"scripts/verify.ps1",
	".github/workflows/",
	"server/cmd/verify/",
	"server/internal/conformance/",
	"docs/00_context/technology_versions.md",
}

// IsUnityRelevantPath reports whether one changed path forces scope=full.
func IsUnityRelevantPath(p string) bool {
	for _, pat := range UnityRelevantPatterns {
		switch {
		case strings.HasSuffix(pat, "/"):
			if strings.HasPrefix(p, pat) {
				return true
			}
		case strings.HasSuffix(pat, "*"):
			if strings.HasPrefix(p, strings.TrimSuffix(pat, "*")) {
				return true
			}
		default:
			if p == pat {
				return true
			}
		}
	}
	// scripts/codegen.* — any extension
	if strings.HasPrefix(p, "scripts/codegen.") {
		return true
	}
	return false
}

// ResolveUnityScope returns "full" or "none" (ADR-0073). scope=none is only
// legal on a pull_request event with a determinable diff; every other case —
// push, imp/IMP-068-* and *-done status heads, or an undeterminable diff —
// is scope=full.
func ResolveUnityScope(eventName, headBranch string, changedPaths []string, diffErr error) string {
	if eventName != "pull_request" && eventName != "pull_request_target" {
		return "full"
	}
	if diffErr != nil {
		return "full"
	}
	if strings.HasPrefix(headBranch, "imp/IMP-068-") || strings.HasSuffix(headBranch, "-done") {
		return "full"
	}
	for _, p := range changedPaths {
		if IsUnityRelevantPath(p) {
			return "full"
		}
	}
	return "none"
}
