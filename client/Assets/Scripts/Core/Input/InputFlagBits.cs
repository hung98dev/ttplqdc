namespace ThinhThan.Core.Input
{
    /// <summary>
    /// Client-owned bit layout for <c>C2S_INPUT_STATE.input_flags</c>
    /// (messages.md: "bitmask of currently held directions/actions"; the wire
    /// pins no bit order, so the producer owns it). Held state only — edges
    /// never appear here.
    /// </summary>
    public static class InputFlagBits
    {
        public const uint MoveLeft = 0x1U;
        public const uint MoveRight = 0x2U;
        public const uint DownHeld = 0x4U;
    }
}
