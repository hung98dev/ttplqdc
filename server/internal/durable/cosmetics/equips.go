package cosmetics

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// slotName is the persisted slot key for the wire slot enum (the
// VARCHAR(32) column stores the enum suffix lower-cased).
func slotName(slot protocolv1.CosmeticSlot) (string, bool) {
	switch slot {
	case protocolv1.CosmeticSlot_COSMETIC_SLOT_TITLE:
		return "title", true
	case protocolv1.CosmeticSlot_COSMETIC_SLOT_TITLE_GLOW:
		return "title_glow", true
	case protocolv1.CosmeticSlot_COSMETIC_SLOT_FRAME:
		return "frame", true
	case protocolv1.CosmeticSlot_COSMETIC_SLOT_NAMEPLATE:
		return "nameplate", true
	case protocolv1.CosmeticSlot_COSMETIC_SLOT_APPEARANCE:
		return "appearance", true
	case protocolv1.CosmeticSlot_COSMETIC_SLOT_WEAPON_TRAIL:
		return "weapon_trail", true
	case protocolv1.CosmeticSlot_COSMETIC_SLOT_AURA:
		return "aura", true
	case protocolv1.CosmeticSlot_COSMETIC_SLOT_CHARACTER_SHRINE:
		return "character_shrine", true
	case protocolv1.CosmeticSlot_COSMETIC_SLOT_GUILD_STONE_INSCRIPTION:
		return "guild_stone_inscription", true
	}
	return "", false
}

// Equip validates and persists one slot change. cosmeticID empty
// unequips; otherwise the cosmetic must exist, match the slot's
// category and be owned by this character (character rows) or its
// account (IAP). The first equip of an account-entitled cosmetic sets
// first_equipped_at exactly once (monetization.md refund
// classification). Equips never touch gameplay state — presentation
// only (cosmetics.md § Equip).
func (s *Store) Equip(ctx context.Context, tx pgx.Tx, char id.UUID,
	slot protocolv1.CosmeticSlot, cosmeticID string) error {
	name, ok := slotName(slot)
	if !ok {
		return ErrInvalidSlot
	}
	if cosmeticID == "" {
		_, err := tx.Exec(ctx,
			`DELETE FROM character_cosmetic_equips
			 WHERE character_id = $1 AND slot = $2`, char[:], name)
		return err
	}
	def, ok := Lookup(cosmeticID)
	if !ok || def.Scope == ScopeGuild {
		return ErrUnknownCosmetic
	}
	want, hasSlot := def.Category.Slot()
	if !hasSlot || want != slot {
		return ErrInvalidSlot
	}
	acct, err := s.accountOf(ctx, tx, char)
	if err != nil {
		return err
	}
	owned, err := s.Owned(ctx, tx, acct, char, cosmeticID)
	if err != nil {
		return err
	}
	if !owned {
		return ErrNotOwned
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO character_cosmetic_equips (character_id, slot, cosmetic_id)
		 VALUES ($1,$2,$3)
		 ON CONFLICT (character_id, slot)
		 DO UPDATE SET cosmetic_id = EXCLUDED.cosmetic_id`,
		char[:], name, cosmeticID); err != nil {
		return err
	}
	if def.Scope == ScopeAccount {
		// Write-once: only rows still NULL count as a first equip.
		if _, err := tx.Exec(ctx,
			`UPDATE account_cosmetic_entitlements
			 SET first_equipped_at = now()
			 WHERE account_id = $1 AND cosmetic_id = $2
			   AND first_equipped_at IS NULL`, acct[:], cosmeticID); err != nil {
			return fmt.Errorf("cosmetics: first_equipped_at: %w", err)
		}
	}
	return nil
}

// StateFor renders S2C_COSMETIC_STATE (438): the owned set (CHARACTER
// scope rows + account-entitled rows) plus equipped slots. An
// entitlement that is no longer owned or no longer resolvable falls
// back to none — the equip row is reported only while the cosmetic is
// owned and its category still fits the slot (cosmetics.md §40).
func (s *Store) StateFor(ctx context.Context, tx pgx.Tx,
	char id.UUID) (*protocolv1.S2CCosmeticState, error) {
	acct, err := s.accountOf(ctx, tx, char)
	if err != nil {
		return nil, err
	}
	out := &protocolv1.S2CCosmeticState{}
	seen := map[string]bool{}
	charRows, err := s.CharacterEntitlements(ctx, tx, char)
	if err != nil {
		return nil, err
	}
	for _, r := range charRows {
		if !seen[r.CosmeticID] {
			seen[r.CosmeticID] = true
			out.Owned = append(out.Owned, &protocolv1.OwnedCosmeticView{
				CosmeticId: r.CosmeticID,
				Scope:      protocolv1.CosmeticScope_COSMETIC_SCOPE_CHARACTER,
			})
		}
	}
	acctIDs, err := s.AccountEntitlements(ctx, tx, acct)
	if err != nil {
		return nil, err
	}
	for _, cid := range acctIDs {
		if !seen[cid] {
			seen[cid] = true
			out.Owned = append(out.Owned, &protocolv1.OwnedCosmeticView{
				CosmeticId: cid,
				Scope:      protocolv1.CosmeticScope_COSMETIC_SCOPE_ACCOUNT,
			})
		}
	}
	equips, err := s.Equips(ctx, tx, char)
	if err != nil {
		return nil, err
	}
	for slot, cid := range equips {
		protoSlot, ok := protoSlot(slot)
		if !ok {
			continue
		}
		def, ok := Lookup(cid)
		if !ok || def.Scope == ScopeGuild {
			continue
		}
		want, hasSlot := def.Category.Slot()
		if !hasSlot || want != protoSlot {
			continue
		}
		owned, err := s.Owned(ctx, tx, acct, char, cid)
		if err != nil {
			return nil, err
		}
		if !owned {
			continue
		}
		out.Equipped = append(out.Equipped, &protocolv1.EquippedCosmetic{
			Slot:       protoSlot,
			CosmeticId: cid,
		})
	}
	return out, nil
}

func protoSlot(name string) (protocolv1.CosmeticSlot, bool) {
	switch name {
	case "title":
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_TITLE, true
	case "title_glow":
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_TITLE_GLOW, true
	case "frame":
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_FRAME, true
	case "nameplate":
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_NAMEPLATE, true
	case "appearance":
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_APPEARANCE, true
	case "weapon_trail":
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_WEAPON_TRAIL, true
	case "aura":
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_AURA, true
	case "character_shrine":
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_CHARACTER_SHRINE, true
	case "guild_stone_inscription":
		return protocolv1.CosmeticSlot_COSMETIC_SLOT_GUILD_STONE_INSCRIPTION, true
	}
	return protocolv1.CosmeticSlot_COSMETIC_SLOT_UNSPECIFIED, false
}
