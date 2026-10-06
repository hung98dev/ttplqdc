using UnityEngine;

namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// HUD root (§2): the dynamic canvas carries vitals/target/buffs/
    /// hotbar; the static canvas carries quest + chat docks + the map/
    /// menu cluster — nested Canvas split so a dynamic value change never
    /// rebuilds static layout (PERF-022). <see cref="Apply"/> runs at most
    /// once per frame (called by <see cref="CoreHudSystem"/>) and only
    /// touches widgets whose model section is dirty.
    /// </summary>
    public sealed class CoreHudView : MonoBehaviour
    {
        /// <summary>Frequently-updated widgets (bars, icons, sweeps).</summary>
        public Canvas? DynamicCanvas;

        /// <summary>Rarely-updated widgets (docks, map/menu cluster).</summary>
        public Canvas? StaticCanvas;

        public VitalsBarWidget? Vitals;
        public TargetFrameWidget? TargetFrame;
        public BuffIconRow? BuffRow;
        public SkillHotbarWidget? Hotbar;
        public QuestTrackerWidget? QuestDock;
        public ChatDockWidget? ChatDock;
        public ContextButtonWidget? ContextButton;
        public MenuButtonWidget? MenuCluster;
        public VirtualJoystickWidget? Joystick;

        /// <summary>Total Apply calls — once-per-frame proof for tests.</summary>
        public int ApplyCount
        {
            get;
            private set;
        }

        /// <summary>
        /// Applies dirty model sections to their widgets. Not called by
        /// Unity — <see cref="CoreHudSystem"/> owns cadence.
        /// </summary>
        public void Apply(in HudDataModel m, ulong serverTick)
        {
            ApplyCount++;
            HudDirty dirty = m.Dirty;

            if ((dirty & HudDirty.Vitals) != 0 && Vitals != null)
            {
                Vitals.Apply(in m);
            }

            if ((dirty & HudDirty.Target) != 0 && TargetFrame != null)
            {
                TargetFrame.Apply(in m);
            }

            if ((dirty & HudDirty.Statuses) != 0 && BuffRow != null)
            {
                BuffRow.Apply(in m);
            }

            if ((dirty & HudDirty.Cooldowns) != 0 && Hotbar != null)
            {
                Hotbar.Apply(in m, serverTick);
            }

            if ((dirty & HudDirty.WorldContext) != 0 && MenuCluster != null)
            {
                MenuCluster.Apply(in m);
            }

            if ((dirty & HudDirty.QuestDock) != 0 && QuestDock != null)
            {
                QuestDock.Apply(in m);
            }

            if ((dirty & HudDirty.ChatDock) != 0 && ChatDock != null)
            {
                ChatDock.Apply(in m);
            }

            if ((dirty & HudDirty.Interact) != 0 && ContextButton != null)
            {
                ContextButton.Apply(in m);
            }
        }
    }
}
