package equipment

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/character"
	"thinhthan/internal/durable/idempotency"
	"thinhthan/internal/durable/items"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Family is the §7 durable family name for wire id 402
// (router.durableFamiliesExact).
const Family = "loadout.change"

// Deps are the executor dependencies the composition root injects.
// Defs resolves equipment defs (nil = fail closed); Locks resolves the
// live trade-lock ledger per owner (nil = no locks held).
type Deps struct {
	Store      *Store
	Items      *items.Store
	Characters *character.Store
	Defs       Defs
	Locks      func(owner id.UUID) *items.TradeLockLedger
}

// Executors returns the family→executor map merged by the composition
// ProducerClient family-mux.
func Executors(d Deps) map[string]queue.Executor {
	return map[string]queue.Executor{Family: d.executor}
}

func identity(rec *journalv1.DurableCommandRecord) (charID, opID id.UUID, err error) {
	cmd := rec.GetClient()
	if cmd == nil {
		return id.UUID{}, id.UUID{}, fmt.Errorf("equipment: client record without payload")
	}
	if len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 || len(cmd.GetCharacterId()) != 16 {
		return id.UUID{}, id.UUID{}, fmt.Errorf("equipment: malformed record identity")
	}
	copy(charID[:], cmd.GetCharacterId())
	copy(opID[:], rec.GetOperationId())
	var owner id.UUID
	copy(owner[:], rec.GetOwnerId())
	if owner != charID {
		return id.UUID{}, id.UUID{}, fmt.Errorf("equipment: owner %v != character %v", owner, charID)
	}
	return charID, opID, nil
}

func (d Deps) executor(ctx context.Context, tx pgx.Tx, rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != Family {
		return idempotency.Outcome{}, fmt.Errorf("equipment: family %q", rec.GetOperationFamily())
	}
	charID, opID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	req := rec.GetClient().GetC2SLoadoutChange()
	if req == nil {
		return idempotency.Outcome{}, fmt.Errorf("equipment: record missing C2SLoadoutChange")
	}

	// Canonical lock order (database.md): characters first, then the
	// aggregates the op touches (inventories, souls, items).
	if err := lockorder.Acquire(ctx, tx,
		lockorder.RowLock("characters", charID),
		lockorder.RowLock("character_inventories", charID),
		lockorder.RowLock("character_souls", charID),
	); err != nil {
		return idempotency.Outcome{}, err
	}

	loadouts, err := d.Store.loadoutsLocked(ctx, tx, charID)
	if err != nil {
		return idempotency.Outcome{}, err
	}

	var changed []*protocolv1.LoadoutChangedSlot
	var souls []*protocolv1.SoulContractBinding
	var codeErr error

	switch req.GetKind() {
	case protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_EQUIP:
		changed, codeErr = d.equip(ctx, tx, charID, loadouts, req.GetEquip())
	case protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_UNEQUIP:
		changed, souls, codeErr = d.unequip(ctx, tx, charID, loadouts, req.GetUnequip())
	case protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_SWITCH_ACTIVE:
		changed, codeErr = d.switchActive(ctx, tx, charID, loadouts, req.GetSwitchActive())
	case protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_SKILL_SET:
		codeErr = errSkillLoadout
	case protocolv1.LoadoutChangeKind_LOADOUT_CHANGE_KIND_SOUL_CONTRACT:
		codeErr = errSoulContractLimit
	default:
		codeErr = errMalformed
	}
	if codeErr != nil {
		return d.verdict(opID, req, codeErr)
	}

	rev, err := d.Store.RevisionSum(ctx, tx, charID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	active := ""
	for _, l := range loadouts {
		if l.Role == "ACTIVE" {
			active = l.ID
		}
	}
	return commitOutcome(opID, req.GetKind(), active, rev, changed, souls)
}

func findLoadout(loadouts []Loadout, lid string) (*Loadout, error) {
	for i := range loadouts {
		if loadouts[i].ID == lid {
			return &loadouts[i], nil
		}
	}
	return nil, errMalformed
}

// equip moves the item from CHARACTER_INVENTORY to the named EQUIPPED
// slot, displacing any occupant to a free inventory slot (ADR-0060).
func (d Deps) equip(ctx context.Context, tx pgx.Tx, charID id.UUID,
	loadouts []Loadout, eq *protocolv1.LoadoutEquip) ([]*protocolv1.LoadoutChangedSlot, error) {
	if eq == nil || !isSlot(eq.GetSlotId()) || len(eq.GetItemInstanceId()) != 16 {
		return nil, errMalformed
	}
	l, err := findLoadout(loadouts, eq.GetLoadoutId())
	if err != nil {
		return nil, err
	}
	slotID := eq.GetSlotId()
	var instID id.UUID
	copy(instID[:], eq.GetItemInstanceId())

	var itemID string
	err = tx.QueryRow(ctx,
		`SELECT item_id FROM item_instances WHERE item_instance_id = $1`,
		instID.String()).Scan(&itemID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errItemNotFound
		}
		return nil, err
	}
	if d.Defs == nil {
		return nil, errItemNotFound
	}
	def, err := d.Defs(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if !def.EquipmentItem {
		return nil, errItemNotFound
	}
	if def.SlotID != slotID {
		return nil, errSlotMismatch
	}
	level, err := d.charLevel(ctx, tx, charID)
	if err != nil {
		return nil, err
	}
	if level < def.LevelMin {
		return nil, errLevelTooLow
	}
	lock := d.lockOf(charID)
	// Displace the occupant into a free inventory slot (ADR-0060).
	if occupant, ok := l.Items[slotID]; ok {
		free, err := d.firstFreeSlot(ctx, tx, charID)
		if err != nil {
			return nil, err
		}
		if err := d.Items.Unequip(ctx, tx, occupant, free, lock); err != nil {
			return nil, mapCustodyErr(err)
		}
	}
	if err := d.Items.Equip(ctx, tx, instID, def.Def, charID,
		equippedSlot(l.ID, slotID), lock); err != nil {
		return nil, mapCustodyErr(err)
	}
	if err := d.Store.bumpRevision(ctx, tx, charID, l.Index); err != nil {
		return nil, err
	}
	return []*protocolv1.LoadoutChangedSlot{{
		LoadoutId: l.ID, SlotId: slotID, ItemInstanceId: instID[:],
	}}, nil
}

// unequip returns the slot's item to inventory (capacity required) and
// releases its contracted Soul to Collection atomically (ADR-0060).
func (d Deps) unequip(ctx context.Context, tx pgx.Tx, charID id.UUID,
	loadouts []Loadout, uq *protocolv1.LoadoutUnequip) ([]*protocolv1.LoadoutChangedSlot,
	[]*protocolv1.SoulContractBinding, error) {
	if uq == nil || !isSlot(uq.GetSlotId()) {
		return nil, nil, errMalformed
	}
	l, err := findLoadout(loadouts, uq.GetLoadoutId())
	if err != nil {
		return nil, nil, err
	}
	slotID := uq.GetSlotId()
	instID, ok := l.Items[slotID]
	if !ok {
		return nil, nil, errItemNotFound
	}
	free, err := d.firstFreeSlot(ctx, tx, charID)
	if err != nil {
		return nil, nil, err
	}
	if err := d.Items.Unequip(ctx, tx, instID, free, d.lockOf(charID)); err != nil {
		return nil, nil, mapCustodyErr(err)
	}
	// Contracted Soul returns to Collection atomically.
	var souls []*protocolv1.SoulContractBinding
	var soulID string
	err = tx.QueryRow(ctx,
		`SELECT soul_instance_id FROM character_souls
		 WHERE character_id = $1 AND contracted_item_instance_id = $2 FOR UPDATE`,
		charID.String(), instID.String()).Scan(&soulID)
	switch {
	case err == nil:
		if _, err := tx.Exec(ctx,
			`UPDATE character_souls SET contracted_item_instance_id = NULL
			 WHERE soul_instance_id = $1`, soulID); err != nil {
			return nil, nil, err
		}
		sid, err := id.ParseUUID(soulID)
		if err != nil {
			return nil, nil, err
		}
		souls = append(souls, &protocolv1.SoulContractBinding{
			SoulInstanceId: sid[:], // empty item_instance_id = Collection
		})
	case errors.Is(err, pgx.ErrNoRows):
	default:
		return nil, nil, err
	}
	if err := d.Store.bumpRevision(ctx, tx, charID, l.Index); err != nil {
		return nil, nil, err
	}
	return []*protocolv1.LoadoutChangedSlot{{
		LoadoutId: l.ID, SlotId: slotID, // emptied slot: no item_instance_id
	}}, souls, nil
}

// switchActive flips the ACTIVE role to the named loadout; items never
// pass through inventory.
func (d Deps) switchActive(ctx context.Context, tx pgx.Tx, charID id.UUID,
	loadouts []Loadout, sw *protocolv1.LoadoutSwitchActive) ([]*protocolv1.LoadoutChangedSlot, error) {
	if sw == nil {
		return nil, errMalformed
	}
	target, err := findLoadout(loadouts, sw.GetLoadoutId())
	if err != nil {
		return nil, err
	}
	if target.Role == "ACTIVE" {
		return nil, nil // idempotent no-op
	}
	var cur *Loadout
	for i := range loadouts {
		if loadouts[i].Role == "ACTIVE" {
			cur = &loadouts[i]
		}
	}
	if cur == nil {
		return nil, errStateConflict
	}
	if _, err := tx.Exec(ctx,
		`UPDATE character_loadouts SET role = 'SUPPORT', revision = revision + 1
		 WHERE character_id = $1 AND loadout_index = $2`, charID.String(), cur.Index); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE character_loadouts SET role = 'ACTIVE', revision = revision + 1
		 WHERE character_id = $1 AND loadout_index = $2`, charID.String(), target.Index); err != nil {
		return nil, err
	}
	cur.Role, target.Role = "SUPPORT", "ACTIVE"
	var changed []*protocolv1.LoadoutChangedSlot
	for _, l := range []*Loadout{cur, target} {
		for sid, inst := range l.Items {
			changed = append(changed, &protocolv1.LoadoutChangedSlot{
				LoadoutId: l.ID, SlotId: sid, ItemInstanceId: inst[:],
			})
		}
	}
	return changed, nil
}

// --- helpers ---

func (d Deps) lockOf(charID id.UUID) *items.TradeLockLedger {
	if d.Locks == nil {
		return nil
	}
	return d.Locks(charID)
}

func (d Deps) charLevel(ctx context.Context, tx pgx.Tx, charID id.UUID) (int, error) {
	var lvl int
	err := tx.QueryRow(ctx,
		`SELECT level FROM characters WHERE character_id = $1`, charID.String()).Scan(&lvl)
	return lvl, err
}

// firstFreeSlot finds the lowest free inv.N under the held lock;
// capacity exhausted -> INVENTORY_FULL.
func (d Deps) firstFreeSlot(ctx context.Context, tx pgx.Tx, charID id.UUID) (string, error) {
	var cap int
	if err := tx.QueryRow(ctx,
		`SELECT capacity FROM character_inventories WHERE character_id = $1`,
		charID.String()).Scan(&cap); err != nil {
		return "", err
	}
	rows, err := tx.Query(ctx,
		`SELECT slot FROM item_locations
		 WHERE character_id = $1 AND location_kind = 'CHARACTER_INVENTORY'`, charID.String())
	if err != nil {
		return "", err
	}
	defer rows.Close()
	used := map[int]bool{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return "", err
		}
		var n int
		if _, err := fmt.Sscanf(s, "inv.%d", &n); err == nil {
			used[n] = true
		}
	}
	for n := 0; n < cap; n++ {
		if !used[n] {
			return fmt.Sprintf("inv.%d", n), nil
		}
	}
	return "", errInventoryFull
}

// mapCustodyErr projects items-layer custody errors onto the 402 set.
func mapCustodyErr(err error) error {
	switch {
	case errors.Is(err, items.ErrTradeLocked):
		return errItemLocked
	case errors.Is(err, items.ErrSoulContracted):
		return errItemLocked
	case errors.Is(err, items.ErrLocationConflict), errors.Is(err, items.ErrInvalidLocation):
		return errItemNotFound
	case errors.Is(err, items.ErrSlotOccupied):
		return errStateConflict
	default:
		return err
	}
}

// --- outcomes ---

func (d Deps) verdict(opID id.UUID, req *protocolv1.C2SLoadoutChange, cause error) (idempotency.Outcome, error) {
	code := codeOf(cause)
	outcome := &journalv1.JournalOutcome{
		ClientResult: &journalv1.JournalOutcome_S2CLoadoutResult{
			S2CLoadoutResult: &protocolv1.S2CLoadoutResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					ErrorCode:   code,
				},
				Kind: req.GetKind(),
			},
		},
	}
	return marshalOutcome(outcome)
}

func commitOutcome(opID id.UUID, kind protocolv1.LoadoutChangeKind, activeID string,
	rev int64, changed []*protocolv1.LoadoutChangedSlot,
	souls []*protocolv1.SoulContractBinding) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		ClientResult: &journalv1.JournalOutcome_S2CLoadoutResult{
			S2CLoadoutResult: &protocolv1.S2CLoadoutResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
				},
				Kind:            kind,
				LoadoutRevision: uint64(rev),
				ActiveLoadoutId: activeID,
				ChangedSlots:    changed,
				SoulContracts:   souls,
			},
		},
	}
	return marshalOutcome(outcome)
}

func marshalOutcome(outcome *journalv1.JournalOutcome) (idempotency.Outcome, error) {
	b, err := protojson.Marshal(outcome)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: b}, nil
}

func codeOf(err error) protocolv1.ErrorCode {
	switch {
	case errors.Is(err, errMalformed):
		return protocolv1.ErrorCode_ERROR_CODE_INVALID_STATE
	case errors.Is(err, errItemNotFound):
		return protocolv1.ErrorCode_ERROR_CODE_ITEM_NOT_FOUND
	case errors.Is(err, errItemLocked):
		return protocolv1.ErrorCode_ERROR_CODE_ITEM_LOCKED
	case errors.Is(err, errSlotMismatch):
		return protocolv1.ErrorCode_ERROR_CODE_SLOT_MISMATCH
	case errors.Is(err, errLevelTooLow):
		return protocolv1.ErrorCode_ERROR_CODE_LEVEL_TOO_LOW
	case errors.Is(err, errInventoryFull):
		return protocolv1.ErrorCode_ERROR_CODE_INVENTORY_FULL
	case errors.Is(err, errSkillLoadout):
		return protocolv1.ErrorCode_ERROR_CODE_SKILL_LOADOUT_INVALID
	case errors.Is(err, errSoulContractLimit):
		return protocolv1.ErrorCode_ERROR_CODE_SOUL_CONTRACT_LIMIT_REACHED
	default:
		return protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT
	}
}
