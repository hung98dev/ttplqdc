package auction

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Durable family names (save_rules.md §7 / registry clientFamiliesExact).
const (
	ListFamily     = "auction.list"
	BuyFamily      = "auction.buy"
	CancelFamily   = "auction.cancel"
	ReclaimFamily  = "auction.reclaim"
	ProceedsFamily = "auction.proceeds"
	JobKind        = "auction"
)

// Deps are the executor dependencies the composition root injects.
type Deps struct {
	Store *Store
	Locks func(id.UUID) *items.TradeLockLedger
}

// Executors returns the family→executor map the composition
// ProducerClient family-mux merges. Exactly the five families this task
// owns; the `auction` JournalJob executor joins via JobExecutors.
func Executors(d Deps) map[string]queue.Executor {
	return map[string]queue.Executor{
		ListFamily:     d.listExecutor,
		BuyFamily:      d.buyExecutor,
		CancelFamily:   d.cancelExecutor,
		ReclaimFamily:  d.reclaimExecutor,
		ProceedsFamily: d.proceedsExecutor,
	}
}

// identity validates the shared record identity and returns
// (accountID, characterID, opID); owner must equal the character.
func identity(rec *journalv1.DurableCommandRecord) (id.UUID, id.UUID, id.UUID, error) {
	cmd := rec.GetClient()
	if cmd == nil {
		return id.UUID{}, id.UUID{}, id.UUID{}, fmt.Errorf("auction: client record without payload")
	}
	if len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 ||
		len(cmd.GetCharacterId()) != 16 || len(cmd.GetAccountId()) != 16 {
		return id.UUID{}, id.UUID{}, id.UUID{}, fmt.Errorf("auction: malformed record identity")
	}
	var accountID, charID, opID, owner id.UUID
	copy(accountID[:], cmd.GetAccountId())
	copy(charID[:], cmd.GetCharacterId())
	copy(opID[:], rec.GetOperationId())
	copy(owner[:], rec.GetOwnerId())
	if owner != charID {
		return id.UUID{}, id.UUID{}, id.UUID{}, fmt.Errorf("auction: owner %s != character %s", owner, charID)
	}
	return accountID, charID, opID, nil
}

// codeOf maps a domain rejection to its wire ErrorCode.
func codeOf(err error) protocolv1.ErrorCode {
	switch {
	case errors.Is(err, ErrLevelGate):
		return protocolv1.ErrorCode_ERROR_CODE_AH_ELIGIBILITY_LEVEL_REQUIRED
	case errors.Is(err, ErrAgeGate):
		return protocolv1.ErrorCode_ERROR_CODE_AH_ELIGIBILITY_AGE_REQUIRED
	case errors.Is(err, ErrBelowFloor):
		return protocolv1.ErrorCode_ERROR_CODE_AH_PRICE_FLOOR_NOT_MET
	case errors.Is(err, ErrSameAccount):
		return protocolv1.ErrorCode_ERROR_CODE_SAME_ACCOUNT_FORBIDDEN
	case errors.Is(err, ErrNotActive):
		return protocolv1.ErrorCode_ERROR_CODE_AUCTION_LISTING_NOT_ACTIVE
	case errors.Is(err, ErrCapacityFull):
		return protocolv1.ErrorCode_ERROR_CODE_CAPACITY_FULL
	case errors.Is(err, ErrItemLocked):
		return protocolv1.ErrorCode_ERROR_CODE_ITEM_LOCKED
	case errors.Is(err, ErrInsufficient):
		return protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_CURRENCY
	case errors.Is(err, ErrInventoryFull):
		return protocolv1.ErrorCode_ERROR_CODE_INVENTORY_FULL
	case errors.Is(err, ErrCapExceeded):
		return protocolv1.ErrorCode_ERROR_CODE_CURRENCY_CAP_EXCEEDED
	case errors.Is(err, ErrStateConflict):
		return protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT
	case errors.Is(err, ErrOutOfRange), errors.Is(err, ErrNotSeller),
		errors.Is(err, ErrNotReclaimable), errors.Is(err, ErrProceedsNotPending),
		errors.Is(err, ErrNotEscrowed), errors.Is(err, ErrUntradable):
		return protocolv1.ErrorCode_ERROR_CODE_OUT_OF_RANGE
	default:
		return protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE
	}
}

// opResult wraps a wire code/status pair for an S2C result message.
func opResult(opID id.UUID, code protocolv1.ErrorCode) *protocolv1.OperationResult {
	status := protocolv1.ResultStatus_RESULT_STATUS_SUCCESS
	if code != protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		status = protocolv1.ResultStatus_RESULT_STATUS_ERROR
	}
	return &protocolv1.OperationResult{OperationId: opID[:], Status: status, ErrorCode: code}
}

func (d Deps) locks(charID id.UUID) *items.TradeLockLedger {
	if d.Locks == nil {
		return nil
	}
	return d.Locks(charID)
}

func (d Deps) listExecutor(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != ListFamily {
		return idempotency.Outcome{}, fmt.Errorf("auction: unsupported client family %q", rec.GetOperationFamily())
	}
	accountID, charID, opID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	req := rec.GetClient().GetC2SAuctionList()
	if req == nil {
		return idempotency.Outcome{}, fmt.Errorf("auction: record without c2s_auction_list")
	}
	var iid id.UUID
	copy(iid[:], req.GetItemInstanceId())
	l, err := d.Store.List(ctx, tx, ListIn{
		OperationID: opID, CharacterID: charID, AccountID: accountID,
		SessionEpoch:   rec.GetClient().GetSessionEpoch(),
		ItemInstanceID: iid, Quantity: int64(req.GetQuantity()),
		PriceCommon: req.GetPriceCommon(), Lock: d.locks(charID),
	})
	if err != nil {
		return d.verdict(opID, err, func(r *protocolv1.OperationResult) *journalv1.JournalOutcome {
			return &journalv1.JournalOutcome{ClientResult: &journalv1.JournalOutcome_S2CAuctionListResult{
				S2CAuctionListResult: &protocolv1.S2CAuctionListResult{Result: r}}}
		})
	}
	return marshalOutcome(&journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		CreatedIds:  []*journalv1.JournalCreatedId{{Kind: "AUCTION_LISTING", Id: l.ListingID[:]}},
		ClientResult: &journalv1.JournalOutcome_S2CAuctionListResult{
			S2CAuctionListResult: &protocolv1.S2CAuctionListResult{
				Result:     opResult(opID, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED),
				ListingId:  l.ListingID[:],
				ListingFee: l.ListingFeeCommon,
				ExpiresAt:  l.ExpiresAt.UnixMilli(),
			}},
	})
}

func (d Deps) buyExecutor(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != BuyFamily {
		return idempotency.Outcome{}, fmt.Errorf("auction: unsupported client family %q", rec.GetOperationFamily())
	}
	accountID, charID, opID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	req := rec.GetClient().GetC2SAuctionBuy()
	if req == nil {
		return idempotency.Outcome{}, fmt.Errorf("auction: record without c2s_auction_buy")
	}
	var lid id.UUID
	copy(lid[:], req.GetListingId())
	out, err := d.Store.Buy(ctx, tx, BuyIn{
		OperationID: opID, CharacterID: charID, AccountID: accountID,
		ListingID: lid, ExpectedPriceCommon: req.GetExpectedPriceCommon(),
		Lock: d.locks(charID),
	})
	if err != nil {
		return d.verdict(opID, err, func(r *protocolv1.OperationResult) *journalv1.JournalOutcome {
			return &journalv1.JournalOutcome{ClientResult: &journalv1.JournalOutcome_S2CAuctionBuyResult{
				S2CAuctionBuyResult: &protocolv1.S2CAuctionBuyResult{Result: r, ListingId: lid[:]}}}
		})
	}
	return marshalOutcome(&journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		CreatedIds:  []*journalv1.JournalCreatedId{{Kind: "AUCTION_PROCEEDS", Id: out.ProceedsID[:]}},
		ClientResult: &journalv1.JournalOutcome_S2CAuctionBuyResult{
			S2CAuctionBuyResult: &protocolv1.S2CAuctionBuyResult{
				Result:      opResult(opID, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED),
				ListingId:   lid[:],
				PriceCommon: out.Price,
			}},
	})
}

func (d Deps) cancelExecutor(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != CancelFamily {
		return idempotency.Outcome{}, fmt.Errorf("auction: unsupported client family %q", rec.GetOperationFamily())
	}
	_, charID, opID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	req := rec.GetClient().GetC2SAuctionCancelListing()
	if req == nil {
		return idempotency.Outcome{}, fmt.Errorf("auction: record without c2s_auction_cancel_listing")
	}
	var lid id.UUID
	copy(lid[:], req.GetListingId())
	l, err := d.Store.Cancel(ctx, tx, CancelIn{
		OperationID: opID, CharacterID: charID, AccountID: mustAccount(rec), ListingID: lid,
	})
	if err != nil {
		return d.verdict(opID, err, func(r *protocolv1.OperationResult) *journalv1.JournalOutcome {
			return &journalv1.JournalOutcome{ClientResult: &journalv1.JournalOutcome_S2CAuctionCancelResult{
				S2CAuctionCancelResult: &protocolv1.S2CAuctionCancelResult{Result: r, ListingId: lid[:]}}}
		})
	}
	return marshalOutcome(&journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CAuctionCancelResult{
			S2CAuctionCancelResult: &protocolv1.S2CAuctionCancelResult{
				Result:        opResult(opID, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED),
				ListingId:     lid[:],
				EscrowAssetId: l.ItemInstanceID[:],
			}},
	})
}

func (d Deps) reclaimExecutor(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != ReclaimFamily {
		return idempotency.Outcome{}, fmt.Errorf("auction: unsupported client family %q", rec.GetOperationFamily())
	}
	_, charID, opID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	req := rec.GetClient().GetC2SAuctionReclaim()
	if req == nil {
		return idempotency.Outcome{}, fmt.Errorf("auction: record without c2s_auction_reclaim")
	}
	var lid id.UUID
	copy(lid[:], req.GetEscrowAssetId())
	l, err := d.Store.Reclaim(ctx, tx, ReclaimIn{
		OperationID: opID, CharacterID: charID, AccountID: mustAccount(rec),
		ListingID: lid, Lock: d.locks(charID),
	})
	if err != nil {
		return d.verdict(opID, err, func(r *protocolv1.OperationResult) *journalv1.JournalOutcome {
			return &journalv1.JournalOutcome{ClientResult: &journalv1.JournalOutcome_S2CAuctionReclaimResult{
				S2CAuctionReclaimResult: &protocolv1.S2CAuctionReclaimResult{Result: r, EscrowAssetId: lid[:]}}}
		})
	}
	return marshalOutcome(&journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CAuctionReclaimResult{
			S2CAuctionReclaimResult: &protocolv1.S2CAuctionReclaimResult{
				Result:        opResult(opID, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED),
				EscrowAssetId: l.ItemInstanceID[:],
			}},
	})
}

func (d Deps) proceedsExecutor(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != ProceedsFamily {
		return idempotency.Outcome{}, fmt.Errorf("auction: unsupported client family %q", rec.GetOperationFamily())
	}
	_, charID, opID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	req := rec.GetClient().GetC2SAuctionProceedsClaim()
	if req == nil {
		return idempotency.Outcome{}, fmt.Errorf("auction: record without c2s_auction_proceeds_claim")
	}
	var pid id.UUID
	copy(pid[:], req.GetProceedsId())
	p, err := d.Store.ClaimProceeds(ctx, tx, ProceedsClaimIn{
		OperationID: opID, CharacterID: charID, ProceedsID: pid,
	})
	if err != nil {
		return d.verdict(opID, err, func(r *protocolv1.OperationResult) *journalv1.JournalOutcome {
			return &journalv1.JournalOutcome{ClientResult: &journalv1.JournalOutcome_S2CAuctionProceedsResult{
				S2CAuctionProceedsResult: &protocolv1.S2CAuctionProceedsResult{Result: r, ProceedsId: pid[:]}}}
		})
	}
	return marshalOutcome(&journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CAuctionProceedsResult{
			S2CAuctionProceedsResult: &protocolv1.S2CAuctionProceedsResult{
				Result:       opResult(opID, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED),
				ProceedsId:   pid[:],
				AmountCommon: p.ProceedsAmount,
			}},
	})
}

// verdict commits a deterministic rejection as a JournalOutcome
// nonexecution carrying the in-set S2C result for the family.
func (d Deps) verdict(opID id.UUID, err error,
	wrap func(*protocolv1.OperationResult) *journalv1.JournalOutcome) (idempotency.Outcome, error) {
	if !IsRejectCode(err) {
		return idempotency.Outcome{}, err
	}
	code := codeOf(err)
	outcome := wrap(opResult(opID, code))
	outcome.Status = protocolv1.ResultStatus_RESULT_STATUS_ERROR
	outcome.ErrorCode = code
	outcome.OperationId = opID[:]
	return marshalOutcome(outcome)
}

func mustAccount(rec *journalv1.DurableCommandRecord) id.UUID {
	var a id.UUID
	copy(a[:], rec.GetClient().GetAccountId())
	return a
}

func marshalOutcome(outcome *journalv1.JournalOutcome) (idempotency.Outcome, error) {
	b, err := protojson.Marshal(outcome)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: b}, nil
}

// clientRecord builds the shared JournalClientCommand identity for one
// attached-session auction intent; opID is the original client UUIDv7.
func clientRecord(family string, accountID id.UUID, sessionEpoch, ownershipEpoch uint64,
	characterID id.UUID, req proto.Message, opID id.UUID,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	var request *journalv1.JournalClientCommand
	switch r := req.(type) {
	case *protocolv1.C2SAuctionList:
		request = &journalv1.JournalClientCommand{Request: &journalv1.JournalClientCommand_C2SAuctionList{C2SAuctionList: r}}
	case *protocolv1.C2SAuctionBuy:
		request = &journalv1.JournalClientCommand{Request: &journalv1.JournalClientCommand_C2SAuctionBuy{C2SAuctionBuy: r}}
	case *protocolv1.C2SAuctionCancelListing:
		request = &journalv1.JournalClientCommand{Request: &journalv1.JournalClientCommand_C2SAuctionCancelListing{C2SAuctionCancelListing: r}}
	case *protocolv1.C2SAuctionReclaim:
		request = &journalv1.JournalClientCommand{Request: &journalv1.JournalClientCommand_C2SAuctionReclaim{C2SAuctionReclaim: r}}
	case *protocolv1.C2SAuctionProceedsClaim:
		request = &journalv1.JournalClientCommand{Request: &journalv1.JournalClientCommand_C2SAuctionProceedsClaim{C2SAuctionProceedsClaim: r}}
	default:
		return nil, fmt.Errorf("auction: unsupported request type %T", req)
	}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("auction: marshal %s request: %w", family, err)
	}
	fpr := id.OperationFingerprint(family, characterID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID)
	request.AccountId = accountID[:]
	request.CharacterId = characterID[:]
	request.SessionEpoch = sessionEpoch
	request.OwnershipEpoch = &ownershipEpoch
	request.AdmittedAtMs = admittedAt.UnixMilli()
	request.IssuedAtMs = issuedMs
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    family,
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
		OwnerId:            characterID[:],
		OperationId:        opID[:],
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT,
		EnqueuedAtMs:       admittedAt.UnixMilli(),
		RequestFingerprint: fpr[:],
		Command: &journalv1.DurableCommandRecord_Client{
			Client: request,
		},
	}, nil
}

func opIDOf(raw []byte) (id.UUID, error) {
	if len(raw) != 16 {
		return id.UUID{}, fmt.Errorf("auction: operation_id %d bytes", len(raw))
	}
	var opID id.UUID
	copy(opID[:], raw)
	return opID, nil
}

// ListRecord builds the record for C2S_AUCTION_LIST (730).
func ListRecord(accountID id.UUID, sessionEpoch, ownershipEpoch uint64,
	characterID id.UUID, req *protocolv1.C2SAuctionList,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	opID, err := opIDOf(req.GetOperationId())
	if err != nil {
		return nil, err
	}
	return clientRecord(ListFamily, accountID, sessionEpoch, ownershipEpoch, characterID,
		req, opID, admittedAt)
}

// BuyRecord builds the record for C2S_AUCTION_BUY (732).
func BuyRecord(accountID id.UUID, sessionEpoch, ownershipEpoch uint64,
	characterID id.UUID, req *protocolv1.C2SAuctionBuy,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	opID, err := opIDOf(req.GetOperationId())
	if err != nil {
		return nil, err
	}
	return clientRecord(BuyFamily, accountID, sessionEpoch, ownershipEpoch, characterID,
		req, opID, admittedAt)
}

// CancelRecord builds the record for C2S_AUCTION_CANCEL_LISTING (734).
func CancelRecord(accountID id.UUID, sessionEpoch, ownershipEpoch uint64,
	characterID id.UUID, req *protocolv1.C2SAuctionCancelListing,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	opID, err := opIDOf(req.GetOperationId())
	if err != nil {
		return nil, err
	}
	return clientRecord(CancelFamily, accountID, sessionEpoch, ownershipEpoch, characterID,
		req, opID, admittedAt)
}

// ReclaimRecord builds the record for C2S_AUCTION_RECLAIM (740).
func ReclaimRecord(accountID id.UUID, sessionEpoch, ownershipEpoch uint64,
	characterID id.UUID, req *protocolv1.C2SAuctionReclaim,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	opID, err := opIDOf(req.GetOperationId())
	if err != nil {
		return nil, err
	}
	return clientRecord(ReclaimFamily, accountID, sessionEpoch, ownershipEpoch, characterID,
		req, opID, admittedAt)
}

// ProceedsClaimRecord builds the record for C2S_AUCTION_PROCEEDS_CLAIM (742).
func ProceedsClaimRecord(accountID id.UUID, sessionEpoch, ownershipEpoch uint64,
	characterID id.UUID, req *protocolv1.C2SAuctionProceedsClaim,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	opID, err := opIDOf(req.GetOperationId())
	if err != nil {
		return nil, err
	}
	return clientRecord(ProceedsFamily, accountID, sessionEpoch, ownershipEpoch, characterID,
		req, opID, admittedAt)
}
