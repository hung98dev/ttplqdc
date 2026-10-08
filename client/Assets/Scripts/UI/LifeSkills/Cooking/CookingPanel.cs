namespace ThinhThan.UI.LifeSkills.Cooking
{
    /// <summary>
    /// Cooking panel surface: holds the last rendered
    /// <see cref="CookingPanelModel"/> so widgets and tests read
    /// current state. Pure C# — the visual layer binds against this
    /// model; headless PlayMode tests drive it without scene objects.
    /// </summary>
    public sealed class CookingPanel
    {
        /// <summary>Model of the latest render; null until first.</summary>
        public CookingPanelModel? Current
        {
            get;
            private set;
        }

        /// <summary>Render-call count (once-per-frame proof).</summary>
        public int RenderCount
        {
            get;
            private set;
        }

        /// <summary>Presents one model snapshot.</summary>
        public void Render(CookingPanelModel model)
        {
            Current = model;
            RenderCount++;
        }

        /// <summary>Clears presented state (panel closed / map change).</summary>
        public void Clear()
        {
            Current = null;
        }
    }
}
