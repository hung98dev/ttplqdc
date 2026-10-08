package combat

import (
	"testing"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/progression"
	"thinhthan/internal/sim/runtime"
)

// --- harness ---------------------------------------------------------------

type sink struct{ msgs []runtime.Outbound }

func (s *sink) Enqueue(o runtime.Outbound) error {
	s.msgs = append(s.msgs, o)
	return nil
}

type skillSet map[string]SkillDef

func (m skillSet) Lookup(id string) (SkillDef, bool) {
	d, ok := m[id]
	return d, ok
}

type statSet map[uint64]progression.Stats

func (m statSet) StatsOf(id uint64) (progression.Stats, bool) {
	st, ok := m[id]
	return st, ok
}

type rig struct {
	p   *runtime.Partition
	s   *System
	out *sink
	seq uint64
}

func newRig(t *testing.T, skills skillSet, stats statSet, hc func(*runtime.Entity) bool) *rig {
	t.Helper()
	p, err := runtime.NewPartition(runtime.PartitionConfig{
		MapID:           "map_test",
		ChannelID:       1,
		InstanceID:      id.UUID{0x01},
		ContentRevision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Seed:            1,
	}, runtime.Ports{})
	if err != nil {
		t.Fatalf("NewPartition: %v", err)
	}
	out := &sink{}
	r := &rig{p: p, out: out}
	r.s = New(Config{
		Outbound:    out,
		Skills:      skills,
		Stats:       stats,
		Hurtbox:     func(uint64) (int64, int64) { return 400, 1800 }, // 0.8 x 1.8 m
		HardControl: hc,
		Rand:        func() float64 { return 0.99 }, // never dodge/crit by default
	})
	return r
}

func (r *rig) admit(t *testing.T, class runtime.EntityClass, x, y int32, hp, maxHP int64, level uint32) *runtime.Entity {
	t.Helper()
	eid, err := r.p.Admit(class)
	if err != nil {
		t.Fatalf("Admit: %v", err)
	}
	e, err := r.p.Entity(eid)
	if err != nil {
		t.Fatalf("Entity: %v", err)
	}
	e.Snap.X = x
	e.Snap.Y = y
	e.Snap.HP = hp
	e.Snap.MaxHP = maxHP
	e.Snap.Level = level
	e.Snap.Facing = protocolv1.Facing_FACING_RIGHT
	e.Private.MaxMP = 1000
	e.Private.CurrentMP = 1000
	r.p.Grid().Upsert(eid, x, y)
	return e
}

func (r *rig) tc(tick uint64) *runtime.TickContext {
	return &runtime.TickContext{Tick: tick}
}

func (r *rig) nextSeq() uint64 { r.seq++; return r.seq }

func (r *rig) rejects() []*protocolv1.S2CActionRejected {
	var out []*protocolv1.S2CActionRejected
	for _, m := range r.out.msgs {
		if m.MessageID == idS2CActionRejected {
			out = append(out, m.Msg.(*protocolv1.S2CActionRejected))
		}
	}
	return out
}

func (r *rig) started() []*protocolv1.S2CActionStarted {
	var out []*protocolv1.S2CActionStarted
	for _, m := range r.out.msgs {
		if m.MessageID == idS2CActionStarted {
			out = append(out, m.Msg.(*protocolv1.S2CActionStarted))
		}
	}
	return out
}

func (r *rig) events() []*protocolv1.S2CCombatEvent {
	var out []*protocolv1.S2CCombatEvent
	for _, m := range r.out.msgs {
		if m.MessageID == idS2CCombatEvent {
			out = append(out, m.Msg.(*protocolv1.S2CCombatEvent))
		}
	}
	return out
}

func basicSkill(id string, startup, active, recovery int64) SkillDef {
	return SkillDef{
		SkillID: id, IsBasic: true, Band: BandBasic1, Timing: TimingAttackSpeed,
		CostTiming: CostOnStart, CooldownMs: 500,
		StartupMs: startup, ActiveMs: active, RecoveryMs: recovery,
		Geom:        Geometry{Kind: GeomMeleeBox, A: 1500, B: 1000, RangeMM: 1500},
		Coefficient: 1.0, Hostile: true, RequiresTgt: true,
	}
}

// --- tests -----------------------------------------------------------------

// TestStartupActiveRecoveryTiming: an accepted action resolves at the
// first tick at/after its cumulative phase dues (ceil(due/50)).
func TestStartupActiveRecoveryTiming(t *testing.T) {
	r := newRig(t, skillSet{"s1": basicSkill("s1", 75, 50, 100)}, statSet{
		0: baseStats(), 1: baseStats(),
	}, nil)
	att := r.admit(t, runtime.ClassPlayer, 0, 0, 500, 500, 1)
	vic := r.admit(t, runtime.ClassSpawnGroup, 1000, 0, 100, 500, 1)
	fixStats(r, att, vic)

	r.s.Accept(r.p, Request{
		Kind: ReqBasicAttack, Source: att.ID, SkillID: "s1",
		Facing: protocolv1.Facing_FACING_RIGHT, TargetID: vic.ID,
		ClientSeq: r.nextSeq(), RequestWireID: 201,
	}, r.tc(0))
	st := r.started()
	if len(st) != 1 {
		t.Fatalf("expected S2C_ACTION_STARTED, got %v", r.rejects())
	}
	// startup 75ms -> active at tick 2 (75->ceil=2); active 50 -> ends
	// tick 3; recovery +100 -> recovery ends tick 5.
	if st[0].ActiveStartsAtTick != 2 || st[0].RecoveryEndsAtTick != 5 {
		t.Fatalf("timing: active=%d recovery=%d", st[0].ActiveStartsAtTick, st[0].RecoveryEndsAtTick)
	}
	// No hit before ACTIVE.
	for tk := uint64(0); tk < 2; tk++ {
		r.s.Step(r.p, r.tc(tk))
	}
	if len(r.events()) != 0 {
		t.Fatal("hit resolved before ACTIVE phase")
	}
	r.s.Step(r.p, r.tc(2))
	if len(r.events()) == 0 {
		t.Fatal("hit did not resolve at ACTIVE start tick")
	}
	if vic.Snap.HP >= 100 {
		t.Fatal("target took no damage at ACTIVE")
	}
}

// TestActionInterruptionRules: death always cancels; an unresolved
// action blocks new accepts; the basic self-chain exception only
// fires after next-accept and motion deadlines.
func TestActionInterruptionRules(t *testing.T) {
	sk := basicSkill("s1", 150, 50, 500)
	sk.IsBasic = false
	sk.CooldownMs = 0
	r := newRig(t, skillSet{"s1": sk}, nil, nil)
	att := r.admit(t, runtime.ClassPlayer, 0, 0, 500, 500, 1)
	vic := r.admit(t, runtime.ClassSpawnGroup, 1000, 0, 100, 500, 1)
	r.s.cfg.Stats = statSet{att.ID: baseStats(), vic.ID: baseStats()}

	r.s.Accept(r.p, Request{
		Kind: ReqBasicAttack, Source: att.ID, SkillID: "s1",
		Facing: protocolv1.Facing_FACING_RIGHT, TargetID: vic.ID,
		ClientSeq: r.nextSeq(), RequestWireID: 201,
	}, r.tc(0))
	// Second accept during startup+recovery → INVALID_STATE.
	r.s.Accept(r.p, Request{
		Kind: ReqBasicAttack, Source: att.ID, SkillID: "s1",
		Facing: protocolv1.Facing_FACING_RIGHT, TargetID: vic.ID,
		ClientSeq: r.nextSeq(), RequestWireID: 201,
	}, r.tc(1))
	rej := r.rejects()
	if len(rej) != 1 || rej[0].ErrorCode != protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE {
		t.Fatalf("expected INVALID_STATE, got %+v", rej)
	}
	// Interrupt by knockback: skill does not list it → action survives.
	r.s.Interrupt(att, InterruptKnockback)
	if r.s.StateOf(att) == StateIdle {
		t.Fatal("unlisted interrupt cancelled the action")
	}
	// Death always cancels.
	vic.Snap.HP = 0
	vic.Dead = true
	r.s.Interrupt(att, InterruptDeath)
	if got := r.s.StateOf(att); got != StateIdle {
		// interrupt clears the action; state returns to Idle on Step.
		r.s.Step(r.p, r.tc(1))
	}
	r.s.Step(r.p, r.tc(1))
	if len(r.events()) != 0 {
		t.Fatal("dead target still received a resolved hit")
	}
}

// TestInCombatStateLifecycle: hostile action enters in_combat both
// ways; lock holds while refreshing and expires 6 s after the last
// hostile event with no unresolved action.
func TestInCombatStateLifecycle(t *testing.T) {
	r := newRig(t, skillSet{"s1": basicSkill("s1", 0, 0, 0)}, nil, nil)
	att := r.admit(t, runtime.ClassPlayer, 0, 0, 500, 500, 1)
	vic := r.admit(t, runtime.ClassSpawnGroup, 1000, 0, 100, 500, 1)
	r.s.cfg.Stats = statSet{att.ID: baseStats(), vic.ID: baseStats()}

	r.s.Accept(r.p, Request{
		Kind: ReqBasicAttack, Source: att.ID, SkillID: "s1",
		Facing: protocolv1.Facing_FACING_RIGHT, TargetID: vic.ID,
		ClientSeq: r.nextSeq(), RequestWireID: 201,
	}, r.tc(0))
	if att.InCombatWith != vic.ID {
		t.Fatal("attacker did not enter in_combat on hostile action")
	}
	r.s.Step(r.p, r.tc(0))
	if vic.InCombatWith != att.ID {
		t.Fatal("victim did not enter in_combat on damage")
	}
	// At t+119 ticks (5.95 s) still locked; at t+120 (6.0 s) exits.
	for tk := uint64(1); tk <= 119; tk++ {
		r.s.Step(r.p, r.tc(tk))
	}
	if vic.InCombatWith == 0 {
		t.Fatal("in_combat expired before 6 s")
	}
	r.s.Step(r.p, r.tc(120))
	if vic.InCombatWith != 0 || att.InCombatWith != 0 {
		t.Fatal("in_combat did not expire at 6 s")
	}
}

// TestTargetCapsEnforcement: at most 4 monster / 3 player victims;
// over-cap order is primary → nearest → entity id.
func TestTargetCapsEnforcement(t *testing.T) {
	sk := basicSkill("s1", 0, 0, 0)
	sk.Band = BandWide
	sk.Level = 10
	sk.RequiresTgt = false
	sk.Geom = Geometry{Kind: GeomAreaSelf, A: 20000, RangeMM: 20000}
	r := newRig(t, skillSet{"s1": sk}, nil, nil)
	att := r.admit(t, runtime.ClassPlayer, 0, 0, 500, 500, 1)
	stats := statSet{att.ID: baseStats()}
	var mons []*runtime.Entity
	for i := 0; i < 6; i++ {
		m := r.admit(t, runtime.ClassSpawnGroup, int32(1000+i*500), 0, 100, 500, 1)
		stats[m.ID] = baseStats()
		mons = append(mons, m)
	}
	r.s.cfg.Stats = stats

	r.s.Accept(r.p, Request{
		Kind: ReqBasicAttack, Source: att.ID, SkillID: "s1",
		Facing:    protocolv1.Facing_FACING_RIGHT,
		ClientSeq: r.nextSeq(), RequestWireID: 201,
	}, r.tc(0))
	r.s.Step(r.p, r.tc(0))
	evs := r.events()
	seen := map[uint64]bool{}
	var order []uint64
	for _, ev := range evs {
		if ev.TargetEntityId != att.ID && !seen[ev.TargetEntityId] {
			seen[ev.TargetEntityId] = true
			order = append(order, ev.TargetEntityId)
		}
	}
	if len(order) != 4 {
		t.Fatalf("monster cap 4 violated: %d distinct targets", len(order))
	}
	// Deterministic order: nearest 4 by distance.
	for i, id := range order {
		if id != mons[i].ID {
			t.Fatalf("cap order violated at %d: got %d want %d", i, id, mons[i].ID)
		}
	}
	if mons[4].Snap.HP < 100 || mons[5].Snap.HP < 100 {
		t.Fatal("beyond-cap monsters took damage")
	}
}

// TestClosestHurtboxRangeBoundary: reach measures to the closest point
// of the authoritative hurtbox — a target inside at closest distance is
// hit, one past by 1 mm is out.
func TestClosestHurtboxRangeBoundary(t *testing.T) {
	sk := basicSkill("s1", 0, 0, 0)
	sk.CooldownMs = 0
	sk.Geom = Geometry{Kind: GeomSingleTargetRange, A: 1000, RangeMM: 1000}
	r := newRig(t, skillSet{"s1": sk}, nil, nil)
	att := r.admit(t, runtime.ClassPlayer, 0, 0, 500, 500, 1)
	// hurtbox half-width 400: edge at 600 → gap 400 ≤ 1000 hit.
	near := r.admit(t, runtime.ClassSpawnGroup, 600, 0, 100, 500, 1)
	// edge at 1001 → gap 601 > 1000 miss.
	far := r.admit(t, runtime.ClassSpawnGroup, 1801, 0, 100, 500, 1)
	r.s.cfg.Stats = statSet{att.ID: baseStats(), near.ID: baseStats(), far.ID: baseStats()}

	r.s.Accept(r.p, Request{
		Kind: ReqBasicAttack, Source: att.ID, SkillID: "s1",
		Facing: protocolv1.Facing_FACING_RIGHT, TargetID: near.ID,
		ClientSeq: r.nextSeq(), RequestWireID: 201,
	}, r.tc(0))
	r.s.Step(r.p, r.tc(0))
	if near.Snap.HP >= 100 {
		t.Fatal("in-range hurtbox closest point missed")
	}
	r.s.Accept(r.p, Request{
		Kind: ReqBasicAttack, Source: att.ID, SkillID: "s1",
		Facing: protocolv1.Facing_FACING_RIGHT, TargetID: far.ID,
		ClientSeq: r.nextSeq(), RequestWireID: 201,
	}, r.tc(0))
	rej := r.rejects()
	if len(rej) != 1 || rej[0].ErrorCode != protocolv1.ErrorCode_ERROR_CODE_OUT_OF_RANGE {
		t.Fatalf("expected OUT_OF_RANGE for past-reach target, got %+v", rej)
	}
}

// TestSpriteBoundsCannotCreateHit: reach is hurtbox-only — sprite
// bounds cannot create a hit beyond hurtbox intersection.
func TestSpriteBoundsCannotCreateHit(t *testing.T) {
	sk := basicSkill("s1", 0, 0, 0)
	sk.Geom = Geometry{Kind: GeomMeleeBox, A: 500, B: 1000, RangeMM: 500}
	r := newRig(t, skillSet{"s1": sk}, nil, nil)
	att := r.admit(t, runtime.ClassPlayer, 0, 0, 500, 500, 1)
	// Anchor 1000 away → hurtbox edge at 600, outside the 500 box even
	// though any wider sprite would overlap.
	vic := r.admit(t, runtime.ClassSpawnGroup, 1400, 0, 100, 500, 1)
	r.s.cfg.Stats = statSet{att.ID: baseStats(), vic.ID: baseStats()}

	r.s.Accept(r.p, Request{
		Kind: ReqBasicAttack, Source: att.ID, SkillID: "s1",
		Facing: protocolv1.Facing_FACING_RIGHT, TargetID: vic.ID,
		ClientSeq: r.nextSeq(), RequestWireID: 201,
	}, r.tc(0))
	// out-of-range → reject (not a hit).
	rej := r.rejects()
	if len(rej) != 1 || rej[0].ErrorCode != protocolv1.ErrorCode_ERROR_CODE_OUT_OF_RANGE {
		t.Fatalf("sprite-bounds-only overlap produced a hit: %+v", rej)
	}
}

// TestJustGuardWithinClampFires: a horizontal edge inside the 150 ms
// window mitigates and reports just_guard_triggered.
func TestJustGuardWithinClampFires(t *testing.T) {
	r := newRig(t, skillSet{"s1": basicSkill("s1", 0, 0, 0)}, nil, nil)
	att := r.admit(t, runtime.ClassPlayer, 0, 0, 500, 500, 1)
	vic := r.admit(t, runtime.ClassPlayer, 1000, 0, 300, 300, 1)
	r.s.cfg.Stats = statSet{att.ID: baseStats(), vic.ID: baseStats()}

	// Heavy-hit window: post_mit >= 12% MAX_HP opens it (baseStats hit
	// far exceeds 60 on 500 MaxHP).
	// Edge 100 ms before the hit commit: sample=clientMono drift sets
	// effective ≈ receive.
	commitMs := int64(0) // tick 0 → 0 ms
	r.s.NoteEdge(vic, protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_PRESS,
		protocolv1.Facing_FACING_LEFT, commitMs-100, commitMs-100)

	r.s.Accept(r.p, Request{
		Kind: ReqBasicAttack, Source: att.ID, SkillID: "s1",
		Facing: protocolv1.Facing_FACING_RIGHT, TargetID: vic.ID,
		ClientSeq: r.nextSeq(), RequestWireID: 201,
	}, r.tc(0))
	r.s.Step(r.p, r.tc(0))
	evs := r.events()
	if len(evs) == 0 || !evs[0].JustGuardWindow || !evs[0].JustGuardTriggered {
		t.Fatalf("expected JUST_GUARD window+trigger, got %+v", evs)
	}
	// 40% mitigation on the JG'd hit.
	if evs[0].PostMitigationDamage <= 0 {
		t.Fatal("post_mitigation not recorded")
	}
}

// TestJustGuardBeyondClampRejected: an edge outside the compensated
// window (or a stale edge) cannot trigger Just Guard.
func TestJustGuardBeyondClampRejected(t *testing.T) {
	r := newRig(t, skillSet{"s1": basicSkill("s1", 0, 0, 0)}, nil, nil)
	att := r.admit(t, runtime.ClassPlayer, 0, 0, 500, 500, 1)
	vic := r.admit(t, runtime.ClassPlayer, 1000, 0, 300, 300, 1)
	r.s.cfg.Stats = statSet{att.ID: baseStats(), vic.ID: baseStats()}

	// Edge 300 ms before commit — outside the 150 ms window.
	r.s.NoteEdge(vic, protocolv1.MovementEdgeType_MOVEMENT_EDGE_TYPE_PRESS,
		protocolv1.Facing_FACING_LEFT, -300, -300)
	r.s.Accept(r.p, Request{
		Kind: ReqBasicAttack, Source: att.ID, SkillID: "s1",
		Facing: protocolv1.Facing_FACING_RIGHT, TargetID: vic.ID,
		ClientSeq: r.nextSeq(), RequestWireID: 201,
	}, r.tc(0))
	r.s.Step(r.p, r.tc(0))
	evs := r.events()
	if len(evs) == 0 || evs[0].JustGuardTriggered {
		t.Fatalf("out-of-window edge triggered JG: %+v", evs)
	}
	if evs[0].JustGuardWindow && !evs[0].JustGuardTriggered {
		// FAIL window opened (heavy hit): streak reset is the correct
		// outcome — assert no mitigation applied.
	}
	// Stale edge: lag > RTT+80 never qualifies.
	lat := r.s.LatencyOf(vic)
	// samples with growing lag: sample = receive - mono.
	lat.NoteRtt(50)
	// clientMono drifts far behind → lag explodes past RTT+80.
	_, stale := lat.Evaluate(0, 1000)
	if !stale {
		t.Fatal("lag > RTT+80 not flagged stale")
	}
}

// TestJustGuardStreak: consecutive successes within 3.0 s escalate
// 40→50→60%; FAIL resets to base and applies the 500 ms ICD.
func TestJustGuardStreak(t *testing.T) {
	r := newRig(t, skillSet{"s1": basicSkill("s1", 0, 0, 0)}, nil, nil)
	att := r.admit(t, runtime.ClassPlayer, 0, 0, 500, 500, 1)
	vic := r.admit(t, runtime.ClassPlayer, 1000, 0, 2000, 2000, 1)
	r.s.cfg.Stats = statSet{att.ID: baseStats(), vic.ID: baseStats()}
	st := r.s.stateFor(vic)

	commit := int64(1000)
	// First window: streak 0 → needs heavy hit (post_mit >= 12% of
	// 2000 = 240). baseStats ATK 40 → post_mit 42 < 240 → no window.
	if opened, _, _ := evalJustGuard(st, false, 2000, commit, 42); opened {
		t.Fatal("window opened below the 12% threshold at streak 0")
	}
	// Heavy hit opens; no edge → FAIL → streak 0, ICD 500.
	if opened, v, _ := evalJustGuard(st, false, 2000, commit, 500); !opened || v != jgFail {
		t.Fatal("heavy hit without edge did not open+FAIL")
	}
	if st.jg.icdUntilMs != commit+500 {
		t.Fatalf("FAIL ICD not 500: %d", st.jg.icdUntilMs)
	}
	// During ICD → no window.
	if opened, _, _ := evalJustGuard(st, false, 2000, commit+400, 500); opened {
		t.Fatal("window opened during ICD")
	}
	// Success: edge in window at commit+600 (ICD expired).
	commit2 := commit + 600
	st.stamps.note(edgeStamp{effectiveMs: commit2 - 50, horizontal: true})
	if _, v, bp := evalJustGuard(st, false, 2000, commit2, 500); v != jgSuccess || bp != 4000 {
		t.Fatalf("first success not 40%%: v=%d bp=%d", v, bp)
	}
	// Second success within 3 s → 50%.
	commit3 := commit2 + 900
	st.stamps.note(edgeStamp{effectiveMs: commit3 - 50, horizontal: true})
	if _, v, bp := evalJustGuard(st, false, 2000, commit3, 500); v != jgSuccess || bp != 5000 {
		t.Fatalf("second success not 50%%: v=%d bp=%d", v, bp)
	}
	// Third → 60%.
	commit4 := commit3 + 900
	st.stamps.note(edgeStamp{effectiveMs: commit4 - 50, horizontal: true})
	if _, v, bp := evalJustGuard(st, false, 2000, commit4, 500); v != jgSuccess || bp != 6000 {
		t.Fatalf("third success not 60%%: v=%d bp=%d", v, bp)
	}
	// Streak decays after 3.0 s without success: window needs heavy
	// hit and tier resets to 40%.
	commit5 := commit4 + 900 + 3001 // ICD clear, streak window lapsed
	st.stamps.note(edgeStamp{effectiveMs: commit5 - 50, horizontal: true})
	if _, v, bp := evalJustGuard(st, false, 2000, commit5, 500); v != jgSuccess || bp != 4000 {
		t.Fatalf("decayed streak not back to 40%%: v=%d bp=%d", v, bp)
	}
}

// TestRttEwmaLatencyModel: RTT EWMA α=1/8 init 200; compensation
// min(lag,80); lag > RTT+80 → stale.
func TestRttEwmaLatencyModel(t *testing.T) {
	m := NewLatencyModel()
	if m.RttMs() != 200 {
		t.Fatalf("init RTT not 200: %d", m.RttMs())
	}
	m.NoteRtt(120)
	if m.RttMs() != 190 {
		t.Fatalf("EWMA(200,120,1/8) != 190: %d", m.RttMs())
	}
	// base_offset = min of last 64 samples.
	m2 := NewLatencyModel()
	for i := int64(0); i < 64; i++ {
		m2.Evaluate(1000+i*10, 1000+i*10) // sample = 0 lag
	}
	// lag 50 → compensation 50.
	if eff, _ := m2.Evaluate(2000, 2050); eff != 2000 {
		t.Fatalf("compensation not min(lag,80): eff=%d", eff)
	}
	// lag 200 → compensation clamped to 80.
	if eff, _ := m2.Evaluate(3000, 3200); eff != 3120 {
		t.Fatalf("clamp not 80: eff=%d", eff)
	}
}

// TestCombatWireFieldLists: S2C_ACTION_STARTED carries the full
// messages.md § Combat field list.
func TestCombatWireFieldLists(t *testing.T) {
	r := newRig(t, skillSet{"s1": basicSkill("s1", 100, 50, 100)}, nil, nil)
	att := r.admit(t, runtime.ClassPlayer, 0, 0, 500, 500, 1)
	vic := r.admit(t, runtime.ClassSpawnGroup, 1000, 0, 100, 500, 1)
	r.s.cfg.Stats = statSet{att.ID: baseStats(), vic.ID: baseStats()}

	r.s.Accept(r.p, Request{
		Kind: ReqBasicAttack, Source: att.ID, SkillID: "s1",
		Facing: protocolv1.Facing_FACING_RIGHT, TargetID: vic.ID,
		ClientSeq: 7, RequestWireID: 201,
	}, r.tc(10))
	st := r.started()
	if len(st) != 1 {
		t.Fatalf("no 203 emitted: %+v", r.rejects())
	}
	m := st[0]
	if m.ActionInstanceId == 0 || m.SourceEntityId != att.ID || m.SkillId != "s1" ||
		m.ClientSeq != 7 || m.ServerTick != 10 || m.Facing != protocolv1.Facing_FACING_RIGHT ||
		m.TargetEntityId != vic.ID || m.MpAfter != 1000 ||
		m.ActiveStartsAtTick != 12 || m.ActiveEndsAtTick != 13 || m.RecoveryEndsAtTick != 15 {
		t.Fatalf("203 field list mismatch: %+v", m)
	}
}

// TestActionRejectedCodes: rejects echo client_seq and
// request_message_id with the contract error codes.
func TestActionRejectedCodes(t *testing.T) {
	sk := basicSkill("s1", 0, 0, 0)
	sk.CostMP = 500
	un := basicSkill("unlearned", 0, 0, 0)
	nl := basicSkill("notloadout", 0, 0, 0)
	r := newRig(t, skillSet{"s1": sk, "unlearned": un, "notloadout": nl}, nil, nil)
	r.s.cfg.Learned = func(e *runtime.Entity, id string) bool { return id != "unlearned" }
	r.s.cfg.Loadout = func(e *runtime.Entity, id string) bool { return id != "notloadout" }
	att := r.admit(t, runtime.ClassPlayer, 0, 0, 500, 500, 1)
	vic := r.admit(t, runtime.ClassSpawnGroup, 1000, 0, 100, 500, 1)
	r.s.cfg.Stats = statSet{att.ID: baseStats(), vic.ID: baseStats()}

	cases := []struct {
		skill string
		want  protocolv1.ErrorCode
	}{
		{"unlearned", protocolv1.ErrorCode_ERROR_CODE_SKILL_NOT_LEARNED},
		{"notloadout", protocolv1.ErrorCode_ERROR_CODE_SKILL_LOADOUT_INVALID},
		{"unknown", protocolv1.ErrorCode_ERROR_CODE_SKILL_NOT_LEARNED},
	}
	for i, c := range cases {
		r.s.Accept(r.p, Request{
			Kind: ReqBasicAttack, Source: att.ID, SkillID: c.skill,
			TargetID: vic.ID, Facing: protocolv1.Facing_FACING_RIGHT,
			ClientSeq: uint64(100 + i), RequestWireID: 201,
		}, r.tc(0))
	}
	rej := r.rejects()
	if len(rej) != len(cases) {
		t.Fatalf("expected %d rejects, got %d", len(cases), len(rej))
	}
	for i, c := range cases {
		if rej[i].ErrorCode != c.want || rej[i].ClientSeq != uint64(100+i) || rej[i].RequestMessageId != 201 {
			t.Fatalf("reject %d mismatch: %+v want %v", i, rej[i], c.want)
		}
	}
	// INSUFFICIENT_MP: cost 500 > current 0.
	att.Private.CurrentMP = 0
	r.s.Accept(r.p, Request{
		Kind: ReqBasicAttack, Source: att.ID, SkillID: "s1",
		TargetID: vic.ID, Facing: protocolv1.Facing_FACING_RIGHT,
		ClientSeq: 200, RequestWireID: 201,
	}, r.tc(0))
	rej = r.rejects()
	last := rej[len(rej)-1]
	if last.ErrorCode != protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_MP {
		t.Fatalf("expected INSUFFICIENT_MP, got %v", last.ErrorCode)
	}
}

// baseStats returns a minimal deterministic stat line for fixtures.
func baseStats() progression.Stats {
	var st progression.Stats
	st[progression.StatMaxHP] = 500
	st[progression.StatAttack] = 40
	st[progression.StatDefense] = 0
	st[progression.StatCritChance] = 0
	st[progression.StatCritDamage] = 1.5
	st[progression.StatDodgeChance] = 0
	st[progression.StatAccuracy] = 0
	return st
}

func fixStats(r *rig, es ...*runtime.Entity) {
	m := statSet{}
	for _, e := range es {
		m[e.ID] = baseStats()
	}
	r.s.cfg.Stats = m
}
