package cooking

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	journalv1 "thinhthan/internal/durable/journal/v1"
)

// PlanCook resolves a COOK admission into the frozen
// JournalCraftSnapshot carried inside the durable record
// (protobuf_conventions.md §7): recipe at record revision, the material
// instances selected for consumption (lowest inventory slot first),
// every created output with its assigned instance UUID, and the
// authored LIFE_SKILL character EXP at the character's captured act.
// It performs a read-only check of ingredient availability and output
// capacity; the executor repeats them transactionally.
func (d Deps) PlanCook(ctx context.Context, tx pgx.Tx, charID id.UUID,
	recipeID string) (*journalv1.JournalCraftSnapshot, error) {
	if d.Recipes == nil {
		return nil, fmt.Errorf("cooking: nil recipe resolver")
	}
	rec, err := d.Recipes(ctx, recipeID)
	if err != nil {
		return nil, err
	}
	snap := &journalv1.JournalCraftSnapshot{
		RecipeId:      recipeID,
		BatchQuantity: 1,
	}
	// Frozen input selection: lowest-slot compatible stacks first, so
	// replay consumes the identical instances.
	for _, in := range rec.Inputs {
		sel, err := selectConsume(ctx, tx, charID, in.ItemID, in.Quantity)
		if err != nil {
			return nil, err
		}
		snap.Consumed = append(snap.Consumed, sel...)
	}
	// Frozen outputs: dish + guaranteed extra_output, each an assigned
	// instance UUID fixed before the first settlement attempt.
	out, err := d.frozenOutput(ctx, rec.OutputItemID, 1)
	if err != nil {
		return nil, err
	}
	extra, err := d.frozenOutput(ctx, rec.ExtraItemID, 1)
	if err != nil {
		return nil, err
	}
	snap.CreatedItems = append(snap.CreatedItems, out, extra)
	// Captured act from the character row -> authored per-act EXP.
	act, err := characterAct(ctx, tx, charID)
	if err != nil {
		return nil, err
	}
	snap.CharacterExp = rec.LifeSkillExp(act)
	return snap, nil
}

// frozenOutput builds one created JournalItem with its assigned
// instance UUID, effective binding and content revision frozen now.
func (d Deps) frozenOutput(ctx context.Context, itemID string,
	qty uint64) (*journalv1.JournalItem, error) {
	def, err := d.Defs(ctx, itemID)
	if err != nil {
		return nil, err
	}
	inst := id.NewV4()
	j := &journalv1.JournalItem{
		ItemId:           itemID,
		Quantity:         qty,
		EffectiveBinding: def.Def.DefaultBinding.String(),
		ItemInstanceId:   inst[:],
	}
	if d.ContentRevision != nil {
		j.ContentRevision = *d.ContentRevision
	}
	return j, nil
}

// selectConsume picks owned CHARACTER_INVENTORY stacks of itemID in
// ascending slot order until qty is covered (world_rules.md / §7: the
// frozen consumed set uniquely identifies the owned stacks selected
// before admission). ErrInsufficientItems when the total falls short.
func selectConsume(ctx context.Context, tx pgx.Tx, charID id.UUID,
	itemID string, qty uint64) ([]*journalv1.JournalConsumedItem, error) {
	rows, err := tx.Query(ctx, `
		SELECT ii.item_instance_id, ii.quantity, il.slot
		FROM item_instances ii
		JOIN item_locations il ON il.item_instance_id = ii.item_instance_id
		WHERE il.character_id = $1 AND il.location_kind = 'CHARACTER_INVENTORY'
		  AND ii.item_id = $2
		ORDER BY il.slot`, charID, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*journalv1.JournalConsumedItem
	var need = qty
	for rows.Next() {
		var instID id.UUID
		var have int64
		var slot string
		if err := rows.Scan(&instID, &have, &slot); err != nil {
			return nil, err
		}
		take := have
		if int64(need) < take {
			take = int64(need)
		}
		out = append(out, &journalv1.JournalConsumedItem{
			ItemInstanceId: instID[:],
			ItemId:         itemID,
			Quantity:       uint64(take),
		})
		need -= uint64(take)
		if need == 0 {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if need > 0 {
		return nil, fmt.Errorf("%w: %s short %d", ErrInsufficientItems, itemID, need)
	}
	return out, nil
}

// characterAct maps the character's persisted level to its 1..6 act
// band (progression_route.md: I=1..10 ... VI=51..60).
func characterAct(ctx context.Context, tx pgx.Tx, charID id.UUID) (int, error) {
	var level int32
	if err := tx.QueryRow(ctx,
		`SELECT level FROM characters WHERE character_id = $1`, charID).Scan(&level); err != nil {
		return 0, err
	}
	act := int((level + 9) / 10)
	if act < 1 {
		act = 1
	}
	if act > 6 {
		act = 6
	}
	return act, nil
}
