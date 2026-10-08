using System.Collections.Generic;
using System.Net;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using NUnit.Framework;
using ThinhThan.Core.Performance;
using ThinhThan.Core.Rendering;
using ThinhThan.Core.Runtime;
using ThinhThan.Core.Session;
using ThinhThan.Net;
using ThinhThan.UI.Screens;
using UnityEngine;

namespace ThinhThan.Tests.PlayMode.Screens
{
    /// <summary>
    /// IMP-099 screen presenters (client_experience_contract.md §1):
    /// login/register, login-queue display + cancel, settings persistence
    /// and credits resolution. Headless-safe: views are never instantiated,
    /// every dependency is a fake seam (IMP-066 pattern).
    /// </summary>
    public sealed class ScreensTests
    {
        private sealed class FakeClock : IClock
        {
            public double UnscaledDeltaSeconds
            {
                get;
                set;
            }

            public double NowSeconds
            {
                get;
                set;
            }
        }

        private sealed class FakeFrameTime : IFrameTimeSource
        {
            public double FrameSeconds
            {
                get;
                set;
            }
        }

        private sealed class StubHandler : HttpMessageHandler
        {
            private readonly string _body;
            private readonly HttpStatusCode _status;

            public StubHandler(HttpStatusCode status, string body)
            {
                _status = status;
                _body = body;
            }

            protected override Task<HttpResponseMessage> SendAsync(
                HttpRequestMessage request, CancellationToken cancellationToken)
            {
                var response = new HttpResponseMessage(_status);
                response.Content = new StringContent(_body);
                return Task.FromResult(response);
            }
        }

        private const string TokenBody =
            "{\"account_id\":\"acc-1\",\"access_token\":\"at\",\"refresh_token\":\"rt\"," +
            "\"access_expires_at\":\"2030-01-01T00:00:00Z\"," +
            "\"refresh_expires_at\":\"2030-02-01T00:00:00Z\"}";

        /// <summary>
        /// Login + register both funnel through auth → store → connect; a
        /// wrong password stays generic (AUTH_INVALID, no oracle).
        /// </summary>
        [Test]
        public async Task TestLoginRegisterFlow()
        {
            var storage = new InMemorySessionStorage();
            var store = new SessionStore(storage);
            var credentials = new SessionCredentials();
            var connects = 0;

            var auth = new AuthClient(
                "https://auth.test",
                new StubHandler(HttpStatusCode.OK, TokenBody));
            var flow = new AuthFlow(
                auth, credentials, store,
                delegate(CancellationToken c)
                {
                    connects++;
                    return Task.FromResult(Result<bool>.Success(true));
                },
                (provider, creds, c) =>
                    Task.FromResult(Result<TokenResponseDto>.Success(
                        new TokenResponseDto())));
            var presenter = new AuthTitlePresenter(flow);

            await presenter.LoginAsync("player1", "secretpw", CancellationToken.None);
            Assert.AreEqual(1, connects);
            Assert.AreEqual(string.Empty, presenter.MessageKey);
            Assert.AreEqual("at", credentials.AccessToken);
            Assert.IsTrue(storage.TryGet("session.access_token", out string? saved));
            Assert.AreEqual("at", saved);

            await presenter.RegisterAsync(
                "player2", "secretpw", "p2@example.com", CancellationToken.None);
            Assert.AreEqual(2, connects);
            Assert.IsFalse(presenter.Busy);

            var failAuth = new AuthClient(
                "https://auth.test",
                new StubHandler(
                    HttpStatusCode.Unauthorized,
                    "{\"error_code\":\"AUTH_INVALID\"}"));
            var failFlow = new AuthFlow(
                failAuth, new SessionCredentials(), store,
                c => Task.FromResult(Result<bool>.Success(true)),
                (provider, creds, c) =>
                    Task.FromResult(Result<TokenResponseDto>.Failure("AUTH_INVALID")));
            var failPresenter = new AuthTitlePresenter(failFlow);
            await failPresenter.LoginAsync("player1", "wrong", CancellationToken.None);
            Assert.AreEqual(ScreensLoc.AuthInvalid, failPresenter.MessageKey);
        }

        /// <summary>LOGIN_QUEUED surfaces the server queue_position (§1).</summary>
        [Test]
        public void TestLoginQueueDisplay()
        {
            var position = 7;
            var cancelled = false;
            var presenter = new LoginQueuePresenter(
                () => position, () => cancelled = true);

            Assert.AreEqual(7, presenter.Position);
            Assert.AreEqual(ScreensLoc.QueuePosition, presenter.PositionKey);
            Assert.AreEqual(ScreensLoc.QueueRetrying, presenter.RetryingKey);

            position = 3;
            Assert.AreEqual(3, presenter.Position);
            Assert.IsFalse(cancelled);
        }

        /// <summary>Cancel on LOGIN_QUEUED returns to AUTH_TITLE (§1).</summary>
        [Test]
        public void TestLoginQueueCancelReturnsToTitle()
        {
            var cancelled = false;
            var presenter = new LoginQueuePresenter(
                () => 5, () => cancelled = true);

            presenter.Cancel();
            Assert.IsTrue(cancelled);
        }

        /// <summary>
        /// Settings persist through IMP-095: quality.preset round-trips via
        /// QualityBenchmark.Persist and the battery-saver toggle persists its
        /// key while capping the frame rate at 30 (PERF-013).
        /// </summary>
        [Test]
        public void TestSettingsPresetAndBatterySaver()
        {
            var storage = new InMemorySessionStorage();
            var clock = new FakeClock();
            var governor = new QualityGovernor(
                QualityPreset.Medium, 1.0 / 60.0, clock, new FakeFrameTime());
            var saver = new BatterySaver(storage);
            var baseline = Application.targetFrameRate;
            var applied = QualityPreset.Medium;
            var locales = new List<string>();
            var presenter = new SettingsPresenter(
                storage, governor, saver,
                preset => applied = preset,
                code => locales.Add(code));

            try
            {
                Assert.AreEqual(QualityPreset.Medium, presenter.Preset);

                presenter.SetPreset(QualityPreset.High);
                Assert.AreEqual(QualityPreset.High, applied);
                Assert.IsTrue(
                    QualityBenchmark.TryGetPersisted(storage, out QualityPreset persisted));
                Assert.AreEqual(QualityPreset.High, persisted);

                presenter.SetBatterySaver(true);
                Assert.IsTrue(storage.TryGet("quality.battery_saver", out string? flag));
                Assert.AreEqual("1", flag);
                Assert.AreEqual(30, Application.targetFrameRate);
                Assert.IsTrue(presenter.BatterySaverEnabled);

                presenter.SetBatterySaver(false);
                Assert.AreNotEqual(30, Application.targetFrameRate);

                presenter.SetLocale("en-US");
                Assert.AreEqual("en-US", locales[0]);
            }
            finally
            {
                Application.targetFrameRate = baseline;
            }
        }

        /// <summary>
        /// Credits resolves the generated notice via the canonical
        /// addressables key (client_assets.md §118); a failed load surfaces
        /// the unavailable key path.
        /// </summary>
        [Test]
        public async Task TestCreditsKeyResolves()
        {
            string requested = string.Empty;
            var presenter = new CreditsPresenter(
                (key, c) =>
                {
                    requested = key;
                    return Task.FromResult("# Credits\nasset");
                });

            await presenter.LoadAsync(CancellationToken.None);
            Assert.AreEqual("asset.ui.credits.text", requested);
            Assert.AreEqual("# Credits\nasset", presenter.Text);
            Assert.IsFalse(presenter.Unavailable);

            var failing = new CreditsPresenter(
                (key, c) => Task.FromResult(string.Empty));
            await failing.LoadAsync(CancellationToken.None);
            Assert.IsTrue(failing.Unavailable);
        }
    }
}
