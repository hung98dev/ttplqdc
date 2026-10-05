using System;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;

namespace ThinhThan.Net
{
    /// <summary>
    /// The single receiver producer (protocol.md § Client Receive Queue): the
    /// pump owns each frame until it is enqueued, at which point the queue
    /// takes ownership. Decode happens here on the pump task; the main thread
    /// only applies entries under lease.
    /// </summary>
    public sealed class ReceivePump
    {
        private readonly WebSocketTransport _transport;
        private readonly ReceiveQueue _queue;
        private readonly Action<int>? _onBytes;
        private CancellationTokenSource? _cancel;
        private Task? _task;

        public ReceivePump(
            WebSocketTransport transport, ReceiveQueue queue,
            Action<int>? onBytes = null)
        {
            _transport = transport;
            _queue = queue;
            _onBytes = onBytes;
        }

        /// <summary>Terminal exception of the pump loop, if it died.</summary>
        public Exception? Fault
        {
            get;
            private set;
        }

        /// <summary>True once the loop has exited for any reason.</summary>
        public bool Completed
        {
            get;
            private set;
        }

        public void Start()
        {
            _cancel = new CancellationTokenSource();
            _task = Task.Run(RunAsync);
        }

        /// <summary>Cancels the loop and waits for it to join.</summary>
        public async Task StopAsync()
        {
            CancellationTokenSource? cancel = _cancel;
            if (cancel == null)
            {
                return;
            }

            cancel.Cancel();
            try
            {
                if (_task != null)
                {
                    await _task.ConfigureAwait(false);
                }
            }
            catch (OperationCanceledException)
            {
            }
            catch (Exception error)
            {
                Fault = error;
            }
        }

        private async Task RunAsync()
        {
            var buffer = new byte[WireIds.InboundFrameMaxBytes];
            try
            {
                while (true)
                {
                    // _cancel is created in Start() before RunAsync is
                    // scheduled; the loop cannot run without it.
                    int count = await _transport.ReceiveAsync(
                        buffer, _cancel!.Token).ConfigureAwait(false);
                    if (count < 0)
                    {
                        return;
                    }

                    if (_onBytes != null)
                    {
                        _onBytes(count);
                    }
                    DecodedFrame frame = _queue.RentFrame();
                    EnvelopeCodec.Decode(
                        new ReadOnlySpan<byte>(buffer, 0, count), frame);
                    if (!_queue.TryEnqueue(frame))
                    {
                        return;
                    }
                }
            }
            catch (OperationCanceledException)
            {
            }
            catch (InvalidProtocolBufferException error)
            {
                Fault = error;
            }
            catch (Exception error)
            {
                Fault = error;
            }
            finally
            {
                Completed = true;
            }
        }
    }
}
