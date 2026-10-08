package reward

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/lockorder"
)

// Store is the reward_claims projection surface. Every claims-mutating
// statement runs under the caller's transaction while holding the
// character's row lock; each commits with `characters.claims_revision`
// exactly +1 (data_model.md § Character — never SUM-derivable).
type Store struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// Option configures a Store.
type Option func(*Store)

// WithClock overrides the wall-clock source (tests).
func WithClock(now func() time.Time) Option {
	return func(s *Store) { s.now = now }
}

// NewStore builds a Store over pool.
func NewStore(pool *pgxpool.Pool, opts ...Option) *Store {
	s := &Store{pool: pool, now: func() time.Time { return time.Now().UTC() }}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Claim is one reward_claims row plus its typed lines.
type Claim struct {
	ClaimID          id.UUID
	OwnerCharacterID id.UUID
	SourceType       string
	SourceReference  string
	RewardSlot       string
	Kind             string
	ConsolidationKey *string
	State            string
	CreatedAtMs      int64
	ExpiresAtMs      int64 // 0 = no expiry
	ClaimOperationID *id.UUID
	Revision         int64
	Lines            []*Line
}

// Line is one reward_claim_lines row.
type Line struct {
	claimID           id.UUID // parent claim (set at load)
	LineNo            int16
	Kind              string
	ItemID            string
	Quantity          *big.Int
	EffectiveBinding  string
	EnhancementLevel  int32
	ItemState         []byte
	ContentRevision   string
	CurrencyID        string
	Amount            *big.Int
	DeliveredQuantity *big.Int
	DeliveredAmount   *big.Int
}

// Remaining is the exact undelivered quantity/amount (derived, never
// stored: reward_claims.md § Delivery).
func (l *Line) Remaining() *big.Int {
	if l.Kind == lineItem {
		return new(big.Int).Sub(l.Quantity, l.DeliveredQuantity)
	}
	return new(big.Int).Sub(l.Amount, l.DeliveredAmount)
}

// lockCharacter takes the canonical claims-aggregate row lock.
func lockCharacter(ctx context.Context, tx pgx.Tx, characterID id.UUID) error {
	return lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", characterID))
}

// bumpClaimsRevision commits characters.claims_revision += 1 and returns
// the post-increment value (exactly +1 per committed claims-mutating
// transaction — data_model.md § Character).
func (s *Store) bumpClaimsRevision(ctx context.Context, tx pgx.Tx, characterID id.UUID) (int64, error) {
	var rev int64
	err := tx.QueryRow(ctx,
		`UPDATE characters SET claims_revision = claims_revision + 1, updated_at = $2
		 WHERE character_id = $1 RETURNING claims_revision`,
		characterID, s.now()).Scan(&rev)
	return rev, err
}

// ClaimsRevision reads characters.claims_revision (callers hold the
// character lock for a consistent read).
func (s *Store) ClaimsRevision(ctx context.Context, tx pgx.Tx, characterID id.UUID) (int64, error) {
	var rev int64
	err := tx.QueryRow(ctx,
		`SELECT claims_revision FROM characters WHERE character_id = $1`, characterID).Scan(&rev)
	return rev, err
}

// pendingCount is ADR-0062's gate: PENDING non-aggregate claims plus
// aggregate currency claims (one each regardless of contribution count).
func (s *Store) pendingCount(ctx context.Context, tx pgx.Tx, characterID id.UUID) (int64, error) {
	var n int64
	err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM reward_claims
		 WHERE owner_character_id = $1 AND state = 'PENDING'
		   AND (claim_kind <> 'ITEM_CONSOLIDATED')`,
		characterID).Scan(&n)
	return n, err
}

// PendingCount exposes the ADR-0062 pending_count for producer-side
// gates (preventable sources check at action start; hard-ceiling roll
// skips consult it before rolling).
func (s *Store) PendingCount(ctx context.Context, tx pgx.Tx, characterID id.UUID) (int64, error) {
	return s.pendingCount(ctx, tx, characterID)
}

// totalPending counts every PENDING claim (434/439/440 total_count).
func (s *Store) totalPending(ctx context.Context, tx pgx.Tx, characterID id.UUID) (int64, error) {
	var n int64
	err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM reward_claims
		 WHERE owner_character_id = $1 AND state = 'PENDING'`, characterID).Scan(&n)
	return n, err
}

// loadClaimForUpdate reads one claim and its lines under a row lock.
func (s *Store) loadClaimForUpdate(ctx context.Context, tx pgx.Tx, claimID id.UUID) (*Claim, error) {
	c := &Claim{ClaimID: claimID}
	var ckey *string
	var opID *string
	var createdAt, updatedAt time.Time
	var claimedAt *time.Time
	err := tx.QueryRow(ctx,
		`SELECT owner_character_id, source_type, source_reference, reward_slot,
		        claim_kind, consolidation_key, state, created_at, updated_at,
		        claimed_at, claim_operation_id, revision
		 FROM reward_claims WHERE reward_claim_id = $1 FOR UPDATE`,
		claimID).Scan(&c.OwnerCharacterID, &c.SourceType, &c.SourceReference,
		&c.RewardSlot, &c.Kind, &ckey, &c.State, &createdAt, &updatedAt,
		&claimedAt, &opID, &c.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrClaimNotFound
	}
	if err != nil {
		return nil, err
	}
	c.ConsolidationKey = ckey
	c.CreatedAtMs = createdAt.UnixMilli()
	if opID != nil {
		o, err := id.ParseUUID(*opID)
		if err != nil {
			return nil, fmt.Errorf("reward: claim_operation_id %q: %w", *opID, err)
		}
		c.ClaimOperationID = &o
	}
	if err := s.loadLines(ctx, tx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// loadLines reads a claim's typed lines in line_no order (exact
// NUMERIC(38,0) totals project as text then parse as big.Int).
func (s *Store) loadLines(ctx context.Context, tx pgx.Tx, c *Claim) error {
	rows, err := tx.Query(ctx,
		`SELECT line_no, line_kind, item_id, quantity::text, effective_binding,
		        item_state, content_revision, currency_id, amount::text,
		        delivered_quantity::text, delivered_amount::text
		 FROM reward_claim_lines WHERE reward_claim_id = $1 ORDER BY line_no`,
		c.ClaimID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		l := &Line{claimID: c.ClaimID}
		var itemID, binding, rev, currencyID *string
		var qty, amt, dQty, dAmt *string
		if err := rows.Scan(&l.LineNo, &l.Kind, &itemID, &qty, &binding,
			&l.ItemState, &rev, &currencyID, &amt, &dQty, &dAmt); err != nil {
			return err
		}
		if itemID != nil {
			l.ItemID = *itemID
		}
		if binding != nil {
			l.EffectiveBinding = *binding
		}
		if rev != nil {
			l.ContentRevision = *rev
		}
		if currencyID != nil {
			l.CurrencyID = *currencyID
		}
		l.Quantity = new(big.Int)
		l.Amount = new(big.Int)
		l.DeliveredQuantity = new(big.Int)
		l.DeliveredAmount = new(big.Int)
		if qty != nil {
			l.Quantity = dec(*qty)
		}
		if amt != nil {
			l.Amount = dec(*amt)
		}
		if dQty != nil {
			l.DeliveredQuantity = dec(*dQty)
		}
		if dAmt != nil {
			l.DeliveredAmount = dec(*dAmt)
		}
		c.Lines = append(c.Lines, l)
	}
	return rows.Err()
}

// insertClaim commits one reward_claims row.
func (s *Store) insertClaim(ctx context.Context, tx pgx.Tx, claimID id.UUID,
	in *Input, kind string, key *string, at time.Time) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO reward_claims
		  (reward_claim_id, owner_character_id, source_type, source_reference,
		   reward_slot, claim_kind, consolidation_key, state, created_at, updated_at, revision)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,'PENDING',$8,$8,0)`,
		claimID, in.OwnerCharacterID, in.SourceType, in.SourceReference,
		in.RewardSlot, kind, key, at)
	return err
}

// insertLine commits one reward_claim_lines row.
func (s *Store) insertLine(ctx context.Context, tx pgx.Tx, claimID id.UUID,
	lineNo int16, l *LineInput) error {
	switch l.Kind {
	case lineItem:
		state := l.ItemState
		if len(state) == 0 {
			state = []byte("{}")
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO reward_claim_lines
			  (reward_claim_id, line_no, line_kind, item_id, quantity,
			   effective_binding, item_state, content_revision)
			 VALUES ($1,$2,'ITEM',$3,$4::numeric,$5,$6,$7)`,
			claimID, lineNo, l.ItemID, decString(l.Quantity),
			l.EffectiveBinding, state, l.ContentRevision)
		return err
	case lineCurrency:
		_, err := tx.Exec(ctx,
			`INSERT INTO reward_claim_lines
			  (reward_claim_id, line_no, line_kind, currency_id, amount)
			 VALUES ($1,$2,'CURRENCY',$3,$4::numeric)`,
			claimID, lineNo, l.CurrencyID, decString(l.Amount))
		return err
	}
	return fmt.Errorf("%w: line kind %q", ErrMalformed, l.Kind)
}

// insertContribution appends the idempotency-ledger row. It reports
// false when the (source_reward_operation_id, owner, reward_slot) key
// already exists — a replayed contribution mutates nothing
// (reward_claims.md § Currency Overflow, ADR-0065).
func (s *Store) insertContribution(ctx context.Context, tx pgx.Tx,
	in *Input, claimID id.UUID, total *big.Int, at time.Time) (bool, error) {
	tag, err := tx.Exec(ctx,
		`INSERT INTO reward_claim_contributions
		  (source_reward_operation_id, owner_character_id, reward_slot,
		   reward_claim_id, quantity_or_amount, created_at)
		 VALUES ($1,$2,$3,$4,$5::numeric,$6)
		 ON CONFLICT (source_reward_operation_id, owner_character_id, reward_slot) DO NOTHING`,
		in.SourceRewardOperationID, in.OwnerCharacterID, in.RewardSlot,
		claimID, decString(total), at)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// contributionClaim resolves the claim a contribution key already
// committed to (dedup replay path).
func (s *Store) contributionClaim(ctx context.Context, tx pgx.Tx,
	in *Input) (id.UUID, error) {
	var claimID id.UUID
	err := tx.QueryRow(ctx,
		`SELECT reward_claim_id FROM reward_claim_contributions
		 WHERE source_reward_operation_id = $1 AND owner_character_id = $2
		   AND reward_slot = $3`,
		in.SourceRewardOperationID, in.OwnerCharacterID, in.RewardSlot).Scan(&claimID)
	return claimID, err
}

// findConsolidatedForUpdate returns the PENDING consolidated claim for
// (owner, kind, key), row-locked — the unique partial index guarantees
// at most one (ADR-0063).
func (s *Store) findConsolidatedForUpdate(ctx context.Context, tx pgx.Tx,
	characterID id.UUID, kind, key string) (id.UUID, error) {
	var claimID id.UUID
	err := tx.QueryRow(ctx,
		`SELECT reward_claim_id FROM reward_claims
		 WHERE owner_character_id = $1 AND claim_kind = $2 AND consolidation_key = $3
		   AND state = 'PENDING' FOR UPDATE`,
		characterID, kind, key).Scan(&claimID)
	if errors.Is(err, pgx.ErrNoRows) {
		return id.UUID{}, nil
	}
	return claimID, err
}

// addToLineZero merges a contribution into the aggregate's single line
// (line_no 0). The merged total is validated inside the held locks: an
// unrepresentable sum (> 10^38-1) fails closed before the source is
// marked settled — never wraps, clamps or deletes (§ Capacity/Abuse).
func (s *Store) addToLineZero(ctx context.Context, tx pgx.Tx, claimID id.UUID,
	kind string, add *big.Int) error {
	col := "amount"
	if kind == ClaimKindItemConsolidated {
		col = "quantity"
	}
	var cur string
	if err := tx.QueryRow(ctx,
		`SELECT `+col+`::text FROM reward_claim_lines
		 WHERE reward_claim_id = $1 AND line_no = 0 FOR UPDATE`,
		claimID).Scan(&cur); err != nil {
		return err
	}
	sum := dec(cur)
	sum.Add(sum, add)
	if sum.Sign() < 0 || sum.Cmp(dec38Limit) > 0 {
		return ErrMalformed
	}
	_, err := tx.Exec(ctx,
		`UPDATE reward_claim_lines SET `+col+` = $2::numeric
		 WHERE reward_claim_id = $1 AND line_no = 0`,
		claimID, decString(sum))
	return err
}

// bumpClaimRevision marks the claim row mutated (per-claim revision).
func (s *Store) bumpClaimRevision(ctx context.Context, tx pgx.Tx, claimID id.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE reward_claims SET revision = revision + 1, updated_at = $2
		 WHERE reward_claim_id = $1`, claimID, s.now())
	return err
}

// nextLineNo returns the next free line_no under the claim row lock.
func (s *Store) nextLineNo(ctx context.Context, tx pgx.Tx, claimID id.UUID) (int16, error) {
	var n int16
	err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(line_no) + 1, 0) FROM reward_claim_lines
		 WHERE reward_claim_id = $1`, claimID).Scan(&n)
	return n, err
}

// addDeliveredQuantity adds a batch to an ITEM line's delivered counter.
func (s *Store) addDeliveredQuantity(ctx context.Context, tx pgx.Tx, claimID id.UUID,
	lineNo int16, qty *big.Int) error {
	_, err := tx.Exec(ctx,
		`UPDATE reward_claim_lines SET delivered_quantity = delivered_quantity + $1::numeric
		 WHERE reward_claim_id = $2 AND line_no = $3 AND line_kind = 'ITEM'`,
		decString(qty), claimID, lineNo)
	return err
}

// addDeliveredAmount adds a batch to a CURRENCY line's delivered counter.
func (s *Store) addDeliveredAmount(ctx context.Context, tx pgx.Tx, claimID id.UUID,
	lineNo int16, amt *big.Int) error {
	_, err := tx.Exec(ctx,
		`UPDATE reward_claim_lines SET delivered_amount = delivered_amount + $1::numeric
		 WHERE reward_claim_id = $2 AND line_no = $3 AND line_kind = 'CURRENCY'`,
		decString(amt), claimID, lineNo)
	return err
}

// markDelivered stamps the committed batch on the claim row: full
// delivery transitions to CLAIMED with claimed_at + claim_operation_id;
// a remainder stays PENDING with the operation id recorded.
func (s *Store) markDelivered(ctx context.Context, tx pgx.Tx, c *Claim,
	opID id.UUID, claimed bool) error {
	if claimed {
		_, err := tx.Exec(ctx,
			`UPDATE reward_claims SET state = 'CLAIMED', claimed_at = $2,
			 claim_operation_id = $3, revision = revision + 1, updated_at = $2
			 WHERE reward_claim_id = $1`,
			c.ClaimID, s.now(), opID)
		return err
	}
	_, err := tx.Exec(ctx,
		`UPDATE reward_claims SET claim_operation_id = $2,
		 revision = revision + 1, updated_at = $3
		 WHERE reward_claim_id = $1`,
		c.ClaimID, opID, s.now())
	return err
}
