package gates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const vrFixturesXML = `<?xml version="1.0" encoding="utf-8"?>
<test-run>
  <test-suite name="VisualReview" fullname="VisualReview">
    <test-case name="RFloatFixture" fullname="VisualReview.RFloatFixture" result="Passed">
      <properties><property name="Category" value="GraphicsFixtures"/></properties>
    </test-case>
    <test-case name="OtherCase" result="Passed">
      <properties><property name="Category" value="EditMode"/></properties>
    </test-case>
  </test-suite>
</test-run>`

func writeVR(t *testing.T, root string, report, fixtures string, png bool) {
	t.Helper()
	dir := filepath.Join(root, visualReviewDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	w := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if report != "" {
		w("capture-report.json", report)
	}
	if fixtures != "" {
		w("visual-review-fixtures.xml", fixtures)
	}
	if png {
		w("actor-day-1080.png", "\x89PNG fake")
	}
}

func TestUnityVisualReview(t *testing.T) {
	t.Run("clean pass", func(t *testing.T) {
		root := t.TempDir()
		writeVR(t, root, `{"result":"PASS"}`, vrFixturesXML, true)
		errs, missing := (&Runner{Root: root}).unityVisualReview()
		if missing || len(errs) != 0 {
			t.Fatalf("clean fixture failed: missing=%v errs=%v", missing, errs)
		}
	})
	t.Run("missing dir is missing", func(t *testing.T) {
		_, missing := (&Runner{Root: t.TempDir()}).unityVisualReview()
		if !missing {
			t.Fatal("absent visual-review dir must report missing")
		}
	})
	t.Run("missing report is missing", func(t *testing.T) {
		root := t.TempDir()
		writeVR(t, root, "", vrFixturesXML, true)
		_, missing := (&Runner{Root: root}).unityVisualReview()
		if !missing {
			t.Fatal("absent capture-report.json must report missing")
		}
	})
	t.Run("missing fixtures xml is missing", func(t *testing.T) {
		root := t.TempDir()
		writeVR(t, root, `{"result":"PASS"}`, "", true)
		_, missing := (&Runner{Root: root}).unityVisualReview()
		if !missing {
			t.Fatal("absent visual-review-fixtures.xml must report missing")
		}
	})
	t.Run("fail verdict", func(t *testing.T) {
		root := t.TempDir()
		writeVR(t, root, `{"result":"FAIL"}`, vrFixturesXML, true)
		errs, _ := (&Runner{Root: root}).unityVisualReview()
		if len(errs) == 0 {
			t.Fatal("FAIL verdict not flagged")
		}
	})
	t.Run("verdict spellings", func(t *testing.T) {
		for _, doc := range []string{`{"status":"passed"}`, `{"verdict":"ok"}`, `{"passed":true}`, `{"pass":true}`} {
			root := t.TempDir()
			writeVR(t, root, doc, vrFixturesXML, true)
			if errs, _ := (&Runner{Root: root}).unityVisualReview(); len(errs) != 0 {
				t.Fatalf("verdict doc %s flagged: %v", doc, errs)
			}
		}
		root := t.TempDir()
		writeVR(t, root, `{"shots":3}`, vrFixturesXML, true)
		if errs, _ := (&Runner{Root: root}).unityVisualReview(); len(errs) == 0 {
			t.Fatal("report without verdict field not flagged")
		}
	})
	t.Run("empty GraphicsFixtures category", func(t *testing.T) {
		root := t.TempDir()
		xml := strings.Replace(vrFixturesXML, "GraphicsFixtures", "Other", -1)
		writeVR(t, root, `{"result":"PASS"}`, xml, true)
		errs, _ := (&Runner{Root: root}).unityVisualReview()
		if len(errs) == 0 {
			t.Fatal("empty GraphicsFixtures category not flagged")
		}
	})
	t.Run("failed fixture case", func(t *testing.T) {
		root := t.TempDir()
		xml := strings.Replace(vrFixturesXML, `result="Passed"`, `result="Failed"`, 1)
		writeVR(t, root, `{"result":"PASS"}`, xml, true)
		errs, _ := (&Runner{Root: root}).unityVisualReview()
		if len(errs) == 0 {
			t.Fatal("failed GraphicsFixtures case not flagged")
		}
	})
	t.Run("no PNG captures", func(t *testing.T) {
		root := t.TempDir()
		writeVR(t, root, `{"result":"PASS"}`, vrFixturesXML, false)
		errs, _ := (&Runner{Root: root}).unityVisualReview()
		if len(errs) == 0 {
			t.Fatal("absent PNG captures not flagged")
		}
	})
}
