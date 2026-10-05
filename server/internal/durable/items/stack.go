package items

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/lockorder"
)

// MergeStacks folds src into dst up to the definition's stack cap (<= 9999);
// overflow stays as the src residual — all atomic (items.md § Stack Rules).
// Compatible stacks require the same item_id, exact effective_binding, and
// no differing instance state that affects transfer/use semantics. A merge
// into a locked stack is always rejected; merging out of a partially locked
// stack must leave the locked quantity behind.
func (s *Store) MergeStacks(ctx context.Context, tx pgx.Tx, dstID, srcID id.UUID, def Def,
	lock *TradeLockLedger) error {
	if dstID == srcID {
		return ErrIncompatibleStack
	}
	locks := []lockorder.Lock{
		lockorder.RowLock("item_instances", dstID),
		lockorder.RowLock("item_instances", srcID),
		lockorder.RowLock("item_locations", dstID),
		lockorder.RowLock("item_locations", srcID),
	}
	if err := acquireSorted(ctx, tx, locks); err != nil {
		return err
	}
	dst, dstLoc, err := s.peek(ctx, tx, dstID)
	if err != nil {
		return err
	}
	src, srcLoc, err := s.peek(ctx, tx, srcID)
	if err != nil {
		return err
	}
	if dstLoc.Kind != LocCharacterInventory || srcLoc.Kind != LocCharacterInventory ||
		dstLoc.CharacterID != srcLoc.CharacterID {
		return fmt.Errorf("%w: merge across %s/%s", ErrLocationConflict, dstLoc.Kind, srcLoc.Kind)
	}
	compatible, err := s.stacksCompatible(ctx, tx, dstID, srcID)
	if err != nil {
		return err
	}
	if !compatible {
		return ErrIncompatibleStack
	}
	cap := ApplyDefinitionDefaults(def).MaxStack
	if cap > MaxStackCeiling {
		cap = MaxStackCeiling
	}
	moved := src.Quantity
	if dst.Quantity+moved > cap {
		moved = cap - dst.Quantity
	}
	if moved <= 0 {
		return fmt.Errorf("%w: destination at cap %d", ErrIncompatibleStack, cap)
	}
	// Merge-into-locked is always banned; merging out of a locked stack may
	// only take from the free remainder.
	if err := s.assertLock(lock, dstID, dst.Quantity, OpMergeInto, 0); err != nil {
		return err
	}
	if err := s.assertLock(lock, srcID, src.Quantity, OpReduce, moved); err != nil {
		return err
	}
	if moved == src.Quantity {
		if _, err := tx.Exec(ctx,
			`UPDATE item_instances SET quantity = quantity + $2 WHERE item_instance_id = $1`,
			dstID.String(), moved); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM item_locations WHERE item_instance_id = $1`, srcID.String()); err != nil {
			return err
		}
		_, err = tx.Exec(ctx,
			`DELETE FROM item_instances WHERE item_instance_id = $1`, srcID.String())
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE item_instances SET quantity = $2 WHERE item_instance_id = $1`,
		dstID.String(), cap); err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`UPDATE item_instances SET quantity = quantity - $2 WHERE item_instance_id = $1`,
		srcID.String(), moved)
	return err
}

// SplitStack carves qty units into a new instance on a free inventory slot
// (same character, same item, same committed binding — items.md § Stack
// Rules). On a locked stack, only the free remainder may be split off: the
// locked stack must keep >= lockedQty.
func (s *Store) SplitStack(ctx context.Context, tx pgx.Tx, srcID id.UUID, qty int,
	newSlot string, lock *TradeLockLedger) (id.UUID, error) {
	if qty < 1 {
		return id.UUID{}, fmt.Errorf("%w: split qty %d", ErrQuantity, qty)
	}
	// Peek to resolve the owner char; inventory (4) and item rows (5) lock
	// in one sorted acquisition.
	_, srcLoc0, err := s.peek(ctx, tx, srcID)
	if err != nil {
		return id.UUID{}, err
	}
	src, srcLoc, err := s.loadAndLock(ctx, tx, srcID,
		lockorder.RowLock("character_inventories", srcLoc0.CharacterID))
	if err != nil {
		return id.UUID{}, err
	}
	if srcLoc.Kind != srcLoc0.Kind || srcLoc.CharacterID != srcLoc0.CharacterID {
		return id.UUID{}, fmt.Errorf("%w: location changed under lock", ErrLocationConflict)
	}
	if srcLoc.Kind != LocCharacterInventory {
		return id.UUID{}, fmt.Errorf("%w: split from %s", ErrLocationConflict, srcLoc.Kind)
	}
	if qty >= src.Quantity {
		return id.UUID{}, fmt.Errorf("%w: split %d of %d leaves nothing", ErrQuantity, qty, src.Quantity)
	}
	if err := s.assertLock(lock, srcID, src.Quantity, OpReduce, qty); err != nil {
		return id.UUID{}, err
	}
	dst := Location{Kind: LocCharacterInventory, CharacterID: srcLoc.CharacterID, Slot: newSlot}
	if err := s.checkLocation(ctx, tx, dst); err != nil {
		return id.UUID{}, err
	}
	newID := id.NewV4()
	if _, err := tx.Exec(ctx,
		`INSERT INTO item_instances
		 (item_instance_id, item_id, quantity, effective_binding, enhancement_level,
		  item_state, content_revision, created_at)
		 SELECT $1, item_id, $2, effective_binding, enhancement_level,
		        item_state, content_revision, created_at
		   FROM item_instances WHERE item_instance_id = $3`,
		newID.String(), qty, srcID.String()); err != nil {
		return id.UUID{}, fmt.Errorf("items: split insert: %w", err)
	}
	if err := writeLocation(ctx, tx, newID, dst); err != nil {
		return id.UUID{}, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE item_instances SET quantity = quantity - $2 WHERE item_instance_id = $1`,
		srcID.String(), qty); err != nil {
		return id.UUID{}, err
	}
	return newID, nil
}

// stacksCompatible compares the two rows server-side: same item_id, exact
// effective_binding, identical item_state and content_revision
// (items.md § Stack Rules "no differing instance/source state").
func (s *Store) stacksCompatible(ctx context.Context, tx pgx.Tx, a, b id.UUID) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx,
		`SELECT a.item_id = b.item_id
		        AND a.effective_binding = b.effective_binding
		        AND a.item_state = b.item_state
		        AND a.content_revision IS NOT DISTINCT FROM b.content_revision
		   FROM item_instances a, item_instances b
		  WHERE a.item_instance_id = $1 AND b.item_instance_id = $2`,
		a.String(), b.String()).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrUnknownItem
	}
	return ok, err
}
