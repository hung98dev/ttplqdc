package effects

// templates.go — compiled-in registry of the canonical effect
// templates of class_skill_catalog.md § Canonical Effect Templates
// plus the ward/absorb templates of combat.md and the stat-driven
// HEAL_REDUCTION producer family of stats.md § HEAL_REDUCTION
// Resolution.

// Template is one compiled effect definition. Magnitude fields are
// expressed in the spec's units: multipliers as float64 fractions,
// additive stat changes as basis points of PERCENT_ADD.
type Template struct {
	ID   string
	Kind Kind
	Tags TagSet

	// DurationMs is the authored lifetime; 0 = instant (no instance).
	DurationMs int64
	// TickMs is the DoT interval (anchor + k*TickMs); 0 = no ticks.
	TickMs int64
	// PerTickAttackRatio is the DoT per-tick ATTACK ratio
	// (floor(snapshot * ratio) * stack_count).
	PerTickAttackRatio float64
	// Element is the damage element ("KIM","MOC","HOA","THUY","THO").
	Element string

	Reapply   ReapplyMode
	MaxStacks int32
	// PerSource keys instances (target, effect_id, source_id).
	PerSource bool
	// Control marks STUN/ROOT/FREEZE/AIRBORNE — reapply uses
	// expires_at = max(expires_at, now + duration).
	Control bool

	Dispellable         bool
	DispelPriority      int
	PersistAcrossMap    bool
	PersistThroughDeath bool
	ResidualExtensionMs int64 // marker lifetime past damage deadline

	// Magnitude fields (spec "exact payload" column).
	MoveSpeedMult        float64 // 0 = unset
	DefenseAddBP         int32
	AttackAddBP          int32
	CritChanceAddBP      int32
	HealingReceivedMult  float64 // 0 = unset
	ElementDmgTakenMult  float64 // 0 = unset
	DamageReductionAddBP int32

	// ImmunityTags the active POSITIVE instance confers on its holder.
	ImmunityTags TagSet
	// RemoveTagsOnApply strips matching NEGATIVE instances on apply
	// (luu_bo_haste removes SLOW).
	RemoveTagsOnApply TagSet
	// LinkedShield is the shield effect_id a ward's lifetime follows.
	LinkedShield string

	// Shield capacity coefficients (KindShield only).
	ShieldMaxHPCoef   float64
	ShieldAttackCoef  float64
	ShieldDefenseCoef float64
}

// dot builds a DoT template row.
func dot(id, element string, ratio float64, durMs, tickMs int64, stacks int32, dispellable bool, re ReapplyMode) *Template {
	return &Template{
		ID: id, Kind: KindDoT,
		Tags:    TagNegative | TagDoT,
		Element: element, DurationMs: durMs, TickMs: tickMs,
		PerTickAttackRatio: ratio, Reapply: re,
		MaxStacks: stacks, PerSource: true,
		Dispellable: dispellable, DispelPriority: 50,
	}
}

// shield builds a shield template row (capacity resolved at grant).
func shield(id string, hpCoef, atkCoef, defCoef float64, durMs int64) *Template {
	return &Template{
		ID: id, Kind: KindShield, DurationMs: durMs,
		Reapply:         RefreshDuration,
		ShieldMaxHPCoef: hpCoef, ShieldAttackCoef: atkCoef,
		ShieldDefenseCoef: defCoef,
	}
}

var templates = map[string]*Template{
	// ---- DoT family (class_skill_catalog DoT table) ----
	"effect.basic.bleed_3s":        dot("effect.basic.bleed_3s", "KIM", 0.10, 3000, 1000, 1, true, RefreshDuration),
	"effect.basic.poison_4s":       dot("effect.basic.poison_4s", "MOC", 0.08, 4000, 1000, 1, true, RefreshDuration),
	"effect.basic.poison_stack_4s": dot("effect.basic.poison_stack_4s", "MOC", 0.08, 4000, 1000, 3, true, Stack),
	"effect.basic.burn_3s":         dot("effect.basic.burn_3s", "HOA", 0.12, 3000, 1000, 1, true, RefreshDuration),
	"effect.basic.burn_true_3s":    dot("effect.basic.burn_true_3s", "HOA", 0.16, 3000, 1000, 1, false, RefreshDuration),

	// ---- Non-DoT negatives ----
	"effect.basic.vulnerable_8_4s": {
		ID: "effect.basic.vulnerable_8_4s", Kind: KindStatus,
		Tags: TagNegative | TagVulnerable, DurationMs: 4000,
		Reapply: RefreshDuration, Dispellable: true, DispelPriority: 50,
		DefenseAddBP: -800,
	},
	"effect.skill.kim.pha_giap_vulnerable_5s": {
		ID: "effect.skill.kim.pha_giap_vulnerable_5s", Kind: KindStatus,
		Tags: TagNegative | TagVulnerable, DurationMs: 5000,
		Reapply: RefreshDuration, Dispellable: true, DispelPriority: 50,
		DefenseAddBP: -1500,
	},
	"effect.basic.crit_mark_3s": {
		ID: "effect.basic.crit_mark_3s", Kind: KindStatus,
		Tags: TagNegative | TagCritMark, DurationMs: 3000,
		Reapply: RefreshDuration, PerSource: true,
		Dispellable: true, DispelPriority: 50,
		CritChanceAddBP: 1000,
	},
	"effect.basic.heal_reduction_3s": {
		ID: "effect.basic.heal_reduction_3s", Kind: KindStatus,
		Tags: TagNegative | TagHealReduction, DurationMs: 3000,
		Reapply: RefreshDuration, Dispellable: true, DispelPriority: 50,
		HealingReceivedMult: 0.75,
	},
	// Stat-driven HEAL_REDUCTION family (stats.md § HEAL_REDUCTION
	// Resolution): applied by the attacker's stat on committed
	// hp_damage > 0 at magnitude (1 − HEAL_REDUCTION) for 4.0s,
	// refreshed on re-application; per-source key.
	"effect.stat.heal_reduction_4s": {
		ID: "effect.stat.heal_reduction_4s", Kind: KindStatus,
		Tags: TagNegative | TagHealReduction, DurationMs: 4000,
		Reapply: RefreshDuration, PerSource: true,
		Dispellable: true, DispelPriority: 50,
		HealingReceivedMult: 1.0, // overridden per-request by stat
	},
	"effect.basic.slow_15_2s": {
		ID: "effect.basic.slow_15_2s", Kind: KindStatus,
		Tags: TagNegative | TagSlow, DurationMs: 2000,
		Reapply: RefreshDuration, Dispellable: true, DispelPriority: 50,
		MoveSpeedMult: 0.85,
	},
	"effect.basic.slow_20_3s": {
		ID: "effect.basic.slow_20_3s", Kind: KindStatus,
		Tags: TagNegative | TagSlow, DurationMs: 3000,
		Reapply: RefreshDuration, Dispellable: true, DispelPriority: 50,
		MoveSpeedMult: 0.80,
	},
	"effect.basic.slow_25_3s": {
		ID: "effect.basic.slow_25_3s", Kind: KindStatus,
		Tags: TagNegative | TagSlow, DurationMs: 3000,
		Reapply: RefreshDuration, Dispellable: true, DispelPriority: 50,
		MoveSpeedMult: 0.75,
	},
	"effect.skill.thuy.slow_40_3s": {
		ID: "effect.skill.thuy.slow_40_3s", Kind: KindStatus,
		Tags: TagNegative | TagSlow, DurationMs: 3000,
		Reapply: RefreshDuration, Dispellable: true, DispelPriority: 50,
		MoveSpeedMult: 0.60,
	},
	"effect.skill.kim.kiem_tran_slow": {
		ID: "effect.skill.kim.kiem_tran_slow", Kind: KindStatus,
		Tags: TagNegative | TagSlow, DurationMs: 600,
		Reapply: RefreshDuration, Dispellable: true, DispelPriority: 50,
		MoveSpeedMult: 0.70,
	},
	"effect.skill.moc.thanh_dang_slow": {
		ID: "effect.skill.moc.thanh_dang_slow", Kind: KindStatus,
		Tags: TagNegative | TagSlow, DurationMs: 1100,
		Reapply: RefreshDuration, Dispellable: true, DispelPriority: 50,
		MoveSpeedMult: 0.60,
	},
	"effect.skill.thuy.chill_3s": {
		ID: "effect.skill.thuy.chill_3s", Kind: KindStatus,
		Tags: TagNegative | TagChill, DurationMs: 3000,
		Reapply: Stack, MaxStacks: 3, PerSource: true,
		Dispellable: true, DispelPriority: 50,
	},
	"effect.basic.freeze_1200ms": {
		ID: "effect.basic.freeze_1200ms", Kind: KindStatus,
		Tags:       TagNegative | TagFreeze | TagHardControl | TagMovementControl | TagControl,
		DurationMs: 1200, Reapply: RefreshDuration, Control: true,
		Dispellable: true, DispelPriority: 50,
	},
	"effect.skill.thuy.freeze_han_khi": {
		ID: "effect.skill.thuy.freeze_han_khi", Kind: KindStatus,
		Tags:       TagNegative | TagFreeze | TagHardControl | TagMovementControl | TagControl,
		DurationMs: 0, Reapply: RefreshDuration, Control: true, // duration comes from han_khi passive via Request.DurationMs
		Dispellable: true, DispelPriority: 50,
	},
	"effect.skill.thuy.freeze_2000ms": {
		ID: "effect.skill.thuy.freeze_2000ms", Kind: KindStatus,
		Tags:       TagNegative | TagFreeze | TagHardControl | TagMovementControl | TagControl,
		DurationMs: 2000, Reapply: RefreshDuration, Control: true,
		Dispellable: true, DispelPriority: 50,
	},
	"effect.basic.stun_400ms": {
		ID: "effect.basic.stun_400ms", Kind: KindStatus,
		Tags:       TagNegative | TagStun | TagHardControl | TagMovementControl | TagControl,
		DurationMs: 400, Reapply: RefreshDuration, Control: true,
		Dispellable: true, DispelPriority: 50,
	},
	"effect.basic.stun_500ms": {
		ID: "effect.basic.stun_500ms", Kind: KindStatus,
		Tags:       TagNegative | TagStun | TagHardControl | TagMovementControl | TagControl,
		DurationMs: 500, Reapply: RefreshDuration, Control: true,
		Dispellable: true, DispelPriority: 50,
	},
	"effect.skill.tho.stun_1500ms": {
		ID: "effect.skill.tho.stun_1500ms", Kind: KindStatus,
		Tags:       TagNegative | TagStun | TagHardControl | TagMovementControl | TagControl,
		DurationMs: 1500, Reapply: RefreshDuration, Control: true,
		Dispellable: true, DispelPriority: 50,
	},
	"effect.basic.root_1200ms": {
		ID: "effect.basic.root_1200ms", Kind: KindStatus,
		Tags:       TagNegative | TagRoot | TagHardControl | TagMovementControl | TagControl,
		DurationMs: 1200, Reapply: RefreshDuration, Control: true,
		Dispellable: true, DispelPriority: 50,
	},
	"effect.skill.moc.root_1500ms": {
		ID: "effect.skill.moc.root_1500ms", Kind: KindStatus,
		Tags:       TagNegative | TagRoot | TagHardControl | TagMovementControl | TagControl,
		DurationMs: 1500, Reapply: RefreshDuration, Control: true,
		Dispellable: true, DispelPriority: 50,
	},
	"effect.skill.tho.airborne_800ms": {
		ID: "effect.skill.tho.airborne_800ms", Kind: KindStatus,
		Tags:       TagNegative | TagAirborne | TagHardControl | TagMovementControl | TagDisplacement | TagControl,
		DurationMs: 800, Reapply: RefreshDuration, Control: true,
		Dispellable: true, DispelPriority: 50,
	},
	"effect.basic.resist_shred_hoa_3s": {
		ID: "effect.basic.resist_shred_hoa_3s", Kind: KindStatus,
		Tags: TagNegative | TagResistShred, DurationMs: 3000,
		Reapply: RefreshDuration, Dispellable: true, DispelPriority: 50,
		Element: "HOA", ElementDmgTakenMult: 1.10,
	},
	"effect.basic.weaken_10_3s": {
		ID: "effect.basic.weaken_10_3s", Kind: KindStatus,
		Tags: TagNegative | TagWeaken, DurationMs: 3000,
		Reapply: RefreshDuration, Dispellable: true, DispelPriority: 50,
		AttackAddBP: -1000,
	},
	"effect.basic.weaken_12_3s": {
		ID: "effect.basic.weaken_12_3s", Kind: KindStatus,
		Tags: TagNegative | TagWeaken, DurationMs: 3000,
		Reapply: RefreshDuration, Dispellable: true, DispelPriority: 50,
		AttackAddBP: -1200,
	},

	// ---- Positives ----
	"effect.skill.kim.hoi_kiem_guard": {
		ID: "effect.skill.kim.hoi_kiem_guard", Kind: KindStatus,
		Tags: TagPositive, DurationMs: 2000,
		Reapply:              RefreshDuration,
		DamageReductionAddBP: 2000,
	},
	"effect.skill.thuy.luu_bo_haste": {
		ID: "effect.skill.thuy.luu_bo_haste", Kind: KindStatus,
		Tags: TagPositive, DurationMs: 1500,
		Reapply:           RefreshDuration,
		MoveSpeedMult:     1.30,
		RemoveTagsOnApply: TagSlow,
	},
	"effect.skill.thuy.thuy_kinh_ward": {
		ID: "effect.skill.thuy.thuy_kinh_ward", Kind: KindStatus,
		Tags:         TagPositive | TagSlowImmune | TagWard,
		Reapply:      RefreshDuration,
		ImmunityTags: TagSlowImmune,
		LinkedShield: "effect.skill.thuy.thuy_kinh_shield",
	},
	"effect.skill.tho.tho_giap_ward": {
		ID: "effect.skill.tho.tho_giap_ward", Kind: KindStatus,
		Tags:         TagPositive | TagDisplacementImmune | TagWard,
		Reapply:      RefreshDuration,
		ImmunityTags: TagDisplacementImmune,
		LinkedShield: "effect.skill.tho.tho_giap_shield",
	},
	"effect.skill.hoa.hoa_giap_aura": {
		ID: "effect.skill.hoa.hoa_giap_aura", Kind: KindStatus,
		Tags: TagPositive | TagReactive, DurationMs: 6000,
		Reapply:       RefreshDuration,
		MoveSpeedMult: 1.15,
	},
	"effect.skill.hoa.cuong_hoa_buff": {
		ID: "effect.skill.hoa.cuong_hoa_buff", Kind: KindStatus,
		Tags: TagPositive, DurationMs: 4000,
		Reapply: RefreshDuration, // magnitude comes from cuong_hoa passive via Request
	},
	"effect.skill.thuy.bang_giap_guard": {
		ID: "effect.skill.thuy.bang_giap_guard", Kind: KindStatus,
		Tags: TagPositive, DurationMs: 3000,
		Reapply: RefreshDuration, // magnitude comes from bang_giap_tam passive
	},

	// ---- Shields ----
	"effect.skill.thuy.thuy_kinh_shield":     shield("effect.skill.thuy.thuy_kinh_shield", 0.15, 0.30, 0, 5000),
	"effect.skill.tho.tho_giap_shield":       shield("effect.skill.tho.tho_giap_shield", 0.18, 0, 0.40, 6000),
	"effect.skill.tho.thien_son_tran_shield": shield("effect.skill.tho.thien_son_tran_shield", 0.15, 0, 0, 6000),
	// combat.md § ABSORB: shield on the attacker, 6.0s lifetime;
	// capacity = granted absorb amount (already capped upstream).
	"effect.absorb.self":            shield("effect.absorb.self", 0, 0, 0, 6000),
	"effect.beast.emergency_shield": shield("effect.beast.emergency_shield", 0.30, 0, 0, 5000),
}

// Template returns the compiled row for effect_id.
func TemplateByID(id string) (*Template, bool) {
	t, ok := templates[id]
	return t, ok
}
