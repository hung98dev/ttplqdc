using System.Collections.Generic;

namespace ThinhThan.Core.Session
{
    /// <summary>In-memory <see cref="ISessionStorage"/> for tests.</summary>
    public sealed class InMemorySessionStorage : ISessionStorage
    {
        private readonly Dictionary<string, string> _values =
            new Dictionary<string, string>();

        public bool TryGet(string key, out string? value)
        {
            return _values.TryGetValue(key, out value);
        }

        public void Set(string key, string value)
        {
            _values[key] = value;
        }

        public void Delete(string key)
        {
            _values.Remove(key);
        }
    }
}
