package architecture

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// This file implements architecture_conformance.md §4 rules 13 (client API
// fence, CODE-005/PERF-020) and 14 (canonical implementations, CODE-006):
// a token-based scan of first-party runtime C# assemblies
// (ThinhThan.Core/Net/Systems/UI/App) against the §2.5 forbidden-API list,
// minus exact `path:symbol  reason` entries in client_api_allowlist.txt.

const allowlistFile = "server/internal/conformance/architecture/client_api_allowlist.txt"

// fenceRoots are the first-party runtime assembly roots under fence scope.
var fenceRoots = []string{
	"client/Assets/Scripts/Core/",
	"client/Assets/Scripts/Net/",
	"client/Assets/Scripts/Systems/",
	"client/Assets/Scripts/UI/",
	"client/Assets/Scripts/App/",
}

// fenceRule is one §2.5 row: a token pattern plus the scope it applies to.
type fenceRule struct {
	symbol  string         // allowlist symbol (the token named in entries)
	pattern *regexp.Regexp // matched against comment/string-stripped lines
	// scope, when non-nil, restricts the rule to files under these roots;
	// outsideScope, when non-nil, restricts to files NOT under them.
	scope        []string
	outsideScope []string
}

var fenceRules = []fenceRule{
	// Unity message callbacks — FrameLoop-only (PERF-020); exempt via allowlist.
	{symbol: "Update", pattern: regexp.MustCompile(`\bvoid\s+Update\s*\(`)},
	{symbol: "FixedUpdate", pattern: regexp.MustCompile(`\bvoid\s+FixedUpdate\s*\(`)},
	{symbol: "LateUpdate", pattern: regexp.MustCompile(`\bvoid\s+LateUpdate\s*\(`)},
	{symbol: "OnGUI", pattern: regexp.MustCompile(`\bvoid\s+OnGUI\s*\(`)},
	// Find family.
	{symbol: "GameObject.Find", pattern: regexp.MustCompile(`\bGameObject\.Find\s*\(|\bFindObjects?OfType|\bFindFirstObjectByType|\bFindAnyObjectByType`)},
	// Message / invoke family.
	{symbol: "SendMessage", pattern: regexp.MustCompile(`\bSendMessage\s*\(`)},
	{symbol: "BroadcastMessage", pattern: regexp.MustCompile(`\bBroadcastMessage\s*\(`)},
	{symbol: "Invoke", pattern: regexp.MustCompile(`\bInvokeRepeating\s*\(|\bInvoke\s*\(`)},
	// Coroutines.
	{symbol: "StartCoroutine", pattern: regexp.MustCompile(`\bStartCoroutine\s*\(`)},
	{symbol: "IEnumerator", pattern: regexp.MustCompile(`\bIEnumerator\b`)},
	{symbol: "async void", pattern: regexp.MustCompile(`\basync\s+void\b`)},
	// Threading — allowed only inside ThinhThan.Net.
	{symbol: "Task", pattern: regexp.MustCompile(`\bTask\.Run\b|\busing\s+System\.Threading\.Tasks\b|\bnew\s+Task\s*\(|\bTask<`),
		outsideScope: []string{"client/Assets/Scripts/Net/"}},
	{symbol: "Thread", pattern: regexp.MustCompile(`\bnew\s+Thread\b|\bThreadPool\.|\bSystem\.Threading\.Thread`),
		outsideScope: []string{"client/Assets/Scripts/Net/"}},
	// Resources.Load / WaitForCompletion (loading-screen uses are allowlisted).
	{symbol: "Resources.Load", pattern: regexp.MustCompile(`\bResources\.Load`)},
	{symbol: "WaitForCompletion", pattern: regexp.MustCompile(`\.WaitForCompletion\s*\(`)},
	// Camera.main outside the camera service.
	{symbol: "Camera.main", pattern: regexp.MustCompile(`\bCamera\.main\b`),
		outsideScope: []string{"client/Assets/Scripts/Systems/Camera/"}},
	{symbol: "System.Linq", pattern: regexp.MustCompile(`\busing\s+System\.Linq\b|\bSystem\.Linq\.`)},
	{symbol: "Debug.Log", pattern: regexp.MustCompile(`\bDebug\.Log`)},
	{symbol: "material", pattern: regexp.MustCompile(`\.material\b|\bnew\s+Material\s*\(`)},
	{symbol: "GC.Collect", pattern: regexp.MustCompile(`\bGC\.Collect\s*\(`)},
	{symbol: "UnityEngine.Random", pattern: regexp.MustCompile(`\bUnityEngine\.Random\b|\bSystem\.Random\b|\bnew\s+Random\s*\(|\bRandom\.`)},
	{symbol: "UnityEvent", pattern: regexp.MustCompile(`\bUnityEvent\b`)},
	// static mutable fields outside ThinhThan.App.
	// static mutable fields outside ThinhThan.App; `static readonly`/`const`
	// naturally fail this pattern (the qualifier occupies the TYPE slot).
	{symbol: "static field", pattern: regexp.MustCompile(`\bstatic\s+[\w<>\[\],?]+\s+\w+\s*(=|;)`),
		outsideScope: []string{"client/Assets/Scripts/App/"}},
}

// inScope reports whether rel is inside the rule's scope for the fence.
func (r fenceRule) inScope(rel string) bool {
	if len(r.scope) > 0 {
		ok := false
		for _, p := range r.scope {
			if strings.HasPrefix(rel, p) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	for _, p := range r.outsideScope {
		if strings.HasPrefix(rel, p) {
			return false
		}
	}
	return true
}

// fenceExcluded reports whether a .cs path is outside fence scope: Editor/
// folders, tests, generated Protocol/ (§2.5 exclusions).
func fenceExcluded(rel string) bool {
	if !strings.HasPrefix(rel, "client/Assets/") {
		return true
	}
	if strings.Contains(rel, "/Editor/") {
		return true
	}
	if strings.HasPrefix(rel, "client/Assets/Tests/") {
		return true
	}
	if strings.HasPrefix(rel, protocolDir) {
		return true
	}
	for _, root := range fenceRoots {
		if strings.HasPrefix(rel, root) {
			return false
		}
	}
	return true
}

// AllowlistEntry is one parsed client_api_allowlist.txt line.
type AllowlistEntry struct {
	Path   string
	Symbol string
	Reason string
}

// ParseAllowlist parses `path:symbol  reason` lines (two-space separator).
// Lines that are blank or start with '#' are skipped. A line without the
// two-space reason separator or with an empty reason is an error.
func ParseAllowlist(text string) ([]AllowlistEntry, []string) {
	var entries []AllowlistEntry
	var errs []string
	for i, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		loc, reason, ok := strings.Cut(t, "  ") // two-space separator
		if !ok || strings.TrimSpace(reason) == "" {
			errs = append(errs, fmt.Sprintf("allowlist line %d lacks `  reason`: %s", i+1, t))
			continue
		}
		p, sym, ok := strings.Cut(strings.TrimSpace(loc), ":")
		if !ok || p == "" || sym == "" {
			errs = append(errs, fmt.Sprintf("allowlist line %d lacks `path:symbol`: %s", i+1, t))
			continue
		}
		entries = append(entries, AllowlistEntry{Path: p, Symbol: sym, Reason: strings.TrimSpace(reason)})
	}
	return entries, errs
}

func (e AllowlistEntry) exempts(file, symbol string) bool {
	return e.Path == file && (e.Symbol == symbol || e.Symbol == "*")
}

// maskCommentsStrings strips // comments, /* */ blocks and string literals so
// token patterns match code only.
var (
	lineCommentRe = regexp.MustCompile(`//.*$`)
	strLitRe      = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	charLitRe     = regexp.MustCompile(`'(?:[^'\\]|\\.)*'`)
	interpStrRe   = regexp.MustCompile(`\$"(?:[^"\\]|\\.)*"`)
	verbalStrRe   = regexp.MustCompile(`@"(?:[^"]|"")*"`)
)

func maskLine(line string) string {
	line = interpStrRe.ReplaceAllString(line, `""`)
	line = verbalStrRe.ReplaceAllString(line, `""`)
	line = strLitRe.ReplaceAllString(line, `""`)
	line = charLitRe.ReplaceAllString(line, `''`)
	if i := strings.Index(line, "//"); i >= 0 {
		line = line[:i]
	}
	return line
}

func maskSource(src string) []string {
	src = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(src, " ")
	lines := strings.Split(src, "\n")
	for i := range lines {
		lines[i] = maskLine(lines[i])
	}
	return lines
}

// CheckAllowlistReasons validates the allowlist grammar itself (rule 13:
// every entry needs a reason).
func CheckAllowlistReasons(root string) []string {
	b, err := os.ReadFile(path.Join(root, allowlistFile))
	if err != nil {
		return []string{"client_api_allowlist.txt: " + err.Error()}
	}
	_, errs := ParseAllowlist(string(b))
	return errs
}

// CheckClientApiFence runs the §2.5 forbidden-token scan over the runtime
// assemblies, minus valid allowlist entries (Q4.api_fence, CODE-005).
func CheckClientApiFence(root string) []string {
	var out []string
	out = append(out, CheckAllowlistReasons(root)...)
	if len(out) > 0 {
		return out // invalid allowlist fails the whole gate
	}
	b, err := os.ReadFile(path.Join(root, allowlistFile))
	if err != nil {
		return []string{"client_api_allowlist.txt: " + err.Error()}
	}
	entries, _ := ParseAllowlist(string(b))
	files, err := collectCSharp(root)
	if err != nil {
		return []string{"collect C#: " + err.Error()}
	}
	for _, f := range files {
		for i, line := range f.lines {
			for _, rule := range fenceRules {
				if !rule.inScope(f.rel) || !rule.pattern.MatchString(line) {
					continue
				}
				if exempted(entries, f.rel, rule.symbol) {
					continue
				}
				out = append(out, fmt.Sprintf("%s:%d forbidden API %s (%s)", f.rel, i+1, rule.symbol, strings.TrimSpace(line)))
			}
		}
	}
	return out
}

func exempted(entries []AllowlistEntry, file, symbol string) bool {
	for _, e := range entries {
		if e.exempts(file, symbol) {
			return true
		}
	}
	return false
}

// CheckFenceScope verifies exclusions hold: no fence finding may point into
// Editor/, tests, or generated Protocol/ — enforced by scanning those trees
// and asserting every match there is out of scope. (Defence in depth; the
// main path already excludes them.)
func CheckFenceScope(root string) []string {
	files, err := collectCSharp(root)
	if err != nil {
		return []string{"collect C#: " + err.Error()}
	}
	var out []string
	for _, f := range files {
		if fenceExcluded(f.rel) {
			out = append(out, f.rel+" leaked into fence scope")
		}
	}
	return out
}

// CheckFrameLoopCallbacks asserts Unity message callbacks
// (Update/FixedUpdate/LateUpdate/OnGUI) appear only inside FrameLoop
// (PERF-020), i.e. every such finding outside the allowlisted FrameLoop file
// is a violation.
func CheckFrameLoopCallbacks(root string) []string {
	b, err := os.ReadFile(path.Join(root, allowlistFile))
	if err != nil {
		return []string{"client_api_allowlist.txt: " + err.Error()}
	}
	entries, _ := ParseAllowlist(string(b))
	files, err := collectCSharp(root)
	if err != nil {
		return []string{"collect C#: " + err.Error()}
	}
	cbRules := map[string]bool{"Update": true, "FixedUpdate": true, "LateUpdate": true, "OnGUI": true}
	var out []string
	for _, f := range files {
		for i, line := range f.lines {
			for _, rule := range fenceRules {
				if !cbRules[rule.symbol] || !rule.inScope(f.rel) || !rule.pattern.MatchString(line) {
					continue
				}
				if exempted(entries, f.rel, rule.symbol) {
					continue
				}
				out = append(out, fmt.Sprintf("%s:%d Unity callback %s outside FrameLoop (PERF-020)", f.rel, i+1, rule.symbol))
			}
		}
	}
	return out
}

// csFile is one fence-scoped C# file with masked lines.
type csFile struct {
	rel   string
	lines []string
	raw   string
}

func collectCSharp(root string) ([]csFile, error) {
	var out []csFile
	for _, base := range fenceRoots {
		err := filepath.Walk(path.Join(root, base), func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(p, ".cs") {
				return nil
			}
			rel, rerr := filepath.Rel(root, p)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)
			if fenceExcluded(rel) {
				return nil
			}
			b, rerr := os.ReadFile(p)
			if rerr != nil {
				return rerr
			}
			out = append(out, csFile{rel: rel, lines: maskSource(string(b)), raw: string(b)})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ---------- rule 14: canonical implementations ----------

// canonicalPatterns map §2.6 concern patterns to their owner dirs (client).
var canonicalPatterns = []struct {
	pattern *regexp.Regexp
	owner   string // dir prefix where the canonical implementation lives
	concern string
}{
	{regexp.MustCompile(`\b(?:class|struct|interface)\s+\w*FrameLoop\w*`), "client/Assets/Scripts/Core/Runtime/", "frame driver"},
	{regexp.MustCompile(`\b(?:class|struct|interface)\s+\w*FrameBudget\w*`), "client/Assets/Scripts/Core/Runtime/", "work scheduling"},
	{regexp.MustCompile(`\b(?:class|struct|interface)\s+\w*Scheduler\w*`), "client/Assets/Scripts/Core/Runtime/", "work scheduling"},
	{regexp.MustCompile(`\b(?:class|struct|interface)\s+\w*Pool\w*\b`), "client/Assets/Scripts/Core/Runtime/", "pooling"},
	{regexp.MustCompile(`\b(?:class|struct)\s+(?:Log|Logger|\w*Logger)\b`), "client/Assets/Scripts/Core/Runtime/", "logging"},
	{regexp.MustCompile(`\b(?:class|struct|interface)\s+\w*CameraService\w*`), "client/Assets/Scripts/Systems/Camera/", "camera service"},
	{regexp.MustCompile(`\bUnityEngine\.Pool\.`), "client/Assets/Scripts/Core/Runtime/", "pooling"},
	{regexp.MustCompile(`\bSystem\.Random\b|\bnew\s+Random\s*\(`), "client/Assets/Scripts/Core/Runtime/", "randomness"},
}

// CheckCanonical enforces rule 14 (CODE-006): a type whose name or base
// matches a §2.6 concern pattern outside its owner path fails.
func CheckCanonical(root string) []string {
	files, err := collectCSharp(root)
	if err != nil {
		return []string{"collect C#: " + err.Error()}
	}
	return checkCanonicalFiles(files)
}

func checkCanonicalFiles(files []csFile) []string {
	var out []string
	for _, f := range files {
		for i, line := range f.lines {
			for _, cp := range canonicalPatterns {
				if strings.HasPrefix(f.rel, cp.owner) {
					continue
				}
				if cp.pattern.MatchString(line) {
					out = append(out, fmt.Sprintf("%s:%d second %s implementation outside %s (%s)",
						f.rel, i+1, cp.concern, cp.owner, strings.TrimSpace(line)))
				}
			}
		}
	}
	return out
}
