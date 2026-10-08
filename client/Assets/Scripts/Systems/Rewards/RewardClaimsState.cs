using System.Collections.Generic;
using Google.Protobuf;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Rewards
{
    /// <summary>
    /// Authoritative pending-claims mirror (434 snapshot + 441 delta +
    /// 440 paging merge). Order is the server's PENDING order — oldest
    /// first (ADR-0064); the panel renders it verbatim. Bounded by
    /// `cap` (100).
    /// </summary>
    public sealed class RewardClaimsState
    {
        private readonly List<RewardClaimModel> _claims =
            new List<RewardClaimModel>();

        public ulong ClaimsRevision
        {
            get;
            private set;
        }

        public uint TotalCount
        {
            get;
            private set;
        }

        public uint Cap
        {
            get;
            private set;
        }

        public IReadOnlyList<RewardClaimModel> Claims
        {
            get
            {
                return _claims;
            }
        }

        public void Replace(S2CRewardClaimsState s)
        {
            ClaimsRevision = s.ClaimsRevision;
            TotalCount = s.TotalCount;
            Cap = s.Cap;
            _claims.Clear();
            foreach (RewardClaimView v in s.Claims)
            {
                _claims.Add(RewardClaimModel.FromView(v));
            }
        }

        public void ApplyDelta(S2CRewardClaimDelta d)
        {
            ClaimsRevision = d.ClaimsRevision;
            TotalCount = d.TotalCount;
            var removed = new HashSet<ByteString>();
            foreach (ByteString id in d.Removed)
            {
                removed.Add(id);
            }
            if (removed.Count > 0)
            {
                _claims.RemoveAll(c => removed.Contains(
                    ByteString.CopyFrom(c.RewardClaimId)));
            }
            foreach (RewardClaimView v in d.Added)
            {
                _claims.Add(RewardClaimModel.FromView(v));
            }
        }
    }
}
