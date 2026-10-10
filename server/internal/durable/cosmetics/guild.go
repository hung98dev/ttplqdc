package cosmetics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
)

// GuildGrant persists one guild-owned cosmetic unlock. The
// (guild_id, cosmetic_id) PK makes a repeated unlock a committed
// no-op retaining the original grant audit (cosmetics.md §188).
func (s *Store) GuildGrant(ctx context.Context, tx pgx.Tx,
	guildID id.UUID, cosmeticID, sourceRef string,
	opID id.UUID, now time.Time) error {
	def, ok := Lookup(cosmeticID)
	if !ok || def.Scope != ScopeGuild {
		return ErrUnknownCosmetic
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO guild_cosmetic_entitlements
		    (guild_id, cosmetic_id, grant_operation_id,
		     source_reference, acquired_at)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (guild_id, cosmetic_id) DO NOTHING`,
		guildID[:], cosmeticID, opID[:], sourceRef, now)
	return err
}

// GuildCosmeticView is the 628 cosmetic projection the guild
// composition consumes (F-16-12): unlocked ids, the three slot
// selections and the standalone cosmetic revision.
type GuildCosmeticView struct {
	Owned    []string
	Shrine   string
	Banner   string
	Crest    string
	Revision uint64
}

// GuildCosmeticState reads the member-facing guild cosmetic view.
func (s *Store) GuildCosmeticState(ctx context.Context, tx pgx.Tx,
	guildID id.UUID) (GuildCosmeticView, error) {
	v := GuildCosmeticView{}
	if err := s.db(tx).QueryRow(ctx,
		`SELECT guild_cosmetic_revision FROM guilds WHERE guild_id = $1`,
		guildID[:]).Scan(&v.Revision); err != nil {
		return v, fmt.Errorf("cosmetics: guild revision: %w", err)
	}
	rows, err := s.db(tx).Query(ctx,
		`SELECT cosmetic_id FROM guild_cosmetic_entitlements
		 WHERE guild_id = $1 ORDER BY cosmetic_id`, guildID[:])
	if err != nil {
		return v, fmt.Errorf("cosmetics: guild entitlements: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return v, err
		}
		v.Owned = append(v.Owned, c)
	}
	if err := rows.Err(); err != nil {
		return v, err
	}
	sel, err := s.db(tx).Query(ctx,
		`SELECT slot_id, cosmetic_id FROM guild_cosmetic_selections
		 WHERE guild_id = $1`, guildID[:])
	if err != nil {
		return v, fmt.Errorf("cosmetics: guild selections: %w", err)
	}
	defer sel.Close()
	for sel.Next() {
		var slot string
		var c *string
		if err := sel.Scan(&slot, &c); err != nil {
			return v, err
		}
		val := ""
		if c != nil {
			val = *c
		}
		switch slot {
		case "SHRINE":
			v.Shrine = val
		case "BANNER":
			v.Banner = val
		case "CREST":
			v.Crest = val
		}
	}
	return v, sel.Err()
}

// slotCategory maps the wire guild slot onto the required cosmetic
// category (cosmetics.md §190: the selected id must be an unlocked
// cosmetic of that exact category for the same guild).
func slotCategory(slot string) (Category, bool) {
	switch slot {
	case "SHRINE":
		return CategoryGuildShrineVisual, true
	case "BANNER":
		return CategoryGuildBanner, true
	case "CREST":
		return CategoryGuildCrestAccent, true
	}
	return 0, false
}

// memberRole returns the actor's role inside the guild under the
// caller's tx (guild_memberships is IMP-036's table — read-only here).
func (s *Store) memberRole(ctx context.Context, tx pgx.Tx,
	guildID, char id.UUID) (string, error) {
	var role string
	err := tx.QueryRow(ctx,
		`SELECT role FROM guild_memberships
		 WHERE character_id = $1 AND guild_id = $2`,
		char[:], guildID[:]).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotGuildMember
	}
	return role, err
}

// GuildEquip applies one 656 slot selection: LEADER/VICE_LEADER only,
// expected_revision gate against guild_cosmetic_revision, the
// selected id must be unlocked for the same guild and match the
// slot's exact category. Revision bumps only on a real selection
// change; the commit happens inside the caller's tx (cosmetics.md
// §190-192).
func (s *Store) GuildEquip(ctx context.Context, tx pgx.Tx,
	actorChar, guildID id.UUID, slot string, cosmeticID string,
	expectedRevision uint64) error {
	cat, ok := slotCategory(slot)
	if !ok {
		return ErrInvalidSlot
	}
	// Lock the guild row before mutable checks (cosmetics.md §192).
	var rev uint64
	if err := tx.QueryRow(ctx,
		`SELECT guild_cosmetic_revision FROM guilds
		 WHERE guild_id = $1 FOR UPDATE`, guildID[:]).Scan(&rev); err != nil {
		return fmt.Errorf("cosmetics: guild: %w", err)
	}
	role, err := s.memberRole(ctx, tx, guildID, actorChar)
	if err != nil {
		return err
	}
	if role != "LEADER" && role != "VICE_LEADER" {
		return ErrPermission
	}
	if rev != expectedRevision {
		return ErrRevisionConflict
	}
	var current *string
	row := tx.QueryRow(ctx,
		`SELECT cosmetic_id FROM guild_cosmetic_selections
		 WHERE guild_id = $1 AND slot_id = $2`, guildID[:], slot)
	var cur string
	if err := row.Scan(&cur); err == nil {
		current = &cur
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if cosmeticID == "" {
		if current == nil {
			return nil // no real change → no revision bump
		}
		if _, err := tx.Exec(ctx,
			`DELETE FROM guild_cosmetic_selections
			 WHERE guild_id = $1 AND slot_id = $2`,
			guildID[:], slot); err != nil {
			return err
		}
	} else {
		def, ok := Lookup(cosmeticID)
		if !ok || def.Category != cat {
			return ErrWrongCategory
		}
		var unlocked int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM guild_cosmetic_entitlements
			 WHERE guild_id = $1 AND cosmetic_id = $2`,
			guildID[:], cosmeticID).Scan(&unlocked); err != nil {
			return err
		}
		if unlocked == 0 {
			return ErrGuildCosmeticLocked
		}
		if current != nil && *current == cosmeticID {
			return nil // same selection → no revision bump
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO guild_cosmetic_selections
			    (guild_id, slot_id, cosmetic_id)
			 VALUES ($1,$2,$3)
			 ON CONFLICT (guild_id, slot_id)
			 DO UPDATE SET cosmetic_id = EXCLUDED.cosmetic_id`,
			guildID[:], slot, cosmeticID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx,
		`UPDATE guilds SET guild_cosmetic_revision = guild_cosmetic_revision + 1
		 WHERE guild_id = $1`, guildID[:]); err != nil {
		return err
	}
	return nil
}
