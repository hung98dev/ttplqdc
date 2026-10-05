using System;

namespace ThinhThan.Net
{
    /// <summary>One entry of AccountDto.providers.</summary>
    [Serializable]
    public sealed class ProviderLinkDto
    {
        public string provider_id = string.Empty;

        public long linked_at;
    }
}
