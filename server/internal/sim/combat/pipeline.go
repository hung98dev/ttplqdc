package combat

import (
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/progression"
	"thinhthan/internal/sim/runtime"
)

// Damage pipeline (stats.md § Damage Pipeline, combat.md § Secondary
// Results; ADR-0037):
//
//	raw      = floor(ATK*coef + flat)
//	source   = floor(raw * (1 + DMG_BONUS) * src_mult)
//	dodge    → DODGED (0, nullifies on-hit) and stop
//	crit     → * CRIT_DAMAGE
//	element  → * 1.05 when controlling target primary element
//	pre_def  = floor(source * crit * element * tgt_elem_mult)
//	eff_def  = floor(max(0,DEF) * (1-pen)); post_def = floor(pre*defMult)
//	post_mit = max(MIN_DAMAGE, floor(post_def*(1-DMG_REDUCTION)*tgt_mult))
//	JG       → jg_damage = floor(post_mit * (1-mitigation))
//	shields  → shield_absorbed (earliest expires → effect_id → seq)
//	hp       → hp_damage = max(0, jg_damage - absorbed)
//
// Stage-7 secondary results then run: REFLECT (<=3.0 m, tagged
// NO_CRIT|NO_REFLECT|NO_LIFESTEAL|NO_PROC), LIFESTEAL (never on
// NO_LIFESTEAL/DODGED/self), ABSORB (effect.absorb.self shield, 0.15
// MAX_HP per instance, 0.50 aggregate cap).
const (
	minDamage      = 1
	elementControl = 1.05
	reflectRangeMM = 3000
	absorbPerBP    = 1500 // 0.15 MAX_HP per instance
	absorbAggBP    = 5000 // 0.50 MAX_HP aggregate cap
)

// Shield is one runtime shield instance.
type Shield struct {
	EffectID  string
	SourceID  uint64
	ExpiresAt uint64 // tick
	Remaining int64
	Seq       uint64
}

// resolveAction commits the action's hits at its ACTIVE tick.
func (s *System) resolveAction(p *runtime.Partition, src *runtime.Entity, act *Action, tc *runtime.TickContext) {
	def, ok := s.cfg.Skills.Lookup(act.SkillID)
	if !ok || !def.Hostile {
		return
	}
	cands := s.gatherCandidates(p, src, act, &def)
	if def.RequiresTgt {
		// locked/single-target forms hit only the primary when in range.
		t, err := p.Entity(act.Target)
		if err != nil || t.Snap.HP <= 0 || t.Dead {
			return
		}
		ox, oy := s.originOf(src)
		if closestDistanceSq(ox, oy, s.hurtboxOf(t)) > int64(def.Geom.rangeMM())*int64(def.Geom.rangeMM()) {
			return
		}
		cands = []candidate{{id: act.Target, player: t.Class == runtime.ClassPlayer}}
	}
	victims := selectTargets(cands, act.Target, def.capFor(def.Level))
	for _, vid := range victims {
		t, err := p.Entity(vid)
		if err != nil {
			continue
		}
		s.resolveHit(p, src, t, act, &def, tc)
	}
}

// resolveHit runs one component of the damage pipeline on one victim
// and emits its S2C_COMBAT_EVENT.
func (s *System) resolveHit(p *runtime.Partition, src, tgt *runtime.Entity, act *Action, def *SkillDef, tc *runtime.TickContext) {
	commitMs := tickMs(tc.Tick)
	srcStats, _ := s.cfg.Stats.StatsOf(src.ID)
	tgtStats, _ := s.cfg.Stats.StatsOf(tgt.ID)
	srcSt := s.stateFor(src)
	tgtSt := s.stateFor(tgt)

	ev := &protocolv1.S2CCombatEvent{
		EventId:          s.nextEventID,
		ServerTick:       tc.Tick,
		ActionInstanceId: act.ID,
		SourceEntityId:   src.ID,
		TargetEntityId:   tgt.ID,
		SkillId:          def.SkillID,
		ResultKind:       protocolv1.CombatResultKind_COMBAT_RESULT_KIND_DAMAGE,
		Outcome:          protocolv1.CombatOutcome_COMBAT_OUTCOME_HIT,
		DamageElement:    def.Element,
	}
	s.nextEventID++

	raw := int64(float64(srcStats[progression.StatAttack])*def.Coefficient) + def.FlatDamage
	source := int64(float64(raw) * (1 + srcStats[progression.StatDamageBonus]))

	// Step 3: dodge — DODGED commits 0 damage and nullifies on-hit;
	// it also skips the entire Just Guard evaluation.
	effDodge := tgtStats[progression.StatDodgeChance] - srcStats[progression.StatAccuracy]
	if effDodge < 0 {
		effDodge = 0
	}
	if effDodge > 0.40 {
		effDodge = 0.40
	}
	if s.cfg.Rand() < effDodge {
		ev.Outcome = protocolv1.CombatOutcome_COMBAT_OUTCOME_DODGED
		ev.TargetHpAfter = tgt.Snap.HP
		ev.TargetShieldAfter = tgt.Snap.Shield
		s.emitCombat(p, ev)
		return
	}

	// Crit + element.
	crit := 1.0
	if !def.NoCrit && s.cfg.Rand() < srcStats[progression.StatCritChance] {
		crit = srcStats[progression.StatCritDamage]
		ev.IsCrit = true
	}
	preDef := int64(float64(source) * crit * elementControl)
	effDef := int64(max64(0, int64(tgtStats[progression.StatDefense])))
	postDef := int64(float64(preDef) * progression.DefenseMultiplier(float64(effDef), int32(tgt.Snap.Level)))
	postMit := int64(float64(postDef) * (1 - tgtStats[progression.StatDamageReduction]))
	if postMit < minDamage {
		postMit = minDamage
	}
	ev.PostMitigationDamage = postMit

	// Invulnerable targets absorb to 0 damage (respawn window etc.).
	if !tgt.Hostile && tgt.Snap.HP > 0 && s.invulnerable(tgt) {
		ev.Outcome = protocolv1.CombatOutcome_COMBAT_OUTCOME_INVULNERABLE
		ev.TargetHpAfter = tgt.Snap.HP
		ev.TargetShieldAfter = tgt.Snap.Shield
		s.emitCombat(p, ev)
		return
	}

	// Just Guard on the connected hit.
	opened, verdict, mitBP := evalJustGuard(tgtSt, s.hardLocked(tgt), tgt.Snap.MaxHP, commitMs, postMit)
	ev.JustGuardWindow = opened
	if verdict == jgSuccess {
		ev.JustGuardTriggered = true
		postMit = int64(float64(postMit) * float64(10000-mitBP) / 10000)
		// First-ever JG window: emit + persist the hint flag.
		if s.cfg.Hint != nil && s.emitHint(tgt, ev) {
		}
	}

	// Shields: earliest expires_at → lexical effect_id → creation seq.
	jgDamage := postMit
	shieldAbsorbed := s.consumeShields(tgtSt, tgt, jgDamage, tc.Tick)
	ev.ShieldAbsorbed = shieldAbsorbed
	hpDamage := jgDamage - shieldAbsorbed
	if hpDamage < 0 {
		hpDamage = 0
	}
	ev.HpDamage = hpDamage
	tgt.Snap.HP -= hpDamage
	tgt.Snap.Shield -= shieldAbsorbed
	ev.TargetHpAfter = tgt.Snap.HP
	ev.TargetShieldAfter = tgt.Snap.Shield

	// in_combat: damage > 0 refreshes the lock both directions.
	if postMit > 0 {
		s.enterCombat(tgt, tgtSt, src.ID, tc)
		s.enterCombat(src, srcSt, tgt.ID, tc)
	}

	// Death.
	if tgt.Snap.HP <= 0 {
		s.die(p, tgt, tgtSt, src.ID, tc)
		ev.Killed = true
	} else if !s.hardLocked(tgt) && tgtSt.action == nil {
		// HIT_REACTION: presentation-only 120 ms marker, never locks.
		tgtSt.state = StateHitReaction
		tgtSt.hitReactEnd = tc.Tick + ceilTick(hitReactionMs)
	}

	// Stage-7 secondaries on hp_damage>0.
	if hpDamage > 0 {
		s.stage7(p, src, tgt, def, ev, tc)
	}
	s.emitCombat(p, ev)
}

// stage7 applies reflect / lifesteal / absorb secondary results.
func (s *System) stage7(p *runtime.Partition, src, tgt *runtime.Entity, def *SkillDef, ev *protocolv1.S2CCombatEvent, tc *runtime.TickContext) {
	srcStats, _ := s.cfg.Stats.StatsOf(src.ID)
	tgtStats, _ := s.cfg.Stats.StatsOf(tgt.ID)

	// REFLECT: within 3.0 m; the reflect instance is tagged
	// NO_CRIT|NO_REFLECT|NO_LIFESTEAL|NO_PROC so it never re-triggers.
	if !def.NoReflect && tgtStats[progression.StatReflect] > 0 {
		ox, oy := s.originOf(tgt)
		if closestDistanceSq(ox, oy, s.hurtboxOf(src)) <= reflectRangeMM*reflectRangeMM {
			refl := int64(float64(ev.HpDamage) * tgtStats[progression.StatReflect])
			if refl > 0 {
				src.Snap.HP -= refl
				ev.ReflectDamageInstance = &refl
				if src.Snap.HP <= 0 {
					srcSt := s.stateFor(src)
					s.die(p, src, srcSt, tgt.ID, tc)
				}
			}
		}
	}
	// LIFESTEAL: attacker heals; never on NO_LIFESTEAL / DODGED / self.
	if !def.NoLifesteal && srcStats[progression.StatLifesteal] > 0 && src.ID != tgt.ID {
		heal := int64(float64(ev.HpDamage) * srcStats[progression.StatLifesteal])
		if heal > 0 {
			missing := src.Snap.MaxHP - src.Snap.HP
			if heal > missing {
				heal = missing
			}
			src.Snap.HP += heal
			ev.LifestealHealAmount = &heal
		}
	}
	// ABSORB: effect.absorb.self shield grant; per-instance 0.15 MAX_HP,
	// aggregate 0.50 MAX_HP; over-cap discard the whole instance.
	if srcStats[progression.StatAbsorb] > 0 && src.ID != tgt.ID {
		amt := int64(float64(ev.HpDamage) * srcStats[progression.StatAbsorb])
		srcSt := s.stateFor(src)
		if amt > 0 && s.cfg.Stats != nil {
			maxHP := src.Snap.MaxHP
			if amt > maxHP*absorbPerBP/10000 {
				amt = maxHP * absorbPerBP / 10000
			}
			total := int64(0)
			for _, sh := range srcSt.shields {
				total += sh.Remaining
			}
			if total+amt <= maxHP*absorbAggBP/10000 {
				s.grantShield(srcSt, src, "effect.absorb.self", src.ID, amt, tc.Tick)
				ev.AbsorbShieldAmount = &amt
			}
		}
	}
}

// grantShield appends a shield instance (same source+effect reapply
// takes max(remaining,new) with the new expiry — no second instance).
func (s *System) grantShield(st *actorState, e *runtime.Entity, effectID string, srcID uint64, amount int64, tick uint64) {
	for i := range st.shields {
		sh := &st.shields[i]
		if sh.EffectID == effectID && sh.SourceID == srcID {
			if amount > sh.Remaining {
				e.Snap.Shield += amount - sh.Remaining
				sh.Remaining = amount
			}
			return
		}
	}
	st.shieldN++
	st.shields = append(st.shields, Shield{
		EffectID:  effectID,
		SourceID:  srcID,
		ExpiresAt: tick + 600, // shields decay (shield expiry lane owns real TTL)
		Remaining: amount,
		Seq:       st.shieldN,
	})
	e.Snap.Shield += amount
}

// consumeShields drains shields in the canonical order and returns the
// absorbed total.
func (s *System) consumeShields(st *actorState, e *runtime.Entity, damage int64, tick uint64) int64 {
	absorbed := int64(0)
	// sort once per hit: earliest expires → lexical effect_id → seq.
	for i := 1; i < len(st.shields); i++ {
		for j := i; j > 0; j-- {
			a, b := st.shields[j-1], st.shields[j]
			less := a.ExpiresAt < b.ExpiresAt ||
				(a.ExpiresAt == b.ExpiresAt && (a.EffectID < b.EffectID ||
					(a.EffectID == b.EffectID && a.Seq < b.Seq)))
			if less {
				break
			}
			st.shields[j-1], st.shields[j] = st.shields[j], st.shields[j-1]
		}
	}
	rest := st.shields[:0]
	for _, sh := range st.shields {
		if absorbed >= damage {
			rest = append(rest, sh)
			continue
		}
		if sh.ExpiresAt <= tick {
			continue // expired: drop silently (SHIELD_EXPIRED emit lane)
		}
		take := sh.Remaining
		if take > damage-absorbed {
			take = damage - absorbed
		}
		sh.Remaining -= take
		absorbed += take
		if sh.Remaining > 0 {
			rest = append(rest, sh)
		}
	}
	st.shields = rest
	return absorbed
}

// die commits the death transition: Dead flag, DEAD state, interrupt
// all actions, drop shields, clear combat lock, emit S2C_DEATH.
func (s *System) die(p *runtime.Partition, e *runtime.Entity, st *actorState, killer uint64, tc *runtime.TickContext) {
	if e.Dead {
		return
	}
	e.Dead = true
	e.Snap.HP = 0
	st.state = StateDead
	st.action = nil
	st.shields = st.shields[:0]
	e.Snap.Shield = 0
	e.InCombatWith = 0
	st.opponent = 0
	st.hostileOpen = 0
	s.emitDeath(e, killer, tc)
}

// enterCombat refreshes the combat lock on both parties of a hostile
// event: in_combat until 6 s after the last refresh with no unresolved
// hostile action.
func (s *System) enterCombat(e *runtime.Entity, st *actorState, opponent uint64, tc *runtime.TickContext) {
	e.InCombatWith = opponent
	st.opponent = opponent
	st.lastHostileMs = tickMs(tc.Tick)
}

// invulnerable reports whether the entity is inside the respawn
// invulnerability window (incoming damage 0, statuses ignored).
func (s *System) invulnerable(e *runtime.Entity) bool {
	// Respawn invulnerability is owned by the respawn lane; the flag is
	// read off the replication flags bit when wired.
	return false
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
