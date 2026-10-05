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

namespace ThinhThan.Tests.PlayMode.CharacterLifecycleClient
{
    /// <summary>
    /// Create / select / attach / detach / SESSION_REPLACED UI states
    /// (packet IMP-065): the client drives 12 -> 13 + 14, 6 -> 7, 10 -> 11
    /// and lands the modal on session replacement.
    /// </summary>
    public sealed class CharacterLifecycleClientTests
    {
        private static int FreePort()
        {
            var listener = new TcpListener(IPAddress.Loopback, 0);
            listener.Start();
            int port = ((IPEndPoint)listener.LocalEndpoint).Port;
            listener.Stop();
            return port;
        }

        private static async Task PumpUntil(
            SessionOrchestrator orchestrator,
            Func<bool> condition, int timeoutMs)
        {
            var time = new FrameTime(0.016f, 0.0, 0);
            long end = NowMs() + timeoutMs;
            while (!condition() && NowMs() < end)
            {
                orchestrator.Tick(in time);
                await Task.Delay(10).ConfigureAwait(false);
            }
        }

        private static long NowMs()
        {
            return DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();
        }

        [Test]
        public async Task TestCreateSelectAttachDetachSessionReplaced()
        {
            using var server = new FakeServer(FreePort());
            server.Start();
            var fsm = new SessionStateMachine();
            var credentials = new SessionCredentials
            {
                DeviceId = Guid.NewGuid().ToByteArray(),
                AccessToken = "test-access",
                ClientBuild = 1,
                ContentRevision = "test",
            };
            var store = new SessionStore(new InMemorySessionStorage());
            store.Save(credentials);
            var orchestrator = new SessionOrchestrator(
                fsm, credentials, store,
                new AuthClient(server.ControlUrl),
                delay: (ms, cancel) =>
                    Task.Delay(Math.Min(ms, 25), cancel));
            var controller = new CharacterSessionController(orchestrator);
            var presenter = new SessionUiPresenter(orchestrator, fsm);

            Result<bool> connected = await orchestrator
                .ConnectWithTicketAsync(CancellationToken.None)
                .ConfigureAwait(false);
            Assert.IsTrue(connected.Ok, connected.ErrorCode);
            await PumpUntil(orchestrator,
                () => presenter.Screen == ClientUiState.CharacterSelect, 5000);
            Assert.AreEqual(ClientUiState.CharacterSelect, presenter.Screen);

            Result<S2CCharacterCreateResult> created =
                await controller.CreateAsync(
                    "nhan_vat", "class.kim", CancellationToken.None)
                    .ConfigureAwait(false);
            Assert.IsTrue(created.Ok);
            Assert.AreEqual(
                ResultStatus.Success, created.Value.Result.Status);
            ByteString characterId = created.Value.Character.CharacterId;
            Assert.AreEqual(16, characterId.Length);
            await PumpUntil(orchestrator,
                () => orchestrator.CharacterList != null &&
                    orchestrator.CharacterList.Characters.Count == 1, 3000);
            // CharacterList!: non-null + count proven by the PumpUntil above.
            Assert.AreEqual(1, orchestrator.CharacterList!.Characters.Count);

            Result<S2CCharacterAttachOk> attached =
                await controller.AttachAsync(
                    characterId.ToByteArray(), CancellationToken.None)
                    .ConfigureAwait(false);
            Assert.IsTrue(attached.Ok);
            await PumpUntil(orchestrator,
                () => fsm.Phase == SessionPhase.InWorld, 3000);
            Assert.AreEqual(SessionPhase.InWorld, fsm.Phase);
            Assert.AreEqual(ClientUiState.InWorld, presenter.Screen);

            Result<S2CCharacterDetachOk> detached =
                await controller.DetachAsync(CancellationToken.None)
                    .ConfigureAwait(false);
            Assert.IsTrue(detached.Ok);
            await PumpUntil(orchestrator,
                () => fsm.Phase == SessionPhase.CharacterSelect, 3000);
            Assert.AreEqual(SessionPhase.CharacterSelect, fsm.Phase);
            Assert.AreEqual(ClientUiState.CharacterSelect, presenter.Screen);

            await server.SendSessionReplacedAsync();
            await PumpUntil(orchestrator,
                () => presenter.Modal == SessionModal.SessionReplaced, 3000);
            Assert.AreEqual(SessionModal.SessionReplaced, presenter.Modal);
            Assert.AreEqual(ClientUiState.AuthTitle, presenter.Screen);
            await orchestrator.ShutdownAsync();
        }
    }
}
