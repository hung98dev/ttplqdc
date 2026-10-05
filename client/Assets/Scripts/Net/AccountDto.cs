using System;

namespace ThinhThan.Net
{
    /// <summary>Response of GET /api/v1/account.</summary>
    [Serializable]
    public sealed class AccountDto
    {
        public string account_id = string.Empty;

        public ProviderLinkDto[] providers = Array.Empty<ProviderLinkDto>();

        public string status = string.Empty;

        public bool pending_deletion;

        public long deletion_scheduled_at;
    }
}
