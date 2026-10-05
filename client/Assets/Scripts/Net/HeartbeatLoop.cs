using System;
using System.Threading;
using System.Threading.Tasks;

namespace ThinhThan.Net
{
    /// <summary>
    /// Heartbeat cadence (protocol.md § Heartbeat): C2S_HEARTBEAT every
    /// <see cref="DefaultIntervalMs"/> carrying client monotonic ms and the
    /// last observed server ms; connection is considered lost after
    /// <see cref="DefaultLossMs"/> without any inbound frame.
    /// </summary>
    public sealed class HeartbeatLoop : IDisposable
    {
        public const int DefaultIntervalMs = 5000;

        public const int DefaultLossMs = 15000;

        private readonly Func<long> _nowMs;
        private readonly Func<CancellationToken, Task> _sendHeartbeat;
        private readonly Func<CancellationToken, Task> _onLoss;
        private readonly int _intervalMs;
        private readonly int _lossMs;
        private CancellationTokenSource? _cancel;
        private Task? _task;
        private long _lastRxMs;
        private long _lastServerMs;
        private volatile bool _lost;

        /// <param name="intervalMs">From S2C_HELLO_OK.heartbeat_interval_ms.</param>
        /// <param name="lossMs">From S2C_HELLO_OK.connection_timeout_ms.</param>
        /// <param name="nowMs">Injected monotonic clock for tests.</param>
        public HeartbeatLoop(
            int intervalMs,
            int lossMs,
            Func<long> nowMs,
            Func<CancellationToken, Task> sendHeartbeat,
            Func<CancellationToken, Task> onLoss)
        {
            _intervalMs = intervalMs <= 0 ? DefaultIntervalMs : intervalMs;
            _lossMs = lossMs <= 0 ? DefaultLossMs : lossMs;
            _nowMs = nowMs;
            _sendHeartbeat = sendHeartbeat;
            _onLoss = onLoss;
        }

        public bool Lost
        {
            get
            {
                return _lost;
            }
        }

        /// <summary>Echo field: the server_ms from the last S2C_HEARTBEAT.</summary>
        public long LastServerMs
        {
            get
            {
                return _lastServerMs;
            }
        }

        public void Start()
        {
            _lastRxMs = _nowMs();
            _cancel = new CancellationTokenSource();
            _task = Task.Run(RunAsync);
        }

        /// <summary>Any inbound frame counts as liveness.</summary>
        public void NoteInbound()
        {
            _lastRxMs = _nowMs();
            _lost = false;
        }

        /// <summary>S2C_HEARTBEAT received: retain server_ms for echo.</summary>
        public void NotePong(long serverMs)
        {
            _lastServerMs = serverMs;
            NoteInbound();
        }

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
        }

        public void Dispose()
        {
            _cancel?.Cancel();
            _cancel?.Dispose();
        }

        private async Task RunAsync()
        {
            // _cancel is set in Start() before the task is launched; RunAsync
            // never runs without it.
            CancellationToken cancel = _cancel!.Token;
            while (!cancel.IsCancellationRequested)
            {
                try
                {
                    await Task.Delay(_intervalMs, cancel).ConfigureAwait(false);
                }
                catch (OperationCanceledException)
                {
                    return;
                }

                if (_nowMs() - _lastRxMs > _lossMs)
                {
                    _lost = true;
                    await _onLoss(cancel).ConfigureAwait(false);
                    return;
                }

                try
                {
                    await _sendHeartbeat(cancel).ConfigureAwait(false);
                }
                catch (Exception)
                {
                }
            }
        }
    }
}
