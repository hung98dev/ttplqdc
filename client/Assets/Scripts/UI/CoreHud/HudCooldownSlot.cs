namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// One hotbar cooldown slot: skill id plus absolute end tick
    /// (S2C_ACTION_STARTED.cooldown_ends_at_tick).
    /// </summary>
    public readonly struct HudCooldownSlot
    {
        public readonly int Slot;
        public readonly string SkillId;
        public readonly ulong CooldownEndsAtTick;
        public readonly ulong ServerTick;

        public HudCooldownSlot(
            int slot, string skillId, ulong cooldownEndsAtTick,
            ulong serverTick)
        {
            Slot = slot;
            SkillId = skillId;
            CooldownEndsAtTick = cooldownEndsAtTick;
            ServerTick = serverTick;
        }

        /// <summary>0..1 fill remaining at <paramref name="nowTick"/>.</summary>
        public float Fill(ulong nowTick)
        {
            if (CooldownEndsAtTick <= ServerTick || nowTick >= CooldownEndsAtTick)
            {
                return 0f;
            }

            return (float)(
                (CooldownEndsAtTick - nowTick) /
                (double)(CooldownEndsAtTick - ServerTick));
        }
    }
}
