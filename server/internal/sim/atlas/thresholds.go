package atlas

import (
	"strings"

	durableatlas "thinhthan/internal/durable/atlas"
)

// PHAT_HIEN peak sources (atlas.md § First Seen): the first qualifying
// event on a page surfaces exactly one peak. Fish pages share the
// fish settlement's FISH_RARE peak and hidden-chest pages share the
// chest settlement's CHEST_HIDDEN peak, so no extra ATLAS_SEEN is
// emitted for those — every other page peaks ATLAS_SEEN.
const (
	PeakAtlasSeen   = "ATLAS_SEEN"
	PeakFishRare    = "FISH_RARE"
	PeakChestHidden = "CHEST_HIDDEN"
)

// PeakSource returns the PHAT_HIEN source kind a page's first Seen
// rides. The triggering settlement already emits FISH_RARE /
// CHEST_HIDDEN for those source events; the page itself emits
// ATLAS_SEEN only when no shared peak applies.
func PeakSource(p durableatlas.Page) string {
	switch p.Kind {
	case EventFishCaught:
		return PeakFishRare
	case EventChestOpened:
		return PeakChestHidden
	default:
		return PeakAtlasSeen
	}
}

// IsSettlementPeak reports whether the page's first-Seen marker rides a
// settlement-emitted peak rather than its own ATLAS_SEEN.
func IsSettlementPeak(p durableatlas.Page) bool {
	return PeakSource(p) != PeakAtlasSeen
}

// SettlementSource matches a PHAT_HIEN emission (FISH_RARE /
// CHEST_HIDDEN) back to the page family whose Seen shares it — the
// shared-emitter dedupe: one peak per settlement observation.
func SettlementPeakForSource(sourceID string) string {
	switch {
	case strings.HasPrefix(sourceID, "item.material."):
		return PeakFishRare
	case strings.HasPrefix(sourceID, "chest.hidden."):
		return PeakChestHidden
	default:
		return PeakAtlasSeen
	}
}
