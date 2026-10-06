namespace ThinhThan.Systems.Movement
{
    /// <summary>
    /// Client mirror of the canonical movement baseline (movement.md §
    /// Baseline + contract §4.1). All distances are millimetres, speeds
    /// mm/s, accelerations mm/s², ticks 50 ms.
    /// </summary>
    public static class MovementConstants
    {
        public const long TickMillis = 50;
        public const int TicksPerSecond = 20;

        public const int RunSpeedMmS = 6000;
        public const int FirstJumpMmS = 11000;
        public const int SecondJumpMmS = 10000;
        public const int GravityMmS2 = 28000;
        public const int MaxFallMmS = 20000;
        public const int AirControlBp = 8500;
        public const long MaxStepHeightMm = 300;
        public const int DropIgnoreTicks = 6;
        public const int MaxJumpCount = 2;

        /// <summary>Self-ack correction band: 500 mm, squared (synchronization.md § Local Reconciliation).</summary>
        public const long SnapThresholdSqMm = 500L * 500L;

        /// <summary>Sub-threshold corrections lerp over this many ms.</summary>
        public const int SmoothMs = 100;

        /// <summary>C2S_INPUT_STATE cadence: at most one send per 50 ms.</summary>
        public const double HeldSendMinIntervalSec = 0.05;

        /// <summary>While any flag is held, resend at least once per 250 ms.</summary>
        public const double HeldResendIntervalSec = 0.25;

        /// <summary>Bounded seq→record ring for self-ack replay.</summary>
        public const int InputHistoryCapacity = 128;

        /// <summary>Wire ids (messages.md registry).</summary>
        public const uint WireC2SInputState = 100;
        public const uint WireC2SJump = 101;
        public const uint WireC2SDropThrough = 102;
        public const uint WireC2SMovementEdge = 108;
    }
}
