using ThinhThan.Systems.Crafting;
using UnityEngine;

namespace ThinhThan.UI.Crafting
{
    /// <summary>
    /// Bridges the CraftingApplier projection onto the panel at the UI
    /// phase: applies at most once per applied frame version
    /// (PERF-022 dirty flag).
    /// </summary>
    public sealed class CraftingPresenter : MonoBehaviour
    {
        public CraftingApplier? Applier
        {
            get;
            set;
        }
        public CraftingPanel? Panel;

        private ulong _appliedVersion;

        /// <summary>Frame-version watermark applied so far.</summary>
        public ulong AppliedVersion
        {
            get
            {
                return _appliedVersion;
            }
        }

        /// <summary>
        /// Called once by the FrameLoop UI phase. A no-op while the
        /// applier's version is unchanged.
        /// </summary>
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
