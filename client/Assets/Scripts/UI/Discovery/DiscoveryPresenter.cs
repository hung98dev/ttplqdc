using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.UI.Discovery
{
    /// <summary>
    /// Consumes authoritative S2C_PROGRESSION_EVENT frames: a
    /// DISCOVERY event kind with source_kind DISCOVERY presents exactly
    /// one notice per reference_id (map_id). Duplicates — reconnect
    /// restores, redelivered frames — never re-present, matching the
    /// server's once-only grant.
    /// </summary>
    public sealed class DiscoveryPresenter
    {
        private const string DiscoverySourceKind = "DISCOVERY";

        private readonly DiscoveryNoticePanel _panel;
        private readonly HashSet<string> _shown = new HashSet<string>();

        public DiscoveryPresenter(DiscoveryNoticePanel panel)
        {
            _panel = panel;
        }

        /// <summary>Applies one decoded 506 frame; returns true when a notice was shown.</summary>
        public bool Apply(S2CProgressionEvent evt)
        {
            if (evt == null ||
                evt.EventKind != ProgressionEventKind.Discovery ||
                evt.SourceKind != DiscoverySourceKind ||
                string.IsNullOrEmpty(evt.ReferenceId))
            {
                return false;
            }
            if (!_shown.Add(evt.ReferenceId))
            {
                Suppressed++;
                return false;
            }
            _panel.Show(new DiscoveryNoticeModel(
                evt.ReferenceId,
                evt.Amount,
                unchecked((int)evt.LevelAfter),
                evt.ServerTimeMs));
            return true;
        }

        /// <summary>Suppress-count for tests/diagnostics.</summary>
        public int Suppressed { get; private set; }
    }
}
