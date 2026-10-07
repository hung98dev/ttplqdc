package inventory

import (
	"context"
	"strings"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/inventory"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/edge/listener"
	"thinhthan/internal/edge/router"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Wire ids this service owns.
const (
	msgIDMutate           = 400 // C2S_INVENTORY_MUTATE
	msgIDExpand           = 428 // C2S_INVENTORY_EXPAND
	msgIDMutateResult     = 401 // S2C_INVENTORY_RESULT
	msgIDExpandResult     = 429 // S2C_INVENTORY_EXPAND_RESULT
	msgIDWalletState      = 432 // S2C_WALLET_STATE
	msgIDInventoryState   = 433 // S2C_INVENTORY_STATE
	msgIDEntitlementPanel = 435 // S2C_ENTITLEMENT_PANEL_STATE
)

// mutate is the router.Handler for C2S_INVENTORY_MUTATE on the
// ADR-0081 seam: attach required, the session view supplies every
// JournalClientCommand identity field, and USE admission resolves the
// ADR-0083 PartitionTick consult — its tick is frozen into the
// record's spatial_source for the executor's cooldown_ends_at_tick
// projection. A consult timeout, consult error, or no live partition
// rejects at admission with the op's state-class error; the tick is
// never fabricated. The committed JournalOutcome's client_result is
// delivered on 401; the receipt is acknowledged after delivery, and a
// retried operation_id replays the retained outcome.
func (s *Service) mutate(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SInventoryMutate)
	if !ok {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	var spatial *journalv1.JournalSource
	if req.GetOp() == protocolv1.InventoryOp_INVENTORY_OP_USE {
		if s.consults == nil {
			return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE}
		}
		tick, ok, err := s.consults.PartitionTick(*v.CharacterID)
		if err != nil || !ok {
			return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE}
		}
		spatial = &journalv1.JournalSource{Tick: tick}
	}
	rec, err := inventory.MutateRecord(v.AccountID, v.SessionEpoch,
		v.OwnershipEpoch, *v.CharacterID, req, spatial, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	var opID id.UUID
	copy(opID[:], rec.GetOperationId())
	owner := idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: *v.CharacterID}

	if err := s.q.Submit(ctx, rec); err != nil {
		return submitError(err)
	}
	outcome, err := s.q.AwaitClientOutcome(ctx, inventory.MutateFamily, owner, opID)
	if err != nil {
		return awaitError(err)
	}

	if result := outcome.GetS2CInventoryResult(); result != nil {
		if err := s.send(v.Conn, msgIDMutateResult, result); err != nil {
			return err
		}
		if outcome.GetStatus() == protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
			if err := s.emitInventoryState(ctx, v); err != nil {
				return err
			}
		}
		if err := s.q.Ack(ctx, inventory.MutateFamily, owner, opID); err != nil &&
			!strings.Contains(err.Error(), "no ackable") {
			return err
		}
		return nil
	}
	return &router.RejectError{Code: outcome.GetErrorCode()}
}

// expand is the router.Handler for C2S_INVENTORY_EXPAND: same seam,
// no consult.
func (s *Service) expand(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SInventoryExpand)
	if !ok {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	rec, err := inventory.ExpandRecord(v.AccountID, v.SessionEpoch,
		v.OwnershipEpoch, *v.CharacterID, req, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	var opID id.UUID
	copy(opID[:], rec.GetOperationId())
	owner := idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: *v.CharacterID}

	if err := s.q.Submit(ctx, rec); err != nil {
		return submitError(err)
	}
	outcome, err := s.q.AwaitClientOutcome(ctx, inventory.ExpandFamily, owner, opID)
	if err != nil {
		return awaitError(err)
	}

	if result := outcome.GetS2CInventoryExpandResult(); result != nil {
		if err := s.send(v.Conn, msgIDExpandResult, result); err != nil {
			return err
		}
		if outcome.GetStatus() == protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
			if err := s.emitWalletState(ctx, v); err != nil {
				return err
			}
			if err := s.emitInventoryState(ctx, v); err != nil {
				return err
			}
		}
		if err := s.q.Ack(ctx, inventory.ExpandFamily, owner, opID); err != nil &&
			!strings.Contains(err.Error(), "no ackable") {
			return err
		}
		return nil
	}
	return &router.RejectError{Code: outcome.GetErrorCode()}
}

// emitInventoryState pushes 433 after a committed inventory change.
func (s *Service) emitInventoryState(ctx context.Context, v router.View) error {
	if v.Conn == nil || v.CharacterID == nil {
		return nil
	}
	charID := *v.CharacterID
	var ledgerLocked func(id.UUID) int
	if s.locks != nil {
		if l := s.locks(charID); l != nil {
			ledgerLocked = l.LockedQty
		}
	}
	inv, err := s.store.InventoryPush(ctx, charID, ledgerLocked)
	if err != nil {
		return err
	}
	return s.send(v.Conn, msgIDInventoryState, inv)
}

// emitWalletState pushes 432 after a committed wallet change.
func (s *Service) emitWalletState(ctx context.Context, v router.View) error {
	if v.Conn == nil || v.CharacterID == nil {
		return nil
	}
	wallet, err := s.store.WalletPush(ctx, *v.CharacterID)
	if err != nil {
		return err
	}
	return s.send(v.Conn, msgIDWalletState, wallet)
}

// send marshals msg and enqueues it on the bound conn (DeliveryControl).
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
