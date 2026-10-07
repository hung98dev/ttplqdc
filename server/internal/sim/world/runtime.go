package world

import (
	"context"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	observability "thinhthan/internal/observability/core"
	"thinhthan/internal/sim/replication"
	"thinhthan/internal/sim/runtime"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Config is the runtime's injected surface.
type Config struct {
	// Maps is the compiled world catalog.
	Maps MapCatalog
	// Loader is the durable-read port (consequences + checkpoint).
	Loader Loader
	// Durable is the EMIT_DURABLE_COMMANDS surface each partition emits
	// into; nil in tests records nothing.
	Durable runtime.EmitPort
	// Results is the durable-result intake shared by all partitions.
	Results runtime.ResultInPort
	// Outbound carries world-control messages to the edge session queue.
	Outbound runtime.OutboundPort
	// Now is wall-clock time for lifecycle, pending and consult bounds.
	Now func() time.Time
	// SimNow/SimSleep drive each partition's loop clock; nil derives
	// SimNow from Now (relative to construction) and uses time.Sleep.
	SimNow   func() time.Duration
	SimSleep func(time.Duration)
	// Metrics receives channel lifecycle instruments; nil disables.
	Metrics *observability.Registry
	// ContentRevision stamps 105s.
	ContentRevision string
	// Seed bases partition seeds (Seed + channel index).
	Seed uint64
	// SyncStarts runs channel start steps inline (deterministic tests);
	// production leaves starts on their own goroutines.
	SyncStarts bool
	// ManualTick disables the director loop; tests drive TickOnce.
	ManualTick bool
}

type chanKey struct {
	mapID string
	ch    uint32
}

// Runtime owns the director, every running channel host, the pending
// ledger and the checkpoint-write ledger. It is the producer surface of
// the ADR-0083 mailbox request/reply: edge/world posts Consults and
// Commands here; consults never mutate and never journal.
type Runtime struct {
	cfg      Config
	director *Director

	mu          sync.RWMutex
	hosts       map[chanKey]*ChannelHost
	admits      map[chanKey][]*Command // TransferStart queued while starting
	dispatcher  *InteractDispatcher
	hooks       []StartupHook
	checkpointLedger *checkpointWrites

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewRuntime builds the world runtime; the director answers
// director/channel-scoped consults itself through member routing.
func NewRuntime(cfg Config) *Runtime {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.SimNow == nil {
		t0 := cfg.Now()
		cfg.SimNow = func() time.Duration { return cfg.Now().Sub(t0) }
	}
	if cfg.SimSleep == nil {
		cfg.SimSleep = time.Sleep
	}
	w := &Runtime{
		cfg:              cfg,
		hosts:            make(map[chanKey]*ChannelHost),
		admits:           make(map[chanKey][]*Command),
		dispatcher:       NewInteractDispatcher(),
		checkpointLedger: newCheckpointWrites(),
	}
	w.director = NewDirector(cfg.Maps, cfg.Now)
	w.hooks = []StartupHook{consequenceLoadHook}
	return w
}

// Director exposes the channel registry (tests, consult plumbing).
func (w *Runtime) Director() *Director { return w.director }

// Dispatcher exposes the service registry (edge wires IMP-020's travel).
func (w *Runtime) Dispatcher() *InteractDispatcher { return w.dispatcher }

// SetEmitPort installs the partition durable surface post-construction:
// the emit adapter references the runtime (checkpoint payload ledger),
// so composition builds the runtime first, then the adapter over it.
// Partitions read it when their host is ensured.
func (w *Runtime) SetEmitPort(p runtime.EmitPort) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cfg.Durable = p
}

// Start launches the director loop unless ManualTick.
func (w *Runtime) Start(ctx context.Context) {
	w.ctx, w.cancel = context.WithCancel(ctx)
	if w.cfg.ManualTick {
		return
	}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		t := time.NewTicker(50 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-w.ctx.Done():
				return
			case <-t.C:
				w.TickOnce()
			}
		}
	}()
}

// Stop ends the director loop and every running channel host.
func (w *Runtime) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	w.mu.Lock()
	hosts := make([]*ChannelHost, 0, len(w.hosts))
	for _, h := range w.hosts {
		hosts = append(hosts, h)
	}
	w.hosts = make(map[chanKey]*ChannelHost)
	w.mu.Unlock()
	for _, h := range hosts {
		h.stop()
	}
	w.wg.Wait()
}

// TickOnce runs one director tick: pending-placement retries, expired
// transfers (source recovery) and idle-channel stops. The 50 ms period
// matches the sim tick.
func (w *Runtime) TickOnce() {
	ef := w.director.Tick()
	for _, pw := range ef.pending {
		w.retryPending(pw)
	}
	for _, t := range w.director.expiredTransfers() {
		w.recoverTransfer(t)
	}
	for _, s := range ef.idle {
		w.stopChannel(s.MapID, s.Channel)
	}
}

// retryPending re-selects a queued forced placement; success resolves it
// through the registered callback, otherwise it re-emits the 15 and
// reschedules for +5s.
func (w *Runtime) retryPending(pw *PendingWait) {
	res, next, err := w.director.Select(pw.Request)
	if err == nil && next == nil {
		w.director.DropPending(pw.CharacterID)
		if pw.Resolve != nil {
			pw.Resolve(w, res)
		}
		return
	}
	// Still saturated: reschedule and re-emit the retry notice.
	pw.nextAt = w.cfg.Now().Add(PendingRetryMS * time.Millisecond)
	w.emitPending(pw.CharacterID, pw)
}

// emitPending sends the 15 from the runtime (member-less path).
func (w *Runtime) emitPending(characterID id.UUID, pw *PendingWait) {
	w.emit(characterID, MsgS2CPlacementPending, &protocolv1.S2CPlacementPending{
		RequestMessageId: pw.RequestMessageID,
		Reason:           protocolv1.PlacementReason(pw.Reason),
		RetryAfterMs:     pw.RetryAfterMs,
	})
}

// emit sends one world-control message to a character's session.
func (w *Runtime) emit(characterID id.UUID, msgID uint32, msg proto.Message) {
	if w.cfg.Outbound == nil {
		return
	}
	_ = w.cfg.Outbound.Enqueue(runtime.Outbound{
		To:        SessionTarget(characterID),
		MessageID: msgID,
		Class:     replication.DeliveryAuthoritativeEvent,
		Msg:       msg,
	})
}

// registerPending queues a forced placement that found no channel.
func (w *Runtime) registerPending(pw *PendingWait, resolve func(*Runtime, PlacementResult)) {
	pw.Resolve = resolve
	w.director.AddPending(pw)
}

// Consult posts one ADR-0083 read-only consult to the owning partition's
// mailbox; a member-less character fails closed (INVALID_STATE).
func (w *Runtime) Consult(c Consult) error {
	if c.Reply == nil {
		return ErrNoCharacter
	}
	mapID, ch, ok := w.director.Member(c.CharacterID)
	if !ok {
		c.Reply <- ConsultReply{OK: false, Code: protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE}
		return nil
	}
	h, ok := w.hostFor(mapID, ch)
	if !ok || h.part == nil {
		c.Reply <- ConsultReply{OK: false, Code: protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE}
		return nil
	}
	if err := h.mb.Post(Entry{Con: &c}); err != nil {
		c.Reply <- ConsultReply{OK: false, Code: protocolv1.ErrorCode_ERROR_CODE_SERVER_OVERLOADED}
		return nil
	}
	return nil
}

// PostCommand routes a world command to the owning channel's mailbox.
// Member-less characters are the director's domain (attach, pending) —
// commands for them are rejected by the caller before routing.
func (w *Runtime) PostCommand(charID id.UUID, cmd *Command) error {
	mapID, ch, ok := w.director.Member(charID)
	if !ok {
		return ErrNoCharacter
	}
	h, ok := w.hostFor(mapID, ch)
	if !ok {
		return ErrNoCharacter
	}
	return h.mb.Post(Entry{Cmd: cmd})
}

// PlaceAttach runs the forced placement of a member-less attach (first
// login, resume recovery): destination map + anchor come from the
// caller's checkpoint read. On saturation the wait is registered and
// resolve fires when a channel frees — the caller emits its own reply
// (the 7) inside resolve.
func (w *Runtime) PlaceAttach(characterID id.UUID, mapID, spawnAnchorID string,
	requestMessageID uint32, resolve func(*Runtime, PlacementResult)) (PlacementResult, *PendingWait, error) {
	res, pw, err := w.director.Select(PlacementRequest{
		CharacterID:   characterID,
		MapID:         mapID,
		Kind:          PlaceForced,
		ForcedReason:  ForcedReattach,
		SpawnAnchorID: spawnAnchorID,
	})
	if err != nil {
		return PlacementResult{}, nil, err
	}
	if pw != nil {
		pw.RequestMessageID = requestMessageID
		pw.Reason = PendingFirstLogin
		w.registerPending(pw, resolve)
		w.emitPending(characterID, pw)
		return PlacementResult{}, pw, nil
	}
	return res, nil, nil
}

// AdmitCharacter queues the member admission into the resolved channel —
// the attach path posts it after the session's 7 goes out, carrying the
// character's persisted vitals.
func (w *Runtime) AdmitCharacter(characterID id.UUID, res PlacementResult,
	spawnAnchorID string, vitals Vitals) {
	w.admitPlayer(characterID, res, &Command{
		Kind:          CmdTransferStart,
		CharacterID:   characterID,
		SpawnAnchorID: spawnAnchorID,
		Vitals:        vitals,
	})
}

// hostFor returns the running host of a channel.
func (w *Runtime) hostFor(mapID string, ch uint32) (*ChannelHost, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	h, ok := w.hosts[chanKey{mapID, ch}]
	return h, ok
}
