package character

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/account"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// CreateFamily is the durable family C2S_CHARACTER_CREATE (id 12)
// submits under (save_rules.md producer row; registry clientFamiliesExact).
// Its receipt owner is the ACCOUNT — the only ProducerClient family
// whose aggregate is not the character (none exists yet).
const CreateFamily = "character.create"

// classSet is the permanent five-element class roster (character.md,
// characters.class_id CHECK). Class is permanent after creation.
var classSet = map[string]struct{}{
	"class.kim":  {},
	"class.moc":  {},
	"class.thuy": {},
	"class.hoa":  {},
	"class.tho":  {},
}

// starterMap is the creation map of character.md § Creation (the
// matching checkpoint/map row defaults live on the DDL insert).
const starterMap = "map.lang_da.dinh_lang"

// CreateRecord builds the schema-v1 durable command record for one
// C2S_CHARACTER_CREATE intent. The edge fills every JournalClientCommand
// identity field from its injected session view — account_id and
// session_epoch are always present for a live session; character_id and
// ownership_epoch stay absent because id 12 is legal only while
// unattached. operation_id is the original client UUIDv7, never
// re-minted; issued_at_ms is recovered from it.
func CreateRecord(accountID id.UUID, sessionEpoch uint64,
	req *protocolv1.C2SCharacterCreate, admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("character: create request operation_id %d bytes", len(req.GetOperationId()))
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("character: marshal create request: %w", err)
	}
	fpr := id.OperationFingerprint(CreateFamily, accountID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID) // zero when the id is not v7
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    CreateFamily,
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_ACCOUNT,
		OwnerId:            accountID[:],
		OperationId:        opID[:],
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT,
		EnqueuedAtMs:       admittedAt.UnixMilli(),
		RequestFingerprint: fpr[:],
		Command: &journalv1.DurableCommandRecord_Client{
			Client: &journalv1.JournalClientCommand{
				AccountId:    accountID[:],
				SessionEpoch: sessionEpoch,
				AdmittedAtMs: admittedAt.UnixMilli(),
				IssuedAtMs:   issuedMs,
				Request: &journalv1.JournalClientCommand_C2SCharacterCreate{
					C2SCharacterCreate: req,
				},
			},
		},
	}, nil
}

// CreateExecutor is the queue.Executor for ProducerClient records of the
// character.create family, applied inside Store.TrustedReplay under the
// receipt lock (ADR-0081). Deterministic verdicts are committed
// JournalOutcome nonexecutions — a retried operation_id replays the
// retained outcome; only infrastructure failures return a Go error.
// Lock order: accounts (priority 10) before characters (priority 20).
func (s *Store) CreateExecutor(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != CreateFamily {
		return idempotency.Outcome{}, fmt.Errorf("character: unsupported client family %q", rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	if cmd == nil {
		return idempotency.Outcome{}, fmt.Errorf("character: client record without payload")
	}
	req := cmd.GetC2SCharacterCreate()
	if req == nil {
		return idempotency.Outcome{}, fmt.Errorf("character: record without c2s_character_create")
	}
	if len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 {
		return idempotency.Outcome{}, fmt.Errorf("character: malformed record identity")
	}
	var accountID, opID id.UUID
	copy(accountID[:], rec.GetOwnerId())
	copy(opID[:], rec.GetOperationId())

	// Lock the account row first so the slots check serializes per owner.
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("accounts", accountID)); err != nil {
		return idempotency.Outcome{}, err
	}
	status, err := s.AccountStatus(ctx, tx, accountID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	switch status {
	case account.StatusActive:
	case account.StatusSuspendedReconciliation:
		return s.verdict(opID, protocolv1.ErrorCode_ERROR_CODE_ACCOUNT_SUSPENDED, false)
	case account.StatusBanned:
		return s.verdict(opID, protocolv1.ErrorCode_ERROR_CODE_ACCOUNT_BANNED, false)
	case account.StatusPendingDeletion:
		return s.verdict(opID, protocolv1.ErrorCode_ERROR_CODE_ACCOUNT_PENDING_DELETION, false)
	default:
		return idempotency.Outcome{}, fmt.Errorf("character: uncreateable account status %q", status)
	}

	n, err := s.CountByAccount(ctx, tx, accountID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if n >= MaxPerAccount {
		return s.verdict(opID, protocolv1.ErrorCode_ERROR_CODE_CHARACTER_SLOTS_FULL, true)
	}
	if _, ok := classSet[req.GetClassId()]; !ok {
		return s.verdict(opID, protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID, true)
	}
	display, nameKey, err := NormalizeName(req.GetCharacterName())
	if err != nil {
		return s.verdict(opID, protocolv1.ErrorCode_ERROR_CODE_CHARACTER_NAME_INVALID, true)
	}

	// name_key is globally unique; a collision aborts the transaction, so
	// the insert runs under a savepoint — rolling back to it keeps the
	// tx live so the TAKEN verdict commits as a normal terminal outcome
	// instead of degrading to a REJECTED receipt.
	if _, err := tx.Exec(ctx, `SAVEPOINT character_create_insert`); err != nil {
		return idempotency.Outcome{}, err
	}
	characterID := id.NewV4()
	if err := s.Insert(ctx, tx, characterID, accountID, display, nameKey, req.GetClassId()); err != nil {
		if errors.Is(err, ErrNameTaken) {
			if _, rerr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT character_create_insert`); rerr != nil {
				return idempotency.Outcome{}, rerr
			}
			return s.verdict(opID, protocolv1.ErrorCode_ERROR_CODE_CHARACTER_NAME_TAKEN, true)
		}
		return idempotency.Outcome{}, err
	}
	return s.commit(opID, characterID, display, req.GetClassId())
}

// verdict writes a committed terminal nonexecution: Status ERROR plus
// the wire code. In-set rejections embed S2C_CHARACTER_CREATE_RESULT as
// the client_result member the edge delivers on id 13; out-of-set
// rejections (suspended/banned/pending) leave it absent — the edge maps
// those to S2C_ERROR.
func (s *Store) verdict(opID id.UUID, code protocolv1.ErrorCode, clientResult bool) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
		ErrorCode:   code,
		OperationId: opID[:],
	}
	if clientResult {
		outcome.ClientResult = &journalv1.JournalOutcome_S2CCharacterCreateResult{
			S2CCharacterCreateResult: &protocolv1.S2CCharacterCreateResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
					ErrorCode:   code,
				},
			},
		}
	}
	return marshalOutcome(outcome)
}

// commit writes the successful terminal outcome: the committed
// CHARACTER created_id plus the client_result the edge delivers (id 13),
// followed by its declared S2C_CHARACTER_LIST push.
func (s *Store) commit(opID, characterID id.UUID, display, classID string) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		CreatedIds: []*journalv1.JournalCreatedId{
			{Kind: "CHARACTER", Id: characterID[:]},
		},
		ClientResult: &journalv1.JournalOutcome_S2CCharacterCreateResult{
			S2CCharacterCreateResult: &protocolv1.S2CCharacterCreateResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
				},
				Character: &protocolv1.CharacterSummary{
					CharacterId:   characterID[:],
					CharacterName: display,
					ClassId:       classID,
					Level:         1,
					MapId:         starterMap,
					IsAttached:    false,
				},
			},
		},
	}
	return marshalOutcome(outcome)
}

// marshalOutcome serializes the schema-v1 JournalOutcome as protojson —
// the retained durable_command_receipts.outcome / operations.outcome
// representation the client-outcome await seam decodes.
func marshalOutcome(outcome *journalv1.JournalOutcome) (idempotency.Outcome, error) {
	b, err := protojson.Marshal(outcome)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: b}, nil
}
