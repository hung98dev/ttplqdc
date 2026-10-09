package social

import (
	"context"
	"errors"

	"thinhthan/internal/core/id"
	sociald "thinhthan/internal/durable/social"
	"thinhthan/internal/edge/router"
	protocolv1 "thinhthan/internal/protocol/v1"
)

// chatSend is the non-durable handler for C2S_CHAT_SEND (600): validate
// → channel gates → rate limit → mint the message id → fanout 601 →
// always exactly one 655 result (messages.md § Chat).
func (s *Service) chatSend(ctx context.Context, v router.View, r router.Route) error {
	if v.CharacterID == nil {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_MESSAGE_NOT_ALLOWED_IN_STATE}
	}
	req, ok := r.Inbound.Payload.(*protocolv1.C2SChatSend)
	if !ok {
		return &router.RejectError{Code: protocolv1.ErrorCode_ERROR_CODE_PROTOCOL_MALFORMED}
	}
	sender := *v.CharacterID
	fail := func(code protocolv1.ErrorCode) error {
		return s.send(v.Conn, msgIDChatSendResult, &protocolv1.S2CChatSendResult{
			Result: &protocolv1.OperationResult{
				OperationId: req.GetOperationId(),
				Status:      protocolv1.ResultStatus_RESULT_STATUS_ERROR,
				ErrorCode:   code,
			},
		})
	}

	text, ok := sociald.CanonicalText(req.GetMessageText(), maxChatGraphemes)
	if !ok {
		return fail(protocolv1.ErrorCode_ERROR_CODE_CHAT_TEXT_INVALID)
	}
	channel := req.GetChannel()
	if channel == protocolv1.ChatChannel_CHAT_CHANNEL_UNSPECIFIED {
		return fail(protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}

	senderInfo, ok, err := s.store.Character(ctx, nil, sender)
	if err != nil {
		return err
	}
	if !ok {
		return fail(protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
	}

	target := mustID(req.GetTargetCharacterId())
	switch channel {
	case protocolv1.ChatChannel_CHAT_CHANNEL_WORLD:
		if senderInfo.Level < 10 {
			return fail(protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED)
		}
	case protocolv1.ChatChannel_CHAT_CHANNEL_WHISPER:
		if len(req.GetTargetCharacterId()) != 16 || target == sender {
			return fail(protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
		}
		tinfo, ok, err := s.store.Character(ctx, nil, target)
		if err != nil {
			return err
		}
		if !ok || !tinfo.SessionActive {
			return fail(protocolv1.ErrorCode_ERROR_CODE_TARGET_INVALID)
		}
		blocked, err := s.store.BlockedEither(ctx, nil, sender, target)
		if err != nil {
			return err
		}
		if blocked {
			return fail(protocolv1.ErrorCode_ERROR_CODE_TARGET_BLOCKED)
		}
	}

	if s.moderation != nil {
		if code, ok := s.moderation.CheckSend(ctx, sender, channel, target, text); !ok {
			return fail(code)
		}
	}

	if !s.limiter.allowChat(channel, sender, target, s.now()) {
		return fail(protocolv1.ErrorCode_ERROR_CODE_RATE_LIMITED)
	}

	recipients, err := s.fanout(ctx, channel, sender, target)
	if err != nil {
		if errors.Is(err, ErrNotMember) {
			return fail(protocolv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED)
		}
		return err
	}
	msgID := id.NewV4()
	now := s.now()
	for _, rcpt := range dedupe(recipients, sender) {
		if err := s.deliver(ctx, rcpt, msgIDChatMessage, &protocolv1.S2CChatMessage{
			ChatMessageId:     msgID[:],
			Channel:           channel,
			SenderCharacterId: sender[:],
			SenderName:        senderInfo.DisplayName,
			MessageText:       text,
			SentAtMs:          now.UnixMilli(),
		}); err != nil {
			return err
		}
	}
	return s.send(v.Conn, msgIDChatSendResult, &protocolv1.S2CChatSendResult{
		Result: &protocolv1.OperationResult{
			OperationId: req.GetOperationId(),
			Status:      protocolv1.ResultStatus_RESULT_STATUS_SUCCESS,
		},
		ChatMessageId: msgID[:],
	})
}

// fanout resolves the recipient set through the hub; a nil hub has no
// channel rosters, so only WHISPER (sender+target) still works.
func (s *Service) fanout(ctx context.Context,
	channel protocolv1.ChatChannel, sender, target id.UUID) ([]id.UUID, error) {
	if s.hub == nil {
		if channel == protocolv1.ChatChannel_CHAT_CHANNEL_WHISPER {
			return []id.UUID{sender, target}, nil
		}
		return nil, ErrNotMember
	}
	return s.hub.Fanout(ctx, channel, sender, target)
}

// dedupe removes duplicate recipients and appends the sender when the
// hub did not already include them.
func dedupe(recipients []id.UUID, sender id.UUID) []id.UUID {
	seen := map[id.UUID]bool{}
	out := make([]id.UUID, 0, len(recipients)+1)
	for _, r := range recipients {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	if !seen[sender] {
		out = append(out, sender)
	}
	return out
}
