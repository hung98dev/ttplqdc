package social

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Durable operation families of the social intents — protobuf_conventions
// §7 maps every social request id onto its client.<ID> CHARACTER family.
const (
	FriendRequestFamily = "client.611"
	FriendAcceptFamily  = "client.613"
	FriendDeclineFamily = "client.614"
	FriendRemoveFamily  = "client.615"
	BlockAddFamily      = "client.617"
	BlockRemoveFamily   = "client.618"
	ReportFamily        = "client.632"
)

// requestMessageID maps each family back to the C2S wire id — the
// request_message_id carried by the 654 client_result.
var requestMessageID = map[string]uint32{
	FriendRequestFamily: 611,
	FriendAcceptFamily:  613,
	FriendDeclineFamily: 614,
	FriendRemoveFamily:  615,
	BlockAddFamily:      617,
	BlockRemoveFamily:   618,
}

// clientRecord assembles the schema-v1 DurableCommandRecord around a
// pre-built JournalClientCommand (its request oneof already set).
// Identity fields come from the injected session view; operation_id is
// the original client UUIDv7, never re-minted; issued_at_ms is
// recovered from it.
func clientRecord(family string, accountID, characterID id.UUID,
	sessionEpoch, ownershipEpoch uint64,
	cmd *journalv1.JournalClientCommand, opBytes, reqBytes []byte,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(opBytes) != 16 {
		return nil, fmt.Errorf("%w: operation_id %d bytes", ErrMalformedRecord, len(opBytes))
	}
	var opID id.UUID
	copy(opID[:], opBytes)
	fpr := id.OperationFingerprint(family, characterID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID)
	cmd.AccountId = accountID[:]
	cmd.CharacterId = characterID[:]
	cmd.SessionEpoch = sessionEpoch
	cmd.OwnershipEpoch = &ownershipEpoch
	cmd.AdmittedAtMs = admittedAt.UnixMilli()
	cmd.IssuedAtMs = issuedMs
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

// FriendRequestRecord builds the record for C2S_FRIEND_REQUEST (611).
func FriendRequestRecord(accountID, characterID id.UUID, sessionEpoch,
	ownershipEpoch uint64, req *protocolv1.C2SFriendRequest,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("social: marshal friend request: %w", err)
	}
	return clientRecord(FriendRequestFamily, accountID, characterID,
		sessionEpoch, ownershipEpoch,
		&journalv1.JournalClientCommand{
			Request: &journalv1.JournalClientCommand_C2SFriendRequest{
				C2SFriendRequest: req}},
		req.GetOperationId(), reqBytes, admittedAt)
}

// FriendAcceptRecord builds the record for C2S_FRIEND_ACCEPT (613).
func FriendAcceptRecord(accountID, characterID id.UUID, sessionEpoch,
	ownershipEpoch uint64, req *protocolv1.C2SFriendAccept,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("social: marshal friend accept: %w", err)
	}
	return clientRecord(FriendAcceptFamily, accountID, characterID,
		sessionEpoch, ownershipEpoch,
		&journalv1.JournalClientCommand{
			Request: &journalv1.JournalClientCommand_C2SFriendAccept{
				C2SFriendAccept: req}},
		req.GetOperationId(), reqBytes, admittedAt)
}

// FriendDeclineRecord builds the record for C2S_FRIEND_DECLINE (614).
func FriendDeclineRecord(accountID, characterID id.UUID, sessionEpoch,
	ownershipEpoch uint64, req *protocolv1.C2SFriendDecline,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("social: marshal friend decline: %w", err)
	}
	return clientRecord(FriendDeclineFamily, accountID, characterID,
		sessionEpoch, ownershipEpoch,
		&journalv1.JournalClientCommand{
			Request: &journalv1.JournalClientCommand_C2SFriendDecline{
				C2SFriendDecline: req}},
		req.GetOperationId(), reqBytes, admittedAt)
}

// FriendRemoveRecord builds the record for C2S_FRIEND_REMOVE (615).
func FriendRemoveRecord(accountID, characterID id.UUID, sessionEpoch,
	ownershipEpoch uint64, req *protocolv1.C2SFriendRemove,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("social: marshal friend remove: %w", err)
	}
	return clientRecord(FriendRemoveFamily, accountID, characterID,
		sessionEpoch, ownershipEpoch,
		&journalv1.JournalClientCommand{
			Request: &journalv1.JournalClientCommand_C2SFriendRemove{
				C2SFriendRemove: req}},
		req.GetOperationId(), reqBytes, admittedAt)
}

// BlockAddRecord builds the record for C2S_BLOCK_ADD (617).
func BlockAddRecord(accountID, characterID id.UUID, sessionEpoch,
	ownershipEpoch uint64, req *protocolv1.C2SBlockAdd,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("social: marshal block add: %w", err)
	}
	return clientRecord(BlockAddFamily, accountID, characterID,
		sessionEpoch, ownershipEpoch,
		&journalv1.JournalClientCommand{
			Request: &journalv1.JournalClientCommand_C2SBlockAdd{
				C2SBlockAdd: req}},
		req.GetOperationId(), reqBytes, admittedAt)
}

// BlockRemoveRecord builds the record for C2S_BLOCK_REMOVE (618).
func BlockRemoveRecord(accountID, characterID id.UUID, sessionEpoch,
	ownershipEpoch uint64, req *protocolv1.C2SBlockRemove,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("social: marshal block remove: %w", err)
	}
	return clientRecord(BlockRemoveFamily, accountID, characterID,
		sessionEpoch, ownershipEpoch,
		&journalv1.JournalClientCommand{
			Request: &journalv1.JournalClientCommand_C2SBlockRemove{
				C2SBlockRemove: req}},
		req.GetOperationId(), reqBytes, admittedAt)
}

// ReportRecord builds the record for C2S_REPORT_PLAYER (632).
func ReportRecord(accountID, characterID id.UUID, sessionEpoch,
	ownershipEpoch uint64, req *protocolv1.C2SReportPlayer,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("social: marshal report: %w", err)
	}
	return clientRecord(ReportFamily, accountID, characterID,
		sessionEpoch, ownershipEpoch,
		&journalv1.JournalClientCommand{
			Request: &journalv1.JournalClientCommand_C2SReportPlayer{
				C2SReportPlayer: req}},
		req.GetOperationId(), reqBytes, admittedAt)
}
