package guild

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// Blessing pool + fixed priorities (guild_progression.md § Blessings).
// Priority breaks draft/vote ties: lower number wins.
var blessingPriority = map[string]int{
	"guild.blessing.advancement": 1,
	"guild.blessing.endurance":   2,
	"guild.blessing.hunt":        3,
	"guild.blessing.craft":       4,
	"guild.blessing.exploration": 5,
	"guild.blessing.activity":    6,
}

// BlessingPool is the guild-level-gated candidate set.
func BlessingPool(level int) []string {
	pool := []string{
		"guild.blessing.advancement",
		"guild.blessing.hunt",
		"guild.blessing.exploration",
	}
	if level >= 10 {
		pool = append(pool, "guild.blessing.craft")
	}
	if level >= 20 {
		pool = append(pool, "guild.blessing.endurance")
	}
	if level >= 30 {
		pool = append(pool, "guild.blessing.activity")
	}
	return pool
}

// catalogRevision is the content-catalog revision pinned into the
// draft seed. IMP-036 has no catalog table; the string rides with the
// cycle row so the formula stays stable across replays.
const catalogRevision = "imp-036.launch"

// BlessingDuration is the 7-day blessing lifetime.
const BlessingDuration = 7 * 24 * time.Hour

// VoteWindow is the draft-vote duration.
const VoteWindow = 24 * time.Hour

// rankBlessing implements SHA-256(guild_id || ":" || cycle_id || ":"
// || catalog_revision || ":" || blessing_id) big-endian — the three
// lowest ranks are the draft candidates.
func rankBlessing(guildID id.UUID, cycleID, blessingID string) uint64 {
	h := sha256.New()
	fmt.Fprintf(h, "%s:%s:%s:%s", guildID.String(), cycleID, catalogRevision, blessingID)
	return binary.BigEndian.Uint64(h.Sum(nil))
}

// DraftBlessings returns the three lowest-ranked pool members.
func DraftBlessings(guildID id.UUID, cycleID string, level int) []string {
	pool := BlessingPool(level)
	sort.Slice(pool, func(i, j int) bool {
		ri, rj := rankBlessing(guildID, cycleID, pool[i]), rankBlessing(guildID, cycleID, pool[j])
		if ri != rj {
			return ri < rj
		}
		return blessingPriority[pool[i]] < blessingPriority[pool[j]]
	})
	if len(pool) > 3 {
		pool = pool[:3]
	}
	return pool
}

// ---------------------------------------------------------------------------
// job.guild executor — CYCLE_FREEZE / VOTE_FINALIZE / BLESSING_EXPIRE.

func (d Deps) guildJob(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != JobFamily {
		return idempotency.Outcome{}, fmt.Errorf("%w: family %q", ErrMalformedRecord, rec.GetOperationFamily())
	}
	job := rec.GetJob()
	tgt := job.GetGuild()
	if job == nil || tgt == nil || len(tgt.GetGuildId()) != 16 {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	var guildID id.UUID
	copy(guildID[:], tgt.GetGuildId())
	cycleID := tgt.GetCycleId()
	now := d.Now()
	switch job.GetKind() {
	case JobCycleFreeze:
		// Freeze the named cycle at cutoff: materialize if absent and
		// mark rows of cycles before now's cycle as closed for new
		// points (completed_at stays NULL when missed — tombstones
		// simply stop accepting votes/points).
		if err := lockProgression(ctx, tx, guildID); err != nil {
			return idempotency.Outcome{}, err
		}
		if err := d.materializeCycle(ctx, tx, guildID, now); err != nil {
			return idempotency.Outcome{}, err
		}
	case JobVoteFinalize:
		// Finalize the named cycle's draft: candidates must already be
		// stored (drafted at completion/freeze); pick the winner by
		// votes, ties -> fixed priority, zero -> highest priority.
		if err := lockProgression(ctx, tx, guildID); err != nil {
			return idempotency.Outcome{}, err
		}
		if err := d.finalizeVote(ctx, tx, guildID, cycleID, now); err != nil {
			return idempotency.Outcome{}, err
		}
	case JobBlessingExpire:
		if err := lockProgression(ctx, tx, guildID); err != nil {
			return idempotency.Outcome{}, err
		}
		p, err := d.Store.Progression(ctx, tx, guildID)
		if err != nil {
			return idempotency.Outcome{}, err
		}
		if p.ActiveBlessingID != "" && p.BlessingExpiresAt != nil && !p.BlessingExpiresAt.After(now) {
			if _, err := tx.Exec(ctx,
				`UPDATE guild_progression
				 SET active_blessing_id = NULL, blessing_expires_at = NULL, revision = revision + 1
				 WHERE guild_id = $1`, guildID); err != nil {
				return idempotency.Outcome{}, err
			}
		}
	default:
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	rev, err := d.Store.BumpRevision(ctx, tx, guildID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	payload, err := marshalOutcome(eventOutcome(
		[]*journalv1.JournalAggregateRevision{{Aggregate: "guild", OwnerId: guildID[:], Revision: rev}}))
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: payload}, nil
}

// DraftCycle stores the 3-candidate draft and opens the 24h vote.
// Called when a completed cycle freezes (and directly by tests).
func (d Deps) DraftCycle(ctx context.Context, tx pgx.Tx, guildID id.UUID, cycleID string, now time.Time) error {
	c, err := d.Store.Cycle(ctx, tx, guildID, cycleID)
	if err != nil {
		return err
	}
	if c == nil || c.Candidates != nil || c.CompletedAt == nil {
		return nil
	}
	p, err := d.Store.Progression(ctx, tx, guildID)
	if err != nil {
		return err
	}
	cands := DraftBlessings(guildID, cycleID, p.Level)
	closes := now.Add(VoteWindow)
	_, err = tx.Exec(ctx,
		`UPDATE guild_ritual_cycles
		 SET candidate_blessing_ids = $3, vote_closes_at = $4
		 WHERE guild_id = $1 AND cycle_id = $2`, guildID, cycleID, cands, closes)
	return err
}

// finalizeVote elects the blessing: most votes, ties and zero-vote
// drafts resolve by fixed priority (lower number wins).
func (d Deps) finalizeVote(ctx context.Context, tx pgx.Tx, guildID id.UUID, cycleID string, now time.Time) error {
	c, err := d.Store.Cycle(ctx, tx, guildID, cycleID)
	if err != nil {
		return err
	}
	if c == nil || len(c.Candidates) == 0 || c.FinalBlessingID != "" {
		return nil
	}
	counts, err := d.Store.VoteCounts(ctx, tx, guildID, cycleID, c.Candidates)
	if err != nil {
		return err
	}
	winner := ""
	bestVotes, bestPrio := -1, 1<<30
	for i, cand := range c.Candidates {
		if counts[i] > bestVotes ||
			(counts[i] == bestVotes && blessingPriority[cand] < bestPrio) {
			bestVotes, bestPrio, winner = counts[i], blessingPriority[cand], cand
		}
	}
	expires := now.Add(BlessingDuration)
	if _, err := tx.Exec(ctx,
		`UPDATE guild_ritual_cycles SET finalized_blessing_id = $3
		 WHERE guild_id = $1 AND cycle_id = $2`, guildID, cycleID, winner); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE guild_progression
		 SET active_blessing_id = $2, blessing_expires_at = $3, revision = revision + 1
		 WHERE guild_id = $1`, guildID, winner, expires); err != nil {
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// 648 C2S_GUILD_BLESSING_VOTE — one vote per account per draft, only
// while the window is open; voter must be a roster member still current.

func (d Deps) blessingVote(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, BlessingVoteFamily); err != nil {
		return idempotency.Outcome{}, err
	}
	opID, char, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	cmd := rec.GetClient()
	if err := cmdIdentity(cmd, char); err != nil {
		return idempotency.Outcome{}, err
	}
	req := cmd.GetC2SGuildBlessingVote()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	cycleID := req.GetCycleId()
	blessingID := req.GetBlessingId()
	now := admittedAt(rec)
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", char)); err != nil {
		return idempotency.Outcome{}, err
	}
	gid, m, err := d.guildIDOf(ctx, tx, char)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	if err := lockProgression(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	c, err := d.Store.Cycle(ctx, tx, gid, cycleID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if c == nil || len(c.Candidates) == 0 || c.VoteClosesAt == nil || !c.VoteClosesAt.After(now) || c.FinalBlessingID != "" {
		return fail(errVoteClosed)
	}
	eligible := false
	for _, cand := range c.Candidates {
		if cand == blessingID {
			eligible = true
		}
	}
	if !eligible {
		return fail(errNotCandidate)
	}
	// Roster + still-current membership.
	roster, err := d.Store.CycleMembers(ctx, tx, gid, cycleID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	var accountID id.UUID
	onRoster := false
	for _, r := range roster {
		if r.CharacterID == char {
			onRoster = true
			accountID = r.AccountID
		}
	}
	if !onRoster || m == nil {
		return fail(errVoteClosed)
	}
	if err := d.Store.CastVote(ctx, tx, gid, cycleID, accountID, char, blessingID, now); err != nil {
		return fail(err)
	}
	return d.success(rec, ctx, tx, opID, gid, nil)
}

var _ = protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED
var _ = idempotency.Outcome{}
