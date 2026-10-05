using System;

namespace ThinhThan.Net
{
    /// <summary>
    /// Error body shared by every HTTPS endpoint (auth.md § Errors /
    /// errors.md § Error Shape): HTTP status + stable error_code drives
    /// client behavior.
    /// </summary>
    [Serializable]
    public sealed class AuthErrorBody
    {
        public string error_code = string.Empty;

        public string retryability = string.Empty;

        public long retry_after_ms;

        public int queue_position;

        public string safe_message_key = string.Empty;
    }
}
