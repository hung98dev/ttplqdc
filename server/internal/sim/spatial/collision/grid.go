package collision

import (
	"sync"

	"thinhthan/internal/sim/spatial/geometry"
)

// cellMM is the uniform grid pitch. The value only affects index density;
// results are independent of it, so it must stay a fixed constant.
const cellMM = 2048

// World is a read-only uniform-grid spatial index over a validated Geometry.
// After construction it is safe for concurrent use from a single sim
// goroutine or many readers; it must never be mutated.
type World struct {
	g     *geometry.Geometry
	cols  int64
	rows  int64
	cells [][]int // cell -> indices into segs
	segs  []geometry.Segment
}

// NewWorld builds the immutable index for a parsed geometry.
func NewWorld(g *geometry.Geometry) *World {
	w := &World{
		g:    g,
		cols: g.BoundsMM.MaxX/cellMM + 1,
		rows: g.BoundsMM.MaxY/cellMM + 1,
		segs: g.Segments,
	}
	w.cells = make([][]int, w.cols*w.rows)
	for i, s := range w.segs {
		x0, x1 := min64(s.X1, s.X2)/cellMM, max64(s.X1, s.X2)/cellMM
		y0, y1 := min64(s.Y1, s.Y2)/cellMM, max64(s.Y1, s.Y2)/cellMM
		for cy := y0; cy <= y1; cy++ {
			for cx := x0; cx <= x1; cx++ {
				w.cells[cy*w.cols+cx] = append(w.cells[cy*w.cols+cx], i)
			}
		}
	}
	return w
}

// Geometry returns the underlying immutable geometry.
func (w *World) Geometry() *geometry.Geometry { return w.g }

// forEach visits each segment intersecting box b exactly once (order =
// segment order, so behavior is deterministic).
func (w *World) forEach(b AABB, fn func(s geometry.Segment)) {
	x0 := b.MinX / cellMM
	x1 := b.MaxX / cellMM
	y0 := b.MinY / cellMM
	y1 := b.MaxY / cellMM
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 >= w.cols {
		x1 = w.cols - 1
	}
	if y1 >= w.rows {
		y1 = w.rows - 1
	}
	// Dedup scratch comes from a pool so the tick hot path is zero-alloc; the
	// generation stamp avoids a clearing pass between calls. Pooling keeps
	// World immutable — it may be read concurrently.
	mb := markPool.Get().(*markBuf)
	if cap(mb.marks) < len(w.segs) {
		mb.marks = make([]uint32, len(w.segs))
	}
	marks := mb.marks[:len(w.segs)]
	mb.gen++
	if mb.gen == 0 { // wrapped: re-stamp everything
		clear(marks)
		mb.gen = 1
	}
	gen := mb.gen
	defer markPool.Put(mb)
	for cy := y0; cy <= y1; cy++ {
		for cx := x0; cx <= x1; cx++ {
			for _, i := range w.cells[cy*w.cols+cx] {
				if marks[i] == gen {
					continue
				}
				marks[i] = gen
				s := w.segs[i]
				if max64(s.X1, s.X2) < b.MinX || min64(s.X1, s.X2) > b.MaxX ||
					max64(s.Y1, s.Y2) < b.MinY || min64(s.Y1, s.Y2) > b.MaxY {
					continue
				}
				fn(s)
			}
		}
	}
}

// markBuf is the pooled per-call visited set for forEach: marks[i] == gen
// means segment i was already emitted this call.
type markBuf struct {
	gen   uint32
	marks []uint32
}

var markPool = sync.Pool{New: func() any { return &markBuf{} }}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
