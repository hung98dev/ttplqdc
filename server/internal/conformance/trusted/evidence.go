package trusted

import (
	"encoding/json"
	"fmt"
)

// OwnerSetupEvidence is the schema of the recorded Owner Setup facts collected
// via `gh api` under docs/10_implementation/evidence/IMP-068/ (audit_gates.md
// § Owner Setup + ADR-0072). The Apps' permission sets are recorded per
// installation; secrets/variables are recorded by name only.
type OwnerSetupEvidence struct {
	Repo          OwnerSetupRepo  `json:"repo"`
	SecretNames   []string        `json:"secret_names"`
	VariableNames []string        `json:"variable_names"`
	Apps          []OwnerSetupApp `json:"apps"`
	Rulesets      int             `json:"rulesets"`
}

type OwnerSetupRepo struct {
	FullName      string `json:"full_name"`
	Visibility    string `json:"visibility"`
	DefaultBranch string `json:"default_branch"`
}

// OwnerSetupApp is one installed GitHub App and the permissions recorded for
// it (from the installation payload or the App's registered set).
type OwnerSetupApp struct {
	Slug        string            `json:"slug"`
	AppID       int64             `json:"app_id"`
	Permissions map[string]string `json:"permissions"`
}

// Required secret names for the trusted pipeline (ADR-0072 Owner Setup).
var requiredSecretNames = []string{
	"GUARD_APP_ID",
	"GUARD_APP_KEY",
	"UNITY_LICENSE",
	"UNITY_EMAIL",
	"UNITY_PASSWORD",
	"UNITY_SERIAL",
}

// Required repository variables.
var requiredVariableNames = []string{
	"AUTO_MERGE_FROZEN",
}

// Required GitHub Apps and their minimum permission sets.
var requiredApps = map[string]map[string]string{
	"thinhthan-policy-reviewer": {"checks": "write", "metadata": "read"},
	"thinhthan-merge-guard": {
		"contents": "write", "pull_requests": "write",
		"issues": "write", "variables": "write",
	},
}

func hasAll(hay []string, needles []string) []string {
	set := map[string]bool{}
	for _, h := range hay {
		set[h] = true
	}
	var missing []string
	for _, n := range needles {
		if !set[n] {
			missing = append(missing, n)
		}
	}
	return missing
}

// ValidateOwnerSetup returns the detail lines for every Owner Setup fact that
// does not satisfy the audit_gates contract (empty = satisfied).
func ValidateOwnerSetup(e OwnerSetupEvidence) []string {
	var out []string
	if e.Repo.Visibility != "public" {
		out = append(out, fmt.Sprintf("repo visibility %q != public", e.Repo.Visibility))
	}
	if e.Repo.DefaultBranch == "" {
		out = append(out, "repo default_branch empty")
	}
	if e.Rulesets < 1 {
		out = append(out, "no ruleset protecting main is visible")
	}
	for _, n := range hasAll(e.SecretNames, requiredSecretNames) {
		out = append(out, fmt.Sprintf("required Actions secret %q missing", n))
	}
	for _, n := range hasAll(e.VariableNames, requiredVariableNames) {
		out = append(out, fmt.Sprintf("required Actions variable %q missing", n))
	}
	for slug, perms := range requiredApps {
		app, ok := findApp(e.Apps, slug)
		if !ok {
			out = append(out, fmt.Sprintf("required App %q not evidenced as installed", slug))
			continue
		}
		for scope, level := range perms {
			if app.Permissions[scope] != level {
				out = append(out, fmt.Sprintf("App %q permission %s=%q, want %s=%s",
					slug, scope, app.Permissions[scope], scope, level))
			}
		}
	}
	return out
}

func findApp(apps []OwnerSetupApp, slug string) (OwnerSetupApp, bool) {
	for _, a := range apps {
		if a.Slug == slug {
			return a, true
		}
	}
	return OwnerSetupApp{}, false
}

// CheckRunApp is the app identity behind one check run (gh api check-runs).
type CheckRunApp struct {
	ID   int64  `json:"id"`
	Slug string `json:"slug"`
}

// CheckRunsFromApp parses `gh api repos/.../check-runs` output and returns
// the slug of the App that produced the named check run — `policy-review` must
// come from the policy-reviewer App, never from a workflow job (ADR-0072).
func CheckRunsFromApp(data []byte, checkName string) (string, error) {
	var payload struct {
		CheckRuns []struct {
			Name string      `json:"name"`
			App  CheckRunApp `json:"app"`
		} `json:"check_runs"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", err
	}
	for _, r := range payload.CheckRuns {
		if r.Name == checkName {
			if r.App.Slug == "github-actions" || r.App.Slug == "" {
				return "", fmt.Errorf("check run %q produced by app %q (id %d), not the policy-reviewer App",
					checkName, r.App.Slug, r.App.ID)
			}
			return r.App.Slug, nil
		}
	}
	return "", fmt.Errorf("no check run named %q", checkName)
}
