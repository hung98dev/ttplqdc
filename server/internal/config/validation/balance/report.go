package balance

import (
	"fmt"
	"math"
	"strings"

	"thinhthan/internal/config"
	"thinhthan/internal/config/equipment"
)

// ---- Lv60 rotation benchmark ----------------------------------------------

// rotationTTK runs the spec's deterministic 50ms-tick scheduler for one
// class at Lv60: actives Lv10 (damage_scale 1.315, cooldown x0.73) at
// fixed priority, basic_1 filling downtime, MP spent with in-combat
// regen, basic restore +2 MP. Boss is an immortal stationary target at
// melee distance; damage uses the canonical pipeline vs boss DEFENSE.
func rotationTTK(c *config.CandidateSnapshot, cat *equipment.Catalog, class string, boss bossRow, partySize int) float64 {
	type activeSkill struct {
		id      string
		coef    float64
		cdMs    int64
		cost    int64
		busyMs  int64
		readyAt int64
	}
	b := referenceBuild(cat, class, 60, nil)
	var acts []*activeSkill
	for _, suf := range rotationPriority[class] {
		sid := "skill." + lower(class) + ".active." + suf
		for _, r := range familyRecs(c, "skill_action") {
			if fieldStr(r, "skill_id") != sid {
				continue
			}
			sel2 := &activeSkill{id: sid}
			if v, ok := fieldRat(r, "base_cd_s"); ok {
				sel2.cdMs = int64(math.Round(ratFloat(v) * 1000 * 0.73))
			}
			if v, ok := fieldInt(r, "cost_mp"); ok {
				sel2.cost = v
			}
			var st, ac, rc int64
			st, _ = fieldInt(r, "startup_ms")
			ac, _ = fieldInt(r, "active_ms")
			rc, _ = fieldInt(r, "recovery_ms")
			sel2.busyMs = st + ac + rc
			for _, r2 := range familyRecs(c, "skill_effect") {
				if fieldStr(r2, "skill_id") != sid {
					continue
				}
				if p, ok := r2.Fields["payload"]; ok && p.Kind == config.KindRecord &&
					p.Rec["kind"].Str == "DAMAGE" {
					if cf, ok2 := p.Rec["coefficient"]; ok2 && cf.Kind == config.KindRational {
						sel2.coef = ratFloat(cf.Rat) * 1.315
					}
				}
			}
			if sel2.cdMs > 0 {
				acts = append(acts, sel2)
			}
		}
	}
	coeff, cdS, startup, active, ok := basicPin(c, class)
	if !ok {
		return 0
	}
	basicCdMs := int64(math.Round(cdS * 1000))
	basicBusy := int64(startup + active)
	mpRegenPerSec := 3 + 0.10*59 // in-combat reference MP_REGEN
	mp := b.maxMP
	hp := float64(boss.BaseHP)
	var t int64
	busyUntil := int64(0)
	basicReadyAt := int64(0)
	hits := 0
	for hp > 0 && t < 600000 {
		// MP regen ticks at t=1000 then every 1000ms
		if t >= 1000 && t%1000 == 0 {
			mp += mpRegenPerSec
			if mp > b.maxMP {
				mp = b.maxMP
			}
		}
		if t < busyUntil {
			t += 50
			continue
		}
		// first ready affordable active in priority order
		var sel *activeSkill
		for _, a := range acts {
			if a.readyAt <= t && mp >= float64(a.cost) && a.coef > 0 {
				sel = a
				break
			}
		}
		if sel != nil {
			mp -= float64(sel.cost)
			dmg := math.Floor(b.attack*sel.coef) * (1 + b.critChance*0.50)
			k := 100 + 20*float64(boss.Level)
			mult := k / (k + math.Max(0, float64(boss.Defense)))
			if mult < 0.25 {
				mult = 0.25
			}
			hp -= math.Floor(math.Floor(dmg) * mult)
			sel.readyAt = t + sel.cdMs
			busyUntil = t + sel.busyMs
			hits++
			t += 50
			continue
		}
		if basicReadyAt <= t {
			hp -= basicDamage(b, coeff, float64(boss.Defense), float64(boss.Level))
			mp += 2
			if mp > b.maxMP {
				mp = b.maxMP
			}
			iv := basicCdMs
			if iv < basicBusy {
				iv = basicBusy
			}
			iv = 50 * int64(math.Ceil(float64(iv)/50))
			basicReadyAt = t + iv
			busyUntil = t + basicBusy
			hits++
		}
		t += 50
	}
	_ = hits
	solo := float64(t) / 1000
	if partySize > 1 {
		return solo * 0.64 // canonical five-player TTK factor
	}
	return solo
}

// ---- Report ----------------------------------------------------------------

// Report is the § Validation Output serialization for one candidate.
type Report struct {
	lines []string
}

// BuildReport computes every Validation Output field deterministically.
func BuildReport(c *config.CandidateSnapshot, rev string) Report {
	var d config.Diagnostics
	cat, _ := equipment.Load(c)
	rows := checkTTK(c, cat, &d)
	r := Report{lines: []string{"revision: " + rev}}
	for _, row := range rows {
		r.lines = append(r.lines,
			fmt.Sprintf("class_id: %s | tier: %s | level: %d", row.Class, row.Tier, row.Level),
			fmt.Sprintf("  reference ATTACK=%.0f DEFENSE=%.0f MAX_HP=%.0f CRIT=%.4f ATTACK_SPEED=%.3f",
				row.Attack, row.Defense, row.MaxHP, row.CritChance, row.AttackSpd),
			fmt.Sprintf("  NORMAL TTK %.2fs | ELITE TTK %.2fs | boss solo %.2fs | boss 5p %.2fs | boss %s",
				row.NormalTTK, row.EliteTTK, row.BossSoloTTK, row.BossPartyTTK, row.BossID),
			fmt.Sprintf("  heavy-hit HP ratio %.4f", row.HeavyHitPct))
	}
	// Lv60 rotation TTK vs the Lv60 tier boss
	for _, cl := range classIDs {
		for _, bs := range bosses(c) {
			if bs.Level == 60 && bs.Mode == "INSTANCED" {
				r.lines = append(r.lines, fmt.Sprintf(
					"rotation TTK %s vs %s: solo %.2fs | five-player %.2fs",
					cl, bs.ID, rotationTTK(c, cat, cl, bs, 1), rotationTTK(c, cat, cl, bs, 5)))
			}
		}
	}
	for _, s := range newPoolStats {
		if m, ok := expectedMagnitude(cat, s, "T6"); ok {
			r.lines = append(r.lines, fmt.Sprintf("%s expected magnitude T6: %s", s, m.String()))
		}
	}
	if pct, ok := powerDeltaPct(cat, "T6"); ok {
		r.lines = append(r.lines, fmt.Sprintf("secondary-roll pool expected power delta vs pre-change baseline: %+.2f%%", pct))
	}
	if ratio, ok := sustainRatio(c); ok {
		r.lines = append(r.lines, fmt.Sprintf("lifesteal+regen sustain vs boss average DPS ratio: %.4f", ratio))
	}
	reachRows, _ := enumerateReach(c)
	pass := 0
	for _, rr := range reachRows {
		if rr.OK {
			pass++
		}
	}
	r.lines = append(r.lines, fmt.Sprintf("skill geometry rows %d pass %d", len(reachRows), pass))
	for _, rr := range reachRows {
		r.lines = append(r.lines, "  "+rr.String())
	}
	if n, dd, _, _ := separationRatioMM(c); n > 0 {
		r.lines = append(r.lines, fmt.Sprintf("reach separation ratio %.4f", float64(n)/float64(dd)))
	}
	var maxOuter int64
	for _, rr := range reachRows {
		if rr.OuterMM > maxOuter {
			maxOuter = rr.OuterMM
		}
	}
	r.lines = append(r.lines, fmt.Sprintf("max outer reach %dmm | camera margin %dmm", maxOuter, cameraHalfMM-maxOuter))
	for _, fx := range colliderFixtures() {
		r.lines = append(r.lines, fmt.Sprintf("hurtbox fixture %s: %s", fx.Profile, fx.Result))
	}
	// fixture-B violations: max-everywhere / marginal stress diagnostics
	// the gate would raise on this catalog — reported verbatim.
	var sd config.Diagnostics
	checkWindowPreservation(c, cat, &sd)
	r.lines = append(r.lines, fmt.Sprintf("fixture-B violations %d", len(sd)))
	for _, dg := range sd {
		r.lines = append(r.lines, "  "+dg.Message)
	}
	return r
}

// String returns the stable serialized report.
func (r Report) String() string { return strings.Join(r.lines, "\n") + "\n" }
