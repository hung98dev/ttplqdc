package world

import (
	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// ConsultKind is the ADR-0083 initial consult set: read-only questions a
// durable admission asks the owning partition or the director.
type ConsultKind int

const (
	// ConsultPartitionTick returns the owning partition's current tick.
	ConsultPartitionTick ConsultKind = iota
	// ConsultNpcServiceValid validates one 103 NPC_SERVICE admission:
	// live session, range, not-in-combat, registered service; the reply
	// carries the resolved checkpoint fields for set_checkpoint.
	ConsultNpcServiceValid
	// ConsultPlacementPortal validates one 104 admission: portal exists,
	// requirement level, range, not-in-combat, destination capacity.
	ConsultPlacementPortal
	// ConsultPlacementChannel validates one 109 admission: channel index
	// bounds, cooldown, destination capacity.
	ConsultPlacementChannel
)

// Consult is one read-only mailbox question; Reply receives exactly one
// ConsultReply (single-shot). Bounded await: the edge drops the consult
// if the mailbox is full or the reply does not arrive inside its await.
type Consult struct {
	Kind        ConsultKind
	CharacterID id.UUID
	Reply       chan<- ConsultReply

	// NpcServiceValid args
	InteractKind uint32
	TargetID     uint64
	ServiceID    string

	// PlacementPortal / PlacementChannel args
	PortalID      string
	TargetChannel uint32
}

// ConsultReply carries the read answer. Code is the wire ErrorCode the
// edge should reject with when OK=false; the extra fields carry resolved
// payload data (checkpoint, tick, resolved destination).
type ConsultReply struct {
	OK   bool
	Code protocolv1.ErrorCode

	Tick         uint64 // ConsultPartitionTick
	MapID        string // resolved destination map (portal)
	ChannelIndex uint32 // preview channel (placement)
	CheckpointID string // resolved checkpoint (set_checkpoint)
	AnchorID     string // checkpoint respawn anchor
}
