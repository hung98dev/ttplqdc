using System;

namespace ThinhThan.Core.Session
{
    /// <summary>
    /// Persists the durable slice of <see cref="SessionCredentials"/>:
    /// device_id (UUIDv4 generated once on first launch — never a hardware
    /// fingerprint, auth.md § Device ID), refresh credential, resume
    /// credential, and account id.
    /// </summary>
    public sealed class SessionStore
    {
        public const string DeviceIdKey = "tt.session.device_id";

        public const string AccountIdKey = "tt.session.account_id";

        public const string RefreshTokenKey = "tt.session.refresh_token";

        public const string RefreshExpiresKey = "tt.session.refresh_expires_ms";

        public const string ResumeCredentialKey = "tt.session.resume_credential";

        public const string ResumeExpiresKey = "tt.session.resume_expires_ms";

        private readonly ISessionStorage _storage;

        public SessionStore(ISessionStorage storage)
        {
            _storage = storage ?? throw new ArgumentNullException(nameof(storage));
        }

        /// <summary>Loads persisted values into <paramref name="credentials"/>,
        /// generating and persisting a fresh device_id when absent.</summary>
        public void Load(SessionCredentials credentials)
        {
            if (credentials == null)
            {
                throw new ArgumentNullException(nameof(credentials));
            }

            if (_storage.TryGet(DeviceIdKey, out string? deviceText) &&
                Guid.TryParse(deviceText, out Guid deviceId))
            {
                credentials.DeviceId = deviceId.ToByteArray();
            }
            else
            {
                credentials.DeviceId = Guid.NewGuid().ToByteArray();
                _storage.Set(DeviceIdKey, new Guid(credentials.DeviceId).ToString());
            }

            if (_storage.TryGet(AccountIdKey, out string? accountId))
            {
                credentials.AccountId = accountId ?? string.Empty;
            }

            if (_storage.TryGet(RefreshTokenKey, out string? refreshToken))
            {
                credentials.RefreshToken = refreshToken ?? string.Empty;
            }

            if (_storage.TryGet(RefreshExpiresKey, out string? refreshMs) &&
                long.TryParse(refreshMs, out long refreshExpires))
            {
                credentials.RefreshExpiresAtMs = refreshExpires;
            }

            if (_storage.TryGet(ResumeCredentialKey, out string? resume))
            {
                credentials.ResumeCredential = resume ?? string.Empty;
            }

            if (_storage.TryGet(ResumeExpiresKey, out string? resumeMs) &&
                long.TryParse(resumeMs, out long resumeExpires))
            {
                credentials.ResumeExpiresAtMs = resumeExpires;
            }
        }

        /// <summary>Persists refresh + resume credentials (rotation-safe).</summary>
        public void Save(SessionCredentials credentials)
        {
            if (credentials == null)
            {
                throw new ArgumentNullException(nameof(credentials));
            }

            _storage.Set(AccountIdKey, credentials.AccountId);
            _storage.Set(RefreshTokenKey, credentials.RefreshToken);
            _storage.Set(
                RefreshExpiresKey,
                credentials.RefreshExpiresAtMs.ToString());
            _storage.Set(ResumeCredentialKey, credentials.ResumeCredential);
            _storage.Set(
                ResumeExpiresKey,
                credentials.ResumeExpiresAtMs.ToString());
        }

        /// <summary>Wipes everything except device_id (logout/erasure).</summary>
        public void ClearCredentials()
        {
            _storage.Delete(AccountIdKey);
            _storage.Delete(RefreshTokenKey);
            _storage.Delete(RefreshExpiresKey);
            _storage.Delete(ResumeCredentialKey);
            _storage.Delete(ResumeExpiresKey);
        }
    }
}
