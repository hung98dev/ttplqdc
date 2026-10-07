using ThinhThan.Systems.Inventory;
using UnityEngine;

namespace ThinhThan.UI.Inventory
{
    /// <summary>
    /// Bridges the InventoryApplier projection onto the panels at the
    /// UI phase: applies at most once per applied frame version
    /// (PERF-022 dirty flag — no state change, no UI writes).
    /// </summary>
    public sealed class InventoryPresenter : MonoBehaviour
    {
        public InventoryApplier? Applier { get; set; }
        public InventoryPanel? Inventory;
        public EntitlementPanel? Entitlements;

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
            Inventory?.Apply(Applier.State);
            Entitlements?.Apply(Applier.Panel);
        }
    }
}
