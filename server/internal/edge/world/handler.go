package world

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
	durableworld "thinhthan/internal/durable/world"
	"thinhthan/internal/edge/listener"
	"thinhthan/internal/edge/router"
	simworld "thinhthan/internal/sim/world"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// ADR-0083 route per durable id: consult (read-only mailbox answer) →
// build + Submit the JournalClientCommand → AwaitClientOutcome → deliver
// the recorded result → post the world command preserving operation_id
// for ids with a partition-side effect → Ack. Consult transport failure
// or a false verdict rejects admission with the op's typed result
// carrying the consult's code — never a fabricated verdict.

func (s *Service) interact(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SInteract)
	if !ok || len(req.GetOperationId()) != 16 {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	kind := req.GetInteractKind()
	emit116 := func(res *protocolv1.OperationResult) error {
		return s.send(v.Conn, msgIDInteractR, &protocolv1.S2CInteractResult{
			Result:       res,
			InteractKind: kind,
			TargetId:     req.GetTargetId(),
		})
	}
	fail := func(code protocolv1.ErrorCode) error {
		return emit116(opResult(req.GetOperationId(), code))
	}

	var rep simworld.ConsultReply
	var err error
	switch kind {
	case protocolv1.InteractKind_INTERACT_KIND_TALK:
		rep, err = s.consults.TalkAdmission(ctx, *v.CharacterID, req.GetTargetId())
	case protocolv1.InteractKind_INTERACT_KIND_NPC_SERVICE:
		rep, err = s.consults.NpcServiceValid(ctx, *v.CharacterID, req.GetTargetId(), req.GetServiceId())
	default:
		return fail(protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	if err != nil {
		return fail(consultError(err))
	}
	if !rep.OK {
		return fail(rep.Code)
	}

	if kind == protocolv1.InteractKind_INTERACT_KIND_TALK {
		// TALK owns no durable write — the post path opens the volatile
		// NPC session; the 116 is emitted directly.
		if err := s.postWorld(v, &simworld.Command{
			Kind:        simworld.CmdInteract,
			CharacterID: *v.CharacterID,
			OperationID: mustOpID(req.GetOperationId()),
			InteractKind: uint32(kind),
			TargetID:     mustEntityID(req.GetTargetId()),
		}); err != nil {
			return fail(protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE)
		}
		return emit116(opOK(req.GetOperationId()))
	}

	// NPC_SERVICE → interaction.npc_service client command; the resolved
	// checkpoint fields ride optional field 12 for set_checkpoint.
	var checkpoint *journalv1.JournalCheckpoint
	if req.GetServiceId() == durableworld.SetCheckpointService {
		checkpoint = &journalv1.JournalCheckpoint{
			CharacterId:     v.CharacterID[:],
			OwnershipEpoch:  v.OwnershipEpoch,
			CheckpointId:    rep.CheckpointID,
			SafeMapId:       rep.MapID,
			EntrySpawnId:    rep.AnchorID,
			MembershipState: "world",
			RecordedAtMs:    s.now().UnixMilli(),
		}
	}
	rec, err := durableworld.InteractRecord(v.AccountID, *v.CharacterID,
		v.SessionEpoch, v.OwnershipEpoch, req, checkpoint, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	if err := s.q.Submit(ctx, rec); err != nil {
		return fail(submitCode(err))
	}
	outcome, err := s.q.AwaitClientOutcome(ctx, durableworld.FamilyNpcService,
		idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: *v.CharacterID},
		mustOpID(req.GetOperationId()))
	if err != nil {
		return fail(awaitCode(err))
	}
	if result := outcome.GetS2CInteractResult(); result != nil {
		if err := s.send(v.Conn, msgIDInteractR, result); err != nil {
			return err
		}
	} else {
		return &router.RejectError{Code: outcome.GetErrorCode()}
	}
	// Post-commit partition effect: NPC_SERVICE touches the session's
	// expiry inside the owning partition.
	if err := s.postWorld(v, &simworld.Command{
		Kind:         simworld.CmdInteract,
		CharacterID:  *v.CharacterID,
		OperationID:  mustOpID(req.GetOperationId()),
		InteractKind: uint32(kind),
		TargetID:     mustEntityID(req.GetTargetId()),
		ServiceID:    req.GetServiceId(),
	}); err != nil {
		return fail(protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE)
	}
	return s.ack(ctx, durableworld.FamilyNpcService, *v.CharacterID, req.GetOperationId())
}

func (s *Service) portal(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SPortalUse)
	if !ok || len(req.GetOperationId()) != 16 || req.GetPortalId() == "" {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	emit116 := func(code protocolv1.ErrorCode) error {
		return s.send(v.Conn, msgIDInteractR, &protocolv1.S2CInteractResult{
			Result:       opResult(req.GetOperationId(), code),
			InteractKind: protocolv1.InteractKind_INTERACT_KIND_PORTAL,
		})
	}
	rep, err := s.consults.PortalAdmission(ctx, *v.CharacterID, req.GetPortalId())
	if err != nil {
		return emit116(consultError(err))
	}
	if !rep.OK {
		return emit116(rep.Code)
	}
	rec, err := durableworld.PortalRecord(v.AccountID, *v.CharacterID,
		v.SessionEpoch, v.OwnershipEpoch, req, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	if err := s.q.Submit(ctx, rec); err != nil {
		return emit116(submitCode(err))
	}
	opID := mustOpID(req.GetOperationId())
	outcome, err := s.q.AwaitClientOutcome(ctx, durableworld.FamilyPlacementPortal,
		idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: *v.CharacterID}, opID)
	if err != nil {
		return emit116(awaitCode(err))
	}
	if result := outcome.GetS2CInteractResult(); result != nil {
		if err := s.send(v.Conn, msgIDInteractR, result); err != nil {
			return err
		}
	} else {
		return &router.RejectError{Code: outcome.GetErrorCode()}
	}
	if outcome.GetStatus() == protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		if err := s.postWorld(v, &simworld.Command{
			Kind:        simworld.CmdPortal,
			CharacterID: *v.CharacterID,
			OperationID: opID,
			PortalID:    req.GetPortalId(),
		}); err != nil {
			return emit116(protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE)
		}
	}
	return s.ack(ctx, durableworld.FamilyPlacementPortal, *v.CharacterID, req.GetOperationId())
}

func (s *Service) channel(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SChannelSwitch)
	if !ok || len(req.GetOperationId()) != 16 {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	emit110 := func(code protocolv1.ErrorCode, retryAfterMs uint32) error {
		return s.send(v.Conn, msgIDChannelR, &protocolv1.S2CChannelSwitchResult{
			Result:             opResult(req.GetOperationId(), code),
			TargetChannelIndex: req.GetTargetChannelIndex(),
			RetryAfterMs:       retryAfterMs,
		})
	}
	retryFor := func(code protocolv1.ErrorCode) uint32 {
		if code == protocolv1.ErrorCode_ERROR_CODE_MAP_CAPACITY_FULL {
			return 5000 // errors.md: capacity rejections carry retry_after
		}
		return 0
	}
	rep, err := s.consults.ChannelAdmission(ctx, *v.CharacterID, req.GetTargetChannelIndex())
	if err != nil {
		code := consultError(err)
		return emit110(code, retryFor(code))
	}
	if !rep.OK {
		return emit110(rep.Code, retryFor(rep.Code))
	}
	rec, err := durableworld.ChannelRecord(v.AccountID, *v.CharacterID,
		v.SessionEpoch, v.OwnershipEpoch, req, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	if err := s.q.Submit(ctx, rec); err != nil {
		code := submitCode(err)
		return emit110(code, retryFor(code))
	}
	opID := mustOpID(req.GetOperationId())
	outcome, err := s.q.AwaitClientOutcome(ctx, durableworld.FamilyPlacementChannel,
		idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: *v.CharacterID}, opID)
	if err != nil {
		code := awaitCode(err)
		return emit110(code, retryFor(code))
	}
	if result := outcome.GetS2CChannelSwitchResult(); result != nil {
		if err := s.send(v.Conn, msgIDChannelR, result); err != nil {
			return err
		}
	} else {
		return &router.RejectError{Code: outcome.GetErrorCode()}
	}
	if outcome.GetStatus() == protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		if err := s.postWorld(v, &simworld.Command{
			Kind:          simworld.CmdChannelSwitch,
			CharacterID:   *v.CharacterID,
			OperationID:   opID,
			TargetChannel: req.GetTargetChannelIndex(),
		}); err != nil {
			return emit110(protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE, 0)
		}
	}
	return s.ack(ctx, durableworld.FamilyPlacementChannel, *v.CharacterID, req.GetOperationId())
}

// respawn is the ADR-0082 non-durable drain: the validated request posts
// a Respawn world command carrying the original operation_id; the owning
// partition drains it (dead-only gate, delay, checkpoint placement) and
// settles via QueueCommand → committed 207 / rejected 204.
func (s *Service) respawn(ctx context.Context, v router.View, r router.Route) error {
	_ = ctx
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SRespawnRequest)
	if !ok || len(req.GetOperationId()) != 16 {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	if err := s.postWorld(v, &simworld.Command{
		Kind:        simworld.CmdRespawn,
		CharacterID: *v.CharacterID,
		OperationID: mustOpID(req.GetOperationId()),
	}); err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_SERVER_OVERLOADED}
	}
	return nil
}

// postWorld routes the post-commit command to the character's owning
// partition; a missing partition fails the admission (fail-closed).
func (s *Service) postWorld(v router.View, cmd *simworld.Command) error {
	_ = v
	if s.w == nil {
		return ErrConsultUnbound
	}
	return s.w.PostCommand(*v.CharacterID, cmd)
}

func (s *Service) ack(ctx context.Context, family string, charID id.UUID, opBytes []byte) error {
	err := s.q.Ack(ctx, family,
		idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: charID},
		mustOpID(opBytes))
	if err != nil && !strings.Contains(err.Error(), "no ackable") {
		return err
	}
	return nil
}

// send marshals msg and enqueues it on the bound conn.
func (s *Service) send(c *listener.Conn, msgID uint32, msg proto.Message) error {
	if c == nil {
		return nil
	}
	payload, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	return c.Send(&protocolv1.Envelope{MessageId: msgID, Payload: payload},
		listener.DeliveryControl)
}

// opResult builds the OperationResult every typed result wraps.
func opResult(opBytes []byte, code protocolv1.ErrorCode) *protocolv1.OperationResult {
	status := protocolv1.ResultStatus_RESULT_STATUS_SUCCESS
	if code != protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		status = protocolv1.ResultStatus_RESULT_STATUS_ERROR
	}
	return &protocolv1.OperationResult{
		OperationId: opBytes,
		Status:      status,
		ErrorCode:   code,
	}
}

func opOK(opBytes []byte) *protocolv1.OperationResult {
	return opResult(opBytes, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED)
}

func mustOpID(b []byte) id.UUID {
	var op id.UUID
	copy(op[:], b)
	return op
}

func mustEntityID(s string) uint64 {
	n, _ := strconv.ParseUint(s, 10, 64)
	return n
}

// consultError maps transport failures to the op's admission code —
// session/state-class per ADR-0083, never a fabricated verdict.
func consultError(err error) protocolv1.ErrorCode {
	switch {
	case errors.Is(err, ErrConsultTimeout), errors.Is(err, ErrConsultUnbound),
		errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE
	case errors.Is(err, simworld.ErrMailboxFull):
		return protocolv1.ErrorCode_ERROR_CODE_SERVER_OVERLOADED
	default:
		return protocolv1.ErrorCode_ERROR_CODE_TEMPORARY_DEPENDENCY_FAILURE
	}
}

// submitCode maps admission Submit failures to wire codes.
func submitCode(err error) protocolv1.ErrorCode {
	switch {
	case errors.Is(err, queue.ErrQueueFull), errors.Is(err, queue.ErrBackpressure):
		return protocolv1.ErrorCode_ERROR_CODE_SERVER_OVERLOADED
	case errors.Is(err, queue.ErrShutdown):
		return protocolv1.ErrorCode_ERROR_CODE_TEMPORARY_DEPENDENCY_FAILURE
	default:
		return protocolv1.ErrorCode_ERROR_CODE_TEMPORARY_DEPENDENCY_FAILURE
	}
}

// awaitCode maps a non-committed terminal receipt to a wire code.
func awaitCode(err error) protocolv1.ErrorCode {
	var te *idempotency.TerminalError
	if errors.As(err, &te) && te.State == idempotency.ReceiptExpiredUncommitted {
		return protocolv1.ErrorCode_ERROR_CODE_OPERATION_EXPIRED
	}
	return protocolv1.ErrorCode_ERROR_CODE_TEMPORARY_DEPENDENCY_FAILURE
}
