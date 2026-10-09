package fishing

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
	FamilyCast = "interaction.cast"
	FamilyHook = "interaction.hook"
)

// familyOf maps one registered interact_kind to its client family; the
// dispatcher guarantees the kind is registered before record building.
func familyOf(kind protocolv1.InteractKind) string {
	switch kind {
	case protocolv1.InteractKind_INTERACT_KIND_CAST:
		return FamilyCast
	case protocolv1.InteractKind_INTERACT_KIND_HOOK:
		return FamilyHook
	}
	return ""
}

// InteractRecord builds the ProducerClient record for a CAST / HOOK
// 103 (ADR-0083). The request rides the record verbatim; the executors
// derive every settlement input transactionally.
func InteractRecord(accountID, charID id.UUID, sessionEpoch, ownershipEpoch uint64,
	req *protocolv1.C2SInteract, admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("fishing: interact request operation_id %d bytes", len(req.GetOperationId()))
	}
	kind := req.GetInteractKind()
	family := familyOf(kind)
	if family == "" {
		return nil, fmt.Errorf("fishing: unregistered interact_kind %v", kind)
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("fishing: marshal interact request: %w", err)
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
				Request: &journalv1.JournalClientCommand_C2SInteract{
					C2SInteract: req,
				},
			},
		},
	}, nil
}

func ptrU64(v uint64) *uint64 { return &v }
