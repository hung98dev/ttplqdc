using System;
using System.Threading;
using Google.Protobuf;
using ThinhThan.Core.Runtime;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Movement
{
    /// <summary>
    /// Input-phase send component (client_experience_contract.md ~L105):
    /// sampled edges go out immediately as discrete 101/102/108 sends —
    /// never batched, never coalesced; held state goes out as 100 on flag
    /// change (≤1 per 50 ms) and is resent at least once per 250 ms while
    /// any flag is held. Every send records its stamped seq into
    /// <see cref="InputHistory"/> for self-ack replay.
    /// <para>
    /// No prediction math here — integration stays at
    /// <see cref="FramePhase.Prediction"/> ("Do not merge them"). Wired at
    /// composition: registered at <see cref="FramePhase.Input"/> when
    /// movement owns the slot, or invoked by the Input-phase occupant
    /// (F-08).
    /// </para>
    /// </summary>
    public sealed class MovementInputDispatch
    {
        private const int EdgeBufferSize = 16;

        private readonly IInputSource _source;
        private readonly IMovementSender _sender;
        private readonly InputHistory _history;
        private readonly SelfReconciliationSink _sink;
        private readonly LocalEdge[] _edges = new LocalEdge[EdgeBufferSize];
        private readonly PendingSend[] _pending = new PendingSend[64];
        private int _pendingCount;
        private readonly CancellationTokenSource _cancel = new CancellationTokenSource();

        private uint _lastSentFlags;
        private int _lastSentDirection;
        private double _lastHeldSendSec = double.NegativeInfinity;
        private double _lastResendSec;
        private ulong _pendingPredictedTick;

        /// <summary>Sequence of the most recently sent edge/held message.</summary>
        public ulong LastSentSeq
        {
            get
            {
                return _lastSentSeq;
            }
            private set
            {
                _lastSentSeq = value;
            }
        }

        /// <summary>Count of discrete edge sends issued (for tests/diagnostics).</summary>
        public int EdgeSendCount
        {
            get
            {
                return _edgeSendCount;
            }
            private set
            {
                _edgeSendCount = value;
            }
        }

        /// <summary>Count of held-state 100 sends issued.</summary>
        public int HeldSendCount
        {
            get
            {
                return _heldSendCount;
            }
            private set
            {
                _heldSendCount = value;
            }
        }

        private ulong _lastSentSeq;
        private int _edgeSendCount;
        private int _heldSendCount;

        /// <summary>
        /// <paramref name="sink"/> is the seam receiving each sent
        /// client_seq (ReplicationApplier.Self wiring is a composition
        /// concern).
        /// </summary>
        public MovementInputDispatch(
            IInputSource source,
            IMovementSender sender,
            InputHistory history,
            SelfReconciliationSink sink)
        {
            _source = source ?? throw new ArgumentNullException(nameof(source));
            _sender = sender ?? throw new ArgumentNullException(nameof(sender));
            _history = history ?? throw new ArgumentNullException(nameof(history));
            _sink = sink ?? throw new ArgumentNullException(nameof(sink));
        }

        /// <summary>Stamps the predicted tick applied to subsequently recorded inputs.</summary>
        public void SetPredictedTick(ulong tick)
        {
            _pendingPredictedTick = tick;
        }

        /// <summary>
        /// One Input-phase pass (client_experience_contract.md ~L105):
        /// completed sends publish their seq to history, sampled edges go
        /// out immediately — each its own send — then held state on
        /// change/resend cadence.
        /// </summary>
        public void Dispatch(in FrameTime time)
        {
            DrainCompletedSends();
            int n = _source.SampleEdges(_edges);
            for (int i = 0; i < n; i++)
            {
                SendEdge(_edges[i]);
            }

            uint flags = _source.HeldFlags;
            int direction = _source.HeldDirection;
            bool changed = flags != _lastSentFlags;
            bool due = time.NowSeconds - _lastHeldSendSec >=
                MovementConstants.HeldSendMinIntervalSec;
            if (changed && due)
            {
                SendHeld(flags, direction, time);
            }
            else if (
                _lastSentFlags != 0 &&
                time.NowSeconds - _lastHeldSendSec >=
                    MovementConstants.HeldResendIntervalSec)
            {
                SendHeld(_lastSentFlags, _lastSentDirection, time);
            }
            else if (flags == 0 && !changed && due &&
                time.NowSeconds - _lastResendSec >=
                    MovementConstants.HeldResendIntervalSec)
            {
                _lastResendSec = time.NowSeconds;
            }
        }

        /// <summary>Cancels in-flight sends.</summary>
        public void Dispose()
        {
            _cancel.Cancel();
            _cancel.Dispose();
        }

        private void SendEdge(LocalEdge edge)
        {
            var record = new InputRecord
            {
                Kind = InputRecordKind.Edge,
                Edge = edge,
                Tick = _pendingPredictedTick,
                HeldDirection = _source.HeldDirection,
                Flags = _source.HeldFlags,
            };
            uint id;
            IMessage payload;
            if (edge == LocalEdge.Jump)
            {
                id = MovementConstants.WireC2SJump;
                payload = new C2SJump { ClientMonoMs = MonoMs() };
            }
            else if (edge == LocalEdge.Drop)
            {
                id = MovementConstants.WireC2SDropThrough;
                payload = new C2SDropThrough { ClientMonoMs = MonoMs() };
            }
            else
            {
                id = MovementConstants.WireC2SMovementEdge;
                var wire = new InputRecord { Edge = edge };
                wire.TryWireEdge(out MovementEdgeType type, out Facing direction);
                payload = new C2SMovementEdge
                {
                    EdgeType = type,
                    Direction = direction,
                    ClientMonoMs = MonoMs(),
                };
            }

            Send(id, payload, in record);
            EdgeSendCount++;
        }

        private void SendHeld(uint flags, int direction, in FrameTime time)
        {
            var record = new InputRecord
            {
                Kind = InputRecordKind.Held,
                Tick = _pendingPredictedTick,
                HeldDirection = direction,
                Flags = flags,
            };
            var payload = new C2SInputState
            {
                InputFlags = flags,
                ClientMonoMs = MonoMs(),
            };
            Send(MovementConstants.WireC2SInputState, payload, in record);
            _lastSentFlags = flags;
            _lastSentDirection = direction;
            _lastHeldSendSec = time.NowSeconds;
            HeldSendCount++;
        }

        private void Send(uint id, IMessage payload, in InputRecord record)
        {
            Awaitable<ulong> sent = _sender.SendAsync(id, payload, _cancel.Token);
            if (sent.GetAwaiter().IsCompleted)
            {
                DrainCompletedSends();
            }

            if (_pendingCount == _pending.Length)
            {
                // Over 64 sends in flight: drop the oldest record — its
                // seq correlation is lost, replay skips it.
                Log.Warn("movement input send window full; dropping oldest pending record");
                _pending[0] = default;
                Array.Copy(_pending, 1, _pending, 0, _pending.Length - 1);
                _pendingCount--;
            }

            _pending[_pendingCount++] = new PendingSend(sent, record);
        }

        /// <summary>
        /// Publishes every completed pending send (synchronous, never
        /// async void — engineering_conventions.md §2.5).
        /// </summary>
        private void DrainCompletedSends()
        {
            for (int i = 0; i < _pendingCount; i++)
            {
                Awaitable<ulong> sent = _pending[i].Awaitable;
                if (!sent.GetAwaiter().IsCompleted)
                {
                    continue;
                }

                ulong seq;
                try
                {
                    seq = sent.GetAwaiter().GetResult();
                }
                catch (OperationCanceledException)
                {
                    seq = 0UL;
                }

                if (seq != 0UL)
                {
                    Publish(in _pending[i].Record, seq);
                }

                _pending[i] = _pending[--_pendingCount];
                _pending[_pendingCount] = default;
                i--;
            }
        }

        private void Publish(in InputRecord record, ulong seq)
        {
            InputRecord r = record;
            r.Seq = seq;
            _history.Push(in r);
            _sink.NoteInputSent(seq);
            LastSentSeq = seq;
        }

        private readonly struct PendingSend
        {
            public readonly Awaitable<ulong> Awaitable;
            public readonly InputRecord Record;

            public PendingSend(Awaitable<ulong> awaitable, in InputRecord record)
            {
                Awaitable = awaitable;
                Record = record;
            }
        }

        private static ulong MonoMs()
        {
            return (ulong)(Environment.TickCount64);
        }
    }

}
