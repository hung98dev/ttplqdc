package effects

import (
	"testing"

	"thinhthan/internal/sim/replication"
	"thinhthan/internal/sim/skills"
)

// Named tests map 1:1 to the IMP-016 acceptance lines of
// docs/09_testing/gameplay.md § Status Tests + § New-Stat Combat
// Tests. Ticks are 50 ms (20 Hz).

func fighter(id uint64) StatsView {
	return StatsView{EntityID: id, Attack: 5000, MaxHP: 20000, Defense: 800}
}

func kind(results []Result, k ResultKind) *Result {
	for i := range results {
		if results[i].Kind == k {
			return &results[i]
		}
	}
	return nil
}

// TestDamageResolutionOrder — combat.md § Secondary Results +
// stats.md resolutions on committed hp_damage: shield absorb precedes
// hp damage; REFLECT 3.0m gate + 0.03*MAX_HP cap; LIFESTEAL rolling
// HPS cap + 0.30 AoE factor + HEAL_REDUCTION applies; ABSORB
// per-instance/aggregate caps, no HEAL_REDUCTION.
func TestDamageResolutionOrder(t *testing.T) {
	s := New(nil)
	now := uint64(10)
	attacker := fighter(1)
	defender := fighter(2)
	defender.Stats[StatReflect] = 1000  // 10%
	attacker.Stats[StatLifesteal] = 500 // 5%
	attacker.Stats[StatAbsorb] = 1000   // 10%
	attacker.Stats[StatHealReduction] = 2500

	// Grant defender a shield — absorb precedes hp damage.
	out := s.Apply(attacker, TargetView{2}, Request{
		EffectID: "effect.skill.tho.thien_son_tran_shield",
	}, Ctx{Now: now})
	if kind(out, ResultShieldGranted) == nil {
		t.Fatalf("shield not granted: %+v", out)
	}
	// Defender's own MAX_HP scales the shield: 0.15 * 20000 = 3000.
	if got := s.ShieldTotal(2, now); got != 3000 {
		t.Fatalf("shield capacity = %d, want 3000", got)
	}
	absorbed, res := s.ConsumeShields(2, 4000, now)
	if absorbed != 3000 {
		t.Fatalf("absorbed = %d, want 3000", absorbed)
	}
	if kind(res, ResultShieldBroken) == nil {
		t.Fatal("shield depleted without SHIELD_BROKEN")
	}
	hpDamage := int64(4000 - 3000)

	// REFLECT: within 3.0m gate -> reflects 10% of committed damage.
	r := Reflect(attacker, defender, hpDamage, 2500)
	if r == nil || r.Amount != 100 {
		t.Fatalf("reflect = %+v, want 100", r)
	}
	if Reflect(attacker, defender, hpDamage, 3001) != nil {
		t.Fatal("reflect beyond 3.0m gate")
	}
	// Per-hit cap floor(0.03 * defender MAX_HP) = 600.
	defender.Stats[StatReflect] = 5000 // 50%
	if r = Reflect(attacker, defender, hpDamage, 0); r.Amount != 500 {
		t.Fatalf("reflect = %d, want 500 (50%% of 1000)", r.Amount)
	}
	if r = Reflect(attacker, defender, 100000, 0); r.Amount != 600 {
		t.Fatalf("reflect cap = %d, want 600", r.Amount)
	}

	// LIFESTEAL: 5% of hp_damage = 50; heals attacker.
	w := NewLifestealWindow()
	if heal := Lifesteal(w, attacker, hpDamage, false, now); heal != 50 {
		t.Fatalf("lifesteal = %d, want 50", heal)
	}
	// AoE/DoT factor 0.30: 5%*0.30 of 1000 = 15.
	if heal := Lifesteal(w, attacker, hpDamage, true, now+1); heal != 15 {
		t.Fatalf("aoe lifesteal = %d, want 15", heal)
	}
	// Rolling 1.0s HPS cap = 0.015 * 20000 = 300 — discard excess.
	big := StatsView{EntityID: 9, MaxHP: 20000}
	big.Stats[StatLifesteal] = 10000 // 100%
	w2 := NewLifestealWindow()
	if h := Lifesteal(w2, big, 5000, false, now); h != 300 {
		t.Fatalf("lifesteal HPS cap = %d, want 300", h)
	}
	if h := Lifesteal(w2, big, 5000, false, now); h != 0 {
		t.Fatalf("lifesteal beyond window = %d, want 0", h)
	}
	// After the window rolls (20 ticks), capacity returns.
	if h := Lifesteal(w2, big, 5000, false, now+lifestealWindowTicks); h != 300 {
		t.Fatalf("lifesteal after window = %d, want 300", h)
	}
	// Lifesteal IS subject to HEAL_REDUCTION on the healed target:
	// combined multiplier is applied by the heal path (HealingReceived).
	s.Apply(attacker, TargetView{2}, *HealReductionRequest(attacker, hpDamage), Ctx{Now: now})
	hr := s.HealingReceived(2, now)
	if hr != 0.75 {
		t.Fatalf("healing received = %v, want 0.75", hr)
	}

	// ABSORB: 10% of post-mitigation -> shield on attacker; per-hit
	// cap 0.15 * MAX_HP = 3000; NOT subject to HEAL_REDUCTION.
	a := Absorb(attacker, 5000, 0)
	if a == nil || a.Amount != 500 {
		t.Fatalf("absorb = %+v, want 500", a)
	}
	if a = Absorb(attacker, 100000, 0); a.Amount != 3000 {
		t.Fatalf("absorb per-instance cap = %d, want 3000", a.Amount)
	}
	// Aggregate cap 0.50 * MAX_HP = 10000, discard-in-full.
	if a = Absorb(attacker, 100000, 9000); a != nil {
		t.Fatalf("absorb aggregate cap not discard-in-full: %+v", a)
	}
	// Grant through the engine: absorb shield 6.0s on attacker.
	a = Absorb(attacker, 5000, 0)
	out = s.Apply(attacker, TargetView{1}, *a.Request, Ctx{Now: now})
	if kind(out, ResultShieldGranted) == nil {
		t.Fatal("absorb shield not granted")
	}
	if got := s.ShieldTotal(1, now); got != 500 {
		t.Fatalf("absorb pool = %d, want 500", got)
	}
}

// TestShieldAbsorptionLifecycle — grant, consume order (earliest
// expires_at -> lexical effect_id -> seq), broken vs expired vs
// removed, ward linkage.
func TestShieldAbsorptionLifecycle(t *testing.T) {
	s := New(nil)
	src := fighter(1)
	tgt := TargetView{EntityID: 2}
	ctx := Ctx{Now: 100}

	// Grant two shields; tho_giap (6s) later expiry than
	// thien_son_tran (also 6s but lexical + later seq).
	s.Apply(src, tgt, Request{EffectID: "effect.skill.tho.tho_giap_shield", ShieldAmount: 400}, ctx)
	s.Apply(src, tgt, Request{EffectID: "effect.skill.tho.thien_son_tran_shield", ShieldAmount: 300}, ctx)

	// Ward linked to tho_giap_shield rides its lifetime.
	s.Apply(src, tgt, Request{EffectID: "effect.skill.tho.tho_giap_ward"}, ctx)
	if !s.Book(2).HasTag(TagDisplacementImmune, 100) {
		t.Fatal("ward did not confer DISPLACEMENT_IMMUNE")
	}

	// Consume order at equal expiry: lexical effect_id —
	// "thien_son_tran" < "tho_giap" ('i' < 'o') -> thien_son drains
	// first and breaks at its 300 capacity.
	absorbed, res := s.ConsumeShields(2, 450, 100)
	if absorbed != 450 {
		t.Fatalf("absorbed = %d, want 450", absorbed)
	}
	var broken *Result
	for i := range res {
		if res[i].Kind == ResultShieldBroken {
			broken = &res[i]
		}
	}
	if broken == nil || broken.EffectID != "effect.skill.tho.thien_son_tran_shield" {
		t.Fatalf("first broken = %+v, want thien_son_tran_shield", broken)
	}
	// Ward survives: its linked tho_giap shield still holds 250.
	if !s.Book(2).HasTag(TagDisplacementImmune, 100) {
		t.Fatal("ward unlinked while linked shield alive")
	}
	// Drain tho_giap -> ward unlinks.
	if _, res = s.ConsumeShields(2, 250, 100); kind(res, ResultShieldBroken) == nil {
		t.Fatal("tho_giap did not break")
	}
	if s.Book(2).HasTag(TagDisplacementImmune, 100) {
		t.Fatal("ward survived linked shield break")
	}

	// Same source+effect reapply = max(remaining,new) + new expiry.
	s.Apply(src, tgt, Request{EffectID: "effect.skill.tho.thien_son_tran_shield", ShieldAmount: 300}, ctx)
	s.Apply(src, tgt, Request{EffectID: "effect.skill.tho.thien_son_tran_shield", ShieldAmount: 250}, ctx)
	sb := s.shieldBookFor(2)
	if len(sb.entries) != 1 {
		t.Fatalf("entries = %d, want 1 (reapply max)", len(sb.entries))
	}
	if sb.entries[0].Remaining != 300 {
		t.Fatalf("remaining = %d, want max(300,250)=300", sb.entries[0].Remaining)
	}
	want := uint64(100) + msToTicks(6000)
	if sb.entries[0].ExpiresAt != want {
		t.Fatalf("reapply expiry = %d, want %d", sb.entries[0].ExpiresAt, want)
	}

	// Expired -> SHIELD_EXPIRED (distinct from broken/removed).
	s.Apply(src, tgt, Request{EffectID: "effect.skill.thuy.thuy_kinh_shield", ShieldAmount: 100}, ctx)
	res = s.tickTarget(2, 100+msToTicks(5000))
	if kind(res, ResultShieldExpired) == nil {
		t.Fatalf("expected SHIELD_EXPIRED: %+v", res)
	}
	// Removed -> SHIELD_REMOVED.
	s.Apply(src, tgt, Request{EffectID: "effect.skill.tho.thien_son_tran_shield", ShieldAmount: 100}, Ctx{Now: 200})
	res = s.RemoveShield(2, "effect.skill.tho.thien_son_tran_shield", 0, 200)
	if kind(res, ResultShieldRemoved) == nil {
		t.Fatalf("expected SHIELD_REMOVED: %+v", res)
	}
}

// TestStatusEffectStacking — STACK reapply adds stacks to max and
// refreshes expiry; stacked DoT ticks scale with stack_count.
func TestStatusEffectStacking(t *testing.T) {
	s := New(nil)
	src := fighter(1)
	tgt := TargetView{EntityID: 2}

	for i := 0; i < 4; i++ {
		out := s.Apply(src, tgt, Request{EffectID: "effect.skill.thuy.chill_3s"}, Ctx{Now: 10})
		last := out[0]
		if i < 3 && last.Stacks != int32(i+1) {
			t.Fatalf("stack %d = %d", i, last.Stacks)
		}
		if i == 3 && last.Stacks != 3 {
			t.Fatalf("cap: stacks = %d, want 3", last.Stacks)
		}
	}
	inst := s.Book(2).Get(InstanceKey{EffectID: "effect.skill.thuy.chill_3s", SourceID: 1})
	if inst == nil || inst.Stacks != 3 {
		t.Fatal("chill instance missing/capped wrong")
	}

	// Stacked poison ticks 3x the per-stack damage.
	src.Attack = 1000
	for i := 0; i < 3; i++ {
		s.Apply(src, tgt, Request{EffectID: "effect.basic.poison_stack_4s"}, Ctx{Now: 20})
	}
	res := s.tickTarget(2, 20+msToTicks(1000))
	tick := kind(res, ResultDotTick)
	if tick == nil || tick.Amount != int64(0.08*3*1000) {
		t.Fatalf("stacked dot tick = %+v, want 240", tick)
	}
}

// TestRecursionSafetyCap — depth 3 + once per (effect_id,target) per
// root application.
func TestRecursionSafetyCap(t *testing.T) {
	s := New(nil)
	src := fighter(1)
	tgt := TargetView{EntityID: 2}
	g := NewRecursionGuard()

	out := s.Apply(src, tgt, Request{EffectID: "effect.basic.slow_20_3s"}, Ctx{Now: 0, Guard: g})
	if kind(out, ResultApplied) == nil {
		t.Fatal("root apply failed")
	}
	// Same (effect,target) under the root -> rejected.
	out = s.Apply(src, tgt, Request{EffectID: "effect.basic.slow_20_3s"}, Ctx{Now: 0, Guard: g.Nest()})
	if kind(out, ResultRejected) == nil {
		t.Fatal("duplicate (effect,target) not rejected")
	}
	// Depth 3 cap: root(0) -> 1 -> 2 -> 3 rejects.
	g2 := NewRecursionGuard()
	g2.Enter("a", 2)
	d := g2.Nest().Nest().Nest() // depth 3
	out = s.Apply(src, tgt, Request{EffectID: "effect.basic.vulnerable_8_4s"}, Ctx{Now: 0, Guard: d})
	if kind(out, ResultRejected) == nil {
		t.Fatal("depth cap not enforced")
	}
}

// TestReapplySemantics — REFRESH_DURATION is an unconditional
// now+duration reset on non-control templates; magnitude and source
// snapshot take the new application.
func TestReapplySemantics(t *testing.T) {
	s := New(nil)
	src := fighter(1)
	tgt := TargetView{EntityID: 2}

	s.Apply(src, tgt, Request{EffectID: "effect.basic.slow_20_3s"}, Ctx{Now: 0})
	inst := s.Book(2).Get(InstanceKey{EffectID: "effect.basic.slow_20_3s"})
	if inst.ExpiresAtTick != msToTicks(3000) {
		t.Fatalf("expiry = %d", inst.ExpiresAtTick)
	}
	// Reapply at tick 50: unconditional reset to 50 + 60 = 110 even
	// though it SHORTENS nothing / lengthens — unconditional means
	// exactly now+duration.
	s.Apply(src, tgt, Request{EffectID: "effect.basic.slow_20_3s"}, Ctx{Now: 50})
	if inst.ExpiresAtTick != 50+msToTicks(3000) {
		t.Fatalf("refresh expiry = %d, want %d", inst.ExpiresAtTick, 50+msToTicks(3000))
	}
	// A reapply that would shorten is still applied (unconditional).
	s.Apply(src, tgt, Request{EffectID: "effect.basic.slow_20_3s", DurationMs: 500}, Ctx{Now: 60})
	if inst.ExpiresAtTick != 60+msToTicks(500) {
		t.Fatalf("shortened refresh = %d, want %d", inst.ExpiresAtTick, 60+msToTicks(500))
	}
}

// TestControlRefreshNeverShortens — STUN/ROOT/FREEZE/AIRBORNE reapply
// as expires_at = max(expires_at, now + duration).
func TestControlRefreshNeverShortens(t *testing.T) {
	s := New(nil)
	src := fighter(1)
	tgt := TargetView{EntityID: 2}

	s.Apply(src, tgt, Request{EffectID: "effect.basic.stun_500ms"}, Ctx{Now: 0})
	inst := s.Book(2).Get(InstanceKey{EffectID: "effect.basic.stun_500ms"})
	base := inst.ExpiresAtTick

	// Shorter reapply keeps the original expiry.
	s.Apply(src, tgt, Request{EffectID: "effect.basic.stun_500ms", DurationMs: 200}, Ctx{Now: 5})
	if inst.ExpiresAtTick != base {
		t.Fatalf("control shortened: %d -> %d", base, inst.ExpiresAtTick)
	}
	// Longer reapply extends (applied while still active).
	s.Apply(src, tgt, Request{EffectID: "effect.basic.stun_500ms"}, Ctx{Now: 9})
	want := uint64(9) + msToTicks(500)
	if inst.ExpiresAtTick != want {
		t.Fatalf("control extension = %d, want %d", inst.ExpiresAtTick, want)
	}
	// Every control template behaves the same.
	for _, id := range []string{
		"effect.basic.root_1200ms", "effect.basic.freeze_1200ms",
		"effect.skill.tho.airborne_800ms",
	} {
		s.Apply(src, TargetView{EntityID: 3}, Request{EffectID: id}, Ctx{Now: 0})
		i := s.Book(3).Get(InstanceKey{EffectID: id})
		s.Apply(src, TargetView{EntityID: 3}, Request{EffectID: id, DurationMs: 1}, Ctx{Now: 5})
		if i.ExpiresAtTick != msToTicks(i.tmpl.DurationMs) {
			t.Fatalf("%s shortened", id)
		}
	}
}

// TestInstanceKeyTargetVsSource — TARGET-keyed statuses share one
// instance across sources (latest source wins); SOURCE-keyed statuses
// (DoT, CHILL, CRIT_MARK, MA_AM) coexist per source.
func TestInstanceKeyTargetVsSource(t *testing.T) {
	s := New(nil)
	a, b := fighter(1), fighter(2)
	tgt := TargetView{EntityID: 9}

	s.Apply(a, tgt, Request{EffectID: "effect.basic.slow_20_3s"}, Ctx{Now: 0})
	s.Apply(b, tgt, Request{EffectID: "effect.basic.slow_20_3s"}, Ctx{Now: 0})
	if n := len(s.Book(9).inst); n != 1 {
		t.Fatalf("TARGET-keyed instances = %d, want 1", n)
	}
	if got := s.Book(9).Get(InstanceKey{EffectID: "effect.basic.slow_20_3s"}).SourceID; got != 2 {
		t.Fatalf("latest source = %d, want 2", got)
	}

	s.Apply(a, tgt, Request{EffectID: "effect.basic.crit_mark_3s"}, Ctx{Now: 0})
	s.Apply(b, tgt, Request{EffectID: "effect.basic.crit_mark_3s"}, Ctx{Now: 0})
	n := 0
	for k := range s.Book(9).inst {
		if k.EffectID == "effect.basic.crit_mark_3s" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("SOURCE-keyed instances = %d, want 2", n)
	}
}

// TestDotTickAnchorOnRefresh — DoT ticks at anchor + k*1000ms;
// refresh resets damage expiry and the attack snapshot but never the
// anchor.
func TestDotTickAnchorOnRefresh(t *testing.T) {
	s := New(nil)
	src := fighter(1)
	src.Attack = 2000
	tgt := TargetView{EntityID: 2}

	s.Apply(src, tgt, Request{EffectID: "effect.basic.burn_3s"}, Ctx{Now: 0})
	inst := s.Book(2).Get(InstanceKey{EffectID: "effect.basic.burn_3s", SourceID: 1})
	if inst.AnchorTick != 0 {
		t.Fatalf("anchor = %d, want 0", inst.AnchorTick)
	}

	// Tick 20 (1.0s): first anchored tick — floor(2000*0.12) = 240.
	res := s.tickTarget(2, msToTicks(1000))
	tick := kind(res, ResultDotTick)
	if tick == nil || tick.Amount != 240 {
		t.Fatalf("tick1 = %+v, want 240", tick)
	}
	// Off-anchor tick produces nothing.
	if r := s.tickTarget(2, msToTicks(1050)); len(r) != 0 {
		t.Fatalf("off-anchor tick produced %+v", r)
	}

	// Refresh at tick 25 (1.25s): damage deadline and snapshot
	// reset; anchor stays 0. New snapshot 3000 -> ticks 360.
	src.Attack = 3000
	s.Apply(src, tgt, Request{EffectID: "effect.basic.burn_3s"}, Ctx{Now: 25})
	if inst.AnchorTick != 0 {
		t.Fatalf("anchor moved to %d", inst.AnchorTick)
	}
	if inst.DamageExpiresAtTick != 25+msToTicks(3000) {
		t.Fatalf("damage expiry = %d", inst.DamageExpiresAtTick)
	}
	res = s.tickTarget(2, msToTicks(2000)) // anchor + 2000ms
	tick = kind(res, ResultDotTick)
	if tick == nil || tick.Amount != 360 {
		t.Fatalf("post-refresh tick = %+v, want 360", tick)
	}
}

// TestImmunityTagsSlowAndDisplacement — SLOW_IMMUNE rejects new SLOW
// instances; DISPLACEMENT_IMMUNE rejects DISPLACEMENT instances and
// forced-position results while the rest of the hit resolves.
func TestImmunityTagsSlowAndDisplacement(t *testing.T) {
	s := New(nil)
	src := fighter(1)
	tgt := TargetView{EntityID: 2}

	// Grant the displacement ward (carries DISPLACEMENT_IMMUNE) and
	// a slow ward via template immunity for the SLOW check — use a
	// thuy_kinh_ward instance for SLOW_IMMUNE.
	s.Apply(src, tgt, Request{EffectID: "effect.skill.tho.tho_giap_ward"}, Ctx{Now: 0})
	s.Apply(src, tgt, Request{EffectID: "effect.skill.thuy.thuy_kinh_ward"}, Ctx{Now: 0})

	out := s.Apply(src, tgt, Request{EffectID: "effect.basic.slow_20_3s"}, Ctx{Now: 0})
	if r := kind(out, ResultRejected); r == nil || r.RejectedBy != TagSlowImmune {
		t.Fatalf("slow not rejected by SLOW_IMMUNE: %+v", out)
	}
	out = s.Apply(src, tgt, Request{EffectID: "effect.skill.tho.airborne_800ms"}, Ctx{Now: 0})
	if r := kind(out, ResultRejected); r == nil || r.RejectedBy != TagDisplacementImmune {
		t.Fatalf("airborne not rejected by DISPLACEMENT_IMMUNE: %+v", out)
	}
	// Forced-position result suppressed, the hit's other effects
	// still resolve.
	out = s.Apply(src, tgt, Request{
		EffectID: "effect.skill.tho.airborne_800ms", ForcedPosition: true,
	}, Ctx{Now: 0})
	if kind(out, ResultForcedPositionSuppressed) == nil {
		t.Fatalf("forced position not suppressed: %+v", out)
	}
	// A non-displacement negative status still applies.
	out = s.Apply(src, tgt, Request{EffectID: "effect.basic.weaken_10_3s"}, Ctx{Now: 0})
	if kind(out, ResultApplied) == nil {
		t.Fatalf("non-immune status rejected: %+v", out)
	}
}

// TestFillStatusesEviction — Snap.Statuses cap 16 with deterministic
// creation-seq eviction and wire ordering (source, effect, seq).
func TestFillStatusesEviction(t *testing.T) {
	s := New(nil)
	src := fighter(1)
	var snap replication.EntitySnapshot
	tgt := TargetView{EntityID: 2}

	ids := []string{
		"effect.basic.slow_15_2s", "effect.basic.slow_20_3s",
		"effect.basic.slow_25_3s", "effect.skill.thuy.slow_40_3s",
		"effect.skill.kim.kiem_tran_slow", "effect.skill.moc.thanh_dang_slow",
		"effect.skill.thuy.chill_3s", "effect.basic.freeze_1200ms",
		"effect.basic.stun_400ms", "effect.basic.stun_500ms",
		"effect.basic.root_1200ms", "effect.basic.resist_shred_hoa_3s",
		"effect.basic.weaken_10_3s", "effect.basic.weaken_12_3s",
		"effect.basic.vulnerable_8_4s", "effect.skill.kim.pha_giap_vulnerable_5s",
		"effect.basic.crit_mark_3s", "effect.basic.heal_reduction_3s",
	}
	for _, id := range ids {
		s.Apply(src, tgt, Request{EffectID: id}, Ctx{Now: 0})
	}
	FillStatuses(&snap, s.Book(2), 0)
	if snap.StatusN != replication.MaxStatuses {
		t.Fatalf("statuses = %d, want %d", snap.StatusN, replication.MaxStatuses)
	}
	// The 16 oldest creation seqs survive: chill (seq 7) in,
	// heal_reduction (seq 18) out.
	got := make(map[string]bool)
	for i := 0; i < snap.StatusN; i++ {
		got[snap.Statuses[i].EffectID] = true
	}
	if !got["effect.skill.thuy.chill_3s"] || got["effect.basic.heal_reduction_3s"] {
		t.Fatalf("eviction wrong: %v", got)
	}
}

// TestAllRegistryRefIDsResolve — every effect.* RefID referenced by
// skills payloads or proc tables resolves to a registered template
// (IMP-016 ordered step 2).
func TestAllRegistryRefIDsResolve(t *testing.T) {
	missing := map[string]bool{}
	for _, d := range skills.List() {
		for _, p := range d.Payloads {
			if (p.Kind == skills.PayStatus || p.Kind == skills.PayShield) && p.RefID != "" {
				if _, ok := TemplateByID(p.RefID); !ok {
					missing[p.RefID] = true
				}
			}
		}
		for _, id := range d.ProcEffects {
			if _, ok := TemplateByID(id); !ok {
				missing[id] = true
			}
		}
	}
	for id := range missing {
		t.Errorf("unresolved effect RefID %s", id)
	}
}
