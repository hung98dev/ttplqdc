using System;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// Dirty sections of <see cref="HudDataModel"/> — PERF-022: widgets
    /// only re-draw sections whose bit was marked this frame.
    /// </summary>
    [Flags]
    public enum HudDirty : uint
    {
        None = 0U,
        Vitals = 0x1U,
        Target = 0x2U,
        Statuses = 0x4U,
        Cooldowns = 0x8U,
        WorldContext = 0x10U,
        QuestDock = 0x20U,
        ChatDock = 0x40U,
        Interact = 0x80U,
    }
}
