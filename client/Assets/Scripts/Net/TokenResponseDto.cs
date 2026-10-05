using System;

namespace ThinhThan.Net
{
    /// <summary>
    /// TokenResponse wire shape of auth.md § HTTPS Endpoints — flat JSON,
    /// timestamps int64 Unix ms.
    /// </summary>
    [Serializable]
    public sealed class TokenResponseDto
    {
        public string account_id = string.Empty;

        public string access_token = string.Empty;

        public string access_expires_at = string.Empty;

        public string refresh_token = string.Empty;

        public string refresh_expires_at = string.Empty;

        public bool is_new_account;
    }
}
