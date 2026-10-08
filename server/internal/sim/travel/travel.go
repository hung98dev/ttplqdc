package travel

import (
	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
	"thinhthan/internal/sim/world"
)

// Handler carries travel's admission surface. The consult path resolves
// the generic gates (NPC services allow-set, range, state) through the
// world's answerNpcService; this handler supplies the service-specific
// pieces the durable commit and the post-commit effect consume.
type Handler struct {
	// Anchors resolves a travel-eligible safe-anchor map to its entry
	// spawn anchor id (sim/discovery's catalog.SafeAnchor at
	// composition; FIELD and non-catalog maps are not eligible).
	Anchors func(destMapID string) (spawnAnchorID string, ok bool)
	// Discovered reports the character's committed discovery of
	// destMapID (durable/discovery's store answer at composition).
	Discovered func(characterID id.UUID, destMapID string) bool
}

// Destination validates one travel admission and resolves the
// destination placement for the post-commit transfer start
// (messages.md §103: service_param = destination safe-anchor map_id;
// success → one 116 then 105 TRAVEL). Error codes:
// TARGET_INVALID (not a safe anchor), NOT_DISCOVERED (world_route_catalog
// § travel never creates discovery), INVALID_STATE for an empty
// destination. Story-access and in_combat gates are enforced by the
// caller surfaces (consult chain / durable commit).
func (h Handler) Destination(characterID id.UUID, destMapID string) (world.PlacementRequest, protocolv1.ErrorCode) {
	if destMapID == "" {
		return world.PlacementRequest{}, protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE
	}
	spawn, ok := h.Anchors(destMapID)
	if !ok {
		return world.PlacementRequest{}, protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID
	}
	if h.Discovered != nil && !h.Discovered(characterID, destMapID) {
		return world.PlacementRequest{}, protocolv1.ErrorCode_ERROR_CODE_NOT_DISCOVERED
	}
	// Player-initiated destination resolution — auto order, spawn pinned
	// to the anchor's entry spawn (messages.md §103 service_param).
	return world.PlacementRequest{
		CharacterID:   characterID,
		MapID:         destMapID,
		Kind:          world.PlaceAuto,
		SpawnAnchorID: spawn,
	}, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED
}
