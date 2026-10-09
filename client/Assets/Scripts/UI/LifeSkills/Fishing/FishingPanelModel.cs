using System.Collections.Generic;

namespace ThinhThan.UI.LifeSkills.Fishing
{
    /// <summary>
    /// View-state of the fishing panel: the cast prompt, the hook
    /// window strip (opens 2 s after cast, one hook in [0.40 s, 1.20 s]
    /// — timing hint only, the server owns acceptance), the daily
    /// counter and the latest catch/peak rows. Pure data — headless
    /// PlayMode tests drive it without scene objects.
    /// </summary>
    public sealed class FishingPanelModel
    {
        /// <summary>One granted catch row.</summary>
        public readonly struct CatchRow
        {
            public CatchRow(string itemId, uint quantity)
            {
                ItemId = itemId;
                Quantity = quantity;
            }

            /// <summary>item_id of the caught fish.</summary>
            public string ItemId
            {
                get;
            }

            /// <summary>Quantity granted.</summary>
            public uint Quantity
            {
                get;
            }
        }

        /// <summary>Catch rows of the latest HOOK result.</summary>
        public IReadOnlyList<CatchRow> Catches = new List<CatchRow>();

        /// <summary>CAST in flight or committed (window coming).</summary>
        public bool Casting;

        /// <summary>Hook window hint open.</summary>
        public bool HookWindowOpen;

        /// <summary>Window elapsed hint (0.0 at open).</summary>
        public float WindowElapsedSeconds;

        /// <summary>Successes caught today (n/50).</summary>
        public int DailyCatchCount;

        /// <summary>Daily cap projection.</summary>
        public int DailyCatchCap;

        /// <summary>Rare-catch peak pending presentation.</summary>
        public bool RarePeak;

        /// <summary>Latest failed interact error ("" = none).</summary>
        public string ErrorKey = "";
    }
}
