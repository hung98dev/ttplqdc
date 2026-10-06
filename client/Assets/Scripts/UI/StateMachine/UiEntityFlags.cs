namespace ThinhThan.UI.StateMachine
{
    /// <summary>
    /// <c>EntityState.flags</c> bit constants (messages.md L300 field list:
    /// IN_COMBAT | DEAD | INVULNERABLE | INTERACTABLE | ELIGIBLE). Owned here
    /// because the input/HUD layer is the only consumer; the replication
    /// layer stays bit-name-free by contract.
    /// </summary>
    public static class UiEntityFlags
    {
        public const uint InCombat = 0x1U;
        public const uint Dead = 0x2U;
        public const uint Invulnerable = 0x4U;
        public const uint Interactable = 0x8U;
        public const uint Eligible = 0x10U;
    }
}
