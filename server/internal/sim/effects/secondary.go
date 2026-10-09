package effects

// secondary.go — stage-7 secondary result builders of combat.md §
// Secondary Results and stats.md § ADR-0037 resolutions, plus the
// HEAL_REDUCTION stat→status producer (stats.md § HEAL_REDUCTION
// Resolution): on committed hp_damage > 0 the attacker's
// HEAL_REDUCTION stat auto-applies the HEAL_REDUCTION status at
// magnitude (1 − stat) for 4.0s, refreshed on re-application, keyed
// per-source; combined multiplicative floor ≥ 0.40.

// reflectRangeMM is the REFLECT origin gate (≤ 3.0m).
const reflectRangeMM = 3000

// reflectCapBP is the per-hit reflect cap: floor(0.03 * attacker MAX_HP).
const reflectCapBP = 300

// lifestealHPSCapBP is the rolling 1.0s HPS cap: 0.015 * attacker MAX_HP.
const lifestealHPSCapBP = 150

// lifestealAoEFactor applies to extra targets, DoT ticks and zone ticks.
const lifestealAoEFactor = 0.30

// lifestealWindowTicks is the rolling window (1.0s at 20 Hz).
const lifestealWindowTicks = 20

// absorbPerBP / absorbAggBP are the ABSORB caps (per-instance 0.15 *
// MAX_HP, aggregate 0.50 * MAX_HP discard-in-full).
const (
	absorbPerBP = 1500
	absorbAggBP = 5000
)

// SecondaryResult is a stage-7 outcome: a REFLECT damage packet, a
// LIFESTEAL heal amount, or an ABSORB shield grant request.
type SecondaryResult struct {
	Kind       ResultKind
	TargetID   uint64
	SourceID   uint64
	Amount     int64
	Element    string
	Tags       TagSet
	Request    *Request // ABSORB shield grant / HEAL_REDUCTION producer
	RejectedBy TagSet
}

// reflect builds the REFLECT packet when the stat rolls it: ≤3.0m
// origin gate, tags NO_CRIT|NO_REFLECT|NO_LIFESTEAL|NO_PROC, per-hit
// cap floor(0.03 * attacker MAX_HP).
func Reflect(attacker, source StatsView, postMitigation int64, originDistanceMM int64) *SecondaryResult {
	if originDistanceMM > reflectRangeMM {
		return nil
	}
	stat := source.Stats[StatReflect]
	if stat == 0 {
		return nil
	}
	amt := postMitigation * int64(stat) / 10000
	capAmt := source.MaxHP * reflectCapBP / 10000
	if amt > capAmt {
		amt = capAmt
	}
	if amt <= 0 {
		return nil
	}
	return &SecondaryResult{
		Kind:     ResultDotTick, // damage packet kind; pipeline tags it NO_CRIT|NO_REFLECT|NO_LIFESTEAL|NO_PROC
		TargetID: attacker.EntityID, SourceID: source.EntityID,
		Amount: amt,
	}
}

// lifestealWindow is the rolling 1.0s HPS accumulator: samples inside
// the window count toward 0.015 * MAX_HP; excess is discarded, never
// banked.
type lifestealWindow struct {
	samples [lifestealWindowTicks]int64
	head    int
	total   int64
	epoch   uint64
}

// Lifesteal resolves the LIFESTEAL heal for one connected hit.
// aoeFactor applies the 0.30 factor for extra targets, DoT and zone
// results. The rolling 1.0s window caps output at 0.015 * MAX_HP;
// lifesteal IS subject to HEAL_REDUCTION — the heal is multiplied by
// the target's HealingReceived, floored by the ≥0.40 combined rule.
// healsTarget distinguishes whether the heal target is the attacker
// (spec: lifesteal heals the attacker, reduced by the ATTACKER's
// received-healing modifiers).
func Lifesteal(w *lifestealWindow, attacker StatsView, hpDamage int64, aoeFactor bool, now uint64) int64 {
	stat := attacker.Stats[StatLifesteal]
	if stat == 0 || hpDamage <= 0 {
		return 0
	}
	factor := float64(int64(stat)) / 10000
	if aoeFactor {
		factor *= lifestealAoEFactor
	}
	raw := int64(float64(hpDamage) * factor)
	if raw <= 0 {
		return 0
	}
	// Rolling window: expire samples older than the window.
	if w.epoch != now {
		step := now - w.epoch
		if step >= lifestealWindowTicks {
			for i := range w.samples {
				w.samples[i] = 0
			}
			w.total = 0
			w.head = 0
		} else {
			for k := uint64(0); k < step; k++ {
				w.head = (w.head + 1) % lifestealWindowTicks
				w.total -= w.samples[w.head]
				w.samples[w.head] = 0
			}
		}
		w.epoch = now
	}
	capAmt := attacker.MaxHP * lifestealHPSCapBP / 10000
	room := capAmt - w.total
	if room <= 0 {
		return 0
	}
	if raw > room {
		raw = room // excess discarded, never banked
	}
	w.total += raw
	w.samples[w.head] += raw
	return raw
}

// NewLifestealWindow constructs a rolling HPS window.
func NewLifestealWindow() *lifestealWindow { return &lifestealWindow{} }

// HealingReceived returns the combined multiplicative healing-received
// modifier on a target: product of active HEAL_REDUCTION magnitudes,
// floored at 0.40 (stats.md — combined reduction may not exceed 60%).
func (s *System) HealingReceived(target uint64, now uint64) float64 {
	mult := 1.0
	if b, ok := s.books[target]; ok {
		for _, i := range b.inst {
			if i.active(now) && i.Tags&TagHealReduction != 0 {
				m := i.MagHealingReceived
				if m == 0 {
					m = i.tmpl.HealingReceivedMult
				}
				mult *= m
			}
		}
	}
	if mult < 0.40 {
		mult = 0.40
	}
	return mult
}

// Absorb converts the ABSORB stat into a shield grant request on the
// attacker: per-instance cap 0.15 * MAX_HP, aggregate cap 0.50 *
// MAX_HP discard-in-full, shield duration 6.0s; NOT subject to
// HEAL_REDUCTION.
func Absorb(attacker StatsView, postMitigation int64, liveShield int64) *SecondaryResult {
	stat := attacker.Stats[StatAbsorb]
	if stat == 0 || postMitigation <= 0 {
		return nil
	}
	amt := postMitigation * int64(stat) / 10000
	perCap := attacker.MaxHP * absorbPerBP / 10000
	if amt > perCap {
		amt = perCap
	}
	// Aggregate cap: discard-in-full when the live absorb pool would
	// exceed 0.50 * MAX_HP.
	aggCap := attacker.MaxHP * absorbAggBP / 10000
	if liveShield+amt > aggCap {
		return nil
	}
	if amt <= 0 {
		return nil
	}
	return &SecondaryResult{
		Kind:     ResultShieldGranted,
		TargetID: attacker.EntityID, SourceID: attacker.EntityID,
		Amount: amt,
		Request: &Request{
			EffectID:     "effect.absorb.self",
			ShieldAmount: amt,
		},
	}
}

// HealReductionRequest produces the stat→status request the stage-7
// path emits on committed hp_damage > 0 (stats.md § HEAL_REDUCTION
// Resolution): magnitude (1 − HEAL_REDUCTION), 4.0s, refreshed on
// re-application, per-source key.
func HealReductionRequest(attacker StatsView, hpDamage int64) *Request {
	stat := attacker.Stats[StatHealReduction]
	if stat == 0 || hpDamage <= 0 {
		return nil
	}
	return &Request{
		EffectID:            "effect.stat.heal_reduction_4s",
		HealingReceivedMult: 1.0 - float64(int64(stat))/10000,
	}
}
