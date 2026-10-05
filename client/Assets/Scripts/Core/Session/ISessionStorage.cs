namespace ThinhThan.Core.Session
{
    /// <summary>
    /// Minimal string-keyed durable store backing <see cref="SessionStore"/>.
    /// Implementations: PlayerPrefs-backed on the client, in-memory in tests.
    /// </summary>
    public interface ISessionStorage
    {
        bool TryGet(string key, out string? value);

        void Set(string key, string value);

        void Delete(string key);
    }
}
