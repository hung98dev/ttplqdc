package reward

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// Create commits one claim contribution inside the caller's settlement
// transaction. Ordering inside the tx (the character row lock serializes
// all claim creation for the owner, so the check-then-write probe is
// race-free):
//
//  1. character row lock.
//  2. contribution ledger probe — a replayed (source_reward_operation_id,
//     owner, reward_slot) returns the recorded claim with no mutation
//     (ADR-0065; "a duplicate contribution key changes nothing").
//  3. cap policy: preventable sources reject CLAIM_CAP_REACHED at
//     pending_count >= 100 without consuming anything; hard-ceiling
//     item/equipment rolls skip while currency still settles (ADR-0062).
//  4. consolidation merge or new claim + typed lines.
//  5. contribution row insert (uniqueness backstop), claim revision and
//     characters.claims_revision += 1 in the same tx.
func (s *Store) Create(ctx context.Context, tx pgx.Tx, in *Input) (*Created, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	if err := lockCharacter(ctx, tx, in.OwnerCharacterID); err != nil {
		return nil, err
	}
	at := s.now()
	if in.CreatedAtUnixMs != 0 {
		at = time.UnixMilli(in.CreatedAtUnixMs).UTC()
	}
	existing, err := s.contributionClaim(ctx, tx, in)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if existing != (id.UUID{}) {
		return &Created{ClaimID: existing, Inserted: false}, nil
	}
	pending, err := s.pendingCount(ctx, tx, in.OwnerCharacterID)
	if err != nil {
		return nil, err
	}
	if in.Preventable && pending >= softCap {
		return nil, ErrClaimCapReached
	}
	// Hard ceiling: item/equipment rolls skip (nothing earned, nothing
	// recorded) while currency lines still settle (ADR-0062).
	in, kept, skipped := filterAtCeiling(in, pending)
	if skipped > 0 && kept == 0 {
		return &Created{Skipped: true}, nil
	}

	kind := classifyKind(in, pending)
	key := consolidationKey(kind, &in.Lines[0], in)

	var target id.UUID
	if key != nil {
		if target, err = s.findConsolidatedForUpdate(ctx, tx,
			in.OwnerCharacterID, kind, *key); err != nil {
			return nil, err
		}
	}
	isNew := target == (id.UUID{})
	if isNew {
		target = id.NewV4()
		if err := s.insertClaim(ctx, tx, target, in, kind, key, at); err != nil {
			return nil, err
		}
	}

	total := new(big.Int)
	if !isNew && kind != ClaimKindSingle {
		// Aggregate/consolidated rows hold one line (line 0); merge the
		// contribution amount/quantity into it after the 10^38-1 check.
		l := &in.Lines[0]
		sum := l.Amount
		if kind == ClaimKindItemConsolidated {
			sum = l.Quantity
		}
		if err := s.addToLineZero(ctx, tx, target, kind, sum); err != nil {
			return nil, err
		}
		total.Add(total, sum)
	} else {
		lineNo := int16(0)
		if !isNew {
			if lineNo, err = s.nextLineNo(ctx, tx, target); err != nil {
				return nil, err
			}
		}
		for i := range in.Lines {
			if err := s.insertLine(ctx, tx, target, lineNo, &in.Lines[i]); err != nil {
				return nil, err
			}
			lineNo++
			if in.Lines[i].Kind == lineItem {
				total.Add(total, in.Lines[i].Quantity)
			} else {
				total.Add(total, in.Lines[i].Amount)
			}
		}
	}

	inserted, err := s.insertContribution(ctx, tx, in, target, total, at)
	if err != nil {
		return nil, err
	}
	if !inserted {
		return nil, ErrStateConflict
	}
	if err := s.bumpClaimRevision(ctx, tx, target); err != nil {
		return nil, err
	}
	if _, err := s.bumpClaimsRevision(ctx, tx, in.OwnerCharacterID); err != nil {
		return nil, err
	}
	return &Created{ClaimID: target, Kind: kind, Inserted: true}, nil
}
