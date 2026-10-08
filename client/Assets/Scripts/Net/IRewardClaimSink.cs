namespace ThinhThan.Net
{
    /// <summary>
    /// Consumer of reward-claim frames (409, 434, 440, 441) the session
    /// driver forwards under lease. Implemented by Systems/Rewards
    /// (IMP-010).
    /// </summary>
    public interface IRewardClaimSink
    {
        void Apply(DecodedFrame frame);
    }
}
