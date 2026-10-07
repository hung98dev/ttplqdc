namespace ThinhThan.UI.StateMachine
{
    /// <summary>
    /// Active skill slots 1..5 (K, L, U, I, O / Y, B, RB, RT, LB) → skill_id
    /// for C2S_SKILL_USE. Composition supplies the build; the hotbar widget
    /// and the intent dispatcher share it.
    /// </summary>
    public interface ISkillLoadout
    {
        /// <summary>skill_id for slot (null = empty → no wire intent).</summary>
        string? SkillIdAt(int slot);
    }
}
