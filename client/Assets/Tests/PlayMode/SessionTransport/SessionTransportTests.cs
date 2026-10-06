using System;
using System.Net;
using System.Net.Sockets;
using System.Threading;
using System.Threading.Tasks;
using Google.Protobuf;
using NUnit.Framework;
using ThinhThan.Core.Runtime;
using ThinhThan.Core.Session;
using ThinhThan.Net;
using ThinhThan.Protocol.V1;
using ThinhThan.Tests.PlayMode.Harness;
using ThinhThan.UI.Character;

namespace ThinhThan.Tests.PlayMode.SessionTransport
{
    /// <summary>WSS bootstrap + reconnect + SESSION_REPLACED (packet).</summary>
    public sealed class SessionTransportTests
    {
        private static int FreePort()
        {
            var listener = new TcpListener(IPAddress.Loopback, 0);
            listener.Start();
            int port = ((IPEndPoint)listener.LocalEndpoint).Port;
            listener.Stop();
            return port;
        }

        private static SessionOrchestrator NewOrchestrator(
            FakeServer server,
            out SessionStateMachine fsm,
            out SessionCredentials credentials)
        {
            fsm = new SessionStateMachine();
            credentials = new SessionCredentials
            {
                DeviceId = Guid.NewGuid().ToByteArray(),
                AccessToken = "test-access",
                ClientBuild = 1,
                ContentRevision = "test",
            };
            var store = new SessionStore(new InMemorySessionStorage());
            store.Save(credentials);
            var auth = new AuthClient(server.ControlUrl);
            return new SessionOrchestrator(
                fsm, credentials, store, auth,
                delay: (ms, cancel) =>
                    Task.Delay(Math.Min(ms, 25), cancel));
        }

        private static async Task PumpUntil(
            SessionOrchestrator orchestrator,
            Func<bool> condition, int timeoutMs)
        {
            var deadline = new FrameTime(0.016f, 0.0, 0);
            long end = NowMs() + timeoutMs;
            while (!condition() && NowMs() < end)
            {
                orchestrator.Tick(in deadline);
                await Task.Delay(10).ConfigureAwait(false);
            }
        }

        private static long NowMs()
        {
            return DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();
        }

        [Test]
        public async Task TestConnectAuthFlow()
        {
            using var server = new FakeServer(FreePort());
            server.QueueTicketReject(3);
            server.Start();
            SessionOrchestrator orchestrator = NewOrchestrator(
                server, out SessionStateMachine fsm, out _);
            Result<bool> connected = await orchestrator
                .ConnectWithTicketAsync(CancellationToken.None)
                .ConfigureAwait(false);
            Assert.IsTrue(connected.Ok, connected.ErrorCode);
            await PumpUntil(orchestrator,
                () => fsm.Phase == SessionPhase.CharacterSelect, 5000);
            Assert.AreEqual(SessionPhase.CharacterSelect, fsm.Phase);
            Assert.AreEqual(ClientUiState.CharacterSelect, fsm.UiState);

            var summary = new CharacterSummary
            {
                CharacterId = ByteString.CopyFrom(
                    Guid.NewGuid().ToByteArray()),
                CharacterName = "nhan_vat",
                ClassId = "class.kim",
            };
            await server.SendCharacterListAsync(summary);
            await PumpUntil(orchestrator,
                () => orchestrator.CharacterList != null, 3000);
            Assert.IsNotNull(orchestrator.CharacterList);
            // CharacterList!: IsNotNull asserted immediately above.
            Assert.AreEqual(1, orchestrator.CharacterList!.Characters.Count);
            await orchestrator.ShutdownAsync();
        }

        [Test]
        public async Task TestReconnectResume()
        {
            using var server = new FakeServer(FreePort());
            byte[] resumed = Guid.NewGuid().ToByteArray();
            server.ResumedCharacterId = resumed;
            server.Start();
            SessionOrchestrator orchestrator = NewOrchestrator(
                server, out SessionStateMachine fsm, out _);
            Result<bool> connected = await orchestrator
                .ConnectWithTicketAsync(CancellationToken.None)
                .ConfigureAwait(false);
            Assert.IsTrue(connected.Ok, connected.ErrorCode);
            await PumpUntil(orchestrator,
                () => fsm.Phase == SessionPhase.InWorld, 5000);
            Assert.AreEqual(SessionPhase.InWorld, fsm.Phase);

            // Latest!: the connect above must have produced one socket.
            FakeServerSocket first = server.Latest!;
            first.Kill();
            await PumpUntil(orchestrator,
                () => fsm.Phase == SessionPhase.InWorld &&
                    server.Sockets.Count >= 2, 15000);
            Assert.AreEqual(SessionPhase.InWorld, fsm.Phase);
            Assert.GreaterOrEqual(server.Sockets.Count, 2);

            FakeServerSocket resumedSocket = server.Sockets[1];
            bool sawResumeHello = false;
            foreach (FakeServerSocket.Inbound inbound in resumedSocket.Received)
            {
                if (inbound.Payload is C2SHello hello &&
                    hello.CredentialCase ==
                        C2SHello.CredentialOneofCase.ResumeCredential &&
                    hello.ResumeCredential.Length != 0)
                {
                    sawResumeHello = true;
                }
            }

            Assert.IsTrue(sawResumeHello);
            await orchestrator.ShutdownAsync();
        }

        [Test]
        public async Task TestSessionReplacedHandling()
        {
            using var server = new FakeServer(FreePort());
            server.Start();
            SessionOrchestrator orchestrator = NewOrchestrator(
                server, out SessionStateMachine fsm, out _);
            Result<bool> connected = await orchestrator
                .ConnectWithTicketAsync(CancellationToken.None)
                .ConfigureAwait(false);
            Assert.IsTrue(connected.Ok, connected.ErrorCode);
            await PumpUntil(orchestrator,
                () => fsm.Phase == SessionPhase.CharacterSelect, 5000);

            await server.SendSessionReplacedAsync();
            await PumpUntil(orchestrator,
                () => orchestrator.ReplacedByNewerSession, 3000);
            Assert.IsTrue(orchestrator.ReplacedByNewerSession);
            Assert.IsTrue(fsm.SessionReplaced);
            Assert.AreEqual(ClientUiState.AuthTitle, fsm.UiState);

            var presenter = new SessionUiPresenter(orchestrator, fsm);
            Assert.AreEqual(SessionModal.SessionReplaced, presenter.Modal);
            presenter.AcknowledgeSessionReplaced();
            Assert.AreEqual(SessionModal.None, presenter.Modal);
            await orchestrator.ShutdownAsync();
        }
    }
}
