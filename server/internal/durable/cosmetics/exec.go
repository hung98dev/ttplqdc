package cosmetics

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Durable family names (save_rules.md producer registry; router binds
// wire ids 422/424/656).
const (
	RedeemFamily     = "cosmetic.redeem"
	EquipFamily      = "cosmetic.equip"
	GuildEquipFamily = "client.656"
)

// Deps are the executor dependencies the composition root injects.
type Deps struct {
	Store *Store
	Now   func() time.Time
}

// Executors returns the family→executor map the composition
// ProducerClient family-mux merges (feature packages export, never
// self-register — durable/inventory precedent).
func Executors(d Deps) map[string]queue.Executor {
	return map[string]queue.Executor{
		RedeemFamily:     d.redeemExecutor,
		EquipFamily:      d.equipExecutor,
		GuildEquipFamily: d.guildEquipExecutor,
	}
}

func (d Deps) now() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now().UTC()
}

// identity validates the shared CHARACTER-owner record shape and
// returns (charID, opID).
func identity(rec *journalv1.DurableCommandRecord) (id.UUID, id.UUID, error) {
	cmd := rec.GetClient()
	if cmd == nil {
		return id.UUID{}, id.UUID{}, fmt.Errorf("cosmetics: record without client payload")
	}
	if len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 ||
		len(cmd.GetCharacterId()) != 16 {
		return id.UUID{}, id.UUID{}, ErrMalformedRecord
	}
	var charID, opID, owner id.UUID
	copy(charID[:], cmd.GetCharacterId())
	copy(opID[:], rec.GetOperationId())
	copy(owner[:], rec.GetOwnerId())
	if owner != charID {
		return id.UUID{}, id.UUID{}, fmt.Errorf("%w: owner %v vs character %v", ErrMalformedRecord, owner, charID)
	}
	return charID, opID, nil
}

func targetID(b []byte) (id.UUID, error) {
	var t id.UUID
	if len(b) != 16 {
		return t, ErrMalformedRecord
	}
	copy(t[:], b)
	return t, nil
}

// RedeemRecord builds the record for C2S_COSMETIC_REDEEM (1422).
func RedeemRecord(accountID id.UUID, sessionEpoch, ownershipEpoch uint64,
	characterID id.UUID, req *protocolv1.C2SCosmeticRedeem,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("cosmetics: redeem operation_id %d bytes", len(req.GetOperationId()))
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	fpr := id.OperationFingerprint(RedeemFamily, characterID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    RedeemFamily,
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
				Request: &journalv1.JournalClientCommand_C2SCosmeticRedeem{
					C2SCosmeticRedeem: req,
				},
			},
		},
	}, nil
}

// EquipRecord builds the record for C2S_COSMETIC_EQUIP (1424).
func EquipRecord(accountID id.UUID, sessionEpoch, ownershipEpoch uint64,
	characterID id.UUID, req *protocolv1.C2SCosmeticEquip,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("cosmetics: equip operation_id %d bytes", len(req.GetOperationId()))
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	fpr := id.OperationFingerprint(EquipFamily, characterID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    EquipFamily,
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
				Request: &journalv1.JournalClientCommand_C2SCosmeticEquip{
					C2SCosmeticEquip: req,
				},
			},
		},
	}, nil
}

// GuildEquipRecord builds the record for C2S_GUILD_COSMETIC_EQUIP
// (1656). The owner is still the acting character.
func GuildEquipRecord(accountID id.UUID, sessionEpoch,
	ownershipEpoch uint64, characterID id.UUID,
	req *protocolv1.C2SGuildCosmeticEquip,
	admittedAt time.Time) (*journalv1.DurableCommandRecord, error) {
	if len(req.GetOperationId()) != 16 {
		return nil, fmt.Errorf("cosmetics: guild equip operation_id %d bytes", len(req.GetOperationId()))
	}
	var opID id.UUID
	copy(opID[:], req.GetOperationId())
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	fpr := id.OperationFingerprint(GuildEquipFamily, characterID, opID, reqBytes)
	issuedMs, _ := id.OperationIssuedAt(opID)
	return &journalv1.DurableCommandRecord{
		SchemaVersion:      1,
		OperationFamily:    GuildEquipFamily,
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
				Request: &journalv1.JournalClientCommand_C2SGuildCosmeticEquip{
					C2SGuildCosmeticEquip: req,
				},
			},
		},
	}, nil
}

// ---------------------------------------------------------------------------
// Executors.

func (d Deps) redeemExecutor(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != RedeemFamily {
		return idempotency.Outcome{}, fmt.Errorf("cosmetics: unsupported family %q", rec.GetOperationFamily())
	}
	charID, opID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	req := rec.GetClient().GetC2SCosmeticRedeem()
	if req == nil {
		return idempotency.Outcome{}, fmt.Errorf("cosmetics: record without c2s_cosmetic_redeem")
	}
	if err := d.Store.Redeem(ctx, tx, charID, req.GetCosmeticId(),
		req.GetRoute(), opID, d.now()); err != nil {
		return redeemVerdict(opID, req.GetCosmeticId(), err)
	}
	return redeemCommit(opID, req.GetCosmeticId())
}

func (d Deps) equipExecutor(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != EquipFamily {
		return idempotency.Outcome{}, fmt.Errorf("cosmetics: unsupported family %q", rec.GetOperationFamily())
	}
	charID, opID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	req := rec.GetClient().GetC2SCosmeticEquip()
	if req == nil {
		return idempotency.Outcome{}, fmt.Errorf("cosmetics: record without c2s_cosmetic_equip")
	}
	if err := d.Store.Equip(ctx, tx, charID, req.GetSlot(),
		req.GetCosmeticId()); err != nil {
		return equipVerdict(opID, req.GetSlot(), req.GetCosmeticId(), err)
	}
	return equipCommit(opID, req.GetSlot(), req.GetCosmeticId())
}

func (d Deps) guildEquipExecutor(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != GuildEquipFamily {
		return idempotency.Outcome{}, fmt.Errorf("cosmetics: unsupported family %q", rec.GetOperationFamily())
	}
	charID, opID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	req := rec.GetClient().GetC2SGuildCosmeticEquip()
	if req == nil {
		return idempotency.Outcome{}, fmt.Errorf("cosmetics: record without c2s_guild_cosmetic_equip")
	}
	guildID, err := targetID(req.GetGuildId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	slot := guildSlotName(req.GetSlot())
	if slot == "" {
		return guildVerdict(opID, guildID, ErrInvalidSlot)
	}
	if err := d.Store.GuildEquip(ctx, tx, charID, guildID, slot,
		req.GetCosmeticId(), req.GetExpectedRevision()); err != nil {
		return guildVerdict(opID, guildID, err)
	}
	return guildCommit(opID, guildID)
}

func guildSlotName(s protocolv1.GuildCosmeticSlot) string {
	switch s {
	case protocolv1.GuildCosmeticSlot_GUILD_COSMETIC_SLOT_SHRINE:
		return "SHRINE"
	case protocolv1.GuildCosmeticSlot_GUILD_COSMETIC_SLOT_BANNER:
		return "BANNER"
	case protocolv1.GuildCosmeticSlot_GUILD_COSMETIC_SLOT_CREST:
		return "CREST"
	}
	return ""
}

// ---------------------------------------------------------------------------
// Outcomes — deterministic rejections commit the typed result inside
// the declared in-set; out-of-set codes leave client_result absent so
// the edge answers S2C_ERROR (durable/inventory precedent).

func redeemVerdict(opID id.UUID, cosmeticID string, err error) (idempotency.Outcome, error) {
	code := codeOf(err)
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
		ErrorCode:   code,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CCosmeticRedeemResult{
			S2CCosmeticRedeemResult: &protocolv1.S2CCosmeticRedeemResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
					ErrorCode:   code,
				},
				CosmeticId: cosmeticID,
			},
		},
	}
	return marshalOutcome(outcome)
}

func redeemCommit(opID id.UUID, cosmeticID string) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CCosmeticRedeemResult{
			S2CCosmeticRedeemResult: &protocolv1.S2CCosmeticRedeemResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
				},
				CosmeticId: cosmeticID,
			},
		},
	}
	return marshalOutcome(outcome)
}

func equipVerdict(opID id.UUID, slot protocolv1.CosmeticSlot,
	cosmeticID string, err error) (idempotency.Outcome, error) {
	code := codeOf(err)
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
		ErrorCode:   code,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CCosmeticEquipResult{
			S2CCosmeticEquipResult: &protocolv1.S2CCosmeticEquipResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
					ErrorCode:   code,
				},
				Slot:       slot,
				CosmeticId: cosmeticID,
			},
		},
	}
	return marshalOutcome(outcome)
}

func equipCommit(opID id.UUID, slot protocolv1.CosmeticSlot,
	cosmeticID string) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CCosmeticEquipResult{
			S2CCosmeticEquipResult: &protocolv1.S2CCosmeticEquipResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
				},
				Slot:       slot,
				CosmeticId: cosmeticID,
			},
		},
	}
	return marshalOutcome(outcome)
}

func guildVerdict(opID, guildID id.UUID, err error) (idempotency.Outcome, error) {
	code := codeOf(err)
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
		ErrorCode:   code,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CGuildResult{
			S2CGuildResult: &protocolv1.S2CGuildResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
					ErrorCode:   code,
				},
				RequestMessageId: 656,
				GuildId:          guildID[:],
			},
		},
	}
	return marshalOutcome(outcome)
}

func guildCommit(opID, guildID id.UUID) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CGuildResult{
			S2CGuildResult: &protocolv1.S2CGuildResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
				},
				RequestMessageId: 656,
				GuildId:          guildID[:],
			},
		},
	}
	return marshalOutcome(outcome)
}

func marshalOutcome(outcome *journalv1.JournalOutcome) (idempotency.Outcome, error) {
	b, err := protojson.Marshal(outcome)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: b}, nil
}
