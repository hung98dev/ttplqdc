using System;
using System.Net.WebSockets;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Net
{
    /// <summary>
    /// One live WSS session: transport + receive pump + bounded receive
    /// queue + outbound sequence stamping (protocol.md § Sequence Semantics).
    /// HELLO is sent with session_epoch = 0 and client_seq = 1; after
    /// HELLO_OK the epoch is stamped and client_seq keeps strictly
    /// increasing. A new connection calls <see cref="BeginConnection"/>,
    /// which advances the receive generation and cancels the old producer.
    /// </summary>
    public sealed class NetSession : IDisposable
    {
        private readonly WebSocketTransport _transport;
        private readonly ReceiveQueue _queue;
        private readonly ReceivePump _pump;
        private readonly byte[] _sendBuffer =
            new byte[WireIds.OutboundFrameMaxBytes];

        private ulong _sessionEpoch;
        private ulong _nextClientSeq = 1UL;
        private readonly SemaphoreSlim _sendGate = new SemaphoreSlim(1, 1);

        public NetSession(ReceiveQueue? sharedQueue = null)
        {
            _transport = new WebSocketTransport();
            _queue = sharedQueue ?? new ReceiveQueue();
            _pump = new ReceivePump(_transport, _queue);
        }

        /// <summary>Terminal error seen by the pump, if any.</summary>
        public Exception? PumpFault
        {
            get
            {
                return _pump.Fault;
            }
        }

        /// <summary>True once the receive loop has exited.</summary>
        public bool PumpCompleted
        {
            get
            {
                return _pump.Completed;
            }
        }

        public WebSocketState TransportState
        {
            get
            {
                return _transport.State;
            }
        }

        public WebSocketCloseStatus? CloseStatus
        {
            get
            {
                return _transport.CloseStatus;
            }
        }

        public string? CloseReason
        {
            get
            {
                return _transport.CloseReason;
            }
        }

        public ulong SessionEpoch
        {
            get
            {
                return _sessionEpoch;
            }
        }

        /// <summary>Connects, advances the receive generation, starts the pump.</summary>
        public async Task ConnectAsync(Uri wssUri, CancellationToken cancel)
        {
            _queue.BeginGeneration();
            await _transport.ConnectAsync(wssUri, cancel).ConfigureAwait(false);
            _pump.Start();
        }

        /// <summary>Stamps envelope fields and sends one payload message.</summary>
        public async Task SendAsync(
            uint messageId, IMessage payload, CancellationToken cancel,
            ulong correlationId = 0UL)
        {
            await _sendGate.WaitAsync(cancel).ConfigureAwait(false);
            try
            {
                int count = EnvelopeCodec.Encode(
                    messageId,
                    _sessionEpoch,
                    _nextClientSeq++,
                    payload,
                    _sendBuffer,
                    correlationId);
                await _transport.SendAsync(
                    new ReadOnlyMemory<byte>(_sendBuffer, 0, count), cancel)
                    .ConfigureAwait(false);
            }
            finally
            {
                _sendGate.Release();
            }
        }

        /// <summary>Sends C2S_HELLO — always session_epoch=0, client_seq=1.</summary>
        public Task SendHelloAsync(C2SHello hello, CancellationToken cancel)
        {
            _nextClientSeq = 1UL;
            return SendAsync(WireIds.C2SHello, hello, cancel);
        }

        /// <summary>Applies the epoch HELLO_OK assigned; seq keeps rising.</summary>
        public void AcceptEpoch(ulong sessionEpoch)
        {
            _sessionEpoch = sessionEpoch;
        }

        /// <summary>Drains the next queued frame under its lease.</summary>
        public bool TryDequeue(out ReceiveLease lease)
        {
            return _queue.TryDequeue(out lease);
        }

        /// <summary>Graceful close + pump join.</summary>
        public async Task CloseAsync()
        {
            await _pump.StopAsync().ConfigureAwait(false);
            using var cancel = new CancellationTokenSource(TimeSpan.FromSeconds(2));
            await _transport.CloseAsync(cancel.Token).ConfigureAwait(false);
        }

        /// <summary>Hard drop (loss/exhaustion/supersede).</summary>
        public async Task AbortAsync()
        {
            _transport.Abort();
            await _pump.StopAsync().ConfigureAwait(false);
        }

        public void Dispose()
        {
            _transport.Dispose();
        }
    }
}
