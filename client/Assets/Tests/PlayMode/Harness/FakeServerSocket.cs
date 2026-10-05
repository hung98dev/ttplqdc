using System;
using System.Collections.Generic;
using System.Net.WebSockets;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Tests.PlayMode.Harness
{
    /// <summary>
    /// One accepted WSS connection inside <see cref="FakeServer"/>: receives
    /// client envelopes, dispatches them to scripted handlers, and sends S2C
    /// envelopes through the NetworkEmulator with server-owned server_seq.
    /// </summary>
    public sealed class FakeServerSocket : IDisposable
    {
        /// <summary>Recorded inbound frame for assertions.</summary>
        public sealed class Inbound
        {
            public uint MessageId
            {
                get;
                set;
            }

            public ulong ClientSeq
            {
                get;
                set;
            }

            public ulong SessionEpoch
            {
                get;
                set;
            }

            public uint ProtocolMajor
            {
                get;
                set;
            }

            public ulong CorrelationId
            {
                get;
                set;
            }

            public IMessage? Payload
            {
                get;
                set;
            }
        }

        private readonly FakeServer _owner;
        private readonly WebSocket _socket;
        private readonly List<Inbound> _received = new List<Inbound>();
        private readonly object _gate = new object();
        private readonly SemaphoreSlim _sendGate = new SemaphoreSlim(1, 1);
        private readonly CancellationTokenSource _cancel =
            new CancellationTokenSource();
        private ulong _nextServerSeq;
        private bool _closed;

        public FakeServerSocket(FakeServer owner, WebSocket socket)
        {
            _owner = owner;
            _socket = socket;
            ReceiveTask = Task.Run(ReceiveLoop);
        }

        /// <summary>Completes when the receive loop exits (close or error).</summary>
        public Task ReceiveTask
        {
            get;
        }

        /// <summary>Every inbound envelope, in arrival order.</summary>
        public IReadOnlyList<Inbound> Received
        {
            get
            {
                lock (_gate)
                {
                    return _received.ToArray();
                }
            }
        }

        /// <summary>Sends an S2C envelope through the emulator.</summary>
        public async Task SendAsync(uint messageId, IMessage payload)
        {
            var envelope = new Envelope
            {
                ProtocolMajor = WireIds.ProtocolMajor,
                ProtocolMinor = WireIds.ProtocolMinor,
                MessageId = messageId,
                SessionEpoch = _owner.SessionEpoch,
                ServerSeq = ++_nextServerSeq,
                Payload = payload.ToByteString(),
            };
            byte[] bytes = envelope.ToByteArray();
            if (!await _owner.Emulator.PassAsync(_cancel.Token)
                    .ConfigureAwait(false))
            {
                return;
            }

            await _sendGate.WaitAsync().ConfigureAwait(false);
            try
            {
                await _socket.SendAsync(
                    new ArraySegment<byte>(bytes),
                    WebSocketMessageType.Binary, true, _cancel.Token)
                    .ConfigureAwait(false);
            }
            finally
            {
                _sendGate.Release();
            }
        }

        /// <summary>Unclean close (simulates transport loss).</summary>
        public void Kill()
        {
            _closed = true;
            _socket.Abort();
            _cancel.Cancel();
        }

        /// <summary>Clean WS close handshake.</summary>
        public async Task CloseAsync()
        {
            _closed = true;
            try
            {
                await _socket.CloseAsync(
                    WebSocketCloseStatus.NormalClosure, "bye",
                    CancellationToken.None).ConfigureAwait(false);
            }
            catch (Exception)
            {
            }

            _cancel.Cancel();
        }

        public void Dispose()
        {
            Kill();
            _socket.Dispose();
            _cancel.Dispose();
        }

        private async Task ReceiveLoop()
        {
            var buffer = new byte[WireIds.InboundFrameMaxBytes];
            var stream = new List<byte>();
            try
            {
                while (!_closed && _socket.State == WebSocketState.Open)
                {
                    WebSocketReceiveResult result = await _socket.ReceiveAsync(
                        new ArraySegment<byte>(buffer), _cancel.Token)
                        .ConfigureAwait(false);
                    if (result.MessageType == WebSocketMessageType.Close)
                    {
                        return;
                    }

                    for (int i = 0; i < result.Count; i++)
                    {
                        stream.Add(buffer[i]);
                    }

                    if (!result.EndOfMessage)
                    {
                        continue;
                    }

                    if (stream.Count > WireIds.InboundFrameMaxBytes)
                    {
                        stream.Clear();
                        continue;
                    }

                    Envelope envelope = Envelope.Parser.ParseFrom(
                        stream.ToArray());
                    stream.Clear();
                    HandleInbound(envelope);
                }
            }
            catch (Exception)
            {
            }
        }

        private void HandleInbound(Envelope envelope)
        {
            var inbound = new Inbound
            {
                MessageId = envelope.MessageId,
                ClientSeq = envelope.ClientSeq,
                SessionEpoch = envelope.SessionEpoch,
                ProtocolMajor = envelope.ProtocolMajor,
                CorrelationId = envelope.CorrelationId,
            };
            switch (envelope.MessageId)
            {
                case WireIds.C2SHello:
                    inbound.Payload = C2SHello.Parser.ParseFrom(
                        envelope.Payload);
                    break;
                case WireIds.C2SHeartbeat:
                    inbound.Payload = C2SHeartbeat.Parser.ParseFrom(
                        envelope.Payload);
                    break;
                case WireIds.C2SCharacterAttach:
                    inbound.Payload = C2SCharacterAttach.Parser.ParseFrom(
                        envelope.Payload);
                    break;
                case WireIds.C2SCharacterDetach:
                    inbound.Payload =
                        C2SCharacterDetach.Parser.ParseFrom(envelope.Payload);
                    break;
                case WireIds.C2SCharacterCreate:
                    inbound.Payload = C2SCharacterCreate.Parser.ParseFrom(
                        envelope.Payload);
                    break;
                case WireIds.C2SPresentationReady:
                    inbound.Payload = C2SPresentationReady.Parser.ParseFrom(
                        envelope.Payload);
                    break;
                case WireIds.C2SBaselineAck:
                    inbound.Payload = C2SBaselineAck.Parser.ParseFrom(
                        envelope.Payload);
                    break;
                case WireIds.C2SBaselineResyncRequest:
                    inbound.Payload = C2SBaselineResyncRequest.Parser
                        .ParseFrom(envelope.Payload);
                    break;
                default:
                    break;
            }

            lock (_gate)
            {
                _received.Add(inbound);
            }

            _owner.Dispatch(this, inbound);
        }
    }
}
