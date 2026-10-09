using ThinhThan.Net;

namespace ThinhThan.Systems.Atlas
{
    /// <summary>
    /// Inbound sink contract for Atlas frames. The orchestrator's Net/
    /// seam forwards decoded 505/506/518 payloads under the session
    /// lease (binding lands via the routed IMP-065 gatefix — see the
    /// packet's surface flag).
    /// </summary>
    public interface IAtlasSink
    {
        void Apply(DecodedFrame frame);
    }
}
