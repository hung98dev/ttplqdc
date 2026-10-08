namespace ThinhThan.Net
{
    /// <summary>
    /// Consumer of party frames (603, 607, 636, 653) the session driver
    /// forwards under lease. Implemented by Systems/Party (IMP-035).
    /// </summary>
    public interface IPartySink
    {
        void Apply(DecodedFrame frame);
    }
}
