using System;
using System.Collections.Generic;
using System.IO;
using System.Net;
using System.Net.WebSockets;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using ThinhThan.Protocol.V1;

namespace ThinhThan.Tests.PlayMode.Harness
{
    /// <summary>
    /// In-proc fake edge for PlayMode tests (packet § Harness): one
    /// <see cref="HttpListener"/> serves the HTTPS control plane
    /// (gameplay/ticket, auth/refresh, account) and accepts the WSS upgrade
    /// at /ws, scripting S2C responses. Deterministic defaults; scenario
    /// knobs via public delegates and helpers.
    /// </summary>
    public sealed class FakeServer : IDisposable
    {
        /// <summary>Handler: inbound frame -> respond via socket.</summary>
        public delegate Task InboundHandler(
            FakeServerSocket socket, FakeServerSocket.Inbound inbound);

        private readonly HttpListener _listener = new HttpListener();
        private readonly List<FakeServerSocket> _sockets =
            new List<FakeServerSocket>();
        private readonly object _gate = new object();
        private readonly CancellationTokenSource _cancel =
            new CancellationTokenSource();
        private readonly Queue<int> _ticketQueuePositions = new Queue<int>();
        private readonly byte[] _ticketBytes =
            Guid.NewGuid().ToByteArray();
        private readonly byte[] _accountId =
            Guid.NewGuid().ToByteArray();
        private readonly string _resumeCredential =
            Guid.NewGuid().ToString("N");

        public FakeServer(int port)
        {
            Port = port;
            Emulator = new NetworkEmulator(0x5eedu);
            SessionEpoch = 1UL;
            HeartbeatIntervalMs = 5000;
            ConnectionTimeoutMs = 15000;
            _listener.Prefixes.Add("http://127.0.0.1:" + port + "/");
        }

        public int Port
        {
            get;
        }

        public NetworkEmulator Emulator
        {
            get;
        }

        /// <summary>Control-plane base URL (http loopback).</summary>
        public string ControlUrl
        {
            get
            {
                return "http://127.0.0.1:" + Port + "/";
            }
        }

        /// <summary>WSS endpoint handed out by the ticket response.</summary>
        public Uri WsUrl
        {
            get
            {
                return new Uri("ws://127.0.0.1:" + Port + "/ws");
            }
        }

        /// <summary>Epoch written into every S2C envelope.</summary>
        public ulong SessionEpoch
        {
            get;
            set;
        }

        /// <summary>HELLO_OK knobs.</summary>
        public int HeartbeatIntervalMs
        {
            get;
            set;
        }

        public int ConnectionTimeoutMs
        {
            get;
            set;
        }

        /// <summary>When set, HELLO_OK carries resumed_character_id.</summary>
        public byte[]? ResumedCharacterId
        {
            get;
            set;
        }

        /// <summary>When set, HELLO_OK carries pending_deletion.</summary>
        public bool PendingDeletion
        {
            get;
            set;
        }

        /// <summary>Accepted sockets, in accept order.</summary>
        public IReadOnlyList<FakeServerSocket> Sockets
        {
            get
            {
                lock (_gate)
                {
                    return _sockets.ToArray();
                }
            }
        }

        /// <summary>Latest accepted socket or null.</summary>
        public FakeServerSocket? Latest
        {
            get
            {
                lock (_gate)
                {
                    return _sockets.Count > 0
                        ? _sockets[_sockets.Count - 1]
                        : null;
                }
            }
        }

        /// <summary>Optional extra inbound handler (runs after defaults).</summary>
        public InboundHandler? OnInbound
        {
            get;
            set;
        }

        /// <summary>Overrides the HELLO reply; default sends HELLO_OK.</summary>
        public InboundHandler? OnHello
        {
            get;
            set;
        }

        /// <summary>Queue one SERVER_OVERLOADED queue_position per ticket POST
        /// before the first success.</summary>
        public void QueueTicketReject(int queuePosition)
        {
            _ticketQueuePositions.Enqueue(queuePosition);
        }

        public void Start()
        {
            _listener.Start();
            _ = Task.Run(AcceptLoop);
        }

        /// <summary>Sends S2C_SESSION_REPLACED on the latest socket.</summary>
        public Task SendSessionReplacedAsync()
        {
            FakeServerSocket? socket = Latest;
            if (socket == null)
            {
                return Task.CompletedTask;
            }

            var replaced = new S2CSessionReplaced
            {
                Reason = SessionReplacedReason.NewerSession,
            };
            return socket.SendAsync(8, replaced);
        }

        /// <summary>Sends S2C_SERVER_DRAINING on the latest socket.</summary>
        public Task SendServerDrainingAsync(uint retryAfterMs)
        {
            FakeServerSocket? socket = Latest;
            if (socket == null)
            {
                return Task.CompletedTask;
            }

            var draining = new S2CServerDraining
            {
                Reason = ServerDrainingReason.Maintenance,
                DrainDeadlineMs =
                    DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() + 30000,
                ReconnectAfterMs = retryAfterMs,
            };
            return socket.SendAsync(9, draining);
        }

        /// <summary>Pushes CHARACTER_LIST on the latest socket.</summary>
        public Task SendCharacterListAsync(params CharacterSummary[] rows)
        {
            FakeServerSocket? socket = Latest;
            if (socket == null)
            {
                return Task.CompletedTask;
            }

            var list = new S2CCharacterList();
            list.Characters.Add(rows);
            return socket.SendAsync(14, list);
        }

        /// <summary>Sends S2C_ERROR with close_after on the latest socket.</summary>
        public Task SendErrorAsync(
            ErrorCode errorCode, bool closeAfter, uint retryAfterMs = 0)
        {
            FakeServerSocket? socket = Latest;
            if (socket == null)
            {
                return Task.CompletedTask;
            }

            var error = new S2CError
            {
                ErrorCode = errorCode,
                CloseAfter = closeAfter,
                RetryAfterMs = retryAfterMs,
            };
            return socket.SendAsync(3, error);
        }

        public void Dispose()
        {
            _cancel.Cancel();
            _listener.Stop();
            FakeServerSocket[] sockets;
            lock (_gate)
            {
                sockets = _sockets.ToArray();
            }

            foreach (FakeServerSocket socket in sockets)
            {
                socket.Dispose();
            }

            _listener.Close();
            _cancel.Dispose();
        }

        internal void Dispatch(
            FakeServerSocket socket, FakeServerSocket.Inbound inbound)
        {
            switch (inbound.MessageId)
            {
                case 1:
                    if (OnHello != null)
                    {
                        _ = OnHello(socket, inbound);
                        break;
                    }

                    _ = ReplyHelloOk(socket);
                    break;
                case 4:
                    var pong = new S2CHeartbeat
                    {
                        ServerMs = (ulong)(
                            DateTimeOffset.UtcNow.ToUnixTimeMilliseconds()),
                    };
                    _ = socket.SendAsync(5, pong);
                    break;
                case 6:
                    // Payload! : FakeServerSocket decodes id 6 to
                    // C2SCharacterAttach and never emits a null payload.
                    var attach = (C2SCharacterAttach)inbound.Payload!;
                    var attachOk = new S2CCharacterAttachOk
                    {
                        CharacterId = attach.CharacterId,
                    };
                    _ = socket.SendAsync(7, attachOk);
                    break;
                case 10:
                    _ = socket.SendAsync(11, new S2CCharacterDetachOk());
                    break;
                case 12:
                    // Payload! : decoder maps id 12 to C2SCharacterCreate;
                    // payload is never null for mapped ids.
                    var create = (C2SCharacterCreate)inbound.Payload!;
                    var character = new CharacterSummary
                    {
                        CharacterId = ByteString.CopyFrom(
                            Guid.NewGuid().ToByteArray()),
                        CharacterName = create.CharacterName,
                        ClassId = create.ClassId,
                        Level = 1,
                    };
                    var created = new S2CCharacterCreateResult
                    {
                        Result = new OperationResult
                        {
                            OperationId = create.OperationId,
                            Status = ResultStatus.Success,
                        },
                        Character = character,
                    };
                    _ = socket.SendAsync(13, created).ContinueWith(
                        _ => socket.SendAsync(14, ListWith(character)));
                    break;
                case 106:
                    break;
                case 306:
                    break;
                case 307:
                    // Payload! : decoder maps id 307 to
                    // C2SBaselineResyncRequest; payload is never null.
                    var resync = (C2SBaselineResyncRequest)inbound.Payload!;
                    var result = new S2CBaselineResyncResult
                    {
                        RequestId = resync.RequestId,
                        Status = ResultStatus.Success,
                    };
                    _ = socket.SendAsync(308, result);
                    break;
                default:
                    break;
            }

            if (OnInbound != null)
            {
                _ = OnInbound(socket, inbound);
            }
        }

        private async Task ReplyHelloOk(FakeServerSocket socket)
        {
            var ok = new S2CHelloOk
            {
                SessionId = ByteString.CopyFrom(Guid.NewGuid().ToByteArray()),
                SessionEpoch = SessionEpoch,
                AccountId = ByteString.CopyFrom(_accountId),
                ServerTimeMs =
                    DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(),
                HeartbeatIntervalMs = (uint)HeartbeatIntervalMs,
                ConnectionTimeoutMs = (uint)ConnectionTimeoutMs,
                ResumeCredential = _resumeCredential,
                ResumeExpiresAtMs =
                    DateTimeOffset.UtcNow.ToUnixTimeMilliseconds() + 600000,
                PendingDeletion = PendingDeletion,
            };
            if (ResumedCharacterId != null)
            {
                ok.ResumedCharacterId =
                    ByteString.CopyFrom(ResumedCharacterId);
            }

            await socket.SendAsync(2, ok).ConfigureAwait(false);
        }

        private static S2CCharacterList ListWith(CharacterSummary character)
        {
            var list = new S2CCharacterList();
            list.Characters.Add(character);
            return list;
        }

        private async Task AcceptLoop()
        {
            try
            {
                while (!_cancel.IsCancellationRequested)
                {
                    HttpListenerContext context =
                        await _listener.GetContextAsync()
                            .ConfigureAwait(false);
                    _ = HandleContext(context);
                }
            }
            catch (Exception)
            {
            }
        }

        private async Task HandleContext(HttpListenerContext context)
        {
            try
            {
                string path = context.Request.Url != null
                    ? context.Request.Url.AbsolutePath
                    : string.Empty;
                if (context.Request.IsWebSocketRequest && path == "/ws")
                {
                    HttpListenerWebSocketContext wsContext =
                        await context.AcceptWebSocketAsync(null)
                            .ConfigureAwait(false);
                    var socket = new FakeServerSocket(this, wsContext.WebSocket);
                    lock (_gate)
                    {
                        _sockets.Add(socket);
                    }

                    return;
                }

                await HandleHttp(context, path).ConfigureAwait(false);
            }
            catch (Exception)
            {
                context.Response.StatusCode = 500;
                context.Response.Close();
            }
        }

        private Task HandleHttp(HttpListenerContext context, string path)
        {
            string method = context.Request.HttpMethod;
            if (method == "POST" && path == "/api/v1/gameplay/ticket")
            {
                if (_ticketQueuePositions.Count > 0)
                {
                    int position = _ticketQueuePositions.Dequeue();
                    WriteJson(context, 503,
                        "{\"error_code\":\"SERVER_OVERLOADED\"," +
                        "\"retryability\":\"RETRYABLE\"," +
                        "\"retry_after_ms\":5000," +
                        "\"queue_position\":" + position + "," +
                        "\"safe_message_key\":\"error.server_overloaded\"," +
                        "\"close_after\":false}");
                    return Task.CompletedTask;
                }

                WriteJson(context, 200,
                    "{\"ticket\":\"" +
                    Convert.ToBase64String(_ticketBytes) + "\"," +
                    "\"ticket_expires_at\":\"" +
                    DateTimeOffset.UtcNow.AddSeconds(60)
                        .ToString("o") + "\"," +
                    "\"wss_url\":\"" + WsUrl + "\"," +
                    "\"protocol_minor_min\":0," +
                    "\"client_build_min\":0," +
                    "\"content_revision\":\"test\"}");
                return Task.CompletedTask;
            }

            if (method == "POST" && path == "/api/v1/auth/refresh")
            {
                WriteJson(context, 200,
                    "{\"access_token\":\"fake-access\"," +
                    "\"access_expires_at\":\"" +
                    DateTimeOffset.UtcNow.AddMinutes(15).ToString("o") + "\"," +
                    "\"refresh_token\":\"fake-refresh-" +
                    Guid.NewGuid().ToString("N") + "\"," +
                    "\"refresh_expires_at\":\"" +
                    DateTimeOffset.UtcNow.AddDays(30).ToString("o") + "\"}");
                return Task.CompletedTask;
            }

            if (method == "GET" && path == "/api/v1/account")
            {
                WriteJson(context, 200,
                    "{\"account_id\":\"" +
                    new Guid(_accountId) + "\"," +
                    "\"username\":\"fake\"," +
                    "\"email_verified\":true," +
                    "\"pending_deletion\":false," +
                    "\"created_at\":\"" +
                    DateTimeOffset.UtcNow.ToString("o") + "\"}");
                return Task.CompletedTask;
            }

            if (method == "POST" && path == "/api/v1/auth/logout")
            {
                context.Response.StatusCode = 204;
                context.Response.Close();
                return Task.CompletedTask;
            }

            WriteJson(context, 404,
                "{\"error_code\":\"MESSAGE_UNKNOWN\"," +
                "\"retryability\":\"FATAL\",\"close_after\":false}");
            return Task.CompletedTask;
        }

        private static void WriteJson(
            HttpListenerContext context, int status, string body)
        {
            byte[] bytes = Encoding.UTF8.GetBytes(body);
            context.Response.StatusCode = status;
            context.Response.ContentType = "application/json";
            context.Response.ContentLength64 = bytes.Length;
            context.Response.OutputStream.Write(bytes, 0, bytes.Length);
            context.Response.Close();
        }
    }
}
