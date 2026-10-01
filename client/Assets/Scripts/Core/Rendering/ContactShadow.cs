using System;
using UnityEngine;

namespace ThinhThan.Core.Rendering
{
    /// <summary>
    /// Runtime soft contact shadow (client.md § Rendering;
    /// presentation_asset_manifest.md §3.5): a transform-anchored sprite child
    /// under every actor — never painted into the sprite. A static per-class
    /// profile table supplies scale/offset/softness. The component holds no
    /// frame callbacks; the shadow follows through the transform hierarchy.
    /// </summary>
    public sealed class ContactShadow : MonoBehaviour
    {
        /// <summary>Actor classes with a contact-shadow profile.</summary>
        public enum ActorProfile
        {
            Character,
            MonsterSmall,
            MonsterMedium,
            MonsterElite,
            BossLarge,
            WorldBoss,
            SpiritBeast,
            NpcHumanoid,
        }

        /// <summary>
        /// Shadow geometry for one actor class: ellipse size in world meters,
        /// offset from the feet anchor, and edge softness in [0,1] as the
        /// fraction of the radius over which alpha falls to zero.
        /// </summary>
        public readonly struct Profile
        {
            public Profile(Vector2 scale, Vector2 offset, float softness)
            {
                Scale = scale;
                Offset = offset;
                Softness = softness;
            }

            public readonly Vector2 Scale;
            public readonly Vector2 Offset;
            public readonly float Softness;
        }

        /// <summary>Name of the shadow child GameObject under the actor.</summary>
        public const string ChildName = "ContactShadow";

        /// <summary>
        /// Static profile table per actor class. Ellipse widths sit just under
        /// the class silhouette/collider width at 50 px/m
        /// (physics_geometry_contract.md §3, presentation_asset_manifest.md §3).
        /// </summary>
        public static Profile ProfileFor(ActorProfile actor)
        {
            switch (actor)
            {
                case ActorProfile.Character:
                    return new Profile(new Vector2(0.95f, 0.42f), Vector2.zero, 0.5f);
                case ActorProfile.MonsterSmall:
                    return new Profile(new Vector2(0.70f, 0.32f), Vector2.zero, 0.5f);
                case ActorProfile.MonsterMedium:
                    return new Profile(new Vector2(1.15f, 0.52f), Vector2.zero, 0.5f);
                case ActorProfile.MonsterElite:
                    return new Profile(new Vector2(1.80f, 0.80f), Vector2.zero, 0.5f);
                case ActorProfile.BossLarge:
                    return new Profile(new Vector2(2.70f, 1.20f), Vector2.zero, 0.5f);
                case ActorProfile.WorldBoss:
                    return new Profile(new Vector2(3.40f, 1.50f), Vector2.zero, 0.5f);
                case ActorProfile.SpiritBeast:
                    return new Profile(new Vector2(0.72f, 0.32f), Vector2.zero, 0.5f);
                case ActorProfile.NpcHumanoid:
                    return new Profile(new Vector2(0.95f, 0.42f), Vector2.zero, 0.5f);
                default:
                    throw new ArgumentOutOfRangeException(nameof(actor), actor, null);
            }
        }

        /// <summary>
        /// Creates the transform-anchored shadow child under this actor: a 1x1m
        /// soft ellipse scaled into the class profile, drawn one order band
        /// under the actor sprite. The caller supplies the shared shadow sprite
        /// and the shared Sprite-Lit material.
        /// </summary>
        public SpriteRenderer Configure(Profile profile, Sprite shadowSprite, Material shadowMaterial)
        {
            var shadow = new GameObject(ChildName);
            Transform anchor = shadow.transform;
            anchor.SetParent(transform, false);
            anchor.localPosition = new Vector3(profile.Offset.x, profile.Offset.y, 0f);
            anchor.localScale = new Vector3(profile.Scale.x, profile.Scale.y, 1f);
            var renderer = shadow.AddComponent<SpriteRenderer>();
            renderer.sprite = shadowSprite;
            renderer.sharedMaterial = shadowMaterial;
            renderer.sortingOrder = RenderingContract.ContactShadowOrderInLayer;
            return renderer;
        }

        /// <summary>
        /// Bakes the runtime soft ellipse: a 32x32 sprite (1x1m) whose alpha is
        /// 1 inside the core and falls linearly to 0 across the outer
        /// <paramref name="softness"/> fraction of the radius. The caller owns
        /// and caches the result per profile.
        /// </summary>
        public static Sprite CreateShadowSprite(float softness)
        {
            const int size = 32;
            float band = Mathf.Clamp(softness, 0.05f, 1f);
            var texture = new Texture2D(size, size, TextureFormat.RGBA32, false);
            texture.name = "ContactShadowSoftEllipse";
            texture.filterMode = FilterMode.Bilinear;
            texture.wrapMode = TextureWrapMode.Clamp;
            var pixels = new Color32[size * size];
            for (int y = 0; y < size; y++)
            {
                for (int x = 0; x < size; x++)
                {
                    float nx = (x + 0.5f) / size * 2f - 1f;
                    float ny = (y + 0.5f) / size * 2f - 1f;
                    float radius = Mathf.Sqrt(nx * nx + ny * ny);
                    float alpha = Mathf.Clamp01((1f - radius) / band);
                    pixels[y * size + x] = new Color32(0, 0, 0, (byte)Mathf.RoundToInt(alpha * 255f));
                }
            }
            texture.SetPixels32(pixels);
            texture.Apply();
            return Sprite.Create(texture, new Rect(0f, 0f, size, size), new Vector2(0.5f, 0.5f), size);
        }
    }
}
