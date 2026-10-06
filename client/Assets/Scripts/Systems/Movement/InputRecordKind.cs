namespace ThinhThan.Systems.Movement
{
    /// <summary>What kind of wire send produced an input record.</summary>
    public enum InputRecordKind
    {
        /// <summary>Held-state 100 send — <see cref="InputRecord.Flags"/> changes held input.</summary>
        Held,
        /// <summary>Edge send (101/102/108) — <see cref="InputRecord.Edge"/> is a discrete intent.</summary>
        Edge,
    }
}
