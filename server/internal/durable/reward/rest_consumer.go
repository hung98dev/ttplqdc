package reward

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	"thinhthan/internal/durable/progression"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// RestKind is the JournalRewardCommand.kind produced by
// sim.rest_settlement for each bonfire-rest tick intent
// (save_rules.md § Closed Durable Queue Producer Registry).
const RestKind = "rest"

// restDailyCap is the world_rules.md § Passive Rest EXP daily bound:
// at most 180 granted ticks per (character_id, utc_date) — declared in
// types.go.

// expCap is the progression.md level-60 ceiling on current_exp.
const expCap = 702100000

// RestExecutor returns the queue.Executor for kind "rest"; the
// composition registers it under ProducerReward via KindMux. Each
// committed tick grants the slot's character_exp and bumps
// character_rest_daily.rest_ticks_gained atomically under the
// character lock; a tick at the daily cap commits the journal row but
// grants nothing (reward_claims.md packet: enforced inside the
// settlement tx, never by skipping the journal row).
func RestExecutor(s *Store, prog *progression.Store) queue.Executor {
	return func(ctx context.Context, tx pgx.Tx, rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
		rw := rec.GetReward()
		if rw == nil || rw.GetKind() != RestKind {
			return idempotency.Outcome{}, fmt.Errorf("reward: rest executor saw kind %q", rw.GetKind())
		}
		charID, err := rewardOwner(rec)
		if err != nil {
			return idempotency.Outcome{}, err
		}
		if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", charID)); err != nil {
			return idempotency.Outcome{}, err
		}
		var granted uint64
		day := s.now().UTC().Truncate(24 * time.Hour)
		for _, slot := range rw.GetSlots() {
			ok, err := s.takeRestTick(ctx, tx, charID, day)
			if err != nil {
				return idempotency.Outcome{}, err
			}
			if !ok {
				continue // 180/day cap reached inside the tx — grant nothing.
			}
			exp := slot.GetCharacterExp()
			if exp == 0 {
				continue
			}
			if err := applyCharacterExp(ctx, tx, s, prog, charID, exp); err != nil {
				return idempotency.Outcome{}, err
			}
			granted += exp
		}
		outcome := &journalv1.JournalOutcome{
			Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
			OperationId: rec.GetOperationId(),
		}
		_ = granted
		return marshalOutcome(outcome)
	}
}

// takeRestTick consumes one daily tick under the character lock:
// atomic insert-or-increment bounded by the 180/day CHECK.
func (s *Store) takeRestTick(ctx context.Context, tx pgx.Tx, charID id.UUID, day time.Time) (bool, error) {
	var n int16
	err := tx.QueryRow(ctx,
		`INSERT INTO character_rest_daily (character_id, utc_date, rest_ticks_gained)
		 VALUES ($1, $2, 1)
		 ON CONFLICT (character_id, utc_date)
		 DO UPDATE SET rest_ticks_gained = character_rest_daily.rest_ticks_gained + 1
		 WHERE character_rest_daily.rest_ticks_gained < $3
		 RETURNING rest_ticks_gained`,
		charID.String(), day, restDailyCap).Scan(&n)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil // cap reached — no row mutation.
		}
		return false, err
	}
	return true, nil
}

// applyCharacterExp adds EXP under the character lock, derives the new
// level per progression.md § Level Derivation, applies the per-level
// gains (+1 skill point, +4 potential points; multiple level-ups in one
// grant process all intermediate rewards in this tx), and caps at the
// level-60 EXP ceiling.
func applyCharacterExp(ctx context.Context, tx pgx.Tx, s *Store, prog *progression.Store,
	charID id.UUID, exp uint64) error {
	p, err := prog.Read(ctx, tx, charID)
	if err != nil {
		return err
	}
	newExp := int64(p.CurrentExp) + int64(exp) // exp <= 1<<63 bounded by spec
	if newExp > expCap {
		newExp = expCap
	}
	newLevel := levelForExp(newExp)
	skillGain := int32(0)
	potGain := int32(0)
	if newLevel > p.Level {
		skillGain = newLevel - p.Level
		potGain = 4 * (newLevel - p.Level)
	}
	return prog.Write(ctx, tx, charID,
		newLevel, int32(newExp),
		p.UnspentSkillPoints+skillGain,
		p.UnspentPotentialPoints+potGain)
}

// levelForExp mirrors progression.md: L = max{k in [1..60] |
// cumulative_exp_to_reach(k) <= exp}, cumulative_exp_to_reach(k) =
// sum(10000 * i * i for i = 1..k-1).
func levelForExp(exp int64) int32 {
	level := int32(1)
	cum := int64(0)
	for k := int32(2); k <= 60; k++ {
		cum += 10000 * int64(k-1) * int64(k-1)
		if exp < cum {
			break
		}
		level = k
	}
	return level
}

// rewardOwner extracts the character id from a producer record.
func rewardOwner(rec *journalv1.DurableCommandRecord) (id.UUID, error) {
	if len(rec.GetOwnerId()) != 16 ||
		rec.GetOwnerKind() != journalv1.JournalOwnerKind_JOURNAL_OWNER_KIND_CHARACTER {
		return id.UUID{}, fmt.Errorf("reward: malformed producer record owner")
	}
	var c id.UUID
	copy(c[:], rec.GetOwnerId())
	return c, nil
}
