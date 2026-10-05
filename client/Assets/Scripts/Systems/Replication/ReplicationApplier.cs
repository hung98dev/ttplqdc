using System;
using System.Collections.Generic;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Replication
{
    /// <summary>
    /// Applies replication frames from the receive queue
    /// (synchronization.md): baseline swaps the whole view, spawn/despawn
    /// mutate it, deltas merge field-wise — all gated on baseline_id and
    /// monotonic server_seq. Self state is tracked separately (never in the
    /// entity arrays); self-ack drives reconciliation.
    /// </summary>
    public sealed class ReplicationApplier : IReplicationSink
    {
        // messages.md entity_state flags bitmask:
        // IN_COMBAT | DEAD | INVULNERABLE | INTERACTABLE | ELIGIBLE.
        private const uint FlagDead = 0x2U;

        private readonly EntityViewStore _store = new EntityViewStore();
        private readonly BaselineTracker _tracker = new BaselineTracker();
        private readonly SelfReconciliation _self = new SelfReconciliation();
        private readonly Dictionary<ulong, SnapshotBuffer> _snapshots =
            new Dictionary<ulong, SnapshotBuffer>();

        private readonly Func<double> _nowSeconds;
        private readonly Action<ulong>? _ackBaseline;
        private Action<ulong, ulong>? _resyncSend;

        /// <param name="nowSeconds">Render clock for interpolation buffers.</param>
        /// <param name="ackBaseline">Sends C2S_BASELINE_ACK (306).</param>
        public ReplicationApplier(
            Func<double>? nowSeconds = null,
            Action<ulong>? ackBaseline = null)
        {
            _nowSeconds = nowSeconds ?? (() => 0.0);
            _ackBaseline = ackBaseline;
        }

        public EntityViewStore Store
        {
            get
            {
                return _store;
            }
        }

        public BaselineTracker Tracker
        {
            get
            {
                return _tracker;
            }
        }

        public SelfReconciliation Self
        {
            get
            {
                return _self;
            }
        }

        /// <summary>Latest authoritative self state from baseline.</summary>
        public EntityState? SelfState
        {
            get;
            private set;
        }

        /// <summary>Latest self-private projection (mp/target).</summary>
        public SelfPrivateState? SelfPrivate
        {
            get;
            private set;
        }

        /// <summary>Latest self checkpoint (baseline or delta self_ack).</summary>
        public MovementCheckpoint? SelfCheckpoint
        {
            get;
            private set;
        }

        /// <summary>Baseline ids rejected as stale/unknown this session.</summary>
        public int InvalidBaselineCount
        {
            get;
            private set;
        }

        /// <summary>Wires the 307 sender (called by the session driver).</summary>
        public void SetResyncSender(Action<ulong, ulong> resyncSend)
        {
            _resyncSend = resyncSend;
        }

        public void Apply(DecodedFrame frame)
        {
            // Payload!: decoder guarantees payload for these mapped ids.
            switch (frame.MessageId)
            {
                case WireIds.S2CWorldBaseline:
                    ApplyBaseline((S2CWorldBaseline)frame.Payload!);
                    break;
                case WireIds.S2CEntitySpawn:
                    ApplySpawn(frame, (S2CEntitySpawn)frame.Payload!);
                    break;
                case WireIds.S2CEntityDespawn:
                    ApplyDespawn(frame, (S2CEntityDespawn)frame.Payload!);
                    break;
                case WireIds.S2CStateDelta:
                    ApplyDelta(frame, (S2CStateDelta)frame.Payload!);
                    break;
                case WireIds.S2CMovementCorrection:
                    ApplyCorrection(
                        (S2CMovementCorrection)frame.Payload!);
                    break;
                case WireIds.S2CDeath:
                    ApplyDeath((S2CDeath)frame.Payload!);
                    break;
                case WireIds.S2CRespawn:
                    ApplyRespawn((S2CRespawn)frame.Payload!);
                    break;
                case WireIds.S2CBaselineResyncResult:
                    ApplyResyncResult(
                        (S2CBaselineResyncResult)frame.Payload!);
                    break;
                default:
                    break;
            }
        }

        /// <summary>
        /// Interpolated render position for entity index
        /// <paramref name="index"/> (see <see cref="EntityViewStore.Ids"/>).
        /// </summary>
        public (double XMm, double YMm) RenderPosition(int index)
        {
            ulong id = _store.Ids[index];
            if (!_snapshots.TryGetValue(id, out SnapshotBuffer? buffer))
            {
                EntityState? state = _store.States[index];
                return state == null ? (0.0, 0.0) : (state.XMm, state.YMm);
            }

            return buffer.Interpolate(_nowSeconds());
        }

        /// <summary>Requests a resync if the rate gate allows it.</summary>
        public bool TryRequestResync(long nowMs)
        {
            ulong requestId = (ulong)(nowMs < 0 ? -nowMs : nowMs) + 1UL;
            if (!_tracker.TryBeginResync(requestId, nowMs) ||
                _resyncSend == null)
            {
                return false;
            }

            _resyncSend(requestId, _tracker.CurrentBaseline);
            return true;
        }

        public void Invalidate()
        {
            _store.Clear();
            _tracker.Invalidate();
            _self.Reset();
            SelfState = null;
            SelfPrivate = null;
            SelfCheckpoint = null;
        }

        private void ApplyBaseline(S2CWorldBaseline baseline)
        {
            _tracker.AcceptBaseline(baseline.BaselineId);
            _store.Clear();
            SelfState = baseline.Self;
            SelfPrivate = baseline.SelfPrivate;
            SelfCheckpoint = baseline.SelfCheckpoint;
            double now = _nowSeconds();
            for (int i = 0; i < baseline.Entities.Count; i++)
            {
                EntityState entity = baseline.Entities[i];
                _store.Upsert(entity);
                PushBaselineSnapshot(entity, baseline.ServerTick, now);
            }

            if (_ackBaseline != null)
            {
                _ackBaseline(baseline.BaselineId);
            }
        }

        private void ApplySpawn(DecodedFrame frame, S2CEntitySpawn spawn)
        {
            if (!Legal(frame))
            {
                return;
            }

            _store.Upsert(spawn.Entity);
            PushSnapshot(spawn.Entity, spawn.ServerTick, _nowSeconds());
            _tracker.NoteApplied(frame);
        }

        private void ApplyDespawn(
            DecodedFrame frame, S2CEntityDespawn despawn)
        {
            if (!Legal(frame))
            {
                return;
            }

            _store.Remove(despawn.EntityId);
            _snapshots.Remove(despawn.EntityId);
            _tracker.NoteApplied(frame);
        }

        private void ApplyDelta(DecodedFrame frame, S2CStateDelta delta)
        {
            if (!Legal(frame))
            {
                return;
            }

            double now = _nowSeconds();
            for (int i = 0; i < delta.Entities.Count; i++)
            {
                EntityDelta entityDelta = delta.Entities[i];
                if (_store.ApplyDelta(entityDelta) &&
                    _store.TryGetIndex(entityDelta.EntityId, out int index))
                {
                    EntityState? state = _store.States[index];
                    if (state != null)
                    {
                        PushSnapshot(state, delta.ServerTick, now);
                    }
                }
            }

            SelfAck? ack = delta.SelfAck;
            if (ack != null)
            {
                if (ack.Checkpoint != null)
                {
                    SelfCheckpoint = ack.Checkpoint;
                }

                int shownX = SelfCheckpoint != null ? SelfCheckpoint.XMm : 0;
                int shownY = SelfCheckpoint != null ? SelfCheckpoint.YMm : 0;
                _self.ApplyAck(ack, shownX, shownY);
            }

            if (delta.SelfPrivate != null)
            {
                ApplySelfPrivateDelta(delta.SelfPrivate);
            }

            _tracker.NoteApplied(frame);
        }

        private void ApplyCorrection(S2CMovementCorrection correction)
        {
            if (correction.Checkpoint != null)
            {
                SelfCheckpoint = correction.Checkpoint;
                _self.ForceSnap(correction.Checkpoint);
            }
        }

        private void ApplyDeath(S2CDeath death)
        {
            if (SelfState != null &&
                death.EntityId == SelfState.EntityId)
            {
                SelfState.Flags |= FlagDead;
            }
        }

        private void ApplyRespawn(S2CRespawn respawn)
        {
            if (SelfState != null &&
                respawn.EntityId == SelfState.EntityId)
            {
                SelfState.Flags &= ~FlagDead;
                SelfState.XMm = respawn.XMm;
                SelfState.YMm = respawn.YMm;
                SelfState.Hp = respawn.HpAfter;
            }
        }

        private void ApplyResyncResult(S2CBaselineResyncResult result)
        {
            _tracker.TryCompleteResync(result.RequestId);
        }

        private void ApplySelfPrivateDelta(SelfPrivateDelta delta)
        {
            SelfPrivateState? current = SelfPrivate;
            if (current == null)
            {
                current = new SelfPrivateState();
                SelfPrivate = current;
            }

            if (delta.HasCurrentMp)
            {
                current.CurrentMp = delta.CurrentMp;
            }

            if (delta.HasMaxMp)
            {
                current.MaxMp = delta.MaxMp;
            }

            if (delta.HasAcceptedTargetEntityId)
            {
                current.AcceptedTargetEntityId =
                    delta.AcceptedTargetEntityId;
            }
        }

        private bool Legal(DecodedFrame frame)
        {
            if (frame.BaselineId != _tracker.CurrentBaseline)
            {
                InvalidBaselineCount++;
                return false;
            }

            if (!_tracker.IsLegalDelta(frame))
            {
                return false;
            }

            return true;
        }

        private void PushBaselineSnapshot(
            EntityState entity, ulong serverTick, double nowSeconds)
        {
            SnapshotBuffer? buffer = GetOrCreateBuffer(entity.EntityId);
            buffer.Clear();
            buffer.Push(
                serverTick, entity.XMm, entity.YMm,
                entity.VxMmS, entity.VyMmS, nowSeconds);
        }

        private void PushSnapshot(
            EntityState entity, ulong serverTick, double nowSeconds)
        {
            SnapshotBuffer? buffer = GetOrCreateBuffer(entity.EntityId);
            buffer.Push(
                serverTick, entity.XMm, entity.YMm,
                entity.VxMmS, entity.VyMmS, nowSeconds);
        }

        /// <summary>Get-or-create keyed on entity id: buffers survive
        /// baselines so a baseline swap does not allocate (PERF-024).
        /// Buffers of entities absent from the new baseline simply idle.
        /// </summary>
        private SnapshotBuffer GetOrCreateBuffer(ulong entityId)
        {
            if (!_snapshots.TryGetValue(entityId, out SnapshotBuffer? buffer))
            {
                buffer = new SnapshotBuffer();
                _snapshots[entityId] = buffer;
            }

            return buffer;
        }
    }
}
