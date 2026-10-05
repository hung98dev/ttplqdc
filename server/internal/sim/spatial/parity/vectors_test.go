package parity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"thinhthan/internal/config"
	"thinhthan/internal/sim/spatial/collision"
	"thinhthan/internal/sim/spatial/geometry"
)

// Golden movement vectors: a fixed obstacle course plus a table of move
// inputs with recorded ResolveMove outputs. The identical table is replayed
// by the C# GeometryMath port test — both engines must produce identical
// exported fields. Regenerate with IMP062_VECTORS=1.

const courseFile = "vector_course.geom.json"
const vectorsFile = "vectors.json"

type aabbJSON struct {
	MinX int64 `json:"min_x"`
	MinY int64 `json:"min_y"`
	MaxX int64 `json:"max_x"`
	MaxY int64 `json:"max_y"`
}

type contactJSON struct {
	SegmentID int64  `json:"segment_id"`
	Kind      string `json:"kind"`
	X         int64  `json:"x"`
	Y         int64  `json:"y"`
}

type resultJSON struct {
	Final         aabbJSON      `json:"final"`
	VxZeroed      bool          `json:"vx_zeroed"`
	VyZeroed      bool          `json:"vy_zeroed"`
	Grounded      bool          `json:"grounded"`
	GroundSegment int64         `json:"ground_segment"`
	OnOneWay      bool          `json:"on_one_way"`
	Contacts      []contactJSON `json:"contacts"`
}

type vectorCase struct {
	Name                 string     `json:"name"`
	Start                aabbJSON   `json:"start"`
	Dx                   int64      `json:"dx"`
	Dy                   int64      `json:"dy"`
	StepHeightMM         int64      `json:"step_height_mm"`
	DropIgnorePlatformID int64      `json:"drop_ignore_platform_id"`
	DropIgnoreUntilTick  uint64     `json:"drop_ignore_until_tick"`
	Tick                 uint64     `json:"tick"`
	Expected             resultJSON `json:"expected"`
}

type vectorFile struct {
	Course  string       `json:"course"`
	Vectors []vectorCase `json:"vectors"`
}

func testdataDir() string {
	return filepath.Join(env.root, "server", "internal", "sim", "spatial", "parity", "testdata")
}

func courseModel() *SceneModel {
	return &SceneModel{
		SpaceID:       "vector.course",
		SpaceKind:     "FIELD_OR_TOWN",
		LayoutProfile: "VECTOR_COURSE",
		BoundsMaxX:    51.2,
		BoundsMaxY:    28.8,
		Segments: []SceneSegment{
			{KindToken: "ground", X1: 0, Y1: 4, X2: 12, Y2: 4},
			{KindToken: "wall", X1: 0, Y1: 4, X2: 0, Y2: 14},
			{KindToken: "slope", X1: 12, Y1: 4, X2: 18, Y2: 7},
			{KindToken: "ground", X1: 18, Y1: 7, X2: 24, Y2: 7},
			{KindToken: "slope", X1: 24, Y1: 7, X2: 30, Y2: 4},
			{KindToken: "ground", X1: 30, Y1: 4, X2: 40, Y2: 4},
			{KindToken: "wall", X1: 40, Y1: 4, X2: 40, Y2: 4.3}, // 300mm mountable step
			{KindToken: "ground", X1: 40, Y1: 4.3, X2: 46, Y2: 4.3},
			{KindToken: "wall", X1: 46, Y1: 4.3, X2: 46, Y2: 5.2}, // 900mm wall stop
			{KindToken: "ground", X1: 46, Y1: 4, X2: 51.2, Y2: 4},
			{KindToken: "oneway", X1: 24, Y1: 9.5, X2: 30, Y2: 9.5},
			{KindToken: "ceiling", X1: 14, Y1: 10.5, X2: 20, Y2: 10.5},
			{KindToken: "wall", X1: 51.2, Y1: 4, X2: 51.2, Y2: 14},
		},
		Regions: []SceneRegion{{MinX: 0, MinY: 0, MaxX: 51.2, MaxY: 28.8}},
		Anchors: []SceneAnchor{
			{ID: "vec.a", X: 6, Y: 4},
			{ID: "vec.b", X: 21, Y: 7},
		},
	}
}

func courseRecord() config.SpaceRecord {
	var rec config.SpaceRecord
	rec.SpaceID = "vector.course"
	rec.SpaceKind = "FIELD_OR_TOWN"
	rec.LayoutProfile = "VECTOR_COURSE"
	rec.BoundsMM.MaxX = 51200
	rec.BoundsMM.MaxY = 28800
	rec.Anchors = []string{"vec.a", "vec.b"}
	return rec
}

func charBoxAt(feetX, feetY int64) aabbJSON {
	return aabbJSON{MinX: feetX, MinY: feetY, MaxX: feetX + 800, MaxY: feetY + 1600}
}

// vectorInputs is the authored input table; expected outputs are generated.
func vectorInputs() []vectorCase {
	return []vectorCase{
		{Name: "walk_on_flat", Start: charBoxAt(4000, 4000), Dx: 2000, Dy: 0, StepHeightMM: 300},
		{Name: "walk_up_slope", Start: charBoxAt(10000, 4000), Dx: 4000, Dy: 0, StepHeightMM: 300},
		{Name: "mount_300mm_step", Start: charBoxAt(38800, 4000), Dx: 1500, Dy: 0, StepHeightMM: 300},
		{Name: "blocked_900mm_wall", Start: charBoxAt(44400, 4300), Dx: 2000, Dy: 0, StepHeightMM: 300},
		{Name: "fall_to_floor", Start: charBoxAt(33000, 9000), Dx: 0, Dy: -4000, StepHeightMM: 300},
		{Name: "drop_through_oneway", Start: charBoxAt(25500, 9500), Dx: 0, Dy: -3000, StepHeightMM: 300,
			DropIgnorePlatformID: 7, DropIgnoreUntilTick: 100, Tick: 1},
		{Name: "rise_hits_ceiling", Start: charBoxAt(15000, 7600), Dx: 0, Dy: 1500, StepHeightMM: 300},
		{Name: "rise_passes_oneway", Start: charBoxAt(25000, 6500), Dx: 0, Dy: 2000, StepHeightMM: 300},
		{Name: "land_on_oneway", Start: charBoxAt(25000, 10100), Dx: 0, Dy: -2000, StepHeightMM: 300},
		{Name: "idle_stays_grounded", Start: charBoxAt(4000, 4000), Dx: 0, Dy: 0, StepHeightMM: 300},
	}
}

func runVectors(w *collision.World, in []vectorCase) []vectorCase {
	out := make([]vectorCase, 0, len(in))
	for _, v := range in {
		box := collision.AABB{MinX: v.Start.MinX, MinY: v.Start.MinY, MaxX: v.Start.MaxX, MaxY: v.Start.MaxY}
		res := w.ResolveMove(box, v.Dx, v.Dy, collision.MoveOpts{
			StepHeightMM:         v.StepHeightMM,
			DropIgnorePlatformID: v.DropIgnorePlatformID,
			DropIgnoreUntilTick:  v.DropIgnoreUntilTick,
			Tick:                 v.Tick,
		})
		v.Expected = resultJSON{
			Final:         aabbJSON{res.Final.MinX, res.Final.MinY, res.Final.MaxX, res.Final.MaxY},
			VxZeroed:      res.VxZeroed,
			VyZeroed:      res.VyZeroed,
			Grounded:      res.Grounded,
			GroundSegment: res.GroundSegment,
			OnOneWay:      res.OnOneWay,
		}
		for _, c := range res.Contacts {
			v.Expected.Contacts = append(v.Expected.Contacts, contactJSON{
				SegmentID: c.SegmentID, Kind: c.Kind.String(), X: c.X, Y: c.Y,
			})
		}
		if v.Expected.Contacts == nil {
			v.Expected.Contacts = []contactJSON{}
		}
		out = append(out, v)
	}
	return out
}

func loadCourseWorld(t *testing.T) *collision.World {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataDir(), courseFile))
	if err != nil {
		t.Fatalf("read course: %v", err)
	}
	g, err := geometry.Parse(data, courseRecord())
	if err != nil {
		t.Fatalf("parse course: %v", err)
	}
	return collision.NewWorld(g)
}

// TestGoldenVectors replays every committed vector through ResolveMove and
// compares the full exported result — the identical table is replayed by the
// C# GeometryMath port, so both engines agree byte-for-byte on these inputs.
func TestGoldenVectors(t *testing.T) {
	w := loadCourseWorld(t)
	data, err := os.ReadFile(filepath.Join(testdataDir(), vectorsFile))
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var vf vectorFile
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&vf); err != nil {
		t.Fatalf("decode vectors: %v", err)
	}
	got := runVectors(w, vf.Vectors)
	for i, v := range got {
		want := vf.Vectors[i].Expected
		gj := v.Expected
		if gj.Final != want.Final || gj.VxZeroed != want.VxZeroed || gj.VyZeroed != want.VyZeroed ||
			gj.Grounded != want.Grounded || gj.GroundSegment != want.GroundSegment || gj.OnOneWay != want.OnOneWay {
			t.Fatalf("vector %s: got %+v want %+v", v.Name, gj, want)
		}
		if len(gj.Contacts) != len(want.Contacts) {
			t.Fatalf("vector %s: %d contacts want %d", v.Name, len(gj.Contacts), len(want.Contacts))
		}
		for j := range want.Contacts {
			if gj.Contacts[j] != want.Contacts[j] {
				t.Fatalf("vector %s contact %d: got %+v want %+v", v.Name, j, gj.Contacts[j], want.Contacts[j])
			}
		}
	}
}

// TestGenerateVectorFixtures regenerates the committed course + vectors.
// Run: IMP062_VECTORS=1 go test ./internal/sim/spatial/parity -run TestGenerateVectorFixtures
func TestGenerateVectorFixtures(t *testing.T) {
	if os.Getenv("IMP062_VECTORS") == "" {
		t.Skip("set IMP062_VECTORS=1 to regenerate vector fixtures")
	}
	canon, err := CanonicalJSON(courseModel())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testdataDir(), courseFile), canon, 0o644); err != nil {
		t.Fatal(err)
	}
	w := loadCourseWorld(t)
	vf := vectorFile{Course: courseFile, Vectors: runVectors(w, vectorInputs())}
	buf, err := json.MarshalIndent(vf, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(testdataDir(), vectorsFile), append(buf, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	fmt.Println("wrote", courseFile, "and", vectorsFile)
}
