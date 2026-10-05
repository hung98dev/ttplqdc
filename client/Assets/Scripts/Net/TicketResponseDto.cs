using System;

namespace ThinhThan.Net
{
    /// <summary>Response of POST /api/v1/gameplay/ticket.</summary>
    [Serializable]
    public sealed class TicketResponseDto
    {
        public string ticket = string.Empty;

        public string ticket_expires_at = string.Empty;

        public string wss_url = string.Empty;

        public uint protocol_minor_min;

        public uint client_build_min;

        public string content_revision = string.Empty;
    }
}
