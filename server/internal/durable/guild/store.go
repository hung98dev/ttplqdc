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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/lockorder"
)

// Canonical constants — guild.md § Identity / Membership / Invitations
// / Applications, guild_progression.md § Capacity / Guild Level.
const (
	MaxNameGraphemes = 24
	maxNameBytes     = 96
	maxNameKeyRunes  = 256

	MinJoinLevel     = 10
	MinCreateLevel   = 20
	CreateCostCommon = 10_000

	InviteTTL              = 10 * time.Minute
	ApplicationTTL         = 7 * 24 * time.Hour
	MaxPendingApplications = 5

	MaxLevel            = 30
	GuildActivityDayCap = 1

	LeaderInactivityDays = 30
	ClaimantTenureDays   = 14
	ClaimantAttachDays   = 7
	ClaimCooldownDays    = 30
)

// Role ids are the content ids persisted in guild_memberships.role.
const (
	RoleLeader     = "LEADER"
	RoleViceLeader = "VICE_LEADER"
	RoleOfficer    = "OFFICER"
	RoleMember     = "MEMBER"
)

// Wire content ids for 626 new_role (messages.md).
const (
	RoleIDLeader     = "guild.role.leader"
	RoleIDViceLeader = "guild.role.vice_leader"
	RoleIDOfficer    = "guild.role.officer"
	RoleIDMember     = "guild.role.member"
)

var roleIDs = map[string]string{
	RoleIDLeader:     RoleLeader,
	RoleIDViceLeader: RoleViceLeader,
	RoleIDOfficer:    RoleOfficer,
	RoleIDMember:     RoleMember,
}

// roleRank orders authority (guild.md § Roles).
func roleRank(role string) int {
	switch role {
	case RoleLeader:
		return 4
	case RoleViceLeader:
		return 3
	case RoleOfficer:
		return 2
	case RoleMember:
		return 1
	default:
		return 0
	}
}

// RequiredGuildEXP mirrors required_total_guild_exp(L) =
// 100*(L-1)^2 + 200*(L-1) (guild_progression.md § EXP Threshold).
func RequiredGuildEXP(level int) int64 {
	k := int64(level - 1)
	return 100*k*k + 200*k
}

// LevelForEXP maps cumulative EXP onto a level (one grant may cross
// levels; max 30).
func LevelForEXP(exp int64) int {
	lv := 1
	for lv < MaxLevel && exp >= RequiredGuildEXP(lv+1) {
		lv++
	}
	return lv
}

// MemberCapacity derives the member cap from guild level
// (guild_progression.md § Capacity).
func MemberCapacity(level int) int {
	switch {
	case level >= 30:
		return 60
	case level >= 25:
		return 55
	case level >= 20:
		return 50
	case level >= 15:
		return 45
	case level >= 10:
		return 40
	case level >= 5:
		return 35
	default:
		return 30
	}
}

// ViceCapacity derives the VICE_LEADER cap (guild_progression.md).
func ViceCapacity(level int) int {
	switch {
	case level >= 30:
		return 3
	case level >= 15:
		return 2
	default:
		return 1
	}
}

// Element ids are the journal element ordinals 1..5 in
// KIM/MOC/THUY/HOA/THO order (protobuf_conventions §7).
const (
	ElementKim  uint32 = 1
	ElementMoc  uint32 = 2
	ElementThuy uint32 = 3
	ElementHoa  uint32 = 4
	ElementTho  uint32 = 5
)

// elementOrder is the SERVER_ROTATION pointer cycle.
var elementOrder = [5]uint32{ElementKim, ElementMoc, ElementThuy, ElementHoa, ElementTho}

// elementName renders the rotation_pointer column value.
func elementName(e uint32) string {
	switch e {
	case ElementKim:
		return "KIM"
	case ElementMoc:
		return "MOC"
	case ElementThuy:
		return "THUY"
	case ElementHoa:
		return "HOA"
	default:
		return "THO"
	}
}

var caseFold = cases.Fold()

// NormalizeGuildName applies the guild row of text.md § Name Limits:
// trim + NFC, malformed/empty/control-rejected, 1..24 graphemes,
// <=96 UTF-8 bytes; name_key = NFC(case-fold(display)) capped at 256
// runes with the reserved anonymized_ prefix closed.
func NormalizeGuildName(raw string) (display, nameKey string, err error) {
	bad := func(why string) (string, string, error) {
		return "", "", fmt.Errorf("%w: %s", ErrGuildNameInvalid, why)
	}
	if !utf8.ValidString(raw) {
		return bad("malformed UTF-8")
	}
	display = norm.NFC.String(strings.TrimSpace(raw))
	if display == "" {
		return bad("empty after trim")
	}
	for _, r := range display {
		if unicode.IsControl(r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			return bad("control or separator character")
		}
	}
	clusters := 0
	it := graphemes.FromString(display)
	for it.Next() {
		clusters++
		if clusters > MaxNameGraphemes {
			break
		}
	}
	if clusters == 0 || clusters > MaxNameGraphemes {
		return bad("grapheme count outside 1..24")
	}
	if len(display) > maxNameBytes {
		return bad("over 96 UTF-8 bytes")
	}
	nameKey = norm.NFC.String(caseFold.String(display))
	if strings.HasPrefix(nameKey, "anonymized_") {
		return bad("reserved name_key prefix")
	}
	if utf8.RuneCountInString(nameKey) > maxNameKeyRunes {
		return bad("name_key over 256 characters")
	}
	return display, nameKey, nil
}

// DBTX is the narrow SQL surface both Store and tests use.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Store wraps the pool; every method takes the caller's transaction so
// executors own lock acquisition (lockorder) and atomicity.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore binds the package to one pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Begin is a convenience for tests and read-only call sites.
func (s *Store) Begin(ctx context.Context) (pgx.Tx, error) {
	return s.pool.Begin(ctx)
}

// GuildRow mirrors guilds.
type GuildRow struct {
	ID               id.UUID
	Name             string
	NameKey          string
	State            string
	RecruitmentMode  string
	Leader           id.UUID // nil-able encoded as IsNil
	HasLeader        bool
	Motd             string
	Revision         uint64
	StorageRevision  uint64
	CosmeticRevision uint64
	CreatedAt        time.Time
	DisbandedAt      *time.Time
}

// MemberRow mirrors guild_memberships.
type MemberRow struct {
	CharacterID  id.UUID
	GuildID      id.UUID
	Role         string
	JoinedAt     time.Time
	MembershipID id.UUID
}

// ContributionRow mirrors guild_member_contributions.
type ContributionRow struct {
	GuildID     id.UUID
	CharacterID id.UUID
	Lifetime    uint64
	CycleID     string
	Cycle       uint64
}

// InviteRow mirrors guild_invites.
type InviteRow struct {
	ID        id.UUID
	GuildID   id.UUID
	Inviter   id.UUID
	Target    id.UUID
	State     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// ApplicationRow mirrors guild_applications.
type ApplicationRow struct {
	ID        id.UUID
	GuildID   id.UUID
	Applicant id.UUID
	State     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// ProgressionRow mirrors guild_progression.
type ProgressionRow struct {
	GuildID           id.UUID
	EXP               uint64
	Level             int
	Streak            int
	ActiveBlessingID  string
	BlessingExpiresAt *time.Time
	Revision          uint64
}

// CycleRow mirrors guild_ritual_cycles.
type CycleRow struct {
	GuildID         id.UUID
	CycleID         string
	MEffective      int
	Required        int
	Points          [5]int // KIM/MOC/THUY/HOA/THO
	Pointer         string // element name KIM/MOC/THUY/HOA/THO
	CompletedAt     *time.Time
	Candidates      []string
	VoteClosesAt    *time.Time
	FinalBlessingID string
}

func (r *GuildRow) from(row pgx.Row) error {
	var leader []byte
	var disbanded *time.Time
	err := row.Scan(&r.ID, &r.Name, &r.NameKey, &r.State, &r.RecruitmentMode,
		&leader, &r.Motd, &r.Revision, &r.StorageRevision, &r.CosmeticRevision,
		&r.CreatedAt, &disbanded)
	if err != nil {
		return err
	}
	if leader != nil {
		copy(r.Leader[:], leader)
		r.HasLeader = true
	}
	r.DisbandedAt = disbanded
	return nil
}

const guildCols = `guild_id, name, name_key, state, recruitment_mode,
	leader_character_id, motd, guild_revision, guild_storage_revision,
	guild_cosmetic_revision, created_at, disbanded_at`

// Guild loads one row by id.
func (s *Store) Guild(ctx context.Context, db DBTX, guildID id.UUID) (GuildRow, error) {
	var r GuildRow
	err := r.from(db.QueryRow(ctx,
		`SELECT `+guildCols+` FROM guilds WHERE guild_id = $1`, guildID))
	if errors.Is(err, pgx.ErrNoRows) {
		return r, errGuildNotFound
	}
	return r, err
}

// CharacterLevel returns the character's level and account id.
func (s *Store) CharacterLevel(ctx context.Context, db DBTX, char id.UUID) (level int, account id.UUID, err error) {
	err = db.QueryRow(ctx,
		`SELECT level, account_id FROM characters WHERE character_id = $1`, char).
		Scan(&level, &account)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, account, errNotMember
	}
	return level, account, err
}

// Member loads the caller's current membership (nil when guildless or
// not in that guild).
func (s *Store) Member(ctx context.Context, db DBTX, char id.UUID) (*MemberRow, error) {
	var m MemberRow
	err := db.QueryRow(ctx,
		`SELECT character_id, guild_id, role, joined_at, membership_id
		 FROM guild_memberships WHERE character_id = $1`, char).
		Scan(&m.CharacterID, &m.GuildID, &m.Role, &m.JoinedAt, &m.MembershipID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// MemberIn loads one character's membership inside a specific guild.
func (s *Store) MemberIn(ctx context.Context, db DBTX, char, guildID id.UUID) (*MemberRow, error) {
	var m MemberRow
	err := db.QueryRow(ctx,
		`SELECT character_id, guild_id, role, joined_at, membership_id
		 FROM guild_memberships WHERE character_id = $1 AND guild_id = $2`,
		char, guildID).
		Scan(&m.CharacterID, &m.GuildID, &m.Role, &m.JoinedAt, &m.MembershipID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Members lists the roster oldest-join first.
func (s *Store) Members(ctx context.Context, db DBTX, guildID id.UUID) ([]MemberRow, error) {
	rows, err := db.Query(ctx,
		`SELECT character_id, guild_id, role, joined_at, membership_id
		 FROM guild_memberships WHERE guild_id = $1 ORDER BY joined_at, character_id`, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemberRow
	for rows.Next() {
		var m MemberRow
		if err := rows.Scan(&m.CharacterID, &m.GuildID, &m.Role, &m.JoinedAt, &m.MembershipID); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// MemberCount counts current rows; roleCount counts one role.
func (s *Store) MemberCount(ctx context.Context, db DBTX, guildID id.UUID) (int, error) {
	var n int
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM guild_memberships WHERE guild_id = $1`, guildID).Scan(&n)
	return n, err
}

func (s *Store) roleCount(ctx context.Context, db DBTX, guildID id.UUID, role string) (int, error) {
	var n int
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM guild_memberships WHERE guild_id = $1 AND role = $2`,
		guildID, role).Scan(&n)
	return n, err
}

// AddMember inserts the current membership plus its history interval
// atomically (data_model.md: join writes both).
func (s *Store) AddMember(ctx context.Context, tx pgx.Tx, guildID, char id.UUID,
	role string, membershipID id.UUID, now time.Time) error {
	if _, err := tx.Exec(ctx,
		`INSERT INTO guild_membership_history (membership_id, guild_id, character_id, joined_at)
		 VALUES ($1,$2,$3,$4)`, membershipID, guildID, char, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO guild_memberships (character_id, guild_id, role, joined_at, membership_id)
		 VALUES ($1,$2,$3,$4,$5)`, char, guildID, role, now, membershipID)
	return err
}

// RemoveMember ends the history interval and deletes the current row.
// Contributions survive (ADR-0065).
func (s *Store) RemoveMember(ctx context.Context, tx pgx.Tx, guildID, char id.UUID,
	now time.Time) error {
	if _, err := tx.Exec(ctx,
		`UPDATE guild_membership_history SET left_at = GREATEST($3, joined_at)
		 WHERE guild_id = $1 AND character_id = $2 AND left_at IS NULL`,
		guildID, char, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx,
		`DELETE FROM guild_memberships WHERE guild_id = $1 AND character_id = $2`,
		guildID, char)
	return err
}

// SetRole updates one member's role.
func (s *Store) SetRole(ctx context.Context, tx pgx.Tx, guildID, char id.UUID, role string) error {
	_, err := tx.Exec(ctx,
		`UPDATE guild_memberships SET role = $3 WHERE guild_id = $1 AND character_id = $2`,
		guildID, char, role)
	return err
}

// SetLeader repoints guilds.leader_character_id.
func (s *Store) SetLeader(ctx context.Context, tx pgx.Tx, guildID, char id.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE guilds SET leader_character_id = $2 WHERE guild_id = $1`, guildID, char)
	return err
}

// BumpRevision increments guild_revision and returns the new value.
func (s *Store) BumpRevision(ctx context.Context, tx pgx.Tx, guildID id.UUID) (uint64, error) {
	var v uint64
	err := tx.QueryRow(ctx,
		`UPDATE guilds SET guild_revision = guild_revision + 1 WHERE guild_id = $1
		 RETURNING guild_revision`, guildID).Scan(&v)
	return v, err
}

// Revision reads guild_revision.
func (s *Store) Revision(ctx context.Context, db DBTX, guildID id.UUID) (uint64, error) {
	var v uint64
	err := db.QueryRow(ctx,
		`SELECT guild_revision FROM guilds WHERE guild_id = $1`, guildID).Scan(&v)
	return v, err
}

// PendingInvite loads the live pending invite for (guild,target).
func (s *Store) PendingInvite(ctx context.Context, db DBTX, guildID, target id.UUID, now time.Time) (*InviteRow, error) {
	var r InviteRow
	err := db.QueryRow(ctx,
		`SELECT invite_id, guild_id, inviter_character_id, target_character_id, state, created_at, expires_at
		 FROM guild_invites WHERE guild_id = $1 AND target_character_id = $2 AND state = 'PENDING'`,
		guildID, target).
		Scan(&r.ID, &r.GuildID, &r.Inviter, &r.Target, &r.State, &r.CreatedAt, &r.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !r.ExpiresAt.After(now) {
		return nil, nil
	}
	return &r, nil
}

// InsertInvite writes a PENDING invite.
func (s *Store) InsertInvite(ctx context.Context, tx pgx.Tx, inviteID, guildID, inviter, target id.UUID, now time.Time) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO guild_invites (invite_id, guild_id, inviter_character_id, target_character_id, state, created_at, expires_at)
		 VALUES ($1,$2,$3,$4,'PENDING',$5::timestamptz,$5::timestamptz + INTERVAL '10 minutes')`,
		inviteID, guildID, inviter, target, now)
	return err
}

// ResolveInvite moves a PENDING invite to its terminal state.
func (s *Store) ResolveInvite(ctx context.Context, tx pgx.Tx, inviteID id.UUID, state string, now time.Time) error {
	tag, err := tx.Exec(ctx,
		`UPDATE guild_invites SET state = $2, resolved_at = $3
		 WHERE invite_id = $1 AND state = 'PENDING'`, inviteID, state, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errAlreadyResolved
	}
	return nil
}

// PendingApplication loads the live pending application for
// (guild,applicant).
func (s *Store) PendingApplication(ctx context.Context, db DBTX, guildID, applicant id.UUID, now time.Time) (*ApplicationRow, error) {
	var r ApplicationRow
	err := db.QueryRow(ctx,
		`SELECT application_id, guild_id, applicant_character_id, state, created_at, expires_at
		 FROM guild_applications WHERE guild_id = $1 AND applicant_character_id = $2 AND state = 'PENDING'`,
		guildID, applicant).
		Scan(&r.ID, &r.GuildID, &r.Applicant, &r.State, &r.CreatedAt, &r.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !r.ExpiresAt.After(now) {
		return nil, nil
	}
	return &r, nil
}

// PendingApplicationCount counts one character's live pending rows
// across guilds (5-cap check, guild.md § Applications).
func (s *Store) PendingApplicationCount(ctx context.Context, db DBTX, applicant id.UUID, now time.Time) (int, error) {
	var n int
	err := db.QueryRow(ctx,
		`SELECT COUNT(*) FROM guild_applications
		 WHERE applicant_character_id = $1 AND state = 'PENDING' AND expires_at > $2`,
		applicant, now).Scan(&n)
	return n, err
}

// InsertApplication writes a PENDING application.
func (s *Store) InsertApplication(ctx context.Context, tx pgx.Tx, appID, guildID, applicant id.UUID, now time.Time) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO guild_applications (application_id, guild_id, applicant_character_id, state, created_at, expires_at)
		 VALUES ($1,$2,$3,'PENDING',$4::timestamptz,$4::timestamptz + INTERVAL '7 days')`,
		appID, guildID, applicant, now)
	return err
}

// ResolveApplication moves a PENDING application to terminal.
func (s *Store) ResolveApplication(ctx context.Context, tx pgx.Tx, appID id.UUID, state string, resolver id.UUID, now time.Time) error {
	tag, err := tx.Exec(ctx,
		`UPDATE guild_applications SET state = $2, resolved_at = $3, resolver_character_id = $4
		 WHERE application_id = $1 AND state = 'PENDING'`, appID, state, now, resolver)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errAlreadyResolved
	}
	return nil
}

// Progression loads the progression row.
func (s *Store) Progression(ctx context.Context, db DBTX, guildID id.UUID) (ProgressionRow, error) {
	var p ProgressionRow
	err := db.QueryRow(ctx,
		`SELECT guild_id, guild_exp, guild_level, ritual_streak,
		        COALESCE(active_blessing_id, ''), blessing_expires_at, revision
		 FROM guild_progression WHERE guild_id = $1`, guildID).
		Scan(&p.GuildID, &p.EXP, &p.Level, &p.Streak,
			&p.ActiveBlessingID, &p.BlessingExpiresAt, &p.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, errGuildNotFound
	}
	return p, err
}

// AddEXP applies one cumulative-EXP grant and recomputes level.
func (s *Store) AddEXP(ctx context.Context, tx pgx.Tx, guildID id.UUID, amount uint64) (ProgressionRow, error) {
	var p ProgressionRow
	err := tx.QueryRow(ctx,
		`UPDATE guild_progression SET guild_exp = guild_exp + $2,
		        guild_level = LEAST(30,
		          (SELECT COALESCE(MAX(l), 1) FROM generate_series(1,30) l
		           WHERE guild_progression.guild_exp + $2 >= 100 * (l-1) * (l-1) + 200 * (l-1))),
		        revision = revision + 1
		 WHERE guild_id = $1
		 RETURNING guild_id, guild_exp, guild_level, ritual_streak,
		           COALESCE(active_blessing_id, ''), blessing_expires_at, revision`,
		guildID, int64(amount)).
		Scan(&p.GuildID, &p.EXP, &p.Level, &p.Streak,
			&p.ActiveBlessingID, &p.BlessingExpiresAt, &p.Revision)
	return p, err
}

// GrantContribution adds to a member's lifetime + cycle totals
// (survives leave; rejoin resumes lifetime but the current cycle row
// carries the current cycle id).
func (s *Store) GrantContribution(ctx context.Context, tx pgx.Tx, guildID, char id.UUID,
	cycleID string, amount uint64) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO guild_member_contributions (guild_id, character_id, lifetime_contribution, cycle_id, cycle_contribution)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (guild_id, character_id) DO UPDATE
		 SET lifetime_contribution = guild_member_contributions.lifetime_contribution + $3,
		     cycle_id = EXCLUDED.cycle_id,
		     cycle_contribution = CASE
		       WHEN guild_member_contributions.cycle_id = EXCLUDED.cycle_id
		       THEN guild_member_contributions.cycle_contribution + $5
		       ELSE $5 END`,
		guildID, char, int64(amount), cycleID, int64(amount))
	return err
}

// Contribution reads one member's totals.
func (s *Store) Contribution(ctx context.Context, db DBTX, guildID, char id.UUID) (ContributionRow, error) {
	var c ContributionRow
	err := db.QueryRow(ctx,
		`SELECT guild_id, character_id, lifetime_contribution, cycle_id, cycle_contribution
		 FROM guild_member_contributions WHERE guild_id = $1 AND character_id = $2`,
		guildID, char).
		Scan(&c.GuildID, &c.CharacterID, &c.Lifetime, &c.CycleID, &c.Cycle)
	if errors.Is(err, pgx.ErrNoRows) {
		return ContributionRow{GuildID: guildID, CharacterID: char}, nil
	}
	return c, err
}

// CycleID renders the Monday-UTC cycle key (YYYY-MM-DD) for ts.
func CycleID(ts time.Time) string {
	u := ts.UTC()
	monday := u.AddDate(0, 0, -((int(u.Weekday()) + 6) % 7))
	return monday.Format("2006-01-02")
}

// CycleStart returns the Monday 00:00 UTC instant of a cycle id.
func CycleStart(cycleID string) (time.Time, error) {
	return time.Parse("2006-01-02", cycleID)
}

// Cycle loads the open (or historical) cycle row.
func (s *Store) Cycle(ctx context.Context, db DBTX, guildID id.UUID, cycleID string) (*CycleRow, error) {
	var c CycleRow
	err := db.QueryRow(ctx,
		`SELECT guild_id, cycle_id, m_effective, required_points_per_element,
		        points_kim, points_moc, points_thuy, points_hoa, points_tho,
		        rotation_pointer, completed_at, candidate_blessing_ids,
		        vote_closes_at, COALESCE(finalized_blessing_id, '') AS finalized_blessing_id
		 FROM guild_ritual_cycles WHERE guild_id = $1 AND cycle_id = $2`,
		guildID, cycleID).
		Scan(&c.GuildID, &c.CycleID, &c.MEffective, &c.Required,
			&c.Points[0], &c.Points[1], &c.Points[2], &c.Points[3], &c.Points[4],
			&c.Pointer, &c.CompletedAt, &c.Candidates, &c.VoteClosesAt, &c.FinalBlessingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CycleMembers lists the immutable snapshot roster.
func (s *Store) CycleMembers(ctx context.Context, db DBTX, guildID id.UUID, cycleID string) ([]struct {
	CharacterID id.UUID
	AccountID   id.UUID
	JoinedAt    time.Time
}, error) {
	rows, err := db.Query(ctx,
		`SELECT character_id, account_id, membership_joined_at
		 FROM guild_ritual_cycle_members WHERE guild_id = $1 AND cycle_id = $2`,
		guildID, cycleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		CharacterID id.UUID
		AccountID   id.UUID
		JoinedAt    time.Time
	}
	for rows.Next() {
		var m struct {
			CharacterID id.UUID
			AccountID   id.UUID
			JoinedAt    time.Time
		}
		var account *id.UUID
		if err := rows.Scan(&m.CharacterID, &account, &m.JoinedAt); err != nil {
			return nil, err
		}
		if account != nil {
			m.AccountID = *account
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ActiveMembershipsAt returns the membership intervals open at cutoff
// (ritual snapshot input): current members whose interval was live at
// the cutoff instant.
func (s *Store) ActiveMembershipsAt(ctx context.Context, db DBTX, guildID id.UUID, cutoff time.Time) ([]MemberRow, error) {
	rows, err := db.Query(ctx,
		`SELECT character_id, guild_id, membership_id, joined_at
		 FROM guild_membership_history
		 WHERE guild_id = $1 AND joined_at <= $2 AND (left_at IS NULL OR left_at > $2)
		 ORDER BY joined_at, character_id`, guildID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MemberRow
	for rows.Next() {
		var m MemberRow
		if err := rows.Scan(&m.CharacterID, &m.GuildID, &m.MembershipID, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AttachedWithin reports a real attach event in (from, to] —
// character_attach_events is the schema name (spec F-4).
func (s *Store) AttachedWithin(ctx context.Context, db DBTX, char id.UUID, from, to time.Time) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM character_attach_events
		 WHERE character_id = $1 AND attached_at > $2 AND attached_at <= $3)`,
		char, from, to).Scan(&ok)
	return ok, err
}

// LastAttachedAt reads character_activity.last_attached_at.
func (s *Store) LastAttachedAt(ctx context.Context, db DBTX, char id.UUID) (*time.Time, bool, error) {
	var last *time.Time
	var active bool
	err := db.QueryRow(ctx,
		`SELECT last_attached_at, session_active FROM character_activity WHERE character_id = $1`,
		char).Scan(&last, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	return last, active, err
}

// CharacterCreatedAt reads characters.created_at (leader-inactivity
// fallback for never-attached leaders, guild.md § Leader Inactivity).
func (s *Store) CharacterCreatedAt(ctx context.Context, db DBTX, char id.UUID) (time.Time, error) {
	var t time.Time
	err := db.QueryRow(ctx,
		`SELECT created_at FROM characters WHERE character_id = $1`, char).Scan(&t)
	return t, err
}

// LastLeadershipClaimAt reads the most recent audited claim —
// audit_events is the canonical claim ledger (one per guild per 30d).
func (s *Store) LastLeadershipClaimAt(ctx context.Context, db DBTX, guildID id.UUID) (*time.Time, error) {
	var t *time.Time
	err := db.QueryRow(ctx,
		`SELECT MAX(occurred_at) FROM audit_events
		 WHERE action = 'GUILD_LEADERSHIP_CLAIM' AND payload->>'guild_id' = $1`,
		guildID.String()).Scan(&t)
	return t, err
}

// InsertAudit writes one audit_events row for the guild audit list
// (guild.md § Revision / Audit).
func (s *Store) InsertAudit(ctx context.Context, tx pgx.Tx, auditID, actor id.UUID,
	action string, guildID id.UUID, operationID id.UUID, payload string, now time.Time) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO audit_events (audit_event_id, occurred_at, actor_kind, actor_id, action, operation_id, payload)
		 VALUES ($1,$2,'PLAYER',$3,$4,$5,$6::jsonb)`,
		auditID, now, actor, action, operationID, payload)
	return err
}

// CastVote inserts one vote row; the (guild,cycle,account) PK enforces
// the one-account-one-vote rule transactionally.
func (s *Store) CastVote(ctx context.Context, tx pgx.Tx, guildID id.UUID, cycleID string,
	accountID, char id.UUID, blessingID string, now time.Time) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO guild_blessing_votes (guild_id, cycle_id, account_id, character_id, blessing_id, voted_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`, guildID, cycleID, accountID, char, blessingID, now)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return errAlreadyResolved
	}
	return err
}

// VoteCounts aggregates the draft's votes in candidate order.
func (s *Store) VoteCounts(ctx context.Context, db DBTX, guildID id.UUID, cycleID string,
	candidates []string) ([]int, error) {
	counts := make([]int, len(candidates))
	if len(candidates) == 0 {
		return counts, nil
	}
	rows, err := db.Query(ctx,
		`SELECT blessing_id, COUNT(*) FROM guild_blessing_votes
		 WHERE guild_id = $1 AND cycle_id = $2 GROUP BY blessing_id`,
		guildID, cycleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := map[string]int{}
	for rows.Next() {
		var b string
		var n int
		if err := rows.Scan(&b, &n); err != nil {
			return nil, err
		}
		byID[b] = n
	}
	for i, c := range candidates {
		counts[i] = byID[c]
	}
	return counts, rows.Err()
}

// ReceiverVote returns the account's recorded ballot.
func (s *Store) ReceiverVote(ctx context.Context, db DBTX, guildID id.UUID, cycleID string, accountID id.UUID) (string, error) {
	var b *string
	err := db.QueryRow(ctx,
		`SELECT blessing_id FROM guild_blessing_votes
		 WHERE guild_id = $1 AND cycle_id = $2 AND account_id = $3`,
		guildID, cycleID, accountID).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if b == nil {
		return "", nil
	}
	return *b, nil
}

// StorageEmpty reports whether any GUILD_STORAGE items remain
// (disband precondition).
func (s *Store) StorageEmpty(ctx context.Context, db DBTX, guildID id.UUID) (bool, error) {
	var empty bool
	err := db.QueryRow(ctx,
		`SELECT NOT EXISTS(SELECT 1 FROM item_locations
		 WHERE guild_id = $1 AND location_kind = 'GUILD_STORAGE')`, guildID).Scan(&empty)
	return empty, err
}

// ApprovedClaimsOpen reports APPROVED storage claims remaining
// (disband precondition).
func (s *Store) ApprovedClaimsOpen(ctx context.Context, db DBTX, guildID id.UUID) (bool, error) {
	var open bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM guild_storage_claims
		 WHERE guild_id = $1 AND state = 'APPROVED')`, guildID).Scan(&open)
	return open, err
}

// lockGuilds acquires canonical guild-scope row locks for the whole
// mutation surface of one guild (priority 11, listed order).
func lockGuilds(ctx context.Context, tx pgx.Tx, guildID id.UUID) error {
	return lockorder.Acquire(ctx, tx,
		lockorder.RowLock("guilds", guildID),
		lockorder.RowLock("guild_memberships", guildID),
		lockorder.RowLock("guild_membership_history", guildID),
		lockorder.RowLock("guild_member_contributions", guildID),
		lockorder.RowLock("guild_invites", guildID),
		lockorder.RowLock("guild_applications", guildID))
}

// lockProgression acquires the priority-12 progression/ritual/vote
// tier.
func lockProgression(ctx context.Context, tx pgx.Tx, guildID id.UUID) error {
	return lockorder.Acquire(ctx, tx,
		lockorder.RowLock("guild_progression", guildID),
		lockorder.RowLock("guild_ritual_cycles", guildID),
		lockorder.RowLock("guild_ritual_cycle_members", guildID),
		lockorder.RowLock("guild_blessing_votes", guildID))
}
