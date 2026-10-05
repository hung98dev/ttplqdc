using System;
using UnityEngine;

namespace ThinhThan.Core.Runtime
{
    /// <summary>
    /// The only first-party type holding Unity frame callbacks
    /// (client_performance.md § Smoothness by Construction). It drives the
    /// fixed phase order <see cref="FramePhase.Input"/> through
    /// <see cref="FramePhase.UI"/> from Update and
    /// <see cref="FramePhase.Camera"/> from LateUpdate. Systems register at
    /// composition or map-load time only; registering while a tick is running
    /// throws.
    /// </summary>
    public sealed class FrameLoop : MonoBehaviour
    {
        private const int PhaseCount = 7;

        private readonly IFrameSystem?[] _systems = new IFrameSystem[PhaseCount];
        private readonly bool[] _occupied = new bool[PhaseCount];
        private IClock _clock = new UnityClock();
        private long _frameIndex;
        private bool _ticking;

        /// <summary>
        /// Replaces the time source. Allowed only outside a tick; intended for
        /// tests and composition-time wiring.
        /// </summary>
        public void SetClock(IClock clock)
        {
            if (clock == null)
            {
                throw new ArgumentNullException(nameof(clock));
            }

            ThrowIfTicking();
            _clock = clock;
        }

        /// <summary>
        /// Registers <paramref name="system"/> as the single occupant of
        /// <paramref name="phase"/>. One system per phase; call only at
        /// composition or map load — never from inside a
        /// <see cref="IFrameSystem.Tick"/>.
        /// </summary>
        public void Register(FramePhase phase, IFrameSystem system)
        {
            if (system == null)
            {
                throw new ArgumentNullException(nameof(system));
            }

            ThrowIfTicking();
            int index = (int)phase;
            if (index < 0 || index >= PhaseCount)
            {
                throw new ArgumentOutOfRangeException(nameof(phase));
            }

            if (_occupied[index])
            {
                throw new InvalidOperationException(
                    "FramePhase already occupied: " + phase);
            }

            _systems[index] = system;
            _occupied[index] = true;
        }

        /// <summary>Removes the system occupying <paramref name="phase"/>.</summary>
        public void Unregister(FramePhase phase)
        {
            ThrowIfTicking();
            int index = (int)phase;
            if (index < 0 || index >= PhaseCount)
            {
                throw new ArgumentOutOfRangeException(nameof(phase));
            }

            _systems[index] = null;
            _occupied[index] = false;
        }

        /// <summary>
        /// Runs one frame: Update phases (Input..UI) then the Camera phase.
        /// Called by Unity's Update/LateUpdate pair and directly by tests.
        /// </summary>
        public void TickOnce()
        {
            var time = new FrameTime(
                _clock.UnscaledDeltaSeconds, _clock.NowSeconds, _frameIndex++);
            TickUpdatePhases(in time);
            TickCameraPhase(in time);
        }

        /// <summary>Runs only the Update phases (Input..UI).</summary>
        public void TickUpdatePhases(in FrameTime time)
        {
            _ticking = true;
            try
            {
                for (int i = (int)FramePhase.Input; i <= (int)FramePhase.UI; i++)
                {
                    _systems[i]?.Tick(in time);
                }
            }
            finally
            {
                _ticking = false;
            }
        }

        /// <summary>Runs only the LateUpdate phase (Camera).</summary>
        public void TickCameraPhase(in FrameTime time)
        {
            _ticking = true;
            try
            {
                _systems[(int)FramePhase.Camera]?.Tick(in time);
            }
            finally
            {
                _ticking = false;
            }
        }

        private void Update()
        {
            var time = new FrameTime(
                _clock.UnscaledDeltaSeconds, _clock.NowSeconds, _frameIndex++);
            TickUpdatePhases(in time);
        }

        private void LateUpdate()
        {
            var time = new FrameTime(
                _clock.UnscaledDeltaSeconds, _clock.NowSeconds, _frameIndex);
            TickCameraPhase(in time);
        }

        private void ThrowIfTicking()
        {
            if (_ticking)
            {
                throw new InvalidOperationException(
                    "FrameLoop registration is composition-time only; " +
                    "systems cannot register or unregister during Tick.");
            }
        }
    }
}
