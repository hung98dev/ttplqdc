using System;
using System.Threading;
using System.Threading.Tasks;
using ThinhThan.Core.Session;
using ThinhThan.Net;

namespace ThinhThan.UI.Screens
{
    /// <summary>
    /// Login / register driver between the title screen and the session
    /// orchestrator (client_experience_contract.md §1 AUTH_TITLE):
    /// password login + registration and the federated entry points all
    /// funnel into <c>SessionOrchestrator.ConnectWithTicketAsync</c>, which
    /// owns the SERVER_OVERLOADED → LOGIN_QUEUED retry loop
    /// (queue_position + retry_after_ms are server-owned). Successful token
    /// responses persist through <see cref="SessionStore"/> before connect.
    /// Errors surface as <see cref="Result{T}"/> error codes — the view maps
    /// them through <see cref="AuthErrorMap"/> so AUTH_INVALID stays generic.
    /// </summary>
    public sealed class AuthFlow
    {
        /// <summary>Federated sign-in entry (Apple / Google / Steam).</summary>
        public enum Provider
        {
            Apple,
            Google,
            Steam,
        }

        private readonly AuthClient _auth;
        private readonly SessionCredentials _credentials;
        private readonly SessionStore _store;
        private readonly Func<CancellationToken, Task<Result<bool>>> _connect;
        private readonly Func<Provider, SessionCredentials, CancellationToken, Task<Result<TokenResponseDto>>> _federatedLogin;

        /// <param name="connect">Bound to SessionOrchestrator.ConnectWithTicketAsync.</param>
        /// <param name="federatedLogin">Provider → token seam; composition binds the
        /// real platform bridges, tests fake it.</param>
        public AuthFlow(
            AuthClient auth,
            SessionCredentials credentials,
            SessionStore store,
            Func<CancellationToken, Task<Result<bool>>> connect,
            Func<Provider, SessionCredentials, CancellationToken, Task<Result<TokenResponseDto>>> federatedLogin)
        {
            _auth = auth ?? throw new ArgumentNullException(nameof(auth));
            _credentials = credentials ??
                throw new ArgumentNullException(nameof(credentials));
            _store = store ?? throw new ArgumentNullException(nameof(store));
            _connect = connect ?? throw new ArgumentNullException(nameof(connect));
            _federatedLogin = federatedLogin ??
                throw new ArgumentNullException(nameof(federatedLogin));
        }

        /// <summary>POST password login → store → ticket connect.</summary>
        public async Task<Result<bool>> LoginPasswordAsync(
            string username, string password, CancellationToken cancel)
        {
            Result<TokenResponseDto> token = await _auth
                .LoginPasswordAsync(username, password, _credentials, cancel)
                .ConfigureAwait(false);
            if (!token.Ok)
            {
                return Result<bool>.Failure(token.ErrorCode, token.RetryAfterMs);
            }

            ApplyToken(token.Value);
            return await _connect(cancel).ConfigureAwait(false);
        }

        /// <summary>POST password register → store → ticket connect.</summary>
        public async Task<Result<bool>> RegisterPasswordAsync(
            string username, string password, string email,
            CancellationToken cancel)
        {
            Result<TokenResponseDto> token = await _auth
                .RegisterPasswordAsync(username, password, email, _credentials, cancel)
                .ConfigureAwait(false);
            if (!token.Ok)
            {
                return Result<bool>.Failure(token.ErrorCode, token.RetryAfterMs);
            }

            ApplyToken(token.Value);
            return await _connect(cancel).ConfigureAwait(false);
        }

        /// <summary>Federated provider login → store → ticket connect.</summary>
        public async Task<Result<bool>> LoginFederatedAsync(
            Provider provider, CancellationToken cancel)
        {
            Result<TokenResponseDto> token = await _federatedLogin(
                provider, _credentials, cancel).ConfigureAwait(false);
            if (!token.Ok)
            {
                return Result<bool>.Failure(token.ErrorCode, token.RetryAfterMs);
            }

            ApplyToken(token.Value);
            return await _connect(cancel).ConfigureAwait(false);
        }

        /// <summary>Applies token fields onto credentials + persists.</summary>
        private void ApplyToken(TokenResponseDto token)
        {
            _credentials.AccountId = token.account_id ?? string.Empty;
            _credentials.AccessToken = token.access_token ?? string.Empty;
            _credentials.AccessExpiresAtMs = ParseExpiresMs(token.access_expires_at);
            _credentials.RefreshToken = token.refresh_token ?? string.Empty;
            _credentials.RefreshExpiresAtMs = ParseExpiresMs(token.refresh_expires_at);
            _store.Save(_credentials);
        }

        /// <summary>RFC3339 auth timestamps → unix ms (auth.md contract).</summary>
        internal static long ParseExpiresMs(string value)
        {
            if (DateTimeOffset.TryParse(
                value, System.Globalization.CultureInfo.InvariantCulture,
                System.Globalization.DateTimeStyles.RoundtripKind,
                out DateTimeOffset parsed))
            {
                return parsed.ToUnixTimeMilliseconds();
            }

            return 0L;
        }
    }
}
