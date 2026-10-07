using System.Threading;
using ThinhThan.Protocol.V1;
using UnityEngine;

namespace ThinhThan.Systems.Progression
{
    /// <summary>
    /// Client → wire intent seam for the 511–513 durable commands
    /// (messages.md § Progression). The production adapter wraps the Net
    /// session sender; results arrive later through IProgressionSink (514)
    /// — an intent method resolving means the frame was sent, never that
    /// the mutation committed.
    /// </summary>
    public interface IProgressionIntents
    {
        /// <summary>C2S_SKILL_UPGRADE (511): expected_level is the
        /// optimistic concurrency guard (0 = unchecked).</summary>
        Awaitable<bool> RequestUpgrade(
            string skillId, int expectedLevel, CancellationToken cancel);

        /// <summary>C2S_POTENTIAL_ALLOCATE (512): all-or-nothing deltas.</summary>
        Awaitable<bool> RequestAllocate(
            PotentialDelta deltas, CancellationToken cancel);

        /// <summary>C2S_RESPEC (513): npc_id must name an open NPC
        /// service session — the edge consult validates it.</summary>
        Awaitable<bool> RequestRespec(
            string npcId, RespecKind kind, CancellationToken cancel);
    }
}
