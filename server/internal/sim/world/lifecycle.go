package world

import (
	"context"
	"fmt"
	"time"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
)

// StartupHook is one channel-start pipeline stage; hooks run inside the
// start goroutine after the partition exists and before it runs — a hook
// failure quarantines the channel (sharding.md start pipeline).
type StartupHook func(ctx context.Context, h *ChannelHost) error

// RegisterStartupHook appends a hook to the start pipeline (after the
// built-in consequenceLoadHook, before the channel opens for admission).
func (w *Runtime) RegisterStartupHook(fn StartupHook) {
	w.hooks = append(w.hooks, fn)
}

// consequenceLoadHook is the built-in loader gate: the partition
// StartLoaderPort loads durable partition state under a 5s timeout; a
// timeout or load failure quarantines the channel.
func consequenceLoadHook(ctx context.Context, h *ChannelHost) error {
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return h.part.Start(c)
}

// admitPlayer ensures the destination channel runs, then queues the
// admission command into its mailbox. Occupancy was reserved by Select.
func (w *Runtime) admitPlayer(characterID id.UUID, res PlacementResult, cmd *Command) {
	cmd.MapID = res.MapID
	h, err := w.ensureHost(res.MapID, res.ChannelIndex)
	if err != nil {
		// Start failed (quarantined): the reserved occupancy is rolled
		// back and the request re-enters the forced order as pending.
		w.reservedFailed(res)
		pw := NewPendingWait(w.cfg.Now(), characterID, 0,
			PendingReconnect, cmdRequest(cmd))
		w.registerPending(pw, func(w *Runtime, r PlacementResult) {
			w.admitPlayer(characterID, r, cmd)
		})
		w.emitPending(characterID, pw)
		return
	}
	w.mu.Lock()
	if h != nil && h.started() {
		w.mu.Unlock()
		_ = h.mb.Post(Entry{Cmd: cmd})
		return
	}
	key := chanKey{res.MapID, res.ChannelIndex}
	w.admits[key] = append(w.admits[key], cmd)
	w.mu.Unlock()
}

func cmdRequest(cmd *Command) PlacementRequest {
	return PlacementRequest{
		CharacterID:   cmd.CharacterID,
		Kind:          PlaceForced,
		ForcedReason:  ForcedReattach,
		SpawnAnchorID: cmd.SpawnAnchorID,
	}
}

// reservedFailed rolls back one placement reservation on a channel whose
// start failed, so occupancy cannot leak through a quarantine.
func (w *Runtime) reservedFailed(res PlacementResult) {
	if c, ok := w.director.Channel(res.MapID, res.ChannelIndex); ok && c.Occupancy > 0 {
		c.Occupancy--
	}
}

// ensureHost returns the running host if any, else drives channel start
// (async unless cfg.SyncStarts) and returns nil — admissions queue on
// the admits ledger until the host marks running. startHost runs outside
// w.mu: it locks internally (admits flush, director callbacks), so the
// owner first registers the host then starts it.
func (w *Runtime) ensureHost(mapID string, ch uint32) (*ChannelHost, error) {
	key := chanKey{mapID, ch}
	w.mu.Lock()
	if h, ok := w.hosts[key]; ok {
		w.mu.Unlock()
		if h.started() {
			return h, nil
		}
		return nil, nil // start in-flight elsewhere
	}
	c, ok := w.director.Channel(mapID, ch)
	if !ok {
		w.mu.Unlock()
		return nil, fmt.Errorf("world: no channel %s[%d]", mapID, ch)
	}
	if c.State == ChannelDraining {
		w.mu.Unlock()
		return nil, fmt.Errorf("world: channel %s[%d] draining", mapID, ch)
	}
	h, err := w.newHost(mapID, ch)
	if err != nil {
		w.mu.Unlock()
		return nil, err
	}
	w.hosts[key] = h
	w.mu.Unlock()
	if w.cfg.SyncStarts {
		if err := w.startHost(w.ctxOrBackground(), h); err != nil {
			w.mu.Lock()
			delete(w.hosts, key)
			w.mu.Unlock()
			return nil, err
		}
	} else {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			if err := w.startHost(w.ctx, h); err != nil {
				w.mu.Lock()
				delete(w.hosts, key)
				w.mu.Unlock()
			}
		}()
	}
	if h.started() {
		return h, nil
	}
	return nil, nil
}

func (w *Runtime) ctxOrBackground() context.Context {
	if w.ctx != nil {
		return w.ctx
	}
	return context.Background()
}

// newHost wires the partition, mailbox, drain system and result handler
// for one channel; not yet started.
func (w *Runtime) newHost(mapID string, ch uint32) (*ChannelHost, error) {
	h := &ChannelHost{
		w:         w,
		mapID:     mapID,
		ch:        ch,
		mb:        NewMailbox(),
		players:   make(map[id.UUID]*worldPlayer),
		npcs:      make(map[uint64]*npcInst),
		sessions:  newNpcSessions(),
		fishing:   fishingNoops,
		respawned: make(map[[16]byte]*protocolv1.S2CRespawn),
	}
	if w.fishingFactory != nil {
		if fh := w.fishingFactory(mapID, ch); fh != nil {
			h.fishing = fh
			if h.fishing.Cast == nil {
				h.fishing.Cast = fishingNoops.Cast
			}
			if h.fishing.Hook == nil {
				h.fishing.Hook = fishingNoops.Hook
			}
		}
	}
	pcfg := runtime.PartitionConfig{
		MapID:           mapID,
		ChannelID:       uint64(ch),
		ContentRevision: w.cfg.ContentRevision,
		Seed:            w.cfg.Seed + uint64(ch),
		Now:             w.cfg.SimNow,
		Sleep:           w.cfg.SimSleep,
		Metrics:         w.cfg.Metrics,
	}
	p, err := runtime.NewPartition(pcfg, runtime.Ports{
		Durable: w.cfg.Durable,
		Results: w.cfg.Results,
		Loader:  loaderPort{w: w},
		// Replication traffic and world control share the outbound port:
		// replication rows carry the entity id, world-control rows carry
		// the session target (SessionTarget); the edge pump resolves both.
		Outbound: w.cfg.Outbound,
	})
	if err != nil {
		return nil, err
	}
	h.part = p
	p.SetResultHandler(h.onDurableResult)
	p.RegisterSystem(runtime.PhaseExternalResults, h.drainMailbox)
	return h, nil
}

// started reports the partition is running (post-hooks, pre-Run).
func (h *ChannelHost) started() bool { return h.part != nil && h.part.Started() }

// startHost runs the hook pipeline, spawns the map's NPCs, marks the
// channel running, flushes queued admissions and launches the tick loop.
// A hook failure quarantines the channel.
func (w *Runtime) startHost(ctx context.Context, h *ChannelHost) error {
	for _, hook := range w.hooks {
		if err := hook(ctx, h); err != nil {
			w.director.hostFailed(h.mapID, h.ch, err)
			return err
		}
	}
	h.spawnNpcs()
	w.director.hostRunning(h.mapID, h.ch, h)
	// Flush queued admissions.
	w.mu.Lock()
	key := chanKey{h.mapID, h.ch}
	pending := w.admits[key]
	delete(w.admits, key)
	w.mu.Unlock()
	for _, cmd := range pending {
		_ = h.mb.Post(Entry{Cmd: cmd})
	}
	h.ctx, h.cancel = context.WithCancel(ctx)
	h.done = make(chan struct{})
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		defer close(h.done)
		_ = h.part.Run(h.ctx)
	}()
	return nil
}

// spawnNpcs admits the map's NPC roster as event-class entities at their
// anchor positions. NPCs are static fixtures; despawn closes sessions.
func (h *ChannelHost) spawnNpcs() {
	rec, ok := h.w.cfg.Maps.Lookup(h.mapID)
	if !ok {
		return
	}
	for _, def := range rec.Npcs {
		a, ok := rec.Anchor(def.AnchorID)
		if !ok {
			continue
		}
		eid, err := h.part.Admit(runtime.ClassEvent)
		if err != nil {
			continue
		}
		e, err := h.part.Entity(eid)
		if err != nil {
			continue
		}
		_ = h.part.SetPosition(eid, int32(a.X), int32(a.Y))
		e.Snap.ContentID = def.NpcID
		e.Snap.Kind = protocolv1.EntityKind_ENTITY_KIND_NPC
		h.npcs[eid] = &npcInst{EntityID: eid, Def: def}
	}
}

// onDurableResult tracks pending durable emits so idle stop waits for
// commits (sharding.md stop semantics).
func (h *ChannelHost) onDurableResult(p *runtime.Partition, r *runtime.Result) {
	if h.pendingDurable.Load() > 0 {
		h.pendingDurable.Add(-1)
	}
}

// stop cancels the host's partition loop.
func (h *ChannelHost) stop() {
	if h.cancel != nil {
		h.cancel()
	}
}

// stopChannel drains pending durable emits then discards a channel —
// "stop emits pending durable and waits for their commits before
// discarding" (sharding.md). The drain wait is bounded: durable emit
// retries that exceed the budget still leave the channel stoppable, and
// the durable queue's own retry path owns eventual settlement.
func (w *Runtime) stopChannel(mapID string, ch uint32) {
	key := chanKey{mapID, ch}
	w.mu.Lock()
	h, ok := w.hosts[key]
	if ok {
		delete(w.hosts, key)
	}
	w.mu.Unlock()
	if !ok {
		return
	}
	// Wait for pending durable commits (bounded by the stop budget).
	deadline := w.cfg.Now().Add(5 * time.Second)
	for h.pendingDurable.Load() > 0 && w.cfg.Now().Before(deadline) {
		w.cfg.SimSleep(10 * time.Millisecond)
	}
	h.stop()
	w.director.hostStopped(mapID, ch)
}

// commitCheckpointWrite registers the typed payload and queues the flat
// sim.checkpoint command on the partition.
func (w *Runtime) commitCheckpointWrite(p *runtime.Partition, op [16]byte,
	cp *CheckpointWrite) {
	w.checkpointLedger.put(op, cp)
	var opID id.UUID
	copy(opID[:], op[:])
	name, _, err := p.SourceEvent()
	if err != nil {
		w.checkpointLedger.Drop(op)
		return
	}
	_ = p.QueueCommand(runtime.DurableCommand{
		Kind:            runtime.CmdCheckpoint,
		Family:          FamilySimCheckpoint,
		OwnerKind:       runtime.OwnerCharacter,
		Owner:           cp.CharacterID,
		OperationID:     opID,
		SourceEvent:     name,
		ContentRevision: w.cfg.ContentRevision,
		Tick:            p.TickN(),
	})
	h, ok := w.partitionHost(p)
	if ok {
		h.pendingDurable.Add(1)
	}
}

// partitionHost finds the host owning a partition (checkpoint writes are
// queued inside the drain — the owning host is unambiguous).
func (w *Runtime) partitionHost(p *runtime.Partition) (*ChannelHost, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, h := range w.hosts {
		if h.part == p {
			return h, true
		}
	}
	return nil, false
}

// loaderPort adapts the injected world Loader to the partition's
// StartLoaderPort surface: durable rows become plain sim values —
// durable isolation means the sim never sees SQL.
type loaderPort struct{ w *Runtime }

// LoadPartitionState returns active consequence rows for one channel,
// converting wall-clock expiry to partition-local ticks (tick base 0 at
// start).
func (l loaderPort) LoadPartitionState(ctx context.Context, mapID string,
	channelID uint64) (runtime.PartitionState, error) {
	rows, err := l.w.cfg.Loader.LoadConsequences(ctx, mapID, channelID)
	if err != nil {
		return runtime.PartitionState{}, err
	}
	st := runtime.PartitionState{}
	now := l.w.cfg.Now()
	for _, r := range rows {
		if !r.Active {
			continue
		}
		var ticks uint64
		if d := r.ExpiresAt.Sub(now); d > 0 {
			ticks = uint64(d / (50 * time.Millisecond))
		}
		st.Consequences = append(st.Consequences, runtime.WorldConsequence{
			RelicID:       r.RelicID,
			Active:        true,
			ExpiresAtTick: ticks,
		})
	}
	return st, nil
}
