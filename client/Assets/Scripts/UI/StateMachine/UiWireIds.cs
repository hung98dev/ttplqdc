namespace ThinhThan.UI.StateMachine
{
    /// <summary>
    /// Wire ids owned by the input/HUD layer (messages.md). Outbound:
    /// interact/portal and combat intents dispatched by
    /// <see cref="ActionIntentDispatch"/>. Inbound: the HUD combat/status
    /// feed and interact result consumed by the CoreHud trackers.
    /// Movement ids (100/101/102/108) stay on
    /// <c>ThinhThan.Systems.Movement.MovementConstants</c>.
    /// </summary>
    public static class UiWireIds
    {
        public const uint C2SInteract = 103;
        public const uint C2SPortalUse = 104;
        public const uint S2CInteractResult = 116;

        public const uint C2SSkillUse = 200;
        public const uint C2SBasicAttack = 201;
        public const uint C2STargetIntent = 202;
        public const uint S2CActionStarted = 203;
        public const uint S2CActionRejected = 204;
        public const uint S2CStatusEvent = 205;
        public const uint S2CCombatEvent = 304;
    }
}
