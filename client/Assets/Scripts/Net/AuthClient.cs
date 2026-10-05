using System;
using System.Net.Http;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using ThinhThan.Core.Session;
using UnityEngine;

namespace ThinhThan.Net
{
    /// <summary>
    /// HTTPS control-plane client (auth.md § HTTPS Endpoints): JSON bodies,
    /// Bearer access token, canonical error body mapping. Never carries
    /// gameplay payloads — the control plane is auth/account/ticket only.
    /// </summary>
    public class AuthClient
    {
        private readonly HttpClient _http;
        private readonly string _baseUrl;

        public AuthClient(string baseUrl, HttpMessageHandler? handler = null)
        {
            _baseUrl = baseUrl.TrimEnd('/');
            _http = handler == null ? new HttpClient() : new HttpClient(handler);
        }

        /// <summary>POST /api/v1/auth/password/register.</summary>
        public Task<Result<TokenResponseDto>> RegisterPasswordAsync(
            string username, string password, string email,
            SessionCredentials creds, CancellationToken cancel)
        {
            var body = new PasswordRegisterRequest
            {
                username = username,
                password = password,
                email = email,
                device_id = DeviceText(creds),
                client_build = creds.ClientBuild,
                platform = "WINDOWS",
            };
            return PostAsync<PasswordRegisterRequest, TokenResponseDto>(
                "/api/v1/auth/password/register", body, null, cancel);
        }

        /// <summary>POST /api/v1/auth/password/login.</summary>
        public Task<Result<TokenResponseDto>> LoginPasswordAsync(
            string username, string password,
            SessionCredentials creds, CancellationToken cancel)
        {
            var body = new PasswordLoginRequest
            {
                username = username,
                password = password,
                device_id = DeviceText(creds),
                client_build = creds.ClientBuild,
                platform = "WINDOWS",
            };
            return PostAsync<PasswordLoginRequest, TokenResponseDto>(
                "/api/v1/auth/password/login", body, null, cancel);
        }

        /// <summary>POST /api/v1/auth/refresh (rotation per auth.md).</summary>
        public Task<Result<TokenResponseDto>> RefreshAsync(
            SessionCredentials creds, CancellationToken cancel)
        {
            var body = new RefreshRequest
            {
                refresh_token = creds.RefreshToken,
                device_id = DeviceText(creds),
            };
            return PostAsync<RefreshRequest, TokenResponseDto>(
                "/api/v1/auth/refresh", body, null, cancel);
        }

        /// <summary>POST /api/v1/auth/logout scope=SESSION.</summary>
        public async Task<Result<bool>> LogoutAsync(
            SessionCredentials creds, CancellationToken cancel)
        {
            var body = new LogoutRequest { scope = "SESSION" };
            Result<EmptyResponse> result = await PostAsync<LogoutRequest, EmptyResponse>(
                "/api/v1/auth/logout", body, creds.AccessToken, cancel)
                .ConfigureAwait(false);
            return result.Ok
                ? Result<bool>.Success(true)
                : Result<bool>.Failure(result.ErrorCode, result.RetryAfterMs);
        }

        /// <summary>GET /api/v1/account.</summary>
        public Task<Result<AccountDto>> GetAccountAsync(
            SessionCredentials creds, CancellationToken cancel)
        {
            return SendAsync<AccountDto>(
                HttpMethod.Get, "/api/v1/account", null, creds.AccessToken,
                cancel);
        }

        /// <summary>POST /api/v1/gameplay/ticket — the only admission gate
        /// (session.md § Login Queue); SERVER_OVERLOADED carries
        /// queue_position + retry_after_ms.</summary>
        public Task<Result<TicketResponseDto>> RequestTicketAsync(
            SessionCredentials creds, CancellationToken cancel)
        {
            var body = new TicketRequest
            {
                client_build = creds.ClientBuild,
                platform = "WINDOWS",
                protocol_major = WireIds.ProtocolMajor,
                protocol_minor = WireIds.ProtocolMinor,
                content_revision = creds.ContentRevision,
            };
            return PostAsync<TicketRequest, TicketResponseDto>(
                "/api/v1/gameplay/ticket", body, creds.AccessToken, cancel);
        }

        protected virtual async Task<Result<TResponse>> PostAsync<TRequest, TResponse>(
            string path, TRequest body, string? bearer, CancellationToken cancel)
            where TResponse : class, new()
        {
            string json = JsonUtility.ToJson(body);
            var content = new StringContent(json, Encoding.UTF8, "application/json");
            var request = new HttpRequestMessage(HttpMethod.Post, _baseUrl + path)
            {
                Content = content,
            };
            return await SendCoreAsync<TResponse>(request, bearer, cancel)
                .ConfigureAwait(false);
        }

        private async Task<Result<TResponse>> SendAsync<TResponse>(
            HttpMethod method, string path, HttpContent? content, string? bearer,
            CancellationToken cancel)
            where TResponse : class, new()
        {
            var request = new HttpRequestMessage(method, _baseUrl + path)
            {
                Content = content,
            };
            return await SendCoreAsync<TResponse>(request, bearer, cancel)
                .ConfigureAwait(false);
        }

        private async Task<Result<TResponse>> SendCoreAsync<TResponse>(
            HttpRequestMessage request, string? bearer, CancellationToken cancel)
            where TResponse : class, new()
        {
            try
            {
                if (bearer != null)
                {
                    request.Headers.Authorization =
                        new System.Net.Http.Headers.AuthenticationHeaderValue(
                            "Bearer", bearer);
                }

                HttpResponseMessage response = await _http
                    .SendAsync(request, cancel).ConfigureAwait(false);
                string text = await response.Content
                    .ReadAsStringAsync().ConfigureAwait(false);
                if (!response.IsSuccessStatusCode)
                {
                    var error = JsonUtility.FromJson<AuthErrorBody>(text);
                    if (error == null || error.error_code.Length == 0)
                    {
                        return Result<TResponse>.Failure(
                            "TEMPORARY_DEPENDENCY_FAILURE");
                    }

                    return Result<TResponse>.Failure(
                        error.error_code, error.retry_after_ms);
                }

                var dto = JsonUtility.FromJson<TResponse>(text);
                return dto == null
                    ? Result<TResponse>.Failure("PROTOCOL_MALFORMED")
                    : Result<TResponse>.Success(dto);
            }
            catch (OperationCanceledException)
            {
                throw;
            }
            catch (Exception)
            {
                return Result<TResponse>.Failure(
                    "TEMPORARY_DEPENDENCY_FAILURE");
            }
        }

        private static string DeviceText(SessionCredentials creds)
        {
            return creds.DeviceId.Length == 16
                ? new Guid(creds.DeviceId).ToString()
                : string.Empty;
        }

        [Serializable]
        private sealed class PasswordRegisterRequest
        {
            public string username = string.Empty;
            public string password = string.Empty;
            public string email = string.Empty;
            public string device_id = string.Empty;
            public uint client_build;
            public string platform = string.Empty;
        }

        [Serializable]
        private sealed class PasswordLoginRequest
        {
            public string username = string.Empty;
            public string password = string.Empty;
            public string device_id = string.Empty;
            public uint client_build;
            public string platform = string.Empty;
        }

        [Serializable]
        private sealed class RefreshRequest
        {
            public string refresh_token = string.Empty;
            public string device_id = string.Empty;
        }

        [Serializable]
        private sealed class LogoutRequest
        {
            public string scope = string.Empty;
        }

        [Serializable]
        private sealed class TicketRequest
        {
            public uint client_build;
            public string platform = string.Empty;
            public uint protocol_major;
            public uint protocol_minor;
            public string content_revision = string.Empty;
        }

        [Serializable]
        private sealed class EmptyResponse
        {
        }
    }
}
