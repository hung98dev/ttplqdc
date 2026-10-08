using System.Threading;
using Google.Protobuf;
using UnityEngine;

namespace ThinhThan.Systems.Combat
{
    /// <summary>
    /// Transport seam the combat intents use to emit C2S messages; resolves
    /// to the stamped client_seq on the shared sequence.
    /// </summary>
    public interface ICombatSender
    {
        Awaitable<ulong> SendAsync(
            uint messageId, IMessage payload, CancellationToken cancel);
    }
}
