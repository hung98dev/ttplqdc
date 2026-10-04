package aoi

import (
	"slices"
	"testing"
)

// testSource answers Info lookups from a static table.
type testSource struct {
	infos map[uint64]EntityInfo
}

func (s *testSource) Info(_, id uint64) (EntityInfo, bool) {
	info, ok := s.infos[id]
	return info, ok
}

// upsert moves id on the grid and in the info table together.
func upsert(g *Grid, src *testSource, id uint64, x, y int32) {
	g.Upsert(id, x, y)
	src.infos[id] = EntityInfo{X: x, Y: y}
}

func TestAoiEnterLeave(t *testing.T) {
	g := NewGrid(LeaveMM)
	v := &Viewer{EntityID: 1}
	src := &testSource{infos: map[uint64]EntityInfo{}}
	s := &Scratch{}

	// Entity enters inside EnterMM: spawns into the visible set.
	upsert(g, src, 2, int32(EnterMM-1000), 0)
	set := v.Recompute(g, src, 0, 0, s)
	if len(set.Enter) != 1 || set.Enter[0] != 2 {
		t.Fatalf("Enter = %v, want [2]", set.Enter)
	}
	if len(set.Visible) != 1 || set.Visible[0] != 2 || len(set.Leave) != 0 {
		t.Fatalf("Visible=%v Leave=%v", set.Visible, set.Leave)
	}

	// Hysteresis band: a visible entity between EnterMM and LeaveMM stays.
	upsert(g, src, 2, int32(EnterMM+1000), 0)
	set = v.Recompute(g, src, 0, 0, s)
	if len(set.Visible) != 1 || len(set.Enter)+len(set.Leave)+len(set.Shed) != 0 {
		t.Fatalf("hysteresis retained: Visible=%v Enter=%v Leave=%v Shed=%v",
			set.Visible, set.Enter, set.Leave, set.Shed)
	}

	// Beyond LeaveMM: the entity leaves.
	upsert(g, src, 2, int32(LeaveMM+1000), 0)
	set = v.Recompute(g, src, 0, 0, s)
	if len(set.Leave) != 1 || set.Leave[0] != 2 || len(set.Visible) != 0 {
		t.Fatalf("Leave=%v Visible=%v, want leave [2]", set.Leave, set.Visible)
	}

	// Back inside the hysteresis band but outside EnterMM: stays out.
	upsert(g, src, 2, int32(LeaveMM-1000), 0)
	set = v.Recompute(g, src, 0, 0, s)
	if len(set.Enter) != 0 || len(set.Visible) != 0 {
		t.Fatalf("hysteresis blocked re-enter: Enter=%v Visible=%v", set.Enter, set.Visible)
	}

	// Back inside EnterMM: re-enters.
	upsert(g, src, 2, int32(EnterMM-1000), 0)
	set = v.Recompute(g, src, 0, 0, s)
	if len(set.Enter) != 1 || set.Enter[0] != 2 || len(set.Visible) != 1 {
		t.Fatalf("re-enter failed: Enter=%v Visible=%v", set.Enter, set.Visible)
	}

	// Removal from the grid despawns the entity.
	g.Remove(2)
	set = v.Recompute(g, src, 0, 0, s)
	if len(set.Leave) != 1 || set.Leave[0] != 2 || len(set.Visible) != 0 {
		t.Fatalf("removal: Leave=%v Visible=%v", set.Leave, set.Visible)
	}
}

func TestInterestSetBoundaries(t *testing.T) {
	g := NewGrid(LeaveMM)
	v := &Viewer{EntityID: 1}
	src := &testSource{infos: map[uint64]EntityInfo{}}
	s := &Scratch{}

	// Self rides the dedicated self fields and is never a candidate.
	g.Upsert(1, 0, 0)
	src.infos[1] = EntityInfo{X: 0}

	// 45 plain entities inside the enter radius: the cap keeps the 40
	// nearest, the 5 farthest carry cap-shed marks.
	const others = 45
	for i := 0; i < others; i++ {
		id := uint64(100 + i)
		upsert(g, src, id, int32(1000+i*700), 0)
	}
	set := v.Recompute(g, src, 0, 0, s)
	if len(set.Visible) != MaxVisible {
		t.Fatalf("Visible=%d, want cap %d", len(set.Visible), MaxVisible)
	}
	if len(set.Enter) != MaxVisible {
		t.Fatalf("Enter=%d, want %d", len(set.Enter), MaxVisible)
	}
	if slices.Contains(set.Visible, 1) {
		t.Fatal("self inside the replicated entity set")
	}
	if slices.Contains(set.Visible, 144) {
		t.Fatal("farthest entity admitted over nearer ones")
	}

	// Three nearer entities push the three farthest visible ones out:
	// they transition visible -> shed this tick.
	wasVisible := slices.Clone(set.Visible)
	for i, id := range []uint64{300, 301, 302} {
		upsert(g, src, id, int32(500+i*100), 0)
	}
	set = v.Recompute(g, src, 0, 0, s)
	if len(set.Shed) != 3 {
		t.Fatalf("Shed=%v, want 3 displaced entities", set.Shed)
	}
	for _, id := range set.Shed {
		if !slices.Contains(wasVisible, id) {
			t.Fatalf("shed entity %d was not visible", id)
		}
	}
	for _, id := range []uint64{300, 301, 302} {
		if !slices.Contains(set.Enter, id) {
			t.Fatalf("nearer entity %d not admitted", id)
		}
	}

	// Never-shed classes: a party member and an objective entity are
	// admitted even while the cap is saturated.
	upsert(g, src, 200, 3000, 0)
	src.infos[200] = EntityInfo{X: 3000, Party: true}
	upsert(g, src, 201, 4000, 0)
	src.infos[201] = EntityInfo{X: 4000, Objective: true}
	set = v.Recompute(g, src, 0, 0, s)
	if !slices.Contains(set.Visible, 200) || !slices.Contains(set.Visible, 201) {
		t.Fatalf("never-shed classes dropped: Visible=%v", set.Visible)
	}
	if len(set.Visible) != MaxVisible {
		t.Fatalf("Visible=%d, want cap %d", len(set.Visible), MaxVisible)
	}

	// Drop the 10 nearest visible entities: the count falls at or below
	// ReAddAt and cap-shed entities rejoin in rank order.
	removed := 0
	for _, id := range wasVisible {
		if id == 200 || id == 201 {
			continue
		}
		g.Remove(id)
		delete(src.infos, id)
		removed++
		if removed >= 10 {
			break
		}
	}
	set = v.Recompute(g, src, 0, 0, s)
	if len(set.Enter) == 0 {
		t.Fatal("cap-shed entities were not re-added at ReAddAt")
	}
	if len(set.Visible) > MaxVisible {
		t.Fatalf("Visible=%d exceeds cap", len(set.Visible))
	}
	for _, id := range set.Enter {
		if slices.Contains(wasVisible, id) {
			continue
		}
		// re-added entity must be live in the source
		if _, ok := src.infos[id]; !ok {
			t.Fatalf("Enter lists removed entity %d", id)
		}
	}
}
