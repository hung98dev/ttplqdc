package character

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/character"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/edge/listener"
	"thinhthan/internal/edge/router"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// create is the router.Handler for C2S_CHARACTER_CREATE on the ADR-0081
// seam: the injected session view supplies every JournalClientCommand
// identity field (account_id/session_epoch always; character_id and
// ownership_epoch are absent by definition — id 12 is legal only while
// unattached). The committed JournalOutcome's client_result is delivered
// on id 13, then S2C_CHARACTER_LIST on success, and the receipt is
// acknowledged only after delivery. A retried operation_id replays the
// retained outcome with no re-execution.
func (s *Service) create(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SCharacterCreate)
	if !ok {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	rec, err := character.CreateRecord(v.AccountID, v.SessionEpoch, req, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	var opID id.UUID
	copy(opID[:], rec.GetOperationId())
	owner := idempotency.Owner{Kind: idempotency.OwnerAccount, ID: v.AccountID}

	if err := s.q.Submit(ctx, rec); err != nil {
		return submitError(err)
	}
	outcome, err := s.q.AwaitClientOutcome(ctx, character.CreateFamily, owner, opID)
	if err != nil {
		return awaitError(err)
	}

	// In-set verdicts carry the typed result member; deliver it on 13.
	if result := outcome.GetS2CCharacterCreateResult(); result != nil {
		if err := s.send(v.Conn, 13, result); err != nil {
			return err
		}
		if outcome.GetStatus() == protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
			if err := s.pushList(ctx, v); err != nil {
				return err
			}
		}
		// Deliver-then-ack (ADR-0081): a replayed delivery may re-ack an
		// already-acknowledged receipt — "no ackable terminal receipt" is
		// the only non-sentinel signal for that benign case.
		if err := s.q.Ack(ctx, character.CreateFamily, owner, opID); err != nil &&
			!strings.Contains(err.Error(), "no ackable") {
			return err
		}
		return nil
	}

	// Out-of-set verdict (suspended/banned/pending-deletion): the wire
	// contract sends S2C_ERROR, never a result-13.
	return &router.RejectError{Code: outcome.GetErrorCode()}
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

// send marshals msg and enqueues it on the bound conn (DeliveryControl —
// the listener's class registry is authoritative per id).
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

// pushList emits S2C_CHARACTER_LIST (id 14) — the declared follow-up
// push after a successful create (messages.md § Character).
func (s *Service) pushList(ctx context.Context, v router.View) error {
	rows, err := s.accounts.ListCharacters(ctx, nil, v.AccountID)
	if err != nil {
		return err
	}
	list := &protocolv1.S2CCharacterList{CharacterSlots: 3}
	for _, c := range rows {
		var lastOnline int64
		if c.LastOnlineAt != nil {
			lastOnline = c.LastOnlineAt.UnixMilli()
		}
		list.Characters = append(list.Characters, &protocolv1.CharacterSummary{
			CharacterId:    c.CharacterID[:],
			CharacterName:  c.Name,
			ClassId:        c.ClassID,
			Level:          uint32(c.Level),
			MapId:          c.MapID,
			LastOnlineAtMs: lastOnline,
			IsAttached:     c.SessionActive,
		})
	}
	return s.send(v.Conn, 14, list)
}
