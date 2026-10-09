package guild_storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/guild"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/inventory"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// client.<id> families (messages.md / protobuf_conventions.md §7).
const (
	DepositFamily      = "client.629"
	WithdrawFamily     = "client.630"
	MoveFamily         = "client.644"
	ClaimRequestFamily = "client.645"
	ClaimDecideFamily  = "client.646"
)

// requestMessageID maps each family back to the C2S wire id carried
// in the 649 request_message_id field.
var requestMessageID = map[string]uint32{
	DepositFamily:      629,
	WithdrawFamily:     630,
	MoveFamily:         644,
	ClaimRequestFamily: 645,
	ClaimDecideFamily:  646,
}

// Deps are the executor dependencies.
type Deps struct {
	Store *Store
	Guild *guild.Store
	Items *items.Store
	Inv   *inventory.Store
	Now   func() time.Time
}

// Executors returns one queue.Executor per guild-storage client.<ID>
// family (family-muxed by the composition root, ADR-0081 — the
// package never self-registers). The STORAGE_CLAIM_EXPIRE sweep is
// exported separately as StorageClaimExpireJob for the job.guild
// kind-mux.
func (d Deps) Executors() map[string]queue.Executor {
	if d.Now == nil {
		d.Now = func() time.Time { return time.Now().UTC() }
	}
	return map[string]queue.Executor{
		DepositFamily:      d.depositExec,
		WithdrawFamily:     d.withdrawExec,
		MoveFamily:         d.moveExec,
		ClaimRequestFamily: d.claimRequestExec,
		ClaimDecideFamily:  d.claimDecideExec,
	}
}

// ---------------------------------------------------------------------------
// Shared record plumbing (mirrors durable/guild's identity/family checks).

func identity(rec *journalv1.DurableCommandRecord) (opID, characterID id.UUID, err error) {
	if len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 {
		return opID, characterID, ErrMalformedRecord
	}
	copy(opID[:], rec.GetOperationId())
	copy(characterID[:], rec.GetOwnerId())
	return opID, characterID, nil
}

func cmdIdentity(cmd *journalv1.JournalClientCommand, characterID id.UUID) error {
	if cmd == nil {
		return ErrMalformedRecord
	}
	if len(cmd.GetCharacterId()) != 16 {
		return ErrMalformedRecord
	}
	var cid id.UUID
	copy(cid[:], cmd.GetCharacterId())
	if cid != characterID {
		return fmt.Errorf("%w: owner %v vs character %v", ErrMalformedRecord, characterID, cid)
	}
	return nil
}

func targetID(b []byte) (id.UUID, error) {
	var t id.UUID
	if len(b) != 16 {
		return t, fmt.Errorf("%w: target %d bytes", ErrMalformedRecord, len(b))
	}
	copy(t[:], b)
	return t, nil
}

func family(rec *journalv1.DurableCommandRecord, want string) error {
	if rec.GetOperationFamily() != want {
		return fmt.Errorf("%w: family %q", ErrMalformedRecord, rec.GetOperationFamily())
	}
	return nil
}

// sectionOf maps the wire enum onto the canonical section strings.
func sectionOf(s protocolv1.GuildStorageSection) (string, error) {
	switch s {
	case protocolv1.GuildStorageSection_GUILD_STORAGE_SECTION_COMMON:
		return SectionCommon, nil
	case protocolv1.GuildStorageSection_GUILD_STORAGE_SECTION_RESERVE:
		return SectionReserve, nil
	default:
		return "", errInvalidSection
	}
}

// ---------------------------------------------------------------------------
// Verdict plumbing: domain errors map to a 649 ERROR outcome carrying
// the request's wire id; success emits SUCCESS + the storage revision.

func (d Deps) verdict(rec *journalv1.DurableCommandRecord, opID id.UUID,
	guildID id.UUID, err error) (idempotency.Outcome, error) {
	code := codeOf(err)
	out := outcomeResult(protocolv1.ResultStatus_RESULT_STATUS_ERROR,
		opID, code, rec.GetOperationFamily(), guildID, nil, nil)
	payload, merr := marshalOutcome(out)
	if merr != nil {
		return idempotency.Outcome{}, merr
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: payload}, nil
}

func (d Deps) success(rec *journalv1.DurableCommandRecord, opID id.UUID,
	guildID id.UUID, rev uint64, created []*journalv1.JournalCreatedId) (idempotency.Outcome, error) {
	revs := []*journalv1.JournalAggregateRevision{
		{Aggregate: "guild_storage", OwnerId: guildID[:], Revision: rev},
	}
	out := outcomeResult(protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		opID, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED,
		rec.GetOperationFamily(), guildID, revs, created)
	payload, err := marshalOutcome(out)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: payload}, nil
}

// marshalOutcome serializes the outcome payload for the durable op
// ledger.
func marshalOutcome(o *journalv1.JournalOutcome) ([]byte, error) {
	b, err := protojson.MarshalOptions{EmitUnpopulated: false}.Marshal(o)
	if err != nil {
		return nil, fmt.Errorf("guild_storage: marshal outcome: %w", err)
	}
	return b, nil
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

// ---------------------------------------------------------------------------
// Executors.

func (d Deps) depositExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, DepositFamily); err != nil {
		return idempotency.Outcome{}, err
	}
	opID, actor, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	cmd := rec.GetClient()
	req := cmd.GetC2SGuildStorageDeposit()
	if cmdIdentity(cmd, actor) != nil || req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	instanceID, err := targetID(req.GetItemInstanceId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	section, err := sectionOf(req.GetSection())
	if err != nil {
		return d.verdict(rec, opID, id.UUID{}, err)
	}
	rev, err := d.Deposit(ctx, tx, opID, actor, instanceID, req.GetQuantity(), section)
	if err != nil {
		return d.verdict(rec, opID, guildOf(ctx, tx, d.Guild, actor), err)
	}
	return d.success(rec, opID, guildOf(ctx, tx, d.Guild, actor), rev, nil)
}

func (d Deps) withdrawExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, WithdrawFamily); err != nil {
		return idempotency.Outcome{}, err
	}
	opID, actor, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	cmd := rec.GetClient()
	req := cmd.GetC2SGuildStorageWithdraw()
	if cmdIdentity(cmd, actor) != nil || req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	instanceID, err := targetID(req.GetItemInstanceId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	section, err := sectionOf(req.GetSection())
	if err != nil {
		return d.verdict(rec, opID, id.UUID{}, err)
	}
	rev, err := d.Withdraw(ctx, tx, opID, actor, instanceID, req.GetQuantity(), section)
	if err != nil {
		return d.verdict(rec, opID, guildOf(ctx, tx, d.Guild, actor), err)
	}
	return d.success(rec, opID, guildOf(ctx, tx, d.Guild, actor), rev, nil)
}

func (d Deps) moveExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, MoveFamily); err != nil {
		return idempotency.Outcome{}, err
	}
	opID, actor, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	cmd := rec.GetClient()
	req := cmd.GetC2SGuildStorageMove()
	if cmdIdentity(cmd, actor) != nil || req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	instanceID, err := targetID(req.GetItemInstanceId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	to, err := sectionOf(req.GetToSection())
	if err != nil {
		return d.verdict(rec, opID, id.UUID{}, err)
	}
	rev, err := d.Move(ctx, tx, opID, actor, instanceID, to)
	if err != nil {
		return d.verdict(rec, opID, guildOf(ctx, tx, d.Guild, actor), err)
	}
	return d.success(rec, opID, guildOf(ctx, tx, d.Guild, actor), rev, nil)
}

func (d Deps) claimRequestExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, ClaimRequestFamily); err != nil {
		return idempotency.Outcome{}, err
	}
	opID, actor, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	cmd := rec.GetClient()
	req := cmd.GetC2SGuildStorageClaimRequest()
	if cmdIdentity(cmd, actor) != nil || req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	instanceID, err := targetID(req.GetItemInstanceId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	claimID, rev, err := d.ClaimRequest(ctx, tx, opID, actor, instanceID, req.GetQuantity())
	if err != nil {
		return d.verdict(rec, opID, guildOf(ctx, tx, d.Guild, actor), err)
	}
	created := []*journalv1.JournalCreatedId{
		{Kind: "guild_storage_claim", Id: claimID[:]},
	}
	return d.success(rec, opID, guildOf(ctx, tx, d.Guild, actor), rev, created)
}

func (d Deps) claimDecideExec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, ClaimDecideFamily); err != nil {
		return idempotency.Outcome{}, err
	}
	opID, actor, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	cmd := rec.GetClient()
	req := cmd.GetC2SGuildStorageClaimDecide()
	if cmdIdentity(cmd, actor) != nil || req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	claimID, err := targetID(req.GetClaimId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	rev, err := d.ClaimDecide(ctx, tx, opID, actor, claimID, req.GetDecision())
	if err != nil {
		return d.verdict(rec, opID, guildOf(ctx, tx, d.Guild, actor), err)
	}
	return d.success(rec, opID, guildOf(ctx, tx, d.Guild, actor), rev, nil)
}

// guildOf resolves the actor's guild for the verdict's guild_id field.
func guildOf(ctx context.Context, tx pgx.Tx, gs *guild.Store, actor id.UUID) id.UUID {
	m, err := gs.Member(ctx, tx, actor)
	if err != nil || m == nil {
		return id.UUID{}
	}
	return m.GuildID
}

// Record builds the client.<id> journal record for one storage request
// (test + producer path share it).
func Record(family string, actor id.UUID, opID id.UUID,
	req proto.Message, admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	jc := &journalv1.JournalClientCommand{
		CharacterId:  actor[:],
		AdmittedAtMs: admittedAt.UnixMilli(),
	}
	switch family {
	case DepositFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildStorageDeposit{C2SGuildStorageDeposit: req.(*protocolv1.C2SGuildStorageDeposit)}
	case WithdrawFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildStorageWithdraw{C2SGuildStorageWithdraw: req.(*protocolv1.C2SGuildStorageWithdraw)}
	case MoveFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildStorageMove{C2SGuildStorageMove: req.(*protocolv1.C2SGuildStorageMove)}
	case ClaimRequestFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildStorageClaimRequest{C2SGuildStorageClaimRequest: req.(*protocolv1.C2SGuildStorageClaimRequest)}
	case ClaimDecideFamily:
		jc.Request = &journalv1.JournalClientCommand_C2SGuildStorageClaimDecide{C2SGuildStorageClaimDecide: req.(*protocolv1.C2SGuildStorageClaimDecide)}
	default:
		return nil, fmt.Errorf("%w: family %q", ErrMalformedRecord, family)
	}
	fpr := id.OperationFingerprint(family, actor, opID, nil)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    family,
		OwnerKind:          journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER,
		OwnerId:            actor[:],
		OperationId:        opID[:],
		CommandType:        journalv1.JournalCommandType_JOURNAL_COMMAND_TYPE_CLIENT,
		EnqueuedAtMs:       admittedAt.UnixMilli(),
		RequestFingerprint: fpr[:],
		Command:            &journalv1.DurableCommandRecord_Client{Client: jc},
	}, nil
}

// JobRecord builds the job.guild STORAGE_CLAIM_EXPIRE record.
func JobRecord(guildID, opID id.UUID, dueAt time.Time) *journalv1.DurableCommandRecord {
	tgt := &journalv1.JournalGuildJob{GuildId: guildID[:]}
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
			Kind:    JobKindStorageClaimExpire,
			JobKey:  JobKindStorageClaimExpire + ":" + guildID.String(),
			DueAtMs: dueAt.UnixMilli(),
			Target:  &journalv1.JournalJob_Guild{Guild: tgt},
		}},
	}
}
