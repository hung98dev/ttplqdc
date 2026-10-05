using System;
using System.Collections.Generic;
using Google.Protobuf;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Tests.PlayMode.NetReceive.Fixtures
{
    /// <summary>
    /// Deterministic hotspot stream fixture (test_and_release_evidence §3):
    /// 60 s at 10 Hz, 40 entities + local player. Byte layout:
    /// [u32 magic 'HSOT'][u32 version 1][u32 frameCount] then per frame
    /// [u32 length][serialized Envelope]. Frame 0 is S2C_WORLD_BASELINE;
    /// the rest are S2C_STATE_DELTA. The generator's math is integer-only
    /// so the C# and the checked-in fixture are byte-identical — the test
    /// asserts regeneration instead of trusting the binary.
    /// </summary>
    public static class HotspotStreamGenerator
    {
        public const uint Magic = 0x48534F54;

        public const int EntityCount = 40;

        public const int DeltaFrameCount = 600;

        public const int Seed = 0x1051;

        /// <summary>
        /// Builds the whole fixture (baseline + 600 deltas) as the
        /// container byte blob.
        /// </summary>
        public static byte[] Generate()
        {
            var output = new List<byte>(DeltaFrameCount * 512);
            WriteInt(output, unchecked((int)Magic));
            WriteInt(output, 1);
            WriteInt(output, DeltaFrameCount + 1);
            WriteFrame(output, BuildBaselineEnvelope());
            for (int frame = 1; frame <= DeltaFrameCount; frame++)
            {
                WriteFrame(output, BuildDeltaEnvelope(frame));
            }

            return output.ToArray();
        }

        /// <summary>Builds envelope 0 — the world baseline.</summary>
        public static Envelope BuildBaselineEnvelope()
        {
            var baseline = new S2CWorldBaseline
            {
                BaselineId = 1,
                ServerTick = 0,
                MapId = "map.lang_da.dinh_lang",
                ContentRevision = "test",
                Self = new EntityState
                {
                    EntityId = 1,
                    EntityKind = EntityKind.Player,
                    ContentId = "class.kim",
                    DisplayName = "self",
                    Level = 10,
                    XMm = 5000,
                    YMm = 5000,
                    Hp = 1000,
                    MaxHp = 1000,
                    Facing = Facing.Right,
                    MovementState = MovementState.Idle,
                },
                SelfPrivate = new SelfPrivateState
                {
                    CurrentMp = 400,
                    MaxMp = 400,
                },
                SelfCheckpoint = new MovementCheckpoint
                {
                    XMm = 5000,
                    YMm = 5000,
                    IsGrounded = true,
                    EffectiveParameters = new EffectiveMovementParameters
                    {
                        RunSpeedMmS = 2400,
                        FirstJumpMmS = 7000,
                        SecondJumpMmS = 6000,
                        GravityMmS2 = 20000,
                        MaxFallMmS = 9000,
                        AirControlBp = 6000,
                        MaxStepHeightMm = 400,
                    },
                },
            };
            for (int i = 0; i < EntityCount; i++)
            {
                baseline.Entities.Add(new EntityState
                {
                    EntityId = (ulong)(100 + i),
                    EntityKind = EntityKind.Monster,
                    ContentId = "mob.slime",
                    DisplayName = "e" + i,
                    Level = (uint)(i % 10 + 1),
                    XMm = CenterX(i),
                    YMm = CenterY(i),
                    VxMmS = VelocityX(i),
                    VyMmS = VelocityY(i),
                    Hp = 800 + i,
                    MaxHp = 1000,
                    Facing = Facing.Left,
                    MovementState = MovementState.Run,
                });
            }

            return Wrap(300, baseline, 0);
        }

        /// <summary>Builds the envelope for delta frame f (1..600).</summary>
        public static Envelope BuildDeltaEnvelope(int frame)
        {
            var delta = new S2CStateDelta
            {
                BaselineId = 1,
                ServerTick = (ulong)frame * 2,
                SelfAck = new SelfAck
                {
                    LastProcessedClientSeq = (ulong)(20 + frame),
                    Checkpoint = new MovementCheckpoint
                    {
                        XMm = 5000 + frame,
                        YMm = 5000,
                        IsGrounded = true,
                    },
                },
                SelfPrivate = new SelfPrivateDelta
                {
                    CurrentMp = 400 - frame % 10,
                },
            };
            for (int i = 0; i < EntityCount; i++)
            {
                var entity = new EntityDelta
                {
                    EntityId = (ulong)(100 + i),
                    XMm = CenterX(i) + Mod(frame * DeltaX(i), 4000) - 2000,
                    YMm = CenterY(i) + Mod(frame * DeltaY(i), 3000) - 1500,
                    VxMmS = VelocityX(i),
                    VyMmS = VelocityY(i),
                };
                if (frame % 10 == 0)
                {
                    entity.Hp = 800 + Mod(i * 7 + frame, 200);
                }

                if (frame % 50 == 0)
                {
                    entity.Flags = (uint)(frame / 50);
                }

                delta.Entities.Add(entity);
            }

            return Wrap(303, delta, (ulong)frame);
        }

        /// <summary>
        /// Reads a container blob back into its serialized envelopes.
        /// </summary>
        public static List<byte[]> ReadFrames(byte[] container)
        {
            var frames = new List<byte[]>();
            int offset = 12;
            while (offset + 4 <= container.Length)
            {
                int length = BitConverter.ToInt32(container, offset);
                offset += 4;
                var frame = new byte[length];
                Array.Copy(container, offset, frame, 0, length);
                offset += length;
                frames.Add(frame);
            }

            return frames;
        }

        private static Envelope Wrap(
            uint messageId, IMessage payload, ulong serverSeq)
        {
            return new Envelope
            {
                ProtocolMajor = 1,
                ProtocolMinor = 0,
                MessageId = messageId,
                SessionEpoch = 1,
                ServerSeq = serverSeq,
                Payload = payload.ToByteString(),
            };
        }

        private static int CenterX(int i)
        {
            return 10000 + i * 500;
        }

        private static int CenterY(int i)
        {
            return 20000 + i % 5 * 400;
        }

        private static int DeltaX(int i)
        {
            return 17 + i * 3;
        }

        private static int DeltaY(int i)
        {
            return 11 + i * 2;
        }

        private static int VelocityX(int i)
        {
            return (i + 1) * 37 % 200 - 100;
        }

        private static int VelocityY(int i)
        {
            return (i + 3) * 29 % 160 - 80;
        }

        private static int Mod(int value, int modulus)
        {
            int result = value % modulus;
            return result < 0 ? result + modulus : result;
        }

        private static void WriteFrame(List<byte> output, Envelope envelope)
        {
            byte[] bytes = envelope.ToByteArray();
            WriteInt(output, bytes.Length);
            output.AddRange(bytes);
        }

        private static void WriteInt(List<byte> output, int value)
        {
            output.Add((byte)value);
            output.Add((byte)(value >> 8));
            output.Add((byte)(value >> 16));
            output.Add((byte)(value >> 24));
        }
    }
}
