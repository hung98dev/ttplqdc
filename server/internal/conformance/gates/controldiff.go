package gates

import (
	"path"
	"strings"
)

// FileChange is one changed file with its added/removed line contents.
type FileChange struct {
	Path    string
	Added   []string
	Removed []string
}

// ControlDiff is the classification of a PR diff against control-file policy.
type ControlDiff struct {
	// StatusOnly is true when every changed file is an allowed control file
	// and every changed line is an allowed field for the role.
	StatusOnly bool
	Reason     string
}

const (
	taskQueueFile     = "docs/10_implementation/task_queue.md"
	knownBlockersFile = "docs/10_implementation/known_blockers.md"
	evidenceDirPrefix = "docs/10_implementation/evidence/"
)

var taskQueueAllowedKeys = []string{
	"status:", "claimed_by:", "claimed_at:", "branch:", "blocked_by:",
}

// claimKeys are the task_queue packet fields a claim/ PR may touch (never
// blocked_by: — blocker linkage belongs to block/ and ops/).
var claimKeys = []string{"status:", "claimed_by:", "claimed_at:", "branch:"}

// statusPolicy returns the task_queue field allowlist and known_blockers.md
// edit rights for the branch prefix; ok=false means the prefix has no
// status-only fast path (spec/, revert/, unknown prefixes).
//
//	claim/  — claim fields only; no known_blockers.md at all
//	ops/    — all fields; known_blockers.md edits incl. removals
//	block/  — all fields incl. blocked_by:; known_blockers.md additions only
//	imp/    — all fields; no known_blockers.md (status PRs only)
func statusPolicy(branch string) (keys []string, blockersEdit, blockersAdd, ok bool) {
	switch {
	case strings.HasPrefix(branch, "claim/"):
		return claimKeys, false, false, true
	case strings.HasPrefix(branch, "ops/"):
		return taskQueueAllowedKeys, true, true, true
	case strings.HasPrefix(branch, "block/"):
		return taskQueueAllowedKeys, false, true, true
	case strings.HasPrefix(branch, "imp/"):
		return taskQueueAllowedKeys, false, false, true
	default:
		return nil, false, false, false
	}
}

// ClassifyControlDiff decides whether the diff is a status-only control-file
// change for the head branch's prefix (claim/, ops/, block/, imp/ fast path;
// never a DONE status, never an evidence file, never code). Detection only —
// the role-scoped ownership rule is Q0.control.diff (IMP-083).
func ClassifyControlDiff(branch string, changes []FileChange) ControlDiff {
	if len(changes) == 0 {
		return ControlDiff{StatusOnly: false, Reason: "empty diff"}
	}
	keys, blockersEdit, blockersAdd, ok := statusPolicy(branch)
	if !ok {
		return ControlDiff{StatusOnly: false, Reason: "branch prefix has no status-only fast path"}
	}
	for _, ch := range changes {
		switch ch.Path {
		case taskQueueFile:
			if !allLinesTaskQueue(ch, keys) {
				return ControlDiff{StatusOnly: false, Reason: "task_queue.md changes outside claim/status fields"}
			}
		case knownBlockersFile:
			if len(ch.Removed) > 0 && !blockersEdit {
				return ControlDiff{StatusOnly: false, Reason: "known_blockers.md removals are ops/-only"}
			}
			if len(ch.Added) > 0 && !blockersAdd && !blockersEdit {
				return ControlDiff{StatusOnly: false, Reason: "known_blockers.md additions are block/- or ops/-only"}
			}
		default:
			if strings.HasPrefix(ch.Path, evidenceDirPrefix) {
				return ControlDiff{StatusOnly: false, Reason: "evidence files are not status-only"}
			}
			return ControlDiff{StatusOnly: false, Reason: "non-control file changed: " + ch.Path}
		}
	}
	for _, ch := range changes {
		if ch.Path != taskQueueFile {
			continue
		}
		for _, l := range ch.Added {
			if setsDoneStatus(l) {
				return ControlDiff{StatusOnly: false, Reason: "status-only diff sets DONE"}
			}
		}
	}
	return ControlDiff{StatusOnly: true}
}

// allLinesTaskQueue reports whether every touched line is an allowed packet
// field line (per the branch prefix's allowlist) or a summary-table row.
func allLinesTaskQueue(ch FileChange, keys []string) bool {
	for _, l := range append(append([]string{}, ch.Added...), ch.Removed...) {
		if !isAllowedTaskQueueLine(l, keys) {
			return false
		}
	}
	return true
}

func isAllowedTaskQueueLine(line string, keys []string) bool {
	t := strings.TrimSpace(line)
	if t == "" {
		return true
	}
	// summary-table row: | `IMP-000` | ... |
	if strings.HasPrefix(t, "|") && strings.Contains(t, "IMP-") {
		return true
	}
	for _, k := range keys {
		if strings.HasPrefix(t, k) {
			return true
		}
	}
	return false
}

func setsDoneStatus(line string) bool {
	t := strings.TrimSpace(line)
	if strings.HasPrefix(t, "status:") {
		return strings.TrimSpace(strings.TrimPrefix(t, "status:")) == "DONE"
	}
	return false
}

// IsGeneratedBoundaryException reports whether a file inside a generated-only
// directory is an IMP-000-authored boundary exception (the Protocol asmdef and
// scoped csc.rsp live beside generated C# but are authored files, ADR-0059 /
// rules 00-engineering-baseline).
func IsGeneratedBoundaryException(p string) bool {
	p = path.Clean(p)
	return p == "client/Assets/Scripts/Protocol/ThinhThan.Protocol.asmdef" ||
		p == "client/Assets/Scripts/Protocol/ThinhThan.Protocol.asmdef.meta" ||
		p == "client/Assets/Scripts/Protocol/csc.rsp" ||
		p == "client/Assets/Scripts/Protocol/csc.rsp.meta"
}

// IsNumberedMigrationFile reports whether a name inside server/migrations/
// must obey the NNNNNN_name.{up,down}.sql convention. The schema snapshot is
// the single exempt file (engineering_conventions.md §1.6).
func IsNumberedMigrationFile(name string) bool {
	return name != "schema_snapshot.sql"
}
