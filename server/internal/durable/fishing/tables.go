package fishing

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/rand/v2"
	"strings"

	"thinhthan/internal/core/id"
)

// CatchRow is one authored catch-table entry (item_catalog.md §
// fishing.catch.default + Seasonal catch tables — weights in basis
// points summing 10000).
type CatchRow struct {
	ItemID   string
	Set      string // COMMON_CATCH / RARE_CATCH / SEASONAL_CATCH
	WeightBP int
}

// Item / table identities (world_rules.md § Folk Fishing,
// item_catalog.md § fishing).
const (
	RodItemID  = "item.tool.can_cau_tre"
	BaitItemID = "item.consumable.moi_cau"
	RareItemID = "item.material.ca_chep_hoa_rong"

	setCommon   = "COMMON_CATCH"
	setRare     = "RARE_CATCH"
	setSeasonal = "SEASONAL_CATCH"
	weightSum   = 10000
)

// defaultTable is `fishing.catch.default` verbatim — closed to
// COMMON_CATCH ∪ RARE_CATCH.
var defaultTable = []CatchRow{
	{ItemID: "item.material.ca_bong", Set: setCommon, WeightBP: 4000},
	{ItemID: "item.material.ca_ro_dong", Set: setCommon, WeightBP: 2500},
	{ItemID: "item.material.tom_song", Set: setCommon, WeightBP: 2500},
	{ItemID: "item.material.ca_chep", Set: setCommon, WeightBP: 900},
	{ItemID: RareItemID, Set: setRare, WeightBP: 100},
}

// seasonalExtras: region spot prefix -> (season index, extra row).
// Each seasonal table clones the default, reduces ca_bong to 3900 and
// appends one SEASONAL_CATCH row at 100 bp (item_catalog.md § Seasonal
// catch tables). Seasons 3/4 have no table.
var seasonalExtras = []struct {
	season   int
	prefix   string // fishing_spot.map.<region>.
	extraID  string
	caBongBP int
}{
	{season: 0, prefix: "fishing_spot.map.lang_da.", extraID: "item.material.ca_linh_giang", caBongBP: 3900},
	{season: 1, prefix: "fishing_spot.map.rung_u_minh.", extraID: "item.material.ca_sam_u_minh", caBongBP: 3900},
	{season: 2, prefix: "fishing_spot.map.ben_nuoc_den.", extraID: "item.material.ca_bong_den", caBongBP: 3900},
	{season: 5, prefix: "fishing_spot.map.nui_thieng.", extraID: "item.material.ca_suong_ho", caBongBP: 3900},
}

// seasonalTable builds the clone of the default table with the
// authored ca_bong reduction and the appended seasonal row.
func seasonalTable(extraID string, caBongBP int) []CatchRow {
	rows := make([]CatchRow, 0, len(defaultTable)+1)
	for _, r := range defaultTable {
		row := r
		if row.ItemID == "item.material.ca_bong" {
			row.WeightBP = caBongBP
		}
		rows = append(rows, row)
	}
	rows = append(rows, CatchRow{ItemID: extraID, Set: setSeasonal, WeightBP: 100})
	return rows
}

// TableForSpot resolves the active catch table for one spot: the
// seasonal table while the featured season index matches the spot's
// region (at most one active), otherwise the closed default
// (world_rules.md § Folk Fishing). diTich applies the Di Tích
// rare-fish buff: RARE 100->110 bp and the largest COMMON weight
// reduced by 10 so the table still sums to 10000 (bosses.md buff).
// ok=false when the spot id is not a legal fishing_spot.* identity.
func TableForSpot(spotID string, seasonIndex int, diTich bool) ([]CatchRow, bool) {
	if !strings.HasPrefix(spotID, "fishing_spot.") {
		return nil, false
	}
	rows := defaultTable
	for _, s := range seasonalExtras {
		if seasonIndex == s.season && strings.HasPrefix(spotID, s.prefix) {
			rows = seasonalTable(s.extraID, s.caBongBP)
			break
		}
	}
	out := make([]CatchRow, len(rows))
	copy(out, rows)
	if diTich {
		largest := -1
		for i, r := range out {
			if r.Set == setRare {
				out[i].WeightBP = 110
			}
			if r.Set == setCommon &&
				(largest < 0 || out[i].WeightBP > out[largest].WeightBP) {
				largest = i
			}
		}
		if largest >= 0 {
			out[largest].WeightBP -= 10
		}
	}
	sum := 0
	for _, r := range out {
		if r.WeightBP <= 0 {
			return nil, false
		}
		sum += r.WeightBP
	}
	if sum != weightSum {
		return nil, false
	}
	return out, true
}

// rollKey is the earned roll identity `fishing.<character_id>.
// <utc_date>.<cast_sequence>` (world_rules.md / reward_claims.md).
func rollKey(charID id.UUID, utcDate string, seq uint64) string {
	return fmt.Sprintf("fishing.%s.%s.%d", charID.String(), utcDate, seq)
}

// streamFor derives the deterministic PCG-64 stream for one roll key —
// the client cannot supply the roll (concurrency.md § RNG: math/rand/v2
// PCG-64 streams derived from authority inputs).
func streamFor(key string) *rand.Rand {
	h := sha256.Sum256([]byte(key))
	return rand.New(rand.NewPCG(
		binary.LittleEndian.Uint64(h[0:8]),
		binary.LittleEndian.Uint64(h[8:16]),
	))
}

// Roll rolls the catch table once on the keyed stream and returns the
// winning row's item id.
func Roll(table []CatchRow, key string) string {
	r := streamFor(key)
	n := int(r.Uint64() % uint64(weightSum))
	acc := 0
	for _, row := range table {
		acc += row.WeightBP
		if n < acc {
			return row.ItemID
		}
	}
	return table[len(table)-1].ItemID
}

// expForAct maps the authored per-act LIFE_SKILL award for one
// FISH_CAUGHT (world_rules.md § Folk Fishing / progression_route.md:
// I 6417, II 8283, III 6332, IV 7047, V 9448, VI 10494).
func expForAct(act int) uint64 {
	switch act {
	case 1:
		return 6417
	case 2:
		return 8283
	case 3:
		return 6332
	case 4:
		return 7047
	case 5:
		return 9448
	case 6:
		return 10494
	}
	return 0
}
