using System.Collections.Generic;
using ThinhThan.Core.Runtime;
using ThinhThan.Systems.Party;
using TMPro;
using UnityEngine;

namespace ThinhThan.UI.Party
{
    /// <summary>
    /// Roster panel bound to <see cref="PartyApplier"/>: rows pooled per
    /// render, one render per <see cref="PartyApplier.Version"/> bump.
    /// Shows incoming invites (603) and the last verdict (653).
    /// </summary>
    public sealed class PartyPanel : MonoBehaviour
    {
        private static readonly Pool<PartyMemberRowView> RowPool =
            new Pool<PartyMemberRowView>(CreateRow,
                r => r.gameObject.SetActive(false));

        private static PartyMemberRowView CreateRow()
        {
            var go = new GameObject(
                "PartyRow", typeof(RectTransform),
                typeof(PartyMemberRowView));
            PartyMemberRowView row = go.GetComponent<PartyMemberRowView>();
            row.LeaderMark = new GameObject(
                "LeaderMark", typeof(RectTransform));
            row.LeaderMark.transform.SetParent(go.transform, false);
            return row;
        }

        public PartyApplier? Applier
        {
            get;
            set;
        }
        public Transform? RowContainer;
        public GameObject? InviteBanner;
        public TMP_Text? InviteText;
        public TMP_Text? ResultText;

        private readonly List<PartyMemberRowView> _rows =
            new List<PartyMemberRowView>();
        private ulong _applied;

        /// <summary>Live row count (test introspection).</summary>
        public int RowCount
        {
            get
            {
                return _rows.Count;
            }
        }

        /// <summary>Row i (test introspection).</summary>
        public PartyMemberRowView? RowAt(int i)
        {
            if (i < 0 || i >= _rows.Count)
            {
                return null;
            }
            return _rows[i];
        }

        /// <summary>Render entry point — called by the UI host once per
        /// frame loop tick; no Update() (ADR-0059).</summary>
        public void RenderIfDirty()
        {
            PartyApplier? a = Applier;
            if (a == null || a.Version == _applied)
            {
                return;
            }
            _applied = a.Version;
            RenderRows(a.State);
            RenderInvite(a.LastInvite);
            RenderResult();
        }

        public void FlushRendered()
        {
            PartyApplier? a = Applier;
            if (a == null)
            {
                return;
            }
            RenderRows(a.State);
            RenderInvite(a.LastInvite);
            RenderResult();
            _applied = a.Version;
        }

        private void RenderRows(PartyState state)
        {
            foreach (PartyMemberRowView r in _rows)
            {
                RowPool.Return(r);
            }
            _rows.Clear();
            foreach (PartyMemberModel m in state.Members)
            {
                PartyMemberRowView row = RowPool.Rent();
                row.gameObject.SetActive(true);
                row.transform.SetParent(RowContainer, false);
                row.Bind(m, state.LeaderCharacterId);
                _rows.Add(row);
            }
        }

        private void RenderInvite(
            global::ThinhThan.Protocol.V1.S2CPartyInvite? inv)
        {
            if (InviteBanner != null)
            {
                InviteBanner.SetActive(inv != null);
            }
            if (inv != null && InviteText != null)
            {
                InviteText.text =
                    inv.InviterName + " invites you";
            }
        }

        private void RenderResult()
        {
            if (ResultText == null || Applier == null)
            {
                return;
            }
            global::ThinhThan.Protocol.V1.S2CPartyResult? r =
                Applier.LastResult;
            ResultText.text = r == null
                ? string.Empty
                : r.Result.Status.ToString();
        }
    }
}
