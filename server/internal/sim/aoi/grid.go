package aoi

import "slices"

// Grid is a deterministic uniform spatial index over integer-millimeter
// positions. It is not safe for concurrent use; the owning partition
// serializes every call.
type Grid struct {
	cellMM int64
	ents   map[uint64]int64 // entity id -> linearized cell key
	cells  map[int64][]uint64
}

// NewGrid returns a grid whose cells span cellMM millimeters per side. cellMM
// should be no smaller than the largest query radius so a Within query touches
// at most nine cells.
func NewGrid(cellMM int64) *Grid {
	return &Grid{
		cellMM: cellMM,
		ents:   make(map[uint64]int64, 128),
		cells:  make(map[int64][]uint64, 128),
	}
}

// cellKey linearizes a cell coordinate. Cell coordinates derived from int32
// millimeter positions fit in int32, so packing them into one int64 key keeps
// a single map instead of a nested pair.
func (g *Grid) cellKey(x, y int32) int64 {
	cx := floorDiv(int64(x), g.cellMM)
	cy := floorDiv(int64(y), g.cellMM)
	return (cx << 32) | (cy & 0xffffffff)
}

func floorDiv(v, d int64) int64 {
	q := v / d
	if v%d != 0 && v < 0 {
		q--
	}
	return q
}

// Upsert inserts or moves one entity to (x, y) millimeters.
func (g *Grid) Upsert(id uint64, x, y int32) {
	key := g.cellKey(x, y)
	if old, ok := g.ents[id]; ok {
		if old == key {
			return
		}
		list := g.cells[old]
		for i, e := range list {
			if e == id {
				list[i] = list[len(list)-1]
				list = list[:len(list)-1]
				break
			}
		}
		g.cells[old] = list
	}
	g.ents[id] = key
	g.cells[key] = append(g.cells[key], id)
}

// Remove deletes one entity from the index.
func (g *Grid) Remove(id uint64) {
	key, ok := g.ents[id]
	if !ok {
		return
	}
	delete(g.ents, id)
	list := g.cells[key]
	for i, e := range list {
		if e == id {
			list[i] = list[len(list)-1]
			list = list[:len(list)-1]
			break
		}
	}
	g.cells[key] = list
}

// Within appends every entity indexed in cells intersecting the square of
// side 2*rMM around (x, y), sorted by entity id so callers observe one
// deterministic order. Positions live outside the grid, so the caller
// distance-filters the returned candidates. The caller owns and reuses out;
// it must not retain the slice between Recompute calls.
func (g *Grid) Within(x, y int32, rMM int64, out *[]uint64) {
	*out = (*out)[:0]
	cx0 := floorDiv(int64(x)-rMM, g.cellMM)
	cx1 := floorDiv(int64(x)+rMM, g.cellMM)
	cy0 := floorDiv(int64(y)-rMM, g.cellMM)
	cy1 := floorDiv(int64(y)+rMM, g.cellMM)
	for cx := cx0; cx <= cx1; cx++ {
		for cy := cy0; cy <= cy1; cy++ {
			key := (cx << 32) | (cy & 0xffffffff)
			*out = append(*out, g.cells[key]...)
		}
	}
	slices.Sort(*out)
}
