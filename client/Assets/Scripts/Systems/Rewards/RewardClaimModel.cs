using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Rewards
{
    /// <summary>Flattened pending-claim view for the Rewards UI.</summary>
    public sealed class RewardClaimModel
    {
        public byte[] RewardClaimId = new byte[0];
        public string SourceType = "";
        public string SourceReference = "";
        public string RewardSlot = "";
        public RewardClaimState State = RewardClaimState.Unspecified;
        public int LineCount;

        /// <summary>Item/currency summary: first line's id + qty.</summary>
        public string FirstLineLabel = "";

        public static RewardClaimModel FromView(RewardClaimView v)
        {
            var m = new RewardClaimModel
            {
                RewardClaimId = v.RewardClaimId != null
                    ? v.RewardClaimId.ToByteArray() : new byte[0],
                SourceType = v.SourceType ?? "",
                SourceReference = v.SourceReference ?? "",
                RewardSlot = v.RewardSlot ?? "",
                State = v.State,
                LineCount = v.Lines.Count,
            };
            if (v.Lines.Count > 0)
            {
                RewardClaimLine l = v.Lines[0];
                string id = l.HasItemId ? l.ItemId :
                    (l.HasCurrencyId ? l.CurrencyId : "");
                m.FirstLineLabel = id + " x" + l.Quantity;
                if (v.Lines.Count > 1)
                {
                    m.FirstLineLabel += " (+" + (v.Lines.Count - 1) + ")";
                }
            }
            return m;
        }
    }
}
