package session

import (
	"context"
	"errors"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/account"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// errProto carries an S2C_ERROR code to the adapter.
type protoError struct{ code protocolv1.ErrorCode }

func (e *protoError) Error() string { return e.code.String() }

func protoErr(code protocolv1.ErrorCode) error { return &protoError{code: code} }

// attach handles C2S_CHARACTER_ATTACH (id 6): ownership + pending-deletion
// + already-active checks (messages.md id 6-7), then journals
// character.activity ATTACHED and answers S2C_CHARACTER_ATTACH_OK.
// The login queue never applies here (attach never SERVER_OVERLOADED).
func (r *Registry) attach(ctx context.Context, s *sess, msg *protocolv1.C2SCharacterAttach) error {
	if len(msg.GetCharacterId()) != 16 {
		return protoErr(protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	var charID id.UUID
	copy(charID[:], msg.GetCharacterId())

	r.mu.Lock()
	if s.charID != nil {
		r.mu.Unlock()
		return protoErr(protocolv1.ErrorCode_ERROR_CODE_CHARACTER_ALREADY_ACTIVE)
	}
	if s.pendingDeletion {
		r.mu.Unlock()
		return protoErr(protocolv1.ErrorCode_ERROR_CODE_ACCOUNT_PENDING_DELETION)
	}
	if _, held := r.liveChar[charID]; held {
		r.mu.Unlock()
		return protoErr(protocolv1.ErrorCode_ERROR_CODE_CHARACTER_ALREADY_ACTIVE)
	}
	r.mu.Unlock()

	// Ownership check outside the lock (row read); the winner's claim is
	// re-checked under the mutex below.
	crow, err := r.store.GetCharacterOwner(ctx, nil, charID)
	if err != nil {
		if errors.Is(err, account.ErrNotFound) {
			return protoErr(protocolv1.ErrorCode_ERROR_CODE_NOT_OWNER)
		}
		return err
	}
	if crow.AccountID != s.accountID {
		return protoErr(protocolv1.ErrorCode_ERROR_CODE_NOT_OWNER)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if s.charID != nil {
		return protoErr(protocolv1.ErrorCode_ERROR_CODE_CHARACTER_ALREADY_ACTIVE)
	}
	if _, held := r.liveChar[charID]; held {
		return protoErr(protocolv1.ErrorCode_ERROR_CODE_CHARACTER_ALREADY_ACTIVE)
	}
	now := r.cfg.Now()
	s.charID = &charID
	s.ownershipEpoch = r.mintOwnershipEpochLocked(charID)
	s.attaching = false
	r.liveChar[charID] = s
	r.queue.setState(s.accountID, slotAttached)

	if err := account.SubmitActivity(ctx, r.q, charID, s.epoch,
		account.ActivityAttached, now); err != nil {
		return err
	}
	return r.send(s.conn, 7, &protocolv1.S2CCharacterAttachOk{
		CharacterId:     charID[:],
		OwnershipEpoch:  s.ownershipEpoch,
		MapId:           crow.MapID,
		ChannelIndex:    0,
		InstanceId:      nil, // empty in the normal world
		ContentRevision: s.contentRevision,
	})
}

// detach handles C2S_CHARACTER_DETACH (id 10): the listener only dispatches
// it in IN_WORLD|DEAD, matching messages.md. Journals DETACHED, answers
// S2C_CHARACTER_DETACH_OK followed by S2C_CHARACTER_LIST.
func (r *Registry) detach(ctx context.Context, s *sess, msg *protocolv1.C2SCharacterDetach) error {
	r.mu.Lock()
	if s.charID == nil {
		r.mu.Unlock()
		return protoErr(protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE)
	}
	charID := *s.charID
	s.charID = nil
	s.ownershipEpoch = 0
	delete(r.liveChar, charID)
	r.queue.setState(s.accountID, slotCharacterSelect)
	now := r.cfg.Now()
	r.mu.Unlock()

	if err := account.SubmitActivity(ctx, r.q, charID, s.epoch,
		account.ActivityDetached, now); err != nil {
		return err
	}
	if err := r.send(s.conn, 11, &protocolv1.S2CCharacterDetachOk{
		CharacterId: charID[:],
	}); err != nil {
		return err
	}
	return r.sendCharacterList(ctx, s)
}

// sendCharacterList emits S2C_CHARACTER_LIST (id 14): after HELLO_OK when
// nothing was resumed, after detach, after create.
func (r *Registry) sendCharacterList(ctx context.Context, s *sess) error {
	rows, err := r.store.ListCharacters(ctx, nil, s.accountID)
	if err != nil {
		return err
	}
	list := &protocolv1.S2CCharacterList{CharacterSlots: 3}
	for _, c := range rows {
		lastOnline := int64(0)
		if c.LastOnlineAt != nil {
			lastOnline = c.LastOnlineAt.UnixMilli()
		}
		list.Characters = append(list.Characters, &protocolv1.CharacterSummary{
			CharacterId:    c.CharacterID[:],
			CharacterName:  c.Name,
			ClassId:        c.ClassID,
			Level:          uint32(c.Level),
			MapId:          c.MapID,
			LastOnlineAtMs: lastOnline,
			IsAttached:     c.SessionActive,
		})
	}
	r.mu.Lock()
	conn := s.conn
	r.mu.Unlock()
	if conn == nil {
		return nil
	}
	return r.send(conn, 14, list)
}
