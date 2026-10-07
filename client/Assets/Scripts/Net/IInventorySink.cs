namespace ThinhThan.Net
{
    /// <summary>
    /// Consumer of inventory/wallet/entitlement frames (401, 419, 429,
    /// 432, 433, 435) the session driver forwards under lease. Implemented
    /// by Systems/Inventory (IMP-009).
    /// </summary>
    public interface IInventorySink
    {
        void Apply(DecodedFrame frame);
    }
}
