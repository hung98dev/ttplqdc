using System;

namespace ThinhThan.Net
{
    /// <summary>
    /// Single-owner lease over one queued frame (protocol.md § Client Receive
    /// Queue and Lease Ownership): the queue owns the entry until dequeue, the
    /// consumer owns it until <see cref="Dispose"/> releases the accounted
    /// bytes back. Disposing twice is a no-op.
    /// </summary>
    public struct ReceiveLease : IDisposable
    {
        private ReceiveQueue? _owner;

        internal ReceiveLease(ReceiveQueue owner, DecodedFrame frame)
        {
            _owner = owner;
            Frame = frame;
        }

        public DecodedFrame Frame
        {
            get;
        }

        public void Dispose()
        {
            ReceiveQueue? owner = _owner;
            _owner = null;
            owner?.Release(Frame);
        }
    }
}
