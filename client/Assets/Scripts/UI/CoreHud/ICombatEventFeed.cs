using System;
using ThinhThan.Protocol.V1;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Decoded combat/interact inbound seam (F-2): wire ids 203/204/205/
    /// 304/116 reach the HUD through this feed — the envelope routing lands
    /// in composition, trackers subscribe here. Events fire on the frame
    /// thread only (feed impls are invoked from the NetReceive/UI phases).
    /// </summary>
    public interface ICombatEventFeed
    {
        /// <summary>S2C_ACTION_STARTED (203): cast/cooldown source.</summary>
        event Action<S2CActionStarted> ActionStarted;

        /// <summary>S2C_ACTION_REJECTED (204): clears a pending intent.</summary>
        event Action<S2CActionRejected> ActionRejected;

        /// <summary>S2C_STATUS_EVENT (205): buff/debuff rows.</summary>
        event Action<S2CStatusEvent> StatusEvent;

        /// <summary>S2C_COMBAT_EVENT (304): combat text / target hp.</summary>
        event Action<S2CCombatEvent> CombatEvent;

        /// <summary>S2C_INTERACT_RESULT (116): interact outcome feedback.</summary>
        event Action<S2CInteractResult> InteractResult;
    }
}
