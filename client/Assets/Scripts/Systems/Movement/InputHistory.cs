namespace ThinhThan.Systems.Movement
{
    /// <summary>
    /// Bounded seq→<see cref="InputRecord"/> ring (capacity 128) feeding
    /// self-ack replay. Oldest entries are overwritten when full — records
    /// older than any plausible ack latency are no longer replayable anyway.
    /// </summary>
    public sealed class InputHistory
    {
        private readonly InputRecord[] _ring =
            new InputRecord[MovementConstants.InputHistoryCapacity];

        private int _head;
        private int _count;

        /// <summary>Number of retained records.</summary>
        public int Count
        {
            get
            {
                return _count;
            }
        }

        /// <summary>Oldest retained seq, or 0 when empty.</summary>
        public ulong OldestSeq
        {
            get
            {
                return _count == 0 ? 0UL : _ring[_head].Seq;
            }
        }

        /// <summary>Appends one record; overwrites the oldest when full.</summary>
        public void Push(in InputRecord record)
        {
            int index = (_head + _count) % _ring.Length;
            _ring[index] = record;
            if (_count == _ring.Length)
            {
                _head = (_head + 1) % _ring.Length;
            }
            else
            {
                _count++;
            }
        }

        /// <summary>Drops records with seq &lt;= <paramref name="seq"/>.</summary>
        public void DropThrough(ulong seq)
        {
            while (_count > 0 && _ring[_head].Seq <= seq)
            {
                _head = (_head + 1) % _ring.Length;
                _count--;
            }
        }

        /// <summary>
        /// Replays records with seq &gt; <paramref name="afterSeq"/> in send
        /// order through <paramref name="visitor"/> — allocation-free.
        /// </summary>
        public void ForEachAfter(ulong afterSeq, InputHistoryVisitor visitor)
        {
            for (int i = 0; i < _count; i++)
            {
                InputRecord r = _ring[(_head + i) % _ring.Length];
                if (r.Seq > afterSeq)
                {
                    visitor(in r);
                }
            }
        }

        /// <summary>Removes every record.</summary>
        public void Clear()
        {
            _head = 0;
            _count = 0;
        }
    }

    /// <summary>Allocation-free record callback.</summary>
    public delegate void InputHistoryVisitor(in InputRecord record);
}
