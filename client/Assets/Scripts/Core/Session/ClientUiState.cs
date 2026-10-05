namespace ThinhThan.Core.Session
{
    /// <summary>
    /// UI-state machine states from client_experience_contract.md: BOOT ->
    /// PATCHING_UPDATE | AUTH_TITLE; AUTH_TITLE -> CHARACTER_SELECT |
    /// LOGIN_QUEUED; CHARACTER_SELECT holds the reservation; IN_WORLD ->
    /// CHARACTER_SELECT via detach; TRANSFERRING_MAP carries the
    /// PLACEMENT_PENDING substate (see SessionStateMachine.PlacementPending);
    /// DISCONNECTED retries then falls to AUTH_TITLE.
    /// </summary>
    public enum ClientUiState
    {
        Boot = 0,
        PatchingUpdate = 1,
        AuthTitle = 2,
        LoginQueued = 3,
        CharacterSelect = 4,
        InWorld = 5,
        TransferringMap = 6,
        Disconnected = 7,
    }
}
