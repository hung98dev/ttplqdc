package guild

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/graphemes"
	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/currency"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// WarRegistrationGuard is the disband precondition consult
// (guild.md § Disband): true while any Guild War registration or match
// is QUEUED..RESOLVING. IMP-042 implements it; IMP-069 wires it; tests
// use synthetic states.
type WarRegistrationGuard interface {
	Blocked(ctx context.Context, tx pgx.Tx, guildID id.UUID) (bool, error)
}

// Deps are the executor dependencies.
type Deps struct {
	Store    *Store
	Now      func() time.Time
	Guard    WarRegistrationGuard
	newGuild func() id.UUID
}

// Executors returns one queue.Executor per guild client.<ID> family
// plus the GUILD_EVENT and job.guild producers (family-muxed by the
// composition root, ADR-0081 — the package never self-registers).
func Executors(d Deps) map[string]queue.Executor {
	now := d.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	if d.newGuild == nil {
		d.newGuild = id.NewV4
	}
	d.Now = now
	return map[string]queue.Executor{
		InviteFamily:            d.invite,
		AcceptFamily:            d.accept,
		DeclineFamily:           d.decline,
		LeaveFamily:             d.leave,
		KickFamily:              d.kick,
		RoleUpdateFamily:        d.roleUpdate,
		LeaderTransferFamily:    d.leaderTransfer,
		CreateFamily:            d.create,
		DisbandFamily:           d.disband,
		ApplyFamily:             d.apply,
		ApplicationDecideFamily: d.applicationDecide,
		MotdSetFamily:           d.motdSet,
		LeadershipClaimFamily:   d.leadershipClaim,
		BlessingVoteFamily:      d.blessingVote,
		SettingsSetFamily:       d.settingsSet,
		InviteCancelFamily:      d.inviteCancel,
		ApplicationCancelFamily: d.applicationCancel,
		EventFamily:             d.guildEvent,
		JobFamily:               d.guildJob,
	}
}

// identity validates the record's character-owned identity fields.
func identity(rec *journalv1.DurableCommandRecord) (opID, characterID id.UUID, err error) {
	if len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 {
		return opID, characterID, ErrMalformedRecord
	}
	copy(opID[:], rec.GetOperationId())
	copy(characterID[:], rec.GetOwnerId())
	return opID, characterID, nil
}

// cmdIdentity cross-checks the JournalClientCommand identity against
// the record owner.
func cmdIdentity(cmd *journalv1.JournalClientCommand, characterID id.UUID) error {
	if cmd == nil {
		return ErrMalformedRecord
	}
	if len(cmd.GetCharacterId()) != 16 {
		return ErrMalformedRecord
	}
	var cid id.UUID
	copy(cid[:], cmd.GetCharacterId())
	if cid != characterID {
		return fmt.Errorf("%w: owner %v vs character %v", ErrMalformedRecord, characterID, cid)
	}
	return nil
}

func targetID(b []byte) (id.UUID, error) {
	var t id.UUID
	if len(b) != 16 {
		return t, fmt.Errorf("%w: target %d bytes", ErrMalformedRecord, len(b))
	}
	copy(t[:], b)
	return t, nil
}

// lockCharacters canonicalizes a (caller, target) character lock pair.
func lockCharacters(ctx context.Context, tx pgx.Tx, a, b id.UUID) error {
	if b.String() < a.String() {
		a, b = b, a
	}
	return lockorder.Acquire(ctx, tx,
		lockorder.RowLock("characters", a), lockorder.RowLock("characters", b))
}

func admittedAt(rec *journalv1.DurableCommandRecord) time.Time {
	return time.UnixMilli(rec.GetEnqueuedAtMs()).UTC()
}

// family checks the record is bound for the expected family.
func family(rec *journalv1.DurableCommandRecord, want string) error {
	if rec.GetOperationFamily() != want {
		return fmt.Errorf("%w: family %q", ErrMalformedRecord, rec.GetOperationFamily())
	}
	return nil
}

// ---------------------------------------------------------------------------
// Verdict plumbing: domain errors map to a 649 ERROR outcome; the
// success path emits SUCCESS + revision/created rows.

func (d Deps) verdict(rec *journalv1.DurableCommandRecord, opID id.UUID,
	guildID id.UUID, err error) (idempotency.Outcome, error) {
	code := codeOf(err)
	out := outcomeResult(protocolv1.ResultStatus_RESULT_STATUS_ERROR,
		opID, code, rec.GetOperationFamily(), guildID, nil, nil)
	payload, merr := marshalOutcome(out)
	if merr != nil {
		return idempotency.Outcome{}, merr
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: payload}, nil
}

func (d Deps) success(rec *journalv1.DurableCommandRecord, ctx context.Context,
	tx pgx.Tx, opID, guildID id.UUID,
	created []*journalv1.JournalCreatedId) (idempotency.Outcome, error) {
	var revs []*journalv1.JournalAggregateRevision
	if !guildID.IsNil() {
		rev, err := d.Store.BumpRevision(ctx, tx, guildID)
		if err != nil {
			return idempotency.Outcome{}, err
		}
		revs = []*journalv1.JournalAggregateRevision{
			{Aggregate: "guild", OwnerId: guildID[:], Revision: rev},
		}
	}
	out := outcomeResult(protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		opID, protocolv1.ErrorCode_ERROR_CODE_UNSPECIFIED,
		rec.GetOperationFamily(), guildID, revs, created)
	payload, err := marshalOutcome(out)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: payload}, nil
}

// codeOf maps domain errors onto wire codes (errors.md domain list).
func codeOf(err error) protocolv1.ErrorCode {
	switch {
	case errors.Is(err, errGuildNameTaken):
		return protocolv1.ErrorCode_ERROR_CODE_GUILD_NAME_TAKEN
	case errors.Is(err, ErrGuildNameInvalid):
		return protocolv1.ErrorCode_ERROR_CODE_GUILD_NAME_INVALID
	case errors.Is(err, errLevelTooLow):
		return protocolv1.ErrorCode_ERROR_CODE_LEVEL_TOO_LOW
	case errors.Is(err, errPermissionDenied):
		return protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED
	case errors.Is(err, errCapacityFull), errors.Is(err, errViceCapacityFull):
		return protocolv1.ErrorCode_ERROR_CODE_CAPACITY_FULL
	case errors.Is(err, errDisbandBlocked):
		return protocolv1.ErrorCode_ERROR_CODE_GUILD_DISBAND_BLOCKED
	case errors.Is(err, errInsufficientFunds):
		return protocolv1.ErrorCode_ERROR_CODE_INSUFFICIENT_CURRENCY
	default:
		return protocolv1.ErrorCode_ERROR_CODE_STATE_CONFLICT
	}
}

// ---------------------------------------------------------------------------
// Shared row plumbing.

// checkJoinable revalidates the acceptance-time join gates
// (guild.md § Applications): level >= 10, no current guild, capacity.
func (d Deps) checkJoinable(ctx context.Context, tx pgx.Tx, char, guildID id.UUID) error {
	level, _, err := d.Store.CharacterLevel(ctx, tx, char)
	if err != nil {
		return err
	}
	if level < MinJoinLevel {
		return errLevelTooLow
	}
	cur, err := d.Store.Member(ctx, tx, char)
	if err != nil {
		return err
	}
	if cur != nil {
		return errAlreadyInGuild
	}
	g, err := d.Store.Guild(ctx, tx, guildID)
	if err != nil {
		return err
	}
	if g.State != "ACTIVE" {
		return errStateConflict
	}
	n, err := d.Store.MemberCount(ctx, tx, guildID)
	if err != nil {
		return err
	}
	p, err := d.Store.Progression(ctx, tx, guildID)
	if err != nil {
		return err
	}
	if n >= MemberCapacity(p.Level) {
		return errCapacityFull
	}
	return nil
}

// join inserts the MEMBER membership + audit row.
func (d Deps) join(ctx context.Context, tx pgx.Tx, guildID, char id.UUID, now time.Time) error {
	if err := d.Store.AddMember(ctx, tx, guildID, char, RoleMember, id.NewV4(), now); err != nil {
		return err
	}
	return d.Store.InsertAudit(ctx, tx, id.NewV4(), char, "JOIN", guildID, id.UUID{},
		fmt.Sprintf(`{"guild_id":%q}`, guildID.String()), now)
}

// guildIDOf returns the actor's current guild or errNotMember.
func (d Deps) guildIDOf(ctx context.Context, tx pgx.Tx, char id.UUID) (id.UUID, *MemberRow, error) {
	m, err := d.Store.Member(ctx, tx, char)
	if err != nil {
		return id.UUID{}, nil, err
	}
	if m == nil {
		return id.UUID{}, nil, errNotMember
	}
	return m.GuildID, m, nil
}

// ---------------------------------------------------------------------------
// 637 C2S_GUILD_CREATE — level >= 20, no guild, debit 10,000 common,
// guild + LEADER membership + progression row atomically.

func (d Deps) create(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, CreateFamily); err != nil {
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
	req := cmd.GetC2SGuildCreate()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	now := admittedAt(rec)
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", char)); err != nil {
		return idempotency.Outcome{}, err
	}
	level, _, err := d.Store.CharacterLevel(ctx, tx, char)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	var gid id.UUID
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	if level < MinCreateLevel {
		return fail(errLevelTooLow)
	}
	cur, err := d.Store.Member(ctx, tx, char)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if cur != nil {
		return fail(errAlreadyInGuild)
	}
	display, nameKey, err := NormalizeGuildName(req.GetGuildName())
	if err != nil {
		return fail(err)
	}
	gid = d.newGuild()
	// The name_key gate is evaluated under the characters lock plus the
	// UNIQUE constraint — the name stays claimed forever once written.
	var taken bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM guilds WHERE name_key = $1)`, nameKey).Scan(&taken); err != nil {
		return idempotency.Outcome{}, err
	}
	if taken {
		return fail(errGuildNameTaken)
	}
	if _, err := currency.Debit(ctx, tx, currency.Mutation{
		CharacterID: char,
		CurrencyID:  currency.Common,
		Delta:       CreateCostCommon,
		OperationID: opID,
		ReasonCode:  "guild.create",
		SourceRef:   "guild:" + nameKey,
		Actor:       currency.ActorPlayer,
	}); err != nil {
		if errors.Is(err, currency.ErrInsufficientBalance) {
			return fail(errInsufficientFunds)
		}
		return idempotency.Outcome{}, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO guilds (guild_id, name, name_key, state, recruitment_mode,
		 leader_character_id, motd, guild_revision, guild_storage_revision, created_at)
		 VALUES ($1,$2,$3,'ACTIVE','CLOSED',$4,'',0,0,$5)`,
		gid, display, nameKey, char, now); err != nil {
		return idempotency.Outcome{}, err
	}
	membershipID := id.NewV4()
	if err := d.Store.AddMember(ctx, tx, gid, char, RoleLeader, membershipID, now); err != nil {
		return idempotency.Outcome{}, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO guild_progression (guild_id, guild_exp, guild_level, ritual_streak, revision)
		 VALUES ($1,0,1,0,0)`, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := d.Store.InsertAudit(ctx, tx, id.NewV4(), char, "CREATE", gid, opID,
		fmt.Sprintf(`{"guild_id":%q,"name":%q}`, gid.String(), display), now); err != nil {
		return idempotency.Outcome{}, err
	}
	created := []*journalv1.JournalCreatedId{{Kind: "GUILD", Id: gid[:]}}
	return d.success(rec, ctx, tx, opID, gid, created)
}

// ---------------------------------------------------------------------------
// 608 C2S_GUILD_INVITE — OFFICER+ only; target level >=10, guildless.

func (d Deps) invite(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, InviteFamily); err != nil {
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
	req := cmd.GetC2SGuildInvite()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	target, err := targetID(req.GetTargetCharacterId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	now := admittedAt(rec)
	if err := lockCharacters(ctx, tx, char, target); err != nil {
		return idempotency.Outcome{}, err
	}
	gid, m, err := d.guildIDOf(ctx, tx, char)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	if roleRank(m.Role) < roleRank(RoleOfficer) {
		return fail(errPermissionDenied)
	}
	if err := lockGuilds(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	level, _, err := d.Store.CharacterLevel(ctx, tx, target)
	if err != nil {
		return fail(errTargetInvalid())
	}
	if level < MinJoinLevel {
		return fail(errLevelTooLow)
	}
	tm, err := d.Store.Member(ctx, tx, target)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if tm != nil {
		return fail(errAlreadyInGuild)
	}
	if inv, err := d.Store.PendingInvite(ctx, tx, gid, target, now); err != nil {
		return idempotency.Outcome{}, err
	} else if inv != nil {
		return fail(errStateConflict)
	}
	if err := d.Store.InsertInvite(ctx, tx, id.NewV4(), gid, char, target, now); err != nil {
		return idempotency.Outcome{}, err
	}
	return d.success(rec, ctx, tx, opID, gid, nil)
}

func errTargetInvalid() error { return errNotMember }

// 610 C2S_GUILD_ACCEPT — live pending invite; revalidates join gates.
func (d Deps) accept(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	return d.resolveInviteJoin(ctx, tx, rec, AcceptFamily, true)
}

// 623 C2S_GUILD_DECLINE.
func (d Deps) decline(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	return d.resolveInviteJoin(ctx, tx, rec, DeclineFamily, false)
}

func (d Deps) resolveInviteJoin(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord, fam string, accept bool) (idempotency.Outcome, error) {
	if err := family(rec, fam); err != nil {
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
	var gidBytes []byte
	var reqOp []byte
	switch fam {
	case AcceptFamily:
		r := cmd.GetC2SGuildAccept()
		if r == nil {
			return idempotency.Outcome{}, ErrMalformedRecord
		}
		gidBytes, reqOp = r.GetGuildId(), r.GetOperationId()
	default:
		r := cmd.GetC2SGuildDecline()
		if r == nil {
			return idempotency.Outcome{}, ErrMalformedRecord
		}
		gidBytes, reqOp = r.GetGuildId(), r.GetOperationId()
	}
	if len(reqOp) != 16 {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	gid, err := targetID(gidBytes)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	now := admittedAt(rec)
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", char)); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := lockGuilds(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	inv, err := d.Store.PendingInvite(ctx, tx, gid, char, now)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if inv == nil {
		return fail(errInviteNotFound)
	}
	if !accept {
		if err := d.Store.ResolveInvite(ctx, tx, inv.ID, "DECLINED", now); err != nil {
			return idempotency.Outcome{}, err
		}
		return d.success(rec, ctx, tx, opID, gid, nil)
	}
	if err := d.checkJoinable(ctx, tx, char, gid); err != nil {
		return fail(err)
	}
	if err := d.Store.ResolveInvite(ctx, tx, inv.ID, "ACCEPTED", now); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := d.join(ctx, tx, gid, char, now); err != nil {
		return idempotency.Outcome{}, err
	}
	return d.success(rec, ctx, tx, opID, gid, nil)
}

// 651 C2S_GUILD_INVITE_CANCEL — inviter, LEADER or VICE_LEADER.
func (d Deps) inviteCancel(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, InviteCancelFamily); err != nil {
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
	req := cmd.GetC2SGuildInviteCancel()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	target, err := targetID(req.GetTargetCharacterId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	now := admittedAt(rec)
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", char)); err != nil {
		return idempotency.Outcome{}, err
	}
	gid, m, err := d.guildIDOf(ctx, tx, char)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	if err := lockGuilds(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	inv, err := d.Store.PendingInvite(ctx, tx, gid, target, now)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if inv == nil {
		return fail(errInviteNotFound)
	}
	// inviter / LEADER / VICE_LEADER (ADR-0060).
	if inv.Inviter != char && roleRank(m.Role) < roleRank(RoleViceLeader) {
		return fail(errPermissionDenied)
	}
	if err := d.Store.ResolveInvite(ctx, tx, inv.ID, "CANCELLED", now); err != nil {
		return idempotency.Outcome{}, err
	}
	return d.success(rec, ctx, tx, opID, gid, nil)
}

// ---------------------------------------------------------------------------
// 639/640/652 applications.

func (d Deps) apply(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, ApplyFamily); err != nil {
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
	req := cmd.GetC2SGuildApply()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	gid, err := targetID(req.GetGuildId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	now := admittedAt(rec)
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", char)); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := lockGuilds(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	g, err := d.Store.Guild(ctx, tx, gid)
	if err != nil {
		return fail(errTargetInvalid())
	}
	if g.State != "ACTIVE" {
		return fail(errStateConflict)
	}
	if g.RecruitmentMode != "APPLICATIONS" {
		return fail(errPermissionDenied)
	}
	level, _, err := d.Store.CharacterLevel(ctx, tx, char)
	if err != nil {
		return fail(errTargetInvalid())
	}
	if level < MinJoinLevel {
		return fail(errLevelTooLow)
	}
	cur, err := d.Store.Member(ctx, tx, char)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if cur != nil {
		return fail(errAlreadyInGuild)
	}
	if app, err := d.Store.PendingApplication(ctx, tx, gid, char, now); err != nil {
		return idempotency.Outcome{}, err
	} else if app != nil {
		return fail(errStateConflict)
	}
	n, err := d.Store.PendingApplicationCount(ctx, tx, char, now)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if n >= MaxPendingApplications {
		return fail(errApplicationCap)
	}
	if err := d.Store.InsertApplication(ctx, tx, id.NewV4(), gid, char, now); err != nil {
		return idempotency.Outcome{}, err
	}
	return d.success(rec, ctx, tx, opID, gid, nil)
}

func (d Deps) applicationDecide(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, ApplicationDecideFamily); err != nil {
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
	req := cmd.GetC2SGuildApplicationDecide()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	applicant, err := targetID(req.GetApplicantCharacterId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	dec := req.GetDecision()
	if dec != protocolv1.GuildApplicationDecision_GUILD_APPLICATION_DECISION_ACCEPT &&
		dec != protocolv1.GuildApplicationDecision_GUILD_APPLICATION_DECISION_REJECT {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	now := admittedAt(rec)
	if err := lockCharacters(ctx, tx, char, applicant); err != nil {
		return idempotency.Outcome{}, err
	}
	gid, m, err := d.guildIDOf(ctx, tx, char)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	if roleRank(m.Role) < roleRank(RoleOfficer) {
		return fail(errPermissionDenied)
	}
	if err := lockGuilds(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	app, err := d.Store.PendingApplication(ctx, tx, gid, applicant, now)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if app == nil {
		return fail(errApplicationNotFound)
	}
	if dec == protocolv1.GuildApplicationDecision_GUILD_APPLICATION_DECISION_REJECT {
		if err := d.Store.ResolveApplication(ctx, tx, app.ID, "REJECTED", char, now); err != nil {
			return idempotency.Outcome{}, err
		}
		return d.success(rec, ctx, tx, opID, gid, nil)
	}
	if err := d.checkJoinable(ctx, tx, applicant, gid); err != nil {
		return fail(err)
	}
	if err := d.Store.ResolveApplication(ctx, tx, app.ID, "ACCEPTED", char, now); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := d.join(ctx, tx, gid, applicant, now); err != nil {
		return idempotency.Outcome{}, err
	}
	return d.success(rec, ctx, tx, opID, gid, nil)
}

func (d Deps) applicationCancel(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, ApplicationCancelFamily); err != nil {
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
	req := cmd.GetC2SGuildApplicationCancel()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	gid, err := targetID(req.GetGuildId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	now := admittedAt(rec)
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", char)); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := lockGuilds(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	app, err := d.Store.PendingApplication(ctx, tx, gid, char, now)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if app == nil {
		return fail(errApplicationNotFound)
	}
	if err := d.Store.ResolveApplication(ctx, tx, app.ID, "CANCELLED", char, now); err != nil {
		return idempotency.Outcome{}, err
	}
	return d.success(rec, ctx, tx, opID, gid, nil)
}

// ---------------------------------------------------------------------------
// 624/625/626/627/638 membership + lifecycle.

func (d Deps) leave(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, LeaveFamily); err != nil {
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
	if cmd.GetC2SGuildLeave() == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	now := admittedAt(rec)
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", char)); err != nil {
		return idempotency.Outcome{}, err
	}
	gid, m, err := d.guildIDOf(ctx, tx, char)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	if err := lockGuilds(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	if m.Role == RoleLeader {
		n, err := d.Store.MemberCount(ctx, tx, gid)
		if err != nil {
			return idempotency.Outcome{}, err
		}
		if n > 1 {
			// must transfer first (guild.md § Leave).
			return fail(errStateConflict)
		}
		// sole-leader leave = disband request.
		if err := d.checkDisband(ctx, tx, gid); err != nil {
			return fail(errDisbandBlocked)
		}
		return d.commitDisband(ctx, tx, rec, opID, gid, char, now)
	}
	if err := d.Store.RemoveMember(ctx, tx, gid, char, now); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := d.Store.InsertAudit(ctx, tx, id.NewV4(), char, "LEAVE", gid, opID,
		fmt.Sprintf(`{"guild_id":%q}`, gid.String()), now); err != nil {
		return idempotency.Outcome{}, err
	}
	return d.success(rec, ctx, tx, opID, gid, nil)
}

func (d Deps) kick(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, KickFamily); err != nil {
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
	req := cmd.GetC2SGuildKick()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	target, err := targetID(req.GetTargetCharacterId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	now := admittedAt(rec)
	if err := lockCharacters(ctx, tx, char, target); err != nil {
		return idempotency.Outcome{}, err
	}
	gid, m, err := d.guildIDOf(ctx, tx, char)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	if char == target {
		return fail(errPermissionDenied)
	}
	if err := lockGuilds(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	tm, err := d.Store.MemberIn(ctx, tx, target, gid)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if tm == nil {
		return fail(errTargetInvalid())
	}
	// strictly lower role only (guild.md § Kick).
	if roleRank(m.Role) <= roleRank(tm.Role) {
		return fail(errPermissionDenied)
	}
	if err := d.Store.RemoveMember(ctx, tx, gid, target, now); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := d.Store.InsertAudit(ctx, tx, id.NewV4(), char, "KICK", gid, opID,
		fmt.Sprintf(`{"guild_id":%q,"target":%q}`, gid.String(), target.String()), now); err != nil {
		return idempotency.Outcome{}, err
	}
	return d.success(rec, ctx, tx, opID, gid, nil)
}

func (d Deps) roleUpdate(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, RoleUpdateFamily); err != nil {
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
	req := cmd.GetC2SGuildRoleUpdate()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	target, err := targetID(req.GetTargetCharacterId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	newRole, ok := roleIDs[req.GetNewRole()]
	if !ok || newRole == RoleLeader {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	now := admittedAt(rec)
	if err := lockCharacters(ctx, tx, char, target); err != nil {
		return idempotency.Outcome{}, err
	}
	gid, m, err := d.guildIDOf(ctx, tx, char)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	if err := lockGuilds(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	tm, err := d.Store.MemberIn(ctx, tx, target, gid)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if tm == nil {
		return fail(errTargetInvalid())
	}
	if !roleChangeAllowed(m.Role, tm.Role, newRole) {
		return fail(errPermissionDenied)
	}
	if newRole == RoleViceLeader && tm.Role != RoleViceLeader {
		p, err := d.Store.Progression(ctx, tx, gid)
		if err != nil {
			return idempotency.Outcome{}, err
		}
		n, err := d.Store.roleCount(ctx, tx, gid, RoleViceLeader)
		if err != nil {
			return idempotency.Outcome{}, err
		}
		if n >= ViceCapacity(p.Level) {
			return fail(errViceCapacityFull)
		}
	}
	if err := d.Store.SetRole(ctx, tx, gid, target, newRole); err != nil {
		return idempotency.Outcome{}, err
	}
	action := "ROLE_CHANGE"
	if m.Role == RoleLeader && (newRole == RoleViceLeader || tm.Role == RoleViceLeader) {
		action = "VICE_ASSIGN"
		if tm.Role == RoleViceLeader {
			action = "VICE_REMOVE"
		}
	}
	if err := d.Store.InsertAudit(ctx, tx, id.NewV4(), char, action, gid, opID,
		fmt.Sprintf(`{"guild_id":%q,"target":%q,"role":%q}`, gid.String(), target.String(), newRole), now); err != nil {
		return idempotency.Outcome{}, err
	}
	return d.success(rec, ctx, tx, opID, gid, nil)
}

// roleChangeAllowed mirrors guild.md § Promotion/Demotion:
// LEADER: MEMBER<->OFFICER, MEMBER/OFFICER->VICE_LEADER, VICE_LEADER->OFFICER;
// VICE_LEADER: MEMBER<->OFFICER only (ADR-0060 demote OFFICER included).
func roleChangeAllowed(actor, current, next string) bool {
	switch actor {
	case RoleLeader:
		switch {
		case current == RoleMember && (next == RoleOfficer || next == RoleViceLeader):
			return true
		case current == RoleOfficer && (next == RoleMember || next == RoleViceLeader):
			return true
		case current == RoleViceLeader && next == RoleOfficer:
			return true
		}
	case RoleViceLeader:
		return (current == RoleMember && next == RoleOfficer) ||
			(current == RoleOfficer && next == RoleMember)
	}
	return false
}

func (d Deps) leaderTransfer(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, LeaderTransferFamily); err != nil {
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
	req := cmd.GetC2SGuildLeaderTransfer()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	target, err := targetID(req.GetTargetCharacterId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	now := admittedAt(rec)
	if err := lockCharacters(ctx, tx, char, target); err != nil {
		return idempotency.Outcome{}, err
	}
	gid, m, err := d.guildIDOf(ctx, tx, char)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	if m.Role != RoleLeader {
		return fail(errPermissionDenied)
	}
	if err := lockGuilds(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	tm, err := d.Store.MemberIn(ctx, tx, target, gid)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if tm == nil || tm.Role == RoleLeader {
		return fail(errTargetInvalid())
	}
	// Demote first: the partial one-LEADER index rejects two leaders
	// even inside a transaction.
	// The forced old-LEADER -> VICE_LEADER demotion bypasses the
	// level-derived vice cap (the cap gates explicit promotions only).
	if err := d.Store.SetRole(ctx, tx, gid, char, RoleViceLeader); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := d.Store.SetRole(ctx, tx, gid, target, RoleLeader); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := d.Store.SetLeader(ctx, tx, gid, target); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := d.Store.InsertAudit(ctx, tx, id.NewV4(), char, "LEADERSHIP_TRANSFER", gid, opID,
		fmt.Sprintf(`{"guild_id":%q,"to":%q}`, gid.String(), target.String()), now); err != nil {
		return idempotency.Outcome{}, err
	}
	return d.success(rec, ctx, tx, opID, gid, nil)
}

// ---------------------------------------------------------------------------
// 638 C2S_GUILD_DISBAND — LEADER only; preconditions + flow.

func (d Deps) disband(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, DisbandFamily); err != nil {
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
	if cmd.GetC2SGuildDisband() == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	now := admittedAt(rec)
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", char)); err != nil {
		return idempotency.Outcome{}, err
	}
	gid, m, err := d.guildIDOf(ctx, tx, char)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	if m.Role != RoleLeader {
		return fail(errPermissionDenied)
	}
	if err := lockGuilds(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := lockProgression(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := d.checkDisband(ctx, tx, gid); err != nil {
		return fail(errDisbandBlocked)
	}
	return d.commitDisband(ctx, tx, rec, opID, gid, char, now)
}

// checkDisband evaluates the three disband preconditions
// (guild.md § Disband).
func (d Deps) checkDisband(ctx context.Context, tx pgx.Tx, guildID id.UUID) error {
	empty, err := d.Store.StorageEmpty(ctx, tx, guildID)
	if err != nil {
		return err
	}
	if !empty {
		return errDisbandBlocked
	}
	open, err := d.Store.ApprovedClaimsOpen(ctx, tx, guildID)
	if err != nil {
		return err
	}
	if open {
		return errDisbandBlocked
	}
	if d.Guard != nil {
		blocked, err := d.Guard.Blocked(ctx, tx, guildID)
		if err != nil {
			return err
		}
		if blocked {
			return errDisbandBlocked
		}
	}
	return nil
}

// commitDisband performs ACTIVE -> DISBANDED atomically (idempotent:
// re-request on a DISBANDED guild succeeds without further change).
func (d Deps) commitDisband(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord, opID, gid, char id.UUID,
	now time.Time) (idempotency.Outcome, error) {
	g, err := d.Store.Guild(ctx, tx, gid)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if g.State == "DISBANDED" {
		return d.success(rec, ctx, tx, opID, gid, nil)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE guild_invites SET state = 'EXPIRED', resolved_at = $2
		 WHERE guild_id = $1 AND state = 'PENDING'`, gid, now); err != nil {
		return idempotency.Outcome{}, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE guild_applications SET state = 'EXPIRED', resolved_at = $2
		 WHERE guild_id = $1 AND state = 'PENDING'`, gid, now); err != nil {
		return idempotency.Outcome{}, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE guild_membership_history SET left_at = GREATEST($2, joined_at)
		 WHERE guild_id = $1 AND left_at IS NULL`, gid, now); err != nil {
		return idempotency.Outcome{}, err
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM guild_memberships WHERE guild_id = $1`, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	// Active progression projections die; history rows survive.
	if _, err := tx.Exec(ctx,
		`DELETE FROM guild_progression WHERE guild_id = $1`, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM guild_cosmetic_entitlements WHERE guild_id = $1`, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM guild_cosmetic_selections WHERE guild_id = $1`, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE guilds SET state = 'DISBANDED', leader_character_id = NULL, disbanded_at = $2
		 WHERE guild_id = $1`, gid, now); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := d.Store.InsertAudit(ctx, tx, id.NewV4(), char, "DISBAND", gid, opID,
		fmt.Sprintf(`{"guild_id":%q}`, gid.String()), now); err != nil {
		return idempotency.Outcome{}, err
	}
	return d.success(rec, ctx, tx, opID, gid, nil)
}

// ---------------------------------------------------------------------------
// 643 C2S_GUILD_LEADERSHIP_CLAIM — 30-day inactivity takeover.

func (d Deps) leadershipClaim(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, LeadershipClaimFamily); err != nil {
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
	if cmd.GetC2SGuildLeadershipClaim() == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	now := admittedAt(rec)
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", char)); err != nil {
		return idempotency.Outcome{}, err
	}
	gid, m, err := d.guildIDOf(ctx, tx, char)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	if err := lockGuilds(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	g, err := d.Store.Guild(ctx, tx, gid)
	if err != nil {
		return fail(err)
	}
	if g.State != "ACTIVE" {
		return fail(errStateConflict)
	}
	// Leader inactivity: no live session AND attach-age >= 30 days
	// (creation time only when the leader never attached).
	last, active, err := d.Store.LastAttachedAt(ctx, tx, g.Leader)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if active {
		return fail(errLeaderOfflineWindow)
	}
	var leaderSince time.Time
	if last != nil {
		leaderSince = *last
	} else {
		leaderSince, err = d.Store.CharacterCreatedAt(ctx, tx, g.Leader)
		if err != nil {
			return idempotency.Outcome{}, err
		}
	}
	if now.Sub(leaderSince) < time.Duration(LeaderInactivityDays)*24*time.Hour {
		return fail(errLeaderOfflineWindow)
	}
	// Claimant: the caller must be THE eligible member — highest role
	// (ties -> longest tenure), tenure >= 14d, real attach <= 7d.
	members, err := d.Store.Members(ctx, tx, gid)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	var best *MemberRow
	for i := range members {
		c := members[i]
		if c.CharacterID == g.Leader {
			continue
		}
		if best == nil ||
			roleRank(c.Role) > roleRank(best.Role) ||
			(roleRank(c.Role) == roleRank(best.Role) && c.JoinedAt.Before(best.JoinedAt)) {
			cp := c
			best = &cp
		}
	}
	if best == nil || best.CharacterID != char {
		return fail(errPermissionDenied)
	}
	if now.Sub(m.JoinedAt) < time.Duration(ClaimantTenureDays)*24*time.Hour {
		return fail(errLeaderOfflineWindow)
	}
	attached, err := d.Store.AttachedWithin(ctx, tx, char,
		now.Add(-time.Duration(ClaimantAttachDays)*24*time.Hour), now)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if !attached {
		return fail(errLeaderOfflineWindow)
	}
	lastClaim, err := d.Store.LastLeadershipClaimAt(ctx, tx, gid)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if lastClaim != nil && now.Sub(*lastClaim) < time.Duration(ClaimCooldownDays)*24*time.Hour {
		return fail(errLeaderOfflineWindow)
	}
	if err := d.Store.SetRole(ctx, tx, gid, char, RoleLeader); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := d.Store.SetRole(ctx, tx, gid, g.Leader, RoleMember); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := d.Store.SetLeader(ctx, tx, gid, char); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := d.Store.InsertAudit(ctx, tx, id.NewV4(), char, "GUILD_LEADERSHIP_CLAIM", gid, opID,
		fmt.Sprintf(`{"guild_id":%q,"old_leader":%q}`, gid.String(), g.Leader.String()), now); err != nil {
		return idempotency.Outcome{}, err
	}
	return d.success(rec, ctx, tx, opID, gid, nil)
}

// ---------------------------------------------------------------------------
// 642/650 settings.

func (d Deps) motdSet(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, MotdSetFamily); err != nil {
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
	req := cmd.GetC2SGuildMotdSet()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	motd, ok := canonicalMotd(req.GetMotd())
	if !ok {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	now := admittedAt(rec)
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", char)); err != nil {
		return idempotency.Outcome{}, err
	}
	gid, m, err := d.guildIDOf(ctx, tx, char)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	if m.Role != RoleLeader {
		return fail(errPermissionDenied)
	}
	if err := lockGuilds(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE guilds SET motd = $2 WHERE guild_id = $1`, gid, motd); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := d.Store.InsertAudit(ctx, tx, id.NewV4(), char, "SETTINGS_CHANGE", gid, opID,
		fmt.Sprintf(`{"guild_id":%q,"field":"motd"}`, gid.String()), now); err != nil {
		return idempotency.Outcome{}, err
	}
	return d.success(rec, ctx, tx, opID, gid, nil)
}

// canonicalMotd validates user text within the VARCHAR(1024) column —
// trim+NFC, no control/separator runes, <=256 graphemes and <=1024
// bytes (text.md rules applied to the column budget).
func canonicalMotd(raw string) (string, bool) {
	if !utf8.ValidString(raw) {
		return "", false
	}
	text := norm.NFC.String(strings.TrimSpace(raw))
	if len(text) > 1024 {
		return "", false
	}
	clusters := 0
	it := graphemes.FromString(text)
	for it.Next() {
		clusters++
		if clusters > 256 {
			return "", false
		}
	}
	for _, r := range text {
		if unicode.IsControl(r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			return "", false
		}
	}
	return text, true
}

func (d Deps) settingsSet(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if err := family(rec, SettingsSetFamily); err != nil {
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
	req := cmd.GetC2SGuildSettingsSet()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	var mode string
	switch req.GetRecruitmentMode() {
	case protocolv1.GuildRecruitmentMode_GUILD_RECRUITMENT_MODE_CLOSED:
		mode = "CLOSED"
	case protocolv1.GuildRecruitmentMode_GUILD_RECRUITMENT_MODE_APPLICATIONS:
		mode = "APPLICATIONS"
	default:
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	now := admittedAt(rec)
	if err := lockorder.Acquire(ctx, tx, lockorder.RowLock("characters", char)); err != nil {
		return idempotency.Outcome{}, err
	}
	gid, m, err := d.guildIDOf(ctx, tx, char)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fail := func(e error) (idempotency.Outcome, error) { return d.verdict(rec, opID, gid, e) }
	// Only LEADER sets recruitment mode (ADR-0060).
	if m.Role != RoleLeader {
		return fail(errPermissionDenied)
	}
	if err := lockGuilds(ctx, tx, gid); err != nil {
		return idempotency.Outcome{}, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE guilds SET recruitment_mode = $2 WHERE guild_id = $1`, gid, mode); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := d.Store.InsertAudit(ctx, tx, id.NewV4(), char, "SETTINGS_CHANGE", gid, opID,
		fmt.Sprintf(`{"guild_id":%q,"field":"recruitment_mode","value":%q}`, gid.String(), mode), now); err != nil {
		return idempotency.Outcome{}, err
	}
	return d.success(rec, ctx, tx, opID, gid, nil)
}
