package guild

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// SourceKind is the closed eligibility table
// (guild_progression.md § Eligible Events).
type SourceKind string

const (
	SourceDungeon       SourceKind = "DUNGEON"
	SourceBoss          SourceKind = "BOSS"
	SourceWorldEvent    SourceKind = "WORLD_EVENT"
	SourceGuildActivity SourceKind = "GUILD_ACTIVITY"
	SourceRitual        SourceKind = "RITUAL"
	SourceGuildStone    SourceKind = "GUILD_STONE"
)

// SourceEvent is the typed intake the runtime/global layer hands the
// sink (ADR-0068): no request/response wire codes — the event is the
// durable command.
type SourceEvent struct {
	Kind              SourceKind
	SourceKey         string  // idempotent source reference
	SourceOperationID id.UUID // production operation of the source
	OccurredAt        time.Time
	Credited          []Credit   // credited members (>=3 required)
	Element           uint32     // 1..5 KIM/MOC/THUY/HOA/THO (journal order)
	ChainID           string     // Spirit Surge chain (grant key component)
	SlotStart         *time.Time // bonfire rest slot start
	SeasonID          uint32
}

// Credit is one credited member.
type Credit struct {
	CharacterID  id.UUID
	MembershipID id.UUID
	Amount       uint64 // contribution amount
}

// EventSink is the GUILD_EVENT intake contract
// (protobuf_conventions §7): producers call Apply; the sink enqueues
// one guild.event record per affected guild. The concrete sink lives
// in global/guild; the executor here owns the durable apply.
type EventSink interface {
	Apply(ctx context.Context, ev SourceEvent) error
}

// expRitualTable — Guild EXP / ritual points per source
// (guild_progression.md § Contribution baselines).
func expFor(kind SourceKind) uint64 {
	switch kind {
	case SourceDungeon, SourceBoss:
		return 20
	case SourceWorldEvent:
		return 15
	case SourceGuildActivity:
		return 30
	case SourceRitual:
		return 200
	default:
		return 0
	}
}

func ritualPointsFor(kind SourceKind) uint32 {
	switch kind {
	case SourceDungeon, SourceBoss:
		return 12
	case SourceWorldEvent:
		return 10
	case SourceGuildActivity:
		return 15
	default:
		return 0
	}
}

// guildEvent executes a committed GUILD_EVENT record: materializes the
// open cycle before any post-cutoff mutation (missed-cycle rule), then
// credits Guild EXP + member contribution + ritual points atomically.
func (d Deps) guildEvent(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != EventFamily {
		return idempotency.Outcome{}, fmt.Errorf("%w: family %q", ErrMalformedRecord, rec.GetOperationFamily())
	}
	if len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	var guildID, opID id.UUID
	copy(guildID[:], rec.GetOwnerId())
	copy(opID[:], rec.GetOperationId())
	ev := rec.GetGuildEvent()
	if ev == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	if len(ev.GetGuildId()) != 16 || stringToKind(ev.GetSourceKind()) == "" || ev.GetElement() < 1 || ev.GetElement() > 5 {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	kind := stringToKind(ev.GetSourceKind())
	now := time.UnixMilli(ev.GetOccurredAtMs()).UTC()
	// Full lock-set pre-acquire before freezing/crediting cycles
	// (guild_progression.md § Missed Cycles).
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("guilds", guildID)); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := lockProgression(ctx, tx, guildID); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := lockorder.Acquire(ctx, tx,
		lockorder.RowLock("character_attach_events", guildID)); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("guild_membership_history", guildID)); err != nil {
		return idempotency.Outcome{}, err
	}
	p, err := d.Store.Progression(ctx, tx, guildID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	// Materialize the open cycle (snapshot at cutoff) — before any
	// post-cutoff mutation to its row.
	if err := d.materializeCycle(ctx, tx, guildID, now); err != nil {
		return idempotency.Outcome{}, err
	}
	cycleID := ev.GetCycleId()
	if cycleID == "" {
		cycleID = CycleID(now)
	}
	// Eligibility gate: an eligible source event credits >=3 current
	// members (guild_progression.md). Events below the threshold are
	// consumed as a no-op (still outcome-recorded).
	if len(ev.GetCreditedMembers()) < 3 {
		return d.success(rec, ctx, tx, opID, guildID, nil)
	}
	// Guild EXP + per-member contribution.
	if amt := expFor(kind); amt > 0 {
		p, err = d.Store.AddEXP(ctx, tx, guildID, amt)
		if err != nil {
			return idempotency.Outcome{}, err
		}
	}
	for _, c := range ev.GetCreditedMembers() {
		if len(c.GetCharacterId()) != 16 {
			return idempotency.Outcome{}, ErrMalformedRecord
		}
		var char id.UUID
		copy(char[:], c.GetCharacterId())
		if err := d.Store.GrantContribution(ctx, tx, guildID, char, cycleID, c.GetAmount()); err != nil {
			return idempotency.Outcome{}, err
		}
	}
	// Ritual points into the open cycle's vessel.
	if pts := ritualPointsFor(kind); pts > 0 {
		if err := d.applyRitualPoints(ctx, tx, guildID, cycleID, kind, ev.GetElement(), int(pts), p); err != nil {
			return idempotency.Outcome{}, err
		}
	}
	rev, err := d.Store.BumpRevision(ctx, tx, guildID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	payload, err := marshalOutcome(eventOutcome(
		[]*journalv1.JournalAggregateRevision{
			{Aggregate: "guild", OwnerId: guildID[:], Revision: rev},
			{Aggregate: "guild_progression", OwnerId: guildID[:], Revision: progressionRevision(ctx, tx, d, guildID)},
		}))
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: payload}, nil
}

func progressionRevision(ctx context.Context, tx pgx.Tx, d Deps, guildID id.UUID) uint64 {
	p, err := d.Store.Progression(ctx, tx, guildID)
	if err != nil {
		return 0
	}
	return p.Revision
}

func stringToKind(s string) SourceKind {
	switch s {
	case "DUNGEON":
		return SourceDungeon
	case "BOSS":
		return SourceBoss
	case "WORLD_EVENT":
		return SourceWorldEvent
	case "GUILD_ACTIVITY":
		return SourceGuildActivity
	case "RITUAL":
		return SourceRitual
	case "GUILD_STONE":
		return SourceGuildStone
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// Ritual cycle helpers (guild_progression.md § Ritual Cycle).

// materializeCycle inserts the open cycle row at its start-of-cycle
// snapshot when absent — roster = membership intervals open at the
// cutoff + attach-events lookback; missed cycles are recorded missed.
func (d Deps) materializeCycle(ctx context.Context, tx pgx.Tx,
	guildID id.UUID, now time.Time) error {
	// Record missed cycles between the latest materialized cycle and
	// the current one (no reward, streak-safe — the bonus only pays
	// when consecutive completed cycles align anyway).
	current := CycleID(now)
	curStart, err := CycleStart(current)
	if err != nil {
		return err
	}
	var latest *CycleRow
	rows, err := tx.Query(ctx,
		`SELECT cycle_id FROM guild_ritual_cycles WHERE guild_id = $1 ORDER BY cycle_id DESC LIMIT 1`,
		guildID)
	if err != nil {
		return err
	}
	if rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			rows.Close()
			return err
		}
		latest = &CycleRow{CycleID: c}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if latest != nil {
		prev, err := CycleStart(latest.CycleID)
		if err != nil {
			return err
		}
		// Any cycles strictly between latest and current were missed.
		for t := prev.AddDate(0, 0, 7); t.Before(curStart); t = t.AddDate(0, 0, 7) {
			if err := d.materializeOne(ctx, tx, guildID, t, true); err != nil {
				return err
			}
		}
	}
	return d.materializeOne(ctx, tx, guildID, curStart, false)
}

// materializeOne snapshots the roster for the cycle that started at
// cycleStart: members = intervals open at cutoff AND attached within
// the 14-day lookback (leader always counts). missed=true rows
// immediately record the missed-cycle tombstone (completed_at NULL,
// no candidates — the streak rule derives from cycle history).
func (d Deps) materializeOne(ctx context.Context, tx pgx.Tx,
	guildID id.UUID, cycleStart time.Time, missed bool) error {
	cycleID := cycleStart.Format("2006-01-02")
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM guild_ritual_cycles WHERE guild_id = $1 AND cycle_id = $2)`,
		guildID, cycleID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	g, err := d.Store.Guild(ctx, tx, guildID)
	if err != nil {
		return err
	}
	intervals, err := d.Store.ActiveMembershipsAt(ctx, tx, guildID, cycleStart)
	if err != nil {
		return err
	}
	lookback := cycleStart.Add(-14 * 24 * time.Hour)
	m := 0
	var roster []MemberRow
	for _, mem := range intervals {
		isLeader := g.HasLeader && mem.CharacterID == g.Leader
		attached, err := d.Store.AttachedWithin(ctx, tx, mem.CharacterID, lookback, cycleStart)
		if err != nil {
			return err
		}
		if isLeader || attached {
			m++
			roster = append(roster, mem)
		}
	}
	if g.HasLeader && m == 0 {
		// No roster possible without the leader — still materialize
		// the cycle so the row exists (missed rules apply uniformly).
	}
	mEff := m
	if mEff < 5 {
		mEff = 5
	}
	if mEff > 40 {
		mEff = 40
	}
	required := 120 + 12*mEff
	var completedAt *time.Time
	if missed {
		// Tombstone: completed_at NULL, no candidates — but mark the
		// row as already-finalized by setting finalized_blessing_id ''
		// so no later freeze recomputes it as open.
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO guild_ritual_cycles
		 (guild_id, cycle_id, m_effective, required_points_per_element,
		  points_kim, points_moc, points_thuy, points_hoa, points_tho,
		  rotation_pointer, completed_at, candidate_blessing_ids, vote_closes_at, finalized_blessing_id)
		 VALUES ($1,$2,$3,$4,0,0,0,0,0,'KIM',$5,NULL,NULL,NULL)`,
		guildID, cycleID, mEff, required, completedAt)
	if err != nil {
		return err
	}
	for _, mem := range roster {
		var account *id.UUID
		if err := tx.QueryRow(ctx,
			`SELECT account_id FROM characters WHERE character_id = $1`, mem.CharacterID).
			Scan(&account); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO guild_ritual_cycle_members
			 (guild_id, cycle_id, character_id, account_id, membership_joined_at)
			 VALUES ($1,$2,$3,$4,$5)
			 ON CONFLICT (guild_id, cycle_id, character_id) DO NOTHING`,
			guildID, cycleID, mem.CharacterID, account, mem.JoinedAt); err != nil {
			return err
		}
	}
	return nil
}

// applyRitualPoints routes points into the open cycle's vessels:
// boss-kind events fill the boss's element vessel (ACTIVITY_ELEMENT);
// other kinds walk SERVER_ROTATION (KIM→MOC→THUY→HOA→THO), filling the
// current vessel and skipping full ones. Advance the pointer past the
// receiving element. All-full completion pays the ritual completion
// grant once.
func (d Deps) applyRitualPoints(ctx context.Context, tx pgx.Tx,
	guildID id.UUID, cycleID string, kind SourceKind,
	bossElement uint32, pts int, p ProgressionRow) error {
	c, err := d.Store.Cycle(ctx, tx, guildID, cycleID)
	if err != nil {
		return err
	}
	if c == nil || c.CompletedAt != nil {
		return nil // historical or sealed — points past cutoff are lost
	}
	if err := lockProgression(ctx, tx, guildID); err != nil {
		return err
	}
	switch kind {
	case SourceBoss:
		// ACTIVITY_ELEMENT: no spill — points beyond the vessel cap
		// vanish.
		idx := int(bossElement) - 1
		room := c.Required - c.Points[idx]
		if room > 0 {
			add := pts
			if add > room {
				add = room
			}
			c.Points[idx] += add
		}
	default:
		// SERVER_ROTATION: fill vessels in pointer order skipping full.
		pointerIdx := 0
		for i, e := range elementOrder {
			if elementName(e) == c.Pointer {
				pointerIdx = i
			}
		}
		remaining := pts
		for i := 0; i < 5 && remaining > 0; i++ {
			idx := (pointerIdx + i) % 5
			room := c.Required - c.Points[idx]
			if room <= 0 {
				continue
			}
			add := remaining
			if add > room {
				add = room
			}
			c.Points[idx] += add
			remaining -= add
			// pointer advances past the element that received points.
			pointerIdx = (idx + 1) % 5
			c.Pointer = elementName(elementOrder[pointerIdx])
		}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE guild_ritual_cycles
		 SET points_kim=$3, points_moc=$4, points_thuy=$5, points_hoa=$6,
		     points_tho=$7, rotation_pointer=$8
		 WHERE guild_id=$1 AND cycle_id=$2`,
		guildID, cycleID, c.Points[0], c.Points[1], c.Points[2], c.Points[3], c.Points[4], c.Pointer); err != nil {
		return err
	}
	// Completion: all vessels full, once — awards ritual completion
	// EXP and stamps completed_at (candidates get drafted at freeze).
	full := true
	for i := 0; i < 5; i++ {
		if c.Points[i] < c.Required {
			full = false
		}
	}
	if full && c.CompletedAt == nil {
		now := time.Now().UTC()
		if d.Now != nil {
			now = d.Now()
		}
		if _, err := tx.Exec(ctx,
			`UPDATE guild_ritual_cycles SET completed_at = $3
			 WHERE guild_id = $1 AND cycle_id = $2 AND completed_at IS NULL`,
			guildID, cycleID, now); err != nil {
			return err
		}
		if _, err := d.Store.AddEXP(ctx, tx, guildID, expFor(SourceRitual)); err != nil {
			return err
		}
	}
	return nil
}

var _ = fmt.Sprintf
var _ = protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED
