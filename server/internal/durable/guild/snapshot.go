package guild

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"thinhthan/internal/core/id"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// OnlineLookup resolves roster presence without importing runtime
// state — edge/global injects it when rendering 628 (members-only).
type OnlineLookup func(char id.UUID) (online bool, lastAt *time.Time)

// StateFor renders the S2C_GUILD_STATE (628) payload for the viewer's
// guild: full roster (role + online projection) plus the complete
// GuildProgressionView. Cosmetic fields stay empty — IMP-038 owns
// guild_cosmetic_* rows.
func (d Deps) StateFor(ctx context.Context, tx pgx.Tx, viewerChar id.UUID,
	online OnlineLookup) (*protocolv1.S2CGuildState, error) {
	m, err := d.Store.Member(ctx, tx, viewerChar)
	if err != nil {
		return nil, err
	}
	if m == nil {
		// the roster is members-only (guild.md § Views).
		return &protocolv1.S2CGuildState{}, nil
	}
	guildID := m.GuildID
	g, err := d.Store.Guild(ctx, tx, guildID)
	if err != nil {
		return nil, err
	}
	members, err := d.Store.Members(ctx, tx, guildID)
	if err != nil {
		return nil, err
	}
	p, err := d.Store.Progression(ctx, tx, guildID)
	if err != nil {
		return nil, err
	}
	out := &protocolv1.S2CGuildState{
		GuildId:          g.ID[:],
		GuildRevision:    g.Revision,
		GuildName:        g.Name,
		Role:             m.Role,
		Level:            uint32(p.Level),
		MembersCount:     uint32(len(members)),
		MaxMembers:       uint32(MemberCapacity(p.Level)),
		Motd:             g.Motd,
		Members:          make([]*protocolv1.GuildMemberView, 0, len(members)),
		CosmeticRevision: g.CosmeticRevision,
	}
	if g.RecruitmentMode == "APPLICATIONS" {
		out.RecruitmentMode = protocolv1.GuildRecruitmentMode_GUILD_RECRUITMENT_MODE_APPLICATIONS
	} else {
		out.RecruitmentMode = protocolv1.GuildRecruitmentMode_GUILD_RECRUITMENT_MODE_CLOSED
	}
	now := time.Now().UTC()
	if d.Now != nil {
		now = d.Now()
	}
	for _, mem := range members {
		mv := &protocolv1.GuildMemberView{
			CharacterId: mem.CharacterID[:],
			Role:        mem.Role,
		}
		if err := d.Store.decorateMember(ctx, tx, mv, mem.CharacterID, online, now); err != nil {
			return nil, err
		}
		out.Members = append(out.Members, mv)
	}
	pv, err := d.progressionView(ctx, tx, guildID, viewerChar, p)
	if err != nil {
		return nil, err
	}
	out.Progression = pv
	return out, nil
}

// decorateMember fills the roster row's display/online fields —
// display_name/class_id/level from characters, online_state +
// last_online_at from presence (offline-only field per proto note).
func (s *Store) decorateMember(ctx context.Context, tx pgx.Tx,
	mv *protocolv1.GuildMemberView, char id.UUID,
	online OnlineLookup, now time.Time) error {
	if err := tx.QueryRow(ctx,
		`SELECT name, class_id, level FROM characters WHERE character_id = $1`, char).
		Scan(&mv.DisplayName, &mv.ClassId, &mv.Level); err != nil {
		return err
	}
	isOnline, lastAt := false, (*time.Time)(nil)
	if online != nil {
		isOnline, lastAt = online(char)
	} else {
		l, active, err := s.LastAttachedAt(ctx, tx, char)
		if err != nil {
			return err
		}
		isOnline, lastAt = active, l
	}
	if isOnline {
		mv.OnlineState = protocolv1.OnlineState_ONLINE_STATE_ONLINE
	} else {
		mv.OnlineState = protocolv1.OnlineState_ONLINE_STATE_OFFLINE
		if lastAt != nil {
			mv.LastOnlineAt = lastAt.UnixMilli()
		}
	}
	return nil
}

// progressionView maps guild_progression + the open cycle onto
// GuildProgressionView (628 §progression projection): the current
// cycle's vessels, draft + vote state, and the viewer's contribution
// totals.
func (d Deps) progressionView(ctx context.Context, tx pgx.Tx,
	guildID, viewerChar id.UUID, p ProgressionRow) (*protocolv1.GuildProgressionView, error) {
	pv := &protocolv1.GuildProgressionView{
		GuildExp:         p.EXP,
		RitualStreak:     uint32(p.Streak),
		ActiveBlessingId: p.ActiveBlessingID,
	}
	if p.BlessingExpiresAt != nil {
		pv.BlessingExpiresAtMs = p.BlessingExpiresAt.UnixMilli()
	}
	now := time.Now().UTC()
	if d.Now != nil {
		now = d.Now()
	}
	cycleID := CycleID(now)
	c, err := d.Store.Cycle(ctx, tx, guildID, cycleID)
	if err != nil {
		return nil, err
	}
	pv.CycleId = cycleID
	if c != nil {
		pv.MEffective = uint32(c.MEffective)
		pv.RequiredPointsPerElement = uint32(c.Required)
		pv.Points = elementPoints(c)
		if c.CompletedAt != nil {
			pv.CompletedAtMs = c.CompletedAt.UnixMilli()
		}
		if len(c.Candidates) > 0 {
			pv.CandidateBlessingIds = c.Candidates
			if c.VoteClosesAt != nil {
				pv.VoteClosesAtMs = c.VoteClosesAt.UnixMilli()
			}
			counts, err := d.Store.VoteCounts(ctx, tx, guildID, cycleID, c.Candidates)
			if err != nil {
				return nil, err
			}
			pv.VoteCounts = make([]uint32, len(counts))
			for i, n := range counts {
				pv.VoteCounts[i] = uint32(n)
			}
			if !viewerChar.IsNil() {
				var accountID id.UUID
				if err := tx.QueryRow(ctx,
					`SELECT account_id FROM characters WHERE character_id = $1`, viewerChar).
					Scan(&accountID); err == nil {
					v, err := d.Store.ReceiverVote(ctx, tx, guildID, cycleID, accountID)
					if err != nil {
						return nil, err
					}
					pv.ReceiverVote = v
				}
			}
		}
	}
	if !viewerChar.IsNil() {
		contrib, err := d.Store.Contribution(ctx, tx, guildID, viewerChar)
		if err != nil {
			return nil, err
		}
		pv.ReceiverLifetimeContribution = contrib.Lifetime
		pv.ReceiverCycleContribution = contrib.Cycle
	}
	return pv, nil
}

// elementPoints exports the five vessel counters with the proto
// Element tag (KIM/MOC/THUY/HOA/THO journal order -> proto enum).
func elementPoints(c *CycleRow) []*protocolv1.GuildProgressionPoint {
	return []*protocolv1.GuildProgressionPoint{
		{Element: protocolv1.Element_ELEMENT_KIM, Current: uint32(c.Points[0])},
		{Element: protocolv1.Element_ELEMENT_MOC, Current: uint32(c.Points[1])},
		{Element: protocolv1.Element_ELEMENT_THUY, Current: uint32(c.Points[2])},
		{Element: protocolv1.Element_ELEMENT_HOA, Current: uint32(c.Points[3])},
		{Element: protocolv1.Element_ELEMENT_THO, Current: uint32(c.Points[4])},
	}
}
