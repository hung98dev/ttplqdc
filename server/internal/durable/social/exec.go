package social

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	"thinhthan/internal/core/id"
	"thinhthan/internal/durable/idempotency"
	journalv1 "thinhthan/internal/durable/journal/v1"
	"thinhthan/internal/durable/lockorder"
	"thinhthan/internal/durable/queue"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// maxReportNotesGraphemes mirrors social.md § Reports (<=200).
const maxReportNotesGraphemes = 200

// Executors returns the queue.Executor surface of this package: one
// executor per client.<ID> social family, exported only — composition
// installs them on the ProducerClient family-mux; the package never
// self-registers (ADR-0081). Records run inside Store.TrustedReplay
// under the receipt lock, so every verdict/commit below is a committed
// JournalOutcome: a retried operation_id replays the retained outcome
// with no re-execution.
func Executors(s *Store) map[string]queue.Executor {
	return map[string]queue.Executor{
		FriendRequestFamily: s.friendRequest,
		FriendAcceptFamily:  s.friendAccept,
		FriendDeclineFamily: s.friendDecline,
		FriendRemoveFamily:  s.friendRemove,
		BlockAddFamily:      s.blockAdd,
		BlockRemoveFamily:   s.blockRemove,
		ReportFamily:        s.report,
	}
}

// identity validates the record's character-owned identity fields.
func identity(rec *journalv1.DurableCommandRecord) (opID, characterID id.UUID, err error) {
	if len(rec.GetOwnerId()) != 16 || len(rec.GetOperationId()) != 16 {
		return opID, characterID, ErrMalformedRecord
	}
	copy(opID[:], rec.GetOperationId())
	copy(characterID[:], rec.GetOwnerId())
	return opID, characterID, nil
}

// cmdIdentity cross-checks the JournalClientCommand identity against the
// record owner: a mismatched account/character is a malformed record.
func cmdIdentity(cmd *journalv1.JournalClientCommand, characterID id.UUID) error {
	if cmd == nil {
		return ErrMalformedRecord
	}
	if len(cmd.GetCharacterId()) != 16 {
		return ErrMalformedRecord
	}
	var cid id.UUID
	copy(cid[:], cmd.GetCharacterId())
	if cid != characterID {
		return fmt.Errorf("%w: owner %v vs character %v", ErrMalformedRecord, characterID, cid)
	}
	return nil
}

// targetID extracts the 16-byte target/requester id carried by every
// social request variant.
func targetID(b []byte) (id.UUID, error) {
	var t id.UUID
	if len(b) != 16 {
		return t, fmt.Errorf("%w: target %d bytes", ErrMalformedRecord, len(b))
	}
	copy(t[:], b)
	return t, nil
}

// lockPair acquires both characters' row locks in canonical UUID order
// (data_model.md § Social: the social pair transactions lock both
// characters rows UUID-order at characters' priority).
func lockPair(ctx context.Context, tx pgx.Tx, a, b id.UUID) error {
	lo, hi := lowHigh(a, b)
	return lockorder.Acquire(ctx, tx,
		lockorder.RowLock("characters", lo),
		lockorder.RowLock("characters", hi))
}

// admittedAt returns the record's admission timestamp (frozen at the
// edge) as the transaction clock.
func admittedAt(rec *journalv1.DurableCommandRecord) time.Time {
	return time.UnixMilli(rec.GetEnqueuedAtMs()).UTC()
}

// friendRequest applies C2S_FRIEND_REQUEST (611). Check order: target
// valid/existing → block gate → already friends → live pending pair
// (crossed accepts, same-direction is PENDING_REQUEST_EXISTS) → caps.
func (s *Store) friendRequest(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FriendRequestFamily {
		return idempotency.Outcome{}, fmt.Errorf("%w: family %q", ErrMalformedRecord, rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	opID, characterID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if err := cmdIdentity(cmd, characterID); err != nil {
		return idempotency.Outcome{}, err
	}
	req := cmd.GetC2SFriendRequest()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	target, err := targetID(req.GetTargetCharacterId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fam := rec.GetOperationFamily()
	now := admittedAt(rec)
	if target == characterID {
		return s.socialVerdict(opID, fam, target,
			protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	if err := lockPair(ctx, tx, characterID, target); err != nil {
		return idempotency.Outcome{}, err
	}
	if _, ok, err := s.Character(ctx, tx, target); err != nil {
		return idempotency.Outcome{}, err
	} else if !ok {
		return s.socialVerdict(opID, fam, target,
			protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	blocked, err := s.BlockedEither(ctx, tx, characterID, target)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if blocked {
		return s.socialVerdict(opID, fam, target,
			protocolv1.ErrorCode_ERROR_CODE_TARGET_BLOCKED)
	}
	friends, err := s.AreFriends(ctx, tx, characterID, target)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if friends {
		return s.socialVerdict(opID, fam, target,
			protocolv1.ErrorCode_ERROR_CODE_ALREADY_FRIENDS)
	}
	pending, ok, err := s.PendingBetween(ctx, tx, characterID, target, now)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if ok {
		if pending.Requester == characterID {
			return s.socialVerdict(opID, fam, target,
				protocolv1.ErrorCode_ERROR_CODE_PENDING_REQUEST_EXISTS)
		}
		// Crossed request: resolve the incoming PENDING as ACCEPTED and
		// create the friendship under both caps (social.md § Requests).
		return s.acceptPair(ctx, tx, opID, fam, characterID, target,
			pending, now)
	}
	out, err := s.CountOutgoing(ctx, tx, characterID, now)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if out >= OutgoingCap {
		return s.socialVerdict(opID, fam, target,
			protocolv1.ErrorCode_ERROR_CODE_CAPACITY_FULL)
	}
	if err := s.InsertPending(ctx, tx, id.NewV4(), characterID, target,
		opID, now); err != nil {
		if isUniqueViolation(err) {
			return s.socialVerdict(opID, fam, target,
				protocolv1.ErrorCode_ERROR_CODE_PENDING_REQUEST_EXISTS)
		}
		return idempotency.Outcome{}, err
	}
	return s.socialCommit(opID, fam, target)
}

// acceptPair is the shared accept path for C2S_FRIEND_ACCEPT and the
// crossed-request resolution: under the pair lock it checks both sides'
// friend caps, resolves the pending row ACCEPTED and inserts the pair.
func (s *Store) acceptPair(ctx context.Context, tx pgx.Tx,
	opID id.UUID, fam string, characterID, target id.UUID,
	pending *PendingRequest, now time.Time) (idempotency.Outcome, error) {
	mine, err := s.CountFriends(ctx, tx, characterID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	theirs, err := s.CountFriends(ctx, tx, target)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if mine >= FriendLimit || theirs >= FriendLimit {
		return s.socialVerdict(opID, fam, target,
			protocolv1.ErrorCode_ERROR_CODE_FRIEND_LIMIT_REACHED)
	}
	if err := s.ResolvePending(ctx, tx, pending.RequestID, "ACCEPTED",
		opID, now); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := s.InsertFriend(ctx, tx, characterID, target, opID, now); err != nil {
		return idempotency.Outcome{}, err
	}
	return s.socialCommit(opID, fam, target)
}

// friendAccept applies C2S_FRIEND_ACCEPT (613): the caller accepts a
// live incoming PENDING request.
func (s *Store) friendAccept(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FriendAcceptFamily {
		return idempotency.Outcome{}, fmt.Errorf("%w: family %q", ErrMalformedRecord, rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	opID, characterID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if err := cmdIdentity(cmd, characterID); err != nil {
		return idempotency.Outcome{}, err
	}
	req := cmd.GetC2SFriendAccept()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	requester, err := targetID(req.GetRequesterCharacterId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fam := rec.GetOperationFamily()
	now := admittedAt(rec)
	if requester == characterID {
		return s.socialVerdict(opID, fam, requester,
			protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	if err := lockPair(ctx, tx, characterID, requester); err != nil {
		return idempotency.Outcome{}, err
	}
	pending, ok, err := s.PendingFrom(ctx, tx, requester, characterID, now)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if !ok {
		return s.socialVerdict(opID, fam, requester,
			protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	return s.acceptPair(ctx, tx, opID, fam, characterID, requester,
		pending, now)
}

// friendDecline applies C2S_FRIEND_DECLINE (614): resolve the incoming
// live PENDING as DECLINED.
func (s *Store) friendDecline(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FriendDeclineFamily {
		return idempotency.Outcome{}, fmt.Errorf("%w: family %q", ErrMalformedRecord, rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	opID, characterID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if err := cmdIdentity(cmd, characterID); err != nil {
		return idempotency.Outcome{}, err
	}
	req := cmd.GetC2SFriendDecline()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	requester, err := targetID(req.GetRequesterCharacterId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fam := rec.GetOperationFamily()
	now := admittedAt(rec)
	pending, ok, err := s.PendingFrom(ctx, tx, requester, characterID, now)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if !ok {
		return s.socialVerdict(opID, fam, requester,
			protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	if err := s.ResolvePending(ctx, tx, pending.RequestID, "DECLINED",
		opID, now); err != nil {
		return idempotency.Outcome{}, err
	}
	return s.socialCommit(opID, fam, requester)
}

// friendRemove applies C2S_FRIEND_REMOVE (615): delete the unordered
// pair.
func (s *Store) friendRemove(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != FriendRemoveFamily {
		return idempotency.Outcome{}, fmt.Errorf("%w: family %q", ErrMalformedRecord, rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	opID, characterID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if err := cmdIdentity(cmd, characterID); err != nil {
		return idempotency.Outcome{}, err
	}
	req := cmd.GetC2SFriendRemove()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	target, err := targetID(req.GetTargetCharacterId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fam := rec.GetOperationFamily()
	if target == characterID {
		return s.socialVerdict(opID, fam, target,
			protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	removed, err := s.DeleteFriend(ctx, tx, characterID, target)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if !removed {
		return s.socialVerdict(opID, fam, target,
			protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	return s.socialCommit(opID, fam, target)
}

// blockAdd applies C2S_BLOCK_ADD (617): one transaction inserts the
// directional block, deletes any friendship and cancels every live
// pending request for the pair (data_model.md § Social).
func (s *Store) blockAdd(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != BlockAddFamily {
		return idempotency.Outcome{}, fmt.Errorf("%w: family %q", ErrMalformedRecord, rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	opID, characterID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if err := cmdIdentity(cmd, characterID); err != nil {
		return idempotency.Outcome{}, err
	}
	req := cmd.GetC2SBlockAdd()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	target, err := targetID(req.GetTargetCharacterId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fam := rec.GetOperationFamily()
	now := admittedAt(rec)
	if target == characterID {
		return s.socialVerdict(opID, fam, target,
			protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	if err := lockPair(ctx, tx, characterID, target); err != nil {
		return idempotency.Outcome{}, err
	}
	if _, ok, err := s.Character(ctx, tx, target); err != nil {
		return idempotency.Outcome{}, err
	} else if !ok {
		return s.socialVerdict(opID, fam, target,
			protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	already, err := s.IsBlocked(ctx, tx, characterID, target)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if already {
		return s.socialVerdict(opID, fam, target,
			protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	n, err := s.CountBlocks(ctx, tx, characterID)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if n >= BlockCap {
		return s.socialVerdict(opID, fam, target,
			protocolv1.ErrorCode_ERROR_CODE_CAPACITY_FULL)
	}
	if err := s.InsertBlock(ctx, tx, characterID, target, opID, now); err != nil {
		return idempotency.Outcome{}, err
	}
	if _, err := s.DeleteFriend(ctx, tx, characterID, target); err != nil {
		return idempotency.Outcome{}, err
	}
	if err := s.CancelPendingPair(ctx, tx, characterID, target,
		opID, now); err != nil {
		return idempotency.Outcome{}, err
	}
	return s.socialCommit(opID, fam, target)
}

// blockRemove applies C2S_BLOCK_REMOVE (618): delete the directional
// block row.
func (s *Store) blockRemove(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != BlockRemoveFamily {
		return idempotency.Outcome{}, fmt.Errorf("%w: family %q", ErrMalformedRecord, rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	opID, characterID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if err := cmdIdentity(cmd, characterID); err != nil {
		return idempotency.Outcome{}, err
	}
	req := cmd.GetC2SBlockRemove()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	target, err := targetID(req.GetTargetCharacterId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	fam := rec.GetOperationFamily()
	removed, err := s.DeleteBlock(ctx, tx, characterID, target)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if !removed {
		return s.socialVerdict(opID, fam, target,
			protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	return s.socialCommit(opID, fam, target)
}

// report applies C2S_REPORT_PLAYER (632): reason enum, notes <=200
// graphemes, optional chat_message_id reference, 10/24h per-account cap.
func (s *Store) report(ctx context.Context, tx pgx.Tx,
	rec *journalv1.DurableCommandRecord) (idempotency.Outcome, error) {
	if rec.GetOperationFamily() != ReportFamily {
		return idempotency.Outcome{}, fmt.Errorf("%w: family %q", ErrMalformedRecord, rec.GetOperationFamily())
	}
	cmd := rec.GetClient()
	opID, characterID, err := identity(rec)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if err := cmdIdentity(cmd, characterID); err != nil {
		return idempotency.Outcome{}, err
	}
	req := cmd.GetC2SReportPlayer()
	if req == nil {
		return idempotency.Outcome{}, ErrMalformedRecord
	}
	target, err := targetID(req.GetTargetCharacterId())
	if err != nil {
		return idempotency.Outcome{}, err
	}
	var accountID id.UUID
	if len(cmd.GetAccountId()) == 16 {
		copy(accountID[:], cmd.GetAccountId())
	}
	if accountID == (id.UUID{}) {
		return idempotency.Outcome{}, fmt.Errorf("%w: account_id", ErrMalformedRecord)
	}
	now := admittedAt(rec)
	if target == characterID {
		return s.reportVerdict(opID, nil,
			protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	if req.GetReason() == protocolv1.ReportReason_REPORT_REASON_UNSPECIFIED {
		return s.reportVerdict(opID, nil,
			protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	var notes *string
	if req.ReporterNotes != nil {
		text, ok := CanonicalText(req.GetReporterNotes(), maxReportNotesGraphemes)
		if !ok {
			return s.reportVerdict(opID, nil,
				protocolv1.ErrorCode_ERROR_CODE_CHAT_TEXT_INVALID)
		}
		notes = &text
	}
	var chatRef *id.UUID
	if len(req.GetChatMessageId()) == 16 {
		var m id.UUID
		copy(m[:], req.GetChatMessageId())
		exists, err := s.ChatMessageExists(ctx, tx, m)
		if err != nil {
			return idempotency.Outcome{}, err
		}
		if !exists {
			return s.reportVerdict(opID, nil,
				protocolv1.ErrorCode_ERROR_CODE_ITEM_NOT_FOUND)
		}
		chatRef = &m
	}
	if err := lockorder.Acquire(ctx, tx,
		lockorder.RowLock("characters", characterID)); err != nil {
		return idempotency.Outcome{}, err
	}
	if _, ok, err := s.Character(ctx, tx, target); err != nil {
		return idempotency.Outcome{}, err
	} else if !ok {
		return s.reportVerdict(opID, nil,
			protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}
	n, err := s.ReportCount24h(ctx, tx, accountID, now)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	if n >= ReportLimit {
		return s.reportVerdict(opID, nil,
			protocolv1.ErrorCode_ERROR_CODE_RATE_LIMITED)
	}
	reportID := id.NewV4()
	reason := req.GetReason().String()
	reason = reason[len("REPORT_REASON_"):]
	if err := s.InsertReport(ctx, tx, reportID, opID, accountID,
		characterID, target, reason, chatRef, notes, now); err != nil {
		return idempotency.Outcome{}, err
	}
	return s.reportCommit(opID, reportID)
}

// socialVerdict writes the error terminal outcome carrying a 654
// S2C_SOCIAL_RESULT.
func (s *Store) socialVerdict(opID id.UUID, fam string, target id.UUID,
	code protocolv1.ErrorCode) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
		ErrorCode:   code,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CSocialResult{
			S2CSocialResult: &protocolv1.S2CSocialResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
					ErrorCode:   code,
				},
				RequestMessageId:  requestMessageID[fam],
				TargetCharacterId: target[:],
			},
		},
	}
	return marshalOutcome(outcome)
}

// socialCommit writes the successful terminal outcome carrying the 654.
func (s *Store) socialCommit(opID id.UUID, fam string,
	target id.UUID) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CSocialResult{
			S2CSocialResult: &protocolv1.S2CSocialResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
				},
				RequestMessageId:  requestMessageID[fam],
				TargetCharacterId: target[:],
			},
		},
	}
	return marshalOutcome(outcome)
}

// reportVerdict writes the error terminal outcome carrying a 633.
func (s *Store) reportVerdict(opID id.UUID, _ *id.UUID,
	code protocolv1.ErrorCode) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
		ErrorCode:   code,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CReportPlayerResult{
			S2CReportPlayerResult: &protocolv1.S2CReportPlayerResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
					ErrorCode:   code,
				},
			},
		},
	}
	return marshalOutcome(outcome)
}

// reportCommit writes the successful terminal outcome carrying the 633
// with the minted report_id.
func (s *Store) reportCommit(opID, reportID id.UUID) (idempotency.Outcome, error) {
	outcome := &journalv1.JournalOutcome{
		Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		OperationId: opID[:],
		ClientResult: &journalv1.JournalOutcome_S2CReportPlayerResult{
			S2CReportPlayerResult: &protocolv1.S2CReportPlayerResult{
				Result: &protocolv1.OperationResult{
					OperationId: opID[:],
					Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
				},
				ReportId: reportID[:],
			},
		},
	}
	return marshalOutcome(outcome)
}

// marshalOutcome serializes the schema-v1 JournalOutcome as protojson —
// the retained representation AwaitClientOutcome decodes.
func marshalOutcome(outcome *journalv1.JournalOutcome) (idempotency.Outcome, error) {
	b, err := protojson.Marshal(outcome)
	if err != nil {
		return idempotency.Outcome{}, err
	}
	return idempotency.Outcome{SchemaVersion: 1, Payload: b}, nil
}
