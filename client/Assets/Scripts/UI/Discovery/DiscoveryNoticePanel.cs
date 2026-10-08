using System.Collections.Generic;

namespace ThinhThan.UI.Discovery
{
    /// <summary>
    /// Presentation state for first-discovery notices: the committed
    /// notices in arrival order plus the newest one the banner shows.
    /// Pure C# — headless PlayMode tests drive it without scene objects.
    /// </summary>
    public sealed class DiscoveryNoticePanel
    {
        private readonly List<DiscoveryNoticeModel> _notices = new List<DiscoveryNoticeModel>();
        private int _shownIndex = -1;

        /// <summary>All committed notices, oldest first.</summary>
        public IReadOnlyList<DiscoveryNoticeModel> Notices
        {
            get
            {
                return _notices;
            }
        }

        /// <summary>The notice currently presented; empty when none.</summary>
        public DiscoveryNoticeModel? Current
        {
            get
            {
                return _shownIndex >= 0 ? _notices[_shownIndex] : (DiscoveryNoticeModel?)null;
            }
        }

        /// <summary>Records a granted discovery and presents it.</summary>
        public void Show(DiscoveryNoticeModel notice)
        {
            _notices.Add(notice);
            _shownIndex = _notices.Count - 1;
        }

        /// <summary>Total notices presented (dedupe check in tests).</summary>
        public int Count
        {
            get
            {
                return _notices.Count;
            }
        }
    }
}
