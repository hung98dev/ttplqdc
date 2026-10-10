package guild_storage

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/guild"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// ClaimRequest executes C2S_GUILD_STORAGE_CLAIM_REQUEST (645):
// Officer/Member open a PENDING claim on a RESERVE item (r4:
// requester membership is revalidated here and on every later
// transition).
func (d Deps) ClaimRequest(ctx context.Context, tx pgx.Tx, opID, actor, instanceID id.UUID,
	quantity uint32) (id.UUID, uint64, error) {
	m, err := d.Guild.Member(ctx, tx, actor)
	if err != nil {
		return id.UUID{}, 0, err
	}
	if m == nil {
		return id.UUID{}, 0, errNotMember
	}
	if m.Role == guild.RoleLeader || m.Role == guild.RoleViceLeader {
		return id.UUID{}, 0, errPermissionDenied // L/V withdraw directly
	}
	guildID := m.GuildID
	if err := lockStorage(ctx, tx, guildID); err != nil {
		return id.UUID{}, 0, err
	}
	row, err := storageItem(ctx, tx, guildID, instanceID)
	if err != nil {
		return id.UUID{}, 0, err
	}
	if row.Section != SectionReserve {
		return id.UUID{}, 0, errStateConflict
	}
	qty := row.Quantity
	if quantity > 0 {
		qty = int(quantity)
	}
	if qty < 1 || qty > row.Quantity {
		return id.UUID{}, 0, errInvalidQuantity
	}
	// Reserved quantity must fit inside the stored stack.
	open, err := openClaimsOn(ctx, tx, guildID, instanceID)
	if err != nil {
		return id.UUID{}, 0, err
	}
	reserved := 0
	for _, c := range open {
		if c.State == ClaimApproved {
			reserved += c.Quantity
		}
	}
	if qty > row.Quantity-reserved {
		return id.UUID{}, 0, errStateConflict
	}
	// Same-account prohibition at request time too: a claim on a
	// same-account different-character deposit could only ever fail at
	// DELIVER, so reject it up front.
	if err := d.checkReceiverEligibility(ctx, tx, actor, m, row); err != nil {
		return id.UUID{}, 0, err
	}
	now := d.Now()
	claimID := id.NewV4()
	if _, err := tx.Exec(ctx,
		`INSERT INTO guild_storage_claims
		 (claim_id, guild_id, requester_character_id, item_instance_id,
		  quantity, state, created_at, expires_at)
		 VALUES ($1,$2,$3,$4,$5,'PENDING',$6,$7)`,
		claimID, guildID, actor, instanceID, qty, now,
		now.Add(PendingClaimLifetime)); err != nil {
		return id.UUID{}, 0, err
	}
	if err := audit(ctx, tx, guildID, opID, actor, "CLAIM_REQUEST",
		SectionReserve, row.ItemID, qty, &actor, &row.DepositorCharacterID,
		0, qty, now); err != nil {
		return id.UUID{}, 0, err
	}
	rev, err := bumpStorageRevision(ctx, tx, guildID)
	return claimID, rev, err
}

// ClaimDecide executes C2S_GUILD_STORAGE_CLAIM_DECIDE (646) —
// APPROVE/REJECT = LEADER/VICE_LEADER; CANCEL = requester/L/V;
// DELIVER = requester only. Every transition revalidates the
// requester's membership (r4): a non-member requester flips the claim
// to CANCELLED with reservation release instead of applying the
// decision.
func (d Deps) ClaimDecide(ctx context.Context, tx pgx.Tx, opID, actor, claimID id.UUID,
	decision protocolv1.GuildStorageClaimDecision) (uint64, error) {
	m, err := d.Guild.Member(ctx, tx, actor)
	if err != nil {
		return 0, err
	}
	if m == nil {
		return 0, errNotMember
	}
	guildID := m.GuildID
	if err := lockStorage(ctx, tx, guildID); err != nil {
		return 0, err
	}
	c, err := claimForUpdate(ctx, tx, claimID)
	if err != nil {
		return 0, err
	}
	if c.GuildID != guildID {
		return 0, errClaimNotFound
	}
	row, err := storageItem(ctx, tx, guildID, c.Instance)
	if err != nil {
		return 0, err
	}
	now := d.Now()

	// r4: requester membership revalidation on every transition —
	// leave/kick cannot hook durable/guild's closed executor set, so
	// the storage side cancels open claims whose requester detached.
	if req, err := d.Guild.MemberIn(ctx, tx, c.Requester, guildID); err != nil {
		return 0, err
	} else if req == nil {
		if err := d.transitionClaim(ctx, tx, c, ClaimCancelled, now, nil); err != nil {
			return 0, err
		}
		if err := audit(ctx, tx, guildID, opID, actor, "CLAIM_CANCEL",
			SectionReserve, row.ItemID, c.Quantity, nil,
			&row.DepositorCharacterID, c.Quantity, c.Quantity, now); err != nil {
			return 0, err
		}
		rev, err := bumpStorageRevision(ctx, tx, guildID)
		if err != nil {
			return 0, err
		}
		return rev, errRequesterNotMember
	}

	switch decision {
	case protocolv1.GuildStorageClaimDecision_GUILD_STORAGE_CLAIM_DECISION_APPROVE,
		protocolv1.GuildStorageClaimDecision_GUILD_STORAGE_CLAIM_DECISION_REJECT:
		if m.Role != guild.RoleLeader && m.Role != guild.RoleViceLeader {
			return 0, errPermissionDenied
		}
		if c.State != ClaimPending {
			return 0, errClaimNotDecidable
		}
		var action, state string
		var approvedAt *time.Time
		var expiresAt time.Time
		if decision == protocolv1.GuildStorageClaimDecision_GUILD_STORAGE_CLAIM_DECISION_APPROVE {
			action, state = "CLAIM_APPROVE", ClaimApproved
			approvedAt = &now
			expiresAt = now.Add(ApprovedClaimLifetime)
		} else {
			action, state = "CLAIM_REJECT", ClaimRejected
			expiresAt = c.ExpiresAt
		}
		if err := d.transitionClaimFull(ctx, tx, c, state, now, &actor,
			approvedAt, expiresAt); err != nil {
			return 0, err
		}
		if err := audit(ctx, tx, guildID, opID, actor, action, SectionReserve,
			row.ItemID, c.Quantity, &c.Requester, &row.DepositorCharacterID,
			c.Quantity, c.Quantity, now); err != nil {
			return 0, err
		}
		return bumpStorageRevision(ctx, tx, guildID)

	case protocolv1.GuildStorageClaimDecision_GUILD_STORAGE_CLAIM_DECISION_CANCEL:
		if actor != c.Requester &&
			m.Role != guild.RoleLeader && m.Role != guild.RoleViceLeader {
			return 0, errPermissionDenied
		}
		if c.State != ClaimPending && c.State != ClaimApproved {
			return 0, errClaimNotDecidable
		}
		if err := d.transitionClaim(ctx, tx, c, ClaimCancelled, now, nil); err != nil {
			return 0, err
		}
		if err := audit(ctx, tx, guildID, opID, actor, "CLAIM_CANCEL",
			SectionReserve, row.ItemID, c.Quantity, &c.Requester,
			&row.DepositorCharacterID, c.Quantity, c.Quantity, now); err != nil {
			return 0, err
		}
		return bumpStorageRevision(ctx, tx, guildID)

	case protocolv1.GuildStorageClaimDecision_GUILD_STORAGE_CLAIM_DECISION_DELIVER:
		if actor != c.Requester {
			return 0, errPermissionDenied // ADR-0060
		}
		if c.State != ClaimApproved {
			return 0, errClaimNotDecidable
		}
		if now.After(c.ExpiresAt) {
			return 0, errClaimNotDecidable
		}
		if err := d.checkReceiverEligibility(ctx, tx, actor, m, row); err != nil {
			return 0, err
		}
		invSlot, err := d.FreeInvSlot(ctx, tx, actor)
		if err != nil {
			// INVENTORY_FULL keeps the claim APPROVED (ADR-0060).
			return 0, err
		}
		moveID := c.Instance
		if c.Quantity < row.Quantity {
			moveID, err = splitStorageStack(ctx, tx, row, c.Quantity)
			if err != nil {
				return 0, err
			}
		}
		if err := d.Items.GuildWithdraw(ctx, tx, moveID, actor, invSlot, nil); err != nil {
			return 0, err
		}
		if err := d.transitionClaim(ctx, tx, c, ClaimCompleted, now, nil); err != nil {
			return 0, err
		}
		if row.DepositorCharacterID != actor {
			if err := recordPartnerItems(ctx, tx, actor, row.DepositorCharacterID,
				c.Quantity, utcDayStart(now), now); err != nil {
				return 0, err
			}
		}
		if err := audit(ctx, tx, guildID, opID, actor, "CLAIM_DELIVER",
			SectionReserve, row.ItemID, c.Quantity, &actor,
			&row.DepositorCharacterID, row.Quantity, row.Quantity-c.Quantity,
			now); err != nil {
			return 0, err
		}
		return bumpStorageRevision(ctx, tx, guildID)
	}
	return 0, ErrMalformedRecord
}

// transitionClaim applies a state transition, stamping resolved_at and
// preserving the claim's approver/expiry fields.
func (d Deps) transitionClaim(ctx context.Context, tx pgx.Tx, c *ClaimRow,
	state string, now time.Time, approver *id.UUID) error {
	return d.transitionClaimFull(ctx, tx, c, state, now, approver, c.ApprovedAt, c.ExpiresAt)
}

// transitionClaimFull updates state/resolved_at plus optional approval
// fields (APPROVE sets approver + approved_at + the 7-day expiry).
func (d Deps) transitionClaimFull(ctx context.Context, tx pgx.Tx, c *ClaimRow,
	state string, now time.Time, approver *id.UUID,
	approvedAt *time.Time, expiresAt time.Time) error {
	tag, err := tx.Exec(ctx,
		`UPDATE guild_storage_claims
		    SET state=$1, resolved_at=$2, approver_character_id=$3,
		        approved_at=$4, expires_at=$5
		  WHERE claim_id=$6 AND state=$7`,
		state, now, uuidOrNil(approver), approvedAt, expiresAt,
		c.ID, c.State)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errClaimNotDecidable
	}
	c.State = state
	c.ResolvedAt = &now
	return nil
}

// CancelClaimsForMember cancels every PENDING/APPROVED claim the member
// opened in the guild — the erasure-detach hook (data_model.md
// § Account Erasure step 6) plus the leave/kick path: invocable inside
// the caller's destructive tx; audits each cancel and bumps the
// storage revision once.
func (d Deps) CancelClaimsForMember(ctx context.Context, tx pgx.Tx,
	guildID, member, opID id.UUID, now time.Time) (int, error) {
	rows, err := tx.Query(ctx,
		`SELECT claim_id, item_instance_id, quantity, state
		   FROM guild_storage_claims
		  WHERE guild_id=$1 AND requester_character_id=$2
		    AND state IN ('PENDING','APPROVED') FOR UPDATE`,
		guildID, member)
	if err != nil {
		return 0, err
	}
	var open []ClaimRow
	for rows.Next() {
		var c ClaimRow
		if err := rows.Scan(&c.ID, &c.Instance, &c.Quantity, &c.State); err != nil {
			rows.Close()
			return 0, err
		}
		c.GuildID = guildID
		c.Requester = member
		open = append(open, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, c := range open {
		var itemID string
		if err := tx.QueryRow(ctx,
			`SELECT item_id FROM item_instances WHERE item_instance_id=$1`,
			c.Instance).Scan(&itemID); err != nil {
			return 0, err
		}
		if err := d.transitionClaim(ctx, tx, &c, ClaimCancelled, now, nil); err != nil {
			return 0, err
		}
		if err := audit(ctx, tx, guildID, opID, member, "CLAIM_CANCEL",
			SectionReserve, itemID, c.Quantity, nil, nil,
			c.Quantity, c.Quantity, now); err != nil {
			return 0, err
		}
	}
	if len(open) > 0 {
		if _, err := bumpStorageRevision(ctx, tx, guildID); err != nil {
			return 0, err
		}
	}
	return len(open), nil
}
