package auction

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/items"
	"thinhthan/internal/durable/lockorder"
)

// LookupItem is the resolved catalog surface one listing/purchase needs:
// tradability, NPC base buy price (0 when unset — trading_auction.md §
// min_listing_price), equipment tier/category for floors and search.
type LookupItem struct {
	ItemID          string
	Kind            items.Kind
	Tier            int    // authored equipment tier 1..6 (0 = not equipment)
	Category        string // item catalog type (MATERIAL/CONSUMABLE/...)
	NPCBaseBuyPrice int64  // 0 when the item has no NPC purchase value
	Tradable        bool   // item-level trade rule (items.md § Tradable)
}

// IsEquipment reports whether the tier-floor schedule applies.
func (l *LookupItem) IsEquipment() bool { return l.Tier >= 1 && l.Tier <= 6 }

// Lookup resolves an item_id to its auction surface. Composition injects
// the catalog-backed resolver; npc_base_buy_price arrives with the shop
// content wave — until then it resolves 0 and the floor bottoms at 100.
type Lookup func(ctx context.Context, itemID string) (*LookupItem, error)

// Store is the auction durable surface. Every mutating method runs
// inside the caller's committing transaction and takes its row locks via
// lockorder; the executor adapters run them under the durable queue.
type Store struct {
	pool  *pgxpool.Pool
	items *items.Store
	defs  Lookup
	now   func() time.Time
}

// New builds the store over the shared pool.
func New(pool *pgxpool.Pool, it *items.Store, defs Lookup) *Store {
	return &Store{pool: pool, items: it, defs: defs, now: time.Now}
}

// WithNow overrides the clock (tests; morning-market window cases).
func (s *Store) WithNow(now func() time.Time) *Store {
	c := *s
	c.now = now
	return &c
}

// Listing is one auction_listings row.
type Listing struct {
	ListingID         id.UUID
	SellerCharacterID id.UUID
	SellerAccountID   id.UUID
	ItemInstanceID    id.UUID
	ItemID            string
	Quantity          int64
	PriceCommon       int64
	ListingFeeCommon  int64
	State             ListingState
	ListedAt          time.Time
	ExpiresAt         time.Time
	EndedAt           *time.Time
	BuyerCharacterID  *id.UUID
	Revision          int64
}

// Proceeds is one auction_proceeds row (seller settlement escrow).
type Proceeds struct {
	ProceedsID        id.UUID
	SettledAt         time.Time
	SellerCharacterID id.UUID
	SellerAccountID   id.UUID
	BuyerCharacterID  id.UUID
	BuyerAccountID    id.UUID
	ProceedsAmount    int64
	ItemID            string
	Quantity          int64
	ListingID         id.UUID
	State             ProceedsState
	ClaimedAt         *time.Time
	ClaimOperationID  *id.UUID
}

// characterGate loads level/age under the characters row lock and applies
// the auction eligibility gates (every op: level>=15; listing adds age).
func (s *Store) gate(ctx context.Context, tx pgx.Tx, characterID id.UUID, listing bool) error {
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", characterID)); err != nil {
		return err
	}
	var level int
	var createdAt time.Time
	if err := tx.QueryRow(ctx,
		`SELECT level, created_at FROM characters WHERE character_id=$1`,
		characterID.String()).Scan(&level, &createdAt); err != nil {
		return fmt.Errorf("auction: load character gates: %w", err)
	}
	if level < LevelGate {
		return ErrLevelGate
	}
	if listing && s.now().Sub(createdAt) < time.Duration(AgeGateHours)*time.Hour {
		return ErrAgeGate
	}
	return nil
}

// loadForUpdate takes the auction_listings row lock and returns the row.
func (s *Store) loadForUpdate(ctx context.Context, tx pgx.Tx, listingID id.UUID) (*Listing, error) {
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("auction_listings", listingID)); err != nil {
		return nil, err
	}
	return s.load(ctx, tx, listingID)
}

func (s *Store) load(ctx context.Context, tx pgx.Tx, listingID id.UUID) (*Listing, error) {
	var l Listing
	var buyer *string
	err := tx.QueryRow(ctx, `SELECT listing_id, seller_character_id, seller_account_id,
		item_instance_id, item_id, quantity, price_common, listing_fee_common,
		state, listed_at, expires_at, ended_at, buyer_character_id, revision
	  FROM auction_listings WHERE listing_id=$1`, listingID.String()).Scan(
		&l.ListingID, &l.SellerCharacterID, &l.SellerAccountID,
		&l.ItemInstanceID, &l.ItemID, &l.Quantity, &l.PriceCommon, &l.ListingFeeCommon,
		&l.State, &l.ListedAt, &l.ExpiresAt, &l.EndedAt, &buyer, &l.Revision)
	if err != nil {
		return nil, fmt.Errorf("auction: load listing: %w", err)
	}
	if buyer != nil {
		b, perr := id.ParseUUID(*buyer)
		if perr != nil {
			return nil, perr
		}
		l.BuyerCharacterID = &b
	}
	return &l, nil
}

// transition updates state + ended_at per ADR-0070: ended_at is set on
// every exit from ACTIVE and overwritten on RECLAIMED / MOVED_TO_CLAIM,
// so it always carries the time of the latest state change.
func (s *Store) transition(ctx context.Context, tx pgx.Tx, listingID id.UUID,
	to ListingState, buyer *id.UUID, at time.Time) error {
	if to == StateActive {
		return fmt.Errorf("auction: cannot transition back to ACTIVE")
	}
	_, err := tx.Exec(ctx, `UPDATE auction_listings
	   SET state=$2, ended_at=$3, buyer_character_id=$4, revision=revision+1
	 WHERE listing_id=$1`, listingID.String(), string(to), at, buyer)
	return err
}
