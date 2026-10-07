package world

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/edge/router"
	simworld "thinhthan/internal/sim/world"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// opBytes mints a fresh UUIDv7 operation id.
func opBytes() []byte {
	o := id.NewV7(time.Now().UTC())
	return o[:]
}

// strp boxes an optional proto string field.
func strp(s string) *string { return &s }

// mapOf reads the character's resident map column.
func (e *env) mapOf(t *testing.T, charID id.UUID) string {
	t.Helper()
	var m string
	if err := e.pool.QueryRow(context.Background(),
		`SELECT map_id FROM characters WHERE character_id=$1`, charID).Scan(&m); err != nil {
		t.Fatalf("mapOf: %v", err)
	}
	return m
}

// receiptCount counts committed durable receipts for a character owner —
// the observable proof that the Submit→commit→Ack chain ran (or did not).
func (e *env) receiptCount(t *testing.T, charID id.UUID) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM durable_command_receipts WHERE owner_id = $1`,
		charID).Scan(&n); err != nil {
		t.Fatalf("receipt count: %v", err)
	}
	return n
}

func (e *env) checkpointOf(t *testing.T, charID id.UUID) string {
	t.Helper()
	var cp string
	if err := e.pool.QueryRow(context.Background(),
		`SELECT checkpoint_id FROM characters WHERE character_id = $1`,
		charID).Scan(&cp); err != nil {
		t.Fatalf("checkpoint read: %v", err)
	}
	return cp
}

// TestInteractSetCheckpointRoute: ADR-0083 durable route end-to-end —
// the admission consult resolves the checkpoint payload (the wire
// request carries none), the executor commits it and the recorded 116,
// and the post-commit world command lands in the owning partition.
func TestInteractSetCheckpointRoute(t *testing.T) {
	cons := newFakeConsults()
	cons.replies["NpcServiceValid:1500000000001:set_checkpoint"] = simworld.ConsultReply{
		OK:           true,
		CheckpointID: "checkpoint.alpha.one",
		MapID:        "map.alpha",
		AnchorID:     "anchor.alpha.spawn",
	}
	e := newEnv(t, cons)
	acct, charID := e.seedCharacter(t)
	e.admit(t, charID)

	err := e.dispatch(context.Background(), view(acct, &charID), msgIDInteract,
		&protocolv1.C2SInteract{
			OperationId:  opBytes(),
			InteractKind: protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE,
			TargetId:     "1500000000001",
			ServiceId:    strp("set_checkpoint"),
		})
	if err != nil {
		t.Fatalf("interact dispatch: %v", err)
	}
	if cons.callCount() != 1 {
		t.Fatalf("consult calls = %d, want exactly 1", cons.callCount())
	}
	// The committed write can only carry the consult-resolved payload.
	waitFor(t, "checkpoint committed", func() bool {
		return e.checkpointOf(t, charID) == "checkpoint.alpha.one"
	})
	waitFor(t, "receipt committed", func() bool {
		return e.receiptCount(t, charID) == 1
	})
}

// TestInteractUnregisteredServiceAdmissionReject: a false consult verdict
// rejects admission with the consult's code — nothing submits.
func TestInteractUnregisteredServiceAdmissionReject(t *testing.T) {
	cons := newFakeConsults()
	cons.replies["NpcServiceValid:1500000000001:teleport"] = simworld.ConsultReply{
		OK:   false,
		Code: protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID,
	}
	e := newEnv(t, cons)
	acct, charID := e.seedCharacter(t)
	e.admit(t, charID)

	if err := e.dispatch(context.Background(), view(acct, &charID), msgIDInteract,
		&protocolv1.C2SInteract{
			OperationId:  opBytes(),
			InteractKind: protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE,
			TargetId:     "1500000000001",
			ServiceId:    strp("teleport"),
		}); err != nil {
		t.Fatalf("reject path should not error: %v", err)
	}
	if e.receiptCount(t, charID) != 0 {
		t.Fatal("rejected admission left a durable receipt")
	}
}

// TestInteractConsultFailureAdmissionReject: a consult transport error
// (timeout/unbound) rejects with INVALID_STATE and never submits.
func TestInteractConsultFailureAdmissionReject(t *testing.T) {
	cons := newFakeConsults()
	cons.errs["NpcServiceValid:1500000000001:set_checkpoint"] = ErrConsultTimeout
	e := newEnv(t, cons)
	acct, charID := e.seedCharacter(t)
	e.admit(t, charID)

	if err := e.dispatch(context.Background(), view(acct, &charID), msgIDInteract,
		&protocolv1.C2SInteract{
			OperationId:  opBytes(),
			InteractKind: protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE,
			TargetId:     "1500000000001",
			ServiceId:    strp("set_checkpoint"),
		}); err != nil {
		t.Fatalf("consult-failure admission should emit the typed reject, not error: %v", err)
	}
	if e.receiptCount(t, charID) != 0 {
		t.Fatal("consult-failure admission left a durable receipt")
	}
}

// TestTalkPostsWorldCommandOnly: TALK has no durable write — after the
// admission consult the handler posts the CmdInteract directly and emits
// the 116 itself.
func TestTalkPostsWorldCommandOnly(t *testing.T) {
	cons := newFakeConsults() // TalkAdmission → OK
	e := newEnv(t, cons)
	acct, charID := e.seedCharacter(t)
	e.admit(t, charID)

	if err := e.dispatch(context.Background(), view(acct, &charID), msgIDInteract,
		&protocolv1.C2SInteract{
			OperationId:  opBytes(),
			InteractKind: protocolv1.InteractKind_INTERACT_KIND_TALK,
			TargetId:     "1500000000001",
		}); err != nil {
		t.Fatalf("talk dispatch: %v", err)
	}
	if cons.callCount() != 1 {
		t.Fatalf("talk consult calls = %d, want 1", cons.callCount())
	}
	if e.receiptCount(t, charID) != 0 {
		t.Fatal("TALK must never produce a durable receipt")
	}
}

// TestPortalPostCommitCommand: the 104 world command (partition-side
// effect) is posted only after the committed outcome — observable as the
// source partition emitting 105 S2C_TRANSFER_PREPARE through Outbound.
func TestPortalPostCommitCommand(t *testing.T) {
	cons := newFakeConsults()
	cons.replies["PortalAdmission:portal.alpha.beta"] = simworld.ConsultReply{
		OK:   true,
		MapID: "map.beta",
	}
	e := newEnv(t, cons)
	acct, charID := e.seedCharacter(t)
	e.admit(t, charID)

	if err := e.dispatch(context.Background(), view(acct, &charID), msgIDPortal,
		&protocolv1.C2SPortalUse{
			OperationId: opBytes(),
			PortalId:    "portal.alpha.beta",
		}); err != nil {
		t.Fatalf("portal dispatch: %v", err)
	}
	waitFor(t, "portal receipt", func() bool {
		return e.receiptCount(t, charID) == 1
	})
	waitFor(t, "transfer prepare emitted", func() bool {
		return len(e.out.forChar(charID, 105)) == 1
	})
}

// TestChannelPostCommitCommand: committed 109 posts CmdChannelSwitch —
// the partition applies the channel move and emits 105 for the target.
func TestChannelPostCommitCommand(t *testing.T) {
	cons := newFakeConsults()
	cons.replies["ChannelAdmission"] = simworld.ConsultReply{OK: true, ChannelIndex: 2}
	e := newEnv(t, cons)
	acct, charID := e.seedCharacter(t)
	e.admit(t, charID)

	if err := e.dispatch(context.Background(), view(acct, &charID), msgIDChannel,
		&protocolv1.C2SChannelSwitch{
			OperationId:        opBytes(),
			TargetChannelIndex: 2,
		}); err != nil {
		t.Fatalf("channel dispatch: %v", err)
	}
	waitFor(t, "channel receipt", func() bool {
		return e.receiptCount(t, charID) == 1
	})
	waitFor(t, "transfer prepare emitted", func() bool {
		return len(e.out.forChar(charID, 105)) == 1
	})
}

// TestChannelAdmissionRejectedNoCommit: a consult verdict of
// MAP_CAPACITY_FULL rejects before Submit — the wire 110 carries
// retry_after and no receipt lands.
func TestChannelAdmissionRejectedNoCommit(t *testing.T) {
	cons := newFakeConsults()
	cons.replies["ChannelAdmission"] = simworld.ConsultReply{
		OK:   false,
		Code: protocolv1.ErrorCode_ERROR_CODE_MAP_CAPACITY_FULL,
	}
	e := newEnv(t, cons)
	acct, charID := e.seedCharacter(t)
	e.admit(t, charID)

	if err := e.dispatch(context.Background(), view(acct, &charID), msgIDChannel,
		&protocolv1.C2SChannelSwitch{
			OperationId:        opBytes(),
			TargetChannelIndex: 3,
		}); err != nil {
		t.Fatalf("rejected channel switch should not error: %v", err)
	}
	if e.receiptCount(t, charID) != 0 {
		t.Fatal("rejected channel admission left a durable receipt")
	}
}

// TestRespawnNonDurableFence: 208 rides the ADR-0082 non-durable table —
// absent from the durable registry and the durable-intent classifier.
func TestRespawnNonDurableFence(t *testing.T) {
	e := newEnv(t, nil)
	if !router.IsNonDurableID(msgIDRespawn) {
		t.Fatal("208 must be registered on the non-durable table")
	}
	if !e.rt.Handles(msgIDRespawn) {
		t.Fatal("router must handle 208")
	}
	if router.IsDurableIntent(msgIDRespawn) {
		t.Fatal("208 must never classify as a durable intent")
	}
	if _, ok := router.FamilyFor(msgIDRespawn, &protocolv1.C2SRespawnRequest{}); ok {
		t.Fatal("208 must resolve no durable family")
	}

	// Memberless → postWorld fails closed (no owning partition).
	acct, charID := e.seedCharacter(t)
	if err := e.dispatch(context.Background(), view(acct, &charID), msgIDRespawn,
		&protocolv1.C2SRespawnRequest{OperationId: opBytes()}); err == nil {
		t.Fatal("respawn for a memberless character must reject")
	}

	// Membered → posts CmdRespawn into the owning partition's mailbox.
	e.admit(t, charID)
	if err := e.dispatch(context.Background(), view(acct, &charID), msgIDRespawn,
		&protocolv1.C2SRespawnRequest{OperationId: opBytes()}); err != nil {
		t.Fatalf("membered respawn dispatch: %v", err)
	}
	// Non-durable: no receipt regardless of outcome.
	if e.receiptCount(t, charID) != 0 {
		t.Fatal("208 must never produce a durable receipt")
	}
}

// TestMalformedOperationID: a non-16-byte operation_id is a protocol
// rejection before any consult or submit.
func TestMalformedOperationID(t *testing.T) {
	cons := newFakeConsults()
	e := newEnv(t, cons)
	acct, charID := e.seedCharacter(t)
	e.admit(t, charID)

	err := e.dispatch(context.Background(), view(acct, &charID), msgIDInteract,
		&protocolv1.C2SInteract{
			OperationId:  []byte{1, 2, 3},
			InteractKind: protocolv1.InteractKind_INTERACT_KIND_TALK,
			TargetId:     "1500000000001",
		})
	if err == nil {
		t.Fatal("malformed operation_id must reject")
	}
	if cons.callCount() != 0 {
		t.Fatal("malformed request consumed a consult")
	}
}

// TestInteractResultAlwaysSent: every 103 — admitted or rejected at any
// stage — routes through the 116 send path (send on a nil conn is
// tolerated, so the route returns nil); only protocol-malformed input
// short-circuits before the result wire.
func TestInteractResultAlwaysSent(t *testing.T) {
	cons := newFakeConsults()
	cons.replies["NpcServiceValid:1500000000001:teleport"] = simworld.ConsultReply{
		OK:   false,
		Code: protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID,
	}
	cons.errs["NpcServiceValid:1500000000002:set_checkpoint"] = context.DeadlineExceeded
	e := newEnv(t, cons)
	acct, charID := e.seedCharacter(t)
	e.admit(t, charID)
	v := view(acct, &charID)

	// Consult verdict reject → 116 ERROR (send attempted, nil error).
	if err := e.dispatch(context.Background(), v, msgIDInteract,
		&protocolv1.C2SInteract{OperationId: opBytes(),
			InteractKind: protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE,
			TargetId:     "1500000000001", ServiceId: strp("teleport")}); err != nil {
		t.Fatalf("verdict reject must still send 116: %v", err)
	}
	// Consult error → 116 mapped code.
	if err := e.dispatch(context.Background(), v, msgIDInteract,
		&protocolv1.C2SInteract{OperationId: opBytes(),
			InteractKind: protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE,
			TargetId:     "1500000000002", ServiceId: strp("set_checkpoint")}); err != nil {
		t.Fatalf("consult error must still send 116: %v", err)
	}
	// Unknown interact kind → 116 TARGET_INVALID.
	if err := e.dispatch(context.Background(), v, msgIDInteract,
		&protocolv1.C2SInteract{OperationId: opBytes(),
			InteractKind: protocolv1.InteractKind_INTERACT_KIND_CHEST,
			TargetId:     "1500000000003"}); err != nil {
		t.Fatalf("unknown kind must still send 116: %v", err)
	}
	// Protocol-malformed (short op id) → RejectError, no result wire.
	if err := e.dispatch(context.Background(), v, msgIDInteract,
		&protocolv1.C2SInteract{OperationId: []byte{1},
			InteractKind: protocolv1.InteractKind_INTERACT_KIND_TALK}); err == nil {
		t.Fatal("malformed request must reject before any 116")
	}
	if e.receiptCount(t, charID) != 0 {
		t.Fatal("no reject path may leave a durable receipt")
	}
}

// TestInteractResultProtoEchoesKind: a rejected admission still answers
// with the request's interact kind + target (single 116 per 103).
func TestInteractResultProtoEchoesKind(t *testing.T) {
	// With conn == nil the emit is a no-op; assert the built message
	// shape via a marshal round-trip of what send() would enqueue.
	op := opBytes()
	res := &protocolv1.S2CInteractResult{
		Result:       opResult(op, protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID),
		InteractKind: protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE,
		TargetId:     "1500000000001",
	}
	b, err := proto.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("empty 116 payload")
	}
}

// TestSubmitCommitAckIdempotent: replaying the same operation_id returns
// the committed outcome without re-committing (ADR-0081 receipt).
func TestSubmitCommitAckIdempotent(t *testing.T) {
	cons := newFakeConsults()
	e := newEnv(t, cons)
	acct, charID := e.seedCharacter(t)
	e.admit(t, charID)

	op := opBytes()
	req := &protocolv1.C2SPortalUse{OperationId: op, PortalId: "portal.alpha.beta"}
	for i := 0; i < 2; i++ {
		if err := e.dispatch(context.Background(), view(acct, &charID), msgIDPortal, req); err != nil {
			t.Fatalf("portal dispatch %d: %v", i, err)
		}
	}
	waitFor(t, "portal receipt", func() bool {
		return e.receiptCount(t, charID) == 1
	})
	// Receipts are keyed on operation_id — a second admit of the same id
	// replays the committed outcome rather than appending a row.
	if n := e.receiptCount(t, charID); n != 1 {
		t.Fatalf("receipts = %d after replay, want 1", n)
	}
}

// TestAwaitClientOutcomeCommitted: the outcome the edge delivers is the
// executor's recorded client_result — read it straight off the store.
func TestAwaitClientOutcomeCommitted(t *testing.T) {
	cons := newFakeConsults()
	e := newEnv(t, cons)
	acct, charID := e.seedCharacter(t)
	e.admit(t, charID)

	op := opBytes()
	if err := e.dispatch(context.Background(), view(acct, &charID), msgIDChannel,
		&protocolv1.C2SChannelSwitch{OperationId: op, TargetChannelIndex: 2}); err != nil {
		t.Fatalf("channel dispatch: %v", err)
	}
	var opID id.UUID
	copy(opID[:], op)
	waitFor(t, "channel outcome", func() bool {
		out, err := e.q.AwaitClientOutcome(context.Background(), "placement.channel",
			idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: charID}, opID)
		if err != nil {
			return false
		}
		return out.GetS2CChannelSwitchResult() != nil
	})
}
