package runtime

import (
	"context"
	"time"

	"thinhthan/internal/core/id"
	"thinhthan/internal/core/rng"
	observability "thinhthan/internal/observability/core"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/aoi"
	"thinhthan/internal/sim/replication"
)

// Step is the fixed 50 ms simulation step (20 Hz, ADR-0007).
const Step = 50 * time.Millisecond

// MaxCatchUpTicks bounds consecutive catch-up steps per scheduling slice
// (realtime_loop.md: catch-up cap 3). The sim never runs more than this
// many ticks to close a scheduling gap; deeper lags are resynced and
// counted as overrun.
const MaxCatchUpTicks = 3

// PartitionConfig carries the immutable partition identity and injectable
// clock. ContentRevision must be the canonical 64-hex bundle hash; it
// anchors both RNG stream derivation and the replication baseline.
type PartitionConfig struct {
	MapID           string
	ChannelID       uint64
	InstanceID      id.UUID
	ContentRevision string
	Seed            uint64

	// Now returns monotonic wall time. Tests inject a manual clock.
	Now func() time.Duration
	// Sleep suspends the Run loop; tests inject a no-op or manual wait.
	Sleep func(time.Duration)
	// Metrics receives the per-tick runtime and queue-wait histograms;
	// nil disables emission.
	Metrics *observability.Registry
}

// TickContext is the read-only per-tick view handed to registered systems.
type TickContext struct {
	Tick    uint64
	SimTime time.Duration
	Now     time.Duration
}

type removedLog struct {
	ID     uint64
	Reason uint8
}

const (
	removedRemoved uint8 = iota + 1
	removedDied
	removedTransferred
)

// replClient is one player's replication session: AOI viewer state, the
// message builder, baseline bookkeeping and the last-sent entity view.
type replClient struct {
	attached       bool
	viewer         aoi.Viewer
	builder        *replication.Builder
	baselineID     uint64
	baselineAcked  bool
	pendingBaseline bool
	resyncPending  bool
	resyncReq      uint64
	resyncReason   protocolv1.ResyncReason
	prevEnt        [replication.MaxEntitiesPerView]replication.EntitySnapshot
	prevN          int
	prevPriv       replication.PrivateSnapshot
	curEnt         [replication.MaxEntitiesPerView]replication.EntitySnapshot
}

// prevIdx locates id in the client's last-sent entity list.
func (c *replClient) prevIdx(id uint64) int {
	for i := 0; i < c.prevN; i++ {
		if c.prevEnt[i].ID == id {
			return i
		}
	}
	return -1
}

// prevSet records that the client now knows entity state s (from a spawn or
// baseline).
func (c *replClient) prevSet(s *replication.EntitySnapshot) {
	if i := c.prevIdx(s.ID); i >= 0 {
		c.prevEnt[i] = *s
		return
	}
	if c.prevN >= len(c.prevEnt) {
		return
	}
	c.prevEnt[c.prevN] = *s
	c.prevN++
}

// prevDel drops an entity the client no longer knows.
func (c *replClient) prevDel(id uint64) {
	i := c.prevIdx(id)
	if i < 0 {
		return
	}
	c.prevEnt[i] = c.prevEnt[c.prevN-1]
	c.prevN--
}

// Partition is the single-owner map-instance actor. One goroutine runs
// Run (or drives Tick); every other surface — SubmitIntent, Admit, Remove,
// Stream — is safe to call from other goroutines only as documented.
// SubmitIntent is the mailbox handoff and is always goroutine-safe.
type Partition struct {
	cfg   PartitionConfig
	ports Ports

	mailbox chan Intent

	ent     [EntityCap]Entity
	used    [EntityCap]bool
	classN  [5]int
	nextSeq [EntityCap]uint64

	inboxes [PlayersCap]inbox
	clients [PlayersCap]replClient

	grid    *aoi.Grid
	scratch aoi.Scratch

	streams     map[rng.Context]*rng.Stream
	incarnation id.Incarnation

	pending   [ResultsCap]DurableCommand
	pendingN  int
	resBuf    [ResultsCap]Result
	rejected  [RejectCap]Rejected
	rejectedN int
	removed   [EntityCap]removedLog
	removedN  int

	systems [12][]func(*Partition, *TickContext)

	tickN   uint64
	simTime time.Duration
	started bool

	nextBaseline uint64

	tickRuntime      *observability.Instrument
	queueWait        *observability.Instrument
	overrunTicks     *observability.Instrument
	intentsRejected  *observability.Instrument

	resultHandler func(*Partition, *Result)
}

// SetResultHandler installs the hook invoked once per drained durable
// result inside APPLY_COMMITTED_EXTERNAL_RESULTS.
func (p *Partition) SetResultHandler(fn func(*Partition, *Result)) {
	p.resultHandler = fn
}

// NewPartition wires one partition. ports may be zero-valued for sim-only
// tests; an unset Loader means no durable start gate.
func NewPartition(cfg PartitionConfig, ports Ports) (*Partition, error) {
	if err := id.ValidateContentRevision(cfg.ContentRevision); err != nil {
		return nil, err
	}
	if cfg.Now == nil {
		cfg.Now = monotonic
	}
	if cfg.Sleep == nil {
		cfg.Sleep = time.Sleep
	}
	p := &Partition{
		cfg:          cfg,
		ports:        ports,
		mailbox:      make(chan Intent, MailboxCap),
		grid:         aoi.NewGrid(aoi.LeaveMM),
		streams:      make(map[rng.Context]*rng.Stream, 8),
		incarnation:  id.NewIncarnation(),
		nextBaseline: 1,
	}
	for i := range p.clients {
		p.clients[i].builder = replication.NewBuilder()
	}
	if cfg.Metrics != nil {
		if inst, err := cfg.Metrics.Register(observability.Descriptor{Name: "sim_tick_runtime_ns", Kind: observability.Histogram, Unit: "ns"}); err == nil {
			p.tickRuntime = inst
		}
		if inst, err := cfg.Metrics.Register(observability.Descriptor{Name: "sim_intent_queue_wait_ns", Kind: observability.Histogram, Unit: "ns"}); err == nil {
			p.queueWait = inst
		}
		if inst, err := cfg.Metrics.Register(observability.Descriptor{Name: "sim_tick_overrun", Kind: observability.Counter, Unit: "tick"}); err == nil {
			p.overrunTicks = inst
		}
		if inst, err := cfg.Metrics.Register(observability.Descriptor{Name: "sim_intents_rejected", Kind: observability.Counter, Unit: "intent"}); err == nil {
			p.intentsRejected = inst
		}
	}
	return p, nil
}

// TickN reports the number of completed ticks.
func (p *Partition) TickN() uint64 { return p.tickN }

// SimTime reports the partition's stepped simulation clock.
func (p *Partition) SimTime() time.Duration { return p.simTime }

// Started reports whether the durable start gate has completed.
func (p *Partition) Started() bool { return p.started }

// TickID exposes the deterministic runtime id for tests.
func (p *Partition) Grid() *aoi.Grid { return p.grid }

// RegisterSystem installs fn into the given phase. Systems run after the
// phase's built-in work in registration order; phases with no registered
// system are deterministic no-ops in the fixed order.
func (p *Partition) RegisterSystem(ph PhaseID, fn func(*Partition, *TickContext)) {
	p.systems[ph] = append(p.systems[ph], fn)
}

// attachClient binds a replication client to a freshly admitted player.
func (p *Partition) attachClient(id uint64) {
	c := &p.clients[slotOf(id)]
	c.attached = true
	c.viewer.EntityID = id
	c.pendingBaseline = true
	c.baselineAcked = false
	c.baselineID = p.nextBaseline
	p.nextBaseline++
	c.prevN = 0
}

// detachClient unbinds a removed player's replication client.
func (p *Partition) detachClient(slot int) {
	c := &p.clients[slot]
	b := c.builder
	*c = replClient{}
	c.builder = b
}

// monotonic is the default clock: nanoseconds since process start.
func monotonic() time.Duration { return time.Since(procStart) }

var procStart = time.Now()

// Start runs the durable start gate: world_consequence rows for this
// (map, channel) are loaded before the partition admits or ticks.
func (p *Partition) Start(ctx context.Context) error {
	if p.started {
		return nil
	}
	if p.ports.Loader != nil {
		st, err := p.ports.Loader.LoadPartitionState(ctx, p.cfg.MapID, p.cfg.ChannelID)
		if err != nil {
			return err
		}
		p.applyLoaded(&st)
	}
	p.started = true
	return nil
}

// applyLoaded brings loaded durable state into runtime state. Loaded world
// consequence rows mark event entities; the sim keeps no SQL rows itself.
func (p *Partition) applyLoaded(st *PartitionState) {
	for i := range st.Consequences {
		c := &st.Consequences[i]
		if !c.Active {
			continue
		}
		id, err := p.Admit(ClassEvent)
		if err != nil {
			return
		}
		e, _ := p.Entity(id)
		e.Snap.ContentID = c.RelicID
		e.Snap.Kind = 0
		e.ExpiresAtTick = c.ExpiresAtTick
		e.Objective = true
	}
}

// Run is the owning goroutine loop: start gate, then fixed-step ticking
// with bounded catch-up until ctx is done.
func (p *Partition) Run(ctx context.Context) error {
	if err := p.Start(ctx); err != nil {
		return err
	}
	p.simTime = p.cfg.Now()
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		p.stepUntil(p.cfg.Now())
		wait := p.simTime + Step - p.cfg.Now()
		if wait > 0 {
			p.cfg.Sleep(wait)
		}
	}
}

// stepUntil runs at most MaxCatchUpTicks ticks to close the gap between
// simTime and now, then resyncs deeper lag forward. Every executed tick
// runs all twelve phases; only whole ticks are ever skipped.
func (p *Partition) stepUntil(now time.Duration) {
	ran := 0
	for p.simTime+Step <= now && ran < MaxCatchUpTicks {
		p.tick()
		ran++
	}
	if p.simTime+Step <= now {
		skipped := int64((now - p.simTime) / Step)
		p.simTime += time.Duration(skipped) * Step
		if p.overrunTicks != nil {
			p.overrunTicks.Add(context.Background(), skipped, nil)
		}
	}
}

// tick executes one complete tick: mailbox drain, the twelve ordered
// phases, then tick metrics.
func (p *Partition) tick() {
	start := p.cfg.Now()
	p.rejectedN = 0

	qWaitSum, qWaitN := int64(0), int64(0)
	for {
		select {
		case i := <-p.mailbox:
			qWaitSum += int64(start - i.QueuedAt)
			qWaitN++
			p.routeIntent(i, &p.rejected, &p.rejectedN)
		default:
			goto drained
		}
	}
drained:

	p.tickN++
	p.simTime += Step
	tc := TickContext{Tick: p.tickN, SimTime: p.simTime, Now: start}
	for ph := PhaseID(0); ph < phaseCount; ph++ {
		p.runBuiltin(ph, &tc)
		for _, fn := range p.systems[ph] {
			fn(p, &tc)
		}
	}
	p.emitMetrics(start, qWaitSum, qWaitN)
	// The removed log is consumed by this tick's BUILD phase; removals
	// outside a tick (disconnects, tests) persist into the next one.
	p.removedN = 0
}
