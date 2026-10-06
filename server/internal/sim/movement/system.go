package movement

import (
	"log/slog"

	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
	"thinhthan/internal/sim/spatial/collision"
)

// Config wires the movement system into a partition. World is the
// authoritative collision space; Outbound receives 107 corrections and
// STALE_INPUT errors (nil port disables emission — tests); SeqSource
// reports the entity's last processed client_seq for
// 107.last_processed_client_seq (required when Outbound is set);
// StampSource yields the per-edge (client_mono_ms, receive_ms) pair for
// staleness evaluation — nil disables it until the F-02 feed lands.
type Config struct {
	World       *collision.World
	Outbound    runtime.OutboundPort
	SeqSource   func(id uint64) uint64
	StampSource func(id uint64) (clientMonoMs int64, receiveMs int64, ok bool)
	Logger      *slog.Logger
}

// System is the PhaseMovement system: one instance per partition, all
// per-entity state in a fixed [EntityCap] array indexed by e.Slot.
type System struct {
	world    *collision.World
	outbound runtime.OutboundPort
	seqOf    func(uint64) uint64
	stamps   func(uint64) (int64, int64, bool)
	log      *slog.Logger
	stale    *StaleTracker

	ents [runtime.EntityCap]entityState

	// Enumeration buffer reused every tick; Within fills it id-sorted.
	enumBuf []uint64

	// Edge stream drained by IMP-014; valid until the next Step.
	events  [runtime.PlayersCap * runtime.DiscreteCap]EdgeEvent
	eventsN int

	// Enumeration square over the geometry bounds: every entity the
	// movement phase owns lies inside it (out-of-bounds positions are
	// recovered through the same enumeration).
	enumX, enumY int32
	enumR        int64
}

// entityState holds the sim-only fields that do not replicate inside
// MovementCheckpoint: the recovery anchor and the knockback deadline.
type entityState struct {
	id           uint64 // slot incarnation guard; respawn resets
	lastSafeX    int64
	lastSafeY    int64
	hasLastSafe  bool
	knockUntilTk uint64 // knockback forces velocity until this tick
	onOneWay     bool   // standing on a ONE_WAY_PLATFORM surface
	groundSeg    int64  // segment id of the current ground contact
}

// New constructs the system. cfg.SeqSource must be non-nil when
// cfg.Outbound is — a 107 without an accurate seq field would force the
// client to replay from the wrong input.
func New(cfg Config) *System {
	if cfg.Outbound != nil && cfg.SeqSource == nil {
		panic("movement: SeqSource required when Outbound is set")
	}
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	s := &System{
		world:    cfg.World,
		outbound: cfg.Outbound,
		seqOf:    cfg.SeqSource,
		stamps:   cfg.StampSource,
		log:      log,
		stale:    NewStaleTracker(),
		enumBuf:  make([]uint64, 0, runtime.EntityCap),
	}
	g := cfg.World.Geometry()
	s.enumX = int32(g.BoundsMM.MaxX / 2)
	s.enumY = int32(g.BoundsMM.MaxY / 2)
	// Cover the full diagonal plus a 64 m margin so entities pushed
	// outside the legal bounds still enumerate for recovery.
	dx, dy := g.BoundsMM.MaxX, g.BoundsMM.MaxY
	s.enumR = dx/2 + dy/2 + 64000
	return s
}

// Step is the PhaseMovement entry point: integrate every enumerated
// entity, in id order, then expose the tick's applied edge events.
func (s *System) Step(p *runtime.Partition, tc *runtime.TickContext) {
	s.eventsN = 0
	grid := p.Grid()
	grid.Within(s.enumX, s.enumY, s.enumR, &s.enumBuf)
	for i := 0; i < len(s.enumBuf); i++ {
		id := s.enumBuf[i]
		e, err := p.Entity(id)
		if err != nil {
			continue
		}
		s.stepEntity(p, e, tc)
	}
}

// DrainEdgeEvents copies the applied edge events of the most recent Step
// into out and returns the count. The stream is tick-scoped: results are
// valid only until the next Step call.
func (s *System) DrainEdgeEvents(out []EdgeEvent) int {
	n := copy(out, s.events[:s.eventsN])
	return n
}

// noteEdge records one applied edge in this tick's stream (capped by the
// discrete-queue bound; every submitted edge fits by construction).
func (s *System) noteEdge(e *runtime.Entity, b uint8, tc *runtime.TickContext) {
	if s.eventsN >= len(s.events) {
		return
	}
	var t protocolv1.MovementEdgeType
	var d protocolv1.Facing
	switch b {
	case EdgeJump:
		t = protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_PRESS
		d = protocolv1.Facing_FACING_UNSPECIFIED
	case EdgeDrop:
		t = protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_RELEASE
		d = protocolv1.Facing_FACING_UNSPECIFIED
	default:
		t, d, _ = DecodeEdge(b)
	}
	ev := &s.events[s.eventsN]
	ev.EntityID = e.ID
	ev.Type = t
	ev.Dir = d
	ev.Tick = tc.Tick
	ev.ClientMonoMs = 0 // advisory only; real stamp feeds via F-02
	s.eventsN++
}

// state returns the per-entity sim state, resetting it when the slot's
// incarnation changed (entity respawned on the same slot).
func (s *System) state(e *runtime.Entity) *entityState {
	st := &s.ents[e.Slot]
	if st.id != e.ID {
		*st = entityState{id: e.ID}
	}
	return st
}
