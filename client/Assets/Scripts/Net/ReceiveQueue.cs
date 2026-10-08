using System;
using System.Collections.Generic;
using ThinhThan.Core.Runtime;

namespace ThinhThan.Net
{
    /// <summary>
    /// Bounded client receive queue (protocol.md § Client Receive Queue and
    /// Lease Ownership): at most <see cref="MaxFrames"/> live entries AND
    /// <see cref="MaxBytes"/> accounted bytes. REPLACEABLE_STATE entries
    /// supersede in place keyed by (message_id, entity key) inside the same
    /// barrier ordinal — never across baseline/lifecycle barriers. Control,
    /// authoritative-event, spawn/despawn and baseline frames never coalesce.
    /// The storage is a fixed ring — enqueue/dequeue/lease-release allocate
    /// nothing after construction (PERF-024).
    /// </summary>
    public sealed class ReceiveQueue
    {
        public const int MaxFrames = 256;

        public const int MaxBytes = 4 * 1024 * 1024;

        /// <summary>
        /// Message ids that are REPLACEABLE_STATE whole-frame replaceables,
        /// keyed by entity (0 = whole state). AUTHORITATIVE_EVENT,
        /// DURABLE_RESULT, CONTROL, spawn/despawn/baselines are never here.
        /// </summary>
        private static readonly HashSet<uint> _replaceable =
            new HashSet<uint>
            {
                WireIds.S2CCharacterList,
                WireIds.S2CWalletState,
                WireIds.S2CInventoryState,
                WireIds.S2CRewardClaimsState,
                WireIds.S2CEntitlementPanelState,
                WireIds.S2CProgressionState,
                WireIds.S2CPartyState,
                WireIds.S2CBlockState,
                WireIds.S2CPartyBoardState,
                WireIds.S2CTradeOfferState,
                WireIds.S2CAuctionMyState,
            };

        /// <summary>Message ids that advance the barrier ordinal.</summary>
        private static readonly HashSet<uint> _barriers =
            new HashSet<uint>
            {
                WireIds.S2CHelloOk,
                WireIds.S2CCharacterAttachOk,
                WireIds.S2CSessionReplaced,
                WireIds.S2CCharacterDetachOk,
                WireIds.S2CTransferPrepare,
                WireIds.S2CDeath,
                WireIds.S2CRespawn,
                WireIds.S2CWorldBaseline,
            };

        private readonly DecodedFrame?[] _ring = new DecodedFrame?[MaxFrames];

        /// <summary>Frame-object pool shared between producer and consumer
        /// so the steady-state framing path allocates nothing.</summary>
        private readonly Pool<DecodedFrame> _framePool =
            new Pool<DecodedFrame>(
                () => new DecodedFrame(), maxRetained: MaxFrames);

        private readonly Dictionary<SupersedeKey, DecodedFrame>
            _replaceableIndex = new Dictionary<SupersedeKey, DecodedFrame>();

        private readonly object _gate = new object();
        private int _head;
        private int _count;
        private int _accountedBytes;
        private int _generation;
        private int _barrierOrdinal;
        private long _nextSequence;

        /// <summary>Raised once when an enqueue fails on the bounds.</summary>
        public event Action? Exhausted;

        /// <summary>Current connection_generation stamped on new entries.</summary>
        public int Generation
        {
            get
            {
                return _generation;
            }
        }

        public int Count
        {
            get
            {
                lock (_gate)
                {
                    return _count;
                }
            }
        }

        public int AccountedBytes
        {
            get
            {
                lock (_gate)
                {
                    return _accountedBytes;
                }
            }
        }

        /// <summary>
        /// Starts a new connection generation: bumps the generation counter,
        /// releases every queued entry, and resets the barrier ordinal. The
        /// caller cancels/joins the old producer before calling.
        /// </summary>
        public void BeginGeneration()
        {
            lock (_gate)
            {
                _generation++;
                _barrierOrdinal = 0;
                _nextSequence = 0L;
                for (int i = 0; i < _count; i++)
                {
                    _ring[Physical(i)]?.Reset();
                    _ring[Physical(i)] = null;
                }

                _head = 0;
                _count = 0;
                _replaceableIndex.Clear();
                _accountedBytes = 0;
            }
        }

        /// <summary>
        /// Enqueues a decoded frame. On failure (bounds exceeded) the frame is
        /// reset and <see cref="Exhausted"/> fires — the driver must stop the
        /// receiver, close/reconnect, invalidate baseline and resync.
        /// </summary>
        public bool TryEnqueue(DecodedFrame frame)
        {
            lock (_gate)
            {
                if (_count >= MaxFrames ||
                    _accountedBytes + frame.AccountedBytes > MaxBytes)
                {
                    frame.Reset();
                    _framePool.Return(frame);
                    if (Exhausted != null)
                    {
                        Exhausted();
                    }
                    return false;
                }

                frame.ConnectionGeneration = _generation;
                frame.Sequence = _nextSequence++;
                if (_barriers.Contains(frame.MessageId))
                {
                    _barrierOrdinal++;
                }

                frame.BarrierOrdinal = _barrierOrdinal;
                if (_replaceable.Contains(frame.MessageId))
                {
                    var key = new SupersedeKey(
                        frame.MessageId, frame.BaselineId,
                        frame.BarrierOrdinal);
                    if (_replaceableIndex.TryGetValue(
                        key, out DecodedFrame? old))
                    {
                        int oldIndex = IndexOf(old);
                        if (oldIndex >= 0)
                        {
                            _accountedBytes -= old.AccountedBytes;
                            _ring[Physical(oldIndex)] = frame;
                            _replaceableIndex[key] = frame;
                            _accountedBytes += frame.AccountedBytes;
                            old.Reset();
                            _framePool.Return(old);
                            return true;
                        }
                    }

                    _replaceableIndex[key] = frame;
                }

                _ring[Physical(_count)] = frame;
                _count++;
                _accountedBytes += frame.AccountedBytes;
                return true;
            }
        }

        /// <summary>Dequeues the oldest entry under a single-owner lease.</summary>
        public bool TryDequeue(out ReceiveLease lease)
        {
            lock (_gate)
            {
                if (_count == 0)
                {
                    lease = default;
                    return false;
                }

                // slot non-null: _count>0 means _ring[_head] holds a frame.
                DecodedFrame frame = _ring[_head]!;
                _ring[_head] = null;
                _head = (_head + 1) % MaxFrames;
                _count--;
                if (_replaceable.Contains(frame.MessageId))
                {
                    _replaceableIndex.Remove(
                        new SupersedeKey(
                            frame.MessageId, frame.BaselineId,
                            frame.BarrierOrdinal));
                }

                _accountedBytes -= frame.AccountedBytes;
                lease = new ReceiveLease(this, frame);
                return true;
            }
        }

        /// <summary>Producer-side frame checkout (called by the pump).</summary>
        public DecodedFrame RentFrame()
        {
            return _framePool.Rent();
        }

        /// <summary>Releases a dequeued entry (called by the lease).</summary>
        internal void Release(DecodedFrame frame)
        {
            frame.Reset();
            _framePool.Return(frame);
        }

        /// <summary>Physical ring slot of the i-th logical entry.</summary>
        private int Physical(int logicalIndex)
        {
            return (_head + logicalIndex) % MaxFrames;
        }

        /// <summary>Logical index of a live entry; -1 when absent.</summary>
        private int IndexOf(DecodedFrame frame)
        {
            for (int i = 0; i < _count; i++)
            {
                if (ReferenceEquals(_ring[Physical(i)], frame))
                {
                    return i;
                }
            }

            return -1;
        }

        private readonly struct SupersedeKey : IEquatable<SupersedeKey>
        {
            private readonly uint _messageId;
            private readonly ulong _baselineId;
            private readonly int _barrierOrdinal;

            public SupersedeKey(uint messageId, ulong baselineId, int barrierOrdinal)
            {
                _messageId = messageId;
                _baselineId = baselineId;
                _barrierOrdinal = barrierOrdinal;
            }

            public bool Equals(SupersedeKey other)
            {
                return _messageId == other._messageId &&
                    _baselineId == other._baselineId &&
                    _barrierOrdinal == other._barrierOrdinal;
            }

            public override bool Equals(object? obj)
            {
                return obj is SupersedeKey other && Equals(other);
            }

            public override int GetHashCode()
            {
                return HashCode.Combine(
                    _messageId, _baselineId, _barrierOrdinal);
            }
        }
    }
}
