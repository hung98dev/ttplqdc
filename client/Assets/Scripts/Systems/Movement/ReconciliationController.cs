using System;
using ThinhThan.Core.Geometry;
using ThinhThan.Protocol.V1;
using ThinhThan.Systems.Replication;

namespace ThinhThan.Systems.Movement
{
    /// <summary>
    /// Self-ack reconciliation (synchronization.md § Local Reconciliation):
    /// on each SelfAck — capture displayed position → drop inputs at or
    /// below <c>last_processed_client_seq</c> → restore the complete
    /// checkpoint → replay newer inputs at their recorded local ticks →
    /// compare Δr² against <see cref="MovementConstants.SnapThresholdSqMm"/>
    /// (≤ → 100 ms smooth; &gt; → snap). A S2C_MOVEMENT_CORRECTION (107)
    /// always snaps then replays. Baseline replace resets history.
    /// <para>
    /// Smoothing is a visual offset only — it never feeds physics
    /// (movement.md L87–91).
    /// </para>
    /// </summary>
    public sealed class ReconciliationController
    {
        private readonly MovementPredictionSystem _prediction;
        private SelfAck? _seenAck;
        private long _smoothEndTick;
        private long _smoothStartX;
        private long _smoothStartY;
        private long _smoothTargetX;
        private long _smoothTargetY;
        private ulong _smoothBaseTick;

        /// <summary>Active visual offset in mm, or (0,0) outside smoothing.</summary>
        public (long X, long Y) VisualOffsetMm
        {
            get;
            private set;
        }

        public ReconciliationController(MovementPredictionSystem prediction)
        {
            _prediction = prediction ??
                throw new ArgumentNullException(nameof(prediction));
        }

        /// <summary>
        /// Polls the applier's newest ack/forced-snap each Prediction phase
        /// (NetReceive ran earlier the same frame). Returns true when the
        /// displayed state changed.
        /// </summary>
        public bool Process(SelfReconciliation self, InputHistory history)
        {
            bool dirty = false;

            MovementCheckpoint? snap = self.ForcedSnap;
            if (snap != null)
            {
                // SelfReconciliation.ForceSnap keeps only the checkpoint
                // (IMP-065 seam) — fall back to the newest ack's seq for
                // the replay base; without any ack, replay all pending.
                SelfAck? snapAck = self.LatestAck;
                ulong liveTick = _prediction.CurrentTick;
                _prediction.RestoreFrom(snap);
                ulong lastSeq =
                    snapAck != null ? snapAck.LastProcessedClientSeq : 0UL;
                history.DropThrough(lastSeq);
                ReplayFrom(history, lastSeq, liveTick);
                self.ConsumeForcedSnap();
                VisualOffsetMm = (0, 0);
                _smoothEndTick = 0;
                dirty = true;
            }

            SelfAck? ack = self.LatestAck;
            if (ack != null && !ReferenceEquals(ack, _seenAck))
            {
                _seenAck = ack;
                MovementCheckpoint? cp = ack.Checkpoint;
                if (cp != null)
                {
                    ulong liveTick = _prediction.CurrentTick;
                    history.DropThrough(ack.LastProcessedClientSeq);
                    int displayX = _prediction.State.XMm;
                    int displayY = _prediction.State.YMm;
                    _prediction.RestoreFrom(cp);
                    ReplayFrom(history, ack.LastProcessedClientSeq, liveTick);

                    long dx = displayX - _prediction.State.XMm;
                    long dy = displayY - _prediction.State.YMm;
                    long errSq = dx * dx + dy * dy;
                    if (errSq <= MovementConstants.SnapThresholdSqMm)
                    {
                        // Smooth: keep displaying the pre-ack position and
                        // decay the offset over 100 ms — physics untouched.
                        _smoothStartX = displayX;
                        _smoothStartY = displayY;
                        _smoothTargetX = _prediction.State.XMm;
                        _smoothTargetY = _prediction.State.YMm;
                        _smoothBaseTick = _prediction.CurrentTick;
                        _smoothEndTick = _smoothBaseTick +
                            (ulong)(MovementConstants.SmoothMs /
                                MovementConstants.TickMillis);
                    }
                    else
                    {
                        VisualOffsetMm = (0, 0);
                        _smoothEndTick = 0;
                    }
                    dirty = true;
                }
            }

            if (_smoothEndTick != 0 && _prediction.CurrentTick < _smoothEndTick)
            {
                long span = (long)(_smoothEndTick - _smoothBaseTick);
                long left = (long)(_smoothEndTick - _prediction.CurrentTick);
                VisualOffsetMm = (
                    (_smoothStartX - _smoothTargetX) * left / span,
                    (_smoothStartY - _smoothTargetY) * left / span);
            }
            else if (_smoothEndTick != 0)
            {
                VisualOffsetMm = (0, 0);
                _smoothEndTick = 0;
            }

            return dirty;
        }

        /// <summary>A new world baseline replaced the state — wipe history.</summary>
        public void ResetForBaseline(InputHistory history)
        {
            history.Clear();
            _seenAck = null;
            VisualOffsetMm = (0, 0);
            _smoothEndTick = 0;
        }

        /// <summary>
        /// Replays inputs newer than <paramref name="afterSeq"/> through the
        /// integrator, advancing the shadow back to <paramref name="liveTick"/>
        /// (the tick count captured before the restore).
        /// </summary>
        private void ReplayFrom(
            InputHistory history, ulong afterSeq, ulong liveTick)
        {
            history.ForEachAfter(afterSeq, Replay);
            _prediction.ReplayTo(liveTick);
        }

        private void Replay(in InputRecord record)
        {
            _prediction.ReplayInput(in record);
        }
    }
}
