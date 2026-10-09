package guild_storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// JobFamily is the guild JOB producer family this sweep rides
// (job.<target> = guild, save_rules.md § Closed Durable Queue
// Producer Registry — the storage-claim expiry is one of the guild
// jobs; composition muxes kind inside the family).
const JobFamily = "job.guild"

// JobKindStorageClaimExpire is the sweep kind (closed JOB set).
const JobKindStorageClaimExpire = "STORAGE_CLAIM_EXPIRE"

// StorageClaimExpireJob is the JOB executor — sweeps the target
// guild's PENDING (>72h) and APPROVED (>7d) claims into EXPIRED with
// reservation release, and (r4) cancels open claims whose requester
// is no longer a member. Exported for the composition's job.guild mux.
func (d Deps) StorageClaimExpireJob(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != JobFamily {
		return idempotency.Outcome{}, fmt.Errorf("%w: family %q",
			ErrMalformedRecord, rec.GetOperationFamily())
	}
	job := rec.GetJob()
	tgt := job.GetGuild()
	if job == nil || tgt == nil || job.GetKind() != JobKindStorageClaimExpire ||
		len(tgt.GetGuildId()) != 16 {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	var guildID id.UUID
	copy(guildID[:], tgt.GetGuildId())
	if err := lockStorage(ctx, tx, guildID); err != nil {
		return idempotency.Outcome{}, err
	}
	now := d.Now()
	var opID id.UUID
	copy(opID[:], rec.GetOperationId())
	changed, err := d.sweepClaims(ctx, tx, guildID, opID, now)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	var rev uint64
	if changed > 0 {
		if rev, err = bumpStorageRevision(ctx, tx, guildID); err != nil {
			return idempotency.Outcome{}, err
		}
	}
	out := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: rec.GetOperationId(),
		Revisions: []*journalv1.JournalAggregateRevision{
			{Aggregate: "guild_storage", OwnerId: guildID[:], Revision: rev},
		},
	}
	payload, err := marshalOutcome(out)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: payload}, nil
}

// sweepClaims expires due claims (PENDING>72h, APPROVED>7d) and cancels
// open claims whose requester detached from the guild (r4). Returns the
// transitioned count.
func (d Deps) sweepClaims(ctx context.Context, tx pgx.Tx,
	guildID, opID id.UUID, now time.Time) (int, error) {
	rows, err := tx.Query(ctx,
		`SELECT c.claim_id, c.requester_character_id, c.item_instance_id,
		        c.quantity, c.state, i.item_id,
		        EXISTS(
		          SELECT 1 FROM guild_memberships gm
		           WHERE gm.character_id = c.requester_character_id
		             AND gm.guild_id = c.guild_id) AS requester_member
		   FROM guild_storage_claims c
		     JOIN item_instances i ON i.item_instance_id = c.item_instance_id
		  WHERE c.guild_id=$1 AND c.state IN ('PENDING','APPROVED')
		  ORDER BY c.created_at FOR UPDATE`,
		guildID)
	if err != nil {
		return 0, err
	}
	type sweepRow struct {
		c        ClaimRow
		itemID   string
		isMember bool
	}
	var open []sweepRow
	for rows.Next() {
		var r sweepRow
		if err := rows.Scan(&r.c.ID, &r.c.Requester, &r.c.Instance,
			&r.c.Quantity, &r.c.State, &r.itemID, &r.isMember); err != nil {
			rows.Close()
			return 0, err
		}
		r.c.GuildID = guildID
		open = append(open, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	changed := 0
	for _, r := range open {
		switch {
		case !r.isMember:
			// r4: leave/kick or erasure-detach removed the requester —
			// cancel the claim and release its reservation.
			if err := d.transitionClaim(ctx, tx, &r.c, ClaimCancelled, now, nil); err != nil {
				return 0, err
			}
			if err := audit(ctx, tx, guildID, opID, r.c.Requester,
				"CLAIM_CANCEL", SectionReserve, r.itemID, r.c.Quantity,
				nil, nil, r.c.Quantity, r.c.Quantity, now); err != nil {
				return 0, err
			}
			changed++
		case now.After(r.c.ExpiresAt):
			// PENDING>72h or APPROVED>7d: expire, release reservation,
			// item stays in GUILD_STORAGE.
			if err := d.transitionClaim(ctx, tx, &r.c, ClaimExpired, now, nil); err != nil {
				return 0, err
			}
			if err := audit(ctx, tx, guildID, opID, r.c.Requester,
				"CLAIM_EXPIRE", SectionReserve, r.itemID, r.c.Quantity,
				nil, nil, r.c.Quantity, r.c.Quantity, now); err != nil {
				return 0, err
			}
			changed++
		}
	}
	return changed, nil
}
