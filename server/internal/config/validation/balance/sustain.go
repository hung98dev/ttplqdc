package balance

import (
	"math"

	"thinhthan/internal/config"
	"thinhthan/internal/config/equipment"
)

// Pinned reference values (balance_validation.md § Sustain Benchmark —
// do not re-derive).
const (
	refLv60MaxHP    = 4794.0
	refHPRegenSec   = 9.08
	bossHitInterval = 3.0 // seconds between heavy hits
)

// lifestealCap returns LIFESTEAL_HPS_CAP = 0.015 * MAX_HP per second.
func lifestealCap(maxHP float64) float64 { return 0.015 * maxHP }

// sustainRatio returns total lifesteal+regen sustain divided by the
// boss's average DPS floor — the Validation Output field.
func sustainRatio(c *config.CandidateSnapshot) (float64, bool) {
	minDPS := bossMinDPS(c)
	if minDPS <= 0 {
		return 0, false
	}
	return (lifestealCap(refLv60MaxHP) + refHPRegenSec) / minDPS, true
}

// bossMinDPS is the documented boss average DPS at the pinned Lv60
// reference: heavy hit (1.40 * boss ATTACK at reference DEFENSE 426)
// every ~3.0s, taken over the authored Lv60 boss rows.
func bossMinDPS(c *config.CandidateSnapshot) float64 {
	min := 0.0
	for _, b := range bosses(c) {
		if b.Level != 60 {
			continue
		}
		raw := math.Floor(float64(b.Attack) * 1.40)
		hit := math.Floor(raw * defMult(426, 60))
		dps := hit / bossHitInterval
		if min == 0 || dps < min {
			min = dps
		}
	}
	return min
}

// checkSustain enforces the lifesteal immortality guardrail at the
// pinned reference and the fixture-B sustain/mitigation enumeration for
// the four new stats.
func checkSustain(c *config.CandidateSnapshot, cat *equipment.Catalog, d *config.Diagnostics) {
	capHPS := lifestealCap(refLv60MaxHP) // ~71.9 HP/s
	total := capHPS + refHPRegenSec      // ~81 HP/s
	minDPS := bossMinDPS(c)
	if minDPS <= 0 {
		d.Addf(config.DiagIntegrationCheck, "", 0,
			"balance.lifesteal_sustain: no authored boss attack for sustain reference")
	} else if total >= minDPS {
		d.Addf(config.DiagBalanceGuardrail, "", 0,
			"balance.lifesteal_sustain: LIFESTEAL_HPS_CAP+HP_REGEN %.2f >= boss avg DPS %.2f",
			total, minDPS)
	}

	// Fixture B sustain side: at every legal LIFESTEAL magnitude the
	// effective sustain (capped lifesteal + regen) must stay below the
	// boss DPS floor — never net-immortal.
	roll := cat.Rolls["roll.lifesteal"]
	if roll == nil {
		return
	}
	for _, m := range rollMagnitudes(cat, roll, "T6") {
		ls := ratFloat(m)
		outDPS := 6900.0 * ls // capped synthetic 6900 DPS reference
		if outDPS > capHPS {
			outDPS = capHPS
		}
		sus := outDPS + refHPRegenSec
		if minDPS > 0 && sus >= minDPS {
			d.Addf(config.DiagBalanceGuardrail, "", 0,
				"balance.lifesteal_sustain: roll.lifesteal=%s sustains %.2f >= boss avg DPS %.2f",
				m.String(), sus, minDPS)
		}
	}
}
