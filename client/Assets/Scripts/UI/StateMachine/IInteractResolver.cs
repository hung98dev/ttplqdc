namespace ThinhThan.UI.StateMachine
{
    /// <summary>Resolves the context-interact (F / LT) target.</summary>
    public interface IInteractResolver
    {
        /// <summary>False when nothing interactable is in range.</summary>
        bool TryResolve(
            long selfXMm, long selfYMm, out InteractTarget target);
    }
}
