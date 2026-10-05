package items

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/lockorder"
)

// Binding is the items.md § Binding restriction ladder:
// UNBOUND < ACCOUNT_BOUND < CHARACTER_BOUND (higher = more restrictive).
type Binding uint8

const (
	BindingUnbound Binding = iota
	BindingAccountBound
	BindingCharacterBound
)

func (b Binding) String() string {
	switch b {
	case BindingUnbound:
		return "UNBOUND"
	case BindingAccountBound:
		return "ACCOUNT_BOUND"
	case BindingCharacterBound:
		return "CHARACTER_BOUND"
	}
	return fmt.Sprintf("Binding(%d)", int(b))
}

// ParseBinding reads the item_instances.effective_binding VARCHAR form.
func ParseBinding(s string) (Binding, error) {
	switch s {
	case "UNBOUND":
		return BindingUnbound, nil
	case "ACCOUNT_BOUND":
		return BindingAccountBound, nil
	case "CHARACTER_BOUND":
		return BindingCharacterBound, nil
	}
	return 0, fmt.Errorf("items: unknown binding %q", s)
}

// AtLeast reports b >= floor on the restriction ladder.
func (b Binding) AtLeast(floor Binding) bool { return b >= floor }

// Tighten returns the more restrictive of two bindings.
func Tighten(a, b Binding) Binding {
	if b > a {
		return b
	}
	return a
}

// Trigger is the items.md binding trigger enum (metadata on Def).
type Trigger string

const (
	TriggerNone      Trigger = "NONE"
	TriggerOnAcquire Trigger = "ON_ACQUIRE"
	TriggerOnEquip   Trigger = "ON_EQUIP"
	TriggerOnUse     Trigger = "ON_USE"
)

// resolveEffectiveBinding applies items.md § Source Binding Override:
// effective = max(definition, source override); a source override that
// loosens below the definition is rejected.
func resolveEffectiveBinding(def Def, source *Binding) (Binding, error) {
	if source == nil {
		return def.DefaultBinding, nil
	}
	if *source < def.DefaultBinding {
		return 0, fmt.Errorf("%w: source %s below definition %s",
			ErrBindingOverride, source.String(), def.DefaultBinding.String())
	}
	return *source, nil
}

// AssertTransferable enforces the custody rules of items.md § Account-Bound /
// § Invariants: any custody transition targeting a different character — same
// account or not — is rejected when effective_binding is ACCOUNT_BOUND or
// CHARACTER_BOUND, and both non-UNBOUND bindings are forbidden from guild
// storage and auction escrow entirely.
func AssertTransferable(b Binding, dst Location, curCharacterID id.UUID) error {
	if b == BindingUnbound {
		return nil
	}
	switch dst.Kind {
	case LocGuildStorage, LocAuctionEscrow:
		return ErrBindingBlocksTransfer
	case LocCharacterInventory, LocEquipped:
		if dst.CharacterID != curCharacterID {
			return ErrBindingBlocksTransfer
		}
		return nil
	default:
		return ErrInvalidLocation
	}
}

// SetEffectiveBinding applies a tighten-only binding change on an existing
// instance (items.md: "binding never loosens"; a stricter source re-grant
// keeps the committed stricter value).
func (s *Store) SetEffectiveBinding(ctx context.Context, tx pgx.Tx, instanceID id.UUID, target Binding) error {
	if target.String() == "" || target > BindingCharacterBound {
		return fmt.Errorf("items: unknown binding %d", int(target))
	}
	if err := acquireSorted(ctx, tx, []lockorder.Lock{lockorder.RowLock("item_instances", instanceID)}); err != nil {
		return err
	}
	var cur string
	err := tx.QueryRow(ctx,
		`SELECT effective_binding FROM item_instances WHERE item_instance_id=$1`,
		instanceID.String()).Scan(&cur)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrUnknownItem
		}
		return fmt.Errorf("items: load binding: %w", err)
	}
	curB, err := ParseBinding(cur)
	if err != nil {
		return err
	}
	if target < curB {
		return fmt.Errorf("%w: %s cannot loosen to %s", ErrBindingOverride, curB, target)
	}
	if target == curB {
		return nil
	}
	_, err = tx.Exec(ctx,
		`UPDATE item_instances SET effective_binding=$2 WHERE item_instance_id=$1`,
		instanceID.String(), target.String())
	return err
}
