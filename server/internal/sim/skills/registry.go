package skills

import (
	"fmt"
	"sort"

	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/combat"
)

// Compiled catalog tables — the authoritative source rows of
// docs/07_content/class_skill_catalog.md. 45 basic/active launch
// definitions (document order per class); passives are excluded (no
// execution geometry). Reach values are mm; durations ms; projectile
// speed mm/s; proc chances basis points; cooldown step microseconds.

func box(k combat.GeometryKind, reach, hh int64) GeometrySpec {
	return GeometrySpec{Kind: k, Box: &BoxGeom{ReachMM: reach, HalfHeightMM: hh}}
}

func proj(rangeMM, speed, radius int64) GeometrySpec {
	return GeometrySpec{Kind: combat.GeomProjectile, Projectile: &ProjGeom{RangeMM: rangeMM, SpeedMMPerS: speed, RadiusMM: radius}}
}

func circle(r int64) GeometrySpec {
	return GeometrySpec{Kind: combat.GeomAreaSelf, Circle: &CircleGeom{RadiusMM: r}}
}

func areaCircle(cast, radius int64) GeometrySpec {
	return GeometrySpec{Kind: combat.GeomAreaPosition, Area: &AreaGeom{CastMM: cast, RadiusMM: radius}}
}

func singleRange(r int64) GeometrySpec {
	return GeometrySpec{Kind: combat.GeomSingleTargetRange, Range: &RangeGeom{RangeMM: r}}
}

func line(k combat.GeometryKind, dist, dur, hh int64) GeometrySpec {
	return GeometrySpec{Kind: k, Line: &LineGeom{DistanceMM: dist, DurationMs: dur, HitHalfHeightMM: hh}}
}

func barrierGeom(cast, thickness, height, dur int64) GeometrySpec {
	return GeometrySpec{Kind: combat.GeomBarrier, Barrier: &BarrierGeom{CastMM: cast, ThicknessMM: thickness, HeightMM: height, DurationMs: dur}}
}

func dmg(coef float64) Payload  { return Payload{Kind: PayDamage, Coefficient: coef} }
func st(id string) Payload      { return Payload{Kind: PayStatus, RefID: id} }
func stFirst(id string) Payload { return Payload{Kind: PayStatus, RefID: id, FirstHitOnly: true} }
func sh(id string) Payload      { return Payload{Kind: PayShield, RefID: id} }
func sp(id string) Payload      { return Payload{Kind: PaySpatial, RefID: id} }
func zone(id string) Payload    { return Payload{Kind: PayZone, RefID: id} }

const (
	kim  = "class.kim"
	moc  = "class.moc"
	thuy = "class.thuy"
	hoa  = "class.hoa"
	tho  = "class.tho"
)

var registry = map[string]*Def{
	// ---- KIM — Kiếm Khách ----
	"skill.kim.basic.kiem_thuc": {
		ID: "skill.kim.basic.kiem_thuc", ClassID: kim, Element: protocolv1.Element_ELEMENT_KIM,
		Kind: KindBasic, Unlock: 1, Execution: ExecInstant, Targeting: TargetDirection,
		Tags: TagBasicAttack | TagDamaging | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 160, ActiveMs: 100, RecoveryMs: 240,
		Geom:           box(combat.GeomMeleeBox, 1900, 900),
		AirGeom:        &[]GeometrySpec{box(combat.GeomMeleeBox, 1900, 1200)}[0],
		BaseCooldownMs: 500, Band: combat.BandBasic1,
		BaseCoefficient: 1.00, MaxCooldownMs: 200, CooldownStepUs: 27300,
		BaseProcBP: 800, MaxProcBP: 2500, ProcEffects: []string{"effect.basic.bleed_3s"},
		RestoresMP: 2,
	},
	"skill.kim.basic.truy_phong_kiem": {
		ID: "skill.kim.basic.truy_phong_kiem", ClassID: kim, Element: protocolv1.Element_ELEMENT_KIM,
		Kind: KindBasic, Unlock: 4, Execution: ExecDashAttack, Targeting: TargetDirection,
		Tags: TagBasicAttack | TagDamaging | TagStatusApply | TagMovement, Air: AirAll,
		MoveBehavior: MoveForced,
		Speed:        combat.TimingAttackSpeed, StartupMs: 170, ActiveMs: 100, RecoveryMs: 250,
		Geom:           line(combat.GeomDashLine, 2400, 180, 900),
		BaseCooldownMs: 520, Band: combat.BandBasic2,
		BaseCoefficient: 1.08, MaxCooldownMs: 220, CooldownStepUs: 27300,
		BaseProcBP: 600, MaxProcBP: 2200, ProcEffects: []string{"effect.basic.vulnerable_8_4s"},
	},
	"skill.kim.basic.pha_khong_kiem": {
		ID: "skill.kim.basic.pha_khong_kiem", ClassID: kim, Element: protocolv1.Element_ELEMENT_KIM,
		Kind: KindBasic, Unlock: 18, Execution: ExecInstant, Targeting: TargetDirection,
		Tags: TagBasicAttack | TagDamaging | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 180, ActiveMs: 100, RecoveryMs: 280,
		Geom:           box(combat.GeomDirectionBox, 3200, 900),
		BaseCooldownMs: 560, Band: combat.BandBasic3,
		BaseCoefficient: 1.15, MaxCooldownMs: 240, CooldownStepUs: 29100,
		BaseProcBP: 700, MaxProcBP: 2400, ProcEffects: []string{"effect.basic.crit_mark_3s"},
	},
	"skill.kim.basic.vo_song_kiem": {
		ID: "skill.kim.basic.vo_song_kiem", ClassID: kim, Element: protocolv1.Element_ELEMENT_KIM,
		Kind: KindBasic, Unlock: 36, Execution: ExecInstant, Targeting: TargetDirection,
		Tags: TagBasicAttack | TagDamaging | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 150, ActiveMs: 100, RecoveryMs: 250,
		Geom:           box(combat.GeomMeleeBox, 2400, 1000),
		BaseCooldownMs: 500, Band: combat.BandBasic4,
		BaseCoefficient: 1.25, MaxCooldownMs: 200, CooldownStepUs: 27300,
		BaseProcBP: 800, MaxProcBP: 2500, ProcEffects: []string{"effect.basic.bleed_3s"},
		PenetrationBP: 1500,
	},
	"skill.kim.active.xuyen_phong": {
		ID: "skill.kim.active.xuyen_phong", ClassID: kim, Element: protocolv1.Element_ELEMENT_KIM,
		Kind: KindActive, Unlock: 8, Execution: ExecDashAttack, Targeting: TargetDirection,
		Tags: TagDamaging | TagMovement | TagStatusApply, Air: AirAll, MoveBehavior: MoveForced,
		Speed: combat.TimingCastSpeed, StartupMs: 120, ActiveMs: 300, RecoveryMs: 240,
		Geom:           line(combat.GeomDashLine, 4200, 300, 900),
		BaseCooldownMs: 8000, CostMP: 12, Band: combat.BandSingleTarget,
		Payloads: []Payload{dmg(1.20), stFirst("effect.basic.bleed_3s")},
	},
	"skill.kim.active.hoi_kiem": {
		ID: "skill.kim.active.hoi_kiem", ClassID: kim, Element: protocolv1.Element_ELEMENT_KIM,
		Kind: KindActive, Unlock: 14, Execution: ExecArea, Targeting: TargetAreaSelf,
		Tags: TagDamaging | TagArea | TagDefensive, Air: AirAll,
		Speed: combat.TimingCastSpeed, StartupMs: 240, ActiveMs: 140, RecoveryMs: 320,
		Geom:           circle(2800),
		BaseCooldownMs: 12000, CostMP: 16, Band: combat.BandCleave,
		Payloads: []Payload{dmg(1.30), st("effect.skill.kim.hoi_kiem_guard")},
	},
	"skill.kim.active.pha_giap": {
		ID: "skill.kim.active.pha_giap", ClassID: kim, Element: protocolv1.Element_ELEMENT_KIM,
		Kind: KindActive, Unlock: 22, Execution: ExecInstant, Targeting: TargetDirection,
		Tags: TagDamaging | TagArea | TagStatusApply, Air: AirAll,
		Speed: combat.TimingCastSpeed, StartupMs: 280, ActiveMs: 120, RecoveryMs: 360,
		Geom:           box(combat.GeomMeleeBox, 2500, 900),
		BaseCooldownMs: 10000, CostMP: 18, Band: combat.BandCleave,
		Payloads: []Payload{dmg(1.55), st("effect.skill.kim.pha_giap_vulnerable_5s")},
	},
	"skill.kim.active.kiem_tran": {
		ID: "skill.kim.active.kiem_tran", ClassID: kim, Element: protocolv1.Element_ELEMENT_KIM,
		Kind: KindActive, Unlock: 32, Execution: ExecArea, Targeting: TargetAreaSelf,
		Tags: TagDamaging | TagArea | TagStatusApply, Air: AirGround,
		Speed: combat.TimingCastSpeed, StartupMs: 360, ActiveMs: 80, RecoveryMs: 400,
		Geom:           circle(3200),
		BaseCooldownMs: 16000, CostMP: 24, Band: combat.BandWide,
		Payloads: []Payload{zone("zone.kim.kiem_tran")},
	},
	"skill.kim.active.nhat_kiem_dinh_hon": {
		ID: "skill.kim.active.nhat_kiem_dinh_hon", ClassID: kim, Element: protocolv1.Element_ELEMENT_KIM,
		Kind: KindActive, Unlock: 45, Execution: ExecInstant, Targeting: TargetSingle,
		Tags: TagDamaging | TagSignature, Air: AirAll,
		Speed: combat.TimingCastSpeed, StartupMs: 520, ActiveMs: 120, RecoveryMs: 480,
		Geom:           singleRange(2600),
		BaseCooldownMs: 24000, CostMP: 35, Band: combat.BandSingleTarget,
		Payloads: []Payload{
			{Kind: PayExecute, Ratio: 0.30, Bonus: 0.50},
			dmg(3.20),
		},
	},

	// ---- MOC — Dược Sư ----
	"skill.moc.basic.linh_diep": {
		ID: "skill.moc.basic.linh_diep", ClassID: moc, Element: protocolv1.Element_ELEMENT_MOC,
		Kind: KindBasic, Unlock: 1, Execution: ExecProjectile, Targeting: TargetProjectile,
		Tags: TagBasicAttack | TagDamaging | TagProjectile | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 200, ActiveMs: 50, RecoveryMs: 450,
		Geom:           proj(7500, 11000, 250),
		BaseCooldownMs: 700, Band: combat.BandBasic1,
		BaseCoefficient: 0.90, MaxCooldownMs: 350, CooldownStepUs: 31800,
		BaseProcBP: 800, MaxProcBP: 2500, ProcEffects: []string{"effect.basic.poison_4s"},
		RestoresMP: 2,
	},
	"skill.moc.basic.thao_kich": {
		ID: "skill.moc.basic.thao_kich", ClassID: moc, Element: protocolv1.Element_ELEMENT_MOC,
		Kind: KindBasic, Unlock: 4, Execution: ExecProjectile, Targeting: TargetProjectile,
		Tags: TagBasicAttack | TagDamaging | TagProjectile | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 210, ActiveMs: 50, RecoveryMs: 460,
		Geom:           proj(8000, 11500, 250),
		BaseCooldownMs: 720, Band: combat.BandBasic2,
		BaseCoefficient: 0.95, MaxCooldownMs: 360, CooldownStepUs: 32700,
		BaseProcBP: 700, MaxProcBP: 2300,
		ProcEffects: []string{"effect.basic.poison_4s", "effect.basic.heal_reduction_3s"},
	},
	"skill.moc.basic.truc_phi_tieu": {
		ID: "skill.moc.basic.truc_phi_tieu", ClassID: moc, Element: protocolv1.Element_ELEMENT_MOC,
		Kind: KindBasic, Unlock: 18, Execution: ExecProjectile, Targeting: TargetProjectile,
		Tags: TagBasicAttack | TagDamaging | TagProjectile | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 190, ActiveMs: 50, RecoveryMs: 460,
		Geom:           proj(8500, 12500, 220),
		BaseCooldownMs: 700, Band: combat.BandBasic3,
		BaseCoefficient: 1.02, MaxCooldownMs: 350, CooldownStepUs: 31800,
		BaseProcBP: 800, MaxProcBP: 2500, ProcEffects: []string{"effect.basic.poison_stack_4s"},
	},
	"skill.moc.basic.co_thu_kich": {
		ID: "skill.moc.basic.co_thu_kich", ClassID: moc, Element: protocolv1.Element_ELEMENT_MOC,
		Kind: KindBasic, Unlock: 36, Execution: ExecProjectile, Targeting: TargetProjectile,
		Tags: TagBasicAttack | TagDamaging | TagProjectile | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 200, ActiveMs: 60, RecoveryMs: 430,
		Geom:           proj(8000, 10500, 300),
		BaseCooldownMs: 690, Band: combat.BandBasic4,
		BaseCoefficient: 1.15, MaxCooldownMs: 340, CooldownStepUs: 31800,
		BaseProcBP: 800, MaxProcBP: 2400,
		ProcEffects: []string{"effect.basic.poison_4s", "effect.basic.slow_15_2s"},
	},
	"skill.moc.active.moc_bo": {
		ID: "skill.moc.active.moc_bo", ClassID: moc, Element: protocolv1.Element_ELEMENT_MOC,
		Kind: KindActive, Unlock: 8, Execution: ExecArea, Targeting: TargetAreaPosition,
		Tags: TagDamaging | TagArea | TagStatusApply, Air: AirGround,
		Speed: combat.TimingCastSpeed, StartupMs: 360, ActiveMs: 100, RecoveryMs: 400,
		Geom:           areaCircle(7000, 2500),
		BaseCooldownMs: 9000, CostMP: 14, Band: combat.BandCleave,
		Payloads: []Payload{
			dmg(1.10),
			st("effect.skill.moc.root_1500ms"),
			stFirst("effect.basic.poison_4s"),
		},
	},
	"skill.moc.active.hoi_xuan": {
		ID: "skill.moc.active.hoi_xuan", ClassID: moc, Element: protocolv1.Element_ELEMENT_MOC,
		Kind: KindActive, Unlock: 14, Execution: ExecInstant, Targeting: TargetSingle,
		Tags: TagHeal | TagDefensive, Air: AirAll,
		Speed: combat.TimingCastSpeed, StartupMs: 280, ActiveMs: 80, RecoveryMs: 320,
		Geom:           singleRange(7500),
		BaseCooldownMs: 8000, CostMP: 16, Band: combat.BandSingleTarget,
		Payloads: []Payload{{Kind: PayHeal, Ratio: 0.12, Coefficient: 0.25}},
	},
	"skill.moc.active.thanh_dang": {
		ID: "skill.moc.active.thanh_dang", ClassID: moc, Element: protocolv1.Element_ELEMENT_MOC,
		Kind: KindActive, Unlock: 22, Execution: ExecArea, Targeting: TargetAreaPosition,
		Tags: TagDamaging | TagArea | TagStatusApply, Air: AirGround,
		Speed: combat.TimingCastSpeed, StartupMs: 380, ActiveMs: 80, RecoveryMs: 380,
		Geom:           areaCircle(7000, 3000),
		BaseCooldownMs: 14000, CostMP: 20, Band: combat.BandCleave,
		Payloads: []Payload{zone("zone.moc.thanh_dang")},
	},
	"skill.moc.active.van_doc": {
		ID: "skill.moc.active.van_doc", ClassID: moc, Element: protocolv1.Element_ELEMENT_MOC,
		Kind: KindActive, Unlock: 32, Execution: ExecArea, Targeting: TargetAreaPosition,
		// Van Độc mist is periodic damage, not a separate POISON
		// instance — STATUS_APPLY is removed per the payload note.
		Tags: TagDamaging | TagArea, Air: AirAll,
		Speed: combat.TimingCastSpeed, StartupMs: 340, ActiveMs: 100, RecoveryMs: 380,
		Geom:           areaCircle(7500, 2800),
		BaseCooldownMs: 12000, CostMP: 22, Band: combat.BandWide,
		Payloads: []Payload{dmg(1.40), zone("zone.moc.van_doc")},
	},
	"skill.moc.active.van_moc_hoi_sinh": {
		ID: "skill.moc.active.van_moc_hoi_sinh", ClassID: moc, Element: protocolv1.Element_ELEMENT_MOC,
		Kind: KindActive, Unlock: 45, Execution: ExecArea, Targeting: TargetAreaPosition,
		Tags: TagDamaging | TagArea | TagHeal | TagDisplacement | TagDefensive | TagSignature, Air: AirGround,
		Speed: combat.TimingCastSpeed, StartupMs: 540, ActiveMs: 100, RecoveryMs: 500,
		Geom:           areaCircle(7000, 3500),
		BaseCooldownMs: 25000, CostMP: 36, Band: combat.BandWide,
		Payloads: []Payload{
			dmg(1.50),
			sp("spatial.skill.moc.active.van_moc_hoi_sinh.knockback"),
			zone("zone.moc.van_moc_hoi_sinh"),
		},
	},

	// ---- THUY — Thủy Sư ----
	"skill.thuy.basic.thuy_tien": {
		ID: "skill.thuy.basic.thuy_tien", ClassID: thuy, Element: protocolv1.Element_ELEMENT_THUY,
		Kind: KindBasic, Unlock: 1, Execution: ExecProjectile, Targeting: TargetProjectile,
		Tags: TagBasicAttack | TagDamaging | TagProjectile | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 190, ActiveMs: 50, RecoveryMs: 410,
		Geom:           proj(8000, 12000, 230),
		BaseCooldownMs: 650, Band: combat.BandBasic1,
		BaseCoefficient: 0.92, MaxCooldownMs: 300, CooldownStepUs: 31800,
		BaseProcBP: 800, MaxProcBP: 2500,
		ProcEffects: []string{"effect.skill.thuy.chill_3s", "effect.basic.slow_20_3s"},
		RestoresMP:  2,
	},
	"skill.thuy.basic.bang_phien": {
		ID: "skill.thuy.basic.bang_phien", ClassID: thuy, Element: protocolv1.Element_ELEMENT_THUY,
		Kind: KindBasic, Unlock: 4, Execution: ExecProjectile, Targeting: TargetProjectile,
		Tags: TagBasicAttack | TagDamaging | TagProjectile | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 190, ActiveMs: 50, RecoveryMs: 410,
		Geom:           proj(8200, 12500, 240),
		BaseCooldownMs: 650, Band: combat.BandBasic2,
		BaseCoefficient: 0.98, MaxCooldownMs: 300, CooldownStepUs: 31800,
		BaseProcBP: 700, MaxProcBP: 2400, ProcEffects: []string{"effect.skill.thuy.chill_3s"},
		GuaranteedEvery: 3, // every 3rd connected hit applies 1 CHILL stack regardless of proc
	},
	"skill.thuy.basic.am_luu": {
		ID: "skill.thuy.basic.am_luu", ClassID: thuy, Element: protocolv1.Element_ELEMENT_THUY,
		Kind: KindBasic, Unlock: 18, Execution: ExecProjectile, Targeting: TargetProjectile,
		Tags: TagBasicAttack | TagDamaging | TagProjectile | TagStatusApply | TagDisplacement, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 200, ActiveMs: 60, RecoveryMs: 420,
		Geom:           proj(7500, 10000, 300),
		BaseCooldownMs: 680, Band: combat.BandBasic3,
		BaseCoefficient: 1.05, MaxCooldownMs: 320, CooldownStepUs: 32700,
		BaseProcBP: 600, MaxProcBP: 2200, ProcEffects: []string{"effect.basic.slow_25_3s"},
		ProcSpatial: "spatial.skill.thuy.basic.am_luu.knockback",
	},
	"skill.thuy.basic.huyen_bang_kich": {
		ID: "skill.thuy.basic.huyen_bang_kich", ClassID: thuy, Element: protocolv1.Element_ELEMENT_THUY,
		Kind: KindBasic, Unlock: 36, Execution: ExecProjectile, Targeting: TargetProjectile,
		Tags: TagBasicAttack | TagDamaging | TagProjectile | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 180, ActiveMs: 50, RecoveryMs: 390,
		Geom:           proj(8500, 13000, 250),
		BaseCooldownMs: 620, Band: combat.BandBasic4,
		BaseCoefficient: 1.18, MaxCooldownMs: 300, CooldownStepUs: 29100,
		BaseProcBP: 500, MaxProcBP: 1800, ProcEffects: []string{"effect.basic.freeze_1200ms"},
	},
	"skill.thuy.active.luu_bo": {
		ID: "skill.thuy.active.luu_bo", ClassID: thuy, Element: protocolv1.Element_ELEMENT_THUY,
		Kind: KindActive, Unlock: 8, Execution: ExecMovement, Targeting: TargetDirection,
		Tags: TagMovement | TagStatusApply, Air: AirAll, MoveBehavior: MoveForced,
		Speed: combat.TimingNone, StartupMs: 100, ActiveMs: 280, RecoveryMs: 220,
		Geom:           line(combat.GeomMoveContact, 4500, 280, 1000),
		BaseCooldownMs: 6000, CostMP: 10, Band: combat.BandSingleTarget,
		Payloads: []Payload{
			sp("spatial.skill.thuy.active.luu_bo.contact"),
			st("effect.skill.thuy.luu_bo_haste"),
		},
	},
	"skill.thuy.active.trieu_quyen": {
		ID: "skill.thuy.active.trieu_quyen", ClassID: thuy, Element: protocolv1.Element_ELEMENT_THUY,
		Kind: KindActive, Unlock: 14, Execution: ExecArea, Targeting: TargetAreaPosition,
		Tags: TagDamaging | TagArea | TagDisplacement, Air: AirAll,
		Speed: combat.TimingCastSpeed, StartupMs: 320, ActiveMs: 120, RecoveryMs: 360,
		Geom:           areaCircle(7000, 2600),
		BaseCooldownMs: 11000, CostMP: 18, Band: combat.BandCleave,
		Payloads: []Payload{
			sp("spatial.skill.thuy.active.trieu_quyen.pull"),
			dmg(1.25),
		},
	},
	"skill.thuy.active.thuy_kinh": {
		ID: "skill.thuy.active.thuy_kinh", ClassID: thuy, Element: protocolv1.Element_ELEMENT_THUY,
		Kind: KindActive, Unlock: 22, Execution: ExecInstant, Targeting: TargetSelf,
		Tags: TagShield | TagDefensive | TagStatusApply, Air: AirAll,
		Speed: combat.TimingCastSpeed, StartupMs: 220, ActiveMs: 80, RecoveryMs: 280,
		Geom:           GeometrySpec{Kind: combat.GeomSelf},
		BaseCooldownMs: 13000, CostMP: 20, Band: combat.BandSingleTarget,
		Payloads: []Payload{
			sh("effect.skill.thuy.thuy_kinh_shield"),
			st("effect.skill.thuy.thuy_kinh_ward"),
		},
	},
	"skill.thuy.active.han_trieu": {
		ID: "skill.thuy.active.han_trieu", ClassID: thuy, Element: protocolv1.Element_ELEMENT_THUY,
		Kind: KindActive, Unlock: 32, Execution: ExecArea, Targeting: TargetDirection,
		Tags: TagDamaging | TagArea | TagStatusApply, Air: AirAll,
		Speed: combat.TimingCastSpeed, StartupMs: 340, ActiveMs: 140, RecoveryMs: 380,
		Geom:           box(combat.GeomDirectionBox, 5500, 1400),
		BaseCooldownMs: 15000, CostMP: 24, Band: combat.BandWide,
		Payloads: []Payload{dmg(1.60), st("effect.skill.thuy.slow_40_3s")},
	},
	"skill.thuy.active.thien_ha": {
		ID: "skill.thuy.active.thien_ha", ClassID: thuy, Element: protocolv1.Element_ELEMENT_THUY,
		Kind: KindActive, Unlock: 45, Execution: ExecArea, Targeting: TargetAreaPosition,
		Tags: TagDamaging | TagArea | TagStatusApply | TagSignature, Air: AirGround,
		Speed: combat.TimingCastSpeed, StartupMs: 500, ActiveMs: 100, RecoveryMs: 460,
		Geom:           areaCircle(7500, 3500),
		BaseCooldownMs: 26000, CostMP: 38, Band: combat.BandWide,
		Payloads: []Payload{dmg(2.80), st("effect.skill.thuy.freeze_2000ms")},
	},

	// ---- HOA — Phù Sư ----
	"skill.hoa.basic.hoa_phu": {
		ID: "skill.hoa.basic.hoa_phu", ClassID: hoa, Element: protocolv1.Element_ELEMENT_HOA,
		Kind: KindBasic, Unlock: 1, Execution: ExecProjectile, Targeting: TargetProjectile,
		Tags: TagBasicAttack | TagDamaging | TagProjectile | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 210, ActiveMs: 50, RecoveryMs: 540,
		Geom:           proj(7500, 11000, 250),
		BaseCooldownMs: 800, Band: combat.BandBasic1,
		BaseCoefficient: 0.95, MaxCooldownMs: 400, CooldownStepUs: 36400,
		BaseProcBP: 800, MaxProcBP: 2500, ProcEffects: []string{"effect.basic.burn_3s"},
		RestoresMP: 2,
	},
	"skill.hoa.basic.viem_dan": {
		ID: "skill.hoa.basic.viem_dan", ClassID: hoa, Element: protocolv1.Element_ELEMENT_HOA,
		Kind: KindBasic, Unlock: 4, Execution: ExecProjectile, Targeting: TargetProjectile,
		Tags: TagBasicAttack | TagDamaging | TagProjectile | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 220, ActiveMs: 50, RecoveryMs: 550,
		Geom:           proj(7800, 11000, 280),
		BaseCooldownMs: 820, Band: combat.BandBasic2,
		BaseCoefficient: 1.02, MaxCooldownMs: 410, CooldownStepUs: 37300,
		BaseProcBP: 700, MaxProcBP: 2300,
		ProcEffects: []string{"effect.basic.burn_3s"},
		ProcSpatial: "spatial.effect.basic.area_splash_50",
	},
	"skill.hoa.basic.hoa_xa": {
		ID: "skill.hoa.basic.hoa_xa", ClassID: hoa, Element: protocolv1.Element_ELEMENT_HOA,
		Kind: KindBasic, Unlock: 18, Execution: ExecProjectile, Targeting: TargetProjectile,
		Tags: TagBasicAttack | TagDamaging | TagProjectile | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 200, ActiveMs: 50, RecoveryMs: 530,
		Geom:           proj(8200, 13000, 220),
		BaseCooldownMs: 780, Band: combat.BandBasic3,
		BaseCoefficient: 1.08, MaxCooldownMs: 390, CooldownStepUs: 35500,
		BaseProcBP: 800, MaxProcBP: 2400,
		ProcEffects: []string{"effect.basic.burn_3s", "effect.basic.resist_shred_hoa_3s"},
	},
	"skill.hoa.basic.lua_tao_quan": {
		ID: "skill.hoa.basic.lua_tao_quan", ClassID: hoa, Element: protocolv1.Element_ELEMENT_HOA,
		Kind: KindBasic, Unlock: 36, Execution: ExecProjectile, Targeting: TargetProjectile,
		Tags: TagBasicAttack | TagDamaging | TagProjectile | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 210, ActiveMs: 50, RecoveryMs: 540,
		Geom:           proj(8000, 12000, 260),
		BaseCooldownMs: 800, Band: combat.BandBasic4,
		BaseCoefficient: 1.22, MaxCooldownMs: 400, CooldownStepUs: 36400,
		BaseProcBP: 800, MaxProcBP: 2500, ProcEffects: []string{"effect.basic.burn_true_3s"},
	},
	"skill.hoa.active.boc_bo": {
		ID: "skill.hoa.active.boc_bo", ClassID: hoa, Element: protocolv1.Element_ELEMENT_HOA,
		Kind: KindActive, Unlock: 8, Execution: ExecDashAttack, Targeting: TargetDirection,
		Tags: TagDamaging | TagMovement | TagStatusApply, Air: AirAll, MoveBehavior: MoveForced,
		Speed: combat.TimingCastSpeed, StartupMs: 120, ActiveMs: 300, RecoveryMs: 240,
		Geom:           line(combat.GeomDashLine, 4200, 300, 1000),
		BaseCooldownMs: 7500, CostMP: 12, Band: combat.BandSingleTarget,
		Payloads: []Payload{dmg(1.15), stFirst("effect.basic.burn_3s")},
		// The ember trail is the presentation of the resolved DASH_LINE:
		// it creates no second zone, reach, hit or status result.
	},
	"skill.hoa.active.lien_bao": {
		ID: "skill.hoa.active.lien_bao", ClassID: hoa, Element: protocolv1.Element_ELEMENT_HOA,
		Kind: KindActive, Unlock: 14, Execution: ExecArea, Targeting: TargetAreaPosition,
		Tags: TagDamaging | TagArea | TagStatusApply, Air: AirAll,
		Speed: combat.TimingCastSpeed, StartupMs: 320, ActiveMs: 120, RecoveryMs: 360,
		Geom:           areaCircle(7000, 2800),
		BaseCooldownMs: 10000, CostMP: 18, Band: combat.BandCleave,
		Payloads: []Payload{dmg(1.45), st("effect.basic.stun_500ms")},
	},
	"skill.hoa.active.hoa_giap": {
		ID: "skill.hoa.active.hoa_giap", ClassID: hoa, Element: protocolv1.Element_ELEMENT_HOA,
		Kind: KindActive, Unlock: 22, Execution: ExecInstant, Targeting: TargetSelf,
		Tags: TagDamaging | TagDefensive | TagStatusApply, Air: AirAll,
		Speed: combat.TimingCastSpeed, StartupMs: 220, ActiveMs: 80, RecoveryMs: 280,
		Geom:           GeometrySpec{Kind: combat.GeomSelf},
		BaseCooldownMs: 14000, CostMP: 22, Band: combat.BandSingleTarget,
		Payloads: []Payload{st("effect.skill.hoa.hoa_giap_aura")},
	},
	"skill.hoa.active.hoa_vuc": {
		ID: "skill.hoa.active.hoa_vuc", ClassID: hoa, Element: protocolv1.Element_ELEMENT_HOA,
		Kind: KindActive, Unlock: 32, Execution: ExecArea, Targeting: TargetAreaPosition,
		Tags: TagDamaging | TagArea | TagStatusApply, Air: AirGround,
		Speed: combat.TimingCastSpeed, StartupMs: 360, ActiveMs: 80, RecoveryMs: 400,
		Geom:           areaCircle(7000, 3000),
		BaseCooldownMs: 16000, CostMP: 26, Band: combat.BandWide,
		Payloads: []Payload{zone("zone.hoa.hoa_vuc")},
	},
	"skill.hoa.active.cuu_hoa_lien": {
		ID: "skill.hoa.active.cuu_hoa_lien", ClassID: hoa, Element: protocolv1.Element_ELEMENT_HOA,
		Kind: KindActive, Unlock: 45, Execution: ExecCast, Targeting: TargetAreaPosition,
		Tags: TagDamaging | TagArea | TagStatusApply | TagSignature, Air: AirAll,
		Speed: combat.TimingCastSpeed, StartupMs: 560, ActiveMs: 120, RecoveryMs: 480,
		Geom:           areaCircle(7500, 3500),
		BaseCooldownMs: 28000, CostMP: 40, Band: combat.BandWide,
		Payloads: []Payload{dmg(3.10), st("effect.basic.burn_3s")},
	},

	// ---- THO — Hộ Pháp ----
	"skill.tho.basic.tran_quyen": {
		ID: "skill.tho.basic.tran_quyen", ClassID: tho, Element: protocolv1.Element_ELEMENT_THO,
		Kind: KindBasic, Unlock: 1, Execution: ExecInstant, Targeting: TargetDirection,
		Tags: TagBasicAttack | TagDamaging | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 240, ActiveMs: 120, RecoveryMs: 590,
		Geom:           box(combat.GeomMeleeBox, 2000, 1000),
		BaseCooldownMs: 950, Band: combat.BandBasic1,
		BaseCoefficient: 1.05, MaxCooldownMs: 500, CooldownStepUs: 40900,
		BaseProcBP: 800, MaxProcBP: 2500, ProcEffects: []string{"effect.basic.weaken_10_3s"},
		RestoresMP: 2,
	},
	"skill.tho.basic.pha_thach_kich": {
		ID: "skill.tho.basic.pha_thach_kich", ClassID: tho, Element: protocolv1.Element_ELEMENT_THO,
		Kind: KindBasic, Unlock: 4, Execution: ExecInstant, Targeting: TargetDirection,
		Tags: TagBasicAttack | TagDamaging | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 250, ActiveMs: 120, RecoveryMs: 610,
		Geom:           box(combat.GeomMeleeBox, 2200, 1000),
		BaseCooldownMs: 980, Band: combat.BandBasic2,
		BaseCoefficient: 1.10, MaxCooldownMs: 500, CooldownStepUs: 43600,
		BaseProcBP: 500, MaxProcBP: 1800, ProcEffects: []string{"effect.basic.stun_400ms"},
	},
	"skill.tho.basic.dia_liet_kich": {
		ID: "skill.tho.basic.dia_liet_kich", ClassID: tho, Element: protocolv1.Element_ELEMENT_THO,
		Kind: KindBasic, Unlock: 18, Execution: ExecInstant, Targeting: TargetDirection,
		Tags: TagBasicAttack | TagDamaging | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 260, ActiveMs: 120, RecoveryMs: 620,
		Geom:           box(combat.GeomDirectionBox, 3000, 1000),
		BaseCooldownMs: 1000, Band: combat.BandBasic3,
		BaseCoefficient: 1.15, MaxCooldownMs: 500, CooldownStepUs: 45500,
		BaseProcBP: 700, MaxProcBP: 2200, ProcEffects: []string{"effect.basic.root_1200ms"},
	},
	"skill.tho.basic.kim_cang_quyen": {
		ID: "skill.tho.basic.kim_cang_quyen", ClassID: tho, Element: protocolv1.Element_ELEMENT_THO,
		Kind: KindBasic, Unlock: 36, Execution: ExecInstant, Targeting: TargetDirection,
		Tags: TagBasicAttack | TagDamaging | TagStatusApply, Air: AirAll,
		Speed: combat.TimingAttackSpeed, StartupMs: 230, ActiveMs: 120, RecoveryMs: 600,
		Geom:           box(combat.GeomMeleeBox, 2300, 1100),
		BaseCooldownMs: 950, Band: combat.BandBasic4,
		BaseCoefficient: 1.28, MaxCooldownMs: 480, CooldownStepUs: 42700,
		BaseProcBP: 600, MaxProcBP: 2000,
		ProcEffects: []string{"effect.basic.stun_500ms", "effect.basic.weaken_12_3s"},
	},
	"skill.tho.active.thach_kich": {
		ID: "skill.tho.active.thach_kich", ClassID: tho, Element: protocolv1.Element_ELEMENT_THO,
		Kind: KindActive, Unlock: 8, Execution: ExecInstant, Targeting: TargetDirection,
		Tags: TagDamaging | TagDisplacement | TagStatusApply, Air: AirAll,
		Speed: combat.TimingCastSpeed, StartupMs: 260, ActiveMs: 120, RecoveryMs: 340,
		Geom:           box(combat.GeomMeleeBox, 2400, 1000),
		BaseCooldownMs: 8000, CostMP: 14, Band: combat.BandSingleTarget,
		Payloads: []Payload{
			dmg(1.30),
			sp("spatial.skill.tho.active.thach_kich.knockback"),
			stFirst("effect.basic.weaken_10_3s"),
		},
	},
	"skill.tho.active.tho_giap": {
		ID: "skill.tho.active.tho_giap", ClassID: tho, Element: protocolv1.Element_ELEMENT_THO,
		Kind: KindActive, Unlock: 14, Execution: ExecInstant, Targeting: TargetAreaSelf,
		Tags: TagShield | TagDefensive, Air: AirAll,
		Speed: combat.TimingCastSpeed, StartupMs: 240, ActiveMs: 80, RecoveryMs: 300,
		Geom:           circle(3000),
		BaseCooldownMs: 12000, CostMP: 18, Band: combat.BandSingleTarget,
		Payloads: []Payload{
			sh("effect.skill.tho.tho_giap_shield"),
			st("effect.skill.tho.tho_giap_ward"),
		},
	},
	"skill.tho.active.dia_chan": {
		ID: "skill.tho.active.dia_chan", ClassID: tho, Element: protocolv1.Element_ELEMENT_THO,
		Kind: KindActive, Unlock: 22, Execution: ExecArea, Targeting: TargetAreaSelf,
		Tags: TagDamaging | TagArea | TagDisplacement | TagStatusApply, Air: AirGround,
		Speed: combat.TimingCastSpeed, StartupMs: 360, ActiveMs: 160, RecoveryMs: 420,
		Geom:           circle(3200),
		BaseCooldownMs: 10000, CostMP: 20, Band: combat.BandCleave,
		Payloads: []Payload{
			dmg(1.35),
			sp("spatial.skill.tho.active.dia_chan.airborne"),
		},
	},
	"skill.tho.active.son_bich": {
		ID: "skill.tho.active.son_bich", ClassID: tho, Element: protocolv1.Element_ELEMENT_THO,
		Kind: KindActive, Unlock: 32, Execution: ExecArea, Targeting: TargetAreaPosition,
		Tags: TagArea | TagDefensive, Air: AirGround,
		Speed: combat.TimingCastSpeed, StartupMs: 380, ActiveMs: 80, RecoveryMs: 400,
		Geom:           barrierGeom(6500, 800, 4000, 5000),
		BaseCooldownMs: 15000, CostMP: 24, Band: combat.BandSingleTarget,
		Payloads: []Payload{{Kind: PayBarrier}},
	},
	"skill.tho.active.thien_son_tran": {
		ID: "skill.tho.active.thien_son_tran", ClassID: tho, Element: protocolv1.Element_ELEMENT_THO,
		Kind: KindActive, Unlock: 45, Execution: ExecCast, Targeting: TargetAreaSelf,
		Tags: TagDamaging | TagArea | TagStatusApply | TagShield | TagDefensive | TagSignature, Air: AirGround,
		Speed: combat.TimingCastSpeed, StartupMs: 600, ActiveMs: 160, RecoveryMs: 540,
		Geom:           circle(3800),
		BaseCooldownMs: 27000, CostMP: 40, Band: combat.BandWide,
		Payloads: []Payload{
			dmg(2.60),
			st("effect.skill.tho.stun_1500ms"),
			sh("effect.skill.tho.thien_son_tran_shield"),
		},
	},
}

// ZoneDef is a compiled Zone Schedules row: lifetime, tick schedule,
// per-tick payload and activation status id ("" = NONE).
type ZoneDef struct {
	ID               string
	LifetimeMs       int64
	FirstTickMs      int64
	IntervalMs       int64
	TickCount        int
	Payloads         []Payload
	ActivationStatus string
}

var zoneTable = map[string]ZoneDef{
	"zone.kim.kiem_tran": {
		ID: "zone.kim.kiem_tran", LifetimeMs: 3000, FirstTickMs: 500, IntervalMs: 500, TickCount: 6,
		Payloads:         []Payload{dmg(0.45), st("effect.skill.kim.kiem_tran_slow")},
		ActivationStatus: "effect.skill.kim.kiem_tran_slow",
	},
	"zone.moc.thanh_dang": {
		ID: "zone.moc.thanh_dang", LifetimeMs: 5000, FirstTickMs: 1000, IntervalMs: 1000, TickCount: 5,
		Payloads:         []Payload{dmg(0.30), st("effect.skill.moc.thanh_dang_slow")},
		ActivationStatus: "effect.skill.moc.thanh_dang_slow",
	},
	"zone.moc.van_doc": {
		ID: "zone.moc.van_doc", LifetimeMs: 4000, FirstTickMs: 1000, IntervalMs: 1000, TickCount: 4,
		Payloads: []Payload{dmg(0.25)},
	},
	"zone.moc.van_moc_hoi_sinh": {
		ID: "zone.moc.van_moc_hoi_sinh", LifetimeMs: 6000, FirstTickMs: 1000, IntervalMs: 1000, TickCount: 6,
		Payloads: []Payload{{Kind: PayHeal, Ratio: 0.04}},
	},
	"zone.hoa.hoa_vuc": {
		ID: "zone.hoa.hoa_vuc", LifetimeMs: 5000, FirstTickMs: 500, IntervalMs: 500, TickCount: 10,
		Payloads:         []Payload{dmg(0.40), st("effect.basic.burn_3s")},
		ActivationStatus: "effect.basic.burn_3s",
	},
}

// Lookup returns the compiled definition for a launch basic/active
// skill id, or false when the id is not one of the 45 runtime skills.
func Lookup(skillID string) (*Def, bool) {
	d, ok := registry[skillID]
	return d, ok
}

// Zone returns the compiled zone schedule row.
func Zone(zoneID string) (ZoneDef, bool) {
	z, ok := zoneTable[zoneID]
	return z, ok
}

// IDs returns the 45 skill ids sorted lexically.
func IDs() []string {
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// List returns all compiled defs sorted by id.
func List() []*Def {
	ids := IDs()
	out := make([]*Def, 0, len(ids))
	for _, id := range ids {
		out = append(out, registry[id])
	}
	return out
}

// validate panics at init if the compiled tables are inconsistent
// (join failures are compile-time content defects).
func init() {
	for id, d := range registry {
		if err := validateDef(d); err != nil {
			panic(fmt.Sprintf("skills: %s: %v", id, err))
		}
	}
}
