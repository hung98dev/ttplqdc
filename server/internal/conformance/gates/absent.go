package gates

import (
	"fmt"
	"sort"
	"strings"
)

// PacketSpaceRoots are the planned workspace roots in the implementation
// layout. A tracked file inside one of these must be owned by some packet
// whose status is IN_PROGRESS or DONE.
var PacketSpaceRoots = []string{
	"proto/", "server/", "client/", "scripts/", "deploy/", "tools/", ".github/",
}

// ProtectedRoots are read-only spec space — presence is always legal.
var ProtectedRoots = []string{"docs/", ".devin/"}

// protectedRootFiles are the only unowned root-level files allowed.
var protectedRootFiles = map[string]bool{
	"AGENTS.md": true,
	"README.md": true,
}

// CheckAbsentPaths evaluates Q0.absent_paths: every tracked file under a
// packet-space root must be owned by a live (IN_PROGRESS/DONE) packet; files
// owned by a NOT_STARTED or BLOCKED packet are absent-path violations; files
// under a packet-space root with no owner are unowned-path violations.
func CheckAbsentPaths(idx PacketIndex, tracked []string) []string {
	var violations []string
	for _, f := range tracked {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if underAny(f, ProtectedRoots) || protectedRootFiles[f] {
			continue
		}
		owner := OwnerOf(idx, f)
		switch {
		case owner == nil:
			if underAny(f, PacketSpaceRoots) {
				violations = append(violations, fmt.Sprintf("unowned path in packet space: %s", f))
			} else if !strings.Contains(f, "/") && !strings.HasSuffix(f, ".md") {
				violations = append(violations, fmt.Sprintf("unowned root file: %s", f))
			}
		case owner.Status == "NOT_STARTED" || owner.Status == "BLOCKED":
			violations = append(violations, fmt.Sprintf("absent path present: %s owned by %s packet %s", f, owner.Status, owner.ID))
		}
	}
	sort.Strings(violations)
	return violations
}

func underAny(path string, roots []string) bool {
	for _, r := range roots {
		if strings.HasPrefix(path, r) {
			return true
		}
	}
	return false
}
