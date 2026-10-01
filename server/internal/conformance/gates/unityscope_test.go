package gates

import (
	"errors"
	"strings"
	"testing"
)

func TestUnityRelevantPaths(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"client/Assets/x.cs", true},
		{"client/ProjectSettings/ProjectVersion.txt", true},
		{"proto/thinhthan/v1/m.proto", true},
		{"scripts/codegen.ps1", true},
		{"scripts/codegen.py", true},
		{"scripts/verify.ps1", true},
		{".github/workflows/verify.yml", true},
		{"server/cmd/verify/main.go", true},
		{"server/internal/conformance/gates/runner.go", true},
		{"docs/00_context/technology_versions.md", true},
		{"docs/10_implementation/task_queue.md", false},
		{"server/internal/sim/x.go", false},
		{"scripts/deploy.ps1", false},
		{".github/pull_request_template.md", false},
		{"docs/01_gameplay/combat.md", false},
	}
	for _, c := range cases {
		if got := IsUnityRelevantPath(c.path); got != c.want {
			t.Errorf("IsUnityRelevantPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestResolveUnityScope(t *testing.T) {
	cases := []struct {
		name    string
		event   string
		branch  string
		paths   []string
		diffErr error
		want    string
	}{
		{"docs-only PR", "pull_request", "imp/IMP-001-x", []string{"docs/01_gameplay/x.md"}, nil, "none"},
		{"client PR", "pull_request", "imp/IMP-001-x", []string{"client/Assets/x.cs"}, nil, "full"},
		{"push event", "push", "", []string{"docs/x.md"}, nil, "full"},
		{"done head", "pull_request", "imp/IMP-000-done", []string{"docs/x.md"}, nil, "full"},
		{"IMP-068 head", "pull_request", "imp/IMP-068-x", []string{"docs/x.md"}, nil, "full"},
		{"undeterminable diff", "pull_request", "imp/IMP-001-x", nil, errors.New("diff"), "full"},
		{"verify.ps1 PR", "pull_request", "imp/IMP-001-x", []string{"scripts/verify.ps1"}, nil, "full"},
	}
	for _, c := range cases {
		got := ResolveUnityScope(c.event, c.branch, c.paths, c.diffErr)
		if got != c.want {
			t.Errorf("%s: scope %s, want %s", c.name, got, c.want)
		}
	}
}

func TestWorkflowUnityScopeMatchesVerifier(t *testing.T) {
	// The workflow derives scope only via `verify -plan-unity`; the
	// verifier's UnityRelevantPatterns is the single source (ADR-0073).
	wf := readWorkflow(t)
	u := jobSection(t, wf, "unity-windows")
	if !strings.Contains(u, "-plan-unity") {
		t.Fatal("unity job must call verify -plan-unity")
	}
	// Every pattern class the spec names must be covered.
	for _, want := range []string{
		"client/", "proto/", "scripts/verify.ps1", ".github/workflows/",
		"server/cmd/verify/", "server/internal/conformance/",
		"docs/00_context/technology_versions.md",
	} {
		var covered bool
		for _, p := range UnityRelevantPatterns {
			if p == want {
				covered = true
			}
		}
		if !covered {
			t.Fatalf("pattern %q missing from UnityRelevantPatterns", want)
		}
	}
	if !IsUnityRelevantPath("scripts/codegen.ps1") {
		t.Fatal("scripts/codegen.* must be unity-relevant")
	}
}
