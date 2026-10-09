package cooking

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/items"
)

// consumeFrozen decrements the exact JournalConsumedItem selection
// frozen at admission: each listed instance loses its recorded quantity
// and is deleted when it reaches zero (item_instances.quantity CHECK
// > 0). A missing/short/differently-owned instance fails the commit —
// retry resolves the original operation, never reselects inputs.
func consumeFrozen(ctx context.Context, tx pgx.Tx, charID id.UUID,
	sel []*journalConsumed) error {
	for _, c := range sel {
		var instID id.UUID
		copy(instID[:], c.itemInstanceID)
		var qty int64
		var owner id.UUID
		err := tx.QueryRow(ctx, `
			SELECT ii.quantity, il.character_id
			FROM item_instances ii
			JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
			WHERE ii.item_instance_id = $1 AND il.location_kind = 'CHARACTER_INVENTORY'
			FOR UPDATE OF ii`, instID).Scan(&qty, &owner)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrNotFound, instID)
		}
		if owner != charID || qty < int64(c.quantity) {
			return fmt.Errorf("%w: %s", ErrInsufficientItems, instID)
		}
		if qty == int64(c.quantity) {
			if _, err := tx.Exec(ctx,
				`DELETE FROM item_locations WHERE item_instance_id = $1`, instID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`DELETE FROM item_instances WHERE item_instance_id = $1`, instID); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.Exec(ctx,
			`UPDATE item_instances SET quantity = quantity - $2 WHERE item_instance_id = $1`,
			instID, c.quantity); err != nil {
			return err
		}
	}
	return nil
}

// journalConsumed is the internal frozen-consume shape used by the
// executor (decoded from the record's JournalConsumedItem rows).
type journalConsumed struct {
	itemInstanceID []byte
	itemID         string
	quantity       uint64
}

// grantItem places qty of itemID into the character inventory per
// inventory.md §21: partial compatible stacks top up first, then the
// lowest empty inv.<N> slot. The created JournalItem's frozen
// ItemInstanceId is used when a brand-new stack is created.
// What cannot be placed is returned as overflow for the caller's
// Reward-Claims path — the mutation itself is never partial here
// beyond the deterministic stack ordering.
func (d Deps) grantItem(ctx context.Context, tx pgx.Tx, charID id.UUID,
	item *journalv1ItemDef) (placed uint64, err error) {
	need := item.quantity
	// Top up compatible partial stacks in ascending slot order.
	rows, err := tx.Query(ctx, `
		SELECT ii.item_instance_id, ii.quantity
		FROM item_instances ii
		JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		WHERE il.character_id = $1 AND il.location_kind = 'CHARACTER_INVENTORY'
		  AND ii.item_id = $2 AND ii.effective_binding = $3 AND ii.quantity < $4
		ORDER BY il.slot`, charID, item.itemID, item.binding, item.maxStack)
	if err != nil {
		return 0, err
	}
	var tops []struct {
		instID id.UUID
		qty    int64
	}
	for rows.Next() {
		var instID id.UUID
		var qty int64
		if err := rows.Scan(&instID, &qty); err != nil {
			rows.Close()
			return 0, err
		}
		tops = append(tops, struct {
			instID id.UUID
			qty    int64
		}{instID, qty})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, tp := range tops {
		if need == 0 {
			break
		}
		room := item.maxStack - uint64(tp.qty)
		add := need
		if add > room {
			add = room
		}
		if _, err := tx.Exec(ctx,
			`UPDATE item_instances SET quantity = quantity + $2 WHERE item_instance_id = $1`,
			tp.instID, add); err != nil {
			return 0, err
		}
		need -= add
		placed += add
	}
	if need == 0 {
		return placed, nil
	}
	// New stack at the lowest free inv.<N> slot within capacity.
	slot, err := lowestFreeSlot(ctx, tx, charID, item.capacity)
	if err != nil {
		return placed, err
	}
	if slot < 0 {
		return placed, nil // overflow — caller routes to Reward Claims
	}
	var instID id.UUID
	copy(instID[:], item.instanceID)
	if instID == (id.UUID{}) {
		instID = id.NewV4()
	}
	if _, err := d.Items.Create(ctx, tx, item.def, items.CreateFields{
		InstanceID: instID,
		Quantity:   int(need),
	}, items.Location{
		Kind:        items.LocCharacterInventory,
		CharacterID: charID,
		Slot:        invSlot(slot),
	}); err != nil {
		return placed, err
	}
	return placed + need, nil
}

// lowestFreeSlot returns the smallest unused inv.<N> slot index, or -1
// when the character inventory is at capacity.
func lowestFreeSlot(ctx context.Context, tx pgx.Tx, charID id.UUID,
	capacity int64) (int64, error) {
	var cap2 int64
	if err := tx.QueryRow(ctx,
		`SELECT capacity FROM character_inventories WHERE character_id = $1`,
		charID).Scan(&cap2); err != nil {
		cap2 = capacity
	}
	if cap2 <= 0 {
		cap2 = capacity
	}
	var used []int64
	rows, err := tx.Query(ctx, `
		SELECT slot FROM item_locations
		WHERE character_id = $1 AND location_kind = 'CHARACTER_INVENTORY'`,
		charID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return 0, err
		}
		var n int64
		if _, err := fmt.Sscanf(s, "inv.%d", &n); err == nil {
			used = append(used, n)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	taken := make(map[int64]struct{}, len(used))
	for _, n := range used {
		taken[n] = struct{}{}
	}
	for n := int64(0); n < cap2; n++ {
		if _, ok := taken[n]; !ok {
			return n, nil
		}
	}
	return -1, nil
}

func invSlot(n int64) string { return fmt.Sprintf("inv.%d", n) }

// journalv1ItemDef carries one grant: the frozen JournalItem plus its
// resolved definition fields (stack limit, capacity source).
type journalv1ItemDef struct {
	itemID     string
	quantity   uint64
	binding    string
	instanceID []byte
	maxStack   uint64
	capacity   int64
	def        items.Def
}
