using UnityEngine;

namespace ThinhThan.Core.Runtime
{
    /// <summary><see cref="IClock"/> backed by UnityEngine.Time.</summary>
    public sealed class UnityClock : IClock
    {
        public float UnscaledDeltaSeconds
        {
            get
            {
                return Time.unscaledDeltaTime;
            }
        }

        public double NowSeconds
        {
            get
            {
                return Time.realtimeSinceStartupAsDouble;
            }
        }
    }
}
