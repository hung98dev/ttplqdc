package inventory

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/character"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Durable family names (save_rules.md producer registry; router's
// durableFamiliesExact binds wire ids 400/428 to them).
const (
	MutateFamily = "inventory.mutate"
	ExpandFamily = "inventory.expand"
)

// Deps are the executor dependencies the composition root injects.
// Defs resolves item_id to its runtime definition (catalog binding);
// a nil Defs is fail-closed (every item resolves ITEM_NOT_FOUND).
// Locks resolves the character's live trade-lock ledger (IMP-029's
// session object when it lands); nil resolver or nil ledger = no locks.
type Deps struct {
	Items      *items.Store
	Characters *character.Store
	Cooldowns  *CooldownTracker
	Defs       Defs
	Locks      func(id.UUID) *items.TradeLockLedger
}

// Executors returns the family→executor map the composition
// ProducerClient family-mux merges (one executor per ProducerKind in
// queue terms; feature packages export, never self-register). Exactly
// the two families this task owns — entitlement.claim is IMP-102's.
func Executors(d Deps) map[string]queue.Executor {
	return map[string]queue.Executor{
		MutateFamily: d.mutateExecutor,
		ExpandFamily: d.expandExecutor,
	}
}

// identity validates the shared record identity and returns
// (characterID, opID). Character-owned families carry a nil-Safe owner:
// cmd.character_id must equal rec.OwnerId.
func identity(rec *journalv1.DurableCommandRecord) (id.UUID, id.UUID, error) {
	cmd := rec.GetClient()
	if cmd == nil {
		return id.UUID{}, id.UUID{}, fmt.Errorf("inventory: client record without payload")
	}
	if len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 || len(cmd.GetCharacterId()) != 16 {
		return id.UUID{}, id.UUID{}, fmt.Errorf("inventory: malformed record identity")
	}
	var charID, opID, owner id.UUID
	copy(charID[:], cmd.GetCharacterId())
	copy(opID[:], rec.GetOperationId())
	copy(owner[:], rec.GetOwnerId())
	if owner != charID {
		return id.UUID{}, id.UUID{}, fmt.Errorf("inventory: owner %s != character %s", owner, charID)
	}
	return charID, opID, nil
}

// MutateRecord builds the schema-v1 durable command record for one
// C2S_INVENTORY_MUTATE intent. spatial carries the ADR-0083 admission
// consult evidence (nil when the op needs none — USE requires it).
func MutateRecord(accountID id.UUID, sessionEpoch, ownershipEpoch uint64,
	characterID id.UUID, req *protocolv1.C2SInventoryMutate,
	spatial *journalv1.JournalSource, admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("inventory: mutate request operation_id %d bytes", len(req.GetOperationId()))
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("inventory: marshal mutate request: %w", err)
	}
	fpr := id.OperationFingerprint(MutateFamily, characterID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    MutateFamily,
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
		OwnerId:            characterID[:],
		OperationId:        opID[:],
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT,
		EnqueuedAtMs:       admittedAt.UnixMilli(),
		RequestFingerprint: fpr[:],
		Command: &journalv1.DurableCommandRecord_Client{
			Client: &journalv1.JournalClientCommand{
				AccountId:      accountID[:],
				CharacterId:    characterID[:],
				SessionEpoch:   sessionEpoch,
				OwnershipEpoch: &ownershipEpoch,
				AdmittedAtMs:   admittedAt.UnixMilli(),
				IssuedAtMs:     issuedMs,
				SpatialSource:  spatial,
				Request: &journalv1.JournalClientCommand_C2SInventoryMutate{
					C2SInventoryMutate: req,
				},
			},
		},
	}, nil
}

// ExpandRecord builds the record for C2S_INVENTORY_EXPAND (field 1428).
func ExpandRecord(accountID id.UUID, sessionEpoch, ownershipEpoch uint64,
	characterID id.UUID, req *protocolv1.C2SInventoryExpand,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("inventory: expand request operation_id %d bytes", len(req.GetOperationId()))
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("inventory: marshal expand request: %w", err)
	}
	fpr := id.OperationFingerprint(ExpandFamily, characterID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    ExpandFamily,
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
		OwnerId:            characterID[:],
		OperationId:        opID[:],
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT,
		EnqueuedAtMs:       admittedAt.UnixMilli(),
		RequestFingerprint: fpr[:],
		Command: &journalv1.DurableCommandRecord_Client{
			Client: &journalv1.JournalClientCommand{
				AccountId:      accountID[:],
				CharacterId:    characterID[:],
				SessionEpoch:   sessionEpoch,
				OwnershipEpoch: &ownershipEpoch,
				AdmittedAtMs:   admittedAt.UnixMilli(),
				IssuedAtMs:     issuedMs,
				Request: &journalv1.JournalClientCommand_C2SInventoryExpand{
					C2SInventoryExpand: req,
				},
			},
		},
	}, nil
}
