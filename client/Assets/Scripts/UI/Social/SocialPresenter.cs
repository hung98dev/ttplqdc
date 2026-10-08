using ThinhThan.Systems.Social;
using UnityEngine;

namespace ThinhThan.UI.Social
{
    /// <summary>
    /// Bridges the SocialApplier projection onto the panel at the UI
    /// phase: applies at most once per applied frame version
    /// (PERF-022 dirty flag).
    /// </summary>
    public sealed class SocialPresenter : MonoBehaviour
    {
        public SocialApplier? Applier
        {
            get;
            set;
        }
        public SocialPanel? Panel;

        private ulong _appliedVersion;

        /// <summary>Frame-version watermark applied so far.</summary>
        public ulong AppliedVersion
        {
            get
            {
                return _appliedVersion;
            }
        }

        /// <summary>Called once by the FrameLoop UI phase.</summary>
        public void ApplyIfDirty()
        {
            if (Applier == null || Applier.Version == _appliedVersion)
            {
                return;
            }
            _appliedVersion = Applier.Version;
            Panel?.Apply(Applier.State);
        }
    }
}
