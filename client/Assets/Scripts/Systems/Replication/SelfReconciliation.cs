using System.Collections.Generic;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Replication
{
    /// <summary>
    /// Authoritative-self tracking (synchronization.md § Local
    /// Reconciliation): retains the newest complete SelfAck (server_tick,
    /// last_processed_client_seq, MovementCheckpoint) plus the unacknowledged
    /// input client_seqs so a later prediction step can drop inputs at or
    /// below the ack and replay the rest. A S2C_MOVEMENT_CORRECTION (107)
    /// forces a snap.
    /// </summary>
    public sealed class SelfReconciliation
    {
        /// <summary>Correction distance threshold: 500 mm squared.</summary>
        public const long SnapThresholdSqMm = 500L * 500L;

        private readonly List<ulong> _pendingInputSeqs = new List<ulong>();
        private SelfAck? _latestAck;
        private MovementCheckpoint? _forcedSnap;
        private long _snapErrorSqMm;

        /// <summary>Newest complete SelfAck retained (never coalesced).</summary>
        public SelfAck? LatestAck
        {
            get
            {
                return _latestAck;
            }
        }

        /// <summary>Checkpoint a 107 forced; cleared once consumed.</summary>
        public MovementCheckpoint? ForcedSnap
        {
            get
            {
                return _forcedSnap;
            }
        }

        /// <summary>
        /// Squared mm error between the displayed position and the ack
        /// checkpoint at the last ack apply — smooth-correct over 100 ms when
        /// &lt;= <see cref="SnapThresholdSqMm"/>, snap above.
        /// </summary>
        public long SnapErrorSqMm
        {
            get
            {
                return _snapErrorSqMm;
            }
        }

        /// <summary>Unacked input client_seqs awaiting reconciliation.</summary>
        public IReadOnlyList<ulong> PendingInputSeqs
        {
            get
            {
                return _pendingInputSeqs;
            }
        }

        /// <summary>Called for every outbound realtime input.</summary>
        public void NoteInputSent(ulong clientSeq)
        {
            _pendingInputSeqs.Add(clientSeq);
        }

        /// <summary>
        /// Applies the newest SelfAck: drops inputs at or below
        /// last_processed_client_seq, stores the complete checkpoint, and
        /// records the squared-mm error vs the displayed position.
        /// </summary>
        public void ApplyAck(
            SelfAck ack, int displayedXMm, int displayedYMm)
        {
            _latestAck = ack;
            int drop = 0;
            while (drop < _pendingInputSeqs.Count &&
                _pendingInputSeqs[drop] <= ack.LastProcessedClientSeq)
            {
                drop++;
            }

            if (drop > 0)
            {
                _pendingInputSeqs.RemoveRange(0, drop);
            }

            MovementCheckpoint? checkpoint = ack.Checkpoint;
            if (checkpoint != null)
            {
                long dx = displayedXMm - checkpoint.XMm;
                long dy = displayedYMm - checkpoint.YMm;
                _snapErrorSqMm = dx * dx + dy * dy;
            }
        }

        /// <summary>S2C_MOVEMENT_CORRECTION (107): forced snap, no lerp.</summary>
        public void ForceSnap(MovementCheckpoint checkpoint)
        {
            _forcedSnap = checkpoint;
            _pendingInputSeqs.Clear();
        }

        /// <summary>Clears the forced snap after the presentation applies it.</summary>
        public void ConsumeForcedSnap()
        {
            _forcedSnap = null;
        }

        public void Reset()
        {
            _latestAck = null;
            _forcedSnap = null;
            _snapErrorSqMm = 0L;
            _pendingInputSeqs.Clear();
        }
    }
}
