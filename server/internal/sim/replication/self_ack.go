package replication

import (
	protocolv1 "thinhthan/internal/protocol/v1"
)

// WriteSelfAck fills out with the ADR-0069 self_ack payload every delta
// carries: the last client seq the partition processed for this client and
// the complete current MovementCheckpoint (all 13 fields, effective
// parameters included), so the client can reconcile prediction every delta
// regardless of which entity fields changed.
func (b *Builder) WriteSelfAck(lastProcessedClientSeq uint64, cp *CheckpointSnapshot, out *protocolv1.SelfAck) {
	out.LastProcessedClientSeq = lastProcessedClientSeq
	b.fillCheckpoint(&b.cp, cp)
	out.Checkpoint = &b.cp
}
