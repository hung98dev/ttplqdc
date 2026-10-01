package gates

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CheckWorkflowLint (Q0.ci) performs indentation-aware textual checks on the
// workflow without a YAML dependency (F-05). It verifies the required
// step/precondition structure rather than full YAML semantics.
func CheckWorkflowLint(root string) []string {
	path := filepath.Join(root, ".github", "workflows", "verify.yml")
	b, err := os.ReadFile(path)
	if err != nil {
		return []string{"verify.yml unreadable: " + err.Error()}
	}
	lines := strings.Split(string(b), "\n")
	var errs []string

	errs = append(errs, checkJobStructure(lines)...)
	errs = append(errs, checkPreconditionOrder(lines)...)
	errs = append(errs, checkNoLatestLabels(lines)...)
	errs = append(errs, checkNoSecretsBeforeFork(lines)...)
	return errs
}

// jobInfo records a job's id, name and line span.
type jobInfo struct {
	key, name string
	start     int // index of "  <key>:" line
}

func indentOf(line string) int {
	n := 0
	for _, r := range line {
		if r != ' ' {
			break
		}
		n++
	}
	return n
}

func parseJobs(lines []string) []jobInfo {
	var jobs []jobInfo
	inJobs := false
	for i, l := range lines {
		if strings.TrimSpace(l) == "jobs:" {
			inJobs = true
			continue
		}
		if !inJobs {
			continue
		}
		if indentOf(l) == 0 && strings.TrimSpace(l) != "" {
			break // top-level key ends the jobs map
		}
		if indentOf(l) == 2 && strings.HasSuffix(strings.TrimSpace(l), ":") {
			jobs = append(jobs, jobInfo{key: strings.TrimSuffix(strings.TrimSpace(l), ":"), start: i})
		}
	}
	return jobs
}

func checkJobStructure(lines []string) []string {
	var errs []string
	jobs := parseJobs(lines)
	names := map[string]bool{}
	for _, j := range jobs {
		// job display name = first "    name: X" line after job key
		for _, l := range lines[j.start:] {
			if strings.HasPrefix(strings.TrimSpace(l), "name:") {
				j.name = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "name:"))
				break
			}
			if strings.HasPrefix(strings.TrimSpace(l), "runs-on:") || strings.HasPrefix(strings.TrimSpace(l), "steps:") {
				break
			}
		}
		names[j.name] = true
	}
	for _, want := range []string{"Unity (Windows)", "Q0-Q6 verify (Windows)", "Q0-Q6 verify (Linux)"} {
		if !names[want] {
			errs = append(errs, fmt.Sprintf("verify.yml missing required job %q", want))
		}
	}
	return errs
}

// checkPreconditionOrder: inside every job's steps, the first two step names
// must be "Fork guard" then "Freeze check".
func checkPreconditionOrder(lines []string) []string {
	var errs []string
	jobs := parseJobs(lines)
	for _, j := range jobs {
		var stepNames []string
		inSteps := false
		for _, l := range lines[j.start+1:] {
			trim := strings.TrimSpace(l)
			if indentOf(l) <= 2 && strings.HasSuffix(trim, ":") && trim != "steps:" {
				break // next job key or out of the job block
			}
			if trim == "steps:" {
				inSteps = true
				continue
			}
			if !inSteps {
				continue
			}
			if strings.HasPrefix(trim, "- name:") {
				stepNames = append(stepNames, strings.TrimSpace(strings.TrimPrefix(trim, "- name:")))
			}
		}
		if len(stepNames) < 2 {
			errs = append(errs, fmt.Sprintf("job %s has <2 steps", j.key))
			continue
		}
		if stepNames[0] != "Fork guard" || stepNames[1] != "Freeze check" {
			errs = append(errs, fmt.Sprintf("job %s first steps are %q,%q, want Fork guard,Freeze check", j.key, stepNames[0], stepNames[1]))
		}
	}
	return errs
}

var latestRe = regexp.MustCompile(`[a-zA-Z]+-latest`)

func checkNoLatestLabels(lines []string) []string {
	var errs []string
	for i, l := range lines {
		if latestRe.MatchString(l) {
			errs = append(errs, fmt.Sprintf("verify.yml:%d contains forbidden *-latest label: %s", i+1, strings.TrimSpace(l)))
		}
	}
	return errs
}

// checkNoSecretsBeforeFork: no `${{ secrets.*` may appear before the Fork
// guard step of any job (ADR-0058 — secrets must not be in scope for forks).
func checkNoSecretsBeforeFork(lines []string) []string {
	var errs []string
	jobs := parseJobs(lines)
	for _, j := range jobs {
		for _, l := range lines[j.start:] {
			if strings.Contains(l, "- name: Fork guard") {
				break
			}
			if strings.Contains(l, "secrets.") {
				errs = append(errs, fmt.Sprintf("job %s references secrets before Fork guard", j.key))
				break
			}
		}
	}
	return errs
}
