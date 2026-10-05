using System;
using System.Collections.Generic;

namespace ThinhThan.Core.Runtime
{
    /// <summary>
    /// The single first-party object pool (engineering_conventions.md §2.6 —
    /// no second pool implementation). Thread-safe: the network receive pump
    /// rents on a worker thread while the main thread returns entries.
    /// </summary>
    public sealed class Pool<T>
    {
        private readonly Stack<T> _idle = new Stack<T>();
        private readonly Func<T> _factory;
        private readonly Action<T>? _onReturn;
        private readonly int _maxRetained;
        private int _live;

        /// <param name="factory">Creates a new instance when the pool is empty.</param>
        /// <param name="onReturn">Optional reset applied before an instance idles.</param>
        /// <param name="maxRetained">Idle cap; returns beyond it are dropped.</param>
        public Pool(Func<T> factory, Action<T>? onReturn = null, int maxRetained = 1024)
        {
            _factory = factory ?? throw new ArgumentNullException(nameof(factory));
            _onReturn = onReturn;
            _maxRetained = maxRetained;
        }

        public int IdleCount
        {
            get
            {
                lock (_idle)
                {
                    return _idle.Count;
                }
            }
        }

        public int LiveCount
        {
            get
            {
                lock (_idle)
                {
                    return _live;
                }
            }
        }

        /// <summary>Creates <paramref name="count"/> idle instances up front.</summary>
        public void Prewarm(int count)
        {
            if (count < 0)
            {
                throw new ArgumentOutOfRangeException(nameof(count));
            }

            for (int i = 0; i < count; i++)
            {
                T instance = _factory();
                if (_onReturn != null)
                {
                    _onReturn(instance);
                }
                lock (_idle)
                {
                    _idle.Push(instance);
                }
            }
        }

        public T Rent()
        {
            lock (_idle)
            {
                _live++;
                if (_idle.Count > 0)
                {
                    return _idle.Pop();
                }
            }

            return _factory();
        }

        public void Return(T instance)
        {
            if (instance == null)
            {
                throw new ArgumentNullException(nameof(instance));
            }

            if (_onReturn != null)
            {
                _onReturn(instance);
            }

            lock (_idle)
            {
                _live--;
                if (_idle.Count < _maxRetained)
                {
                    _idle.Push(instance);
                }
            }
        }
    }
}
