package social

import (
	"context"
	"strings"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	sociald "thinhthan/internal/durable/social"
	"thinhthan/internal/edge/router"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// friendRequest is the router.Handler for C2S_FRIEND_REQUEST (611).
func (s *Service) friendRequest(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SFriendRequest)
	if !ok {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	// Social-invite rate budget: 10/60s per requester (messages.md §611).
	if !s.limiter.allowInvite(*v.CharacterID, s.now()) {
		return s.sendSocialResult(ctx, v, req.GetOperationId(), 611,
			req.GetTargetCharacterId(), protocolv1.ErrorCode_ERROR_CODE_RATE_LIMITED)
	}
	rec, err := sociald.FriendRequestRecord(v.AccountID, *v.CharacterID,
		v.SessionEpoch, v.OwnershipEpoch, req, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	outcome, err := s.run(ctx, v, rec)
	if err != nil {
		return err
	}
	result := outcome.GetS2CSocialResult()
	if result == nil {
		return &router.RejectError{Code: outcome.GetErrorCode()}
	}
	if err := s.send(v.Conn, msgIDSocialResult, result); err != nil {
		return err
	}
	if result.GetResult().GetStatus() == protocolv1.ResultStatus_RESULT_STATUS_SUCCESS {
		s.emitFriendPairChange(ctx, *v.CharacterID, req)
	}
	return s.ack(ctx, rec, v)
}

// friendAccept is the router.Handler for C2S_FRIEND_ACCEPT (613).
func (s *Service) friendAccept(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SFriendAccept)
	if !ok {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	rec, err := sociald.FriendAcceptRecord(v.AccountID, *v.CharacterID,
		v.SessionEpoch, v.OwnershipEpoch, req, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	return s.runSocial(ctx, v, rec, req.GetRequesterCharacterId(), func() {
		s.emitFriendUpsert(ctx, *v.CharacterID, mustID(req.GetRequesterCharacterId()))
		s.emitFriendUpsert(ctx, mustID(req.GetRequesterCharacterId()), *v.CharacterID)
		s.emitFriendRequests(ctx, *v.CharacterID)
		s.emitFriendRequests(ctx, mustID(req.GetRequesterCharacterId()))
	})
}

// friendDecline is the router.Handler for C2S_FRIEND_DECLINE (614).
func (s *Service) friendDecline(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SFriendDecline)
	if !ok {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	rec, err := sociald.FriendDeclineRecord(v.AccountID, *v.CharacterID,
		v.SessionEpoch, v.OwnershipEpoch, req, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	return s.runSocial(ctx, v, rec, req.GetRequesterCharacterId(), func() {
		s.emitFriendRequests(ctx, *v.CharacterID)
		s.emitFriendRequests(ctx, mustID(req.GetRequesterCharacterId()))
	})
}

// friendRemove is the router.Handler for C2S_FRIEND_REMOVE (615).
func (s *Service) friendRemove(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SFriendRemove)
	if !ok {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	rec, err := sociald.FriendRemoveRecord(v.AccountID, *v.CharacterID,
		v.SessionEpoch, v.OwnershipEpoch, req, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	return s.runSocial(ctx, v, rec, req.GetTargetCharacterId(), func() {
		target := mustID(req.GetTargetCharacterId())
		s.emitFriendRemoved(ctx, *v.CharacterID, target)
		s.emitFriendRemoved(ctx, target, *v.CharacterID)
	})
}

// blockAdd is the router.Handler for C2S_BLOCK_ADD (617): the commit
// already severed friendship + pendings; pushes send the fresh 619 to
// the blocker and 616 deltas to both sides.
func (s *Service) blockAdd(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SBlockAdd)
	if !ok {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	rec, err := sociald.BlockAddRecord(v.AccountID, *v.CharacterID,
		v.SessionEpoch, v.OwnershipEpoch, req, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	return s.runSocial(ctx, v, rec, req.GetTargetCharacterId(), func() {
		target := mustID(req.GetTargetCharacterId())
		s.emitFriendRemoved(ctx, *v.CharacterID, target)
		s.emitFriendRemoved(ctx, target, *v.CharacterID)
		s.emitFriendRequests(ctx, *v.CharacterID)
		s.emitFriendRequests(ctx, target)
		s.emitBlockState(ctx, *v.CharacterID)
	})
}

// blockRemove is the router.Handler for C2S_BLOCK_REMOVE (618).
func (s *Service) blockRemove(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SBlockRemove)
	if !ok {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	rec, err := sociald.BlockRemoveRecord(v.AccountID, *v.CharacterID,
		v.SessionEpoch, v.OwnershipEpoch, req, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	return s.runSocial(ctx, v, rec, req.GetTargetCharacterId(), func() {
		s.emitBlockState(ctx, *v.CharacterID)
	})
}

// report is the router.Handler for C2S_REPORT_PLAYER (632).
func (s *Service) report(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SReportPlayer)
	if !ok {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	rec, err := sociald.ReportRecord(v.AccountID, *v.CharacterID,
		v.SessionEpoch, v.OwnershipEpoch, req, s.now())
	if err != nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	outcome, err := s.run(ctx, v, rec)
	if err != nil {
		return err
	}
	result := outcome.GetS2CReportPlayerResult()
	if result == nil {
		return &router.RejectError{Code: outcome.GetErrorCode()}
	}
	if err := s.send(v.Conn, msgIDReportResult, result); err != nil {
		return err
	}
	return s.ack(ctx, rec, v)
}

// runSocial is the shared tail for the 654 ops: run → deliver 654 → on
// SUCCESS run the push hook → Ack.
func (s *Service) runSocial(ctx context.Context, v router.View,
	rec *journalv1.DurableCommandRecord, target []byte, push func()) error {
	outcome, err := s.run(ctx, v, rec)
	if err != nil {
		return err
	}
	result := outcome.GetS2CSocialResult()
	if result == nil {
		return &router.RejectError{Code: outcome.GetErrorCode()}
	}
	if err := s.send(v.Conn, msgIDSocialResult, result); err != nil {
		return err
	}
	if result.GetResult().GetStatus() == protocolv1.ResultStatus_RESULT_STATUS_SUCCESS &&
		push != nil {
		push()
	}
	return s.ack(ctx, rec, v)
}

// sendSocialResult sends a pre-admission 654 (rate-limited friend
// request never reaches the queue — the contract still returns the
// typed result).
func (s *Service) sendSocialResult(ctx context.Context, v router.View,
	opBytes []byte, reqMsgID uint32, target []byte,
	code protocolv1.ErrorCode) error {
	return s.send(v.Conn, msgIDSocialResult, &protocolv1.S2CSocialResult{
		Result: &protocolv1.OperationResult{
			OperationId: opBytes,
			Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
			ErrorCode:   code,
		},
		RequestMessageId:  reqMsgID,
		TargetCharacterId: target,
	})
}

// ack closes the durable disposition (deliver-then-ack).
func (s *Service) ack(ctx context.Context, rec *journalv1.DurableCommandRecord,
	v router.View) error {
	var opID id.UUID
	copy(opID[:], rec.GetOperationId())
	owner := idempotency.Owner{Kind: idempotency.OwnerCharacter, ID: *v.CharacterID}
	if err := s.q.Ack(ctx, rec.GetOperationFamily(), owner, opID); err != nil &&
		!strings.Contains(err.Error(), "no ackable") {
		return err
	}
	return nil
}

// emitFriendPairChange pushes the post-request state for 611 success:
// a new live pending (requester→outgoing, target→incoming + 612) or a
// crossed-accept friendship upsert.
func (s *Service) emitFriendPairChange(ctx context.Context, requester id.UUID,
	req *protocolv1.C2SFriendRequest) {
	target := mustID(req.GetTargetCharacterId())
	friends, err := s.store.AreFriends(ctx, nil, requester, target)
	if err == nil && friends {
		s.emitFriendUpsert(ctx, requester, target)
		s.emitFriendUpsert(ctx, target, requester)
		s.emitFriendRequests(ctx, requester)
		s.emitFriendRequests(ctx, target)
		return
	}
	// Fresh pending: outgoing view on the requester, incoming + 612
	// delivery on the target.
	s.emitFriendRequests(ctx, requester)
	s.emitFriendRequests(ctx, target)
	name, expires := s.incomingView(ctx, target, requester)
	_ = s.deliver(ctx, target, msgIDFriendRequestPush, &protocolv1.S2CFriendRequest{
		RequesterCharacterId: requester[:],
		DisplayName:          name,
		ExpiresAtMs:          expires,
	})
}

// incomingView reads the fresh PENDING row (target side) for the 612.
func (s *Service) incomingView(ctx context.Context, target, requester id.UUID) (string, int64) {
	reqs, names, err := s.store.ListIncoming(ctx, nil, target, s.now())
	if err != nil {
		return "", 0
	}
	for _, p := range reqs {
		if p.Requester == requester {
			return names[requester], p.ExpiresAt.UnixMilli()
		}
	}
	return "", 0
}

// emitFriendUpsert pushes a 616 carrying counterpart as UPSERT plus the
// complete request lists.
func (s *Service) emitFriendUpsert(ctx context.Context, me, counterpart id.UUID) {
	s.emitFriendEntry(ctx, me, counterpart,
		protocolv1.FriendChange_FRIEND_CHANGE_UPSERT)
}

// emitFriendRemoved pushes a 616 carrying counterpart as REMOVED plus
// the complete request lists.
func (s *Service) emitFriendRemoved(ctx context.Context, me, counterpart id.UUID) {
	s.emitFriendEntry(ctx, me, counterpart,
		protocolv1.FriendChange_FRIEND_CHANGE_REMOVED)
}

// emitFriendEntry emits the delta 616 for one counterpart change.
func (s *Service) emitFriendEntry(ctx context.Context, me, counterpart id.UUID,
	change protocolv1.FriendChange) {
	name := ""
	if info, ok, err := s.store.Character(ctx, nil, counterpart); err == nil && ok {
		name = info.DisplayName
	}
	pres := s.presence(counterpart)
	entry := &protocolv1.FriendEntry{
		FriendCharacterId: counterpart[:],
		DisplayName:       name,
		Change:            change,
	}
	if pres.Online {
		entry.OnlineState = protocolv1.OnlineState_ONLINE_STATE_ONLINE
		entry.ZoneId = pres.ZoneID
		entry.Activity = pres.Activity
	} else {
		entry.OnlineState = protocolv1.OnlineState_ONLINE_STATE_OFFLINE
	}
	s.emitFriendState(ctx, me, []*protocolv1.FriendEntry{entry})
}

// emitFriendRequests pushes a 616 carrying only the complete request
// lists (no friend-entry changes).
func (s *Service) emitFriendRequests(ctx context.Context, me id.UUID) {
	s.emitFriendState(ctx, me, nil)
}

// emitFriendState emits a 616 (AUTHORITATIVE_EVENT): full_snapshot per
// flag, the given entries, and the complete live request lists.
func (s *Service) emitFriendState(ctx context.Context, me id.UUID,
	entries []*protocolv1.FriendEntry) {
	msg := &protocolv1.S2CFriendState{Entries: entries}
	if inc, names, err := s.store.ListIncoming(ctx, nil, me, s.now()); err == nil {
		for _, p := range inc {
			msg.IncomingRequests = append(msg.IncomingRequests,
				&protocolv1.FriendRequestView{
					RequesterCharacterId: p.Requester[:],
					DisplayName:          names[p.Requester],
					ExpiresAtMs:          p.ExpiresAt.UnixMilli(),
				})
		}
	}
	if out, names, err := s.store.ListOutgoing(ctx, nil, me, s.now()); err == nil {
		for _, p := range out {
			msg.OutgoingRequests = append(msg.OutgoingRequests,
				&protocolv1.FriendOutgoingView{
					TargetCharacterId: p.Target[:],
					DisplayName:       names[p.Target],
					ExpiresAtMs:       p.ExpiresAt.UnixMilli(),
				})
		}
	}
	_ = s.deliver(ctx, me, msgIDFriendState, msg)
}

// emitBlockState pushes the 619 REPLACEABLE_STATE full snapshot.
func (s *Service) emitBlockState(ctx context.Context, me id.UUID) {
	msg := &protocolv1.S2CBlockState{}
	if blocks, err := s.store.ListBlocks(ctx, nil, me); err == nil {
		for _, b := range blocks {
			msg.Blocked = append(msg.Blocked, &protocolv1.BlockEntry{
				BlockedCharacterId: b.BlockedID[:],
				DisplayName:        b.DisplayName,
				BlockedAtMs:        b.CreatedAt.UnixMilli(),
			})
		}
	}
	_ = s.deliver(ctx, me, msgIDBlockState, msg)
}

// EmitSocialState is the composition surface for the attach-time
// pushes: the full 616 snapshot plus the full 619 block list
// (messages.md § Friends & Blocks contract).
func (s *Service) EmitSocialState(ctx context.Context, me id.UUID) error {
	friends, err := s.store.ListFriends(ctx, nil, me)
	if err != nil {
		return err
	}
	msg := &protocolv1.S2CFriendState{FullSnapshot: true}
	for _, f := range friends {
		entry := &protocolv1.FriendEntry{
			FriendCharacterId: f.FriendID[:],
			DisplayName:       f.DisplayName,
			Change:            protocolv1.FriendChange_FRIEND_CHANGE_UPSERT,
		}
		pres := s.presence(f.FriendID)
		if pres.Online {
			entry.OnlineState = protocolv1.OnlineState_ONLINE_STATE_ONLINE
			entry.ZoneId = pres.ZoneID
			entry.Activity = pres.Activity
		} else {
			entry.OnlineState = protocolv1.OnlineState_ONLINE_STATE_OFFLINE
		}
		msg.Entries = append(msg.Entries, entry)
	}
	if inc, names, err := s.store.ListIncoming(ctx, nil, me, s.now()); err == nil {
		for _, p := range inc {
			msg.IncomingRequests = append(msg.IncomingRequests,
				&protocolv1.FriendRequestView{
					RequesterCharacterId: p.Requester[:],
					DisplayName:          names[p.Requester],
					ExpiresAtMs:          p.ExpiresAt.UnixMilli(),
				})
		}
	}
	if out, names, err := s.store.ListOutgoing(ctx, nil, me, s.now()); err == nil {
		for _, p := range out {
			msg.OutgoingRequests = append(msg.OutgoingRequests,
				&protocolv1.FriendOutgoingView{
					TargetCharacterId: p.Target[:],
					DisplayName:       names[p.Target],
					ExpiresAtMs:       p.ExpiresAt.UnixMilli(),
				})
		}
	}
	if err := s.deliver(ctx, me, msgIDFriendState, msg); err != nil {
		return err
	}
	s.emitBlockState(ctx, me)
	return nil
}

// mustID copies a 16-byte id (already shape-validated by the executor).
func mustID(b []byte) id.UUID {
	var u id.UUID
	copy(u[:], b)
	return u
}
