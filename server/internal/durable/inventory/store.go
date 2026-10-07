package inventory

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
)

// DBTX is the query surface shared by *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Store owns read access to the inventory/wallet/entitlement-panel
// projections and the row helpers the executors share. Snapshot reads
// are single consistent transactions (plan S1); the injected clock
// matches the durable package convention.
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

// State is the consistent inventory snapshot for one character.
type State struct {
	Capacity int32
	Revision int64
	Slots    []SlotView
	Loadouts []LoadoutRow
}

// SlotView is one inventory slot: empty (Item nil) or occupied.
// LockedQuantity is merged by the push builder from the injected
// trade-lock ledger; LoadState leaves it 0 (ADR-0064: 0 = none).
type SlotView struct {
	Slot           uint32
	Item           *ItemRow
	LockedQuantity uint32
}

// ItemRow is the persistent instance projection the wire
// ItemInstanceView is built from.
type ItemRow struct {
	InstanceID       id.UUID
	ItemID           string
	Quantity         int32
	EffectiveBinding string
	EnhancementLevel int32
	ContentRevision  string
	ItemState        []byte

	def     ItemDef // sort/merge working fields (executor scope only)
	origQty int32
}

// LoadoutRow is one character_loadouts row plus its EQUIPPED items.
type LoadoutRow struct {
	LoadoutID string // loadout.primary | loadout.secondary_1 | loadout.secondary_2
	IsActive  bool
	Revision  int64
	Slots     []LoadoutSlotRow
}

// LoadoutSlotRow is one equipped slot in a loadout.
type LoadoutSlotRow struct {
	SlotID string // slot.<name> wire id
	Item   *ItemRow
}

// WalletBalance is one character_currencies row projected for 432.
type WalletBalance struct {
	CurrencyID string
	Amount     int64
	Cap        int64
}

// Wallet is the 432 projection state: balances plus the ratified
// wallet_revision = SUM(character_currencies.revision) (data_model.md).
type Wallet struct {
	Balances []WalletBalance
	Revision int64
}

// EntitlementRow is one account_iap_entitlements row projected for 435
// plus this character's claimed tiers.
type EntitlementRow struct {
	EntitlementID     id.UUID
	ProductID         string
	EntitlementType   string
	GrantState        string
	SeasonNumber      int32
	ClaimDeadlineAtMs int64 // 0 when absent
	ClaimedTierIDs    []string
}

// loadoutIDs is the canonical loadout_index → loadout_id mapping
// (equipment.md § Loadouts).
var loadoutIDs = [3]string{"loadout.primary", "loadout.secondary_1", "loadout.secondary_2"}

// currencyCaps mirrors the canonical economy.md caps enforced by
// currency.Apply — duplicated here only for the 432 projection's Cap
// field (same values; keep in lockstep).
var currencyCaps = map[string]int64{
	"currency.common":  2_000_000_000,
	"currency.bound":   100_000_000,
	"currency.special": 1_000_000,
}

// loadState is the tx-scoped variant of LoadState shared by the
// executor-path push builders.
func (s *Store) loadState(ctx context.Context, db DBTX, characterID id.UUID) (State, error) {
	cap, rev, err := loadInventoryRow(ctx, db, characterID)
	if err != nil {
		return State{}, err
	}
	st := State{Capacity: cap, Revision: rev}
	if st.Slots, err = slotViews(ctx, db, characterID); err != nil {
		return State{}, err
	}
	if st.Loadouts, err = s.Loadouts(ctx, db, characterID); err != nil {
		return State{}, err
	}
	return st, nil
}

// LoadState reads the full 433 state inside one consistent
// read-transaction: the character_inventories row (lazily seeded on
// first contact — characters start with no row), every
// CHARACTER_INVENTORY slot, and the three character_loadouts rows with
// their EQUIPPED items (IMP-012 domain, read-only).
func (s *Store) LoadState(ctx context.Context, characterID id.UUID) (State, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return State{}, err
	}
	defer tx.Rollback(ctx)

	cap, rev, err := loadInventoryRow(ctx, tx, characterID)
	if err != nil {
		return State{}, err
	}
	st := State{Capacity: cap, Revision: rev}

	slots, err := slotViews(ctx, tx, characterID)
	if err != nil {
		return State{}, err
	}
	st.Slots = slots

	loads, err := s.Loadouts(ctx, tx, characterID)
	if err != nil {
		return State{}, err
	}
	st.Loadouts = loads
	return st, tx.Commit(ctx)
}

// Wallet reads the 432 projection in one consistent transaction:
// every character_currencies row plus SUM(revision).
func (s *Store) Wallet(ctx context.Context, characterID id.UUID) (Wallet, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Wallet{}, err
	}
	defer tx.Rollback(ctx)
	w, err := s.wallet(ctx, tx, characterID)
	if err != nil {
		return Wallet{}, err
	}
	return w, tx.Commit(ctx)
}

// wallet is the tx-scoped wallet read shared by pushes.
func (s *Store) wallet(ctx context.Context, db DBTX, characterID id.UUID) (Wallet, error) {
	rows, err := db.Query(ctx,
		`SELECT currency_id, balance, revision
		   FROM character_currencies WHERE character_id=$1
		  ORDER BY currency_id`, characterID.String())
	if err != nil {
		return Wallet{}, err
	}
	defer rows.Close()
	var w Wallet
	for rows.Next() {
		var (
			b   WalletBalance
			rev int64
		)
		if err := rows.Scan(&b.CurrencyID, &b.Amount, &rev); err != nil {
			return Wallet{}, err
		}
		b.Cap = capOf(b.CurrencyID)
		w.Balances = append(w.Balances, b)
		w.Revision += rev
	}
	return w, rows.Err()
}

// EntitlementPanel reads the 435 projection in one consistent
// transaction: every account_iap_entitlements row for the account plus
// this character's claimed reward_tier_ids (account_entitlement_claims,
// IMP-102's write table, read-only). Refund-revoked cosmetics are
// absent by definition: revoked rows change grant_state, and revoked
// character_cosmetic_entitlements rows are deleted by the refund path —
// the panel-equippable set therefore never includes them.
func (s *Store) EntitlementPanel(ctx context.Context, accountID, characterID id.UUID) ([]EntitlementRow, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := s.entitlementPanel(ctx, tx, accountID, characterID)
	if err != nil {
		return nil, err
	}
	return rows, tx.Commit(ctx)
}

func (s *Store) entitlementPanel(ctx context.Context, db DBTX, accountID, characterID id.UUID) ([]EntitlementRow, error) {
	rows, err := db.Query(ctx,
		`SELECT entitlement_id, product_id, entitlement_type, grant_state,
		        season_number, claim_deadline_at
		   FROM account_iap_entitlements
		  WHERE account_id=$1
		  ORDER BY entitlement_id`, accountID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EntitlementRow
	for rows.Next() {
		var (
			e        EntitlementRow
			season   *int32
			deadline *time.Time
		)
		if err := rows.Scan(&e.EntitlementID, &e.ProductID, &e.EntitlementType,
			&e.GrantState, &season, &deadline); err != nil {
			return nil, err
		}
		if season != nil {
			e.SeasonNumber = *season
		}
		if deadline != nil {
			e.ClaimDeadlineAtMs = deadline.UnixMilli()
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	rows.Close()

	claimed, err := db.Query(ctx,
		`SELECT account_entitlement_id, reward_tier_id
		   FROM account_entitlement_claims
		  WHERE account_id=$1 AND character_id=$2
		  ORDER BY reward_tier_id`, accountID.String(), characterID.String())
	if err != nil {
		return nil, err
	}
	defer claimed.Close()
	for claimed.Next() {
		var (
			entID id.UUID
			tier  string
		)
		if err := claimed.Scan(&entID, &tier); err != nil {
			return nil, err
		}
		for i := range out {
			if out[i].EntitlementID == entID {
				out[i].ClaimedTierIDs = append(out[i].ClaimedTierIDs, tier)
				break
			}
		}
	}
	return out, claimed.Err()
}

// capOf returns the canonical per-currency cap for the 432 projection
// (economy.md) — the same table currency.Apply enforces.
func capOf(currencyID string) int64 {
	if cap, ok := currencyCaps[currencyID]; ok {
		return cap
	}
	return 0
}

// Capacity reads character_inventories.capacity inside tx (the row is
// expected to be locked by the caller in mutation paths).
func (s *Store) Capacity(ctx context.Context, tx pgx.Tx, characterID id.UUID) (int32, error) {
	cap, _, err := loadInventoryRow(ctx, tx, characterID)
	return cap, err
}

// Loadouts reads the three loadout rows + their EQUIPPED items inside
// an existing tx (IMP-012 write domain, read-only). Missing rows are
// synthesized: loadout.primary defaults is_active (equipment.md requires
// exactly one ACTIVE).
func (s *Store) Loadouts(ctx context.Context, db DBTX, characterID id.UUID) ([]LoadoutRow, error) {
	rows := make([]LoadoutRow, 3)
	for i := range rows {
		rows[i].LoadoutID = loadoutIDs[i]
		rows[i].IsActive = i == 0
	}
	lrows, err := db.Query(ctx,
		`SELECT loadout_index, role, revision
		   FROM character_loadouts
		  WHERE character_id=$1
		  ORDER BY loadout_index`, characterID.String())
	if err != nil {
		return nil, err
	}
	defer lrows.Close()
	for lrows.Next() {
		var (
			idx  int16
			role string
			rev  int64
		)
		if err := lrows.Scan(&idx, &role, &rev); err != nil {
			return nil, err
		}
		if idx < 1 || idx > 3 {
			continue
		}
		rows[idx-1].IsActive = role == "ACTIVE"
		rows[idx-1].Revision = rev
	}
	if err := lrows.Err(); err != nil {
		return nil, err
	}

	equip, err := db.Query(ctx,
		`SELECT l.slot, i.item_instance_id, i.item_id, i.quantity,
		        i.effective_binding, i.enhancement_level, i.item_state,
		        i.content_revision
		   FROM item_locations l
		   JOIN item_instances i ON i.item_instance_id = l.item_instance_id
		  WHERE l.character_id=$1 AND l.location_kind='EQUIPPED'
		  ORDER BY l.slot`, characterID.String())
	if err != nil {
		return nil, err
	}
	defer equip.Close()
	for equip.Next() {
		var (
			slot     string
			item     ItemRow
			revision *string
		)
		if err := equip.Scan(&slot, &item.InstanceID, &item.ItemID, &item.Quantity,
			&item.EffectiveBinding, &item.EnhancementLevel, &item.ItemState, &revision); err != nil {
			return nil, err
		}
		if revision != nil {
			item.ContentRevision = *revision
		}
		loadID, slotID := splitEquippedSlot(slot)
		for i := range rows {
			if rows[i].LoadoutID == loadID {
				rows[i].Slots = append(rows[i].Slots, LoadoutSlotRow{SlotID: slotID, Item: &item})
				break
			}
		}
	}
	return rows, equip.Err()
}

// loadInventoryRow returns (capacity, revision); a missing row yields
// the lazy default (capacity 60, revision 0) — characters gain their
// row on the first committed inventory mutation.
func loadInventoryRow(ctx context.Context, db DBTX, characterID id.UUID) (int32, int64, error) {
	var (
		cap int32
		rev int64
	)
	err := db.QueryRow(ctx,
		`SELECT capacity, revision FROM character_inventories WHERE character_id=$1`,
		characterID.String()).Scan(&cap, &rev)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 60, 0, nil
		}
		return 0, 0, err
	}
	return cap, rev, nil
}

// ensureInventoryRow inserts the lazy character_inventories row inside
// a committing tx (capacity 60, revision 0). Callers hold the row lock.
func ensureInventoryRow(ctx context.Context, tx pgx.Tx, characterID id.UUID) (int32, int64, error) {
	var (
		cap int32
		rev int64
	)
	err := tx.QueryRow(ctx,
		`INSERT INTO character_inventories (character_id, capacity, revision)
		 VALUES ($1, 60, 0)
		 ON CONFLICT (character_id) DO UPDATE SET character_id = EXCLUDED.character_id
		 RETURNING capacity, revision`, characterID.String()).Scan(&cap, &rev)
	return cap, rev, err
}

// bumpRevision increments inventory_revision once per committed
// mutation (inventory.md § Revision / Idempotency).
func bumpRevision(ctx context.Context, tx pgx.Tx, characterID id.UUID) (int64, error) {
	var rev int64
	err := tx.QueryRow(ctx,
		`UPDATE character_inventories SET revision = revision + 1
		  WHERE character_id=$1 RETURNING revision`, characterID.String()).Scan(&rev)
	return rev, err
}

// slotViews reads every occupied CHARACTER_INVENTORY slot joined with
// its instance. slot strings are inv.<N>, N = wire slot (zero-based).
func slotViews(ctx context.Context, db DBTX, characterID id.UUID) ([]SlotView, error) {
	rows, err := db.Query(ctx,
		`SELECT l.slot, i.item_instance_id, i.item_id, i.quantity,
		        i.effective_binding, i.enhancement_level, i.item_state,
		        i.content_revision
		   FROM item_locations l
		   JOIN item_instances i ON i.item_instance_id = l.item_instance_id
		  WHERE l.character_id=$1 AND l.location_kind='CHARACTER_INVENTORY'
		  ORDER BY l.slot`, characterID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SlotView
	for rows.Next() {
		var (
			slotStr  string
			item     ItemRow
			revision *string
		)
		if err := rows.Scan(&slotStr, &item.InstanceID, &item.ItemID, &item.Quantity,
			&item.EffectiveBinding, &item.EnhancementLevel, &item.ItemState, &revision); err != nil {
			return nil, err
		}
		if revision != nil {
			item.ContentRevision = *revision
		}
		n, err := ParseSlot(slotStr)
		if err != nil {
			return nil, err
		}
		it := item
		out = append(out, SlotView{Slot: n, Item: &it})
	}
	return out, rows.Err()
}
