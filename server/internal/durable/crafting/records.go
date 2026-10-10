package crafting

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// CraftRecord builds the ProducerClient record for one C2S_CRAFT (404).
// `snap` is the JournalCraftSnapshot frozen by PlanCraft — required
// (protobuf_conventions.md §7: every 404 record carries craft).
func CraftRecord(accountID, charID id.UUID, sessionEpoch, ownershipEpoch uint64,
	req *protocolv1.C2SCraft, snap *journalv1.JournalCraftSnapshot,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("crafting: craft request operation_id %d bytes", len(req.GetOperationId()))
	}
	if snap == nil {
		return nil, fmt.Errorf("crafting: 404 record without craft snapshot")
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("crafting: marshal craft request: %w", err)
	}
	fpr := id.OperationFingerprint(FamilyCraft, charID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    FamilyCraft,
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
		OwnerId:            charID[:],
		OperationId:        opID[:],
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT,
		EnqueuedAtMs:       admittedAt.UnixMilli(),
		RequestFingerprint: fpr[:],
		Command: &journalv1.DurableCommandRecord_Client{
			Client: &journalv1.JournalClientCommand{
				AccountId:      accountID[:],
				CharacterId:    charID[:],
				SessionEpoch:   sessionEpoch,
				OwnershipEpoch: ptrU64(ownershipEpoch),
				AdmittedAtMs:   admittedAt.UnixMilli(),
				IssuedAtMs:     issuedMs,
				Craft:          snap,
				Request: &journalv1.JournalClientCommand_C2SCraft{
					C2SCraft: req,
				},
			},
		},
	}, nil
}

// EnhanceRecord builds the ProducerClient record for one C2S_ENHANCE
// (406). `result` is the JournalEnhanceResult frozen by PlanEnhance —
// required; it lands on the command's `rng_outputs` field.
func EnhanceRecord(accountID, charID id.UUID, sessionEpoch, ownershipEpoch uint64,
	req *protocolv1.C2SEnhance, result *journalv1.JournalEnhanceResult,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("crafting: enhance request operation_id %d bytes", len(req.GetOperationId()))
	}
	if result == nil {
		return nil, fmt.Errorf("crafting: 406 record without rng_outputs")
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("crafting: marshal enhance request: %w", err)
	}
	fpr := id.OperationFingerprint(FamilyEnhance, charID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    FamilyEnhance,
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
		OwnerId:            charID[:],
		OperationId:        opID[:],
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT,
		EnqueuedAtMs:       admittedAt.UnixMilli(),
		RequestFingerprint: fpr[:],
		Command: &journalv1.DurableCommandRecord_Client{
			Client: &journalv1.JournalClientCommand{
				AccountId:      accountID[:],
				CharacterId:    charID[:],
				SessionEpoch:   sessionEpoch,
				OwnershipEpoch: ptrU64(ownershipEpoch),
				AdmittedAtMs:   admittedAt.UnixMilli(),
				IssuedAtMs:     issuedMs,
				RngOutputs:     result,
				Request: &journalv1.JournalClientCommand_C2SEnhance{
					C2SEnhance: req,
				},
			},
		},
	}, nil
}

func ptrU64(v uint64) *uint64 { return &v }
