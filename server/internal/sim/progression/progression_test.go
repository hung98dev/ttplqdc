package progression

import (
	"testing"
)

func TestLevelEXPCurve(t *testing.T) {
	// exp_required(L) = 10000·L² (×100 scale).
	for _, tc := range []struct {
		level int32
		want  int64
	}{
		{1, 10_000}, {2, 40_000}, {10, 1_000_000}, {59, 34_810_000}, {60, 0},
	} {
		if got := ExpRequired(tc.level); got != tc.want {
			t.Fatalf("ExpRequired(%d) = %d, want %d", tc.level, got, tc.want)
		}
	}
	// CumulativeExp(k) = Σ 10000·i² for i=1..k-1; cum(1)=0; cum(60)=cap.
	if got := CumulativeExp(1); got != 0 {
		t.Fatalf("CumulativeExp(1) = %d", got)
	}
	if got := CumulativeExp(2); got != 10_000 {
		t.Fatalf("CumulativeExp(2) = %d", got)
	}
	if got := CumulativeExp(MaxLevel); got != ExpCap {
		t.Fatalf("CumulativeExp(60) = %d, want %d", got, ExpCap)
	}
	// LevelFor: exact thresholds and boundary off-by-ones.
	for _, tc := range []struct {
		exp  int64
		want int32
	}{
		{-5, 1}, {0, 1}, {9_999, 1}, {10_000, 2}, {49_999, 2}, {50_000, 3},
		{ExpCap - 1, 59}, {ExpCap, 60}, {ExpCap + 1, 60},
	} {
		if got := LevelFor(tc.exp); got != tc.want {
			t.Fatalf("LevelFor(%d) = %d, want %d", tc.exp, got, tc.want)
		}
	}
	// Level-60 stop: EXP does not accumulate past the cap.
	p := PlayerProgress{ClassID: "class.kim", Level: 59, Exp: int32(ExpCap) - 10}
	out, ups := ApplyExp(p, 1_000_000)
	if out.Level != 60 || out.Exp != int32(ExpCap) {
		t.Fatalf("cap apply = lvl %d exp %d", out.Level, out.Exp)
	}
	if len(ups) != 1 || ups[0].Level != 60 || ups[0].SkillPoints != 3 || ups[0].PotentialPoints != 4 {
		t.Fatalf("L60 level-up = %+v", ups)
	}
	out2, ups2 := ApplyExp(out, 500)
	if out2.Exp != int32(ExpCap) || out2.Level != 60 || len(ups2) != 0 {
		t.Fatalf("post-cap apply = %+v ups %d", out2, len(ups2))
	}
	// Multi-level grant: 0 → 3 (cum 50,000) yields 2 level-ups.
	base := PlayerProgress{ClassID: "class.kim", Level: 1, Exp: 0}
	out3, ups3 := ApplyExp(base, 50_000)
	if out3.Level != 3 || len(ups3) != 2 {
		t.Fatalf("multi-level = lvl %d ups %d", out3.Level, len(ups3))
	}
	if out3.UnspentSkillPoints != 2 || out3.UnspentPotentialPoints != 8 {
		t.Fatalf("points after 2 levels = skill %d potential %d",
			out3.UnspentSkillPoints, out3.UnspentPotentialPoints)
	}
	// Unlock milestones learned automatically on the gaining level:
	// reaching L4 (cum 140_000) carries kim's L4 basic.
	out4, ups4 := ApplyExp(base, 140_000)
	if out4.Level != 4 || len(ups4) != 3 {
		t.Fatalf("L4 apply = lvl %d ups %d", out4.Level, len(ups4))
	}
	if len(ups4[2].Unlocks) != 1 || ups4[2].Unlocks[0] != "skill.kim.basic.truy_phong_kiem" {
		t.Fatalf("L4 unlocks = %v", ups4[2].Unlocks)
	}
	if out4.Skills["skill.kim.basic.truy_phong_kiem"] != 1 {
		t.Fatalf("L4 unlock not learned at lv1: %v", out4.Skills)
	}
}

func TestPotentialPointAllocation(t *testing.T) {
	p := PlayerProgress{ClassID: "class.kim", Level: 10,
		UnspentPotentialPoints: 36, PotentialAllocated: PotentialDelta{Str: 4}}
	// earned = 36 + 4 = 40 → cap = floor(0.6·40) = 24.
	next, err := p.ApplyAllocate(PotentialDelta{Str: 10, Vit: 5})
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if next.PotentialAllocated != (PotentialDelta{Str: 14, Vit: 5}) ||
		next.UnspentPotentialPoints != 21 {
		t.Fatalf("after allocate = %+v", next)
	}
	if got := EarnedTotal(next.UnspentPotentialPoints, next.PotentialAllocated); got != 40 {
		t.Fatalf("earned total = %d", got)
	}
	if PerStatCap(356) != 213 {
		t.Fatalf("cap at earned 356 = %d", PerStatCap(356))
	}
}

func TestSkillPointBudget59(t *testing.T) {
	// progression.md budgets: 59 from leveling + 4 bonus (55/60) + 12 books
	// = 75 at level 60 (of 114 needed to max all 12 skills).
	if got := SkillPointsEarned(60); got != 63 {
		t.Fatalf("level-up skill budget at 60 = %d, want 63", got)
	}
	if SkillPointsEarned(1) != 0 || SkillPointsEarned(55) != 56 {
		t.Fatalf("budgets: 1=%d 55=%d", SkillPointsEarned(1), SkillPointsEarned(55))
	}
	if SkillBookPoints(60) != 12 || PotentialBookPoints(60) != 120 {
		t.Fatalf("books at 60 = skill %d potential %d",
			SkillBookPoints(60), PotentialBookPoints(60))
	}
	// The 12 learn milestones cover every class's catalog exactly.
	for _, classID := range []string{"class.kim", "class.moc", "class.thuy", "class.hoa", "class.tho"} {
		learned := LearnableAt(classID, 60)
		if len(learned) != 12 {
			t.Fatalf("%s learned %d skills at 60", classID, len(learned))
		}
		if BasicOne(classID) == "" {
			t.Fatalf("%s basic_1 missing", classID)
		}
	}
}

func TestStatPipelineResolution(t *testing.T) {
	// BASE → FLAT_ADD → PERCENT_ADD → FINAL_MULTIPLY → CLAMP.
	base := Stats{}
	base[StatMaxHP] = 1000
	base[StatAttack] = 50
	mods := []Modifier{
		{Stat: StatMaxHP, Kind: ModifierFlatAdd, Value: 200},
		{Stat: StatMaxHP, Kind: ModifierFlatAdd, Value: -50},
		{Stat: StatMaxHP, Kind: ModifierPercentAdd, Value: 0.10},
		{Stat: StatMaxHP, Kind: ModifierPercentAdd, Value: 0.40},
		{Stat: StatMaxHP, Kind: ModifierFinalMultiply, Value: 2},
		// attack: percent stack + multiply — (50)·1.5·2 = 150
		{Stat: StatAttack, Kind: ModifierPercentAdd, Value: 0.50},
		{Stat: StatAttack, Kind: ModifierFinalMultiply, Value: 2},
	}
	out := ComputeFinal(base, mods, false)
	if out[StatMaxHP] != (1000+150)*1.5*2 {
		t.Fatalf("HP = %v", out[StatMaxHP])
	}
	if out[StatAttack] != 150 {
		t.Fatalf("ATK = %v", out[StatAttack])
	}
	// Clamp: crit chance above 0.60 clamps; MOVE_SPEED floor/ceiling.
	mods = append(mods, Modifier{Stat: StatCritChance, Kind: ModifierFlatAdd, Value: 0.90})
	mods = append(mods, Modifier{Stat: StatMoveSpeed, Kind: ModifierFlatAdd, Value: 10})
	out = ComputeFinal(base, mods, false)
	if out[StatCritChance] != 0.60 || out[StatMoveSpeed] != 1.50 {
		t.Fatalf("clamps: crit %v ms %v", out[StatCritChance], out[StatMoveSpeed])
	}
	// Potential conversions: kim STR 0.75/INT 0.25, VIT +6HP/+0.2DEF,
	// AGI sub-capped MOVE_SPEED.
	kim := PotentialModifiers("class.kim", PotentialDelta{Str: 10, Vit: 5, Agi: 200})
	final := ComputeFinal(ComputeBase("class.kim", 1), kim, false)
	// ATK = 40 + 7.5; HP = 500 + 30; DEF = 20 + 1.0; AGI ms contribution
	// 0.0008·200 = 0.16 sub-caps to 0.15 → MOVE_SPEED = 1.15.
	if final[StatAttack] != 47.5 || final[StatMaxHP] != 530 || final[StatDefense] != 21 {
		t.Fatalf("conversions: atk %v hp %v def %v",
			final[StatAttack], final[StatMaxHP], final[StatDefense])
	}
	if final.MoveSpeed() != 1.15 {
		t.Fatalf("agi-subcapped ms = %v", final.MoveSpeed())
	}
	// INT: hoa gets 0.75 ATK + 1 MAX_MP per point.
	hoa := PotentialModifiers("class.hoa", PotentialDelta{Int: 10})
	f2 := ComputeFinal(ComputeBase("class.hoa", 1), hoa, false)
	if f2[StatAttack] != 47.5 || f2[StatMaxMP] != 210 {
		t.Fatalf("hoa INT: atk %v mp %v", f2[StatAttack], f2[StatMaxMP])
	}
	if DefenseMultiplier(0, 1) != 1 || DefenseMultiplier(1e9, 60) != 0.25 {
		t.Fatalf("defense multiplier floor")
	}
	// Class growth: kim L2 = base + growth.
	b2 := ComputeBase("class.kim", 2)
	if b2[StatMaxHP] != 532 || b2[StatAttack] != 45.5 {
		t.Fatalf("kim L2 = %+v", b2)
	}
}

func TestSkillUpgradeRejects(t *testing.T) {
	p := PlayerProgress{ClassID: "class.kim", Level: 10, UnspentSkillPoints: 1,
		Skills: map[string]int32{"skill.kim.basic.kiem_thuc": 12,
			"skill.kim.active.xuyen_phong": 3}}

	if _, err := p.ApplySkillUpgrade("skill.kim.passive.kiem_tam", 0); !mustReject(err, RejectSkillNotLearned, t) {
	}
	if _, err := p.ApplySkillUpgrade("skill.kim.active.xuyen_phong", 2); !mustReject(err, RejectStateConflict, t) {
	}
	if _, err := p.ApplySkillUpgrade("skill.kim.basic.kiem_thuc", 12); !mustReject(err, RejectSkillMaxLevel, t) {
	}
	broke := p
	broke.UnspentSkillPoints = 0
	if _, err := broke.ApplySkillUpgrade("skill.kim.active.xuyen_phong", 3); !mustReject(err, RejectSkillPointsInsufficient, t) {
	}
	if _, err := p.ApplySkillUpgrade("skill.not.a.real_skill", 0); !mustReject(err, RejectSkillNotLearned, t) {
	}
}

func TestPotentialAllocateAllOrNothingCap(t *testing.T) {
	// earned 40 → cap 24: str already 20, delta 5 would land at 25 — the
	// whole op rejects and nothing changes.
	p := PlayerProgress{ClassID: "class.kim", Level: 10,
		UnspentPotentialPoints: 20,
		PotentialAllocated:     PotentialDelta{Str: 20}}
	if _, err := p.ApplyAllocate(PotentialDelta{Str: 5, Vit: 1}); !mustReject(err, RejectPotentialCapExceeded, t) {
	}
	if p.PotentialAllocated.Str != 20 || p.UnspentPotentialPoints != 20 {
		t.Fatalf("rejected op mutated state: %+v", p)
	}
	// At earned 356 the cap is 213: allocate up to it exactly.
	full := PlayerProgress{ClassID: "class.kim", Level: 60,
		UnspentPotentialPoints: 356}
	next, err := full.ApplyAllocate(PotentialDelta{Str: 213, Vit: 143})
	if err != nil {
		t.Fatalf("cap-edge allocate: %v", err)
	}
	if next.PotentialAllocated.Str != 213 || next.UnspentPotentialPoints != 0 {
		t.Fatalf("cap-edge result = %+v", next)
	}
	if _, err := next.ApplyAllocate(PotentialDelta{Str: 1}); !mustReject(err, RejectPotentialPointsInsufficient, t) {
	}
}

func TestRespecChargeAndRefundAtomic(t *testing.T) {
	p := PlayerProgress{ClassID: "class.kim", Level: 40, UnspentSkillPoints: 3,
		UnspentPotentialPoints: 10,
		PotentialAllocated:     PotentialDelta{Str: 20, Vit: 4},
		Skills:                 map[string]int32{"skill.kim.basic.kiem_thuc": 5, "skill.kim.active.xuyen_phong": 3}}
	price := RespecPrice(40)
	if price != 40_000 {
		t.Fatalf("price(40) = %d", price)
	}
	// Balance below the price rejects with no change at all.
	if _, _, err := p.ApplyRespec(RespecKindPotential, price-1); !mustReject(err, RejectInsufficientCurrency, t) {
	}
	if p.PotentialAllocated.Str != 20 || len(p.Skills) != 2 {
		t.Fatalf("shortfall mutated state")
	}
	// Potential respec: zero + refund all allocated in one application.
	next, out, err := p.ApplyRespec(RespecKindPotential, price)
	if err != nil {
		t.Fatalf("respec: %v", err)
	}
	if out.Price != price || out.PotentialRefunded != 24 || next.UnspentPotentialPoints != 34 ||
		next.PotentialAllocated != (PotentialDelta{}) {
		t.Fatalf("potential respec = %+v out %+v", next, out)
	}
	// Skill respec: learned skills → lv1, refund spent, loadout preserved
	// (skills stay learned).
	next2, out2, err := p.ApplyRespec(RespecKindSkill, price)
	if err != nil {
		t.Fatalf("skill respec: %v", err)
	}
	if out2.SkillRefunded != 6 || next2.UnspentSkillPoints != 9 ||
		next2.Skills["skill.kim.basic.kiem_thuc"] != 1 {
		t.Fatalf("skill respec = %+v out %+v", next2, out2)
	}
	if _, _, err := p.ApplyRespec(RespecKindUnspecified, price); !mustReject(err, RejectInvalidKind, t) {
	}
	// Free at Lv20 and below; priced at 21+.
	if RespecPrice(20) != 0 || RespecPrice(21) != 11_025 || RespecPrice(60) != 90_000 {
		t.Fatalf("price table")
	}
}

func TestProgressionOpsIdempotent(t *testing.T) {
	// The pure ops are deterministic on the value: same input state plus
	// the same request produces byte-identical results — the durable layer
	// keys replay on operation_id so a retried request replays the
	// retained outcome rather than re-applying.
	p := PlayerProgress{ClassID: "class.kim", Level: 10, UnspentSkillPoints: 5,
		Skills: map[string]int32{"skill.kim.active.xuyen_phong": 3}}
	a, errA := p.ApplySkillUpgrade("skill.kim.active.xuyen_phong", 3)
	b, errB := p.ApplySkillUpgrade("skill.kim.active.xuyen_phong", 3)
	if errA != nil || errB != nil {
		t.Fatal("both applies must succeed on the same base value")
	}
	if a.Skills["skill.kim.active.xuyen_phong"] != b.Skills["skill.kim.active.xuyen_phong"] ||
		a.UnspentSkillPoints != b.UnspentSkillPoints {
		t.Fatalf("non-deterministic op: %+v vs %+v", a, b)
	}
	// Replaying the value-level op onto the RESULT is a normal second
	// upgrade — not a replay — and it must not corrupt the original.
	if p.Skills["skill.kim.active.xuyen_phong"] != 3 || p.UnspentSkillPoints != 5 {
		t.Fatalf("value op mutated input (map alias): %+v", p)
	}
}

func TestSkillUpgradeMessage(t *testing.T) {
	// 511 happy path: learned skill at Lv3 → 4, point decremented.
	p := PlayerProgress{ClassID: "class.kim", Level: 10, UnspentSkillPoints: 2,
		Skills: map[string]int32{"skill.kim.active.xuyen_phong": 3}}
	next, err := p.ApplySkillUpgrade("skill.kim.active.xuyen_phong", 3)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if next.Skills["skill.kim.active.xuyen_phong"] != 4 || next.UnspentSkillPoints != 1 {
		t.Fatalf("upgrade result = %+v", next)
	}
	// Upgrade to a kind cap: passive maxes at 6.
	passive := PlayerProgress{ClassID: "class.kim", Level: 55, UnspentSkillPoints: 3,
		Skills: map[string]int32{"skill.kim.passive.kiem_tam": 5}}
	next, err = passive.ApplySkillUpgrade("skill.kim.passive.kiem_tam", 5)
	if err != nil || next.Skills["skill.kim.passive.kiem_tam"] != 6 {
		t.Fatalf("passive 5→6: %v", err)
	}
	if _, err := next.ApplySkillUpgrade("skill.kim.passive.kiem_tam", 6); !mustReject(err, RejectSkillMaxLevel, t) {
	}
}

func TestPotentialAllocateAllOrNothing(t *testing.T) {
	p := PlayerProgress{ClassID: "class.kim", Level: 20,
		UnspentPotentialPoints: 20}
	if _, err := p.ApplyAllocate(PotentialDelta{}); !mustReject(err, RejectInvalidDelta, t) {
	}
	if _, err := p.ApplyAllocate(PotentialDelta{Str: 21}); !mustReject(err, RejectPotentialPointsInsufficient, t) {
	}
	if _, err := p.ApplyAllocate(PotentialDelta{Str: -1, Vit: 2}); !mustReject(err, RejectInvalidDelta, t) {
	}
	next, err := p.ApplyAllocate(PotentialDelta{Str: 5, Vit: 5, Int: 5, Agi: 5})
	if err != nil {
		t.Fatalf("spread allocate: %v", err)
	}
	if next.UnspentPotentialPoints != 0 || next.PotentialAllocated.Sum() != 20 {
		t.Fatalf("spread = %+v", next)
	}
}

func TestRespecMessage(t *testing.T) {
	// 513 covers both kinds; the NPC-admission layer is separate — here
	// the value-level semantics of the two kinds.
	p := PlayerProgress{ClassID: "class.thuy", Level: 30, UnspentSkillPoints: 4,
		UnspentPotentialPoints: 6,
		PotentialAllocated:     PotentialDelta{Int: 8, Agi: 10},
		Skills: map[string]int32{"skill.thuy.basic.thuy_tien": 7,
			"skill.thuy.active.luu_bo": 4, "skill.thuy.passive.han_khi": 6}}
	// SKILL: all learned → 1; refund Σ(level-1) = 6+3+5 = 14.
	next, out, err := p.ApplyRespec(RespecKindSkill, RespecPrice(30))
	if err != nil {
		t.Fatalf("skill respec: %v", err)
	}
	if out.SkillRefunded != 14 || next.UnspentSkillPoints != 18 {
		t.Fatalf("refund = %d unspent %d", out.SkillRefunded, next.UnspentSkillPoints)
	}
	for id, lvl := range next.Skills {
		if lvl != 1 {
			t.Fatalf("skill %s not reset: %d", id, lvl)
		}
	}
	if _, ok := next.Skills["skill.thuy.passive.han_khi"]; !ok {
		t.Fatal("respec must keep skills learned")
	}
	// POTENTIAL: allocations cleared, 18 refunded.
	next2, out2, _ := p.ApplyRespec(RespecKindPotential, RespecPrice(30))
	if out2.PotentialRefunded != 18 || next2.UnspentPotentialPoints != 24 ||
		next2.PotentialAllocated != (PotentialDelta{}) {
		t.Fatalf("potential respec = %+v", next2)
	}
}

func TestProgressionStatePush(t *testing.T) {
	// The 515 projection: learned skills in catalog document order, the
	// derived earned total, and the default loadout (basic_1 + learned
	// actives in document order padded to 5).
	p := PlayerProgress{ClassID: "class.kim", Level: 22, Exp: 300_000,
		UnspentSkillPoints: 2, UnspentPotentialPoints: 12,
		PotentialAllocated: PotentialDelta{Str: 10, Vit: 6},
		Skills: map[string]int32{
			"skill.kim.basic.kiem_thuc":       5,
			"skill.kim.basic.truy_phong_kiem": 1,
			"skill.kim.active.xuyen_phong":    3,
			"skill.kim.active.hoi_kiem":       1,
			"skill.kim.active.pha_giap":       1,
			"skill.kim.passive.kiem_tam":      2,
			"skill.kim.passive.lien_kiem":     1,
		}}
	v := Project(p, 7, nil)
	if v.Level != 22 || v.CurrentExp != 300_000 || v.UnspentSkillPoints != 2 ||
		v.UnspentPotentialPoints != 12 || v.PotentialEarnedTotal != 28 ||
		v.ProgressionRevision != 7 {
		t.Fatalf("projection = %+v", v)
	}
	// Skills ordered by catalog document order, not map order.
	wantOrder := []string{
		"skill.kim.basic.kiem_thuc", "skill.kim.basic.truy_phong_kiem",
		"skill.kim.passive.kiem_tam", "skill.kim.active.xuyen_phong",
		"skill.kim.passive.lien_kiem", "skill.kim.active.hoi_kiem",
		"skill.kim.active.pha_giap"}
	if len(v.Skills) != len(wantOrder) {
		t.Fatalf("skills len %d", len(v.Skills))
	}
	for i, w := range wantOrder {
		if v.Skills[i].SkillID != w {
			t.Fatalf("skills[%d] = %s want %s", i, v.Skills[i].SkillID, w)
		}
	}
	if v.Loadout.BasicSkillID != "skill.kim.basic.kiem_thuc" {
		t.Fatalf("basic = %q", v.Loadout.BasicSkillID)
	}
	wantSlots := [5]string{"skill.kim.active.xuyen_phong", "skill.kim.active.hoi_kiem",
		"skill.kim.active.pha_giap", "", ""}
	if v.Loadout.ActiveSlots != wantSlots {
		t.Fatalf("slots = %v", v.Loadout.ActiveSlots)
	}
	// A persisted SKILL_SET overrides the derived loadout.
	override := &Loadout{BasicSkillID: "skill.kim.basic.truy_phong_kiem"}
	v2 := Project(p, 8, override)
	if v2.Loadout.BasicSkillID != "skill.kim.basic.truy_phong_kiem" {
		t.Fatalf("override loadout not applied")
	}
}

func mustReject(err error, code Reject, t *testing.T) bool {
	t.Helper()
	if err == nil {
		t.Fatalf("expected rejection %s, got nil", code)
		return false
	}
	got, ok := RejectOf(err)
	if !ok || got != code {
		t.Fatalf("expected %s, got %v", code, err)
		return false
	}
	return true
}
