namespace ThinhThan.Core.Session
{
    /// <summary>
    /// Client-side session FSM phases (IMP-065): the transport/session layer
    /// tracks DISCONNECTED -> CONNECTING -> AUTHENTICATING -> CHARACTER_SELECT
    /// -> IN_WORLD, with TRANSFERRING_MAP and RECONNECTING transitions per
    /// reconnect.md and the client experience contract.
    /// </summary>
    public enum SessionPhase
    {
        Disconnected = 0,
        Connecting = 1,
        Authenticating = 2,
        CharacterSelect = 3,
        InWorld = 4,
        TransferringMap = 5,
        Reconnecting = 6,
    }
}
