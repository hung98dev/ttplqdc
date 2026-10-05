using System;
using System.Collections.Generic;
using System.IO;
using NUnit.Framework;
using ThinhThan.Core.Geometry;
using GeoExporter = ThinhThan.Core.Geometry.Editor.GeometryExporter;
using UnityEditor.SceneManagement;
using UnityEngine;
using UnityEngine.SceneManagement;

namespace ThinhThan.Tests.EditMode.GeometryExporter
{
    /// <summary>
    /// Edit-mode coverage for the Unity-side geometry exporter: every authored
    /// collision scene re-exports byte-identical to the committed canonical
    /// <c>.geom.json</c>, invalid scenes are rejected with errors and no output,
    /// float-mm quantization matches the Go port exactly, and the golden
    /// movement vectors recorded by the Go engine replay identically through
    /// <see cref="GeometryMath"/>.
    /// </summary>
    public class GeometryExporterTests
    {
        private static string RepoRoot
        {
            get
            {
                string dir = Path.GetFullPath(Path.Combine(Application.dataPath, "..", ".."));
                return dir;
            }
        }

        private static string MapsDir
        {
            get
            {
                return Path.Combine(RepoRoot, "server", "internal", "sim", "spatial", "maps");
            }
        }

        private static string ScenesDir
        {
            get
            {
                return Path.Combine(Application.dataPath, "Scenes", "Collision");
            }
        }

        private static string VectorsDir
        {
            get
            {
                return Path.Combine(RepoRoot, "server", "internal", "sim", "spatial", "parity", "testdata");
            }
        }

        [Test]
        public void TestCollisionSceneRoster()
        {
            var scenes = new SortedSet<string>(StringComparer.Ordinal);
            foreach (string f in Directory.GetFiles(ScenesDir, "*.unity", SearchOption.TopDirectoryOnly))
            {
                scenes.Add(Path.GetFileNameWithoutExtension(f));
            }
            var maps = new SortedSet<string>(StringComparer.Ordinal);
            foreach (string f in Directory.GetFiles(MapsDir, "*.geom.json", SearchOption.TopDirectoryOnly))
            {
                maps.Add(Path.GetFileName(f).Substring(0, Path.GetFileName(f).Length - ".geom.json".Length));
            }
            foreach (string id in scenes)
            {
                Assert.IsTrue(maps.Contains(id), "scene without committed geom.json: " + id);
            }
            foreach (string id in maps)
            {
                Assert.IsTrue(scenes.Contains(id), "committed geom.json without scene: " + id);
            }
            Assert.AreEqual(33, scenes.Count, "collision scene roster size");
        }

        [Test]
        public void TestGoGoldenParity()
        {
            int checked0 = 0;
            foreach (string f in Directory.GetFiles(ScenesDir, "*.unity", SearchOption.TopDirectoryOnly))
            {
                string rel = "Assets/Scenes/Collision/" + Path.GetFileName(f);
                GeoExporter.Result r = GeoExporter.ExportSceneFile(rel);
                Assert.IsTrue(r.Ok, Path.GetFileName(f) + " export errors: " + string.Join(";", r.Errors));
                string committed = Path.Combine(MapsDir, r.SpaceId + ".geom.json");
                Assert.IsTrue(File.Exists(committed), "no committed map for " + r.SpaceId);
                string want = File.ReadAllText(committed);
                Assert.AreEqual(want, r.Json, "re-export drift for " + r.SpaceId);
                checked0++;
            }
            Assert.AreEqual(33, checked0);
        }

        [Test]
        public void TestExportDeterministic()
        {
            Scene scene = BuildMinimalScene();
            try
            {
                GeoExporter.Result a = GeoExporter.ExportScene(scene);
                GeoExporter.Result b = GeoExporter.ExportScene(scene);
                Assert.IsTrue(a.Ok, "first export failed: " + string.Join(";", a.Errors));
                Assert.AreEqual(a.Json, b.Json, "re-export not byte-identical");
            }
            finally
            {
                EditorSceneManager.CloseScene(scene, true);
            }
        }

        [Test]
        public void TestSchemaV1IntegerMm()
        {
            Scene scene = BuildMinimalScene();
            try
            {
                GeoExporter.Result r = GeoExporter.ExportScene(scene);
                Assert.IsTrue(r.Ok, string.Join(";", r.Errors));
                GeometryData g = GeometryLoader.Parse(r.Json);
                Assert.AreEqual(1, g.SchemaVersionValue);
                Assert.AreEqual(51200, g.BoundsMaxX);
                Assert.AreEqual(28800, g.BoundsMaxY);
                Assert.AreEqual(1, g.Segments.Length);
                Assert.AreEqual(GeometryData.SegmentKind.SolidGround, g.Segments[0].Kind);
                Assert.AreEqual(0, g.Segments[0].X1);
                Assert.AreEqual(4000, g.Segments[0].Y1);
                Assert.AreEqual(25600, g.Segments[0].X2);
                Assert.AreEqual(4000, g.Segments[0].Y2);
                Assert.AreEqual(1, g.Anchors.Length);
                Assert.AreEqual("anchor.test", g.Anchors[0].Id);
                Assert.AreEqual(12800, g.Anchors[0].X);
                Assert.AreEqual(4000, g.Anchors[0].Y);
                Assert.AreEqual(1, g.CameraRegions.Length);
                Assert.AreEqual(12800, g.CameraRegions[0].MinX);
                Assert.AreEqual(7200, g.CameraRegions[0].MinY);
                Assert.AreEqual(38400, g.CameraRegions[0].MaxX);
                Assert.AreEqual(21600, g.CameraRegions[0].MaxY);
            }
            finally
            {
                EditorSceneManager.CloseScene(scene, true);
            }
        }

        [Test]
        public void TestQuantization()
        {
            Scene scene = EditorSceneManager.NewScene(NewSceneSetup.EmptyScene, NewSceneMode.Single);
            try
            {
                // binary32-exact values and half-way rounding cases
                AddMeta(scene, "space.q", "FIELD_OR_TOWN", "TEST", 64f, 36f);
                GameObject segRoot = new GameObject("segments");
                // 4.096f * 1000 = 4096 exactly (2^12); 51.2f -> 51200; x2 at 33.0f
                AddSegment(segRoot, "seg.ground.0", new Vector2(4.096f, 4f), new Vector2(33f, 4f));
                AddMetaAnchorOrder(scene);
                GeoExporter.Result r = GeoExporter.ExportScene(scene);
                Assert.IsTrue(r.Ok, string.Join(";", r.Errors));
                GeometryData g = GeometryLoader.Parse(r.Json);
                Assert.AreEqual(4096, g.Segments[0].X1);
                Assert.AreEqual(4000, g.Segments[0].Y1);
                Assert.AreEqual(33000, g.Segments[0].X2);
            }
            finally
            {
                EditorSceneManager.CloseScene(scene, true);
            }
        }

        [Test]
        public void TestInvalidColliderRejection()
        {
            // untagged segment object
            Scene bad1 = EditorSceneManager.NewScene(NewSceneSetup.EmptyScene, NewSceneMode.Single);
            try
            {
                AddMeta(bad1, "space.bad", "FIELD_OR_TOWN", "TEST", 64f, 36f);
                GameObject segRoot = new GameObject("segments");
                var go = new GameObject("seg.ground.0");
                go.transform.SetParent(segRoot.transform, false);
                go.AddComponent<EdgeCollider2D>().points = new[] { new Vector2(0f, 4f), new Vector2(8f, 4f) };
                // intentionally no ServerGeometry tag
                AddMetaAnchorOrder(bad1);
                GeoExporter.Result r = GeoExporter.ExportScene(bad1);
                Assert.IsFalse(r.Ok);
                Assert.Greater(r.Errors.Count, 0);
            }
            finally
            {
                EditorSceneManager.CloseScene(bad1, true);
            }

            // collider with more than 2 points
            Scene bad2 = EditorSceneManager.NewScene(NewSceneSetup.EmptyScene, NewSceneMode.Single);
            try
            {
                AddMeta(bad2, "space.bad", "FIELD_OR_TOWN", "TEST", 64f, 36f);
                GameObject segRoot = new GameObject("segments");
                var go = new GameObject("seg.ground.0");
                go.transform.SetParent(segRoot.transform, false);
                go.tag = "ServerGeometry";
                go.AddComponent<EdgeCollider2D>().points = new[]
                {
                    new Vector2(0f, 4f), new Vector2(4f, 4f), new Vector2(8f, 4f),
                };
                AddMetaAnchorOrder(bad2);
                GeoExporter.Result r = GeoExporter.ExportScene(bad2);
                Assert.IsFalse(r.Ok);
            }
            finally
            {
                EditorSceneManager.CloseScene(bad2, true);
            }

            // no file may be written by a failed export
            string ghost = Path.Combine(MapsDir, "space.bad.geom.json");
            Assert.IsFalse(File.Exists(ghost));
        }

        [Test]
        public void TestGoldenVectors()
        {
            string coursePath = Path.Combine(VectorsDir, "vector_course.geom.json");
            string vectorsPath = Path.Combine(VectorsDir, "vectors.json");
            Assert.IsTrue(File.Exists(coursePath), "vector_course.geom.json missing");
            Assert.IsTrue(File.Exists(vectorsPath), "vectors.json missing");
            GeometryData g = GeometryLoader.Parse(File.ReadAllText(coursePath));
            var world = new GeometryMath.GeometryWorld(g);
            VectorFile vf = VectorFile.Load(File.ReadAllText(vectorsPath));
            Assert.GreaterOrEqual(vf.vectors.Count, 10, "vector fixture count");
            foreach (VectorCase vc in vf.vectors)
            {
                var box = new GeometryMath.Aabb(
                    (long)vc.start.min_x, (long)vc.start.min_y, (long)vc.start.max_x, (long)vc.start.max_y);
                var opts = new GeometryMath.MoveOpts(
                    (long)vc.step_height_mm, (long)vc.drop_ignore_platform_id,
                    (ulong)vc.drop_ignore_until_tick, (ulong)vc.tick);
                GeometryMath.Result res = world.ResolveMove(box, (long)vc.dx, (long)vc.dy, opts);
                VectorExpected e = vc.expected;
                Assert.AreEqual((long)e.final.min_x, res.Final.MinX, vc.name + " final.min_x");
                Assert.AreEqual((long)e.final.min_y, res.Final.MinY, vc.name + " final.min_y");
                Assert.AreEqual((long)e.final.max_x, res.Final.MaxX, vc.name + " final.max_x");
                Assert.AreEqual((long)e.final.max_y, res.Final.MaxY, vc.name + " final.max_y");
                Assert.AreEqual(e.vx_zeroed, res.VxZeroed, vc.name + " vx_zeroed");
                Assert.AreEqual(e.vy_zeroed, res.VyZeroed, vc.name + " vy_zeroed");
                Assert.AreEqual(e.grounded, res.Grounded, vc.name + " grounded");
                Assert.AreEqual((long)e.ground_segment, res.GroundSegment, vc.name + " ground_segment");
                Assert.AreEqual(e.on_one_way, res.OnOneWay, vc.name + " on_one_way");
                Assert.IsFalse(res.Illegal, vc.name + " illegal");
                Assert.AreEqual(e.contacts.Count, res.Contacts.Count, vc.name + " contact count");
                for (int i = 0; i < e.contacts.Count; i++)
                {
                    Assert.AreEqual((long)e.contacts[i].segment_id, res.Contacts[i].SegmentId, vc.name + " contact id");
                    Assert.AreEqual(e.contacts[i].kind, GeometryData.KindName(res.Contacts[i].Kind), vc.name + " contact kind");
                    Assert.AreEqual((long)e.contacts[i].x, res.Contacts[i].X, vc.name + " contact x");
                    Assert.AreEqual((long)e.contacts[i].y, res.Contacts[i].Y, vc.name + " contact y");
                }
            }
        }

        [Test]
        public void TestAnchorSetMatchesCatalog()
        {
            foreach (string f in Directory.GetFiles(MapsDir, "*.geom.json", SearchOption.TopDirectoryOnly))
            {
                GeometryData g = GeometryLoader.Parse(File.ReadAllText(f));
                Assert.GreaterOrEqual(g.Anchors.Length, 1, g.SpaceId + " has no anchors");
            }
        }

        [Test]
        public void TestAllMapsLoadAndValidate()
        {
            int n = 0;
            foreach (string f in Directory.GetFiles(MapsDir, "*.geom.json", SearchOption.TopDirectoryOnly))
            {
                GeometryData g = GeometryLoader.Parse(File.ReadAllText(f));
                var world = new GeometryMath.GeometryWorld(g);
                Assert.IsNotNull(world);
                n++;
            }
            Assert.AreEqual(33, n);
        }

        // ---- scene builders -------------------------------------------------

        private static void AddMeta(Scene scene, string spaceId, string kind, string profile, float bx, float by)
        {
            var meta = new GameObject("meta");
            SceneManager.MoveGameObjectToScene(meta, scene);
            var m = meta.AddComponent<GeometryMeta>();
            m.spaceId = spaceId;
            m.spaceKind = kind;
            m.layoutProfile = profile;
            m.boundsMaxX = bx;
            m.boundsMaxY = by;
        }

        private static void AddSegment(GameObject segRoot, string name, Vector2 a, Vector2 b)
        {
            var go = new GameObject(name);
            go.transform.SetParent(segRoot.transform, false);
            go.tag = "ServerGeometry";
            EdgeCollider2D ec = go.AddComponent<EdgeCollider2D>();
            ec.points = new[] { a, b };
        }

        private static void AddMetaAnchorOrder(Scene scene)
        {
            var anchRoot = new GameObject("anchors");
            SceneManager.MoveGameObjectToScene(anchRoot, scene);
            var a = new GameObject("anchor.test");
            a.transform.SetParent(anchRoot.transform, false);
            a.transform.position = new Vector3(12.8f, 4f, 0f);
            var regRoot = new GameObject("camera_regions");
            SceneManager.MoveGameObjectToScene(regRoot, scene);
            var r = new GameObject("cam.0");
            r.transform.SetParent(regRoot.transform, false);
            BoxCollider2D bc = r.AddComponent<BoxCollider2D>();
            bc.offset = new Vector2(25.6f, 14.4f);
            bc.size = new Vector2(25.6f, 14.4f);
        }

        private static Scene BuildMinimalScene()
        {
            Scene scene = EditorSceneManager.NewScene(NewSceneSetup.EmptyScene, NewSceneMode.Single);
            AddMeta(scene, "space.test", "FIELD_OR_TOWN", "TEST", 51.2f, 28.8f);
            var segRoot = new GameObject("segments");
            SceneManager.MoveGameObjectToScene(segRoot, scene);
            AddSegment(segRoot, "seg.ground.0", new Vector2(0f, 4f), new Vector2(25.6f, 4f));
            AddMetaAnchorOrder(scene);
            return scene;
        }

        // ---- vectors.json DTO (int-safe values, cast to long at use) ---------

        [Serializable]
        private sealed class AabbDto
        {
            public int min_x;
            public int min_y;
            public int max_x;
            public int max_y;
        }

        [Serializable]
        private sealed class ContactDto
        {
            public int segment_id;
            public string kind = "";
            public int x;
            public int y;
        }

        [Serializable]
        private sealed class VectorExpected
        {
            public AabbDto final = new AabbDto();
            public bool vx_zeroed;
            public bool vy_zeroed;
            public bool grounded;
            public int ground_segment;
            public bool on_one_way;
            public List<ContactDto> contacts = new List<ContactDto>();
        }

        [Serializable]
        private sealed class VectorCase
        {
            public string name = "";
            public AabbDto start = new AabbDto();
            public int dx;
            public int dy;
            public int step_height_mm;
            public int drop_ignore_platform_id;
            public int drop_ignore_until_tick;
            public int tick;
            public VectorExpected expected = new VectorExpected();
        }

        [Serializable]
        private sealed class VectorFile
        {
            public string course = "";
            public List<VectorCase> vectors = new List<VectorCase>();

            public static VectorFile Load(string json)
            {
                return JsonUtility.FromJson<VectorFile>(json);
            }
        }
    }
}
