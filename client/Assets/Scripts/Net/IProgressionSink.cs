namespace ThinhThan.Net
{
    /// <summary>
    /// Consumer of progression frames (514, 515) the session driver
    /// forwards under lease. Implemented by Systems/Progression (IMP-011).
    /// </summary>
    public interface IProgressionSink
    {
        void Apply(DecodedFrame frame);
    }
}
