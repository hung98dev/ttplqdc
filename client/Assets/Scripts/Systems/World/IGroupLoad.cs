namespace ThinhThan.Systems.World
{
    /// <summary>
    /// An in-flight region-group load started by <see cref="MapLoadPipeline"/>.
    /// Composition wraps <c>AsyncOperationHandle</c>; tests fake it.
    /// </summary>
    public interface IGroupLoad
    {
        /// <summary>Whether the load has settled (success or failure).</summary>
        bool Done
        {
            get;
        }

        /// <summary>Whether every asset in the group resolved.</summary>
        bool Succeeded
        {
            get;
        }
    }
}
