using UnityEngine;

namespace ThinhThan.Core.Session
{
    /// <summary><see cref="ISessionStorage"/> backed by PlayerPrefs.</summary>
    public sealed class PlayerPrefsSessionStorage : ISessionStorage
    {
        public bool TryGet(string key, out string? value)
        {
            if (PlayerPrefs.HasKey(key))
            {
                value = PlayerPrefs.GetString(key);
                return true;
            }

            value = string.Empty;
            return false;
        }

        public void Set(string key, string value)
        {
            PlayerPrefs.SetString(key, value);
            PlayerPrefs.Save();
        }

        public void Delete(string key)
        {
            PlayerPrefs.DeleteKey(key);
        }
    }
}
