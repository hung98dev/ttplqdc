package progression

// Stat pipeline of stats.md: BASE (level-1 base + class growth) →
// FLAT_ADD → PERCENT_ADD → FINAL_MULTIPLY → per-stat CLAMP. All values
// are float64 until the final floor on integral stats.

// Stat enumerates the 21 final stats of stats.md.
type Stat uint8

const (
	StatMaxHP Stat = iota
	StatMaxMP
	StatAttack
	StatDefense
	StatHPRegen
	StatMPRegen
	StatCritChance
	StatCritDamage
	StatDamageBonus
	StatDamageReduction
	StatDodgeChance
	StatAccuracy
	StatMoveSpeed
	StatAttackSpeed
	StatCastSpeed
	StatCooldownReduction
	StatLifesteal
	StatHealReduction
	StatReflect
	StatAbsorb
	StatHealingReceived
	statCount
)

// Stats is one value per Stat.
type Stats [statCount]float64

// At returns the value for s.
func (b Stats) At(s Stat) float64 { return b[s] }

// MoveSpeed exposes the resolved move speed — the exported seam
// sim/movement consumes (plan file map).
func (b Stats) MoveSpeed() float64 { return b[StatMoveSpeed] }

// integralStats floor at the end of the pipeline.
var integralStats = [statCount]bool{
	StatMaxHP:   true,
	StatMaxMP:   true,
	StatAttack:  true,
	StatDefense: true,
	StatHPRegen: true,
	StatMPRegen: true,
}

// levelOneBase is the stats.md § Level 1 base table.
var levelOneBase = Stats{
	StatMaxHP:             500,
	StatMaxMP:             200,
	StatAttack:            40,
	StatDefense:           20,
	StatHPRegen:           2,
	StatMPRegen:           3,
	StatCritChance:        0.05,
	StatCritDamage:        1.50,
	StatDamageBonus:       0,
	StatDamageReduction:   0,
	StatDodgeChance:       0.03,
	StatAccuracy:          0,
	StatMoveSpeed:         1.00,
	StatAttackSpeed:       0,
	StatCastSpeed:         0,
	StatCooldownReduction: 0,
	StatLifesteal:         0,
	StatHealReduction:     0,
	StatReflect:           0,
	StatAbsorb:            0,
	StatHealingReceived:   1.00,
}

// classGrowth is the per-level growth table of stats.md § Class Growth —
// {HP, MP, ATK, DEF} per level; all classes add +0.12 HP_REGEN and +0.10
// MP_REGEN.
var classGrowth = map[string][6]float64{
	"class.kim":  {32, 8, 5.5, 2.0, 0.12, 0.10},
	"class.moc":  {36, 12, 4.8, 2.2, 0.12, 0.10},
	"class.thuy": {32, 12, 5.0, 1.9, 0.12, 0.10},
	"class.hoa":  {30, 14, 5.5, 1.8, 0.12, 0.10},
	"class.tho":  {44, 8, 4.4, 3.0, 0.12, 0.10},
}

// ComputeBase returns the level-adjusted base stats: level-1 base plus
// class growth × (level-1). Unknown classes get the level-1 base.
func ComputeBase(classID string, level int32) Stats {
	out := levelOneBase
	if level <= StartLevel {
		return out
	}
	g, ok := classGrowth[classID]
	if !ok {
		return out
	}
	n := float64(level - 1)
	out[StatMaxHP] += g[0] * n
	out[StatMaxMP] += g[1] * n
	out[StatAttack] += g[2] * n
	out[StatDefense] += g[3] * n
	out[StatHPRegen] += g[4] * n
	out[StatMPRegen] += g[5] * n
	return out
}

// ModifierKind is the pipeline stage order of stats.md § Modifier Order.
type ModifierKind uint8

const (
	ModifierFlatAdd ModifierKind = iota
	ModifierPercentAdd
	ModifierFinalMultiply
	ModifierClamp
)

// Modifier is one pipeline contribution.
type Modifier struct {
	Stat  Stat
	Kind  ModifierKind
	Value float64
}

// PotentialModifiers converts an allocation into its pipeline
// contributions per stats.md § Potential Stats:
//
//	STR — kim/tho +0.75 ATK per point, other classes +0.25.
//	INT — moc/thuy/hoa +0.75 ATK per point, kim/tho +0.25; every class
//	      also +1 MAX_MP per point.
//	VIT — +6 MAX_HP and +0.20 DEFENSE per point.
//	AGI — +0.0005 CRIT_CHANCE, +0.0004 DODGE_CHANCE, +0.0008 MOVE_SPEED
//	      (the AGI-derived MOVE_SPEED contribution alone is sub-capped at
//	      0.15) and +0.0002 COOLDOWN_REDUCTION per point.
func PotentialModifiers(classID string, alloc PotentialDelta) []Modifier {
	strPer, intPer := 0.25, 0.25
	switch classID {
	case "class.kim", "class.tho":
		strPer = 0.75
	default:
		intPer = 0.75
	}
	mods := []Modifier{
		{Stat: StatAttack, Kind: ModifierFlatAdd, Value: strPer * float64(alloc.Str)},
		{Stat: StatAttack, Kind: ModifierFlatAdd, Value: intPer * float64(alloc.Int)},
		{Stat: StatMaxMP, Kind: ModifierFlatAdd, Value: 1.0 * float64(alloc.Int)},
		{Stat: StatMaxHP, Kind: ModifierFlatAdd, Value: 6.0 * float64(alloc.Vit)},
		{Stat: StatDefense, Kind: ModifierFlatAdd, Value: 0.20 * float64(alloc.Vit)},
		{Stat: StatCritChance, Kind: ModifierFlatAdd, Value: 0.0005 * float64(alloc.Agi)},
		{Stat: StatDodgeChance, Kind: ModifierFlatAdd, Value: 0.0004 * float64(alloc.Agi)},
		{Stat: StatCooldownReduction, Kind: ModifierFlatAdd, Value: 0.0002 * float64(alloc.Agi)},
	}
	ms := 0.0008 * float64(alloc.Agi)
	if ms > 0.15 {
		ms = 0.15 // AGI-alone sub-cap (stats.md § MOVE_SPEED)
	}
	mods = append(mods, Modifier{Stat: StatMoveSpeed, Kind: ModifierFlatAdd, Value: ms})
	return mods
}

// statCaps is the CLAMP table — {min, max} per stat (max only when min is
// the natural zero floor); the PvP overrides live in statCapsPvP.
var statCaps = map[Stat][2]float64{
	StatCritChance:        {0, 0.60},
	StatCritDamage:        {0, 2.50},
	StatDodgeChance:       {0, 0.40},
	StatAccuracy:          {0, 0.40},
	StatDamageReduction:   {0, 0.40},
	StatLifesteal:         {0, 0.08},
	StatHealReduction:     {0, 0.30},
	StatReflect:           {0, 0.15},
	StatAbsorb:            {0, 0.10},
	StatAttackSpeed:       {0, 0.50},
	StatCastSpeed:         {0, 0.50},
	StatCooldownReduction: {0, 0.35},
	StatMoveSpeed:         {0.40, 1.50},
}

// statCapsPvP carries only the stats whose cap differs in PvP context.
var statCapsPvP = map[Stat][2]float64{
	StatLifesteal:     {0, 0.05},
	StatHealReduction: {0, 0.25},
	StatReflect:       {0, 0.08},
	StatAbsorb:        {0, 0.06},
}

// ComputeFinal resolves base + modifiers through the canonical order: BASE
// → FLAT_ADD (summed) → PERCENT_ADD (summed, then 1+p) → FINAL_MULTIPLY
// (product) → per-stat CLAMP → floor on integral stats. pvp selects the
// PvP clamp overrides.
func ComputeFinal(base Stats, modifiers []Modifier, pvp bool) Stats {
	var flat, pct, mul Stats
	for s := Stat(0); s < statCount; s++ {
		mul[s] = 1.0
	}
	for _, m := range modifiers {
		switch m.Kind {
		case ModifierFlatAdd:
			flat[m.Stat] += m.Value
		case ModifierPercentAdd:
			pct[m.Stat] += m.Value
		case ModifierFinalMultiply:
			mul[m.Stat] *= m.Value
		}
	}
	var out Stats
	for s := Stat(0); s < statCount; s++ {
		v := (base[s] + flat[s]) * (1 + pct[s]) * mul[s]
		c, capped := statCaps[s]
		if p, ok := statCapsPvP[s]; pvp && ok {
			c, capped = p, true
		}
		if capped {
			if v < c[0] {
				v = c[0]
			}
			if v > c[1] {
				v = c[1]
			}
		}
		out[s] = v
	}
	return out
}

// FloorIntegral returns the stats with integral stats floored (stats.md:
// final integer values floor).
func (b Stats) FloorIntegral() Stats {
	out := b
	for s := Stat(0); s < statCount; s++ {
		if integralStats[s] {
			out[s] = float64(int64(out[s]))
		}
	}
	return out
}

// DefenseMultiplier is the damage-reduction factor of stats.md:
// K/(K+max(0, DEFENSE)) with K = 100 + 20·target_level, floored at 0.25.
func DefenseMultiplier(defense float64, targetLevel int32) float64 {
	k := 100 + 20*float64(targetLevel)
	d := defense
	if d < 0 {
		d = 0
	}
	v := k / (k + d)
	if v < 0.25 {
		return 0.25
	}
	return v
}
