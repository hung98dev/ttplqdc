package aoi

import "slices"

// AOI shape from docs/05_network/synchronization.md. Distances are integer
// millimeters.
const (
	// EnterMM admits a not-yet-visible entity that comes this close.
	EnterMM int64 = 35000
	// LeaveMM drops a visible entity once it passes this distance. The
	// 5000 mm gap between EnterMM and LeaveMM is the distance hysteresis
	// band.
	LeaveMM int64 = 40000
	// MaxVisible is MAX_ENTITIES_IN_AOI_PER_CLIENT: the hard cap on
	// replicated entities, self excluded (ADR-0071).
	MaxVisible = 40
	// CountHysteresis keeps an entity dropped by the cap out of the
	// visible set until the visible count falls to
	// MaxVisible-CountHysteresis.
	CountHysteresis = 5
	// ReAddAt is the visible count at or below which cap-shed entities
	// become re-addable.
	ReAddAt = MaxVisible - CountHysteresis

	// ChannelEntityCap is MAX_ENTITIES_PER_CHANNEL from
	// docs/04_architecture/realtime_loop.md; it bounds every fixed
	// scratch buffer in this package.
	ChannelEntityCap = 100
)

// Priority classes for bounded replication; higher numeric values shed
// first. Priority 1 and 2 entities are never shed by the cap.
const (
	PriorityPartyMember = 1 // viewer's party
	PriorityObjective   = 2 // active encounter/boss objective
	PriorityInCombat    = 3 // in combat with the viewer
	PriorityHostile     = 4 // hostiles; nearest retained
	PriorityOther       = 5 // everything else; shed first
)

// EntityInfo is the per-entity input Recompute needs. Party and Objective
// mark the never-shed classes; Hostile and InCombat order the sheddable
// ones. The viewer's own entity is never a candidate (ADR-0071: self rides
// the dedicated self fields, never the entity list).
type EntityInfo struct {
	X, Y      int32
	Party     bool
	Objective bool
	InCombat  bool
	Hostile   bool
}

// Priority returns the entity's shed-priority class.
func (e EntityInfo) Priority() int {
	switch {
	case e.Party:
		return PriorityPartyMember
	case e.Objective:
		return PriorityObjective
	case e.InCombat:
		return PriorityInCombat
	case e.Hostile:
		return PriorityHostile
	default:
		return PriorityOther
	}
}

// Source exposes entity position and priority data to Recompute without the
// package knowing the partition's entity table. The implementing partition
// answers one call per candidate entity per viewer per tick.
type Source interface {
	// Info returns the entity record for id as seen by viewerID, or
	// ok=false when id is not live.
	Info(viewerID, id uint64) (EntityInfo, bool)
}

type candEnt struct {
	ID    uint64
	Dist2 int64
	Pri   uint8
	Was   bool // visible before this recompute
	Shed  bool // cap-shed marker carried across ticks
	Live  bool // inside its applicable distance radius this tick
}

// Scratch owns the backing arrays for the per-tick Set lists so Recompute
// stays allocation-free. One scratch is reused across all viewers of a
// partition tick.
type Scratch struct {
	cand  [ChannelEntityCap]candEnt
	enter [ChannelEntityCap]uint64
	leave [ChannelEntityCap]uint64
	shed  [ChannelEntityCap]uint64
	vis   [ChannelEntityCap]uint64
	ids   [ChannelEntityCap]uint64
}

// Set is one viewer's recomputed interest result. The slices borrow Scratch
// storage and stay valid only until the next Recompute on the same Scratch.
type Set struct {
	// Enter holds ids becoming visible (replication spawns them).
	Enter []uint64
	// Leave holds ids that moved beyond LeaveMM or vanished from the
	// grid (the caller maps the despawn reason).
	Leave []uint64
	// Shed holds previously visible ids still in range but dropped by the
	// cap (despawn SHED).
	Shed []uint64
	// Visible holds the full visible entity set, sorted, self excluded.
	Visible []uint64
}

// Viewer is one replicated client's persistent AOI state: the visible set
// plus the cap-shed marks that give the count hysteresis its memory.
type Viewer struct {
	EntityID uint64
	vis      [MaxVisible]uint64 // currently visible, sorted
	visN     int
	shed     [ChannelEntityCap]uint64 // ids cap-shed, sorted
	shedN    int
}

// Visible reports the viewer's current visible set (sorted, self excluded).
func (v *Viewer) Visible() []uint64 { return v.vis[:v.visN] }

// Contains reports whether id is currently visible to the viewer.
func (v *Viewer) Contains(id uint64) bool {
	_, ok := slices.BinarySearch(v.vis[:v.visN], id)
	return ok
}

func (v *Viewer) isShed(id uint64) bool {
	_, ok := slices.BinarySearch(v.shed[:v.shedN], id)
	return ok
}

func (v *Viewer) markShed(id uint64) {
	i, ok := slices.BinarySearch(v.shed[:v.shedN], id)
	if ok || v.shedN == len(v.shed) {
		return
	}
	copy(v.shed[i+1:v.shedN+1], v.shed[i:v.shedN])
	v.shed[i] = id
	v.shedN++
}

func (v *Viewer) unmarkShed(id uint64) {
	i, ok := slices.BinarySearch(v.shed[:v.shedN], id)
	if !ok {
		return
	}
	copy(v.shed[i:v.shedN-1], v.shed[i+1:v.shedN])
	v.shed[v.shedN-1] = 0
	v.shedN--
}

func rankLess(a, b candEnt) int {
	if a.Pri != b.Pri {
		return int(a.Pri) - int(b.Pri)
	}
	if a.Dist2 != b.Dist2 {
		if a.Dist2 < b.Dist2 {
			return -1
		}
		return 1
	}
	switch {
	case a.ID < b.ID:
		return -1
	case a.ID > b.ID:
		return 1
	}
	return 0
}

// Recompute advances the viewer's interest state by one tick. Candidates
// come from Grid.Within (ascending id order) and are distance-filtered with
// the enter/leave hysteresis; overflow beyond MaxVisible is resolved by the
// fixed shed order — everything else first, then hostiles, then
// in-combat — nearest-first inside each class, id ascending as tiebreak.
// Cap-shed entities keep their shed mark until the visible count is at most
// ReAddAt; the mark expires if the entity leaves the enter radius or is
// re-added. Identical inputs produce identical sets on every partition.
func (v *Viewer) Recompute(g *Grid, src Source, x, y int32, s *Scratch) Set {
	ids := s.ids[:0]
	g.Within(x, y, LeaveMM, &ids)

	enterR2 := EnterMM * EnterMM
	leaveR2 := LeaveMM * LeaveMM

	n := 0
	for _, id := range ids {
		if id == v.EntityID {
			continue
		}
		info, ok := src.Info(v.EntityID, id)
		if !ok {
			continue
		}
		dx := int64(info.X) - int64(x)
		dy := int64(info.Y) - int64(y)
		d2 := dx*dx + dy*dy
		c := candEnt{
			ID:    id,
			Dist2: d2,
			Pri:   uint8(info.Priority()),
			Was:   v.Contains(id),
			Shed:  v.isShed(id),
		}
		switch {
		case c.Was:
			// A still-visible entity cannot carry a cap-shed mark.
			c.Live = d2 <= leaveR2
			c.Shed = false
		case c.Shed:
			// Cap-shed re-add is gated on the visible count below.
			c.Live = d2 <= enterR2
		default:
			c.Live = d2 <= enterR2
		}
		s.cand[n] = c
		n++
	}
	cand := s.cand[:n]

	// Shed marks expire for entities no longer present as cap-shed
	// candidates (they left the enter radius or were removed).
	for i := 0; i < v.shedN; {
		stillShed := false
		for j := range cand {
			if cand[j].ID == v.shed[i] {
				stillShed = cand[j].Shed
				break
			}
		}
		if stillShed {
			i++
			continue
		}
		copy(v.shed[i:v.shedN-1], v.shed[i+1:v.shedN])
		v.shed[v.shedN-1] = 0
		v.shedN--
	}

	slices.SortFunc(cand, rankLess)

	visN, enterN, shedN := 0, 0, 0
	// Pass 1: admit live non-shed candidates in rank order up to the cap.
	// Ranked-tail overflow becomes cap-shed.
	for _, c := range cand {
		if !c.Live || c.Shed {
			continue
		}
		if visN >= MaxVisible && c.Pri > PriorityObjective {
			v.markShed(c.ID)
			if c.Was {
				s.shed[shedN] = c.ID
				shedN++
			}
			continue
		}
		s.vis[visN] = c.ID
		visN++
		if !c.Was {
			s.enter[enterN] = c.ID
			enterN++
		}
	}
	// Pass 2: once the visible count is at most ReAddAt, cap-shed
	// candidates rejoin normal admission in rank order.
	if visN <= ReAddAt {
		for _, c := range cand {
			if !c.Live || !c.Shed || visN >= MaxVisible {
				continue
			}
			s.vis[visN] = c.ID
			visN++
			v.unmarkShed(c.ID)
			if !c.Was {
				s.enter[enterN] = c.ID
				enterN++
			}
		}
	}

	slices.Sort(s.vis[:visN])
	slices.Sort(s.enter[:enterN])
	slices.Sort(s.shed[:shedN])

	// Previously visible entities that are neither in the new visible set
	// nor cap-shed this tick left the AOI (radius or removal).
	leaveN := 0
outer:
	for _, id := range v.vis[:v.visN] {
		for j := 0; j < visN; j++ {
			if s.vis[j] == id {
				continue outer
			}
		}
		for j := 0; j < shedN; j++ {
			if s.shed[j] == id {
				continue outer
			}
		}
		s.leave[leaveN] = id
		leaveN++
	}
	slices.Sort(s.leave[:leaveN])

	copy(v.vis[:], s.vis[:visN])
	v.visN = visN

	return Set{
		Enter:   s.enter[:enterN],
		Leave:   s.leave[:leaveN],
		Shed:    s.shed[:shedN],
		Visible: s.vis[:visN],
	}
}
