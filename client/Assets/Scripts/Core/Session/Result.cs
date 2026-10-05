namespace ThinhThan.Core.Session
{
    /// <summary>
    /// Canonical error/result carrier for session and control-plane calls
    /// (engineering_conventions.md §2.6 — Core owns errors/results). The
    /// string code is the stable wire error_code (errors.md), not free text.
    /// </summary>
    public readonly struct Result<T>
    {
        private readonly T _value;

        private Result(T value, string errorCode, long retryAfterMs)
        {
            _value = value;
            ErrorCode = errorCode;
            RetryAfterMs = retryAfterMs;
        }

        public bool Ok
        {
            get
            {
                return ErrorCode.Length == 0;
            }
        }

        public T Value
        {
            get
            {
                return _value;
            }
        }

        /// <summary>Empty when <see cref="Ok"/>.</summary>
        public string ErrorCode
        {
            get;
        }

        /// <summary>retry_after_ms carried by backoff-class errors.</summary>
        public long RetryAfterMs
        {
            get;
        }

        public static Result<T> Success(T value)
        {
            return new Result<T>(value, string.Empty, 0L);
        }

        public static Result<T> Failure(string errorCode, long retryAfterMs = 0L)
        {
            // default! : a failure result never exposes a value; callers only
            // read Value after Ok, which is false here.
            return new Result<T>(default!, errorCode, retryAfterMs);
        }
    }
}
