using System;
using ThinhThan.Core.Runtime;
using ThinhThan.Systems.Movement;
using UnityEngine;

namespace ThinhThan.Systems.Camera
{
    /// <summary>
    /// Camera-phase follow (physics.md §6.2): chases the predicted anchor
    /// (plus the reconciliation visual offset) through a critically damped
    /// spring τ = 0.12 s — never overshoots — clamped inside the active
    /// camera region. Exactly one camera move per frame at the Camera
    /// phase; snaps only on large jumps (map transfer / hard reconcile)
    /// or an explicit <see cref="RequestSnap"/>.
    /// </summary>
    public sealed class CameraFollowService : IFrameSystem
    {
        /// <summary>Spring time constant (§6.2): critically damped.</summary>
        public const float TauSeconds = 0.12f;

        /// <summary>
        /// Single-frame anchor jump that forces a snap — far above any
        /// legal per-frame motion, far below a map transfer.
        /// </summary>
        public const float SnapThresholdMeters = 4f;

        private const float MilliToMeters = 0.001f;
        private const long SnapThresholdMm = 4000L;

        private readonly MovementPredictionSystem _prediction;
        private readonly CameraRegionResolver _resolver;
        private readonly CameraViewModel _view;
        private readonly Func<(long X, long Y)> _displayOffset;
        private readonly Func<float> _aspect;
        private readonly UnityEngine.Camera? _camera;

        private float _posX;
        private float _posY;
        private float _velX;
        private float _velY;
        private long _anchorXMm;
        private long _anchorYMm;
        private bool _hasAnchor;
        private bool _hasPose;
        private bool _snapPending;

        /// <summary>
        /// <paramref name="displayOffsetMm"/> supplies the reconciliation
        /// visual offset (composition wires
        /// <c>() => controller.VisualOffsetMm</c>); <paramref name="camera"/>
        /// may be null in tests — the pose is then only reported via
        /// <see cref="Pose"/>.
        /// </summary>
        public CameraFollowService(
            MovementPredictionSystem prediction,
            CameraRegionResolver resolver,
            CameraViewModel view,
            UnityEngine.Camera? camera = null,
            Func<(long X, long Y)>? displayOffsetMm = null,
            Func<float>? aspect = null)
        {
            _prediction = prediction ??
                throw new ArgumentNullException(nameof(prediction));
            _resolver = resolver ??
                throw new ArgumentNullException(nameof(resolver));
            _view = view ?? throw new ArgumentNullException(nameof(view));
            _camera = camera;
            _displayOffset = displayOffsetMm ?? (() => (0L, 0L));
            _aspect = aspect ??
                (camera != null
                    ? () => camera.aspect
                    : () => CameraViewModel.ReferenceAspect);
        }

        /// <summary>Last applied camera centre in meters.</summary>
        public Vector2 Pose
        {
            get
            {
                return new Vector2(_posX, _posY);
            }
        }

        /// <summary>Transform.position writes this session — must be ≤ 1/frame.</summary>
        public int MoveCount
        {
            get;
            private set;
        }

        /// <summary>Snaps the camera to the anchor on the next tick.</summary>
        public void RequestSnap()
        {
            _snapPending = true;
        }

        /// <summary>Seeds the pose without a tick (composition bootstrap).</summary>
        public void SetPose(float xMeters, float yMeters)
        {
            _posX = xMeters;
            _posY = yMeters;
            _velX = 0f;
            _velY = 0f;
            _hasPose = true;
        }

        public void Tick(in FrameTime time)
        {
            _view.Update(_aspect());

            PredictedState state = _prediction.State;
            (long offX, long offY) = _displayOffset();
            long anchorX = state.XMm + offX;
            long anchorY = state.YMm + offY;

            _resolver.Resolve(anchorX, anchorY);
            _resolver.ClampCenter(
                anchorX, anchorY, _view.HalfWidthMm, _view.HalfHeightMm,
                out long desiredXMm, out long desiredYMm);
            float desiredX = desiredXMm * MilliToMeters;
            float desiredY = desiredYMm * MilliToMeters;

            // Snap on anchor teleports (map transfer / hard reconcile),
            // not on desired jumps — a clamp-target change when a region
            // flips must still be smoothed.
            bool anchorJump = _hasAnchor &&
                (Math.Abs(anchorX - _anchorXMm) > SnapThresholdMm ||
                    Math.Abs(anchorY - _anchorYMm) > SnapThresholdMm);
            _anchorXMm = anchorX;
            _anchorYMm = anchorY;
            _hasAnchor = true;

            if (!_hasPose || _snapPending || anchorJump)
            {
                SetPose(desiredX, desiredY);
                _snapPending = false;
            }
            else
            {
                float dt = time.Delta;
                Step(ref _posX, ref _velX, desiredX, dt);
                Step(ref _posY, ref _velY, desiredY, dt);
            }

            MoveCount++;
            if (_camera != null)
            {
                Vector3 p = _camera.transform.position;
                _camera.transform.position =
                    new Vector3(_posX, _posY, p.z);
                _camera.orthographicSize = CameraViewModel.OrthoHalfHeight;
                _camera.rect = _view.Viewport;
            }
        }

        /// <summary>
        /// Closed-form critically damped spring (no overshoot): with
        /// ω = 2/τ the error (A + B·t)·e^{−ωt} crosses zero at most once;
        /// the per-axis cross clamp makes "never" literal.
        /// </summary>
        private static void Step(
            ref float pos, ref float vel, float target, float dt)
        {
            if (dt <= 0f)
            {
                return;
            }

            float omega = 2f / TauSeconds;
            float error = pos - target;
            float b = vel + omega * error;
            float decay = Mathf.Exp(-omega * dt);
            float nextError = (error + b * dt) * decay;
            float nextPos = target + nextError;
            float nextVel = (vel - omega * b * dt) * decay;

            if (error != 0f && Mathf.Sign(nextError) != Mathf.Sign(error))
            {
                nextPos = target;
                nextVel = 0f;
            }

            pos = nextPos;
            vel = nextVel;
        }
    }
}
