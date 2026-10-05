using System;
using System.Net.WebSockets;
using System.Threading;
using System.Threading.Tasks;

namespace ThinhThan.Net
{
    /// <summary>
    /// Thin owner of one <see cref="ClientWebSocket"/> binary stream
    /// (protocol.md § Transport): one WebSocket binary message is exactly one
    /// envelope, no per-message compression. Sends are bounded by
    /// <see cref="WireIds.OutboundFrameMaxBytes"/>; the receive loop caps each
    /// inbound message at <see cref="WireIds.InboundFrameMaxBytes"/>.
    /// </summary>
    public sealed class WebSocketTransport : IDisposable
    {
        private readonly ClientWebSocket _socket = new ClientWebSocket();
        private readonly SemaphoreSlim _sendGate = new SemaphoreSlim(1, 1);
        private volatile bool _closed;

        public WebSocketState State
        {
            get
            {
                return _socket.State;
            }
        }

        /// <summary>Close status observed from the peer, if any.</summary>
        public WebSocketCloseStatus? CloseStatus
        {
            get
            {
                return _socket.CloseStatus;
            }
        }

        /// <summary>Close reason text observed from the peer, if any.</summary>
        public string? CloseReason
        {
            get
            {
                return _socket.CloseStatusDescription;
            }
        }

        public async Task ConnectAsync(Uri uri, CancellationToken cancel)
        {
            _closed = false;
            await _socket.ConnectAsync(uri, cancel).ConfigureAwait(false);
        }

        /// <summary>Sends one complete binary frame.</summary>
        public async Task SendAsync(ReadOnlyMemory<byte> frame, CancellationToken cancel)
        {
            if (frame.Length > WireIds.OutboundFrameMaxBytes)
            {
                throw new InvalidOperationException(
                    "Outbound frame exceeds 256 KiB limit");
            }

            await _sendGate.WaitAsync(cancel).ConfigureAwait(false);
            try
            {
                await _socket.SendAsync(
                    frame, WebSocketMessageType.Binary, true, cancel)
                    .ConfigureAwait(false);
            }
            finally
            {
                _sendGate.Release();
            }
        }

        /// <summary>
        /// Receives one complete binary message into
        /// <paramref name="destination"/> (accumulates fragments until
        /// EndOfMessage). Returns bytes written, or -1 when the peer closed.
        /// </summary>
        public async Task<int> ReceiveAsync(
            Memory<byte> destination, CancellationToken cancel)
        {
            int offset = 0;
            while (true)
            {
                var result = await _socket.ReceiveAsync(
                    destination.Slice(offset), cancel).ConfigureAwait(false);
                if (result.MessageType == WebSocketMessageType.Close)
                {
                    _closed = true;
                    return -1;
                }

                offset += result.Count;
                if (offset > WireIds.InboundFrameMaxBytes)
                {
                    throw new InvalidOperationException(
                        "Inbound frame exceeds 64 KiB limit");
                }

                if (result.EndOfMessage)
                {
                    return offset;
                }
            }
        }

        /// <summary>Graceful close handshake; tolerant of peer-side close.</summary>
        public async Task CloseAsync(CancellationToken cancel)
        {
            if (_closed || _socket.State != WebSocketState.Open)
            {
                return;
            }

            try
            {
                await _socket.CloseAsync(
                    WebSocketCloseStatus.NormalClosure, string.Empty, cancel)
                    .ConfigureAwait(false);
            }
            catch (WebSocketException)
            {
            }
            catch (OperationCanceledException)
            {
            }
            finally
            {
                _closed = true;
            }
        }

        /// <summary>Hard abort (drop detected / supersede / exhaustion).</summary>
        public void Abort()
        {
            _closed = true;
            _socket.Abort();
        }

        public void Dispose()
        {
            _sendGate.Dispose();
            _socket.Dispose();
        }
    }
}
