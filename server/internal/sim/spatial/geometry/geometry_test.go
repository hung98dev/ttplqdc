package geometry

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"thinhthan/internal/config"
)

func fieldFlatRecord() config.SpaceRecord {
	return config.SpaceRecord{
		SpaceID:       "field_flat_01",
		SpaceKind:     "FIELD_OR_TOWN",
		LayoutProfile: "flat_field",
		BoundsMM:      struct{ MaxX, MaxY int64 }{51200, 28800},
		Anchors:       []string{"spawn_a", "spawn_b"},
	}
}

func slopeStepsRecord() config.SpaceRecord {
	return config.SpaceRecord{
		SpaceID:       "slope_town_01",
		SpaceKind:     "FIELD_OR_TOWN",
		LayoutProfile: "slope_steps",
		BoundsMM:      struct{ MaxX, MaxY int64 }{51200, 28800},
		Anchors:       []string{"spawn_low", "spawn_mid", "spawn_top"},
	}
}

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

// TestParseValidGeometry parses the two committed fixtures end to end.
func TestParseValidGeometry(t *testing.T) {
	g, err := Parse(loadFixture(t, "field_flat.json"), fieldFlatRecord())
	if err != nil {
		t.Fatalf("Parse field_flat: %v", err)
	}
	if g.SpaceID != "field_flat_01" || g.Kind != SpaceKindFieldOrTown {
		t.Fatalf("identity mismatch: %+v", g)
	}
	if len(g.Segments) != 5 || len(g.CameraRegions) != 1 || len(g.Anchors) != 2 {
		t.Fatalf("counts: segs=%d regions=%d anchors=%d",
			len(g.Segments), len(g.CameraRegions), len(g.Anchors))
	}
	if g.Segments[4].Kind != OneWayPlatform {
		t.Fatalf("kind map: seg4=%v", g.Segments[4].Kind)
	}
	if g.BoundsMM.MaxX != 51200 || g.BoundsMM.MaxY != 28800 {
		t.Fatalf("bounds: %+v", g.BoundsMM)
	}

	g2, err := Parse(loadFixture(t, "slope_steps.json"), slopeStepsRecord())
	if err != nil {
		t.Fatalf("Parse slope_steps: %v", err)
	}
	if g2.Segments[1].Kind != Slope {
		t.Fatalf("slope kind: %v", g2.Segments[1].Kind)
	}
	// Anchor on the slope sits at the quantized surface height.
	if SurfaceHeight(g2.Segments[1], 13000) != 5500 {
		t.Fatalf("anchor surface: %d", SurfaceHeight(g2.Segments[1], 13000))
	}
}

// TestRejectMalformedGeometry covers syntactic failures and schema violations;
// every failure is a path-tagged *Error wrapping the right sentinel.
func TestRejectMalformedGeometry(t *testing.T) {
	valid := string(loadFixture(t, "field_flat.json"))
	rec := fieldFlatRecord()

	cases := []struct {
		name     string
		doc      string
		sentinel error
		pathPart string
	}{
		{"truncated", valid[:len(valid)/2], ErrMalformed, "$"},
		{"not json", `{"schema_version": 1,`, ErrMalformed, "$"},
		{"unknown top key", strings.Replace(valid, `"space_id"`, `"bogus"`, 1), ErrMalformed, "bogus"},
		{"wrong type", strings.Replace(valid, `"max_x": 51200`, `"max_x": "51200"`, 1), ErrMalformed, "bounds_mm.max_x"},
		{"missing key", strings.Replace(valid, `"layout_profile": "flat_field",`, ``, 1), ErrMalformed, "layout_profile"},
		{"schema version", strings.Replace(valid, `"schema_version": 1`, `"schema_version": 2`, 1), ErrSchema, "schema_version"},
		{"bad kind", strings.Replace(valid, `"SOLID_GROUND"`, `"FLOOR"`, 1), ErrMalformed, "kind"},
		{"short revision", strings.Replace(valid, `"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`, `"abcd"`, 1), ErrSchema, "content_revision"},
		{"space mismatch", strings.Replace(valid, `"field_flat_01"`, `"other_space"`, 1), ErrSchema, "space_id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.doc), rec)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !errors.Is(err, tc.sentinel) {
				t.Fatalf("sentinel: %v not %v", err, tc.sentinel)
			}
			var pe *Error
			if !errors.As(err, &pe) {
				t.Fatalf("not a path error: %T", err)
			}
			if !strings.Contains(pe.Path, tc.pathPart) {
				t.Fatalf("path %q missing %q", pe.Path, tc.pathPart)
			}
		})
	}

	// Schema-level rejections on otherwise-well-formed documents.
	schemaCases := []struct {
		name   string
		mutate func(*Geometry)
		path   string
	}{
		{"unsorted segments", func(g *Geometry) {
			g.Segments[0], g.Segments[1] = g.Segments[1], g.Segments[0]
		}, "segments"},
		{"segment out of bounds", func(g *Geometry) {
			g.Segments[0].X2 = g.BoundsMM.MaxX + 1
		}, "segments[0]"},
		{"duplicate segment", func(g *Geometry) {
			g.Segments[1] = g.Segments[0]
			g.Segments[1].ID = g.Segments[0].ID + 1 // keep sorted ids; identical geometry
		}, "segments[1]"},
		{"no camera regions", func(g *Geometry) {
			g.CameraRegions = nil
		}, "camera_regions"},
		{"region under min size", func(g *Geometry) {
			g.CameraRegions[0].MaxX = g.CameraRegions[0].MinX + MinCameraRegionXMM - 1
		}, "camera_regions"},
		{"walkable not covered", func(g *Geometry) {
			g.CameraRegions[0].MinY = 5000 // floors sit at y=4000
		}, "segments"},
		{"anchor missing from record", func(g *Geometry) {
			g.Anchors[0].ID = "bogus"
		}, "anchors"},
		{"anchor off floor", func(g *Geometry) {
			g.Anchors[0].Y += 5
		}, "anchors"},
	}
	for _, tc := range schemaCases {
		t.Run(tc.name, func(t *testing.T) {
			g, err := Parse(loadFixture(t, "field_flat.json"), rec)
			if err != nil {
				t.Fatalf("setup parse: %v", err)
			}
			tc.mutate(g)
			err = Validate(g, &rec)
			if err == nil {
				t.Fatalf("expected schema error")
			}
			if !errors.Is(err, ErrSchema) {
				t.Fatalf("not ErrSchema: %v", err)
			}
			var pe *Error
			if errors.As(err, &pe) && !strings.Contains(pe.Path, tc.path) {
				t.Fatalf("path %q missing %q", pe.Path, tc.path)
			}
		})
	}
}

// TestQuantizationEpsilon pins the contract's exact rounding (§2.2) and the
// quantized surface formula (§2.3).
func TestQuantizationEpsilon(t *testing.T) {
	cases := []struct{ n, d, want int64 }{
		{1, 2, 1}, {-1, 2, -1}, {3, 2, 2}, {-3, 2, -2},
		{-1, 3, 0}, {5, 4, 1}, {-5, 4, -1}, {7, 4, 2}, {-7, 4, -2},
		{0, 7, 0}, {10, 3, 3}, {-10, 3, -3}, {2, 3, 1}, {-2, 3, -1},
	}
	for _, c := range cases {
		if got := RoundDiv(c.n, c.d); got != c.want {
			t.Fatalf("RoundDiv(%d,%d)=%d want %d", c.n, c.d, got, c.want)
		}
	}
	// Quantized surface: y1 + RoundDiv((x-x1)*(y2-y1), x2-x1).
	s := Segment{ID: 1, Kind: Slope, X1: 0, Y1: 1000, X2: 3, Y2: 2000}
	if got := SurfaceHeight(s, 1); got != 1333 {
		t.Fatalf("SurfaceHeight x=1: %d want 1333", got)
	}
	if got := SurfaceHeight(s, 2); got != 1667 {
		t.Fatalf("SurfaceHeight x=2: %d want 1667", got)
	}
	// Down-slope symmetric.
	d := Segment{ID: 2, Kind: Slope, X1: 0, Y1: 2000, X2: 3, Y2: 1000}
	if got := SurfaceHeight(d, 1); got != 1667 {
		t.Fatalf("down-slope x=1: %d want 1667", got)
	}
}

// TestRejectNonIntegerCoordinates rejects float literals anywhere the schema
// requires millimeter integers.
func TestRejectNonIntegerCoordinates(t *testing.T) {
	rec := fieldFlatRecord()
	cases := []struct{ name, doc string }{
		{"segment coord", `{"schema_version":1,"space_id":"field_flat_01","space_kind":"FIELD_OR_TOWN","layout_profile":"flat_field","content_revision":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","bounds_mm":{"max_x":51200,"max_y":28800},"segments":[{"id":1,"kind":"SOLID_GROUND","x1":0.5,"y1":4000,"x2":51200,"y2":4000}],"camera_regions":[{"id":1,"min_x":0,"min_y":0,"max_x":51200,"max_y":28800}],"anchors":[{"id":"spawn_a","x":5000,"y":4000},{"id":"spawn_b","x":12000,"y":8000}]}`},
		{"bounds float", `{"schema_version":1,"space_id":"field_flat_01","space_kind":"FIELD_OR_TOWN","layout_profile":"flat_field","content_revision":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","bounds_mm":{"max_x":51200.0,"max_y":28800},"segments":[],"camera_regions":[],"anchors":[]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.doc), rec)
			if err == nil || !errors.Is(err, ErrMalformed) {
				t.Fatalf("expected malformed, got %v", err)
			}
		})
	}
}

// TestSegmentKindSlopeRules pins the rational slope classification at the
// 5° and 45° boundaries (contract §2.4 / §7).
func TestSegmentKindSlopeRules(t *testing.T) {
	// 5° boundary: dy*1e6 == dx*87489 is still <= 5°.
	// dy=87489, dx=1000000 gives exactly tan(5°).
	seg := func(kind SegmentKind, x1, y1, x2, y2 int64) Segment {
		return Segment{ID: 1, Kind: kind, X1: x1, Y1: y1, X2: x2, Y2: y2}
	}
	must := func(name string, s Segment, ok bool) {
		t.Helper()
		err := checkSlopeRule("s", s)
		if ok && err != nil {
			t.Fatalf("%s: unexpected %v", name, err)
		}
		if !ok && err == nil {
			t.Fatalf("%s: expected rejection", name)
		}
	}

	must("flat ground", seg(SolidGround, 0, 100, 1000, 100), true)
	must("at 5deg ground", seg(SolidGround, 0, 0, 1000000, 87489), true)
	must("at 5deg slope rejected", seg(Slope, 0, 0, 1000000, 87489), false)
	must("over 5deg slope", seg(Slope, 0, 0, 1000000, 87490), true)
	must("over 5deg ground rejected", seg(SolidGround, 0, 0, 1000000, 87490), false)
	must("at 45deg slope", seg(Slope, 0, 0, 1000, 1000), true)
	must("at 45deg wall rejected", seg(Wall, 0, 0, 1000, 1000), false)
	must("over 45deg wall", seg(Wall, 0, 0, 1000, 1001), true)
	must("over 45deg slope rejected", seg(Slope, 0, 0, 1000, 1001), false)
	must("ceiling 45deg", seg(Ceiling, 0, 0, 1000, 1000), true)
	must("ceiling over 45 rejected", seg(Ceiling, 0, 0, 1000, 1001), false)
	must("one-way flat", seg(OneWayPlatform, 0, 500, 5000, 500), true)
	must("one-way over 5 rejected", seg(OneWayPlatform, 0, 0, 1000000, 87490), false)
	must("vertical wall", seg(Wall, 300, 0, 300, 900), true)
	must("vertical wall inverted", seg(Wall, 300, 900, 300, 0), false)
	must("vertical non-wall", seg(SolidGround, 300, 0, 300, 900), false)
	must("x1>x2", seg(SolidGround, 900, 0, 300, 0), false)
}
