using System;
using System.Collections.Generic;
using UnityEngine;
using UnityEngine.Rendering.Universal;

namespace ThinhThan.Core.Rendering
{
    /// <summary>
    /// Active-in-view point Light2D budget (client.md § Rendering;
    /// client_performance.md § Platforms and Device Tiers): at most LOW 4 /
    /// MEDIUM 8 / HIGH 16 point lights are enabled, nearest to the view center
    /// first. The composition root owns one instance per map; the FrameLoop
    /// Presentation phase calls <see cref="ApplyBudget(QualityPreset, Vector2)"/>
    /// once per frame. No Unity frame callbacks.
    /// </summary>
    public sealed class PointLightBudget
    {
        /// <summary>LOW preset active-light cap.</summary>
        public const int LowLimit = 4;

        /// <summary>MEDIUM preset active-light cap.</summary>
        public const int MediumLimit = 8;

        /// <summary>HIGH preset active-light cap.</summary>
        public const int HighLimit = 16;

        private readonly List<Light2D> _lights = new List<Light2D>(32);
        private int[] _order = new int[32];

        /// <summary>Active point-light cap for a quality preset (LOW 4 / MEDIUM 8 / HIGH 16).</summary>
        public static int ActivePointLightLimit(QualityPreset preset)
        {
            switch (preset)
            {
                case QualityPreset.Low:
                    return LowLimit;
                case QualityPreset.Medium:
                    return MediumLimit;
                case QualityPreset.High:
                    return HighLimit;
                default:
                    throw new ArgumentOutOfRangeException(nameof(preset), preset, null);
            }
        }

        /// <summary>Number of registered lights.</summary>
        public int Count => _lights.Count;

        /// <summary>Registers a point light; returns false when null or already registered.</summary>
        public bool Register(Light2D light)
        {
            if (light == null || _lights.Contains(light))
            {
                return false;
            }
            _lights.Add(light);
            return true;
        }

        /// <summary>Unregisters a point light; returns false when null or not registered.</summary>
        public bool Unregister(Light2D light)
        {
            if (light == null)
            {
                return false;
            }
            return _lights.Remove(light);
        }

        /// <summary>
        /// Enables the nearest <paramref name="limit"/> registered lights to
        /// <paramref name="viewCenter"/> and disables the rest. Ordering is
        /// deterministic: squared distance ascending, then registration order.
        /// Destroyed lights are pruned from the registry. Returns the number of
        /// lights left enabled.
        /// </summary>
        public int ApplyBudget(int limit, Vector2 viewCenter)
        {
            if (limit < 0)
            {
                throw new ArgumentOutOfRangeException(nameof(limit));
            }
            for (int i = _lights.Count - 1; i >= 0; i--)
            {
                if (_lights[i] == null)
                {
                    _lights.RemoveAt(i);
                }
            }
            int n = _lights.Count;
            if (_order.Length < n)
            {
                _order = new int[n * 2];
            }
            for (int i = 0; i < n; i++)
            {
                _order[i] = i;
            }
            for (int i = 1; i < n; i++)
            {
                int key = _order[i];
                float keyDistance = DistanceSquared(_lights[key], viewCenter);
                int j = i - 1;
                while (j >= 0 && DistanceSquared(_lights[_order[j]], viewCenter) > keyDistance)
                {
                    _order[j + 1] = _order[j];
                    j--;
                }
                _order[j + 1] = key;
            }
            int enabledCount = 0;
            for (int i = 0; i < n; i++)
            {
                bool enable = i < limit;
                Light2D light = _lights[_order[i]];
                if (light.enabled != enable)
                {
                    light.enabled = enable;
                }
                if (enable)
                {
                    enabledCount++;
                }
            }
            return enabledCount;
        }

        /// <summary>Applies the active-in-view cap of the given quality preset.</summary>
        public int ApplyBudget(QualityPreset preset, Vector2 viewCenter)
        {
            return ApplyBudget(ActivePointLightLimit(preset), viewCenter);
        }

        private static float DistanceSquared(Light2D light, Vector2 viewCenter)
        {
            Vector3 p = light.transform.position;
            float dx = p.x - viewCenter.x;
            float dy = p.y - viewCenter.y;
            return dx * dx + dy * dy;
        }
    }
}
