using System.Collections.Generic;

namespace ThinhThan.Systems.Trade
{
    /// <summary>One side's offer in the 706 projection.</summary>
    public struct TradeSideModel
    {
        public byte[] CharacterId;
        public List<TradeOfferItemModel> Items;
        public long CommonAmount;
        public bool Confirmed;
    }
}
