namespace ThinhThan.Systems.Trade
{
    /// <summary>One offered item instance (whole or partial stack).</summary>
    public struct TradeOfferItemModel
    {
        public byte[] ItemInstanceId;
        public string ItemId;
        public uint Quantity;
    }
}
