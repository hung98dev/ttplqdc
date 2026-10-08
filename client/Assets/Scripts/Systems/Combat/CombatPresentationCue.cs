namespace ThinhThan.Systems.Combat
{
    /// <summary>Display cue emitted by CombatPresentation each tick.</summary>
    public enum CombatPresentationCue
    {
        None,
        ActionStart,
        ActionResolve,
        ActionEnd,
        Rejected,
        HitReaction,
        JustGuardSuccess,
        Death,
    }
}
