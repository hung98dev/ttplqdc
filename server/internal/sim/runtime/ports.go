package runtime

import (
	"context"

	"thinhthan/internal/core/id"
	"thinhthan/internal/sim/replication"

	"google.golang.org/protobuf/proto"
)

// Ports is the typed boundary the partition exposes to Edge and Durable
// (IMP-069 wires the implementations; sim never imports them).
type Ports struct {
	// Durable receives emitted durable commands in tick order.
	Durable EmitPort
	// Results drains committed durable results back into the partition.
	Results ResultInPort
	// Loader gates partition start on the world_consequence rows for
	// this (map, channel).
	Loader StartLoaderPort
	// Outbound receives replication messages tagged with their delivery
	// class and supersede key for the edge queue.
	Outbound OutboundPort
}

// DurableOwnerKind mirrors the owner kinds of
// docs/06_data/save_rules.md's durable queue identity.
type DurableOwnerKind int

const (
	OwnerCharacter DurableOwnerKind = iota
	OwnerGuild
	OwnerAccount
	OwnerWorldID
)

// DurableKind enumerates the registry rows the sim is allowed to produce
// (docs/06_data/save_rules.md § Closed Producer Registry). Movement,
// combat, targeting and baseline-ack traffic is runtime-only and is never
// queued.
type DurableKind int

const (
	CmdWorldConsequence DurableKind = iota
	CmdReward
	CmdCheckpoint
	CmdBossEligibility
	CmdBossChest
	CmdActivity
	CmdChatLog
)

// DurableCommand is one typed, idempotent write the partition asks Durable
// to apply. OperationID plus Fingerprint give the durable side its
// at-least-once dedup identity; SourceEvent is the canonical sim source
// name for the producing event.
type DurableCommand struct {
	Kind            DurableKind
	Family          string
	OwnerKind       DurableOwnerKind
	Owner           id.UUID
	OperationID     id.UUID
	Fingerprint     [32]byte
	SourceEvent     string
	ContentRevision string
	Tick            uint64
}

// EmitPort consumes durable commands in emission order. An error means the
// command was not accepted; the partition keeps it pending and retries.
type EmitPort interface {
	Emit(ctx context.Context, cmd DurableCommand) error
}

// ResultKind tags which durable result a committed row carries.
type ResultKind int

const (
	ResultApplied ResultKind = iota
	ResultRejected
)

// Result is one committed durable result flowing back into the partition.
type Result struct {
	Kind        ResultKind
	OperationID id.UUID
	ErrorCode   uint32
}

// ResultInPort drains pending durable results. DrainInto writes up to
// len(buf) results into buf and returns how many it wrote; results beyond
// the partition's per-tick budget stay pending for the next tick.
type ResultInPort interface {
	DrainInto(buf []Result) int
}

// WorldConsequence is one loaded world_consequence row relevant to this
// (map, channel) partition.
type WorldConsequence struct {
	RelicID       string
	Active        bool
	ExpiresAtTick uint64
}

// PartitionState is the durable state the partition must load before
// admitting players (realtime_loop.md § Restart).
type PartitionState struct {
	Consequences []WorldConsequence
}

// StartLoaderPort loads the durable partition state. A nil or erroring
// loader keeps the partition unstarted.
type StartLoaderPort interface {
	LoadPartitionState(ctx context.Context, mapID string, channelID uint64) (PartitionState, error)
}

// Outbound is one replication message plus the queueing metadata the edge
// outbound queue needs: target player entity, wire message id, delivery
// class and the replaceable-state supersede key when HasKey is set.
type Outbound struct {
	To           uint64
	MessageID    uint32
	Class        replication.DeliveryClass
	SupersedeKey uint64
	HasKey       bool
	Msg          proto.Message
}

// OutboundPort enqueues replication traffic for the edge. The sim emits
// synchronously inside the BUILD phase; message storage is borrowed from
// the producing Builder and is only valid until that builder's next build.
type OutboundPort interface {
	Enqueue(o Outbound) error
}

// IntentInPort is the typed enqueue surface edge calls into the partition
// mailbox; SubmitIntent satisfies it.
type IntentInPort interface {
	Enqueue(i Intent) error
}
