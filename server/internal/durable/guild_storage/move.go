package guild_storage

import (
	"context"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/guild"
)

// Move executes C2S_GUILD_STORAGE_MOVE (644): Leader/Vice/Officer only;
// destination capacity and not-reserved preconditions; slot prefix
// rewrite inside the storage row set; audit + revision bump.
func (d Deps) Move(ctx context.Context, tx pgx.Tx, opID, actor, instanceID id.UUID,
	toSection string) (uint64, error) {
	if !validSection(toSection) {
		return 0, errInvalidSection
	}
	m, err := d.Guild.Member(ctx, tx, actor)
	if err != nil {
		return 0, err
	}
	if m == nil {
		return 0, errNotMember
	}
	if m.Role == guild.RoleMember {
		return 0, errPermissionDenied
	}
	guildID := m.GuildID
	if err := lockStorage(ctx, tx, guildID); err != nil {
		return 0, err
	}
	row, err := storageItem(ctx, tx, guildID, instanceID)
	if err != nil {
		return 0, err
	}
	from := row.Section
	if from == toSection {
		return 0, errStateConflict
	}
	level, err := guildLevel(ctx, tx, guildID)
	if err != nil {
		return 0, err
	}
	capN := SectionCapacity(toSection, level)
	if capN <= 0 {
		return 0, errSectionLocked
	}
	used, err := sectionCount(ctx, tx, guildID, toSection)
	if err != nil {
		return 0, err
	}
	if used >= capN {
		return 0, errCapacityFull
	}
	// APPROVED claims pin the item/quantity — a move that changes the
	// claim's reserved section would desync settlement.
	open, err := openClaimsOn(ctx, tx, guildID, instanceID)
	if err != nil {
		return 0, err
	}
	for _, c := range open {
		if c.State == ClaimApproved {
			return 0, errStateConflict
		}
	}
	slot, err := freeSlot(ctx, tx, guildID, toSection, capN)
	if err != nil {
		return 0, err
	}
	newSlot := slotName(toSection, slot)
	if _, err := tx.Exec(ctx,
		`UPDATE item_locations SET slot=$1, updated_at=NOW()
		  WHERE item_instance_id=$2 AND location_kind='GUILD_STORAGE'`,
		newSlot, instanceID); err != nil {
		return 0, err
	}
	// PENDING claims follow the item to its new section (they name an
	// instance, not a slot) — no state change, reservation untouched.
	if err := audit(ctx, tx, guildID, opID, actor, "MOVE", toSection,
		row.ItemID, row.Quantity, nil, &actor, used, used+1, d.Now()); err != nil {
		return 0, err
	}
	return bumpStorageRevision(ctx, tx, guildID)
}
