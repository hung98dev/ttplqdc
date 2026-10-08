namespace ThinhThan.Systems.Progression
{
    /// <summary>
    /// Outbound progression ids (messages.md § Progression) — the 511-513
    /// durable commands. Inbound 514/515 constants live on
    /// <c>ThinhThan.Net.WireIds</c>.
    /// </summary>
    public static class ProgressionWireIds
    {
        public const uint C2SSkillUpgrade = 511;
        public const uint C2SPotentialAllocate = 512;
        public const uint C2SRespec = 513;
    }
}
