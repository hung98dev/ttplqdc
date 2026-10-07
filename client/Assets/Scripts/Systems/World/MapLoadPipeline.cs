using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using ThinhThan.Core.Runtime;

namespace ThinhThan.Systems.World
{
    /// <summary>
    /// PERF-018 facet: destination map load gate for TRANSFERRING_MAP.
    /// <c>Begin(mapId)</c> pre-sizes the pooled view rows
    /// (actors / projectiles / VFX / floating text / UI rows — the caller's
    /// content counts) and requests the Addressables
    /// <c>region.&lt;zone&gt;</c> group through the injected loader seam
    /// (composition binds <c>Addressables.LoadAssetsAsync</c>; tests fake it).
    /// <see cref="Ready"/> becomes true only when every prewarm applied and
    /// the group load resolved — it gates the 106 C2S_PRESENTATION_READY
    /// the FSM emits while TRANSFERRING_MAP.
    /// A failed load marks the pipeline <see cref="Status.Failed"/> — no
    /// client-chosen fallback and no world-state mutation happens here.
    /// </summary>
    public sealed class MapLoadPipeline
    {
        /// <summary>Pipeline status.</summary>
        public enum Status
        {
            /// <summary>No load in flight.</summary>
            Idle = 0,

            /// <summary>Group load in flight.</summary>
            Loading = 1,

            /// <summary>Pre sized + group resolved — 106 may fire.</summary>
            Ready = 2,

            /// <summary>Load failed — surface the failure upstream.</summary>
            Failed = 3,
        }

        private readonly Func<string, CancellationToken, Task<bool>> _loadGroup;
        private readonly List<Action<int>> _prewarmApply = new List<Action<int>>();
        private readonly List<int> _prewarmCounts = new List<int>();
        private readonly List<string> _prewarmNames = new List<string>();

        /// <summary>
        /// loadGroup resolves the named Addressables group; returns whether
        /// every asset in the group resolved.
        /// </summary>
        public MapLoadPipeline(Func<string, CancellationToken, Task<bool>> loadGroup)
        {
            _loadGroup = loadGroup ?? throw new ArgumentNullException(nameof(loadGroup));
        }

        /// <summary>Current status.</summary>
        public Status Current
        {
            get;
            private set;
        }

        /// <summary>Convenience for <c>Current == Ready</c>.</summary>
        public bool Ready
        {
            get
            {
                return Current == Status.Ready;
            }
        }

        /// <summary>The map currently being loaded / last loaded.</summary>
        public string ActiveMapId
        {
            get;
            private set;
        } = "";

        /// <summary>The group key requested for the active map.</summary>
        public string ActiveGroupKey
        {
            get;
            private set;
        } = "";

        /// <summary>Number of registered prewarm steps.</summary>
        public int PrewarmCount
        {
            get
            {
                return _prewarmApply.Count;
            }
        }

        /// <summary>Registers a pool pre-size step (name for diagnostics).</summary>
        public void AddPrewarm(string name, Action<int> apply, int count)
        {
            if (apply == null)
            {
                throw new ArgumentNullException(nameof(apply));
            }
            _prewarmNames.Add(name);
            _prewarmApply.Add(apply);
            _prewarmCounts.Add(count < 0 ? 0 : count);
        }

        /// <summary>Registers a <see cref="Pool{T}"/> to pre-size at Begin.</summary>
        public void AddPool<T>(Pool<T> pool, int count)
        {
            if (pool == null)
            {
                throw new ArgumentNullException(nameof(pool));
            }
            AddPrewarm(typeof(T).Name, pool.Prewarm, count);
        }

        /// <summary>
        /// Starts the load for a destination map: pre-sizes every registered
        /// pool from its content count, then awaits the region group load.
        /// Unknown map_id or a failed group load -> Failed, false.
        /// </summary>
        public async Task<bool> Begin(string mapId, CancellationToken cancel = default)
        {
            if (!WorldMapRegistry.TryGet(mapId, out var rec))
            {
                Current = Status.Failed;
                return false;
            }
            Current = Status.Loading;
            ActiveMapId = mapId;
            ActiveGroupKey = rec.GroupKey;
            for (var i = 0; i < _prewarmApply.Count; i++)
            {
                _prewarmApply[i](_prewarmCounts[i]);
            }
            bool ok;
            try
            {
                ok = await _loadGroup(rec.GroupKey, cancel);
            }
            catch (OperationCanceledException)
            {
                Current = Status.Idle;
                return false;
            }
            catch (Exception)
            {
                ok = false;
            }
            Current = ok ? Status.Ready : Status.Failed;
            return ok;
        }

        /// <summary>Back to idle (transfer done / aborted).</summary>
        public void Reset()
        {
            Current = Status.Idle;
            ActiveMapId = "";
            ActiveGroupKey = "";
        }
    }
}
