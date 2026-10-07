namespace ThinhThan.UI.StateMachine
{
    /// <summary>
    /// Array-backed skill loadout (composition supplies the build);
    /// empty slots return null → no C2S_SKILL_USE.
    /// </summary>
    public sealed class FixedSkillLoadout : ISkillLoadout
    {
        private readonly string?[] _slots;

        /// <summary>Slot ids in hotbar order (1..5); null = empty.</summary>
        public FixedSkillLoadout(string?[] slotIds)
        {
            _slots = new string?[5];
            for (int i = 0; i < _slots.Length && i < slotIds.Length; i++)
            {
                _slots[i] = slotIds[i];
            }
        }

        public string? SkillIdAt(int slot)
        {
            return slot >= 1 && slot <= _slots.Length
                ? _slots[slot - 1]
                : null;
        }
    }
}
