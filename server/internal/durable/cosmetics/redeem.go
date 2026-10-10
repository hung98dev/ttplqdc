package cosmetics

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/currency"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Redeem settles one 422 attempt: the client selects exactly one
// route per submission (cosmetics.md § Special-Currency Redemption).
// An already-owned entitlement is a SUCCESS no-op consuming nothing —
// acquiring through one route permanently silences the other
// (cosmetics.md §164-169). Every mutation commits atomically inside
// the caller's tx under the operation's stable idempotency key.
func (s *Store) Redeem(ctx context.Context, tx pgx.Tx, char id.UUID,
	cosmeticID string, route protocolv1.CosmeticRoute,
	opID id.UUID, now time.Time) error {
	def, ok := Lookup(cosmeticID)
	if !ok || def.Scope == ScopeGuild {
		return ErrUnknownCosmetic
	}
	r, ok := RoutesFor(cosmeticID)
	if !ok {
		return ErrNoRoute
	}
	acct, err := s.accountOf(ctx, tx, char)
	if err != nil {
		return err
	}
	owned, err := s.Owned(ctx, tx, acct, char, cosmeticID)
	if err != nil {
		return err
	}
	if owned {
		return nil // already-owned no-op: consume nothing
	}
	var sourceRef string
	switch route {
	case protocolv1.CosmeticRoute_COSMETIC_ROUTE_MATERIAL:
		if r.MaterialItem == "" {
			return ErrNoRoute
		}
		if err := consumeMaterial(ctx, tx, char, r.MaterialItem,
			r.MaterialQty); err != nil {
			return err
		}
		sourceRef = "redeem.material." + cosmeticID
	case protocolv1.CosmeticRoute_COSMETIC_ROUTE_CURRENCY_SPECIAL:
		if r.SpecialAmount <= 0 {
			return ErrNoRoute
		}
		if _, err := currency.Debit(ctx, tx, currency.Mutation{
			CharacterID: char, CurrencyID: currency.Special,
			Delta: r.SpecialAmount, OperationID: opID,
			ReasonCode: "COSMETIC_SPECIAL_SINK",
			SourceRef:  "redeem.special." + cosmeticID,
			Actor:      currency.ActorPlayer,
		}); err != nil {
			return fmt.Errorf("%w: %v", ErrInsufficientCurrency, err)
		}
		sourceRef = "redeem.special." + cosmeticID
	case protocolv1.CosmeticRoute_COSMETIC_ROUTE_CURRENCY_COMMON:
		if r.CommonAmount <= 0 {
			return ErrNoRoute
		}
		if _, err := currency.Debit(ctx, tx, currency.Mutation{
			CharacterID: char, CurrencyID: currency.Common,
			Delta: r.CommonAmount, OperationID: opID,
			ReasonCode: "COSMETIC_COMMON_SINK",
			SourceRef:  "common_cosmetic." + cosmeticID,
			Actor:      currency.ActorPlayer,
		}); err != nil {
			return fmt.Errorf("%w: %v", ErrInsufficientCurrency, err)
		}
		sourceRef = "common_cosmetic." + cosmeticID
	default:
		return ErrNoRoute
	}
	return s.Grant(ctx, tx, char, cosmeticID, SourceRedemption,
		sourceRef, nil, opID, now)
}

// consumeMaterial decrements owned CHARACTER_INVENTORY stacks of
// itemID in ascending slot order until qty is covered
// (durable/cooking's consume shape; failing consumes nothing because
// the tx rolls back).
func consumeMaterial(ctx context.Context, tx pgx.Tx, char id.UUID,
	itemID string, qty int64) error {
	rows, err := tx.Query(ctx, `
		SELECT ii.item_instance_id, ii.quantity
		FROM item_instances ii
		JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		WHERE il.character_id = $1 AND il.location_kind = 'CHARACTER_INVENTORY'
		  AND ii.item_id = $2
		ORDER BY il.slot
		FOR UPDATE OF ii`, char[:], itemID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type stack struct {
		inst id.UUID
		take int64
	}
	var sel []stack
	need := qty
	for rows.Next() {
		var inst id.UUID
		var have int64
		if err := rows.Scan(&inst, &have); err != nil {
			return err
		}
		take := have
		if need < take {
			take = need
		}
		sel = append(sel, stack{inst, take})
		need -= take
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	if need > 0 {
		return fmt.Errorf("%w: %s short %d", ErrInsufficientItem, itemID, need)
	}
	for _, c := range sel {
		var have int64
		if err := tx.QueryRow(ctx,
			`SELECT quantity FROM item_instances WHERE item_instance_id = $1`,
			c.inst[:]).Scan(&have); err != nil {
			return err
		}
		if have == c.take {
			if _, err := tx.Exec(ctx,
				`DELETE FROM item_locations WHERE item_instance_id = $1`, c.inst[:]); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`DELETE FROM item_instances WHERE item_instance_id = $1`, c.inst[:]); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(ctx,
			`UPDATE item_instances SET quantity = quantity - $2
			 WHERE item_instance_id = $1`, c.inst[:], c.take); err != nil {
			return err
		}
	}
	return nil
}
