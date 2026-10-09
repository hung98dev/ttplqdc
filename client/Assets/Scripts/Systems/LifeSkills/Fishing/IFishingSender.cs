using System.Threading;
using Google.Protobuf;
using UnityEngine;

namespace ThinhThan.Systems.LifeSkills.Fishing
{
    /// <summary>
    /// Send seam for outbound C2S payloads (same shape as
    /// Systems/LifeSkills/Cooking <c>ICookingSender</c>): production
    /// wraps NetSession, which stamps client_seq privately.
    /// </summary>
    public interface IFishingSender
    {
        Awaitable<ulong> SendAsync(
            uint messageId, IMessage payload, CancellationToken cancel);
    }
}
