namespace ThinhThan.Systems.Combat
{
    /// <summary>
    /// Client-side mirror of the authoritative combat action lifecycle
    /// consumed by presentation/UI. Everything here is display-only — the
    /// server owns resolution (combat.md).
    /// </summary>
    public enum CombatActionState
    {
        Idle,
        Startup,
        Active,
        Recovery,
        HitReaction,
        Dead,
    }
}
