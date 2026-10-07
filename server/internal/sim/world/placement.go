package world

import "thinhthan/internal/core/id"

// PlacementKind selects the admission order (sharding.md § Placement).
type PlacementKind int

const (
	// PlaceAuto is a player-initiated entry (attach, portal, channel
	// switch): most-populated running channel below SoftCap, else the
	// lowest-index stopped channel, else ErrCapacityFull.
	PlaceAuto PlacementKind = iota
	// PlaceForced is a server-driven entry (respawn, recovery, instance
	// return): preferred channel below HardCap, then least-populated
	// running below SoftCap, then lowest stopped, then running below
	// HardCap, else the request goes pending — it never fails
	// CAPACITY_FULL.
	PlaceForced
)

// ForcedReason records why a placement is forced (receipt/fence text).
type ForcedReason string

const (
	ForcedRespawn        ForcedReason = "respawn"
	ForcedRecovery       ForcedReason = "recovery"
	ForcedInstanceReturn ForcedReason = "instance_return"
	ForcedReattach       ForcedReason = "reattach"
)

// PlacementRequest is one placement evaluation.
type PlacementRequest struct {
	CharacterID   id.UUID
	MapID         string
	Kind          PlacementKind
	Preferred     uint32 // forced only; 0 = none
	ForcedReason  ForcedReason
	SpawnAnchorID string // destination anchor override ("" = entry spawn)
}

// PlacementResult is a committed channel selection.
type PlacementResult struct {
	MapID        string
	ChannelIndex uint32
	// Start is true when the chosen channel must be started before the
	// player is admitted (state was stopped).
	Start bool
}
