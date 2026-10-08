namespace ThinhThan.Systems.Trade
{
    /// <summary>Client FSM phases for a direct-trade session.</summary>
    public enum TradePhase
    {
        None = 0,
        Invited = 1,
        Open = 2,
        Locked = 3,
        Committing = 4,
        Completed = 5,
        Cancelled = 6,
    }
}
