package reward

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/currency"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/inventory"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Deps are the claim-executor dependencies the composition injects.
// Defs resolves item_id to its runtime definition (catalog binding);
// nil Defs is fail-closed (ITEM_NOT_FOUND via the shared inventory
// contract).
type Deps struct {
	Store *Store
	Items *items.Store
	Inv   *inventory.Store
	Defs  inventory.Defs
}

// Executors returns the family→executor map the composition
// ProducerClient family-mux merges; the package never self-registers.
func Executors(d Deps) map[string]queue.Executor {
	return map[string]queue.Executor{ClaimFamily: d.claimExecutor}
}

// claimExecutor applies C2S_REWARD_CLAIM (408) inside the committing
// transaction: canonical locks first (characters → character_currencies
// → character_inventories — item rows lock through items primitives,
// the claim row locks last per Rewards priority), ownership/state
// checks, the bounded batch, then the 409 outcome.
func (d Deps) claimExecutor(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != ClaimFamily {
		return idempotency.Outcome{}, fmt.Errorf("reward: unsupported client family %q", rec.GetOperationFamily())
	}
	charID, opID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	req := rec.GetClient().GetC2SRewardClaim()
	if req == nil || len(req.GetRewardClaimId()) != 16 {
		return idempotency.Outcome{}, fmt.Errorf("reward: record without c2s_reward_claim")
	}
	var claimID id.UUID
	copy(claimID[:], req.GetRewardClaimId())

	locks := []lockorder.Lock{
		lockorder.RowLock("characters", charID),
		lockorder.RowLock("character_currencies", charID),
		lockorder.RowLock("character_inventories", charID),
	}
	if err := lockorder.SortLocks(locks); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := lockorder.Acquire(ctx, tx, locks...); err != nil {
		return idempotency.Outcome{}, err
	}

	claim, err := d.Store.loadClaimForUpdate(ctx, tx, claimID)
	if err != nil {
		if errors.Is(err, ErrClaimNotFound) {
			return d.verdict(opID, claimID, ErrClaimNotFound, protocolv1.ErrorCode_ERROR_CODE_NOT_OWNER)
		}
		return idempotency.Outcome{}, err
	}
	switch {
	case claim.OwnerCharacterID != charID:
		return d.verdict(opID, claimID, ErrNotOwner, protocolv1.ErrorCode_ERROR_CODE_NOT_OWNER)
	case claim.State == StateExpired:
		return d.verdict(opID, claimID, ErrClaimExpired, protocolv1.ErrorCode_ERROR_CODE_EXPIRED)
	case claim.State != StatePending:
		// CLAIMED replays surface through the idempotency receipt; a
		// new operation on a consumed claim conflicts (CLAIMING cannot
		// persist outside a tx).
		return d.verdict(opID, claimID, ErrStateConflict, protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT)
	}

	res, err := d.deliver(ctx, tx, claim, opID)
	if err != nil {
		// Domain verdicts commit the 409 error; unexpected (SQL)
		// failures abort the tx so the command never resolves as a
		// committed outcome.
		if code, ok := claimErrCode(err); ok {
			return d.verdict(opID, claimID, err, code)
		}
		return idempotency.Outcome{}, err
	}
	if err := d.Store.markDelivered(ctx, tx, claim, opID, res.claimed); err != nil {
		return idempotency.Outcome{}, err
	}
	if _, err := d.Store.bumpClaimsRevision(ctx, tx, charID); err != nil {
		return idempotency.Outcome{}, err
	}
	return commitOutcome(opID, claimID, res)
}

// delivery accumulates one batch's results for the 409 receipt.
type delivery struct {
	granted    []*protocolv1.ItemGrant
	currencies []*protocolv1.CurrencyDelta
	lines      []*protocolv1.RewardClaimDeliveredLine
	claimed    bool
}

// deliver runs the bounded batch: SINGLE revalidates complete capacity
// and delivers all-or-nothing; aggregates deliver the largest fitting
// amount (currency: cap headroom; items: compatible stacks then empty
// slots ascending), remainders stay PENDING (reward_claims.md §
// Delivery).
func (d Deps) deliver(ctx context.Context, tx pgx.Tx, c *Claim, opID id.UUID) (*delivery, error) {
	switch c.Kind {
	case ClaimKindSingle:
		return d.deliverSingle(ctx, tx, c, opID)
	default:
		return d.deliverBatch(ctx, tx, c, opID)
	}
}

// deliverSingle validates every line fits then applies it; failure
// mutates nothing (validated up front — items writes only after all
// fits pass).
func (d Deps) deliverSingle(ctx context.Context, tx pgx.Tx, c *Claim, opID id.UUID) (*delivery, error) {
	plan := &fitPlan{charID: c.OwnerCharacterID}
	for _, l := range c.Lines {
		switch l.Kind {
		case lineItem:
			fit, err := d.fitItem(ctx, tx, c.OwnerCharacterID, l, l.Quantity)
			if err != nil {
				return nil, err
			}
			// All-or-nothing: a partial fit rejects the whole claim
			// (reward_claims.md § Delivery — SINGLE delivers nothing
			// until every line fits).
			if fit.total.Cmp(l.Quantity) < 0 {
				return nil, ErrInventoryFull
			}
			plan.items = append(plan.items, fit)
		case lineCurrency:
			head, err := d.currencyHeadroom(ctx, tx, c.OwnerCharacterID, l.CurrencyID)
			if err != nil {
				return nil, err
			}
			if head.Cmp(l.Amount) < 0 {
				return nil, ErrCurrencyCapReached
			}
		}
	}
	res := &delivery{claimed: true}
	for _, l := range c.Lines {
		if l.Kind == lineCurrency {
			if err := d.creditLine(ctx, tx, c.OwnerCharacterID, l.CurrencyID, l.Amount, opID, c, res, l.LineNo); err != nil {
				return nil, err
			}
		}
	}
	for i, l := range c.Lines {
		if l.Kind == lineItem {
			if err := d.applyItemFit(ctx, tx, plan.items[fitIdx(i, c)], opID, res, l); err != nil {
				return nil, err
			}
		}
	}
	return res, nil
}

// deliverBatch applies the deterministic bounded batch for aggregate
// kinds: each line delivers min(remainder, capacity) in line order;
// capacity is recomputed under the held locks.
func (d Deps) deliverBatch(ctx context.Context, tx pgx.Tx, c *Claim, opID id.UUID) (*delivery, error) {
	res := &delivery{}
	anyDelivered := false
	for _, l := range c.Lines {
		rem := l.Remaining()
		if rem.Sign() == 0 {
			continue
		}
		switch l.Kind {
		case lineCurrency:
			head, err := d.currencyHeadroom(ctx, tx, c.OwnerCharacterID, l.CurrencyID)
			if err != nil {
				return nil, err
			}
			batch := rem
			if head.Cmp(rem) < 0 {
				batch = head
			}
			if batch.Sign() == 0 {
				continue
			}
			if err := d.creditLine(ctx, tx, c.OwnerCharacterID, l.CurrencyID, batch, opID, c, res, l.LineNo); err != nil {
				return nil, err
			}
			anyDelivered = true
		case lineItem:
			fit, err := d.fitItem(ctx, tx, c.OwnerCharacterID, l, rem)
			if err != nil {
				return nil, err
			}
			if fit.total.Sign() == 0 {
				continue
			}
			if err := d.applyItemFit(ctx, tx, fit, opID, res, l); err != nil {
				return nil, err
			}
			anyDelivered = true
		}
	}
	if !anyDelivered {
		if c.Kind == ClaimKindCurrencyAggregate {
			return nil, ErrCurrencyCapReached
		}
		return nil, ErrInventoryFull
	}
	res.claimed = claimFullyDelivered(c)
	return res, nil
}

// claimFullyDelivered recomputes the post-batch remainder from the
// mutated line counters (in-memory mirror of the committed rows).
func claimFullyDelivered(c *Claim) bool {
	for _, l := range c.Lines {
		if l.Remaining().Sign() > 0 {
			return false
		}
	}
	return true
}

// fitIdx is a placeholder removed by fitPlan ordering — retained for
// symmetry in the single pass.
func fitIdx(i int, c *Claim) int {
	idx := 0
	for j := 0; j < i; j++ {
		if c.Lines[j].Kind == lineItem {
			idx++
		}
	}
	return idx
}

// fitPlan holds the computed per-line item deliveries.
type fitPlan struct {
	charID id.UUID
	items  []*itemFit
}

// itemFit is one line's computed item delivery: existing-stack top-ups
// then new-slot creates, in ascending slot order.
type itemFit struct {
	charID  id.UUID
	line    *Line
	topUps  []topUp
	creates []createAt
	total   *big.Int
}

// topUp adds quantity onto an existing compatible stack.
type topUp struct {
	instanceID id.UUID
	qty        int64
}

// createAt materializes a fresh stack at wire slot n.
type createAt struct {
	slot uint32
	qty  int64
}

// fitItem computes how much of `want` fits: spare room on compatible
// existing stacks first (same item_id + effective_binding, quantity <
// maxStack), then empty slots ascending — each new stack holds at most
// maxStack.
func (d Deps) fitItem(ctx context.Context, tx pgx.Tx, charID id.UUID,
	l *Line, want *big.Int) (*itemFit, error) {
	if d.Defs == nil {
		return nil, fmt.Errorf("reward: no catalog resolver")
	}
	idef, err := d.Defs(ctx, l.ItemID)
	if err != nil {
		return nil, err
	}
	def := items.ApplyDefinitionDefaults(idef.Def)
	maxStack := int64(def.MaxStack)
	if maxStack <= 0 || maxStack > int64(items.MaxStackCeiling) {
		maxStack = int64(items.MaxStackCeiling)
	}
	fit := &itemFit{charID: charID, line: l, total: new(big.Int)}

	rows, err := tx.Query(ctx,
		`SELECT il.slot, ii.item_instance_id, ii.quantity
		 FROM item_locations il JOIN item_instances ii
		   ON ii.item_instance_id = il.item_instance_id
		 WHERE il.character_id = $1 AND il.location_kind = 'CHARACTER_INVENTORY'
		   AND ii.item_id = $2 AND ii.effective_binding = $3`,
		charID, l.ItemID, l.EffectiveBinding)
	if err != nil {
		return nil, err
	}
	var stacks []struct {
		slot, inst string
		qty        int64
	}
	for rows.Next() {
		var slot string
		var inst id.UUID
		var qty int64
		if err := rows.Scan(&slot, &inst, &qty); err != nil {
			rows.Close()
			return nil, err
		}
		stacks = append(stacks, struct {
			slot, inst string
			qty        int64
		}{slot, inst.String(), qty})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(stacks, func(i, j int) bool {
		return slotNum(stacks[i].slot) < slotNum(stacks[j].slot)
	})
	remaining := new(big.Int).Set(want)
	for _, st := range stacks {
		if remaining.Sign() == 0 {
			break
		}
		spare := maxStack - st.qty
		if spare <= 0 {
			continue
		}
		add := new(big.Int).SetInt64(spare)
		if remaining.Cmp(add) < 0 {
			add.Set(remaining)
		}
		var instID id.UUID
		copy(instID[:], mustParseUUIDBytes(st.inst))
		fit.topUps = append(fit.topUps, topUp{instanceID: instID, qty: add.Int64()})
		remaining.Sub(remaining, add)
		fit.total.Add(fit.total, add)
	}
	if remaining.Sign() > 0 {
		free, err := d.freeSlots(ctx, tx, charID)
		if err != nil {
			return nil, err
		}
		for _, n := range free {
			if remaining.Sign() == 0 {
				break
			}
			add := new(big.Int).SetInt64(maxStack)
			if remaining.Cmp(add) < 0 {
				add.Set(remaining)
			}
			fit.creates = append(fit.creates, createAt{slot: n, qty: add.Int64()})
			remaining.Sub(remaining, add)
			fit.total.Add(fit.total, add)
		}
	}
	return fit, nil
}

// applyItemFit executes a computed fit: stack top-ups then slot creates.
func (d Deps) applyItemFit(ctx context.Context, tx pgx.Tx, fit *itemFit,
	opID id.UUID, res *delivery, l *Line) error {
	for _, t := range fit.topUps {
		if err := addStackQuantity(ctx, tx, t.instanceID, t.qty); err != nil {
			return err
		}
		res.granted = append(res.granted, &protocolv1.ItemGrant{
			ItemInstanceId: t.instanceID[:],
			ItemId:         l.ItemID,
			Quantity:       uint32(t.qty),
		})
	}
	for _, cr := range fit.creates {
		idef, err := d.Defs(ctx, l.ItemID)
		if err != nil {
			return err
		}
		binding, err := items.ParseBinding(l.EffectiveBinding)
		if err != nil {
			return err
		}
		instID, err := d.Items.Create(ctx, tx, idef.Def, items.CreateFields{
			Quantity:        int(cr.qty),
			ItemState:       l.ItemState,
			ContentRevision: strPtr(l.ContentRevision),
			SourceBinding:   &binding,
		}, items.Location{
			Kind:        items.LocCharacterInventory,
			CharacterID: fit.charID,
			Slot:        inventory.InvSlot(cr.slot),
		})
		if err != nil {
			return err
		}
		res.granted = append(res.granted, &protocolv1.ItemGrant{
			ItemInstanceId: instID[:],
			ItemId:         l.ItemID,
			Quantity:       uint32(cr.qty),
		})
	}
	res.lines = append(res.lines, &protocolv1.RewardClaimDeliveredLine{
		LineNo:            uint32(l.LineNo),
		DeliveredQuantity: uint32(fit.total.Int64()),
		RemainingAfter:    uint32(new(big.Int).Sub(l.Remaining(), fit.total).Int64()),
	})
	if err := d.Store.addDeliveredQuantity(ctx, tx, l.claimID, l.LineNo, fit.total); err != nil {
		return err
	}
	l.DeliveredQuantity.Add(l.DeliveredQuantity, fit.total)
	return nil
}

// creditLine applies one currency batch through currency.Credit inside
// the tx (character_currencies lock already held).
func (d Deps) creditLine(ctx context.Context, tx pgx.Tx, charID id.UUID,
	currencyID string, amount *big.Int, opID id.UUID, c *Claim,
	res *delivery, lineNo int16) error {
	if !amount.IsInt64() {
		return fmt.Errorf("reward: currency batch exceeds int64")
	}
	if _, err := currency.Credit(ctx, tx, currency.Mutation{
		CharacterID: charID,
		CurrencyID:  currency.ID(currencyID),
		Delta:       amount.Int64(),
		OperationID: opID,
		ReasonCode:  "reward.claim",
		SourceRef:   c.SourceType + ":" + c.SourceReference,
		Actor:       currency.ActorSystem,
	}); err != nil {
		return err
	}
	res.currencies = append(res.currencies, &protocolv1.CurrencyDelta{
		CurrencyId: currencyID,
		Amount:     amount.Int64(),
	})
	res.lines = append(res.lines, &protocolv1.RewardClaimDeliveredLine{
		LineNo:            uint32(lineNo),
		DeliveredQuantity: uint32(amount.Int64()),
		RemainingAfter:    uint32(new(big.Int).Sub(lineRemaining(c, lineNo), amount).Int64()),
	})
	if err := d.Store.addDeliveredAmount(ctx, tx, c.ClaimID, lineNo, amount); err != nil {
		return err
	}
	lineAddDelivered(c, lineNo, amount)
	return nil
}

// currencyHeadroom is cap minus current balance under the currency lock.
func (d Deps) currencyHeadroom(ctx context.Context, tx pgx.Tx,
	charID id.UUID, currencyID string) (*big.Int, error) {
	capv, ok := currency.Caps[currency.ID(currencyID)]
	if !ok {
		return nil, fmt.Errorf("reward: unknown currency %q", currencyID)
	}
	var bal int64
	err := tx.QueryRow(ctx,
		`SELECT balance FROM character_currencies
		 WHERE character_id=$1 AND currency_id=$2`,
		charID.String(), currencyID).Scan(&bal)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	return new(big.Int).SetInt64(capv - bal), nil
}

// freeSlots lists unoccupied wire slot numbers in ascending order.
func (d Deps) freeSlots(ctx context.Context, tx pgx.Tx, charID id.UUID) ([]uint32, error) {
	capN, err := d.Inv.Capacity(ctx, tx, charID)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx,
		`SELECT slot FROM item_locations
		 WHERE character_id=$1 AND location_kind='CHARACTER_INVENTORY'`,
		charID)
	if err != nil {
		return nil, err
	}
	used := make(map[uint32]struct{})
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			rows.Close()
			return nil, err
		}
		if n := slotNum(s); n != 1<<30 {
			used[n] = struct{}{}
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	free := make([]uint32, 0, capN)
	for n := uint32(0); n < uint32(capN); n++ {
		if _, ok := used[n]; !ok {
			free = append(free, n)
		}
	}
	return free, nil
}

// identity validates the shared record identity (owner == character).
func identity(rec *journalv1.DurableCommandRecord) (id.UUID, id.UUID, error) {
	cmd := rec.GetClient()
	if cmd == nil {
		return id.UUID{}, id.UUID{}, fmt.Errorf("reward: client record without payload")
	}
	if len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 || len(cmd.GetCharacterId()) != 16 {
		return id.UUID{}, id.UUID{}, fmt.Errorf("reward: malformed record identity")
	}
	var charID, opID, owner id.UUID
	copy(charID[:], cmd.GetCharacterId())
	copy(opID[:], rec.GetOperationId())
	copy(owner[:], rec.GetOwnerId())
	if owner != charID {
		return id.UUID{}, id.UUID{}, fmt.Errorf("reward: owner %s != character %s", owner, charID)
	}
	return charID, opID, nil
}

// verdict writes a committed error outcome: in-set codes embed the 409
// result; out-of-set codes leave client_result absent (edge answers
// S2C_ERROR).
func (d Deps) verdict(opID, claimID id.UUID, cause error,
	code protocolv1.ErrorCode) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
		ErrorCode:   code,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CRewardClaimResult{
			S2CRewardClaimResult: &protocolv1.S2CRewardClaimResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
					ErrorCode:   code,
				},
				RewardClaimId: claimID[:],
			},
		},
	}
	return marshalOutcome(outcome)
}

// commitOutcome writes the successful 409 outcome.
func commitOutcome(opID, claimID id.UUID, res *delivery) (idempotency.Outcome, error) {
	state := protocolv1.RewardClaimState_REWARD_CLAIM_STATE_PENDING
	if res.claimed {
		state = protocolv1.RewardClaimState_REWARD_CLAIM_STATE_CLAIMED
	}
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CRewardClaimResult{
			S2CRewardClaimResult: &protocolv1.S2CRewardClaimResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
				},
				RewardClaimId:  claimID[:],
				Granted:        res.granted,
				CurrencyDelta:  res.currencies,
				DeliveredLines: res.lines,
				ClaimState:     state,
			},
		},
	}
	return marshalOutcome(outcome)
}

// marshalOutcome serializes the JournalOutcome as protojson (retained
// receipt representation — durable/character precedent).
func marshalOutcome(outcome *journalv1.JournalOutcome) (idempotency.Outcome, error) {
	b, err := protojson.Marshal(outcome)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: b}, nil
}

// claimErrCode maps engine errors to the 408 wire error set; ok=false
// means the error is internal and must abort the tx, not commit a
// verdict.
func claimErrCode(err error) (protocolv1.ErrorCode, bool) {
	switch {
	case errors.Is(err, ErrInventoryFull):
		return protocolv1.ErrorCode_ERROR_CODE_INVENTORY_FULL, true
	case errors.Is(err, ErrCurrencyCapReached):
		return protocolv1.ErrorCode_ERROR_CODE_CURRENCY_CAP_EXCEEDED, true
	case errors.Is(err, ErrClaimCapReached):
		return protocolv1.ErrorCode_ERROR_CODE_CLAIM_CAP_REACHED, true
	case errors.Is(err, ErrNotOwner):
		return protocolv1.ErrorCode_ERROR_CODE_NOT_OWNER, true
	case errors.Is(err, ErrClaimExpired):
		return protocolv1.ErrorCode_ERROR_CODE_EXPIRED, true
	case errors.Is(err, ErrStateConflict):
		return protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT, true
	default:
		return protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED, false
	}
}

// helpers -------------------------------------------------------------

func slotNum(s string) uint32 {
	const p = "inv."
	if !strings.HasPrefix(s, p) {
		return 1 << 30
	}
	n, err := strconv.ParseUint(s[len(p):], 10, 32)
	if err != nil {
		return 1 << 30
	}
	return uint32(n)
}

func mustParseUUIDBytes(s string) []byte {
	u, err := id.ParseUUID(s)
	if err != nil {
		return make([]byte, 16)
	}
	return u[:]
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func lineRemaining(c *Claim, lineNo int16) *big.Int {
	for _, l := range c.Lines {
		if l.LineNo == lineNo {
			return l.Remaining()
		}
	}
	return new(big.Int)
}

func lineAddDelivered(c *Claim, lineNo int16, amount *big.Int) {
	for _, l := range c.Lines {
		if l.LineNo == lineNo {
			l.DeliveredAmount.Add(l.DeliveredAmount, amount)
			return
		}
	}
}

// addStackQuantity tops up one existing stack under the caller's tx.
func addStackQuantity(ctx context.Context, tx pgx.Tx, instanceID id.UUID, qty int64) error {
	_, err := tx.Exec(ctx,
		`UPDATE item_instances SET quantity = quantity + $2 WHERE item_instance_id = $1`,
		instanceID, qty)
	return err
}
