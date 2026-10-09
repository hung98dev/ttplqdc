package guild

import (
	"context"
	"time"

	"thinhthan/internal/core/id"
	durableguild "thinhthan/internal/durable/guild"
)

// Gathering slots (ADR-0062): a bonfire rest credit exists only when
// >=5 current members of one guild stay in the same bonfire + map
// instance for the whole aligned 300-second UTC slot
// floor(unix_seconds/300), and only the FIRST qualifying slot per UTC
// day pays. Grant keys are (guild_id, slot_start); Spirit Surge grants
// are keyed (guild_id, chain_id).

// SlotStart aligns ts to its 300-second UTC bucket.
func SlotStart(ts time.Time) time.Time {
	u := ts.UTC()
	return time.Unix(u.Unix()-u.Unix()%300, 0).UTC()
}

// utcDay is the civil day key for the first-slot-per-day rule.
func utcDay(ts time.Time) string {
	return ts.UTC().Format("2006-01-02")
}

// restStart records one character's rest-session anchor: the bonfire,
// map instance and guild membership at attach time. A member that
// joins, moves or detaches mid-slot drops that slot's quorum.
type restAnchor struct {
	bonfireID    string
	mapInstance  string
	guildID      id.UUID
	membershipID id.UUID
}

// Gathering tracks rest sessions and decides slot-end grants. It is
// in-memory runtime state — single-writer inside the global host; a
// process restart simply re-anchors living sessions (the slot requires
// a full slot anyway, so restarts re-anchor from the next slot).
type Gathering struct {
	Now  func() time.Time
	View MembershipView

	anchors map[id.UUID]restAnchor
	// granted[guildID] = utc-day already paid.
	granted map[id.UUID]string
	// surged[chainKey] dedupes Spirit Surge chain grants.
	surged map[string]bool
}

// NewGathering builds the tracker.
func NewGathering(view MembershipView, now func() time.Time) *Gathering {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Gathering{
		Now:     now,
		View:    view,
		anchors: map[id.UUID]restAnchor{},
		granted: map[id.UUID]string{},
		surged:  map[string]bool{},
	}
}

// RestStart anchors a rest session: the character's CURRENT guild
// binds the session — later joins/leaves are handled per-slot.
func (g *Gathering) RestStart(ctx context.Context, char id.UUID,
	bonfireID, mapInstance string, now time.Time) error {
	gid, memID, err := g.View.GuildOf(ctx, char)
	if err != nil {
		return err
	}
	g.anchors[char] = restAnchor{bonfireID: bonfireID, mapInstance: mapInstance, guildID: gid, membershipID: memID}
	return nil
}

// RestStop drops the anchor — the member no longer counts for any
// slot.
func (g *Gathering) RestStop(char id.UUID) {
	delete(g.anchors, char)
}

// SlotQuorum reports the qualifying guild group inside a slot: the
// largest set of still-anchored members sharing (bonfire, mapInstance,
// guild) >= 5 — and confirms every member was anchored for the WHOLE
// slot (anchor recorded at or before slot start, still present at
// slot end). The anchor is re-validated against the view so members
// who left mid-slot do not count.
func (g *Gathering) SlotQuorum(ctx context.Context, slot time.Time,
	membershipSince map[id.UUID]time.Time) (id.UUID, []durableguild.Credit) {
	type key struct {
		bonfire string
		mi      string
		gid     id.UUID
	}
	groups := map[key][]id.UUID{}
	var order []key
	for char, a := range g.anchors {
		k := key{a.bonfireID, a.mapInstance, a.guildID}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], char)
	}
	var bestChars []id.UUID
	var bestKey key
	for _, k := range order {
		if len(groups[k]) >= 5 && len(groups[k]) > len(bestChars) {
			bestChars = groups[k]
			bestKey = k
		}
	}
	if len(bestChars) == 0 {
		return id.UUID{}, nil
	}
	credits := make([]durableguild.Credit, 0, len(bestChars))
	for _, char := range bestChars {
		// Whole-slot: the character must still be a member of that
		// guild and must have been anchored/membered at slot start.
		gid, memID, err := g.View.GuildOf(ctx, char)
		if err != nil || gid != bestKey.gid || memID.IsNil() {
			return id.UUID{}, nil
		}
		if since, ok := membershipSince[char]; ok && since.After(slot) {
			return id.UUID{}, nil
		}
		credits = append(credits, durableguild.Credit{
			CharacterID:  char,
			MembershipID: memID,
			Amount:       1,
		})
	}
	return bestKey.gid, credits
}

// CloseSlot settles one aligned slot: if a qualifying group exists and
// the guild has not already been credited this UTC day, it emits one
// GUILD_ACTIVITY bonfire event keyed by (guild_id, slot_start).
// Returns the emitted event (nil when nothing qualified).
func (g *Gathering) CloseSlot(ctx context.Context, slot time.Time,
	membershipSince map[id.UUID]time.Time, sink durableguild.EventSink) (*durableguild.SourceEvent, error) {
	gid, credits := g.SlotQuorum(ctx, slot, membershipSince)
	if gid.IsNil() || len(credits) < 5 {
		return nil, nil
	}
	day := utcDay(slot)
	if g.granted[gid] == day {
		return nil, nil
	}
	g.granted[gid] = day
	start := slot
	ev := &durableguild.SourceEvent{
		Kind:       durableguild.SourceGuildActivity,
		SourceKey:  "bonfire:" + gid.String() + ":" + slot.Format(time.RFC3339),
		OccurredAt: slot,
		Credited:   credits,
		Element:    durableguild.ElementTho,
		SlotStart:  &start,
	}
	if sink != nil {
		if err := sink.Apply(ctx, *ev); err != nil {
			return nil, err
		}
	}
	return ev, nil
}

// SurgeGrant emits the Spirit Surge guild grant keyed by
// (guild_id, chain_id): the same chain never double-grants (ADR-0062).
func (g *Gathering) SurgeGrant(ctx context.Context, guildID id.UUID,
	chainID string, element uint32,
	credits []durableguild.Credit, occurredAt time.Time,
	sink durableguild.EventSink) error {
	key := guildID.String() + ":" + chainID
	if g.surged[key] {
		return nil
	}
	g.surged[key] = true
	ev := durableguild.SourceEvent{
		Kind:       durableguild.SourceWorldEvent,
		SourceKey:  "surge:" + key,
		OccurredAt: occurredAt,
		Credited:   credits,
		Element:    element,
		ChainID:    chainID,
	}
	if sink != nil {
		return sink.Apply(ctx, ev)
	}
	return nil
}
