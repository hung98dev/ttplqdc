using System;
using ThinhThan.Net;

namespace ThinhThan.Systems.Replication
{
    /// <summary>
    /// Baseline bookkeeping (synchronization.md): every replication frame
    /// carries baseline_id; deltas are legal only against the current
    /// baseline, spawn/despawn/delta server_seq must be monotonic, and resync
    /// (307) is limited to one outstanding request per 5 s.
    /// </summary>
    public sealed class BaselineTracker
    {
        public const long ResyncCooldownMs = 5000;

        private ulong _currentBaseline;
        private ulong _lastServerSeq;
        private ulong _resyncRequestId;
        private long _lastResyncAtMs = -ResyncCooldownMs;
        private bool _resyncOutstanding;

        public ulong CurrentBaseline
        {
            get
            {
                return _currentBaseline;
            }
        }

        public ulong LastServerSeq
        {
            get
            {
                return _lastServerSeq;
            }
        }

        /// <summary>A resync request is awaiting its 308 result.</summary>
        public bool ResyncOutstanding
        {
            get
            {
                return _resyncOutstanding;
            }
        }

        /// <summary>
        /// Resync rate gate: at most one request outstanding, at most one per
        /// 5 s per connection.
        /// </summary>
        public bool TryBeginResync(ulong requestId, long nowMs)
        {
            if (_resyncOutstanding ||
                nowMs - _lastResyncAtMs < ResyncCooldownMs)
            {
                return false;
            }

            _resyncOutstanding = true;
            _resyncRequestId = requestId;
            _lastResyncAtMs = nowMs;
            return true;
        }

        /// <summary>308 observed: clears the outstanding flag.</summary>
        public bool TryCompleteResync(ulong requestId)
        {
            if (!_resyncOutstanding || _resyncRequestId != requestId)
            {
                return false;
            }

            _resyncOutstanding = false;
            return true;
        }

        /// <summary>New baseline accepted: deltas now apply against it.</summary>
        public void AcceptBaseline(ulong baselineId)
        {
            _currentBaseline = baselineId;
            _lastServerSeq = 0UL;
            _resyncOutstanding = false;
        }

        /// <summary>Baseline invalidated (reconnect/queue exhaustion).</summary>
        public void Invalidate()
        {
            _currentBaseline = 0UL;
            _lastServerSeq = 0UL;
            _resyncOutstanding = false;
        }

        /// <summary>
        /// Frame legality for replication messages: baseline_id must match the
        /// current baseline and server_seq must be strictly newer.
        /// </summary>
        public bool IsLegalDelta(DecodedFrame frame)
        {
            if (frame.BaselineId != _currentBaseline)
            {
                return false;
            }

            if (frame.ServerSeq <= _lastServerSeq)
            {
                return false;
            }

            return true;
        }

        public void NoteApplied(DecodedFrame frame)
        {
            if (frame.ServerSeq > _lastServerSeq)
            {
                _lastServerSeq = frame.ServerSeq;
            }
        }
    }
}
