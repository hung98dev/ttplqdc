using System;
using ThinhThan.Core.Session;
using UnityEngine;

namespace ThinhThan.Core.Performance
{
    /// <summary>
    /// Battery-saver toggle (client_performance.md PERF-013): when on,
    /// <see cref="Application.targetFrameRate"/> is capped at 30 on every
    /// quality tier; when off the platform/tier baseline applies. The flag
    /// persists through <see cref="ISessionStorage"/>.
    /// </summary>
    public sealed class BatterySaver
    {
        /// <summary>Session-storage key for the persisted toggle.</summary>
        public const string PersistedKey = "quality.battery_saver";

        /// <summary>Frame-rate cap applied while enabled.</summary>
        public const int CapFrameRate = 30;

        /// <summary>Unset frame-rate value meaning "let vSync decide".</summary>
        public const int UnsetFrameRate = -1;

        private readonly ISessionStorage? _storage;
        private int _baselineFrameRate = UnsetFrameRate;
        private bool _enabled;

        /// <param name="storage">Optional persistence seam; null keeps the flag in memory.</param>
        public BatterySaver(ISessionStorage? storage = null)
        {
            _storage = storage;
            _enabled = storage != null &&
                storage.TryGet(PersistedKey, out string? raw) &&
                raw == "1";
            Apply();
        }

        /// <summary>Persisted toggle; writes through on change.</summary>
        public bool Enabled
        {
            get
            {
                return _enabled;
            }
            set
            {
                if (_enabled == value)
                {
                    return;
                }

                _enabled = value;
                _storage?.Set(PersistedKey, value ? "1" : "0");
                Apply();
            }
        }

        /// <summary>
        /// Records the platform/tier frame-rate baseline the toggle falls
        /// back to while disabled (desktop <see cref="UnsetFrameRate"/>,
        /// Android the tier target).
        /// </summary>
        public void SetBaselineFrameRate(int fps)
        {
            _baselineFrameRate = fps;
            Apply();
        }

        /// <summary>Effective target frame rate under the current flag.</summary>
        public int EffectiveFrameRate
        {
            get
            {
                return _enabled ? CapFrameRate : _baselineFrameRate;
            }
        }

        private void Apply()
        {
            Application.targetFrameRate = EffectiveFrameRate;
        }
    }
}
