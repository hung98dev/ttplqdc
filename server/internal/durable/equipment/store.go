package equipment

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/items"
)

// EquipDef is the runtime projection of one compiled `equipment`
// family record (catalog_equipment.go emission).
type EquipDef struct {
	Def           items.Def
	SlotID        string // canonical slot (equipment.Slots)
	LevelMin      int
	SetKey        string
	Element       string
	Tier          string
	EquipmentItem bool // kind == EQUIPMENT
}

// Defs resolves an item_id to its equipment definition; composition
// binds the activated content snapshot. nil resolver = fail closed.
type Defs func(ctx context.Context, itemID string) (EquipDef, error)

// Store is the loadout aggregate store bound to the durable pool.
type Store struct{ pool *pgxpool.Pool }

// New returns the store.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Loadout is one character_loadouts row plus its EQUIPPED positions.
type Loadout struct {
	Index    int
	ID       string
	Role     string // ACTIVE | SUPPORT
	Revision int64
	Items    map[string]id.UUID // slot_id -> item_instance_id
}

// loadoutsLocked reads the character's three loadout rows under the
// character lock and materializes any missing rows (first touch seeds
// the fixed roster: index 1 ACTIVE, 2..3 SUPPORT).
func (s *Store) loadoutsLocked(ctx context.Context, tx pgx.Tx, charID id.UUID) ([]Loadout, error) {
	rows := make([]Loadout, 0, loadoutCount)
	r, err := tx.Query(ctx,
		`SELECT loadout_index, role, revision FROM character_loadouts
		 WHERE character_id = $1 ORDER BY loadout_index FOR UPDATE`, charID.String())
	if err != nil {
		return nil, err
	}
	seen := map[int]bool{}
	for r.Next() {
		var idx int
		var role string
		var rev int64
		if err := r.Scan(&idx, &role, &rev); err != nil {
			r.Close()
			return nil, err
		}
		lid, ok := loadoutID(idx)
		if !ok {
			r.Close()
			return nil, fmt.Errorf("equipment: corrupt loadout_index %d", idx)
		}
		rows = append(rows, Loadout{Index: idx, ID: lid, Role: role, Revision: rev,
			Items: map[string]id.UUID{}})
		seen[idx] = true
	}
	r.Close()
	if err := r.Err(); err != nil {
		return nil, err
	}
	for i := 1; i <= loadoutCount; i++ {
		if seen[i] {
			continue
		}
		role := "SUPPORT"
		if i == 1 {
			role = "ACTIVE"
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO character_loadouts (character_id, loadout_index, role, revision)
			 VALUES ($1,$2,$3,0)`, charID.String(), i, role); err != nil {
			return nil, err
		}
		lid, _ := loadoutID(i)
		rows = append(rows, Loadout{Index: i, ID: lid, Role: role, Items: map[string]id.UUID{}})
	}
	// Load EQUIPPED positions for this character.
	r2, err := tx.Query(ctx,
		`SELECT il.slot, il.item_instance_id
		   FROM item_locations il
		  WHERE il.character_id = $1 AND il.location_kind = 'EQUIPPED'
		  FOR UPDATE OF il`, charID.String())
	if err != nil {
		return nil, err
	}
	defer r2.Close()
	for r2.Next() {
		var slot string
		var inst string
		if err := r2.Scan(&slot, &inst); err != nil {
			return nil, err
		}
		lid, sid, ok := splitEquippedSlot(slot)
		if !ok {
			continue
		}
		instID, err := id.ParseUUID(inst)
		if err != nil {
			return nil, err
		}
		for j := range rows {
			if rows[j].ID == lid {
				rows[j].Items[sid] = instID
			}
		}
	}
	return rows, r2.Err()
}

func splitEquippedSlot(s string) (loadoutID, slotID string, ok bool) {
	for _, lid := range loadoutIDs {
		if len(s) > len(lid)+1 && s[:len(lid)] == lid && s[len(lid)] == '.' {
			return lid, s[len(lid)+1:], true
		}
	}
	return "", "", false
}

// bumpRevision increments one loadout row's revision by exactly 1.
func (s *Store) bumpRevision(ctx context.Context, tx pgx.Tx, charID id.UUID, index int) error {
	_, err := tx.Exec(ctx,
		`UPDATE character_loadouts SET revision = revision + 1
		 WHERE character_id = $1 AND loadout_index = $2`, charID.String(), index)
	return err
}

// RevisionSum returns the wire loadout_revision (SUM of the 3 rows).
func (s *Store) RevisionSum(ctx context.Context, tx pgx.Tx, charID id.UUID) (int64, error) {
	var sum int64
	err := tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(revision),0) FROM character_loadouts WHERE character_id = $1`,
		charID.String()).Scan(&sum)
	return sum, err
}
