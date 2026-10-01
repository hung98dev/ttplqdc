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
// packet-space root must be covered by a live (IN_PROGRESS/DONE) packet;
// files covered only by NOT_STARTED or BLOCKED packets are absent-path
// violations; files under a packet-space root with no owner are unowned-path
// violations. A path covered by both a live and a dead packet is present
// legitimately (e.g. materialized ProjectSettings files a future packet owns
// for editing) — the dead packet's ownership still scopes writes. A .meta
// file marking a directory is legitimate whenever the directory contains a
// tracked file covered by a live packet: Unity generates the marker because
// the directory exists.
func CheckAbsentPaths(idx PacketIndex, tracked []string) []string {
	var violations []string
	inTree := make(map[string]bool, len(tracked))
	for _, f := range tracked {
		inTree[f] = true
	}
	for _, f := range tracked {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if underAny(f, ProtectedRoots) || protectedRootFiles[f] {
			continue
		}
		if strings.HasSuffix(f, ".meta") {
			base := strings.TrimSuffix(f, ".meta")
			if !inTree[base] && liveUnderDir(idx, tracked, base+"/") {
				continue
			}
		}
		owner, live := scanOwners(idx, f)
		switch {
		case live:
			// covered by an IN_PROGRESS/DONE packet — present legitimately
		case owner != nil:
			violations = append(violations, fmt.Sprintf("absent path present: %s owned by %s packet %s", f, owner.Status, owner.ID))
		case underAny(f, PacketSpaceRoots):
			violations = append(violations, fmt.Sprintf("unowned path in packet space: %s", f))
		case !strings.Contains(f, "/") && !strings.HasSuffix(f, ".md"):
			violations = append(violations, fmt.Sprintf("unowned root file: %s", f))
		}
	}
	sort.Strings(violations)
	return violations
}

// scanOwners resolves coverage for path: owner is the packet with the
// longest matching owned_path entry (the write-scope owner, as in OwnerOf),
// and live is true when at least one covering packet is live.
func scanOwners(idx PacketIndex, path string) (owner *Packet, live bool) {
	bestLen := -1
	ids := make([]string, 0, len(idx))
	for id := range idx {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := idx[id]
		for _, own := range p.OwnedPaths {
			if !covers(own, path) {
				continue
			}
			if len(own) > bestLen {
				bestLen = len(own)
				owner = p
			}
			if p.Status != "NOT_STARTED" && p.Status != "BLOCKED" {
				live = true
			}
		}
	}
	return owner, live
}

// liveUnderDir reports whether the directory prefix dir contains at least
// one tracked file covered by a live packet.
func liveUnderDir(idx PacketIndex, tracked []string, dir string) bool {
	for _, f := range tracked {
		if !strings.HasPrefix(f, dir) {
			continue
		}
		if _, live := scanOwners(idx, f); live {
			return true
		}
	}
	return false
}

func underAny(path string, roots []string) bool {
	for _, r := range roots {
		if strings.HasPrefix(path, r) {
			return true
		}
	}
	return false
}
