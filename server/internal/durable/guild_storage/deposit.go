package guild_storage

import (
	"context"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/guild"
)

// Deposit executes C2S_GUILD_STORAGE_DEPOSIT (629): role gate
// (Member -> COMMON only), section unlock + capacity, full eligibility
// list (items.GuildDeposit enforces UNBOUND/unequipped/Soul-Contract/
// trade-lock), depositor ids on the location row, and the
// pending-claim same-account rejection.
func (d Deps) Deposit(ctx context.Context, tx pgx.Tx, opID, actor, instanceID id.UUID,
	quantity uint32, section string) (uint64, error) {
	if !validSection(section) {
		return 0, errInvalidSection
	}
	m, err := d.Guild.Member(ctx, tx, actor)
	if err != nil {
		return 0, err
	}
	if m == nil {
		return 0, errNotMember
	}
	// Deposit role gate (D-1): Leader/Vice/Officer may deposit into
	// both sections; Member deposits into COMMON only.
	if section == SectionReserve && m.Role == guild.RoleMember {
		return 0, errPermissionDenied
	}
	guildID := m.GuildID
	if err := lockStorage(ctx, tx, guildID); err != nil {
		return 0, err
	}
	level, err := guildLevel(ctx, tx, guildID)
	if err != nil {
		return 0, err
	}
	capN := SectionCapacity(section, level)
	if capN <= 0 {
		return 0, errSectionLocked
	}
	used, err := sectionCount(ctx, tx, guildID, section)
	if err != nil {
		return 0, err
	}
	if used >= capN {
		return 0, errCapacityFull
	}
	before := used // before_quantity = stored line count pre-deposit

	// Pending-claim same-account deposit reject: refuse a deposit whose
	// item already carries an open claim by a different same-account
	// character (closes the side-channel where the depositor would
	// later DELIVER to their own alt through the claim path).
	acct, err := accountOf(ctx, tx, actor)
	if err != nil {
		return 0, err
	}
	if err := rejectSameAccountClaimedLine(ctx, tx, guildID, instanceID, acct, actor); err != nil {
		return 0, err
	}

	slot, err := freeSlot(ctx, tx, guildID, section, capN)
	if err != nil {
		return 0, err
	}
	if err := d.Items.GuildDeposit(ctx, tx, instanceID, guildID,
		slotName(section, slot), nil); err != nil {
		return 0, err
	}
	var itemID string
	var afterQty int
	if err := tx.QueryRow(ctx,
		`SELECT item_id, quantity FROM item_instances WHERE item_instance_id=$1`,
		instanceID).Scan(&itemID, &afterQty); err != nil {
		return 0, err
	}
	if err := audit(ctx, tx, guildID, opID, actor, "DEPOSIT", section,
		itemID, afterQty, nil, &actor, before, before+1, d.Now()); err != nil {
		return 0, err
	}
	return bumpStorageRevision(ctx, tx, guildID)
}

// rejectSameAccountClaimedLine rejects a deposit when the instance has
// an open (PENDING/APPROVED) claim requested by a different character
// on the depositor's account.
func rejectSameAccountClaimedLine(ctx context.Context, tx pgx.Tx,
	guildID, instanceID, acct, actor id.UUID) error {
	var exists bool
	err := tx.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM guild_storage_claims c
		     JOIN characters ch ON ch.character_id = c.requester_character_id
		   WHERE c.guild_id=$1 AND c.item_instance_id=$2
		     AND c.state IN ('PENDING','APPROVED')
		     AND ch.account_id=$3 AND c.requester_character_id<>$4)`,
		guildID, instanceID, acct, actor).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return errSameAccount
	}
	return nil
}
