package guild_storage

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/guild"
	"thinhthan/internal/durable/lockorder"
)

// DBTX is the transaction/query surface the store runs on.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Store executes guild-storage reads/writes on callers' transactions.
type Store struct{ pool *pgxpool.Pool }

// New returns the store bound to the durable pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// StorageItemRow is one GUILD_STORAGE item_locations row joined with its
// instance fields.
type StorageItemRow struct {
	InstanceID           id.UUID
	ItemID               string
	Quantity             int
	Section              string
	Slot                 string
	GuildID              id.UUID
	DepositorCharacterID id.UUID
	DepositorAccountID   id.UUID
	DepositedAt          time.Time
}

// ClaimRow mirrors guild_storage_claims.
type ClaimRow struct {
	ID         id.UUID
	GuildID    id.UUID
	Requester  id.UUID
	Instance   id.UUID
	Quantity   int
	State      string
	CreatedAt  time.Time
	ApprovedAt *time.Time
	Approver   id.UUID // nil-able encoded as IsNil
	ExpiresAt  time.Time
	ResolvedAt *time.Time
}

// Claim states (schema CHECK).
const (
	ClaimPending   = "PENDING"
	ClaimApproved  = "APPROVED"
	ClaimRejected  = "REJECTED"
	ClaimCancelled = "CANCELLED"
	ClaimExpired   = "EXPIRED"
	ClaimCompleted = "COMPLETED"
)

// PendingClaimLifetime / ApprovedClaimLifetime (guild_storage.md § Claims).
const (
	PendingClaimLifetime  = 72 * time.Hour
	ApprovedClaimLifetime = 7 * 24 * time.Hour
)

// MembershipAgeGate — cross-character withdraws need 72h membership.
const MembershipAgeGate = 72 * time.Hour

// WithdrawQuotaPerDay — Officer 20, Member 5, Leader/Vice unlimited.
func WithdrawQuotaPerDay(role string) int {
	switch role {
	case guild.RoleOfficer:
		return 20
	case guild.RoleMember:
		return 5
	default:
		return 0 // 0 = unlimited (Leader/Vice)
	}
}

// slotName encodes section into the location slot ("COMMON.3").
func slotName(section string, n int) string { return section + "." + strconv.Itoa(n) }

// slotSection reads the section prefix back out of a storage slot.
func slotSection(slot string) string {
	if i := strings.LastIndexByte(slot, '.'); i > 0 {
		return slot[:i]
	}
	return slot
}

func validSection(section string) bool {
	return section == SectionCommon || section == SectionReserve
}

// lockStorage acquires the canonical GuildStorage row-set lock plus the
// claims/audit guild-keyed row locks in priority order.
func lockStorage(ctx context.Context, tx pgx.Tx, guildID id.UUID) error {
	return lockorder.Acquire(ctx, tx,
		lockorder.GuildStorageLock(guildID),
		lockorder.RowLock("guild_storage_claims", guildID),
		lockorder.RowLock("guild_storage_audit", guildID))
}

// guildLevel reads guild_progression.guild_level (created with the
// guild row; missing progression = level 1 floor).
func guildLevel(ctx context.Context, tx pgx.Tx, guildID id.UUID) (int, error) {
	var level int
	err := tx.QueryRow(ctx,
		`SELECT guild_level FROM guild_progression WHERE guild_id=$1`, guildID).
		Scan(&level)
	if errors.Is(err, pgx.ErrNoRows) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	return level, nil
}

// sectionCount counts stored item lines in one section.
func sectionCount(ctx context.Context, tx pgx.Tx, guildID id.UUID, section string) (int, error) {
	var n int
	err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM item_locations
		  WHERE guild_id=$1 AND location_kind='GUILD_STORAGE'
		    AND slot LIKE $2`,
		guildID, section+".%").Scan(&n)
	return n, err
}

// freeSlot returns the first unoccupied slot index inside a section's
// capacity band (indexes start at 0).
func freeSlot(ctx context.Context, tx pgx.Tx, guildID id.UUID, section string, capN int) (int, error) {
	rows, err := tx.Query(ctx,
		`SELECT slot FROM item_locations
		  WHERE guild_id=$1 AND location_kind='GUILD_STORAGE'
		    AND slot LIKE $2`, guildID, section+".%")
	if err != nil {
		return -1, err
	}
	used := make(map[int]struct{})
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			rows.Close()
			return -1, err
		}
		if i := strings.LastIndexByte(s, '.'); i >= 0 {
			if n, err := strconv.Atoi(s[i+1:]); err == nil {
				used[n] = struct{}{}
			}
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return -1, err
	}
	for n := 0; n < capN; n++ {
		if _, ok := used[n]; !ok {
			return n, nil
		}
	}
	return -1, errCapacityFull
}

// storageItem loads one GUILD_STORAGE row + instance under the storage lock.
func storageItem(ctx context.Context, tx pgx.Tx, guildID, instanceID id.UUID) (*StorageItemRow, error) {
	var r StorageItemRow
	err := tx.QueryRow(ctx,
		`SELECT l.item_instance_id, i.item_id, i.quantity, l.slot,
		        l.guild_id,
		        l.depositor_character_id, l.depositor_account_id, l.updated_at
		   FROM item_locations l JOIN item_instances i
		     ON i.item_instance_id = l.item_instance_id
		  WHERE l.item_instance_id=$1 AND l.location_kind='GUILD_STORAGE'`,
		instanceID).
		Scan(&r.InstanceID, &r.ItemID, &r.Quantity, &r.Slot, &r.GuildID,
			&r.DepositorCharacterID, &r.DepositorAccountID, &r.DepositedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errItemNotFound
	}
	if err != nil {
		return nil, err
	}
	if r.GuildID != guildID {
		return nil, errItemNotFound
	}
	r.Section = slotSection(r.Slot)
	return &r, nil
}

// claimForUpdate locks and loads one claim row.
func claimForUpdate(ctx context.Context, tx pgx.Tx, claimID id.UUID) (*ClaimRow, error) {
	var c ClaimRow
	var approver *id.UUID
	err := tx.QueryRow(ctx,
		`SELECT claim_id, guild_id, requester_character_id, item_instance_id,
		        quantity, state, created_at, approved_at, approver_character_id,
		        expires_at, resolved_at
		   FROM guild_storage_claims WHERE claim_id=$1 FOR UPDATE`, claimID).
		Scan(&c.ID, &c.GuildID, &c.Requester, &c.Instance, &c.Quantity,
			&c.State, &c.CreatedAt, &c.ApprovedAt, &approver, &c.ExpiresAt, &c.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errClaimNotFound
	}
	if err != nil {
		return nil, err
	}
	if approver != nil {
		c.Approver = *approver
	}
	return &c, nil
}

// pendingClaimsOn returns open claims (PENDING/APPROVED) on an instance.
func openClaimsOn(ctx context.Context, tx pgx.Tx, guildID, instanceID id.UUID) ([]ClaimRow, error) {
	rows, err := tx.Query(ctx,
		`SELECT claim_id, guild_id, requester_character_id, item_instance_id,
		        quantity, state, created_at, approved_at, approver_character_id,
		        expires_at, resolved_at
		   FROM guild_storage_claims
		  WHERE guild_id=$1 AND item_instance_id=$2
		    AND state IN ('PENDING','APPROVED')`, guildID, instanceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ClaimRow
	for rows.Next() {
		var c ClaimRow
		var approver *id.UUID
		if err := rows.Scan(&c.ID, &c.GuildID, &c.Requester, &c.Instance,
			&c.Quantity, &c.State, &c.CreatedAt, &c.ApprovedAt, &approver,
			&c.ExpiresAt, &c.ResolvedAt); err != nil {
			return nil, err
		}
		if approver != nil {
			c.Approver = *approver
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// accountOf reads a character's account id.
func accountOf(ctx context.Context, tx pgx.Tx, char id.UUID) (id.UUID, error) {
	var a id.UUID
	err := tx.QueryRow(ctx,
		`SELECT account_id FROM characters WHERE character_id=$1`, char).Scan(&a)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, errGuildNotFound
	}
	return a, err
}

// withdrawsToday counts the actor's committed WITHDRAW audit rows since
// the UTC day start (quota counter — audit is the counter of record).
func withdrawsToday(ctx context.Context, tx pgx.Tx, guildID, actor id.UUID, dayStart time.Time) (int, error) {
	var n int
	err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM guild_storage_audit
		  WHERE guild_id=$1 AND actor_character_id=$2 AND action='WITHDRAW'
		    AND occurred_at >= $3`, guildID, actor, dayStart).Scan(&n)
	return n, err
}

// utcDayStart floors a timestamp to 00:00 UTC.
func utcDayStart(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// recordPartnerItems adds qty to the receiver's item_partner_counts[depositor]
// on today's UTC rollup (anti-RMT transfer signal, anti_cheat.md § partner
// counts; same tx, keyed by operation is handled by caller dedupe).
func recordPartnerItems(ctx context.Context, tx pgx.Tx, receiver, depositor id.UUID,
	qty int, day time.Time, now time.Time) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO economy_character_daily_rollups
		 (character_id, utc_day, item_partner_counts, updated_at)
		 VALUES ($1,$2, jsonb_build_object($3::text,$4::bigint), $5)
		 ON CONFLICT (character_id, utc_day) DO UPDATE SET
		   item_partner_counts = jsonb_set(
		     COALESCE(economy_character_daily_rollups.item_partner_counts,'{}'::jsonb),
		     ARRAY[$3::text],
		     to_jsonb(COALESCE((economy_character_daily_rollups.item_partner_counts->>$3)::bigint,0) + $4::bigint)),
		   updated_at = $5`,
		receiver.String(), day, depositor.String(), int64(qty), now)
	return err
}

// audit appends one fully-populated guild_storage_audit row (all
// columns per data_model.md § guild_storage_audit).
func audit(ctx context.Context, tx pgx.Tx, guildID, opID, actor id.UUID,
	action, section, itemID string, qty int, receiver, source *id.UUID,
	before, after int, at time.Time) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO guild_storage_audit
		 (audit_id, guild_id, operation_id, actor_character_id, action,
		  section, item_id, quantity, receiver_character_id,
		  source_character_id, before_quantity, after_quantity, occurred_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		id.NewV4(), guildID, opID, actor, action, section, itemID, qty,
		uuidOrNil(receiver), uuidOrNil(source), before, after, at)
	return err
}

func uuidOrNil(u *id.UUID) any {
	if u == nil || u.IsNil() {
		return nil
	}
	return *u
}

// bumpStorageRevision increments guilds.guild_storage_revision and
// returns the new value (S2C 631/647 carry it).
func bumpStorageRevision(ctx context.Context, tx pgx.Tx, guildID id.UUID) (uint64, error) {
	var rev uint64
	err := tx.QueryRow(ctx,
		`UPDATE guilds SET guild_storage_revision = guild_storage_revision + 1
		  WHERE guild_id=$1 RETURNING guild_storage_revision`, guildID).Scan(&rev)
	return rev, err
}

// FreeInventorySlot picks the first free CHARACTER_INVENTORY wire slot
// ("inv.N") under capacity.
func FreeInventorySlot(ctx context.Context, tx pgx.Tx, inv interface {
	Capacity(context.Context, pgx.Tx, id.UUID) (int32, error)
}, char id.UUID) (string, error) {
	capN, err := inv.Capacity(ctx, tx, char)
	if err != nil {
		return "", err
	}
	rows, err := tx.Query(ctx,
		`SELECT slot FROM item_locations
		  WHERE character_id=$1 AND location_kind='CHARACTER_INVENTORY'`, char)
	if err != nil {
		return "", err
	}
	used := make(map[int]struct{})
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			rows.Close()
			return "", err
		}
		if n, ok := parseInvSlot(s); ok {
			used[n] = struct{}{}
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return "", err
	}
	for n := 0; n < int(capN); n++ {
		if _, ok := used[n]; !ok {
			return fmt.Sprintf("inv.%d", n), nil
		}
	}
	return "", errInventoryFull
}

func parseInvSlot(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimPrefix(s, "inv."))
	return n, err == nil && strings.HasPrefix(s, "inv.")
}

// splitStorageStack carves qty units off an in-storage instance into a
// fresh instance+GUILD_STORAGE row inheriting the depositor ids
// (partial withdraw/deliver: the claim quantity is a slice of the
// stored stack; all-or-nothing delivery is enforced by the caller).
func splitStorageStack(ctx context.Context, tx pgx.Tx, src *StorageItemRow, qty int) (id.UUID, error) {
	if qty < 1 || qty >= src.Quantity {
		return id.UUID{}, fmt.Errorf("%w: split %d of %d", errInvalidQuantity, qty, src.Quantity)
	}
	newID := id.NewV4()
	if _, err := tx.Exec(ctx,
		`INSERT INTO item_instances
		 (item_instance_id, item_id, quantity, effective_binding,
		  enhancement_level, item_state, content_revision, created_at)
		 SELECT $1, item_id, $2, effective_binding, enhancement_level,
		        item_state, content_revision, created_at
		   FROM item_instances WHERE item_instance_id=$3`,
		newID, qty, src.InstanceID); err != nil {
		return id.UUID{}, err
	}
	section := slotSection(src.Slot)
	level, err := guildLevel(ctx, tx, src.GuildID)
	if err != nil {
		return id.UUID{}, err
	}
	slot, err := freeSlot(ctx, tx, src.GuildID, section, SectionCapacity(section, level))
	if err != nil {
		return id.UUID{}, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO item_locations
		 (item_instance_id, location_kind, guild_id, slot,
		  depositor_character_id, depositor_account_id, updated_at)
		 VALUES ($1,'GUILD_STORAGE',$2,$3,$4,$5,NOW())`,
		newID, src.GuildID, slotName(section, slot),
		src.DepositorCharacterID, src.DepositorAccountID); err != nil {
		return id.UUID{}, err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE item_instances SET quantity = quantity - $1
		  WHERE item_instance_id=$2`, qty, src.InstanceID); err != nil {
		return id.UUID{}, err
	}
	return newID, nil
}
