// Package world is the shard director of world_rules.md and sharding.md:
// one normal-world map exposes up to CHANNELS_PER_MAP (30) channel
// instances, each backed by one sim/runtime partition. The package owns
// channel membership, placement orders, pending placement, NPC-service
// consults, transfers, respawn and checkpoint routing; it never touches
// SQL (durable/world owns writes) and never mutates world state on behalf
// of the edge (ADR-0083 consult/admit/apply separation).
package world

import (
	"context"
	"encoding/binary"
	"errors"
	"time"

	"thinhthan/internal/core/id"
	"thinhthan/internal/sim/spatial/geometry"
)

// Channel capacity and lifecycle constants (world_rules.md, sharding.md).
const (
	ChannelsPerMap   = 30 // channel indices run 1..30
	SoftCap          = 18 // player-initiated admission cap per channel
	HardCap          = 22 // forced-placement cap per channel
	PendingRetryMS   = 5000
	IdleStop         = 600 * time.Second
	RespawnDelayTick = 60 // RESPAWN_DELAY 3s at 20 Hz
	InvulnTicks      = 60 // 3.0s invulnerability after respawn
	NPCSessionTicks  = 600 // gameplay-capable session expiry (30s at 20 Hz)
	NPCRangeMM       = 2500
	TransferBudgetWorld    = 30 * time.Second
	TransferBudgetInstance = 120 * time.Second
)

// Wire message ids emitted on the world-control surface (messages.md).
const (
	MsgS2CPlacementPending    uint32 = 15
	MsgS2CTransferPrepare     uint32 = 105
	MsgS2CChannelSwitchResult uint32 = 110
	MsgS2CInteractResult      uint32 = 116
	MsgS2CActionRejected      uint32 = 204
	MsgS2CRespawn             uint32 = 207
)

// sessionTargetBit marks an Outbound.To as a character-session key rather
// than a partition entity id; the edge resolves it via its session
// registry. World-control traffic (15/105/110/116/204/207) always targets
// the character's session — it must reach pending, dead or mid-transfer
// players that have no entity.
const sessionTargetBit = 1 << 63

// SessionTarget converts a character id to its session-scoped Outbound.To.
func SessionTarget(characterID id.UUID) uint64 {
	return sessionTargetBit | binary.LittleEndian.Uint64(characterID[:8])
}

// MapKind is the world_map type field (FIELD or TOWN for normal world).
type MapKind string

const (
	MapField MapKind = "FIELD"
	MapTown  MapKind = "TOWN"
)

// CheckpointDef binds a checkpoint id to its safe map and respawn anchor.
type CheckpointDef struct {
	CheckpointID string
	MapID        string
	AnchorID     string
}

// PortalDef is one directed portal edge (portal family fields).
type PortalDef struct {
	PortalID    string
	SourceMap   string
	DestMap     string
	DestSpawn   string // anchor id on the destination map
	AnchorID    string // portal anchor id on the source map
	RequireFlag string // content flag id, "" when none
	RequireLevel int32 // 0 = no level gate
}

// NpcDef is one NPC placed on a map; Services is its allowed-service set.
type NpcDef struct {
	NpcID    string
	MapID    string
	AnchorID string
	Services []string
}

// MapRecord is the compiled world/space record of one normal-world map.
type MapRecord struct {
	MapID            string
	Kind             MapKind
	Region           string // region key for Addressables region.<zone>
	BoundsMaxX       int64
	BoundsMaxY       int64
	LayoutProfile    string
	RequiredTopology string
	Anchors          []geometry.Anchor
	EntrySpawn       string // world_map entry_spawn anchor id
	RecLevelLo       int32
	RecLevelHi       int32
	Checkpoints      []CheckpointDef
	Portals          []PortalDef
	Npcs             []NpcDef
}

// Anchor returns the named anchor's position.
func (m MapRecord) Anchor(anchorID string) (geometry.Anchor, bool) {
	for _, a := range m.Anchors {
		if a.ID == anchorID {
			return a, true
		}
	}
	return geometry.Anchor{}, false
}

// MapCatalog is the read-side content surface the world runtime needs.
type MapCatalog interface {
	Lookup(mapID string) (MapRecord, bool)
	Maps() []MapRecord
	Checkpoint(checkpointID string) (CheckpointDef, bool)
	Portal(portalID string) (PortalDef, bool)
}

// ConsequenceRow is one durable world-consequence row converted for the
// sim loader (durable isolation: the sim never sees SQL).
type ConsequenceRow struct {
	RelicID   string
	Active    bool
	ExpiresAt time.Time
}

// Loader is the durable-read port injected at composition: partition-state
// loads (world_consequence_relics) and character checkpoint reads.
type Loader interface {
	LoadConsequences(ctx context.Context, mapID string, channelID uint64) ([]ConsequenceRow, error)
	LoadCheckpoint(ctx context.Context, characterID id.UUID) (checkpointID, mapID, anchorID string, err error)
}

var (
	// ErrCapacityFull is a resolved placement finding every candidate
	// channel at capacity (MAP_CAPACITY_FULL wire verdict).
	ErrCapacityFull = errors.New("world: all channels at capacity")
	// ErrNoMap reports a placement/consult naming an unknown map id.
	ErrNoMap = errors.New("world: unknown map")
	// ErrNoCharacter reports an unknown character on a member lookup.
	ErrNoCharacter = errors.New("world: unknown character")
	// ErrMailboxFull rejects a mailbox entry on overflow.
	ErrMailboxFull = errors.New("world: mailbox full")
	// ErrNoCheckpoint reports a checkpoint id absent from the catalog.
	ErrNoCheckpoint = errors.New("world: unknown checkpoint")
)

// FamilySimCheckpoint is the ProducerCheckpoint family string — the
// server-driven checkpoint write path (save_rules.md two-path rule).
const FamilySimCheckpoint = "sim.checkpoint"
