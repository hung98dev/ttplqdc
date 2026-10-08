package world

import (
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/runtime"
)

// npcRangeOK checks the 2.5m session range between the player's entity
// and the NPC's entity.
func npcRangeOK(pe, ne *runtime.Entity) bool {
	dx := int64(pe.Snap.X) - int64(ne.Snap.X)
	dy := int64(pe.Snap.Y) - int64(ne.Snap.Y)
	return dx*dx+dy*dy <= NPCRangeMM*NPCRangeMM
}

// inCombat reads the entity's combat marker.
func inCombat(e *runtime.Entity) bool { return e.InCombatWith != 0 }

// answerNpcService validates a 103 consult (ADR-0083 read-only): live
// member entity, not dead, not in combat, range, open session for
// NPC_SERVICE, and a service admitted by the dispatcher and the NPC's
// own allow-set. The reply for set_checkpoint carries the resolved
// checkpoint payload fields.
func (h *ChannelHost) answerNpcService(con *Consult, p *runtime.Partition) {
	rej := func(code protocolv1.ErrorCode) {
		con.Reply <- ConsultReply{OK: false, Code: code}
	}
	pl, pe, ok := h.playerEntity(con.CharacterID, p)
	if !ok || pe.Snap.HP <= 0 || pe.Dead {
		rej(protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE)
		return
	}
	_ = pl
	if inCombat(pe) {
		rej(protocolv1.ErrorCode_ERROR_CODE_IN_COMBAT)
		return
	}
	npc, ok := h.npcs[con.TargetID]
	if !ok {
		rej(protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
		return
	}
	ne, err := p.Entity(npc.EntityID)
	if err != nil || !npcRangeOK(pe, ne) {
		rej(protocolv1.ErrorCode_ERROR_CODE_OUT_OF_RANGE)
		return
	}
	kind := protocolv1.InteractKind(con.InteractKind)
	if kind == protocolv1.InteractKind_INTERACT_KIND_TALK {
		con.Reply <- ConsultReply{OK: true, Tick: p.TickN()}
		return
	}
	if kind != protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE {
		rej(protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
		return
	}
	if !h.sessions.Valid(con.TargetID, con.CharacterID, p.TickN()) {
		rej(protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE)
		return
	}
	allowed := false
	for _, s := range npc.Def.Services {
		if s == con.ServiceID {
			allowed = true
			break
		}
	}
	if !allowed || !h.w.dispatcher.ServiceRegistered(con.ServiceID) {
		rej(protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
		return
	}
	if con.ServiceID == ServiceSetCheckpoint {
		rec, ok := h.w.cfg.Maps.Lookup(npc.Def.MapID)
		if !ok || len(rec.Checkpoints) == 0 {
			rej(protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
			return
		}
		cp := rec.Checkpoints[0]
		con.Reply <- ConsultReply{
			OK:           true,
			Tick:         p.TickN(),
			MapID:        cp.MapID,
			CheckpointID: cp.CheckpointID,
			AnchorID:     cp.AnchorID,
		}
		return
	}
	con.Reply <- ConsultReply{OK: true, Tick: p.TickN()}
}

// answerPortal validates a 104 consult: portal exists on this map,
// requirement level, range, not in combat, and destination capacity
// under the auto order.
func (h *ChannelHost) answerPortal(con *Consult, p *runtime.Partition) {
	rej := func(code protocolv1.ErrorCode) {
		con.Reply <- ConsultReply{OK: false, Code: code}
	}
	pl, pe, ok := h.playerEntity(con.CharacterID, p)
	if !ok || pe.Snap.HP <= 0 || pe.Dead {
		rej(protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE)
		return
	}
	_ = pl
	if inCombat(pe) {
		rej(protocolv1.ErrorCode_ERROR_CODE_IN_COMBAT)
		return
	}
	pd, ok := h.w.cfg.Maps.Portal(con.PortalID)
	if !ok || pd.SourceMap != h.mapID {
		rej(protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
		return
	}
	rec, ok := h.w.cfg.Maps.Lookup(h.mapID)
	if !ok {
		rej(protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE)
		return
	}
	a, ok := rec.Anchor(pd.AnchorID)
	if !ok {
		rej(protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
		return
	}
	dx := int64(pe.Snap.X) - a.X
	dy := int64(pe.Snap.Y) - a.Y
	if dx*dx+dy*dy > NPCRangeMM*NPCRangeMM {
		rej(protocolv1.ErrorCode_ERROR_CODE_OUT_OF_RANGE)
		return
	}
	if pd.RequireLevel > 0 && pe.Snap.Level < uint32(pd.RequireLevel) {
		rej(protocolv1.ErrorCode_ERROR_CODE_LEVEL_TOO_LOW)
		return
	}
	// Portals into non-world spaces (dungeons) are unusable here.
	if _, ok := h.w.cfg.Maps.Lookup(pd.DestMap); !ok {
		rej(protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
		return
	}
	// Read-only capacity preview under the auto order.
	if ch, ok := h.w.director.peekAuto(pd.DestMap); ok {
		con.Reply <- ConsultReply{OK: true, MapID: pd.DestMap, ChannelIndex: ch}
		return
	}
	rej(protocolv1.ErrorCode_ERROR_CODE_MAP_CAPACITY_FULL)
}

// answerChannel validates a 109 consult: index bounds, not same channel,
// switch cooldown, destination capacity.
func (h *ChannelHost) answerChannel(con *Consult, p *runtime.Partition) {
	rej := func(code protocolv1.ErrorCode) {
		con.Reply <- ConsultReply{OK: false, Code: code}
	}
	if con.TargetChannel == 0 || con.TargetChannel > ChannelsPerMap {
		rej(protocolv1.ErrorCode_ERROR_CODE_OUT_OF_RANGE)
		return
	}
	if con.TargetChannel == h.ch {
		rej(protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE)
		return
	}
	if _, _, ok := h.playerEntity(con.CharacterID, p); !ok {
		rej(protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE)
		return
	}
	if until, ok := h.w.director.switchCooldown(con.CharacterID); ok {
		_ = until
		rej(protocolv1.ErrorCode_ERROR_CODE_COOLDOWN_ACTIVE)
		return
	}
	dc, ok := h.w.director.Channel(h.mapID, con.TargetChannel)
	if !ok || dc.State != ChannelRunning || dc.Occupancy >= SoftCap {
		rej(protocolv1.ErrorCode_ERROR_CODE_MAP_CAPACITY_FULL)
		return
	}
	con.Reply <- ConsultReply{OK: true, ChannelIndex: con.TargetChannel}
}
