using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Rewards
{
    /// <summary>One pending-claim row: source/slot/state + Claim.</summary>
    public sealed class RewardClaimRowView : MonoBehaviour
    {
        public TMP_Text? SourceText;
        public TMP_Text? SlotText;
        public TMP_Text? StateText;
        public Button? ClaimButton;

        private byte[] _claimId = new byte[0];

        /// <summary>The claim id this row shows (test introspection).</summary>
        public byte[] ClaimId
        {
            get
            {
                return _claimId;
            }
        }

        public void Apply(ThinhThan.Systems.Rewards.RewardClaimModel m)
        {
            _claimId = m.RewardClaimId;
            if (SourceText != null)
            {
                SourceText.SetText(m.SourceType);
            }
            if (SlotText != null)
            {
                SlotText.SetText(m.FirstLineLabel);
            }
            if (StateText != null)
            {
                StateText.SetText(m.State.ToString());
            }
        }
    }
}
