namespace ThinhThan.UI.Character
{
    /// <summary>Modal kinds surfaced by <see cref="SessionUiPresenter"/>.</summary>
    public enum SessionModal
    {
        None = 0,
        SessionReplaced = 1,
        ClientUpdateRequired = 2,
        ServerDraining = 3,
        Disconnected = 4,
    }
}
