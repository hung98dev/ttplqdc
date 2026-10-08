package world

import (
	"sync"

	"thinhthan/internal/core/id"
)

// npcSession is one open gameplay-capable NPC session (npcs.md § Session
// Contract): opened by TALK, required by NPC_SERVICE, expires after
// NPCSessionTicks, closes on range exit, transfer, despawn or death.
type npcSession struct {
	openedTick uint64
}

// npcSessions tracks open sessions per host (npc entity id → character).
// The mutex permits off-tick reads (tests, diagnostics); writers stay
// inside the channel tick.
type npcSessions struct {
	mu    sync.Mutex
	byNPC map[uint64]map[id.UUID]npcSession
}

func newNpcSessions() *npcSessions {
	return &npcSessions{byNPC: make(map[uint64]map[id.UUID]npcSession)}
}

// Open starts (or refreshes) the session between a character and an NPC.
func (s *npcSessions) Open(npcEntityID uint64, characterID id.UUID, tick uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.byNPC[npcEntityID]
	if m == nil {
		m = make(map[id.UUID]npcSession)
		s.byNPC[npcEntityID] = m
	}
	m[characterID] = npcSession{openedTick: tick}
}

// Valid reports a live, unexpired session between the pair at tick.
func (s *npcSessions) Valid(npcEntityID uint64, characterID id.UUID, tick uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.byNPC[npcEntityID]
	if m == nil {
		return false
	}
	sess, ok := m[characterID]
	if !ok {
		return false
	}
	return tick-sess.openedTick < NPCSessionTicks
}

// Touch refreshes an open session's expiry (a successful service call).
func (s *npcSessions) Touch(npcEntityID uint64, characterID id.UUID, tick uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.byNPC[npcEntityID]
	if m == nil {
		return
	}
	sess, ok := m[characterID]
	if !ok || tick-sess.openedTick >= NPCSessionTicks {
		return
	}
	m[characterID] = npcSession{openedTick: tick}
}

// Close drops one session; CloseAll drops every session of a character
// (transfer, despawn, death) or an NPC (despawn).
func (s *npcSessions) Close(npcEntityID uint64, characterID id.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m := s.byNPC[npcEntityID]; m != nil {
		delete(m, characterID)
	}
}

// CloseCharacter drops every session the character holds.
func (s *npcSessions) CloseCharacter(characterID id.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.byNPC {
		delete(m, characterID)
	}
}

// CloseNPC drops every session on an NPC entity (despawn).
func (s *npcSessions) CloseNPC(npcEntityID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byNPC, npcEntityID)
}
