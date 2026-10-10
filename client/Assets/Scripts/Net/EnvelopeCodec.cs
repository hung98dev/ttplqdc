using System;
using System.Collections.Generic;
using Google.Protobuf;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Net
{
    /// <summary>
    /// Envelope codec: one WebSocket binary message = one Envelope; payload is
    /// the serialized inner message (protocol.md § Framing). The caller
    /// supplies the outbound buffer so framing itself never allocates.
    /// </summary>
    public static class EnvelopeCodec
    {
        private static readonly Dictionary<uint, MessageParser> _s2cParsers =
            BuildParsers();

        /// <summary>
        /// Serializes <paramref name="payload"/> inside an
        /// <see cref="Envelope"/> and writes it into
        /// <paramref name="destination"/>. Returns the byte count written.
        /// </summary>
        public static int Encode(
            uint messageId,
            ulong sessionEpoch,
            ulong clientSeq,
            IMessage payload,
            Span<byte> destination,
            ulong correlationId = 0UL)
        {
            var envelope = new Envelope
            {
                ProtocolMajor = WireIds.ProtocolMajor,
                ProtocolMinor = WireIds.ProtocolMinor,
                MessageId = messageId,
                SessionEpoch = sessionEpoch,
                ClientSeq = clientSeq,
                ServerSeq = 0UL,
                CorrelationId = correlationId,
                Payload = payload.ToByteString(),
            };
            int size = envelope.CalculateSize();
            if (size > destination.Length)
            {
                throw new InvalidOperationException(
                    "Outbound frame exceeds caller buffer");
            }

            envelope.WriteTo(destination.Slice(0, size));
            return size;
        }

        /// <summary>
        /// Parses one inbound frame into <paramref name="frame"/>. Throws
        /// <see cref="InvalidProtocolBufferException"/> on malformed bytes;
        /// callers enforce the 64 KiB inbound bound before calling.
        /// </summary>
        public static void Decode(ReadOnlySpan<byte> bytes, DecodedFrame frame)
        {
            Envelope envelope = Envelope.Parser.ParseFrom(bytes);
            frame.Envelope = envelope;
            frame.SessionEpoch = envelope.SessionEpoch;
            frame.ServerSeq = envelope.ServerSeq;
            frame.CorrelationId = envelope.CorrelationId;
            frame.MessageId = envelope.MessageId;
            if (_s2cParsers.TryGetValue(envelope.MessageId, out MessageParser? parser))
            {
                frame.Payload = parser.ParseFrom(envelope.Payload);
            }
            else
            {
                frame.Payload = null;
            }

            frame.BaselineId = frame.Payload switch
            {
                S2CWorldBaseline baseline => baseline.BaselineId,
                S2CEntitySpawn spawn => spawn.BaselineId,
                S2CEntityDespawn despawn => despawn.BaselineId,
                S2CStateDelta delta => delta.BaselineId,
                _ => 0UL,
            };
            frame.AccountedBytes = bytes.Length +
                (frame.Payload != null ? frame.Payload.CalculateSize() : 0);
        }

        private static Dictionary<uint, MessageParser> BuildParsers()
        {
            return new Dictionary<uint, MessageParser>
            {
                [WireIds.S2CHelloOk] = S2CHelloOk.Parser,
                [WireIds.S2CError] = S2CError.Parser,
                [WireIds.S2CHeartbeat] = S2CHeartbeat.Parser,
                [WireIds.S2CCharacterAttachOk] = S2CCharacterAttachOk.Parser,
                [WireIds.S2CSessionReplaced] = S2CSessionReplaced.Parser,
                [WireIds.S2CServerDraining] = S2CServerDraining.Parser,
                [WireIds.S2CCharacterDetachOk] = S2CCharacterDetachOk.Parser,
                [WireIds.S2CCharacterCreateResult] = S2CCharacterCreateResult.Parser,
                [WireIds.S2CCharacterList] = S2CCharacterList.Parser,
                [WireIds.S2CPlacementPending] = S2CPlacementPending.Parser,
                [WireIds.S2CResumeCredential] = S2CResumeCredential.Parser,
                [WireIds.S2CTransferPrepare] = S2CTransferPrepare.Parser,
                [WireIds.S2CMovementCorrection] = S2CMovementCorrection.Parser,
                [WireIds.S2CChannelSwitchResult] = S2CChannelSwitchResult.Parser,
                [WireIds.S2CInteractResult] = S2CInteractResult.Parser,
                [WireIds.S2CActionStarted] = S2CActionStarted.Parser,
                [WireIds.S2CActionRejected] = S2CActionRejected.Parser,
                [WireIds.S2CStatusEvent] = S2CStatusEvent.Parser,
                [WireIds.S2CDeath] = S2CDeath.Parser,
                [WireIds.S2CRespawn] = S2CRespawn.Parser,
                [WireIds.S2CWorldBaseline] = S2CWorldBaseline.Parser,
                [WireIds.S2CEntitySpawn] = S2CEntitySpawn.Parser,
                [WireIds.S2CEntityDespawn] = S2CEntityDespawn.Parser,
                [WireIds.S2CStateDelta] = S2CStateDelta.Parser,
                [WireIds.S2CCombatEvent] = S2CCombatEvent.Parser,
                [WireIds.S2CBaselineResyncResult] = S2CBaselineResyncResult.Parser,
                [WireIds.S2CInventoryResult] = S2CInventoryResult.Parser,
                [WireIds.S2CLoadoutResult] = S2CLoadoutResult.Parser,
                [WireIds.S2CCraftResult] = S2CCraftResult.Parser,
                [WireIds.S2CEnhanceResult] = S2CEnhanceResult.Parser,
                [WireIds.S2CRewardClaimResult] = S2CRewardClaimResult.Parser,
                [WireIds.S2CEntitlementClaimResult] = S2CEntitlementClaimResult.Parser,
                [WireIds.S2CInventoryExpandResult] = S2CInventoryExpandResult.Parser,
                [WireIds.S2CWalletState] = S2CWalletState.Parser,
                [WireIds.S2CInventoryState] = S2CInventoryState.Parser,
                [WireIds.S2CRewardClaimsState] = S2CRewardClaimsState.Parser,
                [WireIds.S2CEntitlementPanelState] = S2CEntitlementPanelState.Parser,
                [WireIds.S2CRewardClaimListResult] = S2CRewardClaimListResult.Parser,
                [WireIds.S2CRewardClaimDelta] = S2CRewardClaimDelta.Parser,
                [WireIds.S2CProgressionMutateResult] = S2CProgressionMutateResult.Parser,
                [WireIds.S2CProgressionState] = S2CProgressionState.Parser,
                [WireIds.S2CChatMessage] = S2CChatMessage.Parser,
                [WireIds.S2CPartyInvite] = S2CPartyInvite.Parser,
                [WireIds.S2CPartyState] = S2CPartyState.Parser,
                [WireIds.S2CFriendRequest] = S2CFriendRequest.Parser,
                [WireIds.S2CFriendState] = S2CFriendState.Parser,
                [WireIds.S2CBlockState] = S2CBlockState.Parser,
                [WireIds.S2CReportPlayerResult] = S2CReportPlayerResult.Parser,
                [WireIds.S2CPartyBoardState] = S2CPartyBoardState.Parser,
                [WireIds.S2CPartyResult] = S2CPartyResult.Parser,
                [WireIds.S2CSocialResult] = S2CSocialResult.Parser,
                [WireIds.S2CChatSendResult] = S2CChatSendResult.Parser,
                [WireIds.S2CTradeInvite] = S2CTradeInvite.Parser,
                [WireIds.S2CTradeCancelled] = S2CTradeCancelled.Parser,
                [WireIds.S2CTradeOfferState] = S2CTradeOfferState.Parser,
                [WireIds.S2CTradeResult] = S2CTradeResult.Parser,
                [WireIds.S2CTradeRequestResult] = S2CTradeRequestResult.Parser,
                [WireIds.S2CAuctionListResult] = S2CAuctionListResult.Parser,
                [WireIds.S2CAuctionBuyResult] = S2CAuctionBuyResult.Parser,
                [WireIds.S2CAuctionCancelResult] = S2CAuctionCancelResult.Parser,
                [WireIds.S2CAuctionSold] = S2CAuctionSold.Parser,
                [WireIds.S2CAuctionSearchResult] = S2CAuctionSearchResult.Parser,
                [WireIds.S2CAuctionReclaimResult] = S2CAuctionReclaimResult.Parser,
                [WireIds.S2CAuctionProceedsResult] = S2CAuctionProceedsResult.Parser,
                [WireIds.S2CAuctionMyState] = S2CAuctionMyState.Parser,
            };
        }
    }
}
