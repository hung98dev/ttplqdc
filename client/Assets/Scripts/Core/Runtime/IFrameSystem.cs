namespace ThinhThan.Core.Runtime
{
    /// <summary>
    /// A system ticked by <see cref="FrameLoop"/> inside one
    /// <see cref="FramePhase"/>. Implementations must not hold Unity frame
    /// callbacks; they are driven exclusively through <see cref="Tick"/>.
    /// </summary>
    public interface IFrameSystem
    {
        void Tick(in FrameTime time);
    }
}
