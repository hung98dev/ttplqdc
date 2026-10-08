// Package world owns the durable side of the normal-world placement /
// interaction boundary (IMP-018): characters.checkpoint_id / map_id
// writes and world_consequence_relics reads for the partition loader.
//
// The client-boundary writes ride the ADR-0083 route — edge/world
// submits JournalClientCommand records of the placement.* and
// interaction.npc_service families after the admission consult; the
// executors here commit the write and produce the recorded client_result
// (116 S2C_INTERACT_RESULT / 110 S2C_CHANNEL_SWITCH_RESULT) the edge
// then delivers. Server-driven checkpoint/map writes (transfer
// completion, respawn placement, recovery) arrive as CHECKPOINT records
// of family sim.checkpoint emitted by sim/world via QueueCommand /
// EMIT_DURABLE_COMMANDS; the emit adapter builds the record with
// CheckpointRecord and the executor commits it.
//
// durable/ never imports sim/: LoadConsequences returns plain rows;
// sim/world adapts them into runtime.PartitionState.
package world
