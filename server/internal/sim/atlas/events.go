package atlas

import (
	durableatlas "thinhthan/internal/durable/atlas"
)

// EventKind re-exports the durable event-kind set so sim callers name
// source events without importing the durable package.
type EventKind = durableatlas.EventKind

const (
	EventMonsterKilled = durableatlas.EventMonsterKilled
	EventSoulAcquired  = durableatlas.EventSoulAcquired
	EventBossWitness   = durableatlas.EventBossWitness
	EventChestOpened   = durableatlas.EventChestOpened
	EventFishCaught    = durableatlas.EventFishCaught
	EventDishCooked    = durableatlas.EventDishCooked
)

// SourceEvent is one authoritative observation feeding Atlas pages.
// Value is the event count for ApplyCount pages (kills, opens, catches,
// cooks, witnesses) and the achieved soul level for ApplyMax pages.
type SourceEvent struct {
	Kind     EventKind
	SourceID string // authored source id, e.g. monster.lang_da.dom_dom_ma
	Value    uint64
}

// Resolve returns the authored pages this event feeds; empty when no
// page matches — CHEST_SPOTTED and QUEST_CLUE never resolve.
func Resolve(ev SourceEvent) []durableatlas.Page {
	if ev.Value == 0 {
		return nil
	}
	return durableatlas.PagesForEvent(ev.Kind, ev.SourceID)
}
