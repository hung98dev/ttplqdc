package guild_storage

import (
	"context"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/guild"
)

// Withdraw executes C2S_GUILD_STORAGE_WITHDRAW (630): section + quota
// gates, ADR-0049 same-account prohibition, 72h cross-character
// membership gate, item_partner_counts signal write, and the custody
// move via items.GuildWithdraw (which enforces a free inventory slot
// and binding rules — ErrCapacity maps to INVENTORY_FULL).
func (d Deps) Withdraw(ctx context.Context, tx pgx.Tx, opID, actor, instanceID id.UUID,
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
	guildID := m.GuildID
	// RESERVE direct withdraw is Leader/Vice only.
	if section == SectionReserve &&
		m.Role != guild.RoleLeader && m.Role != guild.RoleViceLeader {
		return 0, errPermissionDenied
	}
	if err := lockStorage(ctx, tx, guildID); err != nil {
		return 0, err
	}
	row, err := storageItem(ctx, tx, guildID, instanceID)
	if err != nil {
		return 0, err
	}
	if row.Section != section {
		return 0, errStateConflict
	}
	// Quantity is all-or-nothing: a partial withdraw splits the storage
	// stack and delivers the requested slice.
	qty := row.Quantity
	if quantity > 0 {
		qty = int(quantity)
	}
	if qty < 1 || qty > row.Quantity {
		return 0, errInvalidQuantity
	}

	// Per-day withdraw quota (COMMON section only — RESERVE is gated
	// by role). Audit rows are the counter of record.
	if section == SectionCommon {
		limit := WithdrawQuotaPerDay(m.Role)
		if limit > 0 {
			used, err := withdrawsToday(ctx, tx, guildID, actor, utcDayStart(d.Now()))
			if err != nil {
				return 0, err
			}
			if used >= limit {
				return 0, errQuotaExceeded
			}
		}
	}

	// ADR-0049 same-account prohibition + 72h cross-character gate.
	if err := d.checkReceiverEligibility(ctx, tx, actor, m, row); err != nil {
		return 0, err
	}

	invSlot, err := d.FreeInvSlot(ctx, tx, actor)
	if err != nil {
		return 0, err
	}
	moveID := instanceID
	if qty < row.Quantity {
		moveID, err = splitStorageStack(ctx, tx, row, qty)
		if err != nil {
			return 0, err
		}
	}
	if err := d.Items.GuildWithdraw(ctx, tx, moveID, actor, invSlot, nil); err != nil {
		return 0, err
	}
	// Cross-character signal: count items received from the depositor.
	if row.DepositorCharacterID != actor {
		if err := recordPartnerItems(ctx, tx, actor, row.DepositorCharacterID,
			qty, utcDayStart(d.Now()), d.Now()); err != nil {
			return 0, err
		}
	}
	if err := audit(ctx, tx, guildID, opID, actor, "WITHDRAW", section,
		row.ItemID, qty, &actor, &row.DepositorCharacterID,
		row.Quantity, row.Quantity-qty, d.Now()); err != nil {
		return 0, err
	}
	return bumpStorageRevision(ctx, tx, guildID)
}

// checkReceiverEligibility enforces ADR-0049: a same-account
// different-character withdraw is always rejected; a different-account
// cross-character withdraw needs joined_at >= 72h.
func (d Deps) checkReceiverEligibility(ctx context.Context, tx pgx.Tx,
	receiver id.UUID, m *guild.MemberRow, row *StorageItemRow) error {
	if row.DepositorCharacterID == receiver {
		return nil // own deposit
	}
	acct, err := accountOf(ctx, tx, receiver)
	if err != nil {
		return err
	}
	if acct == row.DepositorAccountID {
		return errSameAccount
	}
	if d.Now().Sub(m.JoinedAt) < MembershipAgeGate {
		return errMembershipTooNew
	}
	return nil
}

// FreeInvSlot delegates to the inventory-free-slot helper with the
// composed inventory store.
func (d Deps) FreeInvSlot(ctx context.Context, tx pgx.Tx, char id.UUID) (string, error) {
	return FreeInventorySlot(ctx, tx, d.Inv, char)
}
