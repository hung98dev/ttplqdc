using System;
using System.Collections.Generic;
using System.IO;
using UnityEngine;

namespace ThinhThan.Core.Assets.Editor.AssetProduction
{
    /// <summary>
    /// Fixture loading for the ART gates (presentation_asset_manifest.md
    /// sections 3.2/3.6 numeric completion). Fixture PNGs live under
    /// client/Assets/Tests/EditMode/&lt;suite&gt;/Fixtures/ and decode through
    /// UnityEngine.ImageConversion — no hand-rolled PNG parser.
    /// </summary>
    public static class ArtRuleFixtures
    {
        /// <summary>Directory holding the named cutout fixtures.</summary>
        public static string CutoutFixtureDir(string repoRoot)
        {
            return Path.Combine(repoRoot, "client", "Assets", "Tests",
                "EditMode", "CutoutQualityGate", "Fixtures");
        }

        /// <summary>Directory holding the named volume fixtures.</summary>
        public static string VolumeFixtureDir(string repoRoot)
        {
            return Path.Combine(repoRoot, "client", "Assets", "Tests",
                "EditMode", "VolumeDepthGate", "Fixtures");
        }

        /// <summary>Load a PNG file into a row-major RGBA8 buffer.</summary>
        public static LabPixels.Image LoadPng(string path)
        {
            byte[] bytes = File.ReadAllBytes(path);
            return DecodePng(bytes);
        }

        /// <summary>Decode PNG bytes into a row-major RGBA8 buffer.</summary>
        public static LabPixels.Image DecodePng(byte[] bytes)
        {
            var tex = new Texture2D(2, 2, TextureFormat.RGBA32, false);
            try
            {
                if (!ImageConversion.LoadImage(tex, bytes, false))
                {
                    throw new InvalidDataException("not a decodable PNG");
                }
                var px = tex.GetPixels32();
                var img = new LabPixels.Image(tex.width, tex.height);
                // GetPixels32 is bottom-left origin; flip to top-left.
                for (int y = 0; y < tex.height; y++)
                {
                    for (int x = 0; x < tex.width; x++)
                    {
                        var c = px[(tex.height - 1 - y) * tex.width + x];
                        img.Set(x, y, new LabPixels.Rgba
                        {
                            R = c.r,
                            G = c.g,
                            B = c.b,
                            A = c.a,
                        });
                    }
                }
                return img;
            }
            finally
            {
                UnityEngine.Object.DestroyImmediate(tex);
            }
        }

        /// <summary>
        /// Load the companion &lt;file&gt;.translucent.png mask; white
        /// (R=255) marks translucent. Null when absent.
        /// </summary>
        public static bool[]? LoadTranslucentMask(string texturePath)
        {
            string maskPath = texturePath.Substring(
                0, texturePath.Length - Path.GetExtension(texturePath).Length)
                + ".translucent.png";
            if (!File.Exists(maskPath))
            {
                return null;
            }
            var m = LoadPng(maskPath);
            var mask = new bool[m.Width * m.Height];
            for (int i = 0; i < mask.Length; i++)
            {
                mask[i] = m.Pixels[i].R > 0 && m.Pixels[i].A > 0;
            }
            return mask;
        }

        /// <summary>Serialize the buffer to PNG bytes (fixture authoring).</summary>
        public static byte[] EncodePng(LabPixels.Image img)
        {
            var tex = new Texture2D(img.Width, img.Height, TextureFormat.RGBA32, false);
            try
            {
                var px = new Color32[img.Width * img.Height];
                for (int y = 0; y < img.Height; y++)
                {
                    for (int x = 0; x < img.Width; x++)
                    {
                        var p = img.At(x, img.Height - 1 - y);
                        px[y * img.Width + x] = new Color32(p.R, p.G, p.B, p.A);
                    }
                }
                tex.SetPixels32(px);
                tex.Apply();
                return ImageConversion.EncodeToPNG(tex);
            }
            finally
            {
                UnityEngine.Object.DestroyImmediate(tex);
            }
        }

        /// <summary>Enumerate every fixture file in a directory.</summary>
        public static List<string> FixtureFiles(string dir)
        {
            var files = new List<string>();
            if (!Directory.Exists(dir))
            {
                return files;
            }
            files.AddRange(Directory.GetFiles(dir, "*.png", SearchOption.TopDirectoryOnly));
            files.Sort(StringComparer.Ordinal);
            return files;
        }
    }
}
