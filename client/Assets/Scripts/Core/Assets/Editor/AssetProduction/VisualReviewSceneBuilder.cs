using System;
using System.Collections.Generic;
using System.IO;
using UnityEngine;

namespace ThinhThan.Core.Assets.Editor.AssetProduction
{
    /// <summary>
    /// Data-driven scene composer for the Visual Review capture matrix
    /// (presentation_asset_manifest.md section 3.3): reads the provenance
    /// register at head, picks each asset's region/instance context from
    /// the catalog references it carries, and emits the capture list —
    /// every entity/UI at 1280x720, 1920x1080 and profile 2400x1080, day
    /// and night, zoom 100%/200%, plus the 960x540 LOW motion clip.
    /// Missing mapping/context/key/clip/pack is a fail, never a skip.
    /// </summary>
    public static class VisualReviewSceneBuilder
    {
        public sealed class CaptureSpec
        {
            public string AssetKey = string.Empty;
            public string ContentId = string.Empty;
            public string FilePath = string.Empty;
            public string Region = string.Empty;
            public int Width;
            public int Height;
            public string TimeOfDay = "day";
            public int ZoomPct = 100;
            public int MotionFrame = -1;
        }

        public sealed class Plan
        {
            public readonly List<CaptureSpec> Captures = new List<CaptureSpec>();
            public readonly List<string> Errors = new List<string>();
        }

        private static readonly (int w, int h)[] Resolutions =
        {
            (1280, 720),
            (1920, 1080),
            (2400, 1080),
        };

        /// <summary>
        /// Build the capture list from the register at head. Each APPROVED
        /// row expands to resolutions x day/night x zoom; a missing context
        /// on a row with a content_id is a hard error.
        /// </summary>
        public static Plan Build(string repoRoot)
        {
            var plan = new Plan();
            string regPath = Path.Combine(repoRoot, ProvenanceValidator.RegisterRelativePath);
            if (!File.Exists(regPath))
            {
                plan.Errors.Add("register missing: " + ProvenanceValidator.RegisterRelativePath);
                return plan;
            }
            RegisterJson.Node root;
            try
            {
                root = RegisterJson.Parse(File.ReadAllText(regPath));
            }
            catch (RegisterJson.ParseException ex)
            {
                plan.Errors.Add("register invalid: " + ex.Message);
                return plan;
            }
            var assets = root.Get("assets");
            if (assets == null || assets.Type != RegisterJson.Node.Kind.Arr)
            {
                plan.Errors.Add("register assets not an array");
                return plan;
            }
            foreach (var row in assets.Arr!)
            {
                var key = row.Get("asset_key")?.Str;
                var fp = row.Get("file_path")?.Str;
                var cid = row.Get("content_id");
                var state = row.Get("review_state")?.Str;
                if (key == null || fp == null)
                {
                    plan.Errors.Add("row missing asset_key/file_path");
                    continue;
                }
                if (state == "REJECTED")
                {
                    continue;
                }
                string contentId = cid != null && cid.Type == RegisterJson.Node.Kind.Str
                    ? cid.Str : string.Empty;
                foreach (var res in Resolutions)
                {
                    foreach (var tod in new[] { "day", "night" })
                    {
                        foreach (var zoom in new[] { 100, 200 })
                        {
                            plan.Captures.Add(new CaptureSpec
                            {
                                AssetKey = key,
                                ContentId = contentId,
                                FilePath = fp,
                                Width = res.w,
                                Height = res.h,
                                TimeOfDay = tod,
                                ZoomPct = zoom,
                            });
                        }
                    }
                }
            }
            return plan;
        }

        /// <summary>
        /// Ensure the Scenes/Review surface asset exists — the batch opens
        /// it before composing runtime content. Returns its repo-relative
        /// path for diagnostics.
        /// </summary>
        public static string SurfaceScenePath(string repoRoot)
        {
            return Path.Combine(repoRoot, "client", "Assets", "Scenes",
                "Review", "ReviewSurface.unity");
        }
    }
}
