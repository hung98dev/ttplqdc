package progression

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	progressiond "thinhthan/internal/durable/progression"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/edge/listener"
	"thinhthan/internal/edge/router"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// upgrade is the router.Handler for C2S_SKILL_UPGRADE on the ADR-0081
// seam: the injected session view supplies every JournalClientCommand
// identity field, the committed 514 client_result is delivered on the
// bound conn, a successful mutation is followed by the 515 snapshot, and
// the receipt is acknowledged only after delivery.
func (s *Service) upgrade(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SSkillUpgrade)
	if !ok {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	rec, err := progressiond.SkillUpgradeRecord(v.AccountID, *v.CharacterID,
		v.SessionEpoch, v.OwnershipEpoch, req, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	return s.run(ctx, v, rec)
}

// allocate is the router.Handler for C2S_POTENTIAL_ALLOCATE.
func (s *Service) allocate(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SPotentialAllocate)
	if !ok {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	rec, err := progressiond.AllocateRecord(v.AccountID, *v.CharacterID,
		v.SessionEpoch, v.OwnershipEpoch, req, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	return s.run(ctx, v, rec)
}

// respec is the router.Handler for C2S_RESPEC: the NPC-service consult
// admits first (fail-closed when unbound — ADR-0083), then the
// admission-frozen spatial evidence lands on the record and the durable
// executor charges and refunds in one transaction.
func (s *Service) respec(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SRespec)
	if !ok {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	if err := s.consultRespec(*v.CharacterID, req.GetNpcId()); err != nil {
		return err
	}
	mapID, err := s.store.MapID(ctx, *v.CharacterID)
	if err != nil {
		return err
	}
	// Admission-frozen spatial evidence (protobuf_conventions.md §7):
	// channel/partition/tick fields are unproducible until the world
	// partition seam lands (IMP-018) — presence, not completeness, is
	// the contract here.
	spatial := &journalv1.JournalSource{
		MapId:        mapID,
		OccurredAtMs: s.now().UnixMilli(),
	}
	rec, err := progressiond.RespecRecord(v.AccountID, *v.CharacterID,
		v.SessionEpoch, v.OwnershipEpoch, req, spatial, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	return s.run(ctx, v, rec)
}

// consultRespec runs the NpcServiceValid admission check. ConsultPort
// semantics: nil port or an untyped consult failure is a state-class
// rejection (never a fabricated verdict); a typed ConsultError maps its
// wire code; a false verdict is OUT_OF_RANGE.
func (s *Service) consultRespec(characterID id.UUID, npcID string) error {
	if s.consults == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE}
	}
	ok, err := s.consults.NpcServiceValid(characterID.String(), npcID, ServiceRespec)
	switch {
	case err != nil:
		var ce *ConsultError
		if errors.As(err, &ce) {
			return &router.RejectError{Code: ce.Code}
		}
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE}
	case !ok:
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_OUT_OF_RANGE}
	}
	return nil
}

// run is the shared durable-admission tail: Submit → AwaitClientOutcome
// → deliver the 514 client_result on the bound conn → on success the
// 515 snapshot → Ack (deliver-then-ack; a replayed delivery may re-ack).
func (s *Service) run(ctx context.Context, v router.View,
	rec *journalv1.DurableCommandRecord) error {
	var opID id.UUID
	copy(opID[:], rec.GetOperationId())
	fam := rec.GetOperationFamily()
	owner := idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: *v.CharacterID}

	if err := s.q.Submit(ctx, rec); err != nil {
		return submitError(err)
	}
	outcome, err := s.q.AwaitClientOutcome(ctx, fam, owner, opID)
	if err != nil {
		return awaitError(err)
	}
	result := outcome.GetS2CProgressionMutateResult()
	if result == nil {
		// Out-of-set verdict: the wire contract sends S2C_ERROR.
		return &router.RejectError{Code: outcome.GetErrorCode()}
	}
	if err := s.send(v.Conn, msgIDMutateResult, result); err != nil {
		return err
	}
	if outcome.GetStatus() == protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		if err := s.emitState(ctx, v); err != nil {
			return err
		}
	}
	if err := s.q.Ack(ctx, fam, owner, opID); err != nil &&
		!strings.Contains(err.Error(), "no ackable") {
		return err
	}
	return nil
}

// emitState re-reads the committed aggregate and pushes the 515 full
// snapshot — the declared "every change" push after a committed
// progression mutation.
func (s *Service) emitState(ctx context.Context, v router.View) error {
	p, err := s.store.Read(ctx, nil, *v.CharacterID)
	if err != nil {
		return err
	}
	return s.send(v.Conn, msgIDState, ProgressionPush(p))
}

// EmitState is the composition surface for the attach-time snapshot: any
// caller holding a session conn may push the fresh 515 projection (the
// "after attach" half of the messages.md contract).
func (s *Service) EmitStatePushes(ctx context.Context, conn *listener.Conn, characterID id.UUID) error {
	p, err := s.store.Read(ctx, nil, characterID)
	if err != nil {
		return err
	}
	return s.send(conn, msgIDState, ProgressionPush(p))
}

// send marshals msg and enqueues it on the bound conn (the listener's
// registry is authoritative for the delivery class per id).
func (s *Service) send(c *listener.Conn, msgID uint32, msg proto.Message) error {
	if c == nil {
		return nil // session not yet bound to a conn
	}
	payload, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	return c.Send(&protocolv1.Envelope{MessageId: msgID, Payload: payload},
		listener.DeliveryControl)
}

// submitError maps admission failures to wire codes.
func submitError(err error) error {
	switch {
	case errors.Is(err, queue.ErrQueueFull), errors.Is(err, queue.ErrBackpressure):
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_SERVER_OVERLOADED}
	case errors.Is(err, queue.ErrErasureFenced):
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_ACCOUNT_PENDING_DELETION}
	case errors.Is(err, queue.ErrShutdown):
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_TEMPORARY_DEPENDENCY_FAILURE}
	default:
		return err
	}
}

// awaitError maps a non-committed terminal receipt to a wire code; a
// missing receipt after a successful Submit is a seam violation and is
// returned as an infrastructure error.
func awaitError(err error) error {
	var te *idempotency.TerminalError
	if errors.As(err, &te) {
		switch te.State {
		case idempotency.ReceiptExpiredUncommitted:
			return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_OPERATION_EXPIRED}
		default:
			return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_TEMPORARY_DEPENDENCY_FAILURE}
		}
	}
	return err
}
