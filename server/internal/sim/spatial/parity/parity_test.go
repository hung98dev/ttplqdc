package parity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"thinhthan/internal/config"
	"thinhthan/internal/sim/spatial/geometry"
)

// testEnv resolves repo paths and builds the compiled payload once.
type testEnv struct {
	root     string // repo root
	recs     map[string]config.SpaceRecord
	spaceIDs []string
}

var env testEnv

func TestMain(m *testing.M) {
	// server/internal/sim/spatial/parity -> repo root is ../../../../..
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", "..", "..", ".."))
	tmp, err := os.MkdirTemp("", "imp062")
	if err != nil {
		panic(err)
	}
	pay := filepath.Join(tmp, "payload.json")
	run := exec.Command("go", "run", "./cmd/compiler", "-payload", pay)
	run.Dir = filepath.Join(root, "server")
	if out, err := run.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("compiler payload: %v\n%s", err, out))
	}
	data, err := os.ReadFile(pay)
	if err != nil {
		panic(err)
	}
	p, err := LoadPayload(data)
	if err != nil {
		panic(err)
	}
	env = testEnv{root: root, recs: p.Records()}
	for id := range env.recs {
		env.spaceIDs = append(env.spaceIDs, id)
	}
	sort.Strings(env.spaceIDs)
	os.Exit(m.Run())
}

func scenePath(id string) string {
	return filepath.Join(env.root, "client", "Assets", "Scenes", "Collision", id+".unity")
}

func geomPath(id string) string {
	return filepath.Join(env.root, "server", "internal", "sim", "spatial", "maps", id+".geom.json")
}

func anchorsMap(g *geometry.Geometry) map[string][2]int64 {
	m := map[string][2]int64{}
	for _, a := range g.Anchors {
		m[a.ID] = [2]int64{a.X, a.Y}
	}
	return m
}

// derive reads the authored scene and returns its canonical bytes.
func derive(t *testing.T, id string) []byte {
	t.Helper()
	data, err := os.ReadFile(scenePath(id))
	if err != nil {
		t.Fatalf("%s: read scene: %v", id, err)
	}
	m, err := ParseScene(data)
	if err != nil {
		t.Fatalf("%s: parse scene: %v", id, err)
	}
	canon, err := CanonicalJSON(m)
	if err != nil {
		t.Fatalf("%s: canonical: %v", id, err)
	}
	return canon
}

// TestGeometryParity is the core contract: every authored collision scene
// derives to byte-identical canonical JSON as the committed server map, and
// the committed file parses + validates against the registered SpaceRecord
// and §6 topology rules.
func TestGeometryParity(t *testing.T) {
	for _, id := range env.spaceIDs {
		t.Run(id, func(t *testing.T) {
			rec := env.recs[id]
			want, err := os.ReadFile(geomPath(id))
			if err != nil {
				t.Fatalf("committed map missing: %v", err)
			}
			got := derive(t, id)
			if !bytes.Equal(got, want) {
				t.Fatalf("derived canonical bytes differ from committed map")
			}
			g, err := geometry.Parse(want, rec)
			if err != nil {
				t.Fatalf("parse committed: %v", err)
			}
			if v := DeadEndViolations(g, anchorsMap(g), rec.BoundsMM.MaxX, rec.BoundsMM.MaxY); len(v) > 0 {
				t.Fatalf("topology dead ends: %v", v)
			}
		})
	}
}

// TestAllPlayableSpaceBoundsProfiles asserts the committed set covers exactly
// the payload's playable spaces and that bounds_mm equals the SpaceRecord
// (never a scene-default 1280x720 region or similar leak).
func TestAllPlayableSpaceBoundsProfiles(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(env.root, "server", "internal", "sim", "spatial", "maps", "*.geom.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no committed maps: %v", err)
	}
	gotIDs := map[string]bool{}
	for _, f := range files {
		gotIDs[strings.TrimSuffix(filepath.Base(f), ".geom.json")] = true
	}
	if len(gotIDs) != len(env.spaceIDs) {
		t.Fatalf("map set %d files != %d payload spaces", len(gotIDs), len(env.spaceIDs))
	}
	for _, id := range env.spaceIDs {
		if !gotIDs[id] {
			t.Fatalf("missing committed map for %s", id)
		}
	}
	for id := range gotIDs {
		rec, ok := env.recs[id]
		if !ok {
			t.Fatalf("committed map %s is not a registered space", id)
		}
		data, err := os.ReadFile(geomPath(id))
		if err != nil {
			t.Fatal(err)
		}
		g, err := geometry.Parse(data, rec)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if g.BoundsMM != rec.BoundsMM {
			t.Fatalf("%s: bounds %v != record %v", id, g.BoundsMM, rec.BoundsMM)
		}
		// regression: 1280x720 units leaked as millimeters is a known failure mode.
		if rec.BoundsMM.MaxX == 1280000 && rec.BoundsMM.MaxY == 720000 {
			t.Fatalf("%s: bounds look like pixel dimensions, not mm", id)
		}
		if g.Kind.String() != rec.SpaceKind || g.LayoutProfile != rec.LayoutProfile {
			t.Fatalf("%s: kind/profile mismatch vs record", id)
		}
	}
}

// TestCompetitiveMirrorParity checks that the 3 competitive spaces are
// mirror-symmetric about x = bounds.MaxX/2 within 1mm (contract §6.3).
func TestCompetitiveMirrorParity(t *testing.T) {
	for _, id := range env.spaceIDs {
		rec := env.recs[id]
		if rec.SpaceKind != "PVP" && rec.SpaceKind != "GUILD_WAR" {
			continue
		}
		data, err := os.ReadFile(geomPath(id))
		if err != nil {
			t.Fatal(err)
		}
		g, err := geometry.Parse(data, rec)
		if err != nil {
			t.Fatal(err)
		}
		mid := rec.BoundsMM.MaxX
		type seg struct {
			kind           geometry.SegmentKind
			x1, y1, x2, y2 int64
		}
		segSet := map[seg]bool{}
		for _, s := range g.Segments {
			segSet[seg{s.Kind, s.X1, s.Y1, s.X2, s.Y2}] = true
		}
		for _, s := range g.Segments {
			// mirror maps (x,y) -> (mid-x, y): canonical seg (x1,y1,x2,y2)
			// mirrors to (mid-x2, y2, mid-x1, y1), then normalized like the
			// canonical writer (x1<x2; vertical y1<y2)
			m := seg{s.Kind, mid - s.X2, s.Y2, mid - s.X1, s.Y1}
			if m.x1 > m.x2 || (m.x1 == m.x2 && m.y1 > m.y2) {
				m.x1, m.x2 = m.x2, m.x1
				m.y1, m.y2 = m.y2, m.y1
			}
			if segSet[m] {
				continue
			}
			// tolerate +/-1mm quantization drift
			found := false
			for cand := range segSet {
				if cand.kind != s.Kind {
					continue
				}
				if abs(cand.x1-m.x1) <= 1 && abs(cand.y1-m.y1) <= 1 &&
					abs(cand.x2-m.x2) <= 1 && abs(cand.y2-m.y2) <= 1 {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("%s: segment %+v has no mirror image", id, s)
			}
		}
		anchorAt := map[[2]int64]string{}
		for _, a := range g.Anchors {
			anchorAt[[2]int64{a.X, a.Y}] = a.ID
		}
		for _, a := range g.Anchors {
			mx := mid - a.X
			ok := false
			for pos := range anchorAt {
				if abs(pos[0]-mx) <= 1 && abs(pos[1]-a.Y) <= 1 {
					ok = true
					break
				}
			}
			if !ok {
				t.Fatalf("%s: anchor %s at (%d,%d) has no mirrored counterpart", id, a.ID, a.X, a.Y)
			}
		}
	}
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// TestExportDeterministic re-derives each scene twice from a fresh parse and
// requires byte-identical canonical output.
func TestExportDeterministic(t *testing.T) {
	for _, id := range env.spaceIDs {
		a := derive(t, id)
		b := derive(t, id)
		if !bytes.Equal(a, b) {
			t.Fatalf("%s: re-export is not deterministic", id)
		}
	}
}

// TestSchemaV1IntegerMm validates the on-disk schema: ordered top-level keys,
// integer millimeter literals only (no fractions/exponents in coordinates),
// and schema_version 1.
func TestSchemaV1IntegerMm(t *testing.T) {
	wantOrder := []string{
		"schema_version", "space_id", "space_kind", "layout_profile",
		"content_revision", "bounds_mm", "segments", "camera_regions", "anchors",
	}
	for _, id := range env.spaceIDs {
		data, err := os.ReadFile(geomPath(id))
		if err != nil {
			t.Fatal(err)
		}
		// key order: each expected key's first occurrence must be ascending
		last := -1
		for _, k := range wantOrder {
			i := bytes.Index(data, []byte("\""+k+"\""))
			if i < 0 {
				t.Fatalf("%s: missing key %s", id, k)
			}
			if i <= last {
				t.Fatalf("%s: key %s out of order", id, k)
			}
			last = i
		}
		if !bytes.HasSuffix(data, []byte("}\n")) || bytes.Contains(data, []byte("\r")) {
			t.Fatalf("%s: must end with }\\n and contain no CR", id)
		}
		// integer-only: decode numbers and reject non-integer literals
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		var v any
		if err := dec.Decode(&v); err != nil {
			t.Fatal(err)
		}
		var check func(path string, x any)
		check = func(path string, x any) {
			switch e := x.(type) {
			case map[string]any:
				for k, vv := range e {
					check(path+"."+k, vv)
				}
			case []any:
				for i, vv := range e {
					check(fmt.Sprintf("%s[%d]", path, i), vv)
				}
			case json.Number:
				if strings.ContainsAny(e.String(), ".eE") {
					t.Fatalf("%s: %s is not an integer mm literal: %s", id, path, e)
				}
			}
		}
		check("$", v)
	}
}

// TestCameraRegionRules enforces §6.2: >=1 region, min dims, in-bounds, and
// union coverage of every walkable segment's x-span.
func TestCameraRegionRules(t *testing.T) {
	for _, id := range env.spaceIDs {
		rec := env.recs[id]
		data, err := os.ReadFile(geomPath(id))
		if err != nil {
			t.Fatal(err)
		}
		g, err := geometry.Parse(data, rec)
		if err != nil {
			t.Fatal(err)
		}
		if len(g.CameraRegions) == 0 {
			t.Fatalf("%s: no camera regions", id)
		}
		for _, r := range g.CameraRegions {
			if r.MaxX-r.MinX < geometry.MinCameraRegionXMM || r.MaxY-r.MinY < geometry.MinCameraRegionYMM {
				t.Fatalf("%s: region %+v below min dims", id, r)
			}
			if r.MinX < 0 || r.MaxX > rec.BoundsMM.MaxX || r.MinY < 0 || r.MaxY > rec.BoundsMM.MaxY {
				t.Fatalf("%s: region %+v outside bounds", id, r)
			}
		}
		// interval-union coverage of each walkable segment's x span
		for _, s := range g.Segments {
			if !s.Kind.IsWalkable() {
				continue
			}
			if !coveredByRegions(g, s) {
				t.Fatalf("%s: walkable segment %d x-span not covered by regions", id, s.ID)
			}
		}
	}
}

func coveredByRegions(g *geometry.Geometry, s geometry.Segment) bool {
	// remainder of s's [X1,X2] after subtracting region intervals
	rem := [][2]int64{{s.X1, s.X2}}
	for _, r := range g.CameraRegions {
		var next [][2]int64
		for _, iv := range rem {
			if r.MinX > iv[0] {
				next = append(next, [2]int64{iv[0], min(iv[1], r.MinX)})
			}
			if r.MaxX < iv[1] {
				next = append(next, [2]int64{max(iv[0], r.MaxX), iv[1]})
			}
		}
		rem = next
	}
	return len(rem) == 0
}

// TestAnchorSetMatchesCatalog: the committed anchor set equals exactly the
// spec-derived required set (catalog-derived spawn/wave/boss/quest anchors).
func TestAnchorSetMatchesCatalog(t *testing.T) {
	for _, id := range env.spaceIDs {
		rec := env.recs[id]
		data, err := os.ReadFile(geomPath(id))
		if err != nil {
			t.Fatal(err)
		}
		g, err := geometry.Parse(data, rec)
		if err != nil {
			t.Fatal(err)
		}
		if len(rec.Anchors) == 0 {
			t.Fatalf("%s: derived required anchor set is empty", id)
		}
		want := map[string]bool{}
		for _, a := range rec.Anchors {
			want[a] = true
		}
		got := map[string]bool{}
		for _, a := range g.Anchors {
			got[a.ID] = true
			if !want[a.ID] {
				t.Fatalf("%s: anchor %s not in required set", id, a.ID)
			}
		}
		for w := range want {
			if !got[w] {
				t.Fatalf("%s: required anchor %s missing from map", id, w)
			}
		}
	}
}

// TestGenerateCommittedMaps regenerates maps/*.geom.json from the authored
// scenes. Run: IMP062_GEN=1 go test ./internal/sim/spatial/parity -run TestGenerateCommittedMaps
func TestGenerateCommittedMaps(t *testing.T) {
	if os.Getenv("IMP062_GEN") == "" {
		t.Skip("set IMP062_GEN=1 to regenerate committed maps")
	}
	for _, id := range env.spaceIDs {
		canon := derive(t, id)
		if err := os.WriteFile(geomPath(id), canon, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("regenerated %d maps", len(env.spaceIDs))
}
