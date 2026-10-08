using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Trade
{
    /// <summary>
    /// Client-side projection of one direct-trade session: the 706
    /// offer state (wholesale replace) plus invite/cancel/result
    /// events. All session messages are scoped by TradeId; the first
    /// observed id latches the session.
    /// </summary>
    public sealed class TradeState
    {
        /// <summary>Own character id for own/partner side resolution.</summary>
        public byte[] LocalCharacterId = new byte[0];

        public byte[] TradeId = new byte[0];
        public ulong Revision;
        public TradePhase Phase = TradePhase.None;
        public long FeePreview;
        public byte[] InviterCharacterId = new byte[0];
        public string InviterName = "";
        public uint InviteExpiresInSeconds;
        public TradeSideModel OwnSide;
        public TradeSideModel PartnerSide;
        public S2CTradeResult? LastResult;
        public S2CTradeRequestResult? LastRequestResult;
        public TradeCancelReason CancelReason =
            TradeCancelReason.Unspecified;
        public ErrorCode CancelErrorCode = ErrorCode.Unspecified;

        /// <summary>Track a received invite (701) while idle.</summary>
        public void ApplyInvite(S2CTradeInvite invite)
        {
            if (invite == null || Phase != TradePhase.None)
            {
                return;
            }
            TradeId = invite.TradeId.ToByteArray();
            InviterCharacterId = invite.InviterCharacterId.ToByteArray();
            InviterName = invite.InviterName ?? "";
            InviteExpiresInSeconds = invite.ExpiresInSeconds;
            Phase = TradePhase.Invited;
        }

        /// <summary>Replace the offer projection (706).</summary>
        public void ReplaceOfferState(S2CTradeOfferState state)
        {
            if (state == null)
            {
                return;
            }
            TradeId = state.TradeId.ToByteArray();
            Revision = state.Revision;
            FeePreview = state.FeePreview;
            if (state.Sides.Count == 2)
            {
                TradeSideModel a = ToModel(state.Sides[0]);
                TradeSideModel b = ToModel(state.Sides[1]);
                if (IsLocal(b.CharacterId))
                {
                    OwnSide = b;
                    PartnerSide = a;
                }
                else
                {
                    OwnSide = a;
                    PartnerSide = b;
                }
            }
            switch (state.State)
            {
                case TradeOfferState.Open:
                    Phase = TradePhase.Open;
                    break;
                case TradeOfferState.Locked:
                    Phase = TradePhase.Locked;
                    break;
                case TradeOfferState.Committing:
                    Phase = TradePhase.Committing;
                    break;
                default:
                    break;
            }
        }

        /// <summary>Cancel/timeout/disconnect (704) — session closed.</summary>
        public void ApplyCancelled(S2CTradeCancelled cancelled)
        {
            if (cancelled == null)
            {
                return;
            }
            CancelReason = cancelled.Reason;
            CancelErrorCode = cancelled.ErrorCode;
            Phase = TradePhase.Cancelled;
        }

        /// <summary>Final settlement (709) — session completed.</summary>
        public void ApplyResult(S2CTradeResult result)
        {
            if (result == null)
            {
                return;
            }
            LastResult = result;
            Phase = TradePhase.Completed;
        }

        private bool IsLocal(byte[] characterId)
        {
            if (characterId == null ||
                characterId.Length != LocalCharacterId.Length)
            {
                return false;
            }
            for (int i = 0; i < characterId.Length; i++)
            {
                if (characterId[i] != LocalCharacterId[i])
                {
                    return false;
                }
            }
            return characterId.Length > 0;
        }

        private static TradeSideModel ToModel(TradeSide side)
        {
            var items = new List<TradeOfferItemModel>();
            foreach (ItemInstanceView v in side.Items)
            {
                items.Add(new TradeOfferItemModel
                {
                    ItemInstanceId = v.ItemInstanceId.ToByteArray(),
                    ItemId = v.ItemId ?? "",
                    Quantity = v.Quantity,
                });
            }
            return new TradeSideModel
            {
                CharacterId = side.CharacterId.ToByteArray(),
                Items = items,
                CommonAmount = side.CommonAmount,
                Confirmed = side.Confirmed,
            };
        }
    }
}
