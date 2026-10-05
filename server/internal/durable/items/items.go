package items

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/lockorder"
)

// MaxStackCeiling is the items.md § Stack Rules global stack cap.
const MaxStackCeiling = 9999

// Kind is the items.md § Identity/Types item-kind flag set.
type Kind uint32

const (
	KindEquipment Kind = 1 << iota
	KindBeastEquipment
	KindConsumable
	KindMaterial
	KindTool
	KindQuest
	KindMisc
)

// LocationKind is the items.md § Ownership Context canonical enum; the
// values are identical to the item_locations CHECK constraint.
type LocationKind string

const (
	LocCharacterInventory LocationKind = "CHARACTER_INVENTORY"
	LocEquipped           LocationKind = "EQUIPPED"
	LocBeastEquipmentSlot LocationKind = "BEAST_EQUIPMENT_SLOT"
	LocGuildStorage       LocationKind = "GUILD_STORAGE"
	LocAuctionEscrow      LocationKind = "AUCTION_ESCROW"
)

// Sentinel errors; wire codes map at the edge only.
var (
	ErrSlotOccupied          = errors.New("items: slot occupied")
	ErrCapacity              = errors.New("items: inventory capacity exhausted")
	ErrIncompatibleStack     = errors.New("items: incompatible stacks")
	ErrNotDiscardable        = errors.New("items: item is not discardable")
	ErrTradeLocked           = errors.New("items: stack is trade-locked (INVALID_STATE)")
	ErrBindingBlocksTransfer = errors.New("items: binding blocks this transfer")
	ErrLocationConflict      = errors.New("items: conflicting location")
	ErrForbiddenOwner        = errors.New("items: not the owner")
	ErrUnknownItem           = errors.New("items: unknown item instance")
	ErrInvalidLocation       = errors.New("items: invalid location")
	ErrSoulContracted        = errors.New("items: item is soul-contracted")
	ErrBindingOverride       = errors.New("items: source binding override may only tighten")
	ErrQuantity              = errors.New("items: invalid quantity")
)

// Def is a catalog item-definition row projection (items.md § Identity +
// § Definition Defaults + § Binding). Nil DiscardAllowed / empty
// SharedCooldownGroup / zero MaxStack are resolved by
// ApplyDefinitionDefaults; Create applies it internally.
type Def struct {
	ItemID              string
	Kind                Kind
	Stackable           bool
	MaxStack            int
	DiscardAllowed      *bool
	SharedCooldownGroup CooldownGroup
	DefaultBinding      Binding
	BindingTrigger      Trigger
}

// Discardable reports the resolved discard_allowed (defaults applied).
func (d Def) Discardable() bool { return *ApplyDefinitionDefaults(d).DiscardAllowed }

// Instance is one materialized item_instances row.
type Instance struct {
	InstanceID       id.UUID
	ItemID           string
	Quantity         int
	EffectiveBinding Binding
	EnhancementLevel int16
	ItemState        []byte
	ContentRevision  *string
	CreatedAt        time.Time
}

// Location is the single item_locations row of a live instance; per-kind
// required fields mirror the schema CHECKs.
type Location struct {
	Kind                 LocationKind
	CharacterID          id.UUID
	GuildID              id.UUID
	ListingID            id.UUID
	Slot                 string
	DepositorCharacterID id.UUID
	DepositorAccountID   id.UUID
}

// Store executes item custody operations on callers' transactions.
type Store struct{ pool *pgxpool.Pool }

// New returns the primitive bound to the durable pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// CreateFields carries per-instance creation state beyond the definition.
type CreateFields struct {
	InstanceID       id.UUID // zero => generated UUIDv4
	Quantity         int
	EnhancementLevel int16
	ItemState        []byte   // nil => '{}'
	ContentRevision  *string  // nil => NULL
	SourceBinding    *Binding // stricter source override (items.md)
	CreatedAt        time.Time
}

// Create commits item_instances + its exactly-one item_locations row in tx.
// Definition defaults apply; the resolved effective binding is committed
// atomically (items.md § Source Binding Override).
func (s *Store) Create(ctx context.Context, tx pgx.Tx, def Def, f CreateFields, loc Location) (id.UUID, error) {
	def = ApplyDefinitionDefaults(def)
	effective, err := resolveEffectiveBinding(def, f.SourceBinding)
	if err != nil {
		return id.UUID{}, err
	}
	maxStack := def.MaxStack
	if maxStack < 1 {
		maxStack = 1
	}
	if maxStack > MaxStackCeiling {
		maxStack = MaxStackCeiling
	}
	if f.Quantity < 1 || f.Quantity > maxStack {
		return id.UUID{}, fmt.Errorf("%w: quantity %d outside 1..%d", ErrQuantity, f.Quantity, maxStack)
	}
	if f.EnhancementLevel < 0 || f.EnhancementLevel > 16 {
		return id.UUID{}, fmt.Errorf("%w: enhancement_level %d outside 0..16", ErrQuantity, f.EnhancementLevel)
	}
	if err := validateLocation(loc); err != nil {
		return id.UUID{}, err
	}
	// Canonical lock order: the destination inventory row (priority 4) is
	// locked before any item row; guild/escrow destinations sit at
	// priorities 13/15 so they lock after the (nonexistent) item rows.
	var locks []lockorder.Lock
	if loc.Kind == LocCharacterInventory {
		locks = append(locks, lockorder.RowLock("character_inventories", loc.CharacterID))
	}
	if loc.Kind == LocGuildStorage {
		locks = append(locks, lockorder.GuildStorageLock(loc.GuildID))
	}
	if loc.Kind == LocAuctionEscrow {
		locks = append(locks, lockorder.RowLock("auction_listings", loc.ListingID))
	}
	if err := acquireSorted(ctx, tx, locks); err != nil {
		return id.UUID{}, err
	}
	if err := s.checkLocation(ctx, tx, loc); err != nil {
		return id.UUID{}, err
	}
	instanceID := f.InstanceID
	if instanceID.IsNil() {
		instanceID = id.NewV4()
	}
	state := f.ItemState
	if len(state) == 0 {
		state = []byte("{}")
	}
	createdAt := f.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO item_instances
		 (item_instance_id, item_id, quantity, effective_binding, enhancement_level,
		  item_state, content_revision, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8)`,
		instanceID.String(), def.ItemID, f.Quantity, effective.String(),
		f.EnhancementLevel, string(state), f.ContentRevision, createdAt); err != nil {
		return id.UUID{}, fmt.Errorf("items: insert instance: %w", err)
	}
	if err := writeLocation(ctx, tx, instanceID, loc); err != nil {
		return id.UUID{}, err
	}
	return instanceID, nil
}

// Get loads an instance by id.
func (s *Store) Get(ctx context.Context, instanceID id.UUID) (*Instance, error) {
	return scanInstance(s.pool.QueryRow(ctx,
		`SELECT item_instance_id, item_id, quantity, effective_binding,
		        enhancement_level, item_state, content_revision, created_at
		   FROM item_instances WHERE item_instance_id=$1`, instanceID.String()))
}

// Location loads the instance's single location row.
func (s *Store) Location(ctx context.Context, instanceID id.UUID) (*Location, error) {
	return scanLocation(s.pool.QueryRow(ctx,
		`SELECT location_kind, character_id, guild_id, listing_id, slot,
		        depositor_character_id, depositor_account_id
		   FROM item_locations WHERE item_instance_id=$1`, instanceID.String()))
}

// Move transitions an instance between character inventories — the generic
// custody primitive (settlement callers drive owner A -> owner B here).
// Other custody kinds use their dedicated writers (Equip, EquipBeast,
// GuildDeposit, EscrowForListing, and the reverses).
func (s *Store) Move(ctx context.Context, tx pgx.Tx, instanceID id.UUID, dst Location, lock *TradeLockLedger) error {
	if dst.Kind != LocCharacterInventory {
		return fmt.Errorf("%w: %s requires its dedicated writer", ErrInvalidLocation, dst.Kind)
	}
	if err := validateLocation(dst); err != nil {
		return err
	}
	inst, cur, err := s.loadAndLock(ctx, tx, instanceID,
		lockorder.RowLock("character_inventories", dst.CharacterID))
	if err != nil {
		return err
	}
	if err := s.assertLock(lock, instanceID, inst.Quantity, OpCustodyMove, 0); err != nil {
		return err
	}
	if cur.Kind != LocCharacterInventory {
		return fmt.Errorf("%w: cannot move out of %s directly", ErrLocationConflict, cur.Kind)
	}
	// Cross-character custody: non-UNBOUND is rejected including
	// same-account (items.md § Account-Bound, ADR-0029 — no vault mule).
	if err := AssertTransferable(inst.EffectiveBinding, dst, cur.CharacterID); err != nil {
		return err
	}
	if err := s.checkLocation(ctx, tx, dst); err != nil {
		return err
	}
	return updateLocation(ctx, tx, instanceID, dst)
}

// Equip moves an inventory item into a character equipment slot.
func (s *Store) Equip(ctx context.Context, tx pgx.Tx, instanceID id.UUID, def Def,
	characterID id.UUID, slot string, lock *TradeLockLedger) error {
	if def.Kind&KindEquipment == 0 {
		return fmt.Errorf("%w: kind is not EQUIPMENT", ErrInvalidLocation)
	}
	if slot == "" {
		return ErrInvalidLocation
	}
	inst, cur, err := s.loadAndLock(ctx, tx, instanceID)
	if err != nil {
		return err
	}
	if err := s.assertLock(lock, instanceID, inst.Quantity, OpCustodyMove, 0); err != nil {
		return err
	}
	if cur.Kind != LocCharacterInventory || cur.CharacterID != characterID {
		return fmt.Errorf("%w: equip from %s", ErrLocationConflict, cur.Kind)
	}
	var taken bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM item_locations
		  WHERE character_id=$1 AND location_kind='EQUIPPED' AND slot=$2)`,
		characterID.String(), slot).Scan(&taken); err != nil {
		return err
	}
	if taken {
		return ErrSlotOccupied
	}
	return updateLocation(ctx, tx, instanceID,
		Location{Kind: LocEquipped, CharacterID: characterID, Slot: slot})
}

// Unequip returns an EQUIPPED item to a free CHARACTER_INVENTORY slot.
func (s *Store) Unequip(ctx context.Context, tx pgx.Tx, instanceID id.UUID,
	inventorySlot string, lock *TradeLockLedger) error {
	// Peek to resolve the owner character so its inventory row (priority 4)
	// joins the same sorted acquisition as the item rows (5).
	_, cur0, err := s.peek(ctx, tx, instanceID)
	if err != nil {
		return err
	}
	inst, cur, err := s.loadAndLock(ctx, tx, instanceID,
		lockorder.RowLock("character_inventories", cur0.CharacterID))
	if err != nil {
		return err
	}
	if cur.Kind != cur0.Kind || cur.CharacterID != cur0.CharacterID {
		return fmt.Errorf("%w: location changed under lock", ErrLocationConflict)
	}
	if err := s.assertLock(lock, instanceID, inst.Quantity, OpCustodyMove, 0); err != nil {
		return err
	}
	if cur.Kind != LocEquipped {
		return fmt.Errorf("%w: unequip from %s", ErrLocationConflict, cur.Kind)
	}
	dst := Location{Kind: LocCharacterInventory, CharacterID: cur.CharacterID, Slot: inventorySlot}
	if err := s.checkLocation(ctx, tx, dst); err != nil {
		return err
	}
	return updateLocation(ctx, tx, instanceID, dst)
}

// EquipBeast transitions an inventory BEAST_EQUIPMENT item into a Linh Thú
// slot: item_locations -> BEAST_EQUIPMENT_SLOT and a
// beast_equipment_locations row commit atomically (items.md § Beast
// Equipment, data_model.md § Spirit Beasts). The item can never occupy a
// character loadout, guild storage, trade offer or AUCTION_ESCROW.
func (s *Store) EquipBeast(ctx context.Context, tx pgx.Tx, instanceID id.UUID, def Def,
	characterID id.UUID, beastID string, slotID string, lock *TradeLockLedger) error {
	if def.Kind&KindBeastEquipment == 0 {
		return fmt.Errorf("%w: kind is not BEAST_EQUIPMENT", ErrInvalidLocation)
	}
	switch slotID {
	case "vong_co", "ao_giap", "linh_chau":
	default:
		return ErrInvalidLocation
	}
	inst, cur, err := s.loadAndLock(ctx, tx, instanceID,
		lockorder.RowLock("character_beasts", characterID),
		lockorder.RowLock("beast_equipment_locations", characterID))
	if err != nil {
		return err
	}
	if err := s.assertLock(lock, instanceID, inst.Quantity, OpCustodyMove, 0); err != nil {
		return err
	}
	if cur.Kind != LocCharacterInventory || cur.CharacterID != characterID {
		return fmt.Errorf("%w: beast-equip from %s", ErrLocationConflict, cur.Kind)
	}
	var beast bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM character_beasts WHERE character_id=$1 AND beast_id=$2)`,
		characterID.String(), beastID).Scan(&beast); err != nil {
		return err
	}
	if !beast {
		return ErrInvalidLocation
	}
	var taken bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM beast_equipment_locations
		  WHERE character_id=$1 AND beast_id=$2 AND slot_id=$3)`,
		characterID.String(), beastID, slotID).Scan(&taken); err != nil {
		return err
	}
	if taken {
		return ErrSlotOccupied
	}
	if err := updateLocation(ctx, tx, instanceID, Location{
		Kind: LocBeastEquipmentSlot, CharacterID: characterID, Slot: slotID}); err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO beast_equipment_locations (character_id, beast_id, slot_id, item_instance_id)
		 VALUES ($1,$2,$3,$4)`, characterID.String(), beastID, slotID, instanceID.String())
	return translateSlotErr(err)
}

// UnequipBeast deletes the beast_equipment_locations row and returns the
// item to a free CHARACTER_INVENTORY slot atomically.
func (s *Store) UnequipBeast(ctx context.Context, tx pgx.Tx, instanceID id.UUID,
	inventorySlot string, lock *TradeLockLedger) error {
	// Peek to resolve the owner character; inventory (4), item rows (5) and
	// beast_equipment_locations (6) then lock in one canonical batch.
	_, cur0, err := s.peek(ctx, tx, instanceID)
	if err != nil {
		return err
	}
	inst, cur, err := s.loadAndLock(ctx, tx, instanceID,
		lockorder.RowLock("character_inventories", cur0.CharacterID),
		lockorder.RowLock("beast_equipment_locations", cur0.CharacterID))
	if err != nil {
		return err
	}
	if cur.Kind != cur0.Kind || cur.CharacterID != cur0.CharacterID {
		return fmt.Errorf("%w: location changed under lock", ErrLocationConflict)
	}
	if err := s.assertLock(lock, instanceID, inst.Quantity, OpCustodyMove, 0); err != nil {
		return err
	}
	if cur.Kind != LocBeastEquipmentSlot {
		return fmt.Errorf("%w: unequip-beast from %s", ErrLocationConflict, cur.Kind)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM beast_equipment_locations WHERE item_instance_id=$1`,
		instanceID.String()); err != nil {
		return err
	}
	dst := Location{Kind: LocCharacterInventory, CharacterID: cur.CharacterID, Slot: inventorySlot}
	if err := s.checkLocation(ctx, tx, dst); err != nil {
		return err
	}
	return updateLocation(ctx, tx, instanceID, dst)
}

// GuildDeposit moves an inventory item into guild storage; non-UNBOUND
// bindings never enter guild storage (items.md § Account-Bound).
func (s *Store) GuildDeposit(ctx context.Context, tx pgx.Tx, instanceID, guildID id.UUID,
	slot string, lock *TradeLockLedger) error {
	inst, cur, err := s.loadAndLock(ctx, tx, instanceID,
		lockorder.GuildStorageLock(guildID))
	if err != nil {
		return err
	}
	if err := s.assertLock(lock, instanceID, inst.Quantity, OpCustodyMove, 0); err != nil {
		return err
	}
	if cur.Kind != LocCharacterInventory {
		return fmt.Errorf("%w: guild-deposit from %s", ErrLocationConflict, cur.Kind)
	}
	dst := Location{Kind: LocGuildStorage, GuildID: guildID, Slot: slot}
	if err := AssertTransferable(inst.EffectiveBinding, dst, cur.CharacterID); err != nil {
		return err
	}
	// Depositor identity for the ADR-0049 same-account check / transfer
	// signal lives on the location row.
	acct, err := s.accountOf(ctx, tx, cur.CharacterID)
	if err != nil {
		return err
	}
	dst.DepositorCharacterID, dst.DepositorAccountID = cur.CharacterID, acct
	if err := s.checkLocation(ctx, tx, dst); err != nil {
		return err
	}
	return updateLocation(ctx, tx, instanceID, dst)
}

// GuildWithdraw returns a guild-storage item to a character inventory slot.
func (s *Store) GuildWithdraw(ctx context.Context, tx pgx.Tx, instanceID, toCharacterID id.UUID,
	inventorySlot string, lock *TradeLockLedger) error {
	inst, cur, err := s.loadAndLock(ctx, tx, instanceID,
		lockorder.RowLock("character_inventories", toCharacterID))
	if err != nil {
		return err
	}
	if err := s.assertLock(lock, instanceID, inst.Quantity, OpCustodyMove, 0); err != nil {
		return err
	}
	if cur.Kind != LocGuildStorage {
		return fmt.Errorf("%w: guild-withdraw from %s", ErrLocationConflict, cur.Kind)
	}
	if err := acquireSorted(ctx, tx, []lockorder.Lock{lockorder.GuildStorageLock(cur.GuildID)}); err != nil {
		return err
	}
	dst := Location{Kind: LocCharacterInventory, CharacterID: toCharacterID, Slot: inventorySlot}
	if err := AssertTransferable(inst.EffectiveBinding, dst, toCharacterID); err != nil {
		return err
	}
	if err := s.checkLocation(ctx, tx, dst); err != nil {
		return err
	}
	return updateLocation(ctx, tx, instanceID, dst)
}

// EscrowForListing places an inventory item into AUCTION_ESCROW under an
// existing listing row (the listing write itself is the auction caller's).
// Non-UNBOUND and Soul-contracted items are never listed.
func (s *Store) EscrowForListing(ctx context.Context, tx pgx.Tx, instanceID, listingID id.UUID,
	lock *TradeLockLedger) error {
	// Peek for the owner char: souls (7) and the listing (15) join the item
	// locks (5) in one sorted acquisition.
	_, cur0, err := s.peek(ctx, tx, instanceID)
	if err != nil {
		return err
	}
	inst, cur, err := s.loadAndLock(ctx, tx, instanceID,
		lockorder.RowLock("character_souls", cur0.CharacterID),
		lockorder.RowLock("auction_listings", listingID))
	if err != nil {
		return err
	}
	if cur.Kind != cur0.Kind || cur.CharacterID != cur0.CharacterID {
		return fmt.Errorf("%w: location changed under lock", ErrLocationConflict)
	}
	if err := s.assertLock(lock, instanceID, inst.Quantity, OpCustodyMove, 0); err != nil {
		return err
	}
	if cur.Kind != LocCharacterInventory {
		return fmt.Errorf("%w: escrow from %s", ErrLocationConflict, cur.Kind)
	}
	dst := Location{Kind: LocAuctionEscrow, ListingID: listingID}
	if err := AssertTransferable(inst.EffectiveBinding, dst, cur.CharacterID); err != nil {
		return err
	}
	if err := s.assertSoulFree(ctx, tx, instanceID); err != nil {
		return err
	}
	if err := s.checkLocation(ctx, tx, dst); err != nil {
		return err
	}
	return updateLocation(ctx, tx, instanceID, dst)
}

// ReclaimEscrow returns an AUCTION_ESCROW item to its seller's inventory
// (cancelled/expired listing reclaim; the listing state machine is the
// auction caller's).
func (s *Store) ReclaimEscrow(ctx context.Context, tx pgx.Tx, instanceID id.UUID,
	inventorySlot string, lock *TradeLockLedger) error {
	// Peek resolves listing -> seller, so the seller inventory row (4),
	// item rows (5) and listing (15) lock in one canonical batch.
	_, cur0, err := s.peek(ctx, tx, instanceID)
	if err != nil {
		return err
	}
	if cur0.Kind != LocAuctionEscrow {
		return fmt.Errorf("%w: reclaim from %s", ErrLocationConflict, cur0.Kind)
	}
	sellerID, err := s.listingSeller(ctx, tx, cur0.ListingID)
	if err != nil {
		return err
	}
	inst, cur, err := s.loadAndLock(ctx, tx, instanceID,
		lockorder.RowLock("character_inventories", sellerID),
		lockorder.RowLock("auction_listings", cur0.ListingID))
	if err != nil {
		return err
	}
	if cur.Kind != cur0.Kind || cur.ListingID != cur0.ListingID {
		return fmt.Errorf("%w: location changed under lock", ErrLocationConflict)
	}
	if err := s.assertLock(lock, instanceID, inst.Quantity, OpCustodyMove, 0); err != nil {
		return err
	}
	seller2, err := s.listingSeller(ctx, tx, cur.ListingID)
	if err != nil {
		return err
	}
	if seller2 != sellerID {
		return fmt.Errorf("%w: listing seller changed under lock", ErrLocationConflict)
	}
	dst := Location{Kind: LocCharacterInventory, CharacterID: sellerID, Slot: inventorySlot}
	if err := s.checkLocation(ctx, tx, dst); err != nil {
		return err
	}
	return updateLocation(ctx, tx, instanceID, dst)
}

func (s *Store) listingSeller(ctx context.Context, tx pgx.Tx, listingID id.UUID) (id.UUID, error) {
	var seller string
	err := tx.QueryRow(ctx,
		`SELECT seller_character_id FROM auction_listings WHERE listing_id=$1`,
		listingID.String()).Scan(&seller)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return id.UUID{}, ErrInvalidLocation
		}
		return id.UUID{}, err
	}
	return id.ParseUUID(seller)
}

// Discard applies the full items.md § Transfer/Use/Discard guard then
// destroys the instance: discard_allowed required; EQUIPPED /
// AUCTION_ESCROW / BEAST_EQUIPMENT_SLOT / GUILD_STORAGE rows are not
// discardable in place; a trade-locked stack cannot be discarded at all;
// a Soul-contracted instance is protected inside the same transaction.
func (s *Store) Discard(ctx context.Context, tx pgx.Tx, instanceID id.UUID, def Def,
	lock *TradeLockLedger) error {
	if !def.Discardable() {
		return ErrNotDiscardable
	}
	// Peek for the owner char so the soul rows (7) lock in the same sorted
	// batch as the item rows (5).
	_, cur0, err := s.peek(ctx, tx, instanceID)
	if err != nil {
		return err
	}
	inst, cur, err := s.loadAndLock(ctx, tx, instanceID,
		lockorder.RowLock("character_souls", cur0.CharacterID))
	if err != nil {
		return err
	}
	if cur.Kind != cur0.Kind || cur.CharacterID != cur0.CharacterID {
		return fmt.Errorf("%w: location changed under lock", ErrLocationConflict)
	}
	if err := s.assertLock(lock, instanceID, inst.Quantity, OpCustodyMove, 0); err != nil {
		return err
	}
	switch cur.Kind {
	case LocCharacterInventory:
	default:
		return fmt.Errorf("%w: discard from %s", ErrNotDiscardable, cur.Kind)
	}
	if err := s.assertSoulFree(ctx, tx, instanceID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM item_locations WHERE item_instance_id=$1`, instanceID.String()); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM item_instances WHERE item_instance_id=$1`, instanceID.String()); err != nil {
		return err
	}
	return nil
}

// ListForAccount returns every instance owned by characters of the account
// — the account-scoped view grant of items.md § Account-Bound. Custody is
// unaffected: this never moves anything.
func (s *Store) ListForAccount(ctx context.Context, accountID id.UUID) ([]*Instance, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT i.item_instance_id, i.item_id, i.quantity, i.effective_binding,
		        i.enhancement_level, i.item_state, i.content_revision, i.created_at
		   FROM item_instances i
		   JOIN item_locations l ON l.item_instance_id = i.item_instance_id
		   JOIN characters c ON c.character_id = l.character_id
		  WHERE c.account_id = $1
		  ORDER BY i.item_instance_id`, accountID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Instance
	for rows.Next() {
		inst, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inst)
	}
	return out, rows.Err()
}

// AssertUsable enforces account-scoped access semantics (items.md §
// Account-Bound): the item must sit in a character context (inventory,
// equipped, beast slot); UNBOUND and ACCOUNT_BOUND may be used by the owning
// account's characters, CHARACTER_BOUND only by the owning character.
// Custody movement is never implied.
func (s *Store) AssertUsable(ctx context.Context, instanceID, accountID, characterID id.UUID) error {
	loc, err := s.Location(ctx, instanceID)
	if err != nil {
		return err
	}
	if loc.CharacterID.IsNil() {
		return fmt.Errorf("%w: no character context in %s", ErrLocationConflict, loc.Kind)
	}
	inst, err := s.Get(ctx, instanceID)
	if err != nil {
		return err
	}
	ownerAcct, err := s.accountOfPool(ctx, loc.CharacterID)
	if err != nil {
		return err
	}
	if inst.EffectiveBinding == BindingCharacterBound && loc.CharacterID != characterID {
		return ErrForbiddenOwner
	}
	if ownerAcct != accountID {
		return ErrForbiddenOwner
	}
	return nil
}

// ---------------------------------------------------------------------------
// internals
// ---------------------------------------------------------------------------

// loadAndLock row-locks the item_instances + item_locations pair plus any
// extra locks, in one canonical sorted acquisition, then loads both rows.
func (s *Store) loadAndLock(ctx context.Context, tx pgx.Tx, instanceID id.UUID,
	extra ...lockorder.Lock) (*Instance, *Location, error) {
	locks := append([]lockorder.Lock{
		lockorder.RowLock("item_instances", instanceID),
		lockorder.RowLock("item_locations", instanceID),
	}, extra...)
	if err := acquireSorted(ctx, tx, locks); err != nil {
		return nil, nil, err
	}
	inst, err := scanInstance(tx.QueryRow(ctx,
		`SELECT item_instance_id, item_id, quantity, effective_binding,
		        enhancement_level, item_state, content_revision, created_at
		   FROM item_instances WHERE item_instance_id=$1`, instanceID.String()))
	if err != nil {
		return nil, nil, err
	}
	loc, err := scanLocation(tx.QueryRow(ctx,
		`SELECT location_kind, character_id, guild_id, listing_id, slot,
		        depositor_character_id, depositor_account_id
		   FROM item_locations WHERE item_instance_id=$1`, instanceID.String()))
	if err != nil {
		return nil, nil, err
	}
	return inst, loc, nil
}

// peek reads instance+location without locking — used to resolve lock keys
// (owner character, listing) before the canonical sorted acquisition; every
// caller re-loads and re-verifies under the locks.
func (s *Store) peek(ctx context.Context, tx pgx.Tx, instanceID id.UUID) (*Instance, *Location, error) {
	inst, err := scanInstance(tx.QueryRow(ctx,
		`SELECT item_instance_id, item_id, quantity, effective_binding,
		        enhancement_level, item_state, content_revision, created_at
		   FROM item_instances WHERE item_instance_id=$1`, instanceID.String()))
	if err != nil {
		return nil, nil, err
	}
	loc, err := scanLocation(tx.QueryRow(ctx,
		`SELECT location_kind, character_id, guild_id, listing_id, slot,
		        depositor_character_id, depositor_account_id
		   FROM item_locations WHERE item_instance_id=$1`, instanceID.String()))
	if err != nil {
		return nil, nil, err
	}
	return inst, loc, nil
}

// acquireSorted is the single canonical-order acquisition: SortLocks
// (priority/table/key order), then Acquire issues each row lock.
func acquireSorted(ctx context.Context, tx pgx.Tx, locks []lockorder.Lock) error {
	if err := lockorder.SortLocks(locks); err != nil {
		return err
	}
	return lockorder.Acquire(ctx, tx, locks...)
}

func (s *Store) assertLock(lock *TradeLockLedger, instanceID id.UUID, stackQty int, op TradeOp, qty int) error {
	if lock == nil {
		return nil
	}
	return lock.AssertOpAllowed(instanceID, stackQty, op, qty)
}

// assertSoulFree rejects while a character_souls row references the
// instance (items.md § Transfer/Use/Discard; trading_auction.md § Tradable
// Assets requires the Soul Contract resolved). Callers lock the owner
// character's soul rows first.
func (s *Store) assertSoulFree(ctx context.Context, tx pgx.Tx, instanceID id.UUID) error {
	var contracted bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM character_souls WHERE contracted_item_instance_id=$1)`,
		instanceID.String()).Scan(&contracted); err != nil {
		return err
	}
	if contracted {
		return ErrSoulContracted
	}
	return nil
}

// checkLocation asserts the destination-side preconditions after the
// destination locks were taken: inventory slot+capacity, guild slot,
// listing presence.
func (s *Store) checkLocation(ctx context.Context, tx pgx.Tx, loc Location) error {
	switch loc.Kind {
	case LocCharacterInventory:
		var capacity int
		err := tx.QueryRow(ctx,
			`SELECT capacity FROM character_inventories WHERE character_id=$1`,
			loc.CharacterID.String()).Scan(&capacity)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("items: no character_inventories row for %s", loc.CharacterID)
			}
			return err
		}
		var taken bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM item_locations
			  WHERE character_id=$1 AND location_kind='CHARACTER_INVENTORY' AND slot=$2)`,
			loc.CharacterID.String(), loc.Slot).Scan(&taken); err != nil {
			return err
		}
		if taken {
			return ErrSlotOccupied
		}
		var count int
		if err := tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM item_locations
			  WHERE character_id=$1 AND location_kind='CHARACTER_INVENTORY'`,
			loc.CharacterID.String()).Scan(&count); err != nil {
			return err
		}
		if count >= capacity {
			return ErrCapacity
		}
		return nil
	case LocGuildStorage:
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM guilds WHERE guild_id=$1)`,
			loc.GuildID.String()).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrInvalidLocation
		}
		var taken bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM item_locations
			  WHERE guild_id=$1 AND location_kind='GUILD_STORAGE' AND slot=$2)`,
			loc.GuildID.String(), loc.Slot).Scan(&taken); err != nil {
			return err
		}
		if taken {
			return ErrSlotOccupied
		}
		return nil
	case LocAuctionEscrow:
		var exists bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM auction_listings WHERE listing_id=$1)`,
			loc.ListingID.String()).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrInvalidLocation
		}
		return nil
	default:
		return nil
	}
}

func (s *Store) accountOf(ctx context.Context, tx pgx.Tx, characterID id.UUID) (id.UUID, error) {
	var a string
	err := tx.QueryRow(ctx, `SELECT account_id FROM characters WHERE character_id=$1`,
		characterID.String()).Scan(&a)
	if err != nil {
		return id.UUID{}, fmt.Errorf("items: account of %s: %w", characterID, err)
	}
	return id.ParseUUID(a)
}

func (s *Store) accountOfPool(ctx context.Context, characterID id.UUID) (id.UUID, error) {
	var a string
	err := s.pool.QueryRow(ctx, `SELECT account_id FROM characters WHERE character_id=$1`,
		characterID.String()).Scan(&a)
	if err != nil {
		return id.UUID{}, fmt.Errorf("items: account of %s: %w", characterID, err)
	}
	return id.ParseUUID(a)
}

// ---------------------------------------------------------------------------
// row I/O helpers
// ---------------------------------------------------------------------------

func updateLocation(ctx context.Context, tx pgx.Tx, instanceID id.UUID, loc Location) error {
	_, err := tx.Exec(ctx,
		`UPDATE item_locations SET location_kind=$2, character_id=$3, guild_id=$4,
		        listing_id=$5, slot=$6, depositor_character_id=$7,
		        depositor_account_id=$8, updated_at=NOW()
		  WHERE item_instance_id=$1`,
		instanceID.String(), string(loc.Kind), uuidOrNil(loc.CharacterID),
		uuidOrNil(loc.GuildID), uuidOrNil(loc.ListingID), strOrNil(loc.Slot),
		uuidOrNil(loc.DepositorCharacterID), uuidOrNil(loc.DepositorAccountID))
	return translateSlotErr(err)
}

func writeLocation(ctx context.Context, tx pgx.Tx, instanceID id.UUID, loc Location) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO item_locations
		 (item_instance_id, location_kind, character_id, guild_id, listing_id,
		  slot, depositor_character_id, depositor_account_id, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NOW())`,
		instanceID.String(), string(loc.Kind), uuidOrNil(loc.CharacterID),
		uuidOrNil(loc.GuildID), uuidOrNil(loc.ListingID), strOrNil(loc.Slot),
		uuidOrNil(loc.DepositorCharacterID), uuidOrNil(loc.DepositorAccountID))
	return translateSlotErr(err)
}

type scannable interface {
	Scan(dest ...any) error
}

func scanInstance(row scannable) (*Instance, error) {
	var (
		inst Instance
		bind string
	)
	err := row.Scan(&inst.InstanceID, &inst.ItemID, &inst.Quantity, &bind,
		&inst.EnhancementLevel, &inst.ItemState, &inst.ContentRevision, &inst.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUnknownItem
		}
		return nil, fmt.Errorf("items: instance: %w", err)
	}
	b, err := ParseBinding(bind)
	if err != nil {
		return nil, err
	}
	inst.EffectiveBinding = b
	return &inst, nil
}

func scanLocation(row pgx.Row) (*Location, error) {
	var (
		l                 Location
		kind              string
		ch, g, li, dc, da *string
		slot              *string
	)
	err := row.Scan(&kind, &ch, &g, &li, &slot, &dc, &da)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLocationConflict
		}
		return nil, fmt.Errorf("items: location: %w", err)
	}
	l.Kind = LocationKind(kind)
	if slot != nil {
		l.Slot = *slot
	}
	l.CharacterID = parseOptUUID(ch)
	l.GuildID = parseOptUUID(g)
	l.ListingID = parseOptUUID(li)
	l.DepositorCharacterID = parseOptUUID(dc)
	l.DepositorAccountID = parseOptUUID(da)
	return &l, nil
}

func parseOptUUID(s *string) id.UUID {
	if s == nil {
		return id.UUID{}
	}
	u, err := id.ParseUUID(*s)
	if err != nil {
		return id.UUID{}
	}
	return u
}

func uuidOrNil(u id.UUID) any {
	if u.IsNil() {
		return nil
	}
	return u.String()
}

func strOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func validateLocation(loc Location) error {
	switch loc.Kind {
	case LocCharacterInventory, LocEquipped, LocBeastEquipmentSlot:
		if loc.CharacterID.IsNil() || loc.Slot == "" {
			return ErrInvalidLocation
		}
	case LocGuildStorage:
		if loc.GuildID.IsNil() || loc.Slot == "" ||
			loc.DepositorCharacterID.IsNil() || loc.DepositorAccountID.IsNil() {
			return ErrInvalidLocation
		}
	case LocAuctionEscrow:
		if loc.ListingID.IsNil() {
			return ErrInvalidLocation
		}
	default:
		return ErrInvalidLocation
	}
	return nil
}

func translateSlotErr(err error) error {
	if err == nil {
		return nil
	}
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23505" {
		return ErrSlotOccupied
	}
	return err
}
