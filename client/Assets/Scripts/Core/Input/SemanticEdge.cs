namespace ThinhThan.Core.Input
{
    /// <summary>
    /// Semantic input edges produced by <see cref="ClientInputMaps"/> and the
    /// touch virtual joystick — one enum for the whole
    /// client_experience_contract.md §3 input map. Movement-class edges
    /// (through <see cref="Drop"/>) translate to
    /// <c>ThinhThan.Systems.Movement.LocalEdge</c> inside the UI layer's
    /// GameInputSource; action classes drive ActionIntentDispatch onto the
    /// wire; UI-local classes (from <see cref="ChatOpen"/>) reach the shell
    /// only and never hit gameplay channels.
    /// </summary>
    public enum SemanticEdge
    {
        PressLeft,
        PressRight,
        ReleaseLeft,
        ReleaseRight,
        FlipLeft,
        FlipRight,
        Jump,
        Drop,

        BasicAttack,
        Skill1,
        Skill2,
        Skill3,
        Skill4,
        Skill5,
        ContextInteract,
        TargetCycle,
        TargetClear,

        ChatOpen,
        UiNavigate,
    }
}
