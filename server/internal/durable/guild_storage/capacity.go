package guild_storage

// Section names ride the item_locations.slot prefix ("COMMON.N" /
// "RESERVE.N") and the guild_storage_audit.section CHECK.
const (
	SectionCommon  = "COMMON"
	SectionReserve = "RESERVE"
)

// Unlock levels (guild_storage.md § Ownership / Sections).

// Capacity bands across guild levels 1-4/5-9/10-14/15-19/20-24/25-29/30.
var (
	commonCapacityBands  = []int{60, 80, 100, 120, 140, 160, 180}
	reserveCapacityBands = []int{0, 0, 40, 50, 60, 70, 80}
)

func capacityBand(level int) int {
	switch {
	case level >= 30:
		return 6
	case level >= 25:
		return 5
	case level >= 20:
		return 4
	case level >= 15:
		return 3
	case level >= 10:
		return 2
	case level >= 5:
		return 1
	default:
		return 0
	}
}

// CommonCapacity is the COMMON section slot cap at a guild level.
func CommonCapacity(level int) int { return commonCapacityBands[capacityBand(level)] }

// ReserveCapacity is the RESERVE section slot cap (0 below Lv10).
func ReserveCapacity(level int) int { return reserveCapacityBands[capacityBand(level)] }

// SectionCapacity resolves one section's cap.
func SectionCapacity(section string, level int) int {
	if section == SectionReserve {
		return ReserveCapacity(level)
	}
	return CommonCapacity(level)
}
