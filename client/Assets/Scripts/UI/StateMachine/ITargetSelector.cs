namespace ThinhThan.UI.StateMachine
{
    /// <summary>
    /// Hostile-target cycling seam (client_experience_contract.md §3): Tab /
    /// R3 / tap cycles hostiles in AOI by ascending distance (tie → lower
    /// entity_id). Selected intents are ONLY announced via C2S_TARGET_INTENT
    /// — the client displays the server-accepted target, never a local pick.
    /// </summary>
    public interface ITargetSelector
    {
        /// <summary>
        /// Next hostile after <paramref name="currentEntityId"/> (0 = none).
        /// Returns the entity_id to announce in C2S_TARGET_INTENT, or 0 when
        /// no hostile is visible — Tab on 0 then clears.
        /// </summary>
        ulong NextHostile(
            long selfXMm, long selfYMm, ulong currentEntityId);
    }
}
