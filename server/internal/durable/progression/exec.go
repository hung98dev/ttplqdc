package progression

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/currency"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Executors returns the queue.Executor surface of this package: one
// executor per durable family, exported only — composition installs them
// on the ProducerClient family-mux; the package never self-registers
// (ADR-0081). ProducerClient records run inside Store.TrustedReplay
// under the receipt lock, so every verdict/commit below is a committed
// JournalOutcome: a retried operation_id replays the retained outcome
// with no re-execution.
func Executors(s *Store) map[string]queue.Executor {
	return map[string]queue.Executor{
		SkillUpgradeFamily: s.skillUpgrade,
		AllocateFamily:     s.allocate,
		RespecFamily:       s.respec,
	}
}

// skillInfoFor validates the request skill_id against the character's
// own class catalog — a cross-class or unknown id is not learnable.
func skillInfoFor(classID, skillID string) (skillInfo, bool) {
	info, ok := skillFacts[skillID]
	if !ok {
		return skillInfo{}, false
	}
	element, ok := classElements[classID]
	if !ok || !strings.HasPrefix(skillID, "skill."+element+".") {
		return skillInfo{}, false
	}
	return info, true
}

// respecFreeLevel and the price formula mirror progression.md § Respec:
// free at ≤ Lv20, 25·level² above.
const respecFreeLevel int32 = 20

func respecPrice(level int32) int64 {
	if level <= respecFreeLevel {
		return 0
	}
	l := int64(level)
	return 25 * l * l
}

// potentialCap mirrors stats.md § Potential Stats: floor(0.6·earned).
func potentialCap(earned int32) int32 {
	if earned < 0 {
		return 0
	}
	return earned * 6 / 10
}

// identity validates the record's character-owned identity fields.
func identity(rec *journalv1.DurableCommandRecord) (opID, characterID id.UUID, err error) {
	if len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 {
		return opID, characterID, ErrMalformedRecord
	}
	copy(opID[:], rec.GetOperationId())
	copy(characterID[:], rec.GetOwnerId())
	return opID, characterID, nil
}

// cmdIdentity cross-checks the JournalClientCommand identity against the
// record owner: a mismatched account/character is a malformed record.
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

// skillUpgrade applies C2S_SKILL_UPGRADE (511): +1 level to a learned
// skill for 1 unspent point. Check order mirrors sim: learned →
// STATE_CONFLICT (expected_level) → max → insufficient.
func (s *Store) skillUpgrade(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != SkillUpgradeFamily {
		return idempotency.Outcome{}, fmt.Errorf("%w: family %q", ErrMalformedRecord, rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	opID, characterID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if err := cmdIdentity(cmd, characterID); err != nil {
		return idempotency.Outcome{}, err
	}
	req := cmd.GetC2SSkillUpgrade()
	if req == nil || req.GetSkillId() == "" {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	fam := rec.GetOperationFamily()

	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", characterID)); err != nil {
		return idempotency.Outcome{}, err
	}
	p, err := s.Read(ctx, tx, characterID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	info, ok := skillInfoFor(p.ClassID, req.GetSkillId())
	if !ok {
		return s.verdict(opID, fam, protocolv1.ErrorCode_ERROR_CODE_SKILL_NOT_LEARNED)
	}
	rowLvl, persisted := p.Skills[req.GetSkillId()]
	lvl, learned := learnedLevel(rowLvl, persisted, p.Level, info)
	if !learned {
		return s.verdict(opID, fam, protocolv1.ErrorCode_ERROR_CODE_SKILL_NOT_LEARNED)
	}
	if req.GetExpectedLevel() != 0 && req.GetExpectedLevel() != uint32(lvl) {
		return s.verdict(opID, fam, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT)
	}
	if lvl >= info.maxLvl {
		return s.verdict(opID, fam, protocolv1.ErrorCode_ERROR_CODE_SKILL_MAX_LEVEL)
	}
	if p.UnspentSkillPoints < 1 {
		return s.verdict(opID, fam, protocolv1.ErrorCode_ERROR_CODE_SKILL_POINTS_INSUFFICIENT)
	}
	if err := s.SetSkillLevel(ctx, tx, characterID, req.GetSkillId(), lvl+1); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := s.Write(ctx, tx, characterID, p.Level, p.CurrentExp,
		p.UnspentSkillPoints-1, p.UnspentPotentialPoints); err != nil {
		return idempotency.Outcome{}, err
	}
	return s.commit(opID, fam)
}

// allocate applies C2S_POTENTIAL_ALLOCATE (512): all-or-nothing deltas
// into the four potential stats under the 60%-of-earned per-stat cap.
func (s *Store) allocate(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != AllocateFamily {
		return idempotency.Outcome{}, fmt.Errorf("%w: family %q", ErrMalformedRecord, rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	opID, characterID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if err := cmdIdentity(cmd, characterID); err != nil {
		return idempotency.Outcome{}, err
	}
	req := cmd.GetC2SPotentialAllocate()
	d := req.GetDeltas()
	if req == nil || d == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	fam := rec.GetOperationFamily()
	deltas := [4]int64{int64(d.GetStr()), int64(d.GetVit()), int64(d.GetInt()), int64(d.GetAgi())}
	var sum int64
	for _, v := range deltas {
		sum += v
	}
	if sum < 1 {
		return s.verdict(opID, fam, protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED)
	}

	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", characterID)); err != nil {
		return idempotency.Outcome{}, err
	}
	p, err := s.Read(ctx, tx, characterID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if sum > int64(p.UnspentPotentialPoints) {
		return s.verdict(opID, fam, protocolv1.ErrorCode_ERROR_CODE_POTENTIAL_POINTS_INSUFFICIENT)
	}
	cap := potentialCap(p.EarnedPotentialTotal())
	for i, v := range deltas {
		if int64(p.Allocated[i])+v > int64(cap) {
			return s.verdict(opID, fam, protocolv1.ErrorCode_ERROR_CODE_POTENTIAL_CAP_EXCEEDED)
		}
	}
	for i, v := range deltas {
		if v == 0 {
			continue
		}
		if err := s.SetAllocation(ctx, tx, characterID, PotentialIds[i],
			p.Allocated[i]+int32(v)); err != nil {
			return idempotency.Outcome{}, err
		}
	}
	if err := s.Write(ctx, tx, characterID, p.Level, p.CurrentExp,
		p.UnspentSkillPoints, p.UnspentPotentialPoints-int32(sum)); err != nil {
		return idempotency.Outcome{}, err
	}
	return s.commit(opID, fam)
}

// respec applies C2S_RESPEC (513): charge and refund commit in one tx —
// a balance shortfall rejects with no change (INSUFFICIENT_CURRENCY).
// The NPC admission check ran at the edge; durable sees the committed
// evidence (spatial_source) and the kind.
func (s *Store) respec(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != RespecFamily {
		return idempotency.Outcome{}, fmt.Errorf("%w: family %q", ErrMalformedRecord, rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	opID, characterID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if err := cmdIdentity(cmd, characterID); err != nil {
		return idempotency.Outcome{}, err
	}
	req := cmd.GetC2SRespec()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	fam := rec.GetOperationFamily()
	kind := req.GetKind()
	if kind != protocolv1.RespecKind_RESPEC_KIND_SKILL &&
		kind != protocolv1.RespecKind_RESPEC_KIND_POTENTIAL {
		return s.verdict(opID, fam, protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED)
	}

	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", characterID)); err != nil {
		return idempotency.Outcome{}, err
	}
	p, err := s.Read(ctx, tx, characterID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	price := respecPrice(p.Level)
	if price > 0 {
		// Debit inside this tx: ErrInsufficientBalance lands before any
		// write, so the verdict commits cleanly with no savepoint.
		_, err := currency.Debit(ctx, tx, currency.Mutation{
			CharacterID: characterID,
			CurrencyID:  currency.Common,
			Delta:       price,
			OperationID: opID,
			ReasonCode:  "NPC_SERVICE",
			SourceRef:   "progression.respec:" + req.GetNpcId(),
			Actor:       currency.ActorPlayer,
		})
		if errors.Is(err, currency.ErrInsufficientBalance) {
			return s.verdict(opID, fam, protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_CURRENCY)
		}
		if err != nil {
			return idempotency.Outcome{}, err
		}
	}

	switch kind {
	case protocolv1.RespecKind_RESPEC_KIND_SKILL:
		for skillID, lvl := range p.Skills {
			if lvl == 1 {
				continue
			}
			if err := s.SetSkillLevel(ctx, tx, characterID, skillID, 1); err != nil {
				return idempotency.Outcome{}, err
			}
		}
		p.UnspentSkillPoints += p.SpentSkillPoints()
	case protocolv1.RespecKind_RESPEC_KIND_POTENTIAL:
		for i, v := range p.Allocated {
			if v == 0 {
				continue
			}
			if err := s.SetAllocation(ctx, tx, characterID, PotentialIds[i], 0); err != nil {
				return idempotency.Outcome{}, err
			}
		}
		var refunded int32
		for _, v := range p.Allocated {
			refunded += v
		}
		p.UnspentPotentialPoints += refunded
	}
	if err := s.Write(ctx, tx, characterID, p.Level, p.CurrentExp,
		p.UnspentSkillPoints, p.UnspentPotentialPoints); err != nil {
		return idempotency.Outcome{}, err
	}
	return s.commit(opID, fam)
}

// verdict writes a committed terminal nonexecution carrying the wire
// code: the 514 client_result embeds the OperationResult (status ERROR)
// and the C2S request id it answers.
func (s *Store) verdict(opID id.UUID, family string, code protocolv1.ErrorCode) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
		ErrorCode:   code,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CProgressionMutateResult{
			S2CProgressionMutateResult: &protocolv1.S2CProgressionMutateResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
					ErrorCode:   code,
				},
				RequestMessageId: requestMessageID[family],
			},
		},
	}
	return marshalOutcome(outcome)
}

// commit writes the successful terminal outcome carrying the 514 result.
func (s *Store) commit(opID id.UUID, family string) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CProgressionMutateResult{
			S2CProgressionMutateResult: &protocolv1.S2CProgressionMutateResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
				},
				RequestMessageId: requestMessageID[family],
			},
		},
	}
	return marshalOutcome(outcome)
}

// marshalOutcome serializes the schema-v1 JournalOutcome as protojson —
// the retained representation AwaitClientOutcome decodes.
func marshalOutcome(outcome *journalv1.JournalOutcome) (idempotency.Outcome, error) {
	b, err := protojson.Marshal(outcome)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: b}, nil
}
