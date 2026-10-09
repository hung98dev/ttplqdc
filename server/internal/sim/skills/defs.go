package skills

import (
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/combat"
)

// Kind is the runtime catalog kind (passives have no execution geometry
// and are not runtime actions — they are excluded from the registry).
type Kind uint8

const (
	KindBasic Kind = iota
	KindActive
)

// ExecutionType is skills.md § Execution Types (closed set).
type ExecutionType uint8

const (
	ExecInstant ExecutionType = iota
	ExecCast
	ExecChannel
	ExecProjectile
	ExecArea
	ExecDashAttack
	ExecMovement
	ExecSummon
)

// TargetingMode is skills.md § Targeting Modes (closed set).
type TargetingMode uint8

const (
	TargetSelf TargetingMode = iota
	TargetDirection
	TargetSingle
	TargetAreaPosition
	TargetAreaSelf
	TargetProjectile
)

// TagSet is the canonical skill-tag bitmask of skills.md § Canonical
// Skill Tags (12 selectors).
type TagSet uint16

const (
	TagBasicAttack TagSet = 1 << iota
	TagDamaging
	TagArea
	TagProjectile
	TagMovement
	TagHeal
	TagShield
	TagStatusApply
	TagDisplacement
	TagDefensive
	TagSignature
	tagSentinel
)

// Has reports whether the tag is set.
func (t TagSet) Has(tag TagSet) bool { return t&tag != 0 }

// AirProfile is the canonical runtime matrix air availability: ALL
// expands to grounded+jumping+falling, GROUND to grounded only.
type AirProfile uint8

const (
	AirAll AirProfile = iota
	AirGround
)

// Usable reports whether the skill may be accepted in the given
// movement situation.
func (a AirProfile) Usable(grounded, jumping, falling bool) bool {
	switch a {
	case AirAll:
		return grounded || jumping || falling
	default:
		return grounded
	}
}

// MovementBehavior is skills.md § Movement and Air Use.
type MovementBehavior uint8

const (
	MoveAllow MovementBehavior = iota
	MoveLock
	MoveReduced
	MoveForced
)

// --- Geometry variants (skills.md § Geometry Contract; mm + ms) ---

// BoxGeom is MELEE_BOX / DIRECTION_BOX: forward reach and vertical
// half-height around SKILL_ORIGIN_Y.
type BoxGeom struct {
	ReachMM      int64
	HalfHeightMM int64
}

// ProjGeom is PROJECTILE: max range (center travel), speed, hit radius.
type ProjGeom struct {
	RangeMM     int64
	SpeedMMPerS int64
	RadiusMM    int64
}

// CircleGeom is AREA_SELF.
type CircleGeom struct {
	RadiusMM int64
}

// AreaGeom is AREA_POSITION: authored cast range + circle radius.
type AreaGeom struct {
	CastMM   int64
	RadiusMM int64
}

// RangeGeom is SINGLE_TARGET_RANGE.
type RangeGeom struct {
	RangeMM int64
}

// LineGeom is DASH_LINE / MOVE_CONTACT_LINE: authored distance, motion
// duration, and vertical half-height of the swept contact box.
type LineGeom struct {
	DistanceMM      int64
	DurationMs      int64
	HitHalfHeightMM int64
}

// MoveGeom is MOVE_LINE: pure authored reposition.
type MoveGeom struct {
	DistanceMM int64
	DurationMs int64
}

// BarrierGeom is BARRIER_POSITION (CAT-002): cast range, footprint
// thickness, height and authored lifetime.
type BarrierGeom struct {
	CastMM      int64
	ThicknessMM int64
	HeightMM    int64
	DurationMs  int64
}

// GeometrySpec is the typed authoritative geometry union. Exactly the
// field matching Kind is non-nil (SELF carries none).
type GeometrySpec struct {
	Kind combat.GeometryKind

	Box        *BoxGeom     // GeomMeleeBox, GeomDirectionBox
	Projectile *ProjGeom    // GeomProjectile
	Circle     *CircleGeom  // GeomAreaSelf
	Area       *AreaGeom    // GeomAreaPosition
	Range      *RangeGeom   // GeomSingleTargetRange
	Line       *LineGeom    // GeomDashLine, GeomMoveContact
	Move       *MoveGeom    // GeomMoveLine
	Barrier    *BarrierGeom // GeomBarrier
}

// PayloadKind is the closed Active Payloads dispatch of
// class_skill_catalog.md § Compiler Source Schema.
type PayloadKind uint8

const (
	PayDamage PayloadKind = iota
	PayHeal
	PayStatus
	PayShield
	PaySpatial
	PayZone
	PayExecute
	PayBarrier
)

// Payload is one ordered payload cell.
type Payload struct {
	Kind PayloadKind

	// Coefficient is DAMAGE(r) r or HEAL(h,a) a.
	Coefficient float64
	// Ratio is HEAL(h,a) h (target MAX_HP ratio) or EXECUTE(h,m) h
	// (target pre-hit HP threshold).
	Ratio float64
	// Bonus is EXECUTE(h,m) m (source additive multiplier).
	Bonus float64
	// RefID is the effect_id / spatial_effect_id / zone_id for
	// STATUS / SHIELD / SPATIAL / ZONE.
	RefID string
	// FirstHitOnly applies the status to the first connected target
	// only (dash-follow statuses and moc_bo's guaranteed poison).
	FirstHitOnly bool
}

// Def is one compiled basic/active runtime row: the join of the
// Canonical Runtime Matrix, Action Specifications, Active Payloads and
// (for basics) the Cooldown & Status Proc Matrix.
type Def struct {
	ID      string
	ClassID string // e.g. "class.kim"
	Element protocolv1.Element
	Kind    Kind
	Unlock  int32

	Execution      ExecutionType
	Targeting      TargetingMode
	Tags           TagSet
	Air            AirProfile
	MoveBehavior   MovementBehavior
	Speed          combat.TimingSpeed
	StartupMs      int64
	ActiveMs       int64
	RecoveryMs     int64
	Geom           GeometrySpec
	AirGeom        *GeometrySpec // basic-only override; nil = use Geom
	BaseCooldownMs int64
	CostMP         int64
	Band           combat.TargetBand
	Payloads       []Payload

	// Basic-only authored rows (zero for actives).
	BaseCoefficient float64
	MaxCooldownMs   int64 // Lv12 cooldown floor
	CooldownStepUs  int64 // cd_step in microseconds
	BaseProcBP      int64 // proc chance at Lv1, basis points
	MaxProcBP       int64 // proc chance at Lv12, basis points
	ProcEffects     []string
	ProcSpatial     string // spatial id rolled alongside proc (am_luu)
	GuaranteedEvery int    // connected-hit cadence for a guaranteed stack (bang_phien = 3)
	RestoresMP      int64  // basic_1 +2 MP per connected primary hit
	PenetrationBP   int64  // PENETRATE defense_penetration_ratio in bp (1500 = 0.15)
}

// IsBasic reports the dedicated-basic taxonomy.
func (d *Def) IsBasic() bool { return d.Kind == KindBasic }

// Hostile reports whether the action commits damaging/hostile results
// (combat pipeline participation), not presentation.
func (d *Def) Hostile() bool {
	for _, p := range d.Payloads {
		if p.Kind == PayDamage || p.Kind == PayExecute {
			return true
		}
	}
	return false
}
