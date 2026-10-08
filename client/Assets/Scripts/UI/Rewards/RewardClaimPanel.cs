using System.Collections.Generic;
using ThinhThan.Core.Runtime;
using ThinhThan.Systems.Rewards;
using TMPro;
using UnityEngine;
using UnityEngine.UI;

namespace ThinhThan.UI.Rewards
{
    /// <summary>
    /// Reward claims panel: pending-claim rows (oldest first, ADR-0064
    /// 50-row snapshot) + claim buttons firing intents only — the
    /// server adjudicates (408/409); rows re-render only on a new
    /// authoritative frame (434/441).
    /// </summary>
    public sealed class RewardClaimPanel : MonoBehaviour
    {
        public RectTransform? GridRoot;
        public RewardClaimRowView? RowViewPrefab;
        public TMP_Text? HeaderText;
        public TMP_Text? ResultText;
        public IRewardClaimIntents? Intents
        {
            get;
            set;
        }

        private Pool<RewardClaimRowView>? _pool;
        private readonly List<RewardClaimRowView> _live =
            new List<RewardClaimRowView>();
        private byte[] _lastRequestedClaimId = new byte[0];

        /// <summary>Apply-call count (once-per-version proof).</summary>
        public int ApplyCount
        {
            get;
            private set;
        }

        /// <summary>Live row count (test introspection).</summary>
        public int LiveCount
        {
            get
            {
                return _live.Count;
            }
        }

        /// <summary>Row i (test introspection).</summary>
        public RewardClaimRowView CellAt(int i)
        {
            return _live[i];
        }

        /// <summary>Last claim-request id (test introspection).</summary>
        public byte[] LastRequestedClaimId
        {
            get
            {
                return _lastRequestedClaimId;
            }
        }

        private void Awake()
        {
            if (HeaderText != null)
            {
                HeaderText.raycastTarget = false;
            }
            if (ResultText != null)
            {
                ResultText.raycastTarget = false;
            }
            _pool = new Pool<RewardClaimRowView>(CreateRow,
                c => c.gameObject.SetActive(false));
            _pool.Prewarm(8);
        }

        private RewardClaimRowView CreateRow()
        {
            RewardClaimRowView c;
            if (RowViewPrefab != null)
            {
                c = Instantiate(RowViewPrefab, GridRoot, false);
            }
            else
            {
                var go = new GameObject("row", typeof(RectTransform),
                    typeof(RewardClaimRowView));
                if (GridRoot != null)
                {
                    go.transform.SetParent(GridRoot, false);
                }
                c = go.GetComponent<RewardClaimRowView>();
            }
            return c;
        }

        /// <summary>Rebuild rows once per applied authoritative
        /// state — never per frame.</summary>
        public void Apply(RewardClaimsState s)
        {
            ApplyCount++;
            EnsureRows(s.Claims.Count);
            for (int i = 0; i < s.Claims.Count; i++)
            {
                RewardClaimModel m = s.Claims[i];
                _live[i].Apply(m);
                Button? claim = _live[i].ClaimButton;
                if (claim != null)
                {
                    byte[] id = m.RewardClaimId;
                    claim.onClick.RemoveAllListeners();
                    claim.onClick.AddListener(
                        () => OnClaimClicked(id));
                }
            }
            for (int i = s.Claims.Count; i < _live.Count; i++)
            {
                _live[i].gameObject.SetActive(false);
            }
            if (HeaderText != null)
            {
                HeaderText.SetText(
                    s.TotalCount + "/" + s.Cap + " claims");
            }
        }

        /// <summary>Displays the latest 409 verdict (error or ok).</summary>
        public void ApplyResult(ThinhThan.Protocol.V1.S2CRewardClaimResult r)
        {
            if (ResultText == null || r.Result == null)
            {
                return;
            }
            ResultText.SetText(r.Result.ErrorCode ==
                ThinhThan.Protocol.V1.ErrorCode.Unspecified
                    ? "ok" : r.Result.ErrorCode.ToString());
        }

        private void EnsureRows(int needed)
        {
            while (_live.Count < needed)
            {
                RewardClaimRowView c = _pool!.Rent();
                c.gameObject.SetActive(true);
                _live.Add(c);
            }
        }

        private void OnClaimClicked(byte[] claimId)
        {
            if (Intents == null)
            {
                return;
            }
            _lastRequestedClaimId = claimId;
            _ = Intents.RequestClaim(claimId,
                System.Threading.CancellationToken.None);
        }
    }
}
