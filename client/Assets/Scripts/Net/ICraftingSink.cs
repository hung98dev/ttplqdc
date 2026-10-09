namespace ThinhThan.Net
{
    /// <summary>
    /// Consumer of craft/enhance result frames (405, 407) the session
    /// driver forwards under lease. Implemented by Systems/Crafting
    /// (IMP-027).
    /// </summary>
    public interface ICraftingSink
    {
        void Apply(DecodedFrame frame);
    }
}
