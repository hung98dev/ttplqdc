using System.Text;
using ThinhThan.Systems.GuildStorage;
using ThinhThan.Protocol.V1;
using TMPro;
using UnityEngine;

namespace ThinhThan.UI.GuildStorage
{
    /// <summary>
    /// Guild-storage panel bound to <see cref="GuildStorageApplier"/>:
    /// COMMON/RESERVE section item lists, claim list with state, and
    /// the last 649 verdict surface. One render per
    /// <see cref="GuildStorageApplier.Version"/> bump.
    /// </summary>
    public sealed class GuildStoragePanel : MonoBehaviour
    {
        public GuildStorageApplier? Applier
        {
            get;
            set;
        }
        public TMP_Text? CommonText;
        public TMP_Text? ReserveText;
        public TMP_Text? ClaimsText;
        public TMP_Text? ResultText;

        private ulong _applied;

        /// <summary>Render entry point — called by the UI host once per
        /// frame loop tick; no Update() (ADR-0059).</summary>
        public void RenderIfDirty()
        {
            GuildStorageApplier? a = Applier;
            if (a == null || a.Version == _applied)
            {
                return;
            }
            _applied = a.Version;
            RenderAll(a);
        }

        private void RenderAll(GuildStorageApplier a)
        {
            var common = new StringBuilder();
            var reserve = new StringBuilder();
            foreach (var item in a.State.Items)
            {
                var line = item.ItemId + " x" + item.Quantity + "\n";
                if (item.Section == GuildStorageSection.Reserve)
                {
                    reserve.Append(line);
                }
                else
                {
                    common.Append(line);
                }
            }
            if (CommonText != null)
            {
                CommonText.text = common.ToString();
            }
            if (ReserveText != null)
            {
                ReserveText.text = reserve.ToString();
            }
            var claims = new StringBuilder();
            foreach (var c in a.State.Claims)
            {
                claims.Append("claim ").Append(c.State)
                    .Append(" x").Append(c.Quantity).Append('\n');
            }
            if (ClaimsText != null)
            {
                ClaimsText.text = claims.ToString();
            }
            if (ResultText != null)
            {
                ResultText.text = a.LastResult == null
                    ? string.Empty
                    : a.LastResult.Result?.Status.ToString() ?? string.Empty;
            }
        }
    }
}
