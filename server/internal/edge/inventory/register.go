package inventory

import (
	"errors"

	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/queue"
	"thinhthan/internal/edge/router"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Register installs the 400/428 durable handlers on the router
// registry. 418 C2S_ENTITLEMENT_CLAIM is edge/entitlement's (IMP-102)
// and stays unbound here.
func (s *Service) Register(rt *router.Registry) error {
	var err error
	if e := rt.Register(msgIDMutate, s.mutate); e != nil {
		err = e
	}
	if e := rt.Register(msgIDExpand, s.expand); e != nil && err == nil {
		err = e
	}
	return err
}

// submitError maps admission failures to wire codes (ADR-0081 clone).
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
// missing receipt after a successful Submit is a seam violation and
// is returned as an infrastructure error.
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
