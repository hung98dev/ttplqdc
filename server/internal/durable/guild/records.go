package guild

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Durable operation families — protobuf_conventions §7 maps every
// guild request id onto its client.<ID> CHARACTER family.
const (
	InviteFamily            = "client.608"
	AcceptFamily            = "client.610"
	DeclineFamily           = "client.623"
	LeaveFamily             = "client.624"
	KickFamily              = "client.625"
	RoleUpdateFamily        = "client.626"
	LeaderTransferFamily    = "client.627"
	CreateFamily            = "client.637"
	DisbandFamily           = "client.638"
	ApplyFamily             = "client.639"
	ApplicationDecideFamily = "client.640"
	MotdSetFamily           = "client.642"
	LeadershipClaimFamily   = "client.643"
	BlessingVoteFamily      = "client.648"
	SettingsSetFamily       = "client.650"
	InviteCancelFamily      = "client.651"
	ApplicationCancelFamily = "client.652"
)

// EventFamily is the GUILD_EVENT producer family (OwnerGuild).
const EventFamily = "guild.event"

// JobFamily is the guild JOB producer family (job.<target> = guild).
const JobFamily = "job.guild"

// Guild job kinds (protobuf_conventions §7 closed set).
const (
	JobCycleFreeze    = "CYCLE_FREEZE"
	JobVoteFinalize   = "VOTE_FINALIZE"
	JobBlessingExpire = "BLESSING_EXPIRE"
)

// requestMessageID maps each family back to the C2S wire id carried in
// the 649 request_message_id field.
var requestMessageID = map[string]uint32{
	InviteFamily:            608,
	AcceptFamily:            610,
	DeclineFamily:           623,
	LeaveFamily:             624,
	KickFamily:              625,
	RoleUpdateFamily:        626,
	LeaderTransferFamily:    627,
	CreateFamily:            637,
	DisbandFamily:           638,
	ApplyFamily:             639,
	ApplicationDecideFamily: 640,
	MotdSetFamily:           642,
	LeadershipClaimFamily:   643,
	BlessingVoteFamily:      648,
	SettingsSetFamily:       650,
	InviteCancelFamily:      651,
	ApplicationCancelFamily: 652,
}

// clientRecord assembles the schema-v1 DurableCommandRecord around a
// pre-built JournalClientCommand (same contract as durable/social).
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

// ClientRecord builds the durable record for any guild C2S request:
// it marshals req and wraps it in the matching JournalClientCommand
// oneof selected by family.
func ClientRecord(family string, accountID, characterID id.UUID,
	sessionEpoch, ownershipEpoch uint64, req proto.Message,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("guild: marshal %s: %w", family, err)
	}
	op, ok := req.(interface{ GetOperationId() []byte })
	if !ok {
		return nil, fmt.Errorf("%w: %T has no operation_id", ErrMalformedRecord, req)
	}
	cmd, err := clientCommand(family, req)
	if err != nil {
		return nil, err
	}
	return clientRecord(family, accountID, characterID, sessionEpoch,
		ownershipEpoch, cmd, op.GetOperationId(), reqBytes, admittedAt)
}

func clientCommand(family string, req proto.Message) (*journalv1.JournalClientCommand, error) {
	jc := &journalv1.JournalClientCommand{}
	switch family {
	case InviteFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildInvite{C2SGuildInvite: req.(*protocolv1.C2SGuildInvite)}
	case AcceptFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildAccept{C2SGuildAccept: req.(*protocolv1.C2SGuildAccept)}
	case DeclineFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildDecline{C2SGuildDecline: req.(*protocolv1.C2SGuildDecline)}
	case LeaveFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildLeave{C2SGuildLeave: req.(*protocolv1.C2SGuildLeave)}
	case KickFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildKick{C2SGuildKick: req.(*protocolv1.C2SGuildKick)}
	case RoleUpdateFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildRoleUpdate{C2SGuildRoleUpdate: req.(*protocolv1.C2SGuildRoleUpdate)}
	case LeaderTransferFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildLeaderTransfer{C2SGuildLeaderTransfer: req.(*protocolv1.C2SGuildLeaderTransfer)}
	case CreateFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildCreate{C2SGuildCreate: req.(*protocolv1.C2SGuildCreate)}
	case DisbandFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildDisband{C2SGuildDisband: req.(*protocolv1.C2SGuildDisband)}
	case ApplyFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildApply{C2SGuildApply: req.(*protocolv1.C2SGuildApply)}
	case ApplicationDecideFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildApplicationDecide{C2SGuildApplicationDecide: req.(*protocolv1.C2SGuildApplicationDecide)}
	case MotdSetFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildMotdSet{C2SGuildMotdSet: req.(*protocolv1.C2SGuildMotdSet)}
	case LeadershipClaimFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildLeadershipClaim{C2SGuildLeadershipClaim: req.(*protocolv1.C2SGuildLeadershipClaim)}
	case BlessingVoteFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildBlessingVote{C2SGuildBlessingVote: req.(*protocolv1.C2SGuildBlessingVote)}
	case SettingsSetFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildSettingsSet{C2SGuildSettingsSet: req.(*protocolv1.C2SGuildSettingsSet)}
	case InviteCancelFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildInviteCancel{C2SGuildInviteCancel: req.(*protocolv1.C2SGuildInviteCancel)}
	case ApplicationCancelFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildApplicationCancel{C2SGuildApplicationCancel: req.(*protocolv1.C2SGuildApplicationCancel)}
	default:
		return nil, fmt.Errorf("%w: family %q", ErrMalformedRecord, family)
	}
	return jc, nil
}

// EventRecord builds the GUILD_EVENT journal record (family
// guild.event, owner = guild).
func EventRecord(guildID, opID id.UUID, ev *journalv1.JournalGuildEvent,
	admittedAt time.Time) *journalv1.DurableCommandRecord {
	fpr := id.OperationFingerprint(EventFamily, guildID, opID, nil)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    EventFamily,
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_GUILD,
		OwnerId:            guildID[:],
		OperationId:        opID[:],
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_GUILD_EVENT,
		EnqueuedAtMs:       admittedAt.UnixMilli(),
		RequestFingerprint: fpr[:],
		Command:            &journalv1.DurableCommandRecord_GuildEvent{GuildEvent: ev},
	}
}

// JobRecord builds the job.guild journal record (kind + typed target).
func JobRecord(guildID, opID id.UUID, kind, cycleID string,
	claimID *id.UUID, dueAt time.Time) *journalv1.DurableCommandRecord {
	tgt := &journalv1.JournalGuildJob{GuildId: guildID[:], CycleId: cycleID}
	if claimID != nil {
		tgt.ClaimId = claimID[:]
	}
	fpr := id.OperationFingerprint(JobFamily, guildID, opID, nil)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    JobFamily,
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_GUILD,
		OwnerId:            guildID[:],
		OperationId:        opID[:],
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_JOB,
		EnqueuedAtMs:       dueAt.UnixMilli(),
		RequestFingerprint: fpr[:],
		Command: &journalv1.DurableCommandRecord_Job{Job: &journalv1.JournalJob{
			Kind:    kind,
			JobKey:  kind + ":" + guildID.String() + ":" + cycleID,
			DueAtMs: dueAt.UnixMilli(),
			Target:  &journalv1.JournalJob_Guild{Guild: tgt},
		}},
	}
}

// outcomeResult wraps the S2C_GUILD_RESULT in the JournalOutcome
// payload (protojson — the replayed terminal proof).
func outcomeResult(status protocolv1.ResultStatus, opID id.UUID,
	code protocolv1.ErrorCode, family string, guildID id.UUID,
	revisions []*journalv1.JournalAggregateRevision,
	created []*journalv1.JournalCreatedId) *journalv1.JournalOutcome {
	out := &journalv1.JournalOutcome{
		Status:      status,
		ErrorCode:   code,
		OperationId: opID[:],
		CreatedIds:  created,
		Revisions:   revisions,
		ClientResult: &journalv1.JournalOutcome_S2CGuildResult{
			S2CGuildResult: &protocolv1.S2CGuildResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      status,
					ErrorCode:   code,
				},
				RequestMessageId: requestMessageID[family],
			},
		},
	}
	if !guildID.IsNil() {
		out.GetS2CGuildResult().GuildId = guildID[:]
	}
	return out
}

// eventOutcome encodes a GUILD_EVENT commit — no client result, the
// typed provenance lives in the event record itself.
func eventOutcome(revisions []*journalv1.JournalAggregateRevision) *journalv1.JournalOutcome {
	return &journalv1.JournalOutcome{
		Status:    protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		Revisions: revisions,
	}
}

// marshalOutcome serializes the outcome payload for the durable op
// ledger.
func marshalOutcome(o *journalv1.JournalOutcome) ([]byte, error) {
	b, err := protojson.MarshalOptions{EmitUnpopulated: false}.Marshal(o)
	if err != nil {
		return nil, fmt.Errorf("guild: marshal outcome: %w", err)
	}
	return b, nil
}
