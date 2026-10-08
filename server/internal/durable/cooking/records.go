package cooking

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Client families (protobuf_conventions.md §7:
// `interaction.<interact_kind lowercase>` / CHARACTER).
const (
	FamilyKindle = "interaction.kindle"
	FamilyCook   = "interaction.cook"
	FamilyRest   = "interaction.bonfire_rest"
)

// familyOf maps one registered interact_kind to its client family; the
// dispatcher guarantees the kind is registered before record building.
func familyOf(kind protocolv1.InteractKind) string {
	switch kind {
	case protocolv1.InteractKind_INTERACT_KIND_KINDLE:
		return FamilyKindle
	case protocolv1.InteractKind_INTERACT_KIND_COOK:
		return FamilyCook
	case protocolv1.InteractKind_INTERACT_KIND_BONFIRE_REST:
		return FamilyRest
	}
	return ""
}

// InteractRecord builds the ProducerClient record for a KINDLE / COOK /
// BONFIRE_REST 103 (ADR-0083). `craft` is the JournalCraftSnapshot
// frozen by PlanCook — required for COOK, absent otherwise
// (protobuf_conventions.md §7: only 103 COOK carries craft).
func InteractRecord(accountID, charID id.UUID, sessionEpoch, ownershipEpoch uint64,
	req *protocolv1.C2SInteract, craft *journalv1.JournalCraftSnapshot,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("cooking: interact request operation_id %d bytes", len(req.GetOperationId()))
	}
	kind := req.GetInteractKind()
	family := familyOf(kind)
	if family == "" {
		return nil, fmt.Errorf("cooking: unregistered interact_kind %v", kind)
	}
	if kind == protocolv1.InteractKind_INTERACT_KIND_COOK && craft == nil {
		return nil, fmt.Errorf("cooking: COOK record without craft snapshot")
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("cooking: marshal interact request: %w", err)
	}
	fpr := id.OperationFingerprint(family, charID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    family,
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
				Craft:          craft,
				Request: &journalv1.JournalClientCommand_C2SInteract{
					C2SInteract: req,
				},
			},
		},
	}, nil
}

func ptrU64(v uint64) *uint64 { return &v }

// SettlementRecord builds the ProducerReward record for one emitted
// bonfire settlement intent — sim.rest_settlement / sim.beast_settlement
// (JOURNAL_COMMAND_TYPE_REWARD envelope consumed by durable/reward
// RestExecutor or durable/beasts). The emit adapter calls it with the
// typed payload fetched from the channel's Emission ledger.
func SettlementRecord(opID, charID id.UUID, family string, fingerprint [32]byte,
	sourceEvent, contentRevision string, payload *journalv1.JournalRewardCommand,
	enqueuedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if family != "sim.rest_settlement" && family != "sim.beast_settlement" {
		return nil, fmt.Errorf("cooking: unsupported settlement family %q", family)
	}
	if payload == nil || opID == (id.UUID{}) || charID == (id.UUID{}) {
		return nil, fmt.Errorf("cooking: malformed settlement record")
	}
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    family,
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
		OwnerId:            charID[:],
		OperationId:        opID[:],
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_REWARD,
		EnqueuedAtMs:       enqueuedAt.UnixMilli(),
		RequestFingerprint: fingerprint[:],
		Command: &journalv1.DurableCommandRecord_Reward{
			Reward: payload,
		},
	}, nil
}
