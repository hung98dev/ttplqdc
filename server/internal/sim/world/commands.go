package world

import "thinhthan/internal/core/id"

// CommandKind is the closed set of mailbox commands (ADR-0083 effect
// posts and internal world commands).
type CommandKind int

const (
	// CmdInteract is the post-commit world effect of a durable 103 (for
	// IMP-018: TALK opens the NPC session; NPC_SERVICE touches it).
	CmdInteract CommandKind = iota
	// CmdPortal is the post-commit effect of a durable 104: freeze and
	// begin the transfer.
	CmdPortal
	// CmdChannelSwitch is the post-commit effect of a durable 109.
	CmdChannelSwitch
	// CmdRespawn is the non-durable 208 drain (ADR-0082).
	CmdRespawn
	// CmdCheckpointWrite is a server-driven checkpoint write request
	// (QueueCommand on the partition's durable path).
	CmdCheckpointWrite
	// CmdTransferStart admits a player into this partition at a spawn:
	// used for first attach, transfer completion and forced placement.
	CmdTransferStart
	// CmdPresentationReady is the client's 106 readiness signal.
	CmdPresentationReady
	// CmdPlayerLeave removes the character from this channel.
	CmdPlayerLeave
)

// Command is one typed mailbox command.
type Command struct {
	Kind         CommandKind
	CharacterID  id.UUID
	OperationID  [16]byte // durable op id to preserve through effects
	EnqueuedTick uint64

	// Interact (CmdInteract)
	InteractKind uint32
	TargetID     uint64 // npc entity id
	ServiceID    string
	ServiceParam string

	// Portal / channel switch
	PortalID      string
	TargetChannel uint32

	// TransferStart / PresentationReady
	TransferID     [16]byte
	MapID          string
	SpawnAnchorID  string
	SpawnX, SpawnY int32
	Reason         uint8 // PendingReason context for admit bookkeeping

	// Respawn is set on respawn-driven admissions: the dest host emits
	// the committed 207 and queues the checkpoint durable write.
	Respawn *RespawnOutcome

	// Vitals carries the persisted hp/mp on attach-driven admissions
	// (zero value leaves the entity's vitals untouched); the durable
	// row owns them.
	Vitals Vitals
}

// Vitals is one admitted entity's persisted hp/mp pair (death_respawn.md;
// respawn uses RespawnOutcome instead — vitals already carry the 40%
// restore).
type Vitals struct {
	MaxHP, HP, MaxMP, MP int64
}

// RespawnOutcome carries the fields the 207 and the sim.checkpoint
// durable write need once the destination admits the player.
type RespawnOutcome struct {
	CheckpointID string
	HPAfter      int64
	MPAfter      int64
}
