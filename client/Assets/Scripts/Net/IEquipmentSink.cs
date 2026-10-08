namespace ThinhThan.Net
{
    /// <summary>
    /// Consumer of loadout/equipment frames (403) the session driver
    /// forwards under lease. Implemented by Systems/Equipment
    /// (IMP-012).
    /// </summary>
    public interface IEquipmentSink
    {
        void Apply(DecodedFrame frame);
    }
}
