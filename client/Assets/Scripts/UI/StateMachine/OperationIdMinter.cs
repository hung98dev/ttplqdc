using System;
using System.Threading;

namespace ThinhThan.UI.StateMachine
{
    /// <summary>
    /// operation_id minter for the interact/portal intents (103/104):
    /// UUIDv7 per ids.md, 16 bytes — time-ordered millis + per-instance
    /// counter + random tail, same shape the session layer mints for
    /// C2S 12/208.
    /// </summary>
    public sealed class OperationIdMinter
    {
        private int _nextSeed;

        public byte[] Mint()
        {
            long unixMs = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();
            int seed = Interlocked.Increment(ref _nextSeed);
            var bytes = new byte[16];
            bytes[0] = (byte)(unixMs >> 40);
            bytes[1] = (byte)(unixMs >> 32);
            bytes[2] = (byte)(unixMs >> 24);
            bytes[3] = (byte)(unixMs >> 16);
            bytes[4] = (byte)(unixMs >> 8);
            bytes[5] = (byte)unixMs;
            bytes[6] = (byte)(0x70 | (seed >> 8 & 0x0F));
            bytes[7] = (byte)(seed & 0xFF);
            byte[] tail = Guid.NewGuid().ToByteArray();
            for (int i = 8; i < 16; i++)
            {
                bytes[i] = tail[i - 8];
            }

            bytes[8] = (byte)(bytes[8] & 0x3F | 0x80);
            return bytes;
        }
    }
}
