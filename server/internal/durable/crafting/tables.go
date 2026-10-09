package crafting

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
)

// Canonical crafting tables (crafting.md / crafting_catalog.md). All
// values are spec constants, never computed or sampled.

// tierRow is one equipment tier's crafting bases (crafting_catalog.md
// § Equipment Recipes): regional material id and per-weight-unit
// material/common bases, plus the recipe minimum level.
type tierRow struct {
	MaterialID string
	Material   int64
	Common     int64
	MinLevel   int32
}

// tierTable maps `t1..t6` to their authored bases.
var tierTable = map[string]tierRow{
	"t1": {MaterialID: "item.material.lang_da.manh_dong", Material: 3, Common: 100, MinLevel: 1},
	"t2": {MaterialID: "item.material.u_minh.vo_cay", Material: 4, Common: 250, MinLevel: 11},
	"t3": {MaterialID: "item.material.ben_nuoc.da_song", Material: 5, Common: 600, MinLevel: 21},
	"t4": {MaterialID: "item.material.deo_may.da_voi", Material: 6, Common: 1200, MinLevel: 31},
	"t5": {MaterialID: "item.material.thanh_co.gach_co", Material: 8, Common: 2200, MinLevel: 41},
	"t6": {MaterialID: "item.material.nui_thieng.da_suong", Material: 10, Common: 3500, MinLevel: 51},
}

// slotWeights maps an equipment slot to its authored input multiplier
// (sum 47 over all slots).
var slotWeights = map[string]int64{
	"weapon": 5, "head": 3, "body": 5, "hands": 3, "legs": 4,
	"feet": 3, "necklace": 3, "ring": 2, "costume": 4, "talisman": 3,
	"jade": 3, "seal": 4, "relic": 3, "charm": 2,
}

// baseRateBP is the base success rate in bp indexed by current level L
// (crafting.md § Base Success Rates: L -> L+1).
var baseRateBP = [16]int64{
	10000, 10000, 8500, 7000, 5500, 4500, 3500, 2500,
	2000, 1500, 1000, 800, 600, 400, 300, 200,
}

// materialMultiplier and commonMultiplier are the explicit attempt-cost
// multipliers by current level (crafting.md § Costs).
var materialMultiplier = [16]int64{1, 1, 2, 2, 3, 3, 4, 5, 6, 8, 10, 12, 15, 18, 22, 28}
var commonMultiplier = [16]int64{1, 2, 3, 4, 6, 8, 12, 16, 22, 30, 40, 55, 75, 100, 135, 180}

const (
	maxEnhanceLevel = 16
	rateClampBP     = 9500
	blessingBonusBP = 300
	pityStartTarget = 13
	pityMaxFails    = 9
	pityRampAfter   = 5 // pity kicks in after 5 consecutive fails
	pityPerFailBP   = 100
	pityCapBP       = 500
	maxBatch        = 99
	charmKindLucky  = "item.consumable.bua_may"
	charmKindInsure = "item.consumable.bua_giu_bac"
	luckyGradeLow   = "so_cap"
	luckyGradeMid   = "trung_cap"
	luckyGradeHigh  = "cao_cap"
	luckyGradeSuper = "sieu_cap"
)

// floorOf returns the milestone floor for a current level.
func floorOf(level int64) int64 {
	switch {
	case level < 4:
		return 0
	case level < 8:
		return 4
	case level < 12:
		return 8
	case level < 16:
		return 12
	default:
		return 16
	}
}

// luckyBonusBP maps a lucky charm grade to its bp bonus and the
// current-level eligibility bound (crafting.md § Lucky Charm).
func luckyBonusBP(grade string) (bp int64, maxCurrent int64, ok bool) {
	switch grade {
	case luckyGradeLow:
		return 500, 8, true
	case luckyGradeMid:
		return 300, 12, true
	case luckyGradeHigh:
		return 100, 16, true
	case luckyGradeSuper:
		return 300, 16, true
	}
	return 0, 0, false
}

// insuranceEligibility mirrors the lucky table's current-level bounds
// (cao_cap is the top insurance grade — crafting.md § Insurance).
func insuranceEligibility(grade string) (maxCurrent int64, ok bool) {
	switch grade {
	case luckyGradeLow:
		return 8, true
	case luckyGradeMid:
		return 12, true
	case luckyGradeHigh:
		return 16, true
	}
	return 0, false
}

// gradeOf parses `item.consumable.<kind>.<grade>`; kind must match.
func charmGrade(itemID, kind string) (string, bool) {
	prefix := kind + "."
	if len(itemID) > len(prefix) && itemID[:len(prefix)] == prefix {
		return itemID[len(prefix):], true
	}
	return "", false
}

// streamFor derives the deterministic PCG-64 stream for one roll key —
// inputs are authority-frozen so retries replay the same roll
// (concurrency.md § RNG: math/rand/v2 PCG-64 keyed streams).
func streamFor(key string) *rand.Rand {
	h := sha256.Sum256([]byte(key))
	return rand.New(rand.NewPCG(
		binary.LittleEndian.Uint64(h[0:8]),
		binary.LittleEndian.Uint64(h[8:16]),
	))
}

// craftRollKey is the earned roll identity for one created equipment
// instance's persistent stat roll.
func craftRollKey(instanceID string) string {
	return fmt.Sprintf("craft.%s", instanceID)
}

// enhanceRollKey binds one frozen enhance attempt to its durable op —
// retry replays, never rerolls.
func enhanceRollKey(itemInstanceID string, targetLevel int64, opID string) string {
	return fmt.Sprintf("enhance.%s.%d.%s", itemInstanceID, targetLevel, opID)
}
