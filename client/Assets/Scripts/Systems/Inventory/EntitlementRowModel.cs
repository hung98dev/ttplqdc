using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Inventory
{
    /// <summary>One row of the 435 entitlement panel projection.</summary>
    public sealed class EntitlementRowModel
    {
        public byte[] EntitlementId = new byte[16];
        public string ProductId = "";
        public EntitlementType Type;
        public EntitlementGrantState GrantState;
        public uint SeasonNumber;
        public long ClaimDeadlineAtUnix;
        public readonly List<string> ClaimableTierIds = new List<string>();
        public readonly List<string> ClaimedTierIds = new List<string>();

        /// <summary>
        /// A cosmetic/access entitlement the panel may equip or claim
        /// from: refund-revoked or pending rows (PENDING / REJECTED /
        /// REFUNDED / REFUNDED_CONSUMED) are never equippable
        /// (monetization.md).
        /// </summary>
        public bool PanelEquippable
        {
            get
            {
                return GrantState == EntitlementGrantState.Granted;
            }
        }
    }
}
