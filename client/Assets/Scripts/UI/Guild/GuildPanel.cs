using System.Collections.Generic;
using ThinhThan.Core.Runtime;
using ThinhThan.Systems.Guild;
using TMPro;
using UnityEngine;

namespace ThinhThan.UI.Guild
{
    /// <summary>
    /// Guild panel bound to <see cref="GuildApplier"/>: roster rows
    /// pooled per render, header (name/motd/capacity/recruitment),
    /// ritual+blessing summary, and the last 649 verdict. One render
    /// per <see cref="GuildApplier.Version"/> bump.
    /// </summary>
    public sealed class GuildPanel : MonoBehaviour
    {
        private static readonly Pool<GuildMemberRowView> RowPool =
            new Pool<GuildMemberRowView>(CreateRow,
                r => r.gameObject.SetActive(false));

        private static GuildMemberRowView CreateRow()
        {
            var go = new GameObject(
                "GuildRow", typeof(RectTransform),
                typeof(GuildMemberRowView));
            GuildMemberRowView row = go.GetComponent<GuildMemberRowView>();
            row.LeaderMark = new GameObject(
                "LeaderMark", typeof(RectTransform));
            row.LeaderMark.transform.SetParent(go.transform, false);
            return row;
        }

        public GuildApplier? Applier
        {
            get;
            set;
        }
        public Transform? RowContainer;
        public GameObject? InviteBanner;
        public TMP_Text? InviteText;
        public TMP_Text? HeaderText;
        public TMP_Text? MotdText;
        public TMP_Text? ProgressionText;
        public TMP_Text? BlessingText;
        public TMP_Text? ResultText;

        private readonly List<GuildMemberRowView> _rows =
            new List<GuildMemberRowView>();
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
        public GuildMemberRowView? RowAt(int i)
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
            GuildApplier? a = Applier;
            if (a == null || a.Version == _applied)
            {
                return;
            }
            _applied = a.Version;
            RenderAll(a);
        }

        public void FlushRendered()
        {
            GuildApplier? a = Applier;
            if (a == null)
            {
                return;
            }
            RenderAll(a);
            _applied = a.Version;
        }

        private void RenderAll(GuildApplier a)
        {
            RenderHeader(a.State);
            RenderRows(a.State);
            RenderProgression(a.State.Progression);
            RenderInvite(a.LastInvite);
            RenderResult();
        }

        private void RenderHeader(GuildState s)
        {
            if (HeaderText != null)
            {
                HeaderText.text = s.InGuild
                    ? s.GuildName + " Lv" + s.Level + " (" +
                        s.MembersCount + "/" + s.MaxMembers + ") " +
                        s.RecruitmentMode
                    : string.Empty;
            }
            if (MotdText != null)
            {
                MotdText.text = s.Motd;
            }
        }

        private void RenderRows(GuildState state)
        {
            foreach (GuildMemberRowView r in _rows)
            {
                RowPool.Return(r);
            }
            _rows.Clear();
            bool canKick = state.Role == "guild.role.leader" ||
                state.Role == "guild.role.vice_leader" ||
                state.Role == "guild.role.officer";
            foreach (GuildMemberModel m in state.Members)
            {
                GuildMemberRowView row = RowPool.Rent();
                row.gameObject.SetActive(true);
                row.transform.SetParent(RowContainer, false);
                row.Bind(m, canKick);
                _rows.Add(row);
            }
        }

        private void RenderProgression(GuildProgressionModel p)
        {
            if (ProgressionText != null)
            {
                int filled = 0;
                foreach (GuildProgressionModel.ElementPoint pt in p.Points)
                {
                    if (pt.Current >= p.RequiredPerElement)
                    {
                        filled++;
                    }
                }
                ProgressionText.text = "EXP " + p.GuildExp +
                    " | cycle " + p.CycleId + " vessels " + filled + "/5" +
                    " streak " + p.RitualStreak;
            }
            if (BlessingText != null)
            {
                BlessingText.text = p.ActiveBlessingId.Length > 0
                    ? "blessing: " + p.ActiveBlessingId
                    : (p.Candidates.Count > 0
                        ? "vote open: " + p.Candidates.Count + " candidates"
                        : string.Empty);
            }
        }

        private void RenderInvite(
            global::ThinhThan.Protocol.V1.S2CGuildInvite? inv)
        {
            if (InviteBanner != null)
            {
                InviteBanner.SetActive(inv != null);
            }
            if (inv != null && InviteText != null)
            {
                InviteText.text =
                    inv.InviterName + " invites you to " + inv.GuildName;
            }
        }

        private void RenderResult()
        {
            if (ResultText == null || Applier == null)
            {
                return;
            }
            global::ThinhThan.Protocol.V1.S2CGuildResult? r =
                Applier.LastResult;
            ResultText.text = r == null
                ? string.Empty
                : r.Result.Status.ToString() + " " +
                    r.Result.ErrorCode.ToString();
        }
    }
}
