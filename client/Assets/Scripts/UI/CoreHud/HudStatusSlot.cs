namespace ThinhThan.UI.CoreHud
{
    /// <summary>
    /// One buff/debuff row on the HUD (messages.md EntityStatus +
    /// S2C_STATUS_EVENT): effect id, shape-differentiated kind, stacks,
    /// expiry in server ticks.
    /// </summary>
    public readonly struct HudStatusSlot
    {
        public readonly string EffectId;
        public readonly string StatusKind;
        public readonly uint Stacks;
        public readonly ulong ExpiresAtTick;

        public HudStatusSlot(
            string effectId, string statusKind, uint stacks,
            ulong expiresAtTick)
        {
            EffectId = effectId;
            StatusKind = statusKind;
            Stacks = stacks;
            ExpiresAtTick = expiresAtTick;
        }
    }
}
