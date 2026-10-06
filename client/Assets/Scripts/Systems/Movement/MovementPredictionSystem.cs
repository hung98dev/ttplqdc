using System;
using ThinhThan.Core.Geometry;
using ThinhThan.Core.Runtime;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Systems.Movement
{
    /// <summary>
    /// Prediction-phase integrator (client_experience_contract.md ~L105 —
    /// send ≠ predict: inputs are sent by <see cref="MovementInputDispatch"/>
    /// at Input; this system only integrates the local shadow state at
    /// fixed 50 ms substeps). Runs after NetReceive so same-frame self_acks
    /// and 107s reconcile before the next substep.
    /// <para>
    /// Display-only: <see cref="State"/> is the predicted position fed to
    /// presentation; the server remains authoritative — an illegal or OOB
    /// step freezes prediction until the authoritative checkpoint lands.
    /// </para>
    /// </summary>
    public sealed class MovementPredictionSystem : IFrameSystem
    {
        private const int PendingCap = 256;

        private readonly GeometryMath.GeometryWorld _world;
        private readonly InputRecord[] _pending = new InputRecord[PendingCap];
        private int _pendingCount;
        private double _accumulator;

        /// <summary>Current predicted shadow state.</summary>
        public PredictedState State
        {
            get
            {
                return _state;
            }
        }

        /// <summary>Current predicted tick (local monotonic — never server ticks).</summary>
        public ulong CurrentTick
        {
            get
            {
                return _tick;
            }
        }

        /// <summary>Prediction frozen — waiting for the authoritative checkpoint.</summary>
        public bool Frozen
        {
            get
            {
                return _frozen;
            }
        }

        private PredictedState _state;
        private ulong _tick;
        private bool _frozen;
        private int _latestHeldDirection;

        public MovementPredictionSystem(GeometryMath.GeometryWorld world)
        {
            _world = world ?? throw new ArgumentNullException(nameof(world));
        }

        /// <summary>
        /// Queues one locally-applied input for the substeps ahead.
        /// <see cref="MovementInputDispatch"/> pushes every dispatched input
        /// here as it is sent — seq may still be 0 until the sender
        /// returns; replay only needs Tick/HeldDirection/Edge.
        /// </summary>
        public void EnqueueLocal(in InputRecord record)
        {
            if (record.Kind == InputRecordKind.Held)
            {
                _latestHeldDirection = record.HeldDirection;
            }

            if (_pendingCount == PendingCap)
            {
                Log.Warn("movement pending input buffer full; dropping oldest");
                Array.Copy(_pending, 1, _pending, 0, PendingCap - 1);
                _pendingCount--;
            }
            _pending[_pendingCount++] = record;
        }

        /// <summary>Enqueues a record during self-ack replay (same queue).</summary>
        public void ReplayInput(in InputRecord record)
        {
            EnqueueLocal(in record);
        }

        /// <summary>
        /// Restores state from an authoritative checkpoint. The local tick
        /// counter is NOT reset — predicted ticks are a local monotonic
        /// domain (SelfAck carries no server tick field; recorded input
        /// offsets stay consistent in the local domain).
        /// </summary>
        public void RestoreFrom(MovementCheckpoint checkpoint)
        {
            _state.RestoreFrom(checkpoint);
            _frozen = false;
            _pendingCount = 0;
        }

        /// <summary>
        /// Advances the shadow to <paramref name="targetTick"/>: pending
        /// inputs apply at the substep their recorded Tick reached, then
        /// the integrator steps — deterministic parity with the server.
        /// </summary>
        public void ReplayTo(ulong targetTick)
        {
            while (!_frozen && _tick < targetTick)
            {
                StepOne();
            }
        }

        /// <summary>
        /// IFrameSystem.Tick at <see cref="FramePhase.Prediction"/>: converts
        /// frame delta into fixed substeps and integrates pending inputs.
        /// No sends — the Input phase owns them.
        /// </summary>
        public void Tick(in FrameTime time)
        {
            _accumulator += time.Delta;
            double stepSec = MovementConstants.TickMillis / 1000.0;
            while (_accumulator >= stepSec)
            {
                _accumulator -= stepSec;
                if (!_frozen)
                {
                    StepOne();
                }
            }
        }

        /// <summary>Held direction the latest 100 sent (-1/0/+1).</summary>
        public int LatestHeldDirection
        {
            get
            {
                return _latestHeldDirection;
            }
        }

        private void StepOne()
        {
            ulong nextTick = _tick + 1;
            for (int i = 0; i < _pendingCount; i++)
            {
                InputRecord r = _pending[i];
                if (r.Tick > nextTick)
                {
                    continue;
                }

                if (r.Kind == InputRecordKind.Edge)
                {
                    MovementIntegrator.ApplyEdge(ref _state, r.Edge, nextTick);
                }
                else
                {
                    _latestHeldDirection = r.HeldDirection;
                }

                _pending[i] = _pending[--_pendingCount];
                _pending[_pendingCount] = default;
                i--;
            }

            if (!MovementIntegrator.Step(
                ref _state, _world, _latestHeldDirection, nextTick))
            {
                _frozen = true;
            }
            _tick = nextTick;
        }
    }
}
