package progression

// RespecKind mirrors the wire RespecKind enum (content.proto); defined
// locally so sim never reaches into generated types.
type RespecKind uint8

const (
	RespecKindUnspecified RespecKind = 0
	RespecKindSkill       RespecKind = 1
	RespecKindPotential   RespecKind = 2
)

// RespecFreeLevel: respec is free at or below level 20.
const RespecFreeLevel int32 = 20

// RespecPrice is the currency.common cost: 0 at ≤ Lv20, 25·level² above
// (Lv21 = 11_025, Lv40 = 40_000, Lv60 = 90_000).
func RespecPrice(level int32) int64 {
	if level <= RespecFreeLevel {
		return 0
	}
	l := int64(level)
	return 25 * l * l
}
