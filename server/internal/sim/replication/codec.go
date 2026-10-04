package replication

import (
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/aoi"
)

// Bounds for the pooled build arenas. They cover worst case: every field of
// every replicated entity changing in one delta.
const (
	MaxEntitiesPerView = aoi.MaxVisible
	MaxStatuses        = 16
	MaxCosmetics       = 8
	MaxEncounters      = 16
	MaxMechanics       = 8
)

// StatusEntry is the flat value form of pb.EntityStatus.
type StatusEntry struct {
	EffectID       string
	SourceEntityID uint64
	Stacks         uint32
	ExpiresAtTick  uint64
}

// CosmeticEntry is the flat value form of pb.EquippedCosmetic.
type CosmeticEntry struct {
	Slot       protocolv1.CosmeticSlot
	CosmeticID string
}

// MechanicSnapshot is the flat value form of pb.ActiveMechanicState minus the
// oneof payload, which encounter tasks populate.
type MechanicSnapshot struct {
	MechanicInstanceID uint64
	StartsAtTick       uint64
	EndsAtTick         uint64
}

// EncounterSnapshot is the flat value form of pb.EncounterState.
type EncounterSnapshot struct {
	EncounterID uint64
	ContentID   string
	PhaseNumber uint32
	MechanicN   int
	Mechanics   [MaxMechanics]MechanicSnapshot
}

// EntitySnapshot is the flat value form of pb.EntityState: every wire field
// except none — the replication baseline sends all 25 fields and the delta
// may change all but the identity four (entity_id, entity_kind, content_id,
// character_id).
type EntitySnapshot struct {
	ID            uint64
	Kind          protocolv1.EntityKind
	ContentID     string
	CharacterID   [16]byte
	DisplayName   string
	Level         uint32
	OwnerEntityID uint64
	X, Y          int32
	Vx, Vy        int32
	Facing        protocolv1.Facing
	MovementState protocolv1.MovementState
	HP, MaxHP     int64
	Shield        int64
	Flags         uint32
	StatusN       int
	Statuses      [MaxStatuses]StatusEntry
	CosmeticN     int
	Cosmetics     [MaxCosmetics]CosmeticEntry
	EncounterID   uint64
	Stats         [5]uint32 // lifesteal, reflect, absorb, heal_reduction, healing_received
}

// PrivateSnapshot is the flat value form of pb.SelfPrivateState.
type PrivateSnapshot struct {
	CurrentMP        int64
	MaxMP            int64
	AcceptedTargetID uint64
}

// CheckpointSnapshot is the flat value form of pb.MovementCheckpoint with
// EffectiveMovementParameters inlined.
type CheckpointSnapshot struct {
	X, Y                     int32
	Vx, Vy                   int32
	Facing                   protocolv1.Facing
	MovementState            protocolv1.MovementState
	PlatformID               uint64
	IsGrounded               bool
	JumpCount                uint32
	DropIgnorePlatformID     uint64
	DropIgnoreUntilTick      uint64
	HeldHorizontalIntent     protocolv1.HeldHorizontalIntent
	RunSpeedMmS              uint32
	FirstJumpMmS             uint32
	SecondJumpMmS            uint32
	GravityMmS2              uint32
	MaxFallMmS               uint32
	AirControlBp             uint32
	MaxStepHeightMm          uint32
}

// View is everything one replication client sees at one tick. The caller
// fills it from entity state; Entities holds the AOI-visible set sorted by
// entity id, self excluded.
type View struct {
	Tick                   uint64
	BaselineID             uint64
	Self                   EntitySnapshot
	SelfPrivate            PrivateSnapshot
	SelfCheckpoint         CheckpointSnapshot
	LastProcessedClientSeq uint64
	Entities               []EntitySnapshot
	Encounters             []EncounterSnapshot
}

// BaselineParams carries the non-entity fields of S2C_WORLD_BASELINE.
type BaselineParams struct {
	BaselineID      uint64
	ServerTick      uint64
	MapID           string
	ChannelIndex    uint32
	InstanceID      []byte // 16-byte partition incarnation id
	ContentRevision string
}

// Builder serializes snapshots into wire messages using pooled arenas. One
// Builder serves one replication client; every produced message borrows
// Builder storage and stays valid only until the next Reset or next build
// call of the same kind. The owner must enqueue/serialize all produced
// messages before building the next batch — the runtime emits them
// synchronously inside the BUILD phase, so the rule holds by construction.
type Builder struct {
	baseline protocolv1.S2CWorldBaseline
	spawn    [MaxEntitiesPerView]protocolv1.S2CEntitySpawn
	despawn  [MaxEntitiesPerView]protocolv1.S2CEntityDespawn
	deltaMsg protocolv1.S2CStateDelta
	resync   protocolv1.S2CBaselineResyncResult

	ent  [MaxEntitiesPerView]protocolv1.EntityState
	entP [MaxEntitiesPerView]*protocolv1.EntityState
	enc  [MaxEncounters]protocolv1.EncounterState
	encP [MaxEncounters]*protocolv1.EncounterState
	mech [MaxEncounters * MaxMechanics]protocolv1.ActiveMechanicState
	mechP [MaxEncounters * MaxMechanics]*protocolv1.ActiveMechanicState
	st   [MaxEntitiesPerView * MaxStatuses]protocolv1.EntityStatus
	stP  [MaxEntitiesPerView * MaxStatuses]*protocolv1.EntityStatus
	cs   [MaxEntitiesPerView * MaxCosmetics]protocolv1.EquippedCosmetic
	csP  [MaxEntitiesPerView * MaxCosmetics]*protocolv1.EquippedCosmetic

	delta  [MaxEntitiesPerView]protocolv1.EntityDelta
	deltaP [MaxEntitiesPerView]*protocolv1.EntityDelta
	i32    [MaxEntitiesPerView * 4]int32
	i64    [MaxEntitiesPerView*3 + 2]int64
	u32    [MaxEntitiesPerView * 7]uint32
	u64    [MaxEntitiesPerView*2 + 1]uint64
	str    [MaxEntitiesPerView]string
	fac    [MaxEntitiesPerView]protocolv1.Facing
	mv     [MaxEntitiesPerView]protocolv1.MovementState
	sl     [MaxEntitiesPerView]protocolv1.StatusList
	cl     [MaxEntitiesPerView]protocolv1.CosmeticList
	dSt    [MaxEntitiesPerView * MaxStatuses]protocolv1.EntityStatus
	dStP   [MaxEntitiesPerView * MaxStatuses]*protocolv1.EntityStatus
	dCs    [MaxEntitiesPerView * MaxCosmetics]protocolv1.EquippedCosmetic
	dCsP   [MaxEntitiesPerView * MaxCosmetics]*protocolv1.EquippedCosmetic

	self    protocolv1.EntityState
	selfP   protocolv1.SelfPrivateState
	selfPD  protocolv1.SelfPrivateDelta
	ack     protocolv1.SelfAck
	cp      protocolv1.MovementCheckpoint
	emp     protocolv1.EffectiveMovementParameters

	spawnN, despawnN, entN, encN, mechN, stN, csN int
	deltaN, i32N, i64N, u32N, u64N, strN, facN, mvN int
	slN, clN, dStN, dCsN int
}

// NewBuilder returns a ready pooled builder.
func NewBuilder() *Builder { return &Builder{} }

// Reset returns every arena to empty. Callers invoke it once per emitted
// batch before reusing the builder.
func (b *Builder) Reset() {
	b.spawnN, b.despawnN, b.entN, b.encN, b.mechN, b.stN, b.csN = 0, 0, 0, 0, 0, 0, 0
	b.deltaN, b.i32N, b.i64N, b.u32N, b.u64N, b.strN, b.facN, b.mvN = 0, 0, 0, 0, 0, 0, 0, 0
	b.slN, b.clN, b.dStN, b.dCsN = 0, 0, 0, 0
}

func (b *Builder) allocI32(v int32) *int32 {
	p := &b.i32[b.i32N]
	b.i32N++
	*p = v
	return p
}
func (b *Builder) allocI64(v int64) *int64 {
	p := &b.i64[b.i64N]
	b.i64N++
	*p = v
	return p
}
func (b *Builder) allocU32(v uint32) *uint32 {
	p := &b.u32[b.u32N]
	b.u32N++
	*p = v
	return p
}
func (b *Builder) allocU64(v uint64) *uint64 {
	p := &b.u64[b.u64N]
	b.u64N++
	*p = v
	return p
}
func (b *Builder) allocStr(v string) *string {
	p := &b.str[b.strN]
	b.strN++
	*p = v
	return p
}
func (b *Builder) allocFacing(v protocolv1.Facing) *protocolv1.Facing {
	p := &b.fac[b.facN]
	b.facN++
	*p = v
	return p
}
func (b *Builder) allocMove(v protocolv1.MovementState) *protocolv1.MovementState {
	p := &b.mv[b.mvN]
	b.mvN++
	*p = v
	return p
}

// fillStatus writes s into a pooled pb.EntityStatus.
func fillStatus(dst *protocolv1.EntityStatus, s *StatusEntry) {
	dst.EffectId = s.EffectID
	dst.SourceEntityId = s.SourceEntityID
	dst.Stacks = s.Stacks
	dst.ExpiresAtTick = s.ExpiresAtTick
}

func fillCosmetic(dst *protocolv1.EquippedCosmetic, s *CosmeticEntry) {
	dst.Slot = s.Slot
	dst.CosmeticId = s.CosmeticID
}

// fillEntity writes all 25 EntityState wire fields into dst from s.
func (b *Builder) fillEntity(dst *protocolv1.EntityState, s *EntitySnapshot) {
	dst.EntityId = s.ID
	dst.EntityKind = s.Kind
	dst.ContentId = s.ContentID
	dst.CharacterId = append(dst.CharacterId[:0], s.CharacterID[:]...)
	dst.DisplayName = s.DisplayName
	dst.Level = s.Level
	dst.OwnerEntityId = s.OwnerEntityID
	dst.XMm = s.X
	dst.YMm = s.Y
	dst.VxMmS = s.Vx
	dst.VyMmS = s.Vy
	dst.Facing = s.Facing
	dst.MovementState = s.MovementState
	dst.Hp = s.HP
	dst.MaxHp = s.MaxHP
	dst.Shield = s.Shield
	dst.Flags = s.Flags
	n := s.StatusN
	for i := 0; i < n; i++ {
		ent := &b.st[b.stN]
		b.stN++
		fillStatus(ent, &s.Statuses[i])
		b.stP[b.stN-1] = ent
	}
	dst.Statuses = b.stP[b.stN-n : b.stN]
	n = s.CosmeticN
	for i := 0; i < n; i++ {
		ent := &b.cs[b.csN]
		b.csN++
		fillCosmetic(ent, &s.Cosmetics[i])
		b.csP[b.csN-1] = ent
	}
	dst.EquippedCosmetics = b.csP[b.csN-n : b.csN]
	dst.EncounterId = s.EncounterID
	dst.StatLifesteal = s.Stats[0]
	dst.StatReflect = s.Stats[1]
	dst.StatAbsorb = s.Stats[2]
	dst.StatHealReduction = s.Stats[3]
	dst.StatHealingReceived = s.Stats[4]
}

// fillCheckpoint writes the complete MovementCheckpoint into dst.
func (b *Builder) fillCheckpoint(dst *protocolv1.MovementCheckpoint, c *CheckpointSnapshot) {
	dst.XMm = c.X
	dst.YMm = c.Y
	dst.VxMmS = c.Vx
	dst.VyMmS = c.Vy
	dst.Facing = c.Facing
	dst.MovementState = c.MovementState
	dst.PlatformId = c.PlatformID
	dst.IsGrounded = c.IsGrounded
	dst.JumpCount = c.JumpCount
	dst.DropIgnorePlatformId = c.DropIgnorePlatformID
	dst.DropIgnoreUntilTick = c.DropIgnoreUntilTick
	dst.HeldHorizontalIntent = c.HeldHorizontalIntent
	b.emp.RunSpeedMmS = c.RunSpeedMmS
	b.emp.FirstJumpMmS = c.FirstJumpMmS
	b.emp.SecondJumpMmS = c.SecondJumpMmS
	b.emp.GravityMmS2 = c.GravityMmS2
	b.emp.MaxFallMmS = c.MaxFallMmS
	b.emp.AirControlBp = c.AirControlBp
	b.emp.MaxStepHeightMm = c.MaxStepHeightMm
	dst.EffectiveParameters = &b.emp
}

// NewBaseline returns the pooled S2C_WORLD_BASELINE filled for v.
func (b *Builder) NewBaseline(v *View, p *BaselineParams) *protocolv1.S2CWorldBaseline {
	out := &b.baseline
	b.fillBaseline(v, p, out)
	return out
}

// Baseline fills out with the complete S2C_WORLD_BASELINE for v.
func (b *Builder) Baseline(v *View, p *BaselineParams, out *protocolv1.S2CWorldBaseline) {
	b.fillBaseline(v, p, out)
}

// fillBaseline writes the complete S2C_WORLD_BASELINE for v into out.
func (b *Builder) fillBaseline(v *View, p *BaselineParams, out *protocolv1.S2CWorldBaseline) {
	out.BaselineId = p.BaselineID
	out.ServerTick = p.ServerTick
	out.MapId = p.MapID
	out.ChannelIndex = p.ChannelIndex
	out.InstanceId = append(out.InstanceId[:0], p.InstanceID...)
	out.ContentRevision = p.ContentRevision
	b.fillEntity(&b.self, &v.Self)
	out.Self = &b.self
	b.selfP.CurrentMp = v.SelfPrivate.CurrentMP
	b.selfP.MaxMp = v.SelfPrivate.MaxMP
	b.selfP.AcceptedTargetEntityId = v.SelfPrivate.AcceptedTargetID
	out.SelfPrivate = &b.selfP
	b.fillCheckpoint(&b.cp, &v.SelfCheckpoint)
	out.SelfCheckpoint = &b.cp
	n := len(v.Entities)
	if n > MaxEntitiesPerView {
		n = MaxEntitiesPerView
	}
	for i := 0; i < n; i++ {
		ent := &b.ent[b.entN]
		b.entN++
		b.fillEntity(ent, &v.Entities[i])
		b.entP[b.entN-1] = ent
	}
	out.Entities = b.entP[b.entN-n : b.entN]
	m := len(v.Encounters)
	if m > MaxEncounters {
		m = MaxEncounters
	}
	for i := 0; i < m; i++ {
		enc := &b.enc[b.encN]
		b.encN++
		enc.EncounterId = v.Encounters[i].EncounterID
		enc.EncounterContentId = v.Encounters[i].ContentID
		enc.PhaseNumber = v.Encounters[i].PhaseNumber
		mn := v.Encounters[i].MechanicN
		if mn > MaxMechanics {
			mn = MaxMechanics
		}
		for j := 0; j < mn; j++ {
			me := &b.mech[b.mechN]
			b.mechN++
			me.MechanicInstanceId = v.Encounters[i].Mechanics[j].MechanicInstanceID
			me.StartsAtTick = v.Encounters[i].Mechanics[j].StartsAtTick
			me.EndsAtTick = v.Encounters[i].Mechanics[j].EndsAtTick
			b.mechP[b.mechN-1] = me
		}
		enc.ActiveMechanics = b.mechP[b.mechN-mn : b.mechN]
		b.encP[b.encN-1] = enc
	}
	out.Encounters = b.encP[b.encN-m : b.encN]
}

// NewSpawn returns a pooled S2C_ENTITY_SPAWN carrying the full entity state.
// It reports false when the per-build spawn arena is exhausted; that cannot
// happen when callers spawn at most the visible set.
func (b *Builder) NewSpawn(baselineID, tick uint64, s *EntitySnapshot) (*protocolv1.S2CEntitySpawn, bool) {
	if b.spawnN >= len(b.spawn) || b.entN >= len(b.ent) {
		return nil, false
	}
	msg := &b.spawn[b.spawnN]
	b.spawnN++
	msg.BaselineId = baselineID
	msg.ServerTick = tick
	ent := &b.ent[b.entN]
	b.entN++
	b.fillEntity(ent, s)
	msg.Entity = ent
	return msg, true
}

// NewDespawn returns a pooled S2C_ENTITY_DESPAWN.
func (b *Builder) NewDespawn(baselineID, tick, entityID uint64, reason protocolv1.DespawnReason) (*protocolv1.S2CEntityDespawn, bool) {
	if b.despawnN >= len(b.despawn) {
		return nil, false
	}
	msg := &b.despawn[b.despawnN]
	b.despawnN++
	msg.BaselineId = baselineID
	msg.ServerTick = tick
	msg.EntityId = entityID
	msg.Reason = reason
	return msg, true
}

// statusesEqual reports whether the previous and current status lists are
// identical — EntityDelta sends the whole list on any change.
func statusesEqual(p, c *EntitySnapshot) bool {
	if p.StatusN != c.StatusN {
		return false
	}
	for i := 0; i < c.StatusN; i++ {
		if p.Statuses[i] != c.Statuses[i] {
			return false
		}
	}
	return true
}

func cosmeticsEqual(p, c *EntitySnapshot) bool {
	if p.CosmeticN != c.CosmeticN {
		return false
	}
	for i := 0; i < c.CosmeticN; i++ {
		if p.Cosmetics[i] != c.Cosmetics[i] {
			return false
		}
	}
	return true
}

// fillDelta writes the field-wise diff of prev→cur into dst following
// ADR-0064: every mutable EntityState field is a proto3 optional that is
// absent when unchanged; identity fields are never sent; statuses and
// equipped_cosmetics are wrapped lists that replace fully when present.
func (b *Builder) fillDelta(dst *protocolv1.EntityDelta, prev, cur *EntitySnapshot) {
	dst.EntityId = cur.ID
	if prev.DisplayName != cur.DisplayName {
		dst.DisplayName = b.allocStr(cur.DisplayName)
	}
	if prev.Level != cur.Level {
		dst.Level = b.allocU32(cur.Level)
	}
	if prev.OwnerEntityID != cur.OwnerEntityID {
		dst.OwnerEntityId = b.allocU64(cur.OwnerEntityID)
	}
	if prev.X != cur.X {
		dst.XMm = b.allocI32(cur.X)
	}
	if prev.Y != cur.Y {
		dst.YMm = b.allocI32(cur.Y)
	}
	if prev.Vx != cur.Vx {
		dst.VxMmS = b.allocI32(cur.Vx)
	}
	if prev.Vy != cur.Vy {
		dst.VyMmS = b.allocI32(cur.Vy)
	}
	if prev.Facing != cur.Facing {
		dst.Facing = b.allocFacing(cur.Facing)
	}
	if prev.MovementState != cur.MovementState {
		dst.MovementState = b.allocMove(cur.MovementState)
	}
	if prev.HP != cur.HP {
		dst.Hp = b.allocI64(cur.HP)
	}
	if prev.MaxHP != cur.MaxHP {
		dst.MaxHp = b.allocI64(cur.MaxHP)
	}
	if prev.Shield != cur.Shield {
		dst.Shield = b.allocI64(cur.Shield)
	}
	if prev.Flags != cur.Flags {
		dst.Flags = b.allocU32(cur.Flags)
	}
	if prev.EncounterID != cur.EncounterID {
		dst.EncounterId = b.allocU64(cur.EncounterID)
	}
	if prev.Stats[0] != cur.Stats[0] {
		dst.StatLifesteal = b.allocU32(cur.Stats[0])
	}
	if prev.Stats[1] != cur.Stats[1] {
		dst.StatReflect = b.allocU32(cur.Stats[1])
	}
	if prev.Stats[2] != cur.Stats[2] {
		dst.StatAbsorb = b.allocU32(cur.Stats[2])
	}
	if prev.Stats[3] != cur.Stats[3] {
		dst.StatHealReduction = b.allocU32(cur.Stats[3])
	}
	if prev.Stats[4] != cur.Stats[4] {
		dst.StatHealingReceived = b.allocU32(cur.Stats[4])
	}
	if !statusesEqual(prev, cur) {
		sl := &b.sl[b.slN]
		b.slN++
		n := cur.StatusN
		for i := 0; i < n; i++ {
			ent := &b.dSt[b.dStN]
			b.dStN++
			fillStatus(ent, &cur.Statuses[i])
			b.dStP[b.dStN-1] = ent
		}
		sl.Entries = b.dStP[b.dStN-n : b.dStN]
		dst.Statuses = sl
	}
	if !cosmeticsEqual(prev, cur) {
		cl := &b.cl[b.clN]
		b.clN++
		n := cur.CosmeticN
		for i := 0; i < n; i++ {
			ent := &b.dCs[b.dCsN]
			b.dCsN++
			fillCosmetic(ent, &cur.Cosmetics[i])
			b.dCsP[b.dCsN-1] = ent
		}
		cl.Entries = b.dCsP[b.dCsN-n : b.dCsN]
		dst.EquippedCosmetics = cl
	}
}

// NewDelta returns the pooled S2C_STATE_DELTA for cur diffed against prev.
func (b *Builder) NewDelta(prev, cur *View) *protocolv1.S2CStateDelta {
	out := &b.deltaMsg
	b.Delta(prev, cur, out)
	return out
}

// Delta fills out with the S2C_STATE_DELTA for cur diffed against prev.
// ADR-0069: self_ack is always populated — last_processed_client_seq plus the
// complete MovementCheckpoint — even when nothing else changed.
// self_private is present only when one of its fields changed. Entities
// holds one EntityDelta per still-visible entity; entities that entered or
// left the visible set are carried by spawn/despawn, not by deltas.
func (b *Builder) Delta(prev, cur *View, out *protocolv1.S2CStateDelta) {
	out.BaselineId = cur.BaselineID
	out.ServerTick = cur.Tick

	b.WriteSelfAck(cur.LastProcessedClientSeq, &cur.SelfCheckpoint, &b.ack)
	out.SelfAck = &b.ack

	pp, cp := &prev.SelfPrivate, &cur.SelfPrivate
	if pp.CurrentMP != cp.CurrentMP || pp.MaxMP != cp.MaxMP || pp.AcceptedTargetID != cp.AcceptedTargetID {
		b.selfPD.CurrentMp = nil
		b.selfPD.MaxMp = nil
		b.selfPD.AcceptedTargetEntityId = nil
		if pp.CurrentMP != cp.CurrentMP {
			b.selfPD.CurrentMp = b.allocI64(cp.CurrentMP)
		}
		if pp.MaxMP != cp.MaxMP {
			b.selfPD.MaxMp = b.allocI64(cp.MaxMP)
		}
		if pp.AcceptedTargetID != cp.AcceptedTargetID {
			b.selfPD.AcceptedTargetEntityId = b.allocU64(cp.AcceptedTargetID)
		}
		out.SelfPrivate = &b.selfPD
	} else {
		out.SelfPrivate = nil
	}

	n := 0
	for i := range cur.Entities {
		c := &cur.Entities[i]
		var prevEnt *EntitySnapshot
		for j := range prev.Entities {
			if prev.Entities[j].ID == c.ID {
				prevEnt = &prev.Entities[j]
				break
			}
		}
		if prevEnt == nil {
			continue // new entrant: spawn carries its full state
		}
		d := &b.delta[b.deltaN]
		b.deltaN++
		b.fillDelta(d, prevEnt, c)
		b.deltaP[n] = d
		n++
	}
	out.Entities = b.deltaP[:n]
}

// NewResyncResult returns the pooled S2C_BASELINE_RESYNC_RESULT for one
// resync request.
func (b *Builder) NewResyncResult(requestID uint64, status protocolv1.ResultStatus, code protocolv1.ErrorCode, recoveryBaselineID uint64, retryAfterMs uint32) *protocolv1.S2CBaselineResyncResult {
	m := &b.resync
	m.RequestId = requestID
	m.Status = status
	m.ErrorCode = code
	m.RecoveryBaselineId = recoveryBaselineID
	m.RetryAfterMs = retryAfterMs
	return m
}
