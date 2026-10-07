using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Inventory
{
    /// <summary>
    /// Client-side entitlement panel projection (435 REPLACEABLE_STATE +
    /// 419 claim-result overlay): rows swap wholesale on 435; a claim
    /// result only folds its tier onto the matching row's claimed set.
    /// Claim execution itself is the server's (IMP-102) — this is the
    /// display projection.
    /// </summary>
    public sealed class EntitlementPanelState
    {
        private readonly List<EntitlementRowModel> _rows =
            new List<EntitlementRowModel>();
        private readonly Dictionary<string, EntitlementRowModel> _byId =
            new Dictionary<string, EntitlementRowModel>();

        public IReadOnlyList<EntitlementRowModel> Rows
        {
            get
            {
                return _rows;
            }
        }

        public EntitlementRowModel? RowFor(byte[] entitlementId)
        {
            return _byId.TryGetValue(Key(entitlementId),
                out EntitlementRowModel r)
                ? r
                : null;
        }

        /// <summary>435 replace: entitlement rows swap wholesale.</summary>
        internal void Replace(S2CEntitlementPanelState s)
        {
            _rows.Clear();
            _byId.Clear();
            foreach (EntitlementView v in s.Entitlements)
            {
                var m = new EntitlementRowModel
                {
                    EntitlementId = v.EntitlementId.ToByteArray(),
                    ProductId = v.ProductId,
                    Type = v.EntitlementType,
                    GrantState = v.GrantState,
                    SeasonNumber = v.SeasonNumber,
                    ClaimDeadlineAtUnix = v.ClaimDeadlineAt,
                };
                m.ClaimableTierIds.AddRange(v.ClaimableTierIds);
                m.ClaimedTierIds.AddRange(v.ClaimedTierIds);
                _rows.Add(m);
                _byId[Key(m.EntitlementId)] = m;
            }
        }

        /// <summary>
        /// 419 overlay: a successful claim moves reward_tier_id from the
        /// claimable set into the claimed set on its row.
        /// </summary>
        internal void ApplyClaimResult(S2CEntitlementClaimResult r)
        {
            if (r.Result == null ||
                r.Result.Status != ResultStatus.Success)
            {
                return;
            }
            string tier = r.RewardTierId;
            if (r.EntitlementId.Length > 0 &&
                _byId.TryGetValue(Key(r.EntitlementId.ToByteArray()),
                    out EntitlementRowModel? hit))
            {
                if (hit.ClaimableTierIds.Remove(tier))
                {
                    hit.ClaimedTierIds.Add(tier);
                }
                return;
            }
            for (int i = 0; i < _rows.Count; i++)
            {
                EntitlementRowModel row = _rows[i];
                if (row.ClaimableTierIds.Remove(tier))
                {
                    row.ClaimedTierIds.Add(tier);
                    return;
                }
            }
        }

        private static string Key(byte[] id)
        {
            var chars = new char[id.Length * 2];
            for (int i = 0; i < id.Length; i++)
            {
                chars[i * 2] = Nibble(id[i] >> 4);
                chars[i * 2 + 1] = Nibble(id[i] & 0xF);
            }
            return new string(chars);
        }

        private static char Nibble(int n)
        {
            return (char)(n < 10 ? '0' + n : 'a' + (n - 10));
        }
    }
}
