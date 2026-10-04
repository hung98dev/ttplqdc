package lockorder

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	"thinhthan/internal/observability/core"
)

// LockMode selects the acquisition primitive issued for a Lock.
type LockMode int

const (
	// RowUpdate emits SELECT 1 FROM <table> WHERE <lock column> = $1
	// FOR UPDATE on the table's registered lock column.
	RowUpdate LockMode = iota
	// RowShare emits the same probe FOR SHARE.
	RowShare
	// Advisory emits pg_advisory_xact_lock on a canonical key hash —
	// used for composite keys (receipts, operations), the season
	// advisory lock, and tables with no registered row column.
	Advisory
	// AdvisoryShared emits pg_advisory_xact_lock_shared.
	AdvisoryShared
)

func (m LockMode) String() string {
	switch m {
	case RowUpdate:
		return "row_update"
	case RowShare:
		return "row_share"
	case Advisory:
		return "advisory"
	case AdvisoryShared:
		return "advisory_shared"
	}
	return "unknown"
}

// unsetPriority marks a Lock whose priority resolves via PriorityOf.
const unsetPriority Priority = -1

// Lock is one lock acquisition target. Key is the canonical sort key:
// 16-byte UUID for entity/owner locks, ASCII for text keys, or the
// composite receipt encoding (owner16 | family | 0x00 | operation16).
type Lock struct {
	Table    string
	Key      []byte
	Mode     LockMode
	priority Priority
	pred     *rowPredicate
}

// rowPredicate overrides a row lock's probe — used when one canonical
// table locks through different predicates at different priorities.
type rowPredicate struct {
	column string
	tail   string // literal AND-clause, e.g. "AND location_kind = 'GUILD_STORAGE'"
	uuid   bool
}

// RowLock returns an exclusive row lock on table's registered lock
// column for owner (a UUID-keyed column).
func RowLock(table string, owner id.UUID) Lock {
	k := owner.Bytes()
	return Lock{Table: table, Key: k[:], Mode: RowUpdate, priority: unsetPriority}
}

// RowLockText returns an exclusive row lock on a text-keyed lock column.
func RowLockText(table, key string) Lock {
	return Lock{Table: table, Key: []byte(key), Mode: RowUpdate, priority: unsetPriority}
}

// AdvisoryLock returns an exclusive advisory lock on table+key.
func AdvisoryLock(table string, key []byte) Lock {
	return Lock{Table: table, Mode: Advisory, priority: unsetPriority, Key: key}
}

// GuildStorageLock addresses the priority-13 guild-storage subset of
// item_locations — the dual-priority table of database.md § Lock Order.
// It probes the guild-scoped rows (guild_id + location_kind), not the
// item-instance lock column.
func GuildStorageLock(guild id.UUID) Lock {
	k := guild.Bytes()
	return Lock{
		Table:    "item_locations",
		Key:      k[:],
		Mode:     RowUpdate,
		priority: GuildStorage,
		pred:     &rowPredicate{column: "guild_id", tail: " AND location_kind = 'GUILD_STORAGE'", uuid: true},
	}
}

// effectivePriority resolves the lock's canonical priority.
func (l Lock) effectivePriority() (Priority, bool) {
	if l.priority != unsetPriority {
		return l.priority, true
	}
	p, ok := PriorityOf(l.Table)
	return p, ok
}

// rank is the table's index within its priority line (listed order).
func (l Lock) rank() (int, bool) {
	p, ok := l.effectivePriority()
	if !ok {
		return 0, false
	}
	for _, e := range CanonicalOrder {
		if e.Priority == p {
			for i, t := range e.Tables {
				if t == l.Table {
					return i, true
				}
			}
			return 0, false
		}
	}
	return 0, false
}

// less orders locks: priority, then listed table order inside the
// priority, then stable key byte order, then mode for determinism.
func less(a, b Lock) bool {
	pa, oka := a.effectivePriority()
	pb, okb := b.effectivePriority()
	switch {
	case !oka && !okb:
	case !oka:
		return false
	case !okb:
		return true
	case pa != pb:
		return pa < pb
	}
	ra, _ := a.rank()
	rb, _ := b.rank()
	if ra != rb {
		return ra < rb
	}
	if c := bytes.Compare(a.Key, b.Key); c != 0 {
		return c < 0
	}
	return a.Mode < b.Mode
}

// SortLocks orders locks into canonical acquisition order. An
// unregistered table is an error: it has no canonical placement.
func SortLocks(locks []Lock) error {
	for _, l := range locks {
		if _, ok := l.effectivePriority(); !ok {
			return fmt.Errorf("lockorder: unregistered table %q", l.Table)
		}
	}
	sort.SliceStable(locks, func(i, j int) bool { return less(locks[i], locks[j]) })
	return nil
}

// OutOfOrderError reports a caller presenting locks in non-canonical
// order to Acquire. Expected is the lock that must precede; Actual is
// the lock found in its place.
type OutOfOrderError struct {
	Expected string
	Actual   string
}

func (e *OutOfOrderError) Error() string {
	return fmt.Sprintf("lockorder: out-of-order acquisition: %s must precede %s", e.Expected, e.Actual)
}

// describe renders a lock for errors.
func describe(l Lock) string {
	return fmt.Sprintf("%s[%x](%s)", l.Table, l.Key, l.Mode)
}

// CheckOrder returns nil when locks are already in canonical order —
// the contract Acquire enforces before issuing any statement.
func CheckOrder(locks []Lock) error {
	for _, l := range locks {
		if _, ok := l.effectivePriority(); !ok {
			return fmt.Errorf("lockorder: unregistered table %q", l.Table)
		}
	}
	for i := 1; i < len(locks); i++ {
		if less(locks[i], locks[i-1]) {
			return &OutOfOrderError{Expected: describe(locks[i]), Actual: describe(locks[i-1])}
		}
	}
	return nil
}

// lockColumn is a table's registered row-lock column and its type.
type lockColumn struct {
	column string
	uuid   bool
}

// lockColumns maps canonical tables to the column a row lock probes.
// Only owner-keyed or entity-keyed columns register here; composite or
// ambiguous targets (friends pairs, receipts, provider dedup) use
// Advisory locks instead.
var lockColumns = map[string]lockColumn{
	"accounts":                         {column: "account_id", uuid: true},
	"account_password_credentials":     {column: "account_id", uuid: true},
	"account_identities":               {column: "account_id", uuid: true},
	"auth_session_families":            {column: "account_id", uuid: true},
	"auth_revocations":                 {column: "account_id", uuid: true},
	"account_login_history":            {column: "account_id", uuid: true},
	"erasure_intents":                  {column: "account_id", uuid: true},
	"characters":                       {column: "character_id", uuid: true},
	"character_activity":               {column: "character_id", uuid: true},
	"character_attach_events":          {column: "character_id", uuid: true},
	"character_chivalry":               {column: "character_id", uuid: true},
	"character_chat_restrictions":      {column: "character_id", uuid: true},
	"character_currencies":             {column: "character_id", uuid: true},
	"character_inventories":            {column: "character_id", uuid: true},
	"item_instances":                   {column: "item_instance_id", uuid: true},
	"item_locations":                   {column: "item_instance_id", uuid: true},
	"character_beasts":                 {column: "character_id", uuid: true},
	"character_beast_food_daily":       {column: "character_id", uuid: true},
	"beast_equipment_locations":        {column: "character_id", uuid: true},
	"character_souls":                  {column: "character_id", uuid: true},
	"character_soul_collection":        {column: "character_id", uuid: true},
	"character_soul_resonance":         {column: "character_id", uuid: true},
	"account_iap_entitlements":         {column: "account_id", uuid: true},
	"account_refund_consumed_events":   {column: "account_id", uuid: true},
	"account_cosmetic_entitlements":    {column: "account_id", uuid: true},
	"account_entitlement_claims":       {column: "account_id", uuid: true},
	"character_cosmetic_entitlements":  {column: "character_id", uuid: true},
	"character_cosmetic_equips":        {column: "character_id", uuid: true},
	"friend_requests":                  {column: "friend_request_id", uuid: true},
	"guilds":                           {column: "guild_id", uuid: true},
	"guild_memberships":                {column: "guild_id", uuid: true},
	"guild_membership_history":         {column: "guild_id", uuid: true},
	"guild_member_contributions":       {column: "guild_id", uuid: true},
	"guild_invites":                    {column: "guild_id", uuid: true},
	"guild_applications":               {column: "guild_id", uuid: true},
	"guild_stone_category_completions": {column: "guild_id", uuid: true},
	"guild_stone_masteries":            {column: "guild_id", uuid: true},
	"guild_cosmetic_entitlements":      {column: "guild_id", uuid: true},
	"guild_cosmetic_selections":        {column: "guild_id", uuid: true},
	"guild_progression":                {column: "guild_id", uuid: true},
	"guild_ritual_cycles":              {column: "guild_id", uuid: true},
	"guild_ritual_cycle_members":       {column: "guild_id", uuid: true},
	"guild_blessing_votes":             {column: "guild_id", uuid: true},
	"guild_storage_claims":             {column: "guild_id", uuid: true},
	"guild_storage_audit":              {column: "guild_id", uuid: true},
	"trade_settlement_records":         {column: "settlement_id", uuid: true},
	"auction_listings":                 {column: "listing_id", uuid: true},
	"auction_proceeds":                 {column: "listing_id", uuid: true},
	"competitive_match_admissions":     {column: "match_id", uuid: true},
	"pvp_ratings":                      {column: "character_id", uuid: true},
	"pvp_match_settlements":            {column: "pvp_match_id", uuid: true},
	"pvp_sanctions":                    {column: "character_id", uuid: true},
	"guild_war_ratings":                {column: "guild_id", uuid: true},
	"guild_war_settlements":            {column: "guild_war_match_id", uuid: true},
	"competitive_season_frozen_awards": {column: "owner_id", uuid: true},
	"reward_claims":                    {column: "reward_claim_id", uuid: true},
	"reward_claim_lines":               {column: "reward_claim_id", uuid: true},
	"reward_claim_contributions":       {column: "reward_claim_id", uuid: true},
	"boss_chest_eligibility":           {column: "character_id", uuid: true},
	"public_boss_reward_settlements":   {column: "character_id", uuid: true},
	"region_di_tich_markers":           {column: "region_id"},
	"world_consequence_relics":         {column: "relic_id"},
	"public_boss_schedules":            {column: "public_boss_spawn_generation_id", uuid: true},
	"character_feats":                  {column: "character_id", uuid: true},
	"character_feat_milestones":        {column: "character_id", uuid: true},
	"character_atlas":                  {column: "character_id", uuid: true},
	"character_atlas_state":            {column: "character_id", uuid: true},
	"economy_account_daily_rollups":    {column: "account_id", uuid: true},
	"economy_character_daily_rollups":  {column: "character_id", uuid: true},
}

// statement renders the acquisition SQL + argument for one lock.
func (l Lock) statement() (string, any, error) {
	switch l.Mode {
	case RowUpdate, RowShare:
		column, uuidCol, tail := "", false, ""
		if l.pred != nil {
			column, uuidCol, tail = l.pred.column, l.pred.uuid, l.pred.tail
		} else {
			c, ok := lockColumns[l.Table]
			if !ok {
				return "", nil, fmt.Errorf("lockorder: table %q has no registered lock column — use Advisory mode", l.Table)
			}
			column, uuidCol = c.column, c.uuid
		}
		var arg any
		cast := ""
		if uuidCol {
			if len(l.Key) != 16 {
				return "", nil, fmt.Errorf("lockorder: %s lock key must be a 16-byte UUID, got %d bytes", l.Table, len(l.Key))
			}
			var u id.UUID
			copy(u[:], l.Key)
			arg = u.String()
			cast = "::uuid"
		} else {
			arg = string(l.Key)
		}
		mode := "UPDATE"
		if l.Mode == RowShare {
			mode = "SHARE"
		}
		return fmt.Sprintf(`SELECT 1 FROM %q WHERE %q = $1%s%s FOR %s`, l.Table, column, cast, tail, mode), arg, nil
	case Advisory, AdvisoryShared:
		key := append([]byte(l.Table+"\x00"), l.Key...)
		fn := "pg_advisory_xact_lock"
		if l.Mode == AdvisoryShared {
			fn = "pg_advisory_xact_lock_shared"
		}
		// bytea keys carry arbitrary bytes (receipt triples, composite
		// ids); hex-encode before hashing since text params must be UTF-8.
		return fmt.Sprintf(`SELECT %s(hashtextextended(encode($1::bytea, 'hex'), 0))`, fn), key, nil
	}
	return "", nil, fmt.Errorf("lockorder: unknown lock mode %d", l.Mode)
}

// Metrics carries optional observability/core instruments for lock
// acquisition. Nil Metrics = telemetry disabled (the default).
type Metrics struct {
	wait   *core.Instrument
	reject *core.Instrument
}

// NewMetrics registers the lock-wait histogram and reject counter.
// Labels are bounded: mode over the four LockMode values, reason over
// the two reject classes.
func NewMetrics(r *core.Registry) (*Metrics, error) {
	wait, err := r.Register(core.Descriptor{
		Name:   "durable.lock_wait_ms",
		Kind:   core.Histogram,
		Unit:   "ms",
		Labels: []core.LabelDomain{{Key: "mode", Values: []string{"row_update", "row_share", "advisory", "advisory_shared"}}},
	})
	if err != nil {
		return nil, err
	}
	reject, err := r.Register(core.Descriptor{
		Name:   "durable.lock_order_rejects",
		Kind:   core.Counter,
		Labels: []core.LabelDomain{{Key: "reason", Values: []string{"out_of_order", "unregistered_table"}}},
	})
	if err != nil {
		return nil, err
	}
	return &Metrics{wait: wait, reject: reject}, nil
}

// Acquire issues lock statements on tx in canonical order. Callers must
// present locks already ordered by SortLocks (or produced by a helper
// that returns canonical order — ReceiptKeys, AccountCharacterSet);
// any inversion fails with *OutOfOrderError and issues nothing. Rows
// are never mutated here: callers lock the full set, then mutate, then
// insert the operations row last.
func Acquire(ctx context.Context, tx pgx.Tx, locks ...Lock) error {
	return acquire(ctx, tx, nil, locks)
}

// AcquireMetrics is Acquire with optional instrumentation.
func AcquireMetrics(ctx context.Context, tx pgx.Tx, m *Metrics, locks ...Lock) error {
	return acquire(ctx, tx, m, locks)
}

func acquire(ctx context.Context, tx pgx.Tx, m *Metrics, locks []Lock) error {
	for _, l := range locks {
		if _, ok := l.effectivePriority(); !ok {
			m.rejectAdd(ctx, "unregistered_table")
			return fmt.Errorf("lockorder: unregistered table %q", l.Table)
		}
	}
	for i := 1; i < len(locks); i++ {
		if less(locks[i], locks[i-1]) {
			m.rejectAdd(ctx, "out_of_order")
			return &OutOfOrderError{Expected: describe(locks[i]), Actual: describe(locks[i-1])}
		}
	}
	if tx == nil {
		return fmt.Errorf("lockorder: nil tx")
	}
	for _, l := range locks {
		sql, arg, err := l.statement()
		if err != nil {
			m.rejectAdd(ctx, "unregistered_table")
			return err
		}
		start := time.Now()
		if _, err := tx.Exec(ctx, sql, arg); err != nil {
			return fmt.Errorf("lockorder: acquire %s: %w", describe(l), err)
		}
		m.waitRecord(ctx, l.Mode, time.Since(start))
	}
	return nil
}

func (m *Metrics) waitRecord(ctx context.Context, mode LockMode, d time.Duration) {
	if m == nil || m.wait == nil {
		return
	}
	m.wait.Record(ctx, d.Milliseconds(), map[string]string{"mode": mode.String()})
}

func (m *Metrics) rejectAdd(ctx context.Context, reason string) {
	if m == nil || m.reject == nil {
		return
	}
	m.reject.Add(ctx, 1, map[string]string{"reason": reason})
}
