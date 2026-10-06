using System.Threading;
using Google.Protobuf;
using UnityEngine;

namespace ThinhThan.Systems.Movement
{
    /// <summary>
    /// Send seam for outbound C2S messages (F-07): the production adapter
    /// wraps NetSession, which stamps <c>client_seq</c> privately — the
    /// returned Awaitable resolves to that stamped seq so the dispatch can
    /// correlate history with <c>last_processed_client_seq</c>. Task is
    /// Net-only (engineering_conventions.md §2.5); this surface uses
    /// Awaitable.
    /// </summary>
    public interface IMovementSender
    {
        /// <summary>
        /// Sends one payload under <paramref name="messageId"/>; resolves to
        /// the stamped client_seq on the shared sequence.
        /// </summary>
        Awaitable<ulong> SendAsync(
            uint messageId, IMessage payload, CancellationToken cancel);
    }
}
