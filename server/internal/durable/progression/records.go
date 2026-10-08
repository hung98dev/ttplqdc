package progression

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Durable operation families of the 511–513 intents (save_rules.md §7
// client expansion; clientFamiliesExact).
const (
	SkillUpgradeFamily = "skill.upgrade"
	AllocateFamily     = "potential.allocate"
	RespecFamily       = "progression.respec"
)

// requestMessageID maps each family back to the C2S wire id — the
// request_message_id carried by the 514 client_result.
var requestMessageID = map[string]uint32{
	SkillUpgradeFamily: 511,
	AllocateFamily:     512,
	RespecFamily:       513,
}

// clientRecord assembles the schema-v1 DurableCommandRecord for one
// progression intent. Identity fields come from the injected session
// view (account_id/session_epoch always; character_id and
// ownership_epoch present for an attached session); operation_id is the
// original client UUIDv7, never re-minted; issued_at_ms is recovered
// from it. spatial carries the frozen admission evidence (required on
// 513 per protobuf_conventions.md §7).
func clientRecord(family string, accountID, characterID id.UUID,
	sessionEpoch, ownershipEpoch uint64, spatial *journalv1.JournalSource,
	request any, reqBytes []byte,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	var opBytes []byte
	switch r := request.(type) {
	case *journalv1.JournalClientCommand_C2SSkillUpgrade:
		opBytes = r.C2SSkillUpgrade.GetOperationId()
	case *journalv1.JournalClientCommand_C2SPotentialAllocate:
		opBytes = r.C2SPotentialAllocate.GetOperationId()
	case *journalv1.JournalClientCommand_C2SRespec:
		opBytes = r.C2SRespec.GetOperationId()
	default:
		return nil, fmt.Errorf("%w: unhandled request variant %T", ErrMalformedRecord, request)
	}
	if len(opBytes) != 16 {
		return nil, fmt.Errorf("%w: operation_id %d bytes", ErrMalformedRecord, len(opBytes))
	}
	var opID id.UUID
	copy(opID[:], opBytes)
	fpr := id.OperationFingerprint(family, characterID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID) // zero when the id is not v7
	cmd := &journalv1.JournalClientCommand{
		AccountId:      accountID[:],
		CharacterId:    characterID[:],
		SessionEpoch:   sessionEpoch,
		OwnershipEpoch: &ownershipEpoch,
		AdmittedAtMs:   admittedAt.UnixMilli(),
		IssuedAtMs:     issuedMs,
		SpatialSource:  spatial,
	}
	switch r := request.(type) {
	case *journalv1.JournalClientCommand_C2SSkillUpgrade:
		cmd.Request = r
	case *journalv1.JournalClientCommand_C2SPotentialAllocate:
		cmd.Request = r
	case *journalv1.JournalClientCommand_C2SRespec:
		cmd.Request = r
	}
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    family,
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
		OwnerId:            characterID[:],
		OperationId:        opID[:],
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT,
		EnqueuedAtMs:       admittedAt.UnixMilli(),
		RequestFingerprint: fpr[:],
		Command:            &journalv1.DurableCommandRecord_Client{Client: cmd},
	}, nil
}

// SkillUpgradeRecord builds the record for C2S_SKILL_UPGRADE (511,
// family skill.upgrade).
func SkillUpgradeRecord(accountID, characterID id.UUID, sessionEpoch,
	ownershipEpoch uint64, req *protocolv1.C2SSkillUpgrade,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("progression: marshal skill upgrade: %w", err)
	}
	return clientRecord(SkillUpgradeFamily, accountID, characterID,
		sessionEpoch, ownershipEpoch, nil,
		&journalv1.JournalClientCommand_C2SSkillUpgrade{C2SSkillUpgrade: req},
		reqBytes, admittedAt)
}

// AllocateRecord builds the record for C2S_POTENTIAL_ALLOCATE (512,
// family potential.allocate).
func AllocateRecord(accountID, characterID id.UUID, sessionEpoch,
	ownershipEpoch uint64, req *protocolv1.C2SPotentialAllocate,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("progression: marshal potential allocate: %w", err)
	}
	return clientRecord(AllocateFamily, accountID, characterID,
		sessionEpoch, ownershipEpoch, nil,
		&journalv1.JournalClientCommand_C2SPotentialAllocate{C2SPotentialAllocate: req},
		reqBytes, admittedAt)
}

// RespecRecord builds the record for C2S_RESPEC (513, family
// progression.respec). spatial_source is required by
// protobuf_conventions.md §7 — the edge supplies the frozen admission
// evidence; a nil spatial is rejected at build time, never fabricated.
func RespecRecord(accountID, characterID id.UUID, sessionEpoch,
	ownershipEpoch uint64, req *protocolv1.C2SRespec,
	spatial *journalv1.JournalSource,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if spatial == nil {
		return nil, fmt.Errorf("%w: respec requires spatial_source", ErrMalformedRecord)
	}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("progression: marshal respec: %w", err)
	}
	return clientRecord(RespecFamily, accountID, characterID,
		sessionEpoch, ownershipEpoch, spatial,
		&journalv1.JournalClientCommand_C2SRespec{C2SRespec: req},
		reqBytes, admittedAt)
}
