package architecture

import (
	"os"
	"testing"
)

func fenceFixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("_testdata")); err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return root
}

func TestClientApiFence(t *testing.T) {
	got := CheckClientApiFence(fenceFixtureRoot(t))
	for _, w := range []string{
		"GameObject.Find", "SendMessage", "Invoke", "StartCoroutine",
		"IEnumerator", "async void", "Task", "Resources.Load",
		"WaitForCompletion", "Camera.main", "System.Linq", "Debug.Log",
		"material", "GC.Collect", "Random", "UnityEvent", "static field",
	} {
		if len(findDetails(got, "Hud.cs", w)) == 0 {
			t.Errorf("expected Hud.cs %s violation, got %v", w, got)
		}
	}
	// Scoped allowances: Net Task, Camera service, App statics, FrameLoop.
	for _, f := range []string{"NetClient.cs", "CameraService.cs", "AppState.cs", "FrameLoop.cs"} {
		if len(findDetails(got, f)) != 0 {
			t.Errorf("allowed %s flagged: %v", f, findDetails(got, f))
		}
	}
}

func TestAllowlistEntriesNeedReason(t *testing.T) {
	text := "# comment\n" +
		"a/b.cs:Update  FrameLoop owner\n" +
		"c/d.cs:LateUpdate\n" +
		"e/f.cs:OnGUI single-space\n" +
		":Update  empty path\n"
	entries, errs := ParseAllowlist(text)
	if len(entries) != 1 || entries[0].Symbol != "Update" || entries[0].Reason == "" {
		t.Fatalf("want 1 valid entry, got %v", entries)
	}
	if len(errs) != 3 {
		t.Fatalf("want 3 errors, got %v", errs)
	}
}

func TestFenceExcludesEditorTestsGenerated(t *testing.T) {
	got := CheckClientApiFence(fenceFixtureRoot(t))
	for _, f := range []string{"Gen.cs", "Tool.cs", "WidgetTests.cs"} {
		if len(findDetails(got, f)) != 0 {
			t.Errorf("excluded %s flagged: %v", f, findDetails(got, f))
		}
	}
	if got := CheckFenceScope(fenceFixtureRoot(t)); len(got) != 0 {
		t.Errorf("fence scope leak: %v", got)
	}
}

func TestFrameLoopOnlyUnityCallbacks(t *testing.T) {
	got := CheckFrameLoopCallbacks(fenceFixtureRoot(t))
	for _, cb := range []string{"Update", "FixedUpdate", "LateUpdate", "OnGUI"} {
		if len(findDetails(got, "Ticker.cs", cb)) == 0 {
			t.Errorf("expected Ticker.cs %s violation, got %v", cb, got)
		}
	}
	if len(findDetails(got, "FrameLoop.cs")) != 0 {
		t.Errorf("allowlisted FrameLoop flagged: %v", findDetails(got, "FrameLoop.cs"))
	}
}

func TestCanonicalImplementationsUnique(t *testing.T) {
	got := CheckCanonical(fenceFixtureRoot(t))
	// SecondPool.cs declares WidgetPool + SecondLogger outside Core/Runtime.
	if len(findDetails(got, "SecondPool.cs", "pooling")) == 0 {
		t.Errorf("expected second pool violation, got %v", got)
	}
	if len(findDetails(got, "SecondPool.cs", "logging")) == 0 {
		t.Errorf("expected second logger violation, got %v", got)
	}
	// `new System.Random()` in Hud.cs (UI) — randomness owner is Core/Runtime.
	if len(findDetails(got, "Hud.cs", "randomness")) == 0 {
		t.Errorf("expected randomness violation, got %v", got)
	}
	// Owner-path files are clean: FrameLoop inside Core/Runtime,
	// CameraService inside Systems/Camera.
	if len(findDetails(got, "FrameLoop.cs", "CameraService.cs")) != 0 {
		t.Errorf("owner-path implementations flagged: %v", got)
	}
}
