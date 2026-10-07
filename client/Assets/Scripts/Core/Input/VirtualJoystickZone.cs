using UnityEngine;

namespace ThinhThan.Core.Input
{
    /// <summary>
    /// Virtual-joystick zone math (client_experience_contract.md §4.2): the
    /// lower-left quadrant X in [0, 0.4] / Y in [0, 0.5] of the viewport.
    /// Touches inside it feed <see cref="InputSemanticState"/> and never
    /// trigger the chat dock or character widgets. Pure math so the widget
    /// and the tests share one source.
    /// </summary>
    public static class VirtualJoystickZone
    {
        public const float MaxViewportX = 0.4f;
        public const float MaxViewportY = 0.5f;

        /// <summary>Stick displacement dead zone (normalized radius).</summary>
        public const float DeadZone = 0.1f;

        /// <summary>Down threshold: beyond -DeadZone*2 counts as "down".</summary>
        public const float DownThreshold = 0.35f;

        public static bool Contains(Vector2 viewportPoint)
        {
            return viewportPoint.x >= 0f && viewportPoint.x <= MaxViewportX &&
                viewportPoint.y >= 0f && viewportPoint.y <= MaxViewportY;
        }

        /// <summary>
        /// Held horizontal direction for a normalized stick offset:
        /// -1 / 0 / +1 outside the dead zone.
        /// </summary>
        public static int DirectionOf(Vector2 offset)
        {
            if (offset.x < -DeadZone)
            {
                return -1;
            }

            return offset.x > DeadZone ? 1 : 0;
        }

        /// <summary>Joystick pulled down far enough to arm drop-through.</summary>
        public static bool IsDown(Vector2 offset)
        {
            return offset.y < -DownThreshold;
        }
    }
}
