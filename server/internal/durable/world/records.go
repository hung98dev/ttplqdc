package world

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Closed family names (save_rules.md §7 / registry.go).
const (
	FamilyNpcService       = "interaction.npc_service"
	FamilyPlacementPortal  = "placement.portal"
	FamilyPlacementChannel = "placement.channel"
	FamilySimCheckpoint    = "sim.checkpoint"
)

// InteractRecord builds the record for C2S_INTERACT (103). For the
// registered set_checkpoint service the caller passes the checkpoint
// payload resolved by the admission consult (checkpoint = 12); other
// kinds/services pass nil.
func InteractRecord(accountID, charID id.UUID, sessionEpoch, ownershipEpoch uint64,
	req *protocolv1.C2SInteract, checkpoint *journalv1.JournalCheckpoint,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("world: interact request operation_id %d bytes", len(req.GetOperationId()))
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("world: marshal interact request: %w", err)
	}
	fpr := id.OperationFingerprint(FamilyNpcService, charID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    FamilyNpcService,
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
				Checkpoint:     checkpoint,
				Request: &journalv1.JournalClientCommand_C2SInteract{
					C2SInteract: req,
				},
			},
		},
	}, nil
}

// PortalRecord builds the record for C2S_PORTAL_USE (104).
func PortalRecord(accountID, charID id.UUID, sessionEpoch, ownershipEpoch uint64,
	req *protocolv1.C2SPortalUse, admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("world: portal request operation_id %d bytes", len(req.GetOperationId()))
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("world: marshal portal request: %w", err)
	}
	fpr := id.OperationFingerprint(FamilyPlacementPortal, charID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    FamilyPlacementPortal,
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
				Request: &journalv1.JournalClientCommand_C2SPortalUse{
					C2SPortalUse: req,
				},
			},
		},
	}, nil
}

// ChannelRecord builds the record for C2S_CHANNEL_SWITCH (109).
func ChannelRecord(accountID, charID id.UUID, sessionEpoch, ownershipEpoch uint64,
	req *protocolv1.C2SChannelSwitch, admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("world: channel request operation_id %d bytes", len(req.GetOperationId()))
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("world: marshal channel request: %w", err)
	}
	fpr := id.OperationFingerprint(FamilyPlacementChannel, charID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    FamilyPlacementChannel,
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
				Request: &journalv1.JournalClientCommand_C2SChannelSwitch{
					C2SChannelSwitch: req,
				},
			},
		},
	}, nil
}

// CheckpointRecord builds the CHECKPOINT record for one server-driven
// checkpoint/map write (family sim.checkpoint, ProducerCheckpoint).
// The emit adapter passes the flat runtime.DurableCommand identity plus
// the typed payload sim/world registered for that operation id —
// the fingerprint reuses the canonical proto encoding of the payload.
func CheckpointRecord(opID, charID id.UUID, fingerprint [32]byte,
	sourceEvent, contentRevision string,
	cp *journalv1.JournalCheckpoint, enqueuedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if cp == nil {
		return nil, fmt.Errorf("world: checkpoint record without payload")
	}
	rec := &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    FamilySimCheckpoint,
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
		OwnerId:            charID[:],
		OperationId:        opID[:],
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CHECKPOINT,
		EnqueuedAtMs:       enqueuedAt.UnixMilli(),
		RequestFingerprint: fingerprint[:],
		Command: &journalv1.DurableCommandRecord_Checkpoint{
			Checkpoint: cp,
		},
	}
	if err := validate16(rec); err != nil {
		return nil, err
	}
	return rec, nil
}

func validate16(rec *journalv1.DurableCommandRecord) error {
	if len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 {
		return fmt.Errorf("world: malformed record identity")
	}
	return nil
}

// ptrU64 materializes the optional proto scalar.
func ptrU64(v uint64) *uint64 { return &v }
