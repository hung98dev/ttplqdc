package equipment

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Record builds the schema-v1 durable command record for one
// C2S_LOADOUT_CHANGE intent. spatial carries the ADR-0083 admission
// consult evidence (nil when admission consulted nothing).
func Record(accountID id.UUID, sessionEpoch, ownershipEpoch uint64,
	characterID id.UUID, req *protocolv1.C2SLoadoutChange,
	spatial *journalv1.JournalSource, admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("equipment: operation_id %d bytes", len(req.GetOperationId()))
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("equipment: marshal request: %w", err)
	}
	fpr := id.OperationFingerprint(Family, characterID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    Family,
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
				Request: &journalv1.JournalClientCommand_C2SLoadoutChange{
					C2SLoadoutChange: req,
				},
			},
		},
	}, nil
}
