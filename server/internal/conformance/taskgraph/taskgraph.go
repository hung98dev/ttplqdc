// Package taskgraph implements the Q0 task-graph checks of
// docs/10_implementation/audit_gates.md: packet grammar, DAG integrity,
// claim/state semantics, requirement-ID coverage and control-file diff rules.
// Exported Check* entry points return violation details ([]string, empty =
// pass) for the gates.Runner evaluator cases wired by the IMP-000 gatefix.
package taskgraph

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	queueFile    = "docs/10_implementation/task_queue.md"
	blockersFile = "docs/10_implementation/known_blockers.md"
	depGraphFile = "docs/10_implementation/dependency_graph.md"
	evidencePref = "docs/10_implementation/evidence/"
	controlGlob  = "docs/10_implementation/"
	conventions  = "docs/10_implementation/engineering_conventions.md"
	reqPattern   = `[A-Z]{2,6}-\d{3}`
	openSection  = "Open Blockers"
	resSection   = "Resolved Blockers"
)

var (
	packetHeadRe = regexp.MustCompile(`^##\s+` + "`" + `(IMP-\d+)` + "`")
	summaryRowRe = regexp.MustCompile(`^\|\s*` + "`" + `(IMP-\d+)` + "`" + `\s*\|`)
	fieldRe      = regexp.MustCompile(`^([a-z_]+):\s*(.*)$`)
	impRefRe     = regexp.MustCompile(`\bIMP-\d{3}\b`)
	blockRefRe   = regexp.MustCompile(`\b(BLK|OPS)-\d{3}\b`)
	revertRefRe  = regexp.MustCompile(`^REVERT-[0-9a-f]{7,40}$`)
	blockedByRe  = regexp.MustCompile(`^(BLK-\d{3}|OPS-\d{3}|REVERT-[0-9a-f]{7,40})$`)
	reqIDRe      = regexp.MustCompile(reqPattern)
	entryHeadRe  = regexp.MustCompile(`^###\s+` + "`" + `(BLK-\d+|OPS-\d+)` + "`")
	layerRowRe   = regexp.MustCompile(`^\|\s*` + "`?" + `(IMP-\d+)` + "`?" + `\s*\|\s*(\d+)\s*\|`)
)

// Packet is one parsed task packet (per docs/templates/task.md).
type Packet struct {
	ID             string
	Title          string
	Status         string
	ClaimedBy      string
	Branch         string
	ClaimedAt      string
	BlockedBy      string
	Specs          []string
	Adrs           []string
	DependsOn      []string
	OwnedPaths     []string
	ForbiddenPaths []string
	Change         string
	Acceptance     string
	Tests          string
}

// Queue is the parsed task_queue.md: packets in file order plus the summary
// table's per-ID status cell.
type Queue struct {
	Packets  []*Packet
	ByID     map[string]*Packet
	Summary  map[string]string // IMP id -> summary-row status cell (`` ``-stripped)
	PacketID map[string]bool
}

// BlockerEntry is one ### BLK-xxx/OPS-xxx record in known_blockers.md.
type BlockerEntry struct {
	ID      string
	Section string // "open" or "resolved"
	Blocks  []string
}

var listKeys = map[string]bool{
	"specs": true, "adrs": true, "depends_on": true,
	"owned_paths": true, "forbidden_paths": true,
	"contract_inputs": true, "contract_outputs": true,
	"consumers_checked": true, "generated_artifacts": true,
	"cleanup_obligations": true,
}

// ParseQueue parses task_queue.md text into a Queue.
func ParseQueue(text string) (*Queue, error) {
	q := &Queue{ByID: map[string]*Packet{}, Summary: map[string]string{}, PacketID: map[string]bool{}}
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		m := summaryRowRe.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		cells := strings.Split(lines[i], "|")
		if len(cells) >= 4 {
			q.Summary[m[1]] = strings.Trim(strings.TrimSpace(cells[3]), "` ")
		}
	}
	var cur *Packet
	var section string
	var listKey string
	var listAcc strings.Builder
	flushList := func() {
		if listKey != "" {
			assignList(cur, listKey, parseList(listAcc.String()))
			listKey = ""
			listAcc.Reset()
		}
	}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if m := packetHeadRe.FindStringSubmatch(line); m != nil {
			flushList()
			cur = &Packet{ID: m[1], Title: line}
			q.Packets = append(q.Packets, cur)
			if q.ByID[m[1]] != nil {
				return nil, fmt.Errorf("duplicate packet %s", m[1])
			}
			q.ByID[m[1]] = cur
			q.PacketID[m[1]] = true
			section = ""
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(line, "## ") && !packetHeadRe.MatchString(line) {
			flushList()
			section = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			continue
		}
		trim := strings.TrimSpace(line)
		if listKey != "" {
			listAcc.WriteString(" " + trim)
			if strings.Contains(trim, "]") {
				flushList()
			}
			continue
		}
		if fm := fieldRe.FindStringSubmatch(trim); fm != nil && strings.Contains(trim, ":") {
			key, val := fm[1], fm[2]
			if _, ok := map[string]bool{"id": true, "status": true, "claimed_by": true, "branch": true, "claimed_at": true, "blocked_by": true, "evidence_location": true}[key]; ok || listKeys[key] {
				if listKeys[key] {
					if strings.Contains(val, "[") && !strings.Contains(val, "]") {
						listKey = key
						listAcc.WriteString(val)
						continue
					}
					assignList(cur, key, parseList(val))
				} else {
					assignField(cur, key, val)
				}
				continue
			}
		}
		switch section {
		case "Change":
			cur.Change += line + "\n"
		case "Acceptance":
			cur.Acceptance += line + "\n"
		case "Tests":
			cur.Tests += line + "\n"
		}
	}
	flushList()
	if len(q.Packets) == 0 {
		return nil, fmt.Errorf("no packets parsed")
	}
	return q, nil
}

func assignField(p *Packet, key, val string) {
	v := strings.Trim(strings.TrimSpace(val), `"`)
	switch key {
	case "status":
		p.Status = v
	case "claimed_by":
		p.ClaimedBy = v
	case "branch":
		p.Branch = v
	case "claimed_at":
		p.ClaimedAt = v
	case "blocked_by":
		p.BlockedBy = v
	}
}

func assignList(p *Packet, key string, vals []string) {
	switch key {
	case "specs":
		p.Specs = vals
	case "adrs":
		p.Adrs = vals
	case "depends_on":
		p.DependsOn = vals
	case "owned_paths":
		p.OwnedPaths = vals
	case "forbidden_paths":
		p.ForbiddenPaths = vals
	}
}

// parseList splits a `[a, `b`, "c"]` bracket list.
func parseList(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	var out []string
	for _, item := range strings.Split(s, ",") {
		item = strings.Trim(strings.TrimSpace(item), "`\"'")
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

// LoadQueue reads and parses root's task_queue.md.
func LoadQueue(root string) (*Queue, error) {
	b, err := os.ReadFile(path.Join(root, queueFile))
	if err != nil {
		return nil, err
	}
	return ParseQueue(string(b))
}

// ParseBlockers parses known_blockers.md text into section-tagged entries.
func ParseBlockers(text string) map[string]BlockerEntry {
	out := map[string]BlockerEntry{}
	section := ""
	var cur *BlockerEntry
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			t := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if strings.Contains(t, openSection) {
				section = "open"
			} else if strings.Contains(t, resSection) {
				section = "resolved"
			}
			continue
		}
		if m := entryHeadRe.FindStringSubmatch(line); m != nil {
			e := BlockerEntry{ID: m[1], Section: section}
			out[m[1]] = e
			cur = &e
			continue
		}
		if cur != nil && strings.HasPrefix(strings.TrimSpace(line), "blocks:") {
			e := out[cur.ID]
			e.Blocks = append(e.Blocks, impRefRe.FindAllString(line, -1)...)
			if strings.Contains(line, "ALL") {
				e.Blocks = append(e.Blocks, "ALL")
			}
			out[cur.ID] = e
		}
	}
	return out
}

// covers reports whether owned path list grants path p: a `dir/` entry is a
// prefix grant, otherwise exact match; `P.meta` is implied by coverage of P,
// and a folder's own `.meta` (D.meta for owned `D/`) is implied too.
func covers(own []string, p string) bool {
	for _, o := range own {
		if o == p {
			return true
		}
		if strings.HasSuffix(o, "/") {
			if strings.HasPrefix(p, o) {
				return true
			}
			// owning dir D/ implies D.meta (the folder's own meta).
			if strings.HasSuffix(p, ".meta") && strings.TrimSuffix(o, "/")+".meta" == p {
				return true
			}
			continue
		}
		if strings.HasSuffix(p, ".meta") && strings.TrimSuffix(p, ".meta") == o {
			return true
		}
	}
	return false
}

// ownerOf returns the packet owning p via longest matching owned path,
// including .meta-implied ownership (P.meta owned with P; owned dir D/ owns
// D.meta and everything under it).
func ownerOf(q *Queue, p string) string {
	best, bestLen := "", -1
	for _, pk := range q.Packets {
		for _, o := range pk.OwnedPaths {
			if !covers([]string{o}, p) {
				continue
			}
			if len(o) > bestLen {
				best, bestLen = pk.ID, len(o)
			}
		}
	}
	return best
}

// ---------- helpers ----------

const cmdTimeout = 15 * time.Minute

// execTimed mirrors gates.ExecTimed: bounded subprocess + stderr progress.
func execTimed(dir string, argv ...string) (string, error) {
	fmt.Fprintf(os.Stderr, "verify: $ %s\n", strings.Join(argv, " "))
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func gitOut(root string, args ...string) (string, error) {
	argv := append([]string{"git"}, args...)
	return execTimed(root, argv...)
}

func showFile(root, ref, file string) (string, error) {
	return gitOut(root, "show", ref+":"+file)
}

// ---------- DAG / links / state checks ----------

// CheckDag evaluates queue-level graph integrity for Q0.dag: depends_on
// targets exist and are acyclic, no dangling IMP/BLK/OPS/REVERT references,
// spec/ADR link targets exist on disk, dependency_graph.md layer ordering
// holds, owned/forbidden paths do not intersect and cross-packet ownership
// overlap is ordered by depends_on, and every `## Tests` path is inside the
// packet's owned_paths.
func CheckDag(root string) []string {
	q, err := LoadQueue(root)
	if err != nil {
		return []string{"task_queue.md: " + err.Error()}
	}
	var out []string
	out = append(out, checkDag(q)...)
	out = append(out, checkDangling(q, root)...)
	out = append(out, checkOverlap(q)...)
	out = append(out, checkTestsOwned(q)...)
	out = append(out, checkLayers(q, root)...)
	return out
}

func checkDag(q *Queue) []string {
	var out []string
	for _, p := range q.Packets {
		for _, d := range p.DependsOn {
			if !q.PacketID[d] {
				out = append(out, fmt.Sprintf("%s depends_on unknown task %s", p.ID, d))
			}
		}
	}
	// Cycle detection: DFS with coloring.
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var stack []string
	var visit func(id string) bool
	visit = func(id string) bool {
		color[id] = gray
		stack = append(stack, id)
		for _, d := range q.ByID[id].DependsOn {
			if q.ByID[d] == nil {
				continue
			}
			if color[d] == gray {
				out = append(out, "depends_on cycle: "+strings.Join(append(stack, d), " -> "))
				return false
			}
			if color[d] == white && !visit(d) {
				return false
			}
		}
		stack = stack[:len(stack)-1]
		color[id] = black
		return true
	}
	for _, p := range q.Packets {
		if color[p.ID] == white {
			visit(p.ID)
		}
	}
	return out
}

func checkDangling(q *Queue, root string) []string {
	var out []string
	blockers, _ := loadBlockers(root)
	for _, p := range q.Packets {
		if p.BlockedBy != "" {
			switch {
			case strings.HasPrefix(p.BlockedBy, "REVERT-"):
				if !revertRefRe.MatchString(p.BlockedBy) {
					out = append(out, fmt.Sprintf("%s blocked_by %q is not a REVERT-<sha>", p.ID, p.BlockedBy))
				}
			case blockRefRe.MatchString(p.BlockedBy):
				if _, ok := blockers[p.BlockedBy]; !ok {
					out = append(out, fmt.Sprintf("%s blocked_by %s has no entry in known_blockers.md", p.ID, p.BlockedBy))
				}
			default:
				out = append(out, fmt.Sprintf("%s blocked_by %q is not BLK-xxx/OPS-xxx/REVERT-<sha>", p.ID, p.BlockedBy))
			}
		}
		for _, s := range p.Specs {
			if !fileExists(root, resolveDocPath(s)) {
				out = append(out, fmt.Sprintf("%s specs entry %q does not resolve to a file", p.ID, s))
			}
		}
		for _, a := range p.Adrs {
			ap := path.Join(root, "docs/11_decisions", a)
			if !fileExists(root, resolveDocPath(a)) && !fileExistsPath(ap) {
				out = append(out, fmt.Sprintf("%s adrs entry %q does not resolve to docs/11_decisions/", p.ID, a))
			}
		}
	}
	return out
}

func resolveDocPath(ref string) string {
	ref = strings.Trim(ref, "`\"")
	if strings.HasPrefix(ref, "docs/") {
		return ref
	}
	return path.Join("docs/10_implementation", ref)
}

func fileExists(root, rel string) bool {
	return fileExistsPath(path.Join(root, rel))
}

func fileExistsPath(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func checkOverlap(q *Queue) []string {
	var out []string
	for _, p := range q.Packets {
		for _, o := range p.OwnedPaths {
			if covers(p.ForbiddenPaths, o) || covers(p.ForbiddenPaths, strings.TrimSuffix(o, "/")) {
				out = append(out, fmt.Sprintf("%s owned path %s intersects its forbidden_paths", p.ID, o))
			}
		}
	}
	for i := 0; i < len(q.Packets); i++ {
		for j := i + 1; j < len(q.Packets); j++ {
			a, b := q.Packets[i], q.Packets[j]
			shared := sharedPaths(a.OwnedPaths, b.OwnedPaths)
			if len(shared) == 0 {
				continue
			}
			if !dependsTrans(q, a.ID, b.ID) && !dependsTrans(q, b.ID, a.ID) {
				out = append(out, fmt.Sprintf("%s and %s share owned path(s) %s without a depends_on ordering",
					a.ID, b.ID, strings.Join(shared, ", ")))
			}
		}
	}
	return out
}

// unorderedSharedExempt reports whether an identical owned path may be shared
// without a depends_on ordering: the Addressables append-registry grants
// (exact files only — settings + group .assets under the shared registry dir,
// disjoint GUID/address namespaces per repository_layout.md) and the
// co-ownable module lockfile pair.
func unorderedSharedExempt(p string) bool {
	if p == "server/go.mod" || p == "server/go.sum" {
		return true
	}
	const reg = "client/Assets/AddressableAssetsData/"
	if p == reg+"AddressableAssetSettings.asset" {
		return true
	}
	if strings.HasPrefix(p, reg+"AssetGroups/") && strings.HasSuffix(p, ".asset") {
		return true
	}
	return false
}

func sharedPaths(a, b []string) []string {
	var out []string
	for _, x := range a {
		for _, y := range b {
			shared := x == y ||
				(strings.HasSuffix(x, "/") && strings.HasPrefix(y, x)) ||
				(strings.HasSuffix(y, "/") && strings.HasPrefix(x, y))
			if !shared {
				continue
			}
			if x == y && unorderedSharedExempt(x) {
				continue
			}
			out = append(out, x+" ~ "+y)
		}
	}
	return out
}

func dependsTrans(q *Queue, from, to string) bool {
	seen := map[string]bool{}
	var walk func(id string) bool
	walk = func(id string) bool {
		if id == to {
			return true
		}
		if seen[id] {
			return false
		}
		seen[id] = true
		p := q.ByID[id]
		if p == nil {
			return false
		}
		for _, d := range p.DependsOn {
			if walk(d) {
				return true
			}
		}
		return false
	}
	return walk(from)
}

// testsPaths extracts backticked path entries from a `## Tests` section.
func testsPaths(tests string) []string {
	var out []string
	for _, line := range strings.Split(tests, "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "-") {
			continue
		}
		i := strings.Index(t, "`")
		j := strings.Index(t[i+1:], "`")
		if i >= 0 && j > 0 {
			out = append(out, t[i+1:i+1+j])
		}
	}
	return out
}

func checkTestsOwned(q *Queue) []string {
	var out []string
	for _, p := range q.Packets {
		for _, tp := range testsPaths(p.Tests) {
			if !covers(p.OwnedPaths, tp) {
				out = append(out, fmt.Sprintf("%s tests path %s is outside owned_paths", p.ID, tp))
			}
		}
	}
	return out
}

// checkLayers verifies dependency_graph.md layering: a task's layer is never
// lower than any dependency's layer.
func checkLayers(q *Queue, root string) []string {
	b, err := os.ReadFile(path.Join(root, depGraphFile))
	if err != nil {
		return []string{"dependency_graph.md: " + err.Error()}
	}
	layer := map[string]int{}
	for _, line := range strings.Split(string(b), "\n") {
		if m := layerRowRe.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[2])
			layer[m[1]] = n
		}
	}
	var out []string
	for _, p := range q.Packets {
		pl, ok := layer[p.ID]
		if !ok {
			out = append(out, fmt.Sprintf("%s missing from dependency_graph.md Task→Layer table", p.ID))
			continue
		}
		for _, d := range p.DependsOn {
			if dl, ok := layer[d]; ok && pl < dl {
				out = append(out, fmt.Sprintf("%s layer %d below dependency %s layer %d (layer inversion)", p.ID, pl, d, dl))
			}
		}
	}
	return out
}

// ---------- claim/state checks ----------

// CheckClaims evaluates packet status/claim-field invariants for Q0.claims:
// legal status values, summary-row parity, claim fields per status, blocked_by
// reference validity and openness, IN_PROGRESS/DONE only with DONE deps, and
// .meta-implied ownership consistency over tracked paths.
func CheckClaims(root string) []string {
	q, err := LoadQueue(root)
	if err != nil {
		return []string{"task_queue.md: " + err.Error()}
	}
	blockers, berr := loadBlockers(root)
	var out []string
	if berr != nil {
		out = append(out, "known_blockers.md: "+berr.Error())
	}
	out = append(out, checkClaims(q, blockers)...)
	tracked, terr := gitOut(root, "ls-files")
	if terr != nil {
		return append(out, "git ls-files: "+terr.Error())
	}
	out = append(out, checkMetaOwnership(q, strings.Fields(tracked))...)
	return out
}

func loadBlockers(root string) (map[string]BlockerEntry, error) {
	b, err := os.ReadFile(path.Join(root, blockersFile))
	if err != nil {
		return nil, err
	}
	return ParseBlockers(string(b)), nil
}

func checkClaims(q *Queue, blockers map[string]BlockerEntry) []string {
	var out []string
	for _, p := range q.Packets {
		switch p.Status {
		case "NOT_STARTED", "IN_PROGRESS", "BLOCKED", "DONE":
		default:
			out = append(out, fmt.Sprintf("%s illegal status %q", p.ID, p.Status))
		}
		if s, ok := q.Summary[p.ID]; ok && s != p.Status {
			out = append(out, fmt.Sprintf("%s summary-row status %q != packet status %q", p.ID, s, p.Status))
		}
		switch p.Status {
		case "IN_PROGRESS":
			if p.ClaimedBy == "" || p.Branch == "" || p.ClaimedAt == "" {
				out = append(out, fmt.Sprintf("%s IN_PROGRESS with empty claim fields", p.ID))
			}
			if p.BlockedBy != "" {
				out = append(out, fmt.Sprintf("%s IN_PROGRESS but blocked_by set", p.ID))
			}
		case "BLOCKED":
			if !blockedByRe.MatchString(p.BlockedBy) {
				out = append(out, fmt.Sprintf("%s BLOCKED with blocked_by %q not BLK-/OPS-/REVERT-", p.ID, p.BlockedBy))
			} else if e, ok := blockers[p.BlockedBy]; ok && e.Section == "resolved" {
				out = append(out, fmt.Sprintf("%s BLOCKED by resolved entry %s", p.ID, p.BlockedBy))
			}
		case "NOT_STARTED":
			if p.ClaimedBy != "" || p.Branch != "" || p.ClaimedAt != "" || p.BlockedBy != "" {
				out = append(out, fmt.Sprintf("%s NOT_STARTED with non-empty claim fields", p.ID))
			}
		case "DONE":
			if p.BlockedBy != "" {
				out = append(out, fmt.Sprintf("%s DONE but blocked_by set", p.ID))
			}
		}
		if p.Status == "IN_PROGRESS" || p.Status == "DONE" {
			for _, d := range p.DependsOn {
				if dp := q.ByID[d]; dp != nil && dp.Status != "DONE" {
					out = append(out, fmt.Sprintf("%s is %s but dependency %s is %s", p.ID, p.Status, d, dp.Status))
				}
			}
		}
	}
	return out
}

// ---------- .meta-implied ownership ----------

// CheckMetaOwnership verifies (for a queue + tracked path list) that every
// tracked `.meta` resolves to the owner of its asset or of the folder it
// documents, and that no packet explicitly claims a `.meta` whose base is
// owned by another packet. Q0 treats `P.meta` and first-created folder metas
// as owned with P (repository_layout.md § Ownership Rules).
func CheckMetaOwnership(q *Queue, tracked []string) []string {
	return checkMetaOwnership(q, tracked)
}

func checkMetaOwnership(q *Queue, tracked []string) []string {
	var out []string
	fileSet := map[string]bool{}
	for _, t := range tracked {
		fileSet[t] = true
	}
	// Explicit-claim conflicts: a packet lists X.meta while another owns X
	// (or the folder X/ is listed by another packet).
	for _, p := range q.Packets {
		for _, o := range p.OwnedPaths {
			if !strings.HasSuffix(o, ".meta") {
				continue
			}
			base := strings.TrimSuffix(o, ".meta")
			if other := ownerOf(q, base); other != "" && other != p.ID {
				out = append(out, fmt.Sprintf("%s claims %s but %s owns %s", p.ID, o, other, base))
			}
			for _, q2 := range q.Packets {
				if q2.ID != p.ID && covers(q2.OwnedPaths, base+"/") {
					out = append(out, fmt.Sprintf("%s claims %s but %s owns folder %s", p.ID, o, q2.ID, base+"/"))
				}
			}
		}
	}
	// Tracked asset metas: owner(file.meta) must equal owner(file) — the meta
	// is implied by the asset's owner; a different explicit/meta owner means
	// another packet's claim covers the meta but not the asset.
	for _, t := range tracked {
		if !strings.HasSuffix(t, ".meta") || !fileSet[strings.TrimSuffix(t, ".meta")] {
			continue
		}
		base := strings.TrimSuffix(t, ".meta")
		b, m := ownerOf(q, base), ownerOf(q, t)
		switch {
		case b != "" && m == "":
			out = append(out, fmt.Sprintf("%s unowned though %s is owned by %s", t, base, b))
		case b == "" && m != "":
			out = append(out, fmt.Sprintf("%s owned by %s but its asset %s is unowned", t, m, base))
		case b != m:
			out = append(out, fmt.Sprintf("%s owned by %s but %s owned by %s", t, m, base, b))
		}
	}
	return out
}

// ---------- requirement-ID coverage ----------

// CollectReqIDs scans spec "Requirement IDs" tables under docs/00_context..
// docs/09_testing plus engineering_conventions.md; returns id -> spec file.
func CollectReqIDs(root string) (map[string]string, error) {
	out := map[string]string{}
	for d := 0; d <= 9; d++ {
		dir := path.Join(root, fmt.Sprintf("docs/%02d_", d))
		matches, err := matchDirs(dir)
		if err != nil {
			return nil, err
		}
		for _, m := range matches {
			err := collectFromDir(m, out)
			if err != nil {
				return nil, err
			}
		}
	}
	if err := collectFromFile(path.Join(root, conventions), out); err != nil {
		return nil, err
	}
	return out, nil
}

func matchDirs(prefix string) ([]string, error) {
	base := path.Dir(prefix)
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, nil // absent docs tree — nothing to collect
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), path.Base(prefix)) {
			out = append(out, path.Join(base, e.Name()))
		}
	}
	return out, nil
}

func collectFromDir(dir string, out map[string]string) error {
	return walkFiles(dir, func(p string) error {
		if strings.HasSuffix(p, ".md") {
			return collectFromFile(p, out)
		}
		return nil
	})
}

func walkFiles(dir string, fn func(string) error) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		p := path.Join(dir, e.Name())
		if e.IsDir() {
			if err := walkFiles(p, fn); err != nil {
				return err
			}
		} else if err := fn(p); err != nil {
			return err
		}
	}
	return nil
}

// collectFromFile finds a "Requirement IDs" heading and parses `| ID |` rows.
func collectFromFile(file string, out map[string]string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	inTable := false
	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") {
			inTable = strings.Contains(t, "Requirement IDs")
			continue
		}
		if !inTable || !strings.HasPrefix(t, "|") {
			continue
		}
		cells := strings.Split(t, "|")
		if len(cells) < 3 {
			continue
		}
		id := strings.Trim(strings.TrimSpace(cells[1]), "` ")
		if reqIDRe.MatchString(id) && id != "ID" {
			out[id] = file
		}
	}
	return nil
}

// CheckReqCoverage verifies every spec Requirement ID is named in one
// packet's `## Acceptance` and that same packet's `## Tests` (Q0.req.coverage).
func CheckReqCoverage(root string) []string {
	ids, err := CollectReqIDs(root)
	if err != nil {
		return []string{"collect requirement IDs: " + err.Error()}
	}
	q, err := LoadQueue(root)
	if err != nil {
		return []string{"task_queue.md: " + err.Error()}
	}
	return checkReqCoverage(q, ids, root)
}

func checkReqCoverage(q *Queue, ids map[string]string, root string) []string {
	var out []string
	sorted := make([]string, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)
	for _, id := range sorted {
		found := false
		for _, p := range q.Packets {
			if strings.Contains(p.Acceptance, id) && strings.Contains(p.Tests, id) {
				found = true
				break
			}
		}
		if !found {
			out = append(out, fmt.Sprintf("requirement %s (%s) not named in one packet's Acceptance and Tests", id, ids[id]))
		}
	}
	return out
}

// ---------- control-file diff rules ----------

// FileChange mirrors gates.FileChange for control-diff classification
type FileChange struct {
	Path    string
	Added   []string
	Removed []string
}

// BlockerDiff describes what a diff did to known_blockers.md entries.
type BlockerDiff struct {
	Added    map[string]bool
	Removed  map[string]bool
	Resolved map[string]bool // moved open -> resolved
}

// CheckControlDiff evaluates a PR's control-file diff for Q0.control.diff
// per audit_gates.md § Protected Paths. baseRef is the merge base; the head
// queue/blockers come from the working tree (repo root checkout).
func CheckControlDiff(root, baseRef, headBranch string) ([]string, error) {
	nameStatus, err := gitOut(root, "diff", "--name-status", baseRef+"...HEAD", "--", "docs/10_implementation/")
	if err != nil {
		return nil, err
	}
	var tqChanged, kbChanged bool
	var otherControl []string
	var evidence []string
	for _, line := range strings.Split(strings.TrimSpace(nameStatus), "\n") {
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		st, p := f[0], f[1]
		switch {
		case p == queueFile:
			tqChanged = true
		case p == blockersFile:
			kbChanged = true
		case strings.HasPrefix(p, evidencePref):
			if st != "A" && st != "M" {
				evidence = append(evidence, "non-add/modify change on evidence file "+p)
			} else {
				evidence = append(evidence, p)
			}
		case strings.HasPrefix(p, controlGlob) && strings.HasSuffix(p, ".md") && !strings.Contains(p[len(controlGlob):], "/"):
			otherControl = append(otherControl, p)
		}
	}
	var baseQ, headQ *Queue
	if tqChanged {
		bt, err := showFile(root, baseRef, queueFile)
		if err != nil {
			return nil, fmt.Errorf("base queue: %w", err)
		}
		if baseQ, err = ParseQueue(bt); err != nil {
			return nil, fmt.Errorf("base queue parse: %w", err)
		}
		if headQ, err = LoadQueue(root); err != nil {
			return nil, fmt.Errorf("head queue parse: %w", err)
		}
	}
	var bd BlockerDiff
	if kbChanged {
		d, err := blockerDiff(root, baseRef)
		if err != nil {
			return nil, err
		}
		bd = d
	}
	details := checkControlDiff(baseQ, headQ, bd, headBranch, kbChanged, otherControl, evidence)
	return details, nil
}

// blockerDiff computes added/removed/resolved entry IDs in known_blockers.md
// between baseRef and HEAD (head = working tree file).
func blockerDiff(root, baseRef string) (BlockerDiff, error) {
	out := BlockerDiff{Added: map[string]bool{}, Removed: map[string]bool{}, Resolved: map[string]bool{}}
	bt, err := showFile(root, baseRef, blockersFile)
	if err != nil {
		return out, fmt.Errorf("base blockers: %w", err)
	}
	hb, err := os.ReadFile(path.Join(root, blockersFile))
	if err != nil {
		return out, fmt.Errorf("head blockers: %w", err)
	}
	base := ParseBlockers(bt)
	head := ParseBlockers(string(hb))
	for id, e := range head {
		be, ok := base[id]
		if !ok {
			out.Added[id] = true
		} else if be.Section == "open" && e.Section == "resolved" {
			out.Resolved[id] = true
		}
	}
	for id, e := range base {
		if _, ok := head[id]; !ok {
			out.Removed[id] = true
		} else if e.Section == "resolved" {
			// remains resolved
		}
	}
	return out, nil
}

// checkControlDiff is the pure role/transition evaluator (unit-testable
// without git). baseQ/headQ are nil when task_queue.md is unchanged; bd is
// the known_blockers entry diff; otherControl lists other top-level
// docs/10_implementation/*.md files touched; evidence lists evidence/** paths.
func checkControlDiff(baseQ, headQ *Queue, bd BlockerDiff, branch string, kbChanged bool, otherControl, evidence []string) []string {
	var out []string
	prefix := branchPrefix(branch)
	impID := impIDFromBranch(branch)

	// evidence/** only on imp/IMP-XXX-*, own ID, additions/modifications only.
	for _, ev := range evidence {
		if strings.HasPrefix(ev, "non-add/modify") {
			out = append(out, ev)
			continue
		}
		if prefix != "imp" || !strings.HasPrefix(ev, evidencePref+impID+"/") {
			out = append(out, fmt.Sprintf("evidence path %s not writable by branch %s", ev, branch))
		}
	}
	// other top-level control .md: spec-owner (spec/) or coordinator
	// (claim/, ops/) only.
	if len(otherControl) > 0 && prefix != "spec" && prefix != "ops" && prefix != "claim" {
		out = append(out, fmt.Sprintf("control file(s) %s changed by non spec-owner/coordinator branch %s",
			strings.Join(otherControl, ", "), branch))
	}
	if baseQ == nil && !kbChanged {
		return out
	}
	switch prefix {
	case "spec":
		// spec-owner: unrestricted on control files.
	case "revert":
		out = append(out, checkRevertTransitions(baseQ, headQ)...)
	case "claim":
		out = append(out, checkClaimTransitions(baseQ, headQ)...)
		if kbChanged {
			out = append(out, "claim/ branch may not change known_blockers.md")
		}
	case "block":
		out = append(out, checkBlockPR(baseQ, headQ, bd, impID)...)
	case "ops":
		out = append(out, checkOpsPR(baseQ, headQ, bd)...)
	case "imp":
		out = append(out, checkImpPR(baseQ, headQ, impID, branch)...)
		if kbChanged {
			out = append(out, "imp/ branch may not change known_blockers.md")
		}
	default:
		if baseQ != nil {
			out = append(out, fmt.Sprintf("branch %s may not change task_queue.md", branch))
		}
		if kbChanged {
			out = append(out, fmt.Sprintf("branch %s may not change known_blockers.md", branch))
		}
	}
	return out
}

func branchPrefix(branch string) string {
	if i := strings.Index(branch, "/"); i > 0 {
		return branch[:i]
	}
	return ""
}

func impIDFromBranch(branch string) string {
	m := regexp.MustCompile(`^(?:imp|block)/(IMP-\d{3})`).FindStringSubmatch(branch)
	if m != nil {
		return m[1]
	}
	return ""
}

// packetDiff reports which fields of a packet changed between base and head.
type fieldDiff struct {
	status, blockedBy, claimFields, other bool
}

func diffPackets(base, head *Queue) map[string]fieldDiff {
	out := map[string]fieldDiff{}
	if base == nil || head == nil {
		// task_queue.md unchanged — no packet diff exists to validate.
		return out
	}
	for id, bp := range base.ByID {
		hp := head.ByID[id]
		fd := fieldDiff{}
		if hp == nil {
			fd.other = true
			out[id] = fd
			continue
		}
		if bp.Status != hp.Status {
			fd.status = true
		}
		if bp.BlockedBy != hp.BlockedBy {
			fd.blockedBy = true
		}
		if bp.ClaimedBy != hp.ClaimedBy || bp.Branch != hp.Branch || bp.ClaimedAt != hp.ClaimedAt {
			fd.claimFields = true
		}
		if !strEq(bp.Specs, hp.Specs) || !strEq(bp.Adrs, hp.Adrs) || !strEq(bp.DependsOn, hp.DependsOn) ||
			!strEq(bp.OwnedPaths, hp.OwnedPaths) || !strEq(bp.ForbiddenPaths, hp.ForbiddenPaths) ||
			bp.Change != hp.Change || bp.Acceptance != hp.Acceptance || bp.Tests != hp.Tests {
			fd.other = true
		}
		out[id] = fd
	}
	for id := range head.ByID {
		if _, ok := base.ByID[id]; !ok {
			out[id] = fieldDiff{other: true}
		}
	}
	return out
}

func strEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func trans(b, h string) string { return b + " -> " + h }

// checkClaimTransitions enforces claim/ rights: NOT_STARTED<->IN_PROGRESS
// claim-field edits and BLOCKED(REVERT-<sha>) -> NOT_STARTED only.
func checkClaimTransitions(base, head *Queue) []string {
	var out []string
	for id, fd := range diffPackets(base, head) {
		bp, hp := base.ByID[id], head.ByID[id]
		if bp == nil || hp == nil {
			out = append(out, id+": packet added/removed on claim/ branch")
			continue
		}
		if fd.other || fd.blockedBy {
			out = append(out, fmt.Sprintf("%s: claim/ may only touch claim fields", id))
			continue
		}
		if !fd.status {
			continue
		}
		switch {
		case bp.Status == "NOT_STARTED" && hp.Status == "IN_PROGRESS":
		case bp.Status == "IN_PROGRESS" && hp.Status == "NOT_STARTED":
		case bp.Status == "BLOCKED" && hp.Status == "NOT_STARTED" && strings.HasPrefix(bp.BlockedBy, "REVERT-"):
		default:
			out = append(out, fmt.Sprintf("%s: claim/ may not transition %s", id, trans(bp.Status, hp.Status)))
		}
	}
	return out
}

// checkRevertTransitions enforces revert/ semantics: DONE -> IN_PROGRESS or
// DONE -> BLOCKED (blocked_by REVERT-<sha>), and dependents -> BLOCKED with
// REVERT-<sha>.
func checkRevertTransitions(base, head *Queue) []string {
	var out []string
	for id, fd := range diffPackets(base, head) {
		bp, hp := base.ByID[id], head.ByID[id]
		if bp == nil || hp == nil {
			out = append(out, id+": packet added/removed on revert/ branch")
			continue
		}
		if fd.other || fd.claimFields {
			out = append(out, fmt.Sprintf("%s: revert/ may only touch status/blocked_by", id))
			continue
		}
		if !fd.status {
			continue
		}
		ok := (bp.Status == "DONE" && (hp.Status == "IN_PROGRESS" || hp.Status == "BLOCKED")) ||
			(hp.Status == "BLOCKED" && strings.HasPrefix(hp.BlockedBy, "REVERT-"))
		if ok && hp.Status == "BLOCKED" && !strings.HasPrefix(hp.BlockedBy, "REVERT-") {
			ok = false
		}
		if !ok {
			out = append(out, fmt.Sprintf("%s: revert/ may not transition %s", id, trans(bp.Status, hp.Status)))
		}
	}
	return out
}

// checkBlockPR enforces block/IMP-XXX-<n>: only packet IMP-XXX may change and
// only IN_PROGRESS -> BLOCKED, with blocked_by naming an entry appended to
// known_blockers.md in the same diff.
func checkBlockPR(base, head *Queue, bd BlockerDiff, impID string) []string {
	var out []string
	if impID == "" {
		return []string{"block/ branch carries no IMP id"}
	}
	for id, fd := range diffPackets(base, head) {
		if id != impID {
			if fd.status || fd.blockedBy || fd.claimFields || fd.other {
				out = append(out, fmt.Sprintf("block/ changed packet %s other than %s", id, impID))
			}
			continue
		}
		bp, hp := base.ByID[id], head.ByID[id]
		if bp == nil || hp == nil {
			out = append(out, id+": packet added/removed on block/ branch")
			continue
		}
		if fd.other {
			out = append(out, id+": block/ may only touch status/blocked_by/claim fields")
		}
		if fd.status && !(bp.Status == "IN_PROGRESS" && hp.Status == "BLOCKED") {
			out = append(out, fmt.Sprintf("%s: block/ requires IN_PROGRESS -> BLOCKED, got %s", id, trans(bp.Status, hp.Status)))
		}
		if !fd.status {
			out = append(out, id+": block/ must transition IN_PROGRESS -> BLOCKED")
		}
		if !blockedByRe.MatchString(hp.BlockedBy) {
			out = append(out, fmt.Sprintf("%s: blocked_by %q is not BLK-/OPS-/REVERT-", id, hp.BlockedBy))
		} else if strings.HasPrefix(hp.BlockedBy, "REVERT-") {
			out = append(out, id+": blocked_by must name an appended BLK/OPS entry, not REVERT-<sha>")
		} else if !bd.Added[hp.BlockedBy] {
			out = append(out, fmt.Sprintf("%s: blocked_by %s was not appended to known_blockers.md in this diff", id, hp.BlockedBy))
		}
	}
	if len(bd.Removed) > 0 || len(bd.Resolved) > 0 {
		out = append(out, "block/ may only append known_blockers.md entries")
	}
	return out
}

// checkOpsPR enforces ops/ semantics: OPS entries may be opened (added) or
// resolved; BLOCKED -> NOT_STARTED is allowed only for tasks whose blocked_by
// names an OPS entry touched in this diff (and listed in its blocks:).
func checkOpsPR(base, head *Queue, bd BlockerDiff) []string {
	var out []string
	for id, fd := range diffPackets(base, head) {
		bp, hp := base.ByID[id], head.ByID[id]
		if bp == nil || hp == nil {
			out = append(out, id+": packet added/removed on ops/ branch")
			continue
		}
		if fd.other {
			out = append(out, fmt.Sprintf("%s: ops/ may only touch status/blocked_by/claim fields", id))
			continue
		}
		if !fd.status {
			continue
		}
		if !(bp.Status == "BLOCKED" && hp.Status == "NOT_STARTED") {
			out = append(out, fmt.Sprintf("%s: ops/ may not transition %s", id, trans(bp.Status, hp.Status)))
			continue
		}
		if strings.HasPrefix(bp.BlockedBy, "OPS-") {
			if !(bd.Resolved[bp.BlockedBy] || bd.Added[bp.BlockedBy]) {
				out = append(out, fmt.Sprintf("%s: unblocked via %s which this diff does not open/resolve", id, bp.BlockedBy))
			}
		} else {
			out = append(out, fmt.Sprintf("%s: blocked_by %s is not an OPS entry", id, bp.BlockedBy))
		}
		if hp.BlockedBy != "" {
			out = append(out, id+": blocked_by must be cleared on BLOCKED -> NOT_STARTED")
		}
	}
	return out
}

// checkImpPR enforces imp/IMP-XXX-*: only the own packet changes; allowed
// transitions NOT_STARTED -> IN_PROGRESS (claim) and IN_PROGRESS -> DONE
// (only on imp/IMP-XXX-done).
func checkImpPR(base, head *Queue, impID, branch string) []string {
	var out []string
	if impID == "" {
		return []string{"imp/ branch carries no IMP id"}
	}
	for id, fd := range diffPackets(base, head) {
		if id != impID {
			if fd.status || fd.blockedBy || fd.claimFields || fd.other {
				out = append(out, fmt.Sprintf("imp/ changed packet %s other than %s", id, impID))
			}
			continue
		}
		bp, hp := base.ByID[id], head.ByID[id]
		if bp == nil || hp == nil {
			out = append(out, id+": packet added/removed on imp/ branch")
			continue
		}
		if fd.other {
			out = append(out, id+": imp/ may only touch status/blocked_by/claim fields")
		}
		if !fd.status {
			continue
		}
		switch {
		case bp.Status == "NOT_STARTED" && hp.Status == "IN_PROGRESS":
		case bp.Status == "IN_PROGRESS" && hp.Status == "DONE":
			if branch != "imp/"+impID+"-done" {
				out = append(out, fmt.Sprintf("%s: IN_PROGRESS -> DONE only on imp/%s-done", id, impID))
			}
		default:
			out = append(out, fmt.Sprintf("%s: imp/ may not transition %s", id, trans(bp.Status, hp.Status)))
		}
	}
	return out
}
