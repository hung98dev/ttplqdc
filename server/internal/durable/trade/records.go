// Package trade is the durable settlement surface for direct trade:
// the JournalTrade-bearing CLIENT708 executor, the settlement
// transaction (locks, revalidation, currency + fee + cap, item
// transfers, settlement record and economy rollups), and its store.
package trade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/currency"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Verdict is a terminal nonexecution the settlement transaction can
// raise before any mutation — the operation commits as a rejected
// outcome carrying the wire error code.
type Verdict struct {
	Code protocolv1.ErrorCode
}

func (v *Verdict) Error() string { return "trade verdict: " + v.Code.String() }

func verdict(code protocolv1.ErrorCode) *Verdict { return &Verdict{Code: code} }

// CatalogItem is the resolved catalog surface one offered item needs:
// its authored equipment tier (0 = not equipment) for the price floor.
type CatalogItem struct {
	ItemID string
	Tier   int
}

// Catalog resolves an item_id; composition injects the catalog-backed
// resolver (the item-catalog task ships it; 0 tier admits no floor).
type Catalog func(ctx context.Context, itemID string) (*CatalogItem, error)

// Store is the direct-trade durable surface.
type Store struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// New builds the store over the shared pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, now: time.Now}
}

// WithNow overrides the clock (tests).
func (s *Store) WithNow(now func() time.Time) *Store {
	c := *s
	c.now = now
	return &c
}

// SettleIn is the committed settlement input — the embedded
// JournalTrade plus the runtime seams.
type SettleIn struct {
	Trade       *journalv1.JournalTrade
	OperationID id.UUID
	Locks       func(charID id.UUID) *items.TradeLockLedger
	Catalog     Catalog
}

// SettleOut reports the committed transfer per participant.
type SettleOut struct {
	SettlementID  id.UUID
	TradeID       id.UUID
	InitiatorGot  *SideResult
	CounterGot    *SideResult
	Fee           int64
	ItemTransfers []ItemTransfer
}

// SideResult is one side's received view (its 709 payload fields).
type SideResult struct {
	Received       []*protocolv1.ItemGrant
	CommonReceived int64
}

// ItemTransfer is one moved (or split-moved) stack entry recorded in
// trade_settlement_records.item_transfers.
type ItemTransfer struct {
	FromCharacterID, ToCharacterID id.UUID
	ItemInstanceID                 id.UUID
	DeliveredInstanceID            id.UUID // == ItemInstanceID on a full move
	ItemID                         string
	Quantity                       int
}

type offerRow struct {
	instanceID id.UUID
	itemID     string
	quantity   int // offered
	stack      int // current stack quantity
	binding    string
	slot       string
}

type sideState struct {
	charID, accountID id.UUID
	common            int64
	offers            []offerRow
	balance           int64
}

const feePermille = int64(50) // floor(offered_common * 0.05)

func feeOf(offered int64) int64 {
	if offered <= 0 {
		return 0
	}
	return offered * feePermille / 1000
}

var equipmentTierFloors = map[int]int64{1: 500, 2: 1500, 3: 4000, 4: 10000, 5: 25000, 6: 50000}

// Settle commits the atomic two-sided exchange inside the caller's tx:
// canonical lockorder acquisition, full COMMITTING revalidation (the
// durable side is authoritative — the runtime re-check is advisory),
// currency moves with the offering-side fee sink, item transfers
// (split when the offered quantity is a partial stack, ADR-0062), the
// settlement record and the economy rollups.
func (s *Store) Settle(ctx context.Context, tx pgx.Tx, in SettleIn) (SettleOut, error) {
	j := in.Trade
	if j == nil || j.GetInitiator() == nil || j.GetCounterpart() == nil {
		return SettleOut{}, fmt.Errorf("trade: record without journal trade")
	}
	var tradeID, settlementID, initC, initA, cpartC, cpartA id.UUID
	if copy(tradeID[:], j.GetTradeId()) != 16 || copy(settlementID[:], j.GetSettlementId()) != 16 ||
		copy(initC[:], j.GetInitiator().GetCharacterId()) != 16 ||
		copy(initA[:], j.GetInitiator().GetAccountId()) != 16 ||
		copy(cpartC[:], j.GetCounterpart().GetCharacterId()) != 16 ||
		copy(cpartA[:], j.GetCounterpart().GetAccountId()) != 16 {
		return SettleOut{}, fmt.Errorf("trade: malformed journal identity")
	}
	if initC == cpartC || initA == cpartA {
		return SettleOut{}, verdict(protocolv1.ErrorCode_ERROR_CODE_SAME_ACCOUNT_FORBIDDEN)
	}
	now := s.now()
	locks := []lockorder.Lock{
		lockorder.RowLock("characters", initC), lockorder.RowLock("characters", cpartC),
		lockorder.RowLock("character_currencies", initC), lockorder.RowLock("character_currencies", cpartC),
		lockorder.RowLock("character_inventories", initC), lockorder.RowLock("character_inventories", cpartC),
	}
	for _, it := range append(j.GetInitiator().GetItems(), j.GetCounterpart().GetItems()...) {
		var iid id.UUID
		if copy(iid[:], it.GetItemInstanceId()) != 16 {
			return SettleOut{}, fmt.Errorf("trade: malformed offered instance id")
		}
		locks = append(locks, lockorder.RowLock("item_instances", iid), lockorder.RowLock("item_locations", iid))
	}
	locks = append(locks, lockorder.RowLock("character_souls", initC), lockorder.RowLock("character_souls", cpartC))
	if err := lockorder.SortLocks(locks); err != nil {
		return SettleOut{}, err
	}
	if err := lockorder.Acquire(ctx, tx, locks...); err != nil {
		return SettleOut{}, err
	}

	initSide, err := s.loadSide(ctx, tx, initC, initA, j.GetInitiator(), in.Locks, now)
	if err != nil {
		return SettleOut{}, err
	}
	cpartSide, err := s.loadSide(ctx, tx, cpartC, cpartA, j.GetCounterpart(), in.Locks, now)
	if err != nil {
		return SettleOut{}, err
	}
	if initSide.common > 0 && cpartSide.common > 0 {
		return SettleOut{}, verdict(protocolv1.ErrorCode_ERROR_CODE_TRADE_COMMON_BOTH_SIDES)
	}
	if initSide.common > initSide.balance || cpartSide.common > cpartSide.balance {
		return SettleOut{}, verdict(protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_CURRENCY)
	}

	// Equipment price floor: equipment offered one way may only be
	// received against at least the tier floor in common.
	if err := s.checkFloor(ctx, in.Catalog, initSide.offers, cpartSide.common); err != nil {
		return SettleOut{}, err
	}
	if err := s.checkFloor(ctx, in.Catalog, cpartSide.offers, initSide.common); err != nil {
		return SettleOut{}, err
	}

	// Currency cap: BOTH post-trade balances must stay within
	// 0 <= resulting <= cap — validated before any mutation; a breach
	// rejects the whole trade (trading_auction.md § Currency Cap).
	capCommon := currency.Caps[currency.Common]
	if cpartSide.balance+(initSide.common-feeOf(initSide.common)) > capCommon ||
		initSide.balance+(cpartSide.common-feeOf(cpartSide.common)) > capCommon {
		return SettleOut{}, verdict(protocolv1.ErrorCode_ERROR_CODE_CURRENCY_CAP_EXCEEDED)
	}

	// Currency moves: offerer debits the gross offer; the receiver is
	// credited the offer net of the offering-side fee — the fee delta
	// is the DIRECT_TRADE_FEE sink.
	fee := feeOf(initSide.common) + feeOf(cpartSide.common)
	if initSide.common > 0 {
		if err := s.moveCommon(ctx, tx, initSide.charID, cpartSide.charID, initSide.common, in.OperationID, settlementID); err != nil {
			return SettleOut{}, err
		}
	}
	if cpartSide.common > 0 {
		if err := s.moveCommon(ctx, tx, cpartSide.charID, initSide.charID, cpartSide.common, in.OperationID, settlementID); err != nil {
			return SettleOut{}, err
		}
	}

	// Item transfers: full stacks move the instance; partial stacks
	// split the offered quantity into a fresh instance at the
	// receiver's next free inventory slot (ADR-0062).
	transfers, err := s.transferItems(ctx, tx, initSide, cpartSide.charID, in.OperationID, now)
	if err != nil {
		return SettleOut{}, err
	}
	rev, err := s.transferItems(ctx, tx, cpartSide, initSide.charID, in.OperationID, now)
	if err != nil {
		return SettleOut{}, err
	}
	transfers = append(transfers, rev...)

	if err := s.insertSettlement(ctx, tx, settlementID, tradeID, initSide, cpartSide, in.OperationID, transfers, now); err != nil {
		return SettleOut{}, err
	}
	if err := s.rollups(ctx, tx, initSide, cpartSide, transfers, now); err != nil {
		return SettleOut{}, err
	}

	return SettleOut{
		SettlementID:  settlementID,
		TradeID:       tradeID,
		Fee:           fee,
		ItemTransfers: transfers,
		InitiatorGot:  sideResult(cpartSide),
		CounterGot:    sideResult(initSide),
	}, nil
}

// sideResult maps what `of` offered to the receiver's view.
func sideResult(of *sideState) *SideResult {
	r := &SideResult{CommonReceived: of.common - feeOf(of.common)}
	for _, o := range of.offers {
		r.Received = append(r.Received, &protocolv1.ItemGrant{
			ItemInstanceId: o.instanceID[:], ItemId: o.itemID, Quantity: uint32(o.quantity),
		})
	}
	return r
}

// loadSide locks in the committed side payload: character row
// (eligibility re-check), balance, and every offered stack
// (ownership/location/quantity/binding/soul/ledger).
func (s *Store) loadSide(ctx context.Context, tx pgx.Tx, charID, accountID id.UUID,
	js *journalv1.JournalTradeSide, locks func(id.UUID) *items.TradeLockLedger, now time.Time) (*sideState, error) {
	st := &sideState{charID: charID, accountID: accountID, common: js.GetCommonAmount()}
	if st.common < 0 || len(js.GetItems()) > 12 {
		return nil, fmt.Errorf("trade: malformed side payload")
	}
	var level int
	var created time.Time
	var acct id.UUID
	if err := tx.QueryRow(ctx,
		`SELECT account_id, level, created_at FROM characters WHERE character_id=$1`,
		charID.String()).Scan(&acct, &level, &created); err != nil {
		return nil, fmt.Errorf("trade: participant: %w", err)
	}
	if acct != accountID {
		return nil, verdict(protocolv1.ErrorCode_ERROR_CODE_SAME_ACCOUNT_FORBIDDEN)
	}
	if level < 10 {
		return nil, verdict(protocolv1.ErrorCode_ERROR_CODE_TRADE_ELIGIBILITY_LEVEL_REQUIRED)
	}
	if now.Sub(created) < 24*time.Hour {
		return nil, verdict(protocolv1.ErrorCode_ERROR_CODE_TRADE_ELIGIBILITY_AGE_REQUIRED)
	}
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE((SELECT balance FROM character_currencies
		   WHERE character_id=$1 AND currency_id='currency.common'), 0)`,
		charID.String()).Scan(&st.balance); err != nil {
		return nil, fmt.Errorf("trade: balance: %w", err)
	}
	ledger := locks(charID)
	for _, jt := range js.GetItems() {
		var iid id.UUID
		if copy(iid[:], jt.GetItemInstanceId()) != 16 {
			return nil, fmt.Errorf("trade: malformed offered instance id")
		}
		row := offerRow{instanceID: iid, itemID: jt.GetItemId(), quantity: int(jt.GetQuantity())}
		if row.quantity < 1 {
			return nil, fmt.Errorf("trade: nonpositive offered quantity")
		}
		var locKind string
		var locChar *string
		if err := tx.QueryRow(ctx,
			`SELECT i.quantity, i.effective_binding, l.location_kind, l.character_id::text, l.slot
			   FROM item_instances i JOIN item_locations l ON l.item_instance_id = i.item_instance_id
			  WHERE i.item_instance_id=$1`, iid.String()).
			Scan(&row.stack, &row.binding, &locKind, &locChar, &row.slot); err != nil {
			return nil, fmt.Errorf("trade: offered instance: %w", err)
		}
		if locKind != string(items.LocCharacterInventory) || locChar == nil || *locChar != charID.String() {
			return nil, verdict(protocolv1.ErrorCode_ERROR_CODE_ITEM_LOCKED)
		}
		if row.stack < row.quantity {
			return nil, verdict(protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_ITEM)
		}
		if row.binding != items.BindingUnbound.String() {
			return nil, verdict(protocolv1.ErrorCode_ERROR_CODE_ITEM_LOCKED)
		}
		var contracted bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM character_souls WHERE contracted_item_instance_id=$1)`,
			iid.String()).Scan(&contracted); err != nil {
			return nil, fmt.Errorf("trade: soul contract: %w", err)
		}
		if contracted {
			return nil, verdict(protocolv1.ErrorCode_ERROR_CODE_ITEM_LOCKED)
		}
		if ledger == nil || ledger.LockedQty(iid) != row.quantity {
			return nil, verdict(protocolv1.ErrorCode_ERROR_CODE_ITEM_LOCKED)
		}
		st.offers = append(st.offers, row)
	}
	return st, nil
}

// checkFloor applies the equipment price floor against the receiving
// side's common offer.
func (s *Store) checkFloor(ctx context.Context, cat Catalog, offers []offerRow, common int64) error {
	var floor int64
	for _, o := range offers {
		if cat == nil {
			continue
		}
		ci, err := cat(ctx, o.itemID)
		if err != nil {
			return fmt.Errorf("trade: catalog: %w", err)
		}
		if ci != nil {
			floor += equipmentTierFloors[ci.Tier]
		}
	}
	if floor > common {
		return verdict(protocolv1.ErrorCode_ERROR_CODE_TRADE_PRICE_FLOOR_NOT_MET)
	}
	return nil
}

// moveCommon debits `gross` from `from` and credits `gross-fee` to
// `to`; the fee is the sink (recorded in the settlement row).
func (s *Store) moveCommon(ctx context.Context, tx pgx.Tx, from, to id.UUID,
	gross int64, opID, settlementID id.UUID) error {
	m := currency.Mutation{
		CurrencyID:  currency.Common,
		OperationID: opID,
		ReasonCode:  "DIRECT_TRADE",
		SourceRef:   settlementID.String(),
		Actor:       currency.ActorPlayer,
	}
	m.CharacterID, m.Delta = from, gross
	if _, err := currency.Debit(ctx, tx, m); err != nil {
		if errors.Is(err, currency.ErrInsufficientBalance) {
			return verdict(protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_CURRENCY)
		}
		return fmt.Errorf("trade: debit: %w", err)
	}
	m.CharacterID, m.Delta = to, gross-feeOf(gross)
	if _, err := currency.Credit(ctx, tx, m); err != nil {
		if errors.Is(err, currency.ErrCapExceeded) {
			return verdict(protocolv1.ErrorCode_ERROR_CODE_CURRENCY_CAP_EXCEEDED)
		}
		return fmt.Errorf("trade: credit: %w", err)
	}
	return nil
}

// transferItems moves every offered stack of `from` to `to`'s
// inventory, splitting partial offers into fresh instances.
func (s *Store) transferItems(ctx context.Context, tx pgx.Tx, from *sideState,
	to id.UUID, opID id.UUID, now time.Time) ([]ItemTransfer, error) {
	if len(from.offers) == 0 {
		return nil, nil
	}
	free, err := s.freeSlots(ctx, tx, to)
	if err != nil {
		return nil, err
	}
	var out []ItemTransfer
	for _, o := range from.offers {
		if len(free) == 0 {
			return nil, verdict(protocolv1.ErrorCode_ERROR_CODE_INVENTORY_FULL)
		}
		slot := free[0]
		free = free[1:]
		tr := ItemTransfer{
			FromCharacterID: from.charID, ToCharacterID: to,
			ItemInstanceID: o.instanceID, DeliveredInstanceID: o.instanceID,
			ItemID: o.itemID, Quantity: o.quantity,
		}
		if o.quantity == o.stack {
			res, err := tx.Exec(ctx,
				`UPDATE item_locations SET character_id=$2, slot=$3, updated_at=$4
				  WHERE item_instance_id=$1 AND location_kind='CHARACTER_INVENTORY' AND character_id=$5`,
				o.instanceID.String(), to.String(), slot, now, from.charID.String())
			if err != nil {
				return nil, fmt.Errorf("trade: move: %w", err)
			}
			if res.RowsAffected() != 1 {
				return nil, verdict(protocolv1.ErrorCode_ERROR_CODE_ITEM_LOCKED)
			}
		} else {
			newID := id.NewV4()
			if _, err := tx.Exec(ctx,
				`INSERT INTO item_instances
				 (item_instance_id, item_id, quantity, effective_binding, enhancement_level,
				  item_state, content_revision, created_at)
				 SELECT $1, item_id, $2, effective_binding, enhancement_level,
				        item_state, content_revision, created_at
				   FROM item_instances WHERE item_instance_id=$3`,
				newID.String(), o.quantity, o.instanceID.String()); err != nil {
				return nil, fmt.Errorf("trade: split insert: %w", err)
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO item_locations (item_instance_id, location_kind, character_id, slot, updated_at)
				 VALUES ($1,'CHARACTER_INVENTORY',$2,$3,$4)`,
				newID.String(), to.String(), slot, now); err != nil {
				return nil, fmt.Errorf("trade: split location: %w", err)
			}
			res, err := tx.Exec(ctx,
				`UPDATE item_instances SET quantity=quantity-$2 WHERE item_instance_id=$1 AND quantity>=$2`,
				o.instanceID.String(), o.quantity)
			if err != nil {
				return nil, fmt.Errorf("trade: split reduce: %w", err)
			}
			if res.RowsAffected() != 1 {
				return nil, verdict(protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_ITEM)
			}
			tr.DeliveredInstanceID = newID
		}
		out = append(out, tr)
	}
	return out, nil
}

// freeSlots lists the receiver's unoccupied inv.<n> slots in order.
func (s *Store) freeSlots(ctx context.Context, tx pgx.Tx, charID id.UUID) ([]string, error) {
	var capacity int
	if err := tx.QueryRow(ctx,
		`SELECT capacity FROM character_inventories WHERE character_id=$1`,
		charID.String()).Scan(&capacity); err != nil {
		return nil, fmt.Errorf("trade: inventory: %w", err)
	}
	used := map[string]bool{}
	rows, err := tx.Query(ctx,
		`SELECT slot FROM item_locations WHERE location_kind='CHARACTER_INVENTORY' AND character_id=$1`,
		charID.String())
	if err != nil {
		return nil, fmt.Errorf("trade: slots: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var sl string
		if err := rows.Scan(&sl); err != nil {
			return nil, err
		}
		used[sl] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var free []string
	for i := 0; i < capacity; i++ {
		sl := fmt.Sprintf("inv.%d", i)
		if !used[sl] {
			free = append(free, sl)
		}
	}
	return free, nil
}

// insertSettlement writes the trade_settlement_records row.
func (s *Store) insertSettlement(ctx context.Context, tx pgx.Tx,
	settlementID, tradeID id.UUID, init, cpart *sideState, opID id.UUID,
	transfers []ItemTransfer, now time.Time) error {
	payload, err := marshalTransfers(transfers)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx,
		`INSERT INTO trade_settlement_records
		 (settlement_id, settled_at, initiator_character_id, initiator_account_id,
		  counterpart_character_id, counterpart_account_id,
		  common_sent_by_initiator, common_sent_by_counterpart, trade_id,
		  settlement_operation_id, item_transfers)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb)`,
		settlementID.String(), now, init.charID.String(), init.accountID.String(),
		cpart.charID.String(), cpart.accountID.String(),
		init.common, cpart.common, tradeID.String(), opID.String(), payload)
	if err != nil {
		return fmt.Errorf("trade: settlement row: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return verdict(protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT)
	}
	return nil
}

// marshalTransfers serializes the item_transfers JSONB payload.
func marshalTransfers(ts []ItemTransfer) ([]byte, error) {
	type rec struct {
		From      string `json:"from_character_id"`
		To        string `json:"to_character_id"`
		Source    string `json:"item_instance_id"`
		Delivered string `json:"delivered_instance_id"`
		ItemID    string `json:"item_id"`
		Quantity  int    `json:"quantity"`
	}
	out := make([]rec, 0, len(ts))
	for _, t := range ts {
		out = append(out, rec{
			From:      t.FromCharacterID.String(),
			To:        t.ToCharacterID.String(),
			Source:    t.ItemInstanceID.String(),
			Delivered: t.DeliveredInstanceID.String(),
			ItemID:    t.ItemID,
			Quantity:  t.Quantity,
		})
	}
	return json.Marshal(out)
}

// rollups upserts the per-character and per-account daily economy
// rollups the settlement produced (data_model.md § economy rollups).
func (s *Store) rollups(ctx context.Context, tx pgx.Tx, init, cpart *sideState,
	transfers []ItemTransfer, now time.Time) error {
	day := now.UTC().Truncate(24 * time.Hour)
	var inItems, outItems int
	for _, t := range transfers {
		if t.ToCharacterID == init.charID {
			inItems++
		} else {
			outItems++
		}
	}
	// character rollups: common flow + per-partner volumes/counts.
	if err := charRollup(ctx, tx, init, cpart.charID, cpart.common-feeOf(cpart.common), init.common, outItems, inItems, day, now); err != nil {
		return err
	}
	if err := charRollup(ctx, tx, cpart, init.charID, init.common-feeOf(init.common), cpart.common, inItems, outItems, day, now); err != nil {
		return err
	}
	// account rollups: plain common flow.
	if err := acctRollup(ctx, tx, init.accountID, init.common, cpart.common-feeOf(cpart.common), day, now); err != nil {
		return err
	}
	return acctRollup(ctx, tx, cpart.accountID, cpart.common, init.common-feeOf(init.common), day, now)
}

// charRollup upserts one character's daily row: outflow = what they
// sent (gross), inflow = what they received (net), partner volumes and
// item counts accumulate into the jsonb maps keyed by the partner.
func charRollup(ctx context.Context, tx pgx.Tx, self *sideState, partner id.UUID,
	received, sent int64, itemsOut, itemsIn int, day, now time.Time) error {
	pid := partner.String()
	_, err := tx.Exec(ctx,
		`INSERT INTO economy_character_daily_rollups
		 (character_id, utc_day, common_outflow, common_inflow,
		  trade_partner_volumes, item_partner_counts, updated_at)
		 VALUES ($1,$2,$3,$4, jsonb_build_object($5::text,$6::bigint),
		         jsonb_build_object($5::text,$7::bigint), $8)
		 ON CONFLICT (character_id, utc_day) DO UPDATE SET
		   common_outflow = economy_character_daily_rollups.common_outflow + EXCLUDED.common_outflow,
		   common_inflow  = economy_character_daily_rollups.common_inflow  + EXCLUDED.common_inflow,
		   trade_partner_volumes = jsonb_set(
		     COALESCE(economy_character_daily_rollups.trade_partner_volumes,'{}'::jsonb),
		     ARRAY[$5::text],
		     to_jsonb(COALESCE((economy_character_daily_rollups.trade_partner_volumes->>$5)::bigint,0) + $6::bigint)),
		   item_partner_counts = jsonb_set(
		     COALESCE(economy_character_daily_rollups.item_partner_counts,'{}'::jsonb),
		     ARRAY[$5::text],
		     to_jsonb(COALESCE((economy_character_daily_rollups.item_partner_counts->>$5)::bigint,0) + $7::bigint)),
		   updated_at = $8`,
		self.charID.String(), day, sent, received, pid, sent, int64(itemsOut+itemsIn), now)
	if err != nil {
		return fmt.Errorf("trade: character rollup: %w", err)
	}
	return nil
}

// acctRollup upserts one account's daily common flow.
func acctRollup(ctx context.Context, tx pgx.Tx, accountID id.UUID, sent, received int64,
	day, now time.Time) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO economy_account_daily_rollups
		 (account_id, utc_day, common_outflow, common_inflow, updated_at)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (account_id, utc_day) DO UPDATE SET
		   common_outflow = economy_account_daily_rollups.common_outflow + EXCLUDED.common_outflow,
		   common_inflow  = economy_account_daily_rollups.common_inflow  + EXCLUDED.common_inflow,
		   updated_at = $5`,
		accountID.String(), day, sent, received, now)
	if err != nil {
		return fmt.Errorf("trade: account rollup: %w", err)
	}
	return nil
}
