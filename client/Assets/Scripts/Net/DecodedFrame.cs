using Google.Protobuf;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Net
{
    /// <summary>
    /// One decoded inbound envelope plus the lease bookkeeping the receive
    /// queue enforces (protocol.md § Client Receive Queue and Lease
    /// Ownership): every entry carries (connection_generation, session_epoch,
    /// server_seq, baseline_id?) and its live accounted bytes.
    /// </summary>
    public sealed class DecodedFrame
    {
        public int ConnectionGeneration;

        public ulong SessionEpoch;

        public ulong ServerSeq;

        public ulong CorrelationId;

        public uint MessageId;

        /// <summary>baseline_id for replication messages; 0 otherwise.</summary>
        public ulong BaselineId;

        /// <summary>The raw parsed envelope.</summary>
        public Envelope? Envelope;

        /// <summary>Parsed payload message; null for unregistered ids.</summary>
        public IMessage? Payload;

        /// <summary>Barrier ordering counter at enqueue time.</summary>
        public int BarrierOrdinal;

        /// <summary>Accounted live bytes charged against the queue bound.</summary>
        public int AccountedBytes;

        /// <summary>Monotonic enqueue order inside one generation.</summary>
        public long Sequence;

        public void Reset()
        {
            ConnectionGeneration = 0;
            SessionEpoch = 0UL;
            ServerSeq = 0UL;
            CorrelationId = 0UL;
            MessageId = 0U;
            BaselineId = 0UL;
            Envelope = null;
            Payload = null;
            BarrierOrdinal = 0;
            AccountedBytes = 0;
            Sequence = 0L;
        }
    }
}
