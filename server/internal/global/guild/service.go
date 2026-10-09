// Package guild is the runtime-only front of the guild feature
// (guild.md, guild_progression.md, ADR-0062/0064/0068): it resolves
// eligible credited-member groups into guilds, enqueues GUILD_EVENT
// durable commands, tracks bonfire gathering slots and exposes the
// closed C2S -> 649 coverage registry. All durable mutation lives in
// durable/guild; this package never writes SQL directly.
package guild

import (
	"context"
	"fmt"
	"time"

	"thinhthan/internal/core/id"
	durableguild "thinhthan/internal/durable/guild"
	journalv1 "thinhthan/internal/durable/journal/v1"
)

// MembershipView resolves which guild each credited character belongs
// to at event time (the durable store implements it; tests use
// synthetics).
type MembershipView interface {
	// GuildOf returns (guild_id, membership_id) or zeroes when the
	// character is guildless.
	GuildOf(ctx context.Context, char id.UUID) (guildID, membershipID id.UUID, err error)
}

// EnqueuePort hands the assembled guild.event record to the durable
// command queue (IMP-069 wires the real queue; tests use synthetics).
type EnqueuePort interface {
	Enqueue(ctx context.Context, rec *journalv1.DurableCommandRecord) error
}

// Service is the guild EventSink front: producers (dungeon, boss,
// world event, ritual, Spirit Surge, bonfire) submit typed events;
// the service groups credits by guild, enforces the >=3 current-member
// threshold and emits one guild.event record per guild.
type Service struct {
	View    MembershipView
	Enqueue EnqueuePort
	Now     func() time.Time
}

// NewService wires the sink.
func NewService(view MembershipView, enqueue EnqueuePort, now func() time.Time) *Service {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{View: view, Enqueue: enqueue, Now: now}
}

// Apply is the durable EventSink contract (ADR-0068): one typed
// SourceEvent in, zero or more guild.event records out. A credited
// member outside any guild or counted for another guild is ignored —
// eligibility requires >=3 credited *current* members of ONE guild.
func (s *Service) Apply(ctx context.Context, ev durableguild.SourceEvent) error {
	if ev.Kind == "" || len(ev.Credited) < 3 {
		return nil
	}
	if ev.Element < 1 || ev.Element > 5 {
		return fmt.Errorf("guild: element %d outside 1..5", ev.Element)
	}
	groups := map[id.UUID][]durableguild.Credit{}
	order := []id.UUID{}
	for _, c := range ev.Credited {
		gid, memID, err := s.View.GuildOf(ctx, c.CharacterID)
		if err != nil {
			return err
		}
		if gid.IsNil() {
			continue
		}
		c.MembershipID = memID
		if _, ok := groups[gid]; !ok {
			order = append(order, gid)
		}
		groups[gid] = append(groups[gid], c)
	}
	now := s.Now()
	for _, gid := range order {
		credited := groups[gid]
		if len(credited) < 3 {
			continue
		}
		je := &journalv1.JournalGuildEvent{
			GuildId:           gid[:],
			SourceOperationId: ev.SourceOperationID[:],
			SourceKind:        string(ev.Kind),
			SourceReference:   ev.SourceKey,
			OccurredAtMs:      ev.OccurredAt.UnixMilli(),
			Element:           ev.Element,
			RitualPoints:      ritualPoints(ev.Kind),
			GuildExp:          guildEXP(ev.Kind),
			CycleId:           durableguild.CycleID(ev.OccurredAt),
			SeasonId:          ev.SeasonID,
		}
		if ev.ChainID != "" {
			// journal chain_id is bytes — the grant key's canonical
			// encoding (text chain ids encode as UTF-8).
			je.ChainId = []byte(ev.ChainID)
		}
		if ev.SlotStart != nil {
			ms := ev.SlotStart.UnixMilli()
			je.GatheringSlotStartMs = &ms
		}
		if len(ev.Credited) > 0 {
			var first id.UUID
			copy(first[:], ev.Credited[0].CharacterID[:])
			je.CharacterId = ev.Credited[0].CharacterID[:]
			je.MembershipId = ev.Credited[0].MembershipID[:]
		}
		je.CreditedMembers = make([]*journalv1.JournalGuildContribution, 0, len(credited))
		for _, c := range credited {
			je.CreditedMembers = append(je.CreditedMembers, &journalv1.JournalGuildContribution{
				CharacterId:  c.CharacterID[:],
				MembershipId: c.MembershipID[:],
				Amount:       c.Amount,
			})
		}
		opID := eventOperationID(gid, ev)
		if err := s.Enqueue.Enqueue(ctx, durableguild.EventRecord(gid, opID, je, now)); err != nil {
			return err
		}
	}
	return nil
}

// eventOperationID derives the deterministic guild.event operation
// (UUIDv5): (guild_id, source_operation_id, source_key) — replaying
// the same source event dedupes on the receipt, ADR-0062.
func eventOperationID(guildID id.UUID, ev durableguild.SourceEvent) id.UUID {
	return id.ServerJobOperationID(durableguild.EventFamily,
		guildID.String(), ev.SourceOperationID.String(), ev.SourceKey)
}

func guildEXP(kind durableguild.SourceKind) uint64 {
	switch kind {
	case durableguild.SourceDungeon, durableguild.SourceBoss:
		return 20
	case durableguild.SourceWorldEvent:
		return 15
	case durableguild.SourceGuildActivity:
		return 30
	case durableguild.SourceRitual:
		return 200
	default:
		return 0
	}
}

func ritualPoints(kind durableguild.SourceKind) uint32 {
	switch kind {
	case durableguild.SourceDungeon, durableguild.SourceBoss:
		return 12
	case durableguild.SourceWorldEvent:
		return 10
	case durableguild.SourceGuildActivity:
		return 15
	default:
		return 0
	}
}
