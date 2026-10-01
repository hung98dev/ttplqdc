using System;
using UnityEngine;

namespace ThinhThan.Core.Assets
{
    /// <summary>
    /// Shared-asset indirection (client_assets.md § Stable Asset Keys): a
    /// catalog ID that reuses a shared asset keeps its own key, which addresses
    /// a PresentationAlias whose <see cref="target_key"/> names the shared key.
    /// Resolution follows exactly one alias hop; an alias pointing to an alias
    /// fails validation.
    /// </summary>
    public class PresentationAlias : ScriptableObject
    {
        /// <summary>The shared key this alias resolves to.</summary>
        public string target_key = string.Empty;

        /// <summary>
        /// Resolves at most one alias hop. <paramref name="aliasTargetOf"/>
        /// returns the target_key when the entry at a key is a
        /// PresentationAlias, otherwise null (direct asset or missing).
        /// Returns false when the resolved key is itself an alias.
        /// </summary>
        public static bool TryResolve(string key, Func<string, string?> aliasTargetOf, out string resolved)
        {
            resolved = key;
            var target = aliasTargetOf(key);
            if (target == null)
            {
                return true;
            }
            if (aliasTargetOf(target) != null)
            {
                return false;
            }
            resolved = target;
            return true;
        }

        /// <summary>One-hop resolve; throws when the target is itself an alias.</summary>
        public static string Resolve(string key, Func<string, string?> aliasTargetOf)
        {
            if (!TryResolve(key, aliasTargetOf, out var resolved))
            {
                throw new InvalidOperationException("PresentationAlias chain is longer than one hop at " + key);
            }
            return resolved;
        }
    }
}
