namespace ThinhThan.Core.Session
{
    /// <summary>
    /// The credential set the client carries between the HTTPS control plane
    /// and the WSS session (auth.md § Credential Types, session.md § Resume).
    /// Tokens are opaque server values; never logged.
    /// </summary>
    public sealed class SessionCredentials
    {
        /// <summary>device_id — UUIDv4 generated on first launch, persisted.</summary>
        public byte[] DeviceId = System.Array.Empty<byte>();

        public string AccountId = string.Empty;

        public string AccessToken = string.Empty;

        public long AccessExpiresAtMs;

        public string RefreshToken = string.Empty;

        public long RefreshExpiresAtMs;

        /// <summary>Latest resume credential from HELLO_OK or message 16.</summary>
        public string ResumeCredential = string.Empty;

        /// <summary>Second newest resume credential; at most two are valid.</summary>
        public string PreviousResumeCredential = string.Empty;

        public long ResumeExpiresAtMs;

        public ulong SessionEpoch;

        public byte[] SessionId = System.Array.Empty<byte>();

        public byte[] ResumedCharacterId = System.Array.Empty<byte>();

        public uint ClientBuild;

        public string ContentRevision = string.Empty;

        /// <summary>True when a resume credential exists and is not expired.</summary>
        public bool HasUsableResume(long nowMs)
        {
            return ResumeCredential.Length != 0 && ResumeExpiresAtMs > nowMs;
        }

        /// <summary>Rotate: newest becomes previous, previous drops.</summary>
        public void AcceptResumeRotation(string credential, long expiresAtMs)
        {
            PreviousResumeCredential = ResumeCredential;
            ResumeCredential = credential;
            ResumeExpiresAtMs = expiresAtMs;
        }
    }
}
