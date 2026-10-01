// Package gates implements the Q0-Q6 admission/verification gate registry:
// packet parsing, role classification, gate activation, scope/plan resolution
// and the IMP-000-owned gate evaluations. Task-graph (DAG), architecture and
// trusted-CI gates belong to IMP-083 / IMP-068 and report SKIP(owner-not-done).
package gates

import (
	"fmt"
	"sort"
	"strings"
)

// Packet is one task packet parsed from docs/10_implementation/task_queue.md.
type Packet struct {
	ID             string
	Status         string
	ClaimedBy      string
	Branch         string
	ClaimedAt      string
	BlockedBy      string
	DependsOn      []string
	OwnedPaths     []string
	ForbiddenPaths []string
}

// PacketIndex maps task id -> packet for O(1) status/ownership lookups.
type PacketIndex map[string]*Packet

// ParseQueue parses every `## \`IMP-XXX\“ packet block in task_queue.md.
// Field matching for `blocks:` / `blocked_by:` is case-insensitive.
func ParseQueue(src string) (PacketIndex, error) {
	idx := PacketIndex{}
	lines := strings.Split(src, "\n")
	var cur *Packet
	var field map[string]func(string)

	flush := func() {
		if cur != nil {
			idx[cur.ID] = cur
			cur = nil
		}
	}
	setField := func() {
		field = map[string]func(string){
			"status":          func(v string) { cur.Status = v },
			"claimed_by":      func(v string) { cur.ClaimedBy = v },
			"branch":          func(v string) { cur.Branch = v },
			"claimed_at":      func(v string) { cur.ClaimedAt = v },
			"blocked_by":      func(v string) { cur.BlockedBy = v },
			"blocks":          func(v string) { cur.BlockedBy = v }, // blocks: entries read the same way
			"depends_on":      func(v string) { cur.DependsOn = parseList(v) },
			"owned_paths":     func(v string) { cur.OwnedPaths = parseList(v) },
			"forbidden_paths": func(v string) { cur.ForbiddenPaths = parseList(v) },
		}
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "## `IMP-") {
			flush()
			cur = &Packet{}
			setField()
			// Heading form: ## `IMP-XXX` — <title>
			rest := strings.TrimPrefix(trim, "## `")
			if end := strings.IndexByte(rest, '`'); end > 0 {
				if id := rest[:end]; strings.HasPrefix(id, "IMP-") {
					cur.ID = id
				}
			}
			continue
		}
		if cur == nil {
			continue
		}
		// Packet ends at the next `## ` heading or a second-level markdown heading.
		if strings.HasPrefix(trim, "## ") && !strings.HasPrefix(trim, "## `IMP-") {
			flush()
			continue
		}
		key, val, ok := splitField(trim)
		if !ok {
			continue
		}
		// Multi-line bracket lists: accumulate until ']' closes.
		for strings.Count(val, "[") > strings.Count(val, "]") && i+1 < len(lines) {
			i++
			val += " " + strings.TrimSpace(lines[i])
		}
		if fn, ok := field[strings.ToLower(key)]; ok {
			fn(val)
		}
		if cur.ID == "" && strings.EqualFold(key, "id") {
			cur.ID = val
		}
	}
	flush()
	if len(idx) == 0 {
		return nil, fmt.Errorf("no IMP packets parsed")
	}
	return idx, nil
}

func splitField(line string) (key, val string, ok bool) {
	i := strings.Index(line, ":")
	if i < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:i])
	val = strings.TrimSpace(line[i+1:])
	if key == "" {
		return "", "", false
	}
	return key, val, true
}

// parseList reads a `[a, b, "c", \`d\`]` bracket list into trimmed entries.
func parseList(v string) []string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "[") || !strings.HasSuffix(v, "]") {
		v = strings.Trim(v, "`\"")
		if v == "" {
			return nil
		}
		return []string{v}
	}
	inner := strings.TrimSpace(v[1 : len(v)-1])
	if inner == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(inner, ",") {
		p := strings.Trim(strings.TrimSpace(part), "`\"")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// covers reports whether owned_path entry own covers path: a trailing-slash
// entry is a directory prefix grant, an entry without a trailing slash an
// exact-file grant; a .meta path is also covered when its base is covered
// (the .meta companion is owned with its asset).
func covers(own, path string) bool {
	if strings.HasSuffix(own, "/") {
		if strings.HasPrefix(path, own) {
			return true
		}
	} else if path == own {
		return true
	}
	if strings.HasSuffix(path, ".meta") {
		base := strings.TrimSuffix(path, ".meta")
		if strings.HasSuffix(own, "/") {
			return strings.HasPrefix(base, own)
		}
		return base == own
	}
	return false
}

// OwnerOf returns the packet that owns path under the longest-prefix rule.
func OwnerOf(idx PacketIndex, path string) *Packet {
	var best *Packet
	bestLen := -1
	ids := make([]string, 0, len(idx))
	for id := range idx {
		ids = append(ids, id)
	}
	sort.Strings(ids) // deterministic longest-prefix resolution
	for _, id := range ids {
		p := idx[id]
		for _, own := range p.OwnedPaths {
			if covers(own, path) && len(own) > bestLen {
				bestLen = len(own)
				best = p
			}
		}
	}
	return best
}
