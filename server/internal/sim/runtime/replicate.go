package runtime

import (
	"context"
	"time"

	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/aoi"
	"thinhthan/internal/sim/replication"

	"google.golang.org/protobuf/proto"
)

// deltaInterval is the default 10 Hz replication cadence in 20 Hz ticks.
const deltaInterval = 2

// Info implements aoi.Source for the partition entity table.
func (p *Partition) Info(viewerID, id uint64) (aoi.EntityInfo, bool) {
	e := p.entityByID(id)
	if e == nil {
		return aoi.EntityInfo{}, false
	}
	ve := p.entityByID(viewerID)
	var party bool
	if ve != nil && ve.PartyID != 0 && ve.PartyID == e.PartyID {
		party = true
	}
	return aoi.EntityInfo{
		X:         e.Snap.X,
		Y:         e.Snap.Y,
		Party:     party,
		Objective: e.Objective,
		InCombat:  e.InCombatWith == viewerID,
		Hostile:   e.Hostile,
	}, true
}

// SetPosition moves one entity on the AOI grid. Entities are invisible to
// replication until positioned.
func (p *Partition) SetPosition(id uint64, x, y int32) error {
	e := p.entityByID(id)
	if e == nil {
		return ErrUnknownEntity
	}
	e.Snap.X = x
	e.Snap.Y = y
	p.grid.Upsert(id, x, y)
	return nil
}

// QueueCommand stages a durable command for the next EMIT_DURABLE_COMMANDS
// phase. Commands are emitted in queue order; a full buffer reports
// ErrOverload rather than dropping or reordering.
func (p *Partition) QueueCommand(cmd DurableCommand) error {
	if p.pendingN >= len(p.pending) {
		return ErrOverload
	}
	p.pending[p.pendingN] = cmd
	p.pendingN++
	return nil
}

// removedReason maps a removed-log entry to the wire despawn reason.
func removedReason(code uint8) protocolv1.DespawnReason {
	switch code {
	case removedDied:
		return protocolv1.DespawnReason_DESPAWN_REASON_DIED
	case removedTransferred:
		return protocolv1.DespawnReason_DESPAWN_REASON_TRANSFERRED
	default:
		return protocolv1.DespawnReason_DESPAWN_REASON_REMOVED
	}
}

// emit posts one outbound replication message tagged with its delivery
// class and supersede key for the edge queue. Enqueue errors are swallowed:
// the edge owns outbound capacity and the sim never blocks on it.
func (p *Partition) emit(to uint64, msg proto.Message) {
	if p.ports.Outbound == nil {
		return
	}
	mid, ok := replication.MessageID(msg)
	if !ok {
		return
	}
	_, key, hasKey := replication.SupersedeKey(msg)
	_ = p.ports.Outbound.Enqueue(Outbound{
		To:           to,
		MessageID:    mid,
		Class:        replication.ClassOf(msg),
		SupersedeKey: key,
		HasKey:       hasKey,
		Msg:          msg,
	})
}

// buildReplication is the BUILD_REPLICATION_STATE_AND_EVENTS phase: for
// every attached player client it recomputes AOI interest, emits lifecycle
// events for membership changes, services baseline/resync flow and emits
// the 10 Hz state delta.
func (p *Partition) buildReplication(tc *TickContext) {
	for slot := 0; slot < PlayersCap; slot++ {
		if !p.used[slot] {
			continue
		}
		e := &p.ent[slot]
		c := &p.clients[slot]
		if !c.attached {
			continue
		}
		b := c.builder
		b.Reset()

		set := c.viewer.Recompute(p.grid, p, e.Snap.X, e.Snap.Y, &p.scratch)

		for _, id := range set.Leave {
			reason := protocolv1.DespawnReason_DESPAWN_REASON_LEFT_AOI
			for k := 0; k < p.removedN; k++ {
				if p.removed[k].ID == id {
					reason = removedReason(p.removed[k].Reason)
					break
				}
			}
			if msg, ok := b.NewDespawn(c.baselineID, tc.Tick, id, reason); ok {
				p.emit(e.ID, msg)
				c.prevDel(id)
			}
		}
		for _, id := range set.Shed {
			if msg, ok := b.NewDespawn(c.baselineID, tc.Tick, id, protocolv1.DespawnReason_DESPAWN_REASON_SHED); ok {
				p.emit(e.ID, msg)
				c.prevDel(id)
			}
		}
		for _, id := range set.Enter {
			se := p.entityByID(id)
			if se == nil {
				continue
			}
			if msg, ok := b.NewSpawn(c.baselineID, tc.Tick, &se.Snap); ok {
				p.emit(e.ID, msg)
				c.prevSet(&se.Snap)
			}
		}

		curN := 0
		for _, id := range set.Visible {
			se := p.entityByID(id)
			if se == nil || curN >= replication.MaxEntitiesPerView {
				continue
			}
			c.curEnt[curN] = se.Snap
			curN++
		}
		curView := replication.View{
			Tick:                   tc.Tick,
			BaselineID:             c.baselineID,
			Self:                   e.Snap,
			SelfPrivate:            e.Private,
			SelfCheckpoint:         e.Checkpoint,
			LastProcessedClientSeq: e.lastClientSeq,
			Entities:               c.curEnt[:curN],
			Encounters:             nil,
		}

		if c.resyncPending {
			c.baselineID = p.nextBaseline
			p.nextBaseline++
			res := b.NewResyncResult(c.resyncReq,
				protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
				protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED,
				c.baselineID, 0)
			p.emit(e.ID, res)
			c.resyncPending = false
			c.pendingBaseline = true
			c.baselineAcked = false
		}
		if c.pendingBaseline {
			msg := b.NewBaseline(&curView, &replication.BaselineParams{
				BaselineID:      c.baselineID,
				ServerTick:      tc.Tick,
				MapID:           p.cfg.MapID,
				ChannelIndex:    uint32(p.cfg.ChannelID),
				InstanceID:      p.cfg.InstanceID[:],
				ContentRevision: p.cfg.ContentRevision,
			})
			p.emit(e.ID, msg)
			c.pendingBaseline = false
			c.baselineAcked = false
			c.prevN = 0
			for i := 0; i < curN; i++ {
				c.prevSet(&c.curEnt[i])
			}
			c.prevPriv = e.Private
		}
		if c.baselineAcked && tc.Tick%deltaInterval == 0 {
			prevView := replication.View{
				Tick:        tc.Tick,
				BaselineID:  c.baselineID,
				SelfPrivate: c.prevPriv,
				Entities:    c.prevEnt[:c.prevN],
			}
			msg := b.NewDelta(&prevView, &curView)
			p.emit(e.ID, msg)
			copy(c.prevEnt[:], c.curEnt[:curN])
			c.prevN = curN
			c.prevPriv = e.Private
		}
	}
}

// emitMetrics records the tick-budget measurements: tick runtime, mean
// mailbox queue wait and the rejected-intent count. Nil-label emits stay on
// the instrument's allocation-free path (capacity.md HOT-001).
func (p *Partition) emitMetrics(start time.Duration, qWaitSum, qWaitN int64) {
	ctx := context.Background()
	if p.tickRuntime != nil {
		p.tickRuntime.Record(ctx, int64(p.cfg.Now()-start), nil)
	}
	if p.queueWait != nil && qWaitN > 0 {
		p.queueWait.Record(ctx, qWaitSum/qWaitN, nil)
	}
	if p.intentsRejected != nil && p.rejectedN > 0 {
		p.intentsRejected.Add(ctx, int64(p.rejectedN), nil)
	}
}
