package taskgraph

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("_testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

func parseFixture(t *testing.T, name string) *Queue {
	t.Helper()
	q, err := ParseQueue(fixture(t, name))
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return q
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func findDetails(details []string, substrs ...string) []string {
	var out []string
	for _, d := range details {
		for _, s := range substrs {
			if strings.Contains(d, s) {
				out = append(out, d)
				break
			}
		}
	}
	return out
}

func TestDagAcyclic(t *testing.T) {
	q := parseFixture(t, "queue_cycle.md")
	got := checkDag(q)
	if len(findDetails(got, "cycle")) == 0 {
		t.Fatalf("expected cycle detail, got %v", got)
	}

	ok := parseFixture(t, "queue_reqcov.md")
	if got := checkDag(ok); len(got) != 0 {
		t.Fatalf("acyclic queue flagged: %v", got)
	}
}

func TestDanglingRefs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "docs/10_implementation/known_blockers.md", fixture(t, "blockers.md"))
	writeFile(t, root, "docs/10_implementation/existing_spec.md", "# spec\n")
	q := parseFixture(t, "queue_dangling.md")

	if got := checkDag(q); len(findDetails(got, "IMP-999")) == 0 {
		t.Errorf("expected dangling depends_on IMP-999 detail, got %v", got)
	}
	got := checkDangling(q, root)
	want := []string{"BLK-999", "missing_spec.md", "9999-no-such-adr.md"}
	for _, w := range want {
		if len(findDetails(got, w)) == 0 {
			t.Errorf("expected detail naming %s, got %v", w, got)
		}
	}
}

func TestOwnedForbiddenOverlap(t *testing.T) {
	q := parseFixture(t, "queue_overlap.md")
	got := checkOverlap(q)
	if len(findDetails(got, "IMP-001", "forbidden")) == 0 {
		t.Errorf("expected own-path vs forbidden violation, got %v", got)
	}
	if len(findDetails(got, "shared/x.go")) == 0 {
		t.Errorf("expected unordered shared path violation, got %v", got)
	}

	// Legal: same shared file but ordered by depends_on.
	ordered := parseFixture(t, "queue_reqcov.md")
	ordered.Packets[0].OwnedPaths = []string{"shared/x.go"}
	other := &Packet{ID: "IMP-002", Status: "NOT_STARTED", OwnedPaths: []string{"shared/x.go"}, DependsOn: []string{"IMP-001"}}
	ordered.Packets = append(ordered.Packets, other)
	ordered.ByID[other.ID] = other
	if got := checkOverlap(ordered); len(got) != 0 {
		t.Errorf("ordered shared path flagged: %v", got)
	}

	// Legal: Addressables append-registry grant files are unordered-shareable.
	reg := parseFixture(t, "queue_reqcov.md")
	reg.Packets[0].OwnedPaths = []string{"client/Assets/AddressableAssetsData/AddressableAssetSettings.asset"}
	b := &Packet{ID: "IMP-002", Status: "NOT_STARTED", OwnedPaths: []string{"client/Assets/AddressableAssetsData/AddressableAssetSettings.asset"}}
	reg.Packets = append(reg.Packets, b)
	reg.ByID[b.ID] = b
	if got := checkOverlap(reg); len(got) != 0 {
		t.Errorf("grant-file overlap flagged: %v", got)
	}
}

func TestTestPathsOwned(t *testing.T) {
	q := parseFixture(t, "queue_tests_unowned.md")
	got := checkTestsOwned(q)
	if len(findDetails(got, "other/pkg/x_test.go")) == 0 {
		t.Fatalf("expected unowned test path violation, got %v", got)
	}

	ok := parseFixture(t, "queue_reqcov.md") // Tests path a/x_test.go inside owned a/
	if got := checkTestsOwned(ok); len(got) != 0 {
		t.Fatalf("owned test path flagged: %v", got)
	}
}

func TestRequirementIdCoverage(t *testing.T) {
	q := parseFixture(t, "queue_reqcov.md")
	ids := map[string]string{"FOO-001": "spec_a.md", "BAR-002": "spec_a.md"}
	got := checkReqCoverage(q, ids, t.TempDir())
	if len(findDetails(got, "BAR-002")) == 0 {
		t.Errorf("expected BAR-002 uncovered detail, got %v", got)
	}
	if len(findDetails(got, "FOO-001")) != 0 {
		t.Errorf("covered FOO-001 flagged: %v", got)
	}
}

func packet(id, status string) *Packet {
	return &Packet{ID: id, Status: status}
}

func queueOf(ps ...*Packet) *Queue {
	q := &Queue{ByID: map[string]*Packet{}, Summary: map[string]string{}, PacketID: map[string]bool{}}
	for _, p := range ps {
		q.Packets = append(q.Packets, p)
		q.ByID[p.ID] = p
		q.PacketID[p.ID] = true
	}
	return q
}

func noBlockers() BlockerDiff {
	return BlockerDiff{Added: map[string]bool{}, Removed: map[string]bool{}, Resolved: map[string]bool{}}
}

func TestControlFileDiffRules(t *testing.T) {
	base := queueOf(packet("IMP-001", "NOT_STARTED"))
	head := queueOf(packet("IMP-001", "NOT_STARTED"))

	// Random branch may not touch task_queue.md.
	got := checkControlDiff(base, head, noBlockers(), "feat/random", false, nil, nil)
	if len(got) == 0 {
		t.Error("task_queue change on unprefixed branch not flagged")
	}
	// evidence/** on non-imp branch.
	got = checkControlDiff(nil, nil, noBlockers(), "feat/random", false, nil,
		[]string{"docs/10_implementation/evidence/IMP-001/manifest.json"})
	if len(got) == 0 {
		t.Error("evidence write on non-imp branch not flagged")
	}
	// evidence of a different IMP on imp/IMP-002-x.
	got = checkControlDiff(nil, nil, noBlockers(), "imp/IMP-002-x", false, nil,
		[]string{"docs/10_implementation/evidence/IMP-001/manifest.json"})
	if len(got) == 0 {
		t.Error("foreign evidence on imp/ branch not flagged")
	}
	// own evidence on imp/IMP-001-x is allowed.
	got = checkControlDiff(nil, nil, noBlockers(), "imp/IMP-001-x", false, nil,
		[]string{"docs/10_implementation/evidence/IMP-001/manifest.json"})
	if len(got) != 0 {
		t.Errorf("own evidence flagged: %v", got)
	}
	// Other top-level control .md only on spec//ops//claim/.
	got = checkControlDiff(nil, nil, noBlockers(), "imp/IMP-001-x", false,
		[]string{"docs/10_implementation/repository_layout.md"}, nil)
	if len(got) == 0 {
		t.Error("control .md change on imp/ branch not flagged")
	}
	got = checkControlDiff(base, head, noBlockers(), "spec/IMP-000-layout", false,
		[]string{"docs/10_implementation/repository_layout.md"}, nil)
	if len(got) != 0 {
		t.Errorf("spec-owner control change flagged: %v", got)
	}
}

func TestBlockPrAllowedFields(t *testing.T) {
	bp := packet("IMP-001", "IN_PROGRESS")
	hp := packet("IMP-001", "BLOCKED")
	hp.BlockedBy = "BLK-001"
	base, head := queueOf(bp), queueOf(hp)

	// Legal: IN_PROGRESS -> BLOCKED naming an appended BLK entry.
	bd := noBlockers()
	bd.Added["BLK-001"] = true
	if got := checkControlDiff(base, head, bd, "block/IMP-001-spec-gap", true, nil, nil); len(got) != 0 {
		t.Errorf("legal block/ flagged: %v", got)
	}
	// blocked_by not appended in this diff.
	if got := checkControlDiff(base, head, noBlockers(), "block/IMP-001-x", true, nil, nil); len(got) == 0 {
		t.Error("blocked_by without appended BLK entry not flagged")
	}
	// Wrong transition.
	hp2 := packet("IMP-001", "DONE")
	if got := checkControlDiff(base, queueOf(hp2), bd, "block/IMP-001-x", true, nil, nil); len(got) == 0 {
		t.Error("non BLOCKED transition on block/ not flagged")
	}
	// Other packet touched.
	other := queueOf(hp, packet("IMP-002", "DONE"))
	base2 := queueOf(bp, packet("IMP-002", "NOT_STARTED"))
	if got := checkControlDiff(base2, other, bd, "block/IMP-001-x", true, nil, nil); len(findDetails(got, "IMP-002")) == 0 {
		t.Error("foreign packet change on block/ not flagged")
	}
	// Non-status fields touched.
	hp3 := packet("IMP-001", "BLOCKED")
	hp3.BlockedBy = "BLK-001"
	hp3.OwnedPaths = []string{"evil/"}
	if got := checkControlDiff(base, queueOf(hp3), bd, "block/IMP-001-x", true, nil, nil); len(got) == 0 {
		t.Error("non-status field edit on block/ not flagged")
	}
}

func TestOpsPrAllowedFields(t *testing.T) {
	bp := packet("IMP-001", "BLOCKED")
	bp.BlockedBy = "OPS-002"
	hp := packet("IMP-001", "NOT_STARTED")
	base, head := queueOf(bp), queueOf(hp)

	// Legal: unblock via OPS entry resolved in this diff.
	bd := noBlockers()
	bd.Resolved["OPS-002"] = true
	if got := checkControlDiff(base, head, bd, "ops/OPS-002-runner", true, nil, nil); len(got) != 0 {
		t.Errorf("legal ops/ unblock flagged: %v", got)
	}
	// Unblock via OPS not touched in diff.
	if got := checkControlDiff(base, head, noBlockers(), "ops/x", true, nil, nil); len(got) == 0 {
		t.Error("unblock via untouched OPS entry not flagged")
	}
	// Unblock via BLK entry on ops/ is forbidden.
	bp2 := packet("IMP-001", "BLOCKED")
	bp2.BlockedBy = "BLK-001"
	bd2 := noBlockers()
	bd2.Resolved["BLK-001"] = true
	if got := checkControlDiff(queueOf(bp2), head, bd2, "ops/x", true, nil, nil); len(got) == 0 {
		t.Error("BLK unblock on ops/ branch not flagged")
	}
	// blocked_by left set after unblock.
	hp3 := packet("IMP-001", "NOT_STARTED")
	hp3.BlockedBy = "OPS-002"
	if got := checkControlDiff(base, queueOf(hp3), bd, "ops/x", true, nil, nil); len(got) == 0 {
		t.Error("stale blocked_by after unblock not flagged")
	}
}

func TestBlockedToNotStartedOnlyBySpecOrOps(t *testing.T) {
	bp := packet("IMP-001", "BLOCKED")
	bp.BlockedBy = "OPS-002"
	hp := packet("IMP-001", "NOT_STARTED")
	base, head := queueOf(bp), queueOf(hp)
	bd := noBlockers()
	bd.Resolved["OPS-002"] = true

	for _, br := range []string{"imp/IMP-001-x", "claim/IMP-001", "block/IMP-001-x"} {
		if got := checkControlDiff(base, head, bd, br, true, nil, nil); len(got) == 0 {
			t.Errorf("BLOCKED -> NOT_STARTED on %s not flagged", br)
		}
	}
	if got := checkControlDiff(base, head, bd, "spec/IMP-000-fix", true, nil, nil); len(got) != 0 {
		t.Errorf("spec-owner unblock flagged: %v", got)
	}
	if got := checkControlDiff(base, head, bd, "ops/OPS-002", true, nil, nil); len(got) != 0 {
		t.Errorf("ops unblock flagged: %v", got)
	}
}

func TestMetaImpliedByOwnership(t *testing.T) {
	q := parseFixture(t, "queue_meta.md")
	tracked := []string{
		"client/Assets/Foo/Bar.cs",
		"client/Assets/Foo/Bar.cs.meta",
		"client/Assets/Foo.meta",
	}
	got := checkMetaOwnership(q, tracked)
	if len(findDetails(got, "Bar.cs.meta")) == 0 {
		t.Fatalf("expected .meta ownership conflict, got %v", got)
	}
}
