using System;
using System.Collections.Generic;
using System.Diagnostics;
using UnityEngine;

namespace ThinhThan.Core.Runtime
{
    /// <summary>
    /// The single first-party logger (engineering_conventions.md §2.6 — no
    /// second logger; Debug.Log is forbidden outside this type). Dev* calls
    /// compile out unless THINHTHAN_DEV is defined. Warn/Error are always
    /// compiled and rate-limited per message so a hot path cannot spam the
    /// console.
    /// </summary>
    public static class Log
    {
        public const double RateLimitSeconds = 1.0;

        private static readonly Dictionary<string, double> _lastEmitted =
            new Dictionary<string, double>();

        private static readonly object _gate = new object();

        /// <summary>Clears rate-limit state (test isolation).</summary>
        public static void ResetForTests()
        {
            lock (_gate)
            {
                _lastEmitted.Clear();
            }
        }

        [Conditional("THINHTHAN_DEV")]
        public static void Dev(string message)
        {
            UnityEngine.Debug.unityLogger.Log(LogType.Log, message);
        }

        [Conditional("THINHTHAN_DEV")]
        public static void DevFormat(string format, params object[] args)
        {
            UnityEngine.Debug.unityLogger.LogFormat(LogType.Log, format, args);
        }

        public static void Warn(string message)
        {
            if (!Allow(message))
            {
                return;
            }

            UnityEngine.Debug.unityLogger.Log(LogType.Warning, message);
        }

        public static void Error(string message)
        {
            if (!Allow(message))
            {
                return;
            }

            UnityEngine.Debug.unityLogger.Log(LogType.Error, message);
        }

        private static bool Allow(string message)
        {
            double now = Stopwatch.GetTimestamp() / (double)Stopwatch.Frequency;
            lock (_gate)
            {
                if (_lastEmitted.TryGetValue(message, out double last) &&
                    now - last < RateLimitSeconds)
                {
                    return false;
                }

                _lastEmitted[message] = now;
                return true;
            }
        }
    }
}
