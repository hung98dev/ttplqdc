package trade

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

// FinaliseFamily is the durable operation family the router maps
// C2S_TRADE_FINALISE (708) onto (save_rules § producer registry).
const FinaliseFamily = "trade.finalise"

// Deps are the executor dependencies the composition root injects:
// the store, the runtime lock-ledger lookup (the sim manager's
// Ledger accessor), and the item catalog for tier floors.
type Deps struct {
	Store   *Store
	Locks   func(id.UUID) *items.TradeLockLedger
	Catalog Catalog
}

// Executors returns the family→executor map the composition
// ProducerClient family-mux merges. Exactly the one family this task
// owns.
func Executors(d Deps) map[string]queue.Executor {
	return map[string]queue.Executor{
		FinaliseFamily: d.finaliseExecutor,
	}
}

// identity validates the shared record identity and returns
// (accountID, characterID, opID); owner must equal the character.
func identity(rec *journalv1.DurableCommandRecord) (id.UUID, id.UUID, id.UUID, error) {
	cmd := rec.GetClient()
	if cmd == nil {
		return id.UUID{}, id.UUID{}, id.UUID{}, fmt.Errorf("trade: client record without payload")
	}
	if len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 ||
		len(cmd.GetCharacterId()) != 16 || len(cmd.GetAccountId()) != 16 {
		return id.UUID{}, id.UUID{}, id.UUID{}, fmt.Errorf("trade: malformed record identity")
	}
	var accountID, charID, opID id.UUID
	copy(accountID[:], cmd.GetAccountId())
	copy(charID[:], cmd.GetCharacterId())
	copy(opID[:], rec.GetOperationId())
	if rec.GetOwnerId() != nil && string(rec.GetOwnerId()) != string(charID[:]) {
		return id.UUID{}, id.UUID{}, id.UUID{}, fmt.Errorf("trade: owner must be the character")
	}
	return accountID, charID, opID, nil
}

func marshalOutcome(outcome *journalv1.JournalOutcome) (idempotency.Outcome, error) {
	b, err := protojson.Marshal(outcome)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: b}, nil
}

func opResult(opID id.UUID, code protocolv1.ErrorCode) *protocolv1.OperationResult {
	st := protocolv1.ResultStatus_RESULT_STATUS_SUCCESS
	if code != protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED {
		st = protocolv1.ResultStatus_RESULT_STATUS_ERROR
	}
	return &protocolv1.OperationResult{OperationId: opID[:], Status: st, ErrorCode: code}
}

// finaliseExecutor commits the 708 settlement: the embedded
// JournalTrade is the committed snapshot; the store re-validates every
// admission gate authoritatively inside the transaction and either
// moves everything or rejects the whole trade.
func (d Deps) finaliseExecutor(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FinaliseFamily {
		return idempotency.Outcome{}, fmt.Errorf("trade: unsupported client family %q", rec.GetOperationFamily())
	}
	_, charID, opID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	cmd := rec.GetClient()
	j := cmd.GetTrade()
	if j == nil {
		return idempotency.Outcome{}, fmt.Errorf("trade: record without embedded journal trade")
	}
	if req := cmd.GetC2STradeFinalise(); req == nil {
		return idempotency.Outcome{}, fmt.Errorf("trade: record without c2s_trade_finalise")
	}
	var tradeID id.UUID
	copy(tradeID[:], j.GetTradeId())
	result, err := d.Store.Settle(ctx, tx, SettleIn{
		Trade:       j,
		OperationID: opID,
		Locks:       d.Locks,
		Catalog:     d.Catalog,
	})
	if err != nil {
		var v *Verdict
		if errors.As(err, &v) {
			return marshalOutcome(&journalv1.JournalOutcome{
				Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
				ErrorCode:   v.Code,
				OperationId: opID[:],
				ClientResult: &journalv1.JournalOutcome_S2CTradeResult{
					S2CTradeResult: &protocolv1.S2CTradeResult{
						Result:  opResult(opID, v.Code),
						TradeId: tradeID[:],
					},
				},
			})
		}
		return idempotency.Outcome{}, err
	}
	// The actor's 709: operation_id is the committing 708 id; Received
	// is what the actor's partner offered.
	got := result.InitiatorGot
	if charID != initChar(result, j) {
		got = result.CounterGot
	}
	return marshalOutcome(&journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		CreatedIds:  []*journalv1.JournalCreatedId{{Kind: "TRADE_SETTLEMENT", Id: result.SettlementID[:]}},
		ClientResult: &journalv1.JournalOutcome_S2CTradeResult{
			S2CTradeResult: &protocolv1.S2CTradeResult{
				Result:         opResult(opID, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED),
				TradeId:        result.TradeID[:],
				SettlementId:   result.SettlementID[:],
				Received:       got.Received,
				CommonReceived: got.CommonReceived,
				Fee:            result.Fee,
			},
		},
	})
}

// initChar decodes the initiator character of the journal.
func initChar(_ SettleOut, j *journalv1.JournalTrade) id.UUID {
	var c id.UUID
	copy(c[:], j.GetInitiator().GetCharacterId())
	return c
}

// FinaliseRecord builds the DurableCommandRecord for the admitted
// CLIENT708 — original actor-scoped receipt/request/epochs plus the
// embedded two-sided JournalTrade snapshot (packet acceptance § 708).
func FinaliseRecord(accountID id.UUID, sessionEpoch, ownershipEpoch uint64,
	characterID id.UUID, req *protocolv1.C2STradeFinalise, trade *journalv1.JournalTrade,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("trade: operation_id %d bytes", len(req.GetOperationId()))
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	request := &journalv1.JournalClientCommand{
		AccountId:      accountID[:],
		CharacterId:    characterID[:],
		SessionEpoch:   sessionEpoch,
		OwnershipEpoch: &ownershipEpoch,
		AdmittedAtMs:   admittedAt.UnixMilli(),
		Trade:          trade,
		Request:        &journalv1.JournalClientCommand_C2STradeFinalise{C2STradeFinalise: req},
	}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("trade: marshal request: %w", err)
	}
	fpr := id.OperationFingerprint(FinaliseFamily, characterID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID)
	request.IssuedAtMs = issuedMs
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    FinaliseFamily,
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
		OwnerId:            characterID[:],
		OperationId:        opID[:],
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT,
		EnqueuedAtMs:       admittedAt.UnixMilli(),
		RequestFingerprint: fpr[:],
		Command:            &journalv1.DurableCommandRecord_Client{Client: request},
	}, nil
}
