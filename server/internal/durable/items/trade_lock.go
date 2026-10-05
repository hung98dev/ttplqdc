package items

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/lockorder"
)

// TradeOp classifies an operation against a partially locked stack for
// TradeLockLedger.AssertOpAllowed (items.md § Trade Lock verbatim).
type TradeOp int

const (
	// OpCustodyMove covers every operation that moves the locked stack
	// itself or manipulates it as a unit: move, equip, unequip, beast
	// equip/unequip, guild deposit/withdraw, escrow list/reclaim, whole
	// discard, or offering it into another session. Always INVALID_STATE
	// while any quantity is locked.
	OpCustodyMove TradeOp = iota
	// OpReduce removes qty units from the stack — consumable use,
	// split-off, merge-from, free-remainder discard. Rejected when the
	// post-op stack would drop below the locked quantity.
	OpReduce
	// OpMergeInto merges another stack into this one — always banned on a
	// locked stack.
	OpMergeInto
)

// TradeLockLedger is the runtime-only direct-trade lock ledger (items.md §
// Trade Lock, data_model.md § Auction/Trade, trading_auction.md): session
// state of the owning map-instance simulation, never persisted. One ledger
// instance lives per offeror side of a session; Cancel covers every
// non-committing session end (cancel, timeout, disconnect, server restart,
// map transfer, respawn, instance entry, death) — release only, nothing moves.
type TradeLockLedger struct {
	mu     sync.Mutex
	owner  id.UUID
	locked map[id.UUID]int // item_instance_id -> locked quantity
}

// NewTradeLockLedger creates the per-session ledger of one offeror.
func NewTradeLockLedger(ownerCharacterID id.UUID) *TradeLockLedger {
	return &TradeLockLedger{owner: ownerCharacterID, locked: map[id.UUID]int{}}
}

// Owner returns the offeror character the ledger binds to.
func (l *TradeLockLedger) Owner() id.UUID { return l.owner }

// Offer locks exactly q units of a stack of n (partial-stack semantics,
// ADR-0062). Re-offering an already offered stack is INVALID_STATE.
func (l *TradeLockLedger) Offer(instanceID id.UUID, q, n int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if q < 1 || q > n {
		return fmt.Errorf("%w: offer quantity %d outside 1..%d", ErrQuantity, q, n)
	}
	if _, exists := l.locked[instanceID]; exists {
		return ErrTradeLocked
	}
	l.locked[instanceID] = q
	return nil
}

// LockedQty returns the locked quantity of an instance (0 = unlocked).
func (l *TradeLockLedger) LockedQty(instanceID id.UUID) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.locked[instanceID]
}

// LockedInstances returns the locked instance IDs (settlement snapshot).
func (l *TradeLockLedger) LockedInstances() []id.UUID {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]id.UUID, 0, len(l.locked))
	for iid := range l.locked {
		out = append(out, iid)
	}
	return out
}

// AssertOpAllowed enforces the § Trade Lock rules for one stack:
//   - OpCustodyMove on a locked stack -> INVALID_STATE (ErrTradeLocked).
//   - OpReduce(qty) requires stackQty - qty >= lockedQty (the free
//     remainder n - q is usable/sellable/discardable/splittable, never
//     below the locked q).
//   - OpMergeInto -> INVALID_STATE (no merge into a locked stack).
func (l *TradeLockLedger) AssertOpAllowed(instanceID id.UUID, stackQty int, op TradeOp, qty int) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	locked := l.locked[instanceID]
	if locked == 0 {
		return nil
	}
	switch op {
	case OpReduce:
		if qty < 0 || stackQty-qty < locked {
			return fmt.Errorf("%w: reduce %d of %d would breach locked %d",
				ErrTradeLocked, qty, stackQty, locked)
		}
		return nil
	default:
		return ErrTradeLocked
	}
}

// Cancel releases every lock and moves nothing — the release-only end for
// cancel, timeout, disconnect, server restart, map transfer, respawn,
// instance entry, or death of either participant.
func (l *TradeLockLedger) Cancel() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.locked = map[id.UUID]int{}
}

// RevalidateOffers is the COMMITTING re-check (items.md § Trade Lock +
// data_model.md): under row locks, every offered instance must still be in
// the offeror's CHARACTER_INVENTORY with quantity >= locked, UNBOUND
// (player-tradable), and not Soul-contracted. A failure aborts settlement;
// the caller then Cancels (release-only).
func (l *TradeLockLedger) RevalidateOffers(ctx context.Context, tx pgx.Tx) error {
	l.mu.Lock()
	offers := make(map[id.UUID]int, len(l.locked))
	for iid, q := range l.locked {
		offers[iid] = q
	}
	l.mu.Unlock()
	if len(offers) == 0 {
		return nil
	}
	ids := make([]id.UUID, 0, len(offers))
	for iid := range offers {
		ids = append(ids, iid)
	}
	locks := make([]lockorder.Lock, 0, 2*len(ids)+1)
	for _, iid := range ids {
		locks = append(locks,
			lockorder.RowLock("item_instances", iid),
			lockorder.RowLock("item_locations", iid))
	}
	locks = append(locks, lockorder.RowLock("character_souls", l.owner))
	if err := lockorder.SortLocks(locks); err != nil {
		return err
	}
	if err := lockorder.Acquire(ctx, tx, locks...); err != nil {
		return err
	}
	for _, iid := range ids {
		var (
			kind, bind, owner string
			qty               int
			contracted        bool
		)
		err := tx.QueryRow(ctx,
			`SELECT l.location_kind, i.effective_binding, l.character_id::text, i.quantity
			   FROM item_instances i
			   JOIN item_locations l ON l.item_instance_id = i.item_instance_id
			  WHERE i.item_instance_id = $1`, iid.String()).
			Scan(&kind, &bind, &owner, &qty)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: offered instance %s gone", ErrUnknownItem, iid)
		}
		if err != nil {
			return err
		}
		if kind != string(LocCharacterInventory) {
			return fmt.Errorf("%w: offered instance %s in %s", ErrLocationConflict, iid, kind)
		}
		if owner != l.owner.String() {
			return fmt.Errorf("%w: offered instance %s not owned by offeror", ErrForbiddenOwner, iid)
		}
		if bind != "UNBOUND" {
			return fmt.Errorf("%w: offered instance %s is %s", ErrBindingBlocksTransfer, iid, bind)
		}
		if qty < offers[iid] {
			return fmt.Errorf("%w: offered instance %s qty %d below locked %d",
				ErrTradeLocked, iid, qty, offers[iid])
		}
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM character_souls WHERE contracted_item_instance_id=$1)`,
			iid.String()).Scan(&contracted); err != nil {
			return err
		}
		if contracted {
			return fmt.Errorf("%w: offered instance %s", ErrSoulContracted, iid)
		}
	}
	return nil
}
