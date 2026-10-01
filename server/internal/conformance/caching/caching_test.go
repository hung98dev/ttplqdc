// Package caching enforces the CI cache contract (CI-001..CI-004) for
// .github/workflows/verify.yml and its companion cache_warm.yml /
// cache_prune.yml workflows. The canonical policy is
// .devin/scripts/cache-policy.md; these tests are the executable half of it.
package caching

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"thinhthan/internal/stackpin"
)

// wfStep is one workflow step parsed structurally enough for cache policy
// assertions: top-level fields, the `with:` block (including `|` literal
// scalars), the `env:` block, and the `run:` body.
type wfStep struct {
	job         string
	fields      map[string]string
	with        map[string]string
	env         map[string]string
	pathBlock   []string
	restoreKeys []string
	run         []string
}

func (s wfStep) field(k string) string { return s.fields[k] }

// pathText returns the cache path: the inline scalar or the `|` block's
// joined lines (order preserved).
func (s wfStep) pathText() string {
	if v := s.with["path"]; v != "|" && v != "|-" && v != "" {
		return v
	}
	return strings.Join(s.pathBlock, "\n")
}

var (
	jobRe   = regexp.MustCompile(`^  ([A-Za-z0-9_-]+):$`)
	stepRe  = regexp.MustCompile(`^      - `)
	fieldRe = regexp.MustCompile(`^([A-Za-z0-9_-]+):\s*(.*)$`)
)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(wd, "..", "..", "..", ".."))
}

func readRepo(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// parseWorkflow returns every step in file order with its owning job id.
func parseWorkflow(t *testing.T, rel string) []wfStep {
	t.Helper()
	lines := strings.Split(readRepo(t, rel), "\n")
	var steps []wfStep
	var job string
	inJobs := false
	var cur *wfStep
	section := ""
	flush := func() {
		if cur != nil {
			steps = append(steps, *cur)
			cur = nil
		}
	}
	for _, ln := range lines {
		if strings.TrimRight(ln, " \t") == "jobs:" {
			inJobs = true
		}
		if !inJobs {
			continue
		}
		if m := jobRe.FindStringSubmatch(ln); m != nil {
			job = m[1]
			flush()
			section = ""
			continue
		}
		if stepRe.MatchString(ln) {
			flush()
			cur = &wfStep{job: job, fields: map[string]string{}, with: map[string]string{}, env: map[string]string{}}
			section = ""
			ln = ln[8:]
		} else {
			if cur == nil {
				continue
			}
			ln = strings.TrimPrefix(ln, "        ")
		}
		trim := strings.TrimRight(ln, " ")
		if trim == "" {
			continue
		}
		indent := len(ln) - len(strings.TrimLeft(ln, " "))
		if strings.HasPrefix(section, "withblock:") {
			if indent > 2 {
				switch section[10:] {
				case "path":
					cur.pathBlock = append(cur.pathBlock, strings.TrimSpace(ln))
				case "restore-keys":
					cur.restoreKeys = append(cur.restoreKeys, strings.TrimSpace(ln))
				}
				continue
			}
			section = "with" // block scalar ended; back to with: keys
		}
		if section == "run" && indent > 0 {
			cur.run = append(cur.run, ln)
			continue
		}
		if section == "env" && indent > 0 {
			if m := fieldRe.FindStringSubmatch(strings.TrimSpace(ln)); m != nil {
				cur.env[m[1]] = strings.Trim(m[2], "'")
			}
			continue
		}
		if section == "with" && indent > 2 {
			continue
		}
		if indent == 0 {
			section = ""
			m := fieldRe.FindStringSubmatch(trim)
			if m == nil {
				continue
			}
			k, v := m[1], strings.TrimSpace(m[2])
			switch k {
			case "with":
				section = "with"
			case "run":
				section = "run"
				if v != "" && v != "|" && v != "|-" && v != ">" && v != ">-" {
					cur.run = append(cur.run, v)
				}
			case "env":
				section = "env"
			default:
				cur.fields[k] = v
			}
			continue
		}
		if section == "with" && indent == 2 {
			m := fieldRe.FindStringSubmatch(strings.TrimSpace(ln))
			if m == nil {
				continue
			}
			k, v := m[1], strings.TrimSpace(m[2])
			cur.with[k] = v
			if v == "|" || v == "|-" {
				section = "withblock:" + k
			}
		}
	}
	flush()
	return steps
}

func cacheSteps(t *testing.T, rel string) []wfStep {
	t.Helper()
	pin := "actions/cache@" + stackpin.Actions["actions/cache"]
	var out []wfStep
	for _, s := range parseWorkflow(t, rel) {
		if s.field("uses") == pin {
			out = append(out, s)
		}
	}
	return out
}

// keyScope returns the leading scope segment of a
// `<scope>-${{ runner.os }}-...` cache key (scope names themselves contain
// dashes, so the split point is the runner.os marker).
func keyScope(key string) string {
	if i := strings.Index(key, "-${{ runner.os }}"); i > 0 {
		return key[:i]
	}
	return key
}

var workflowFiles = []string{
	".github/workflows/verify.yml",
	".github/workflows/cache_warm.yml",
	".github/workflows/cache_prune.yml",
}

func TestCacheActionPinnedSha(t *testing.T) {
	usesRe := regexp.MustCompile(`^([a-z0-9./_-]+)@([0-9a-f]{40})$`)
	for _, f := range workflowFiles {
		for _, s := range parseWorkflow(t, f) {
			u := s.field("uses")
			if u == "" {
				continue
			}
			m := usesRe.FindStringSubmatch(u)
			if m == nil {
				t.Fatalf("%s: action not pinned to a full commit SHA: %q", f, u)
			}
			want, ok := stackpin.Actions[m[1]]
			if !ok {
				t.Fatalf("%s: unlisted action %q", f, m[1])
			}
			if want != m[2] {
				t.Fatalf("%s: %s@%s, want pinned %s", f, m[1], m[2], want)
			}
		}
	}
}

func TestCacheKeysCoverPinInputs(t *testing.T) {
	for _, f := range workflowFiles {
		for _, s := range cacheSteps(t, f) {
			key := s.with["key"]
			if !strings.Contains(key, "${{ runner.os }}") {
				t.Fatalf("%s: cache key %q missing runner.os", f, key)
			}
			switch keyScope(key) {
			case "go-build":
				for _, want := range []string{"${{ env.GO_VERSION }}", "hashFiles('server/go.sum')"} {
					if !strings.Contains(key, want) {
						t.Fatalf("%s: go-build key %q missing %s", f, key, want)
					}
				}
			case "unity-editor":
				if !strings.Contains(key, "${{ env.UNITY_WINDOWS_EDITOR_SHA256 }}") {
					t.Fatalf("%s: unity-editor key %q missing editor sha pin", f, key)
				}
			case "cli-tools":
				names := map[string]string{"pwsh": "pwsh", "gh": "gh", "jq": "jq", "git-lfs": "lfs"}
				pos := 0
				for _, tool := range stackpin.CliTools {
					seg := names[tool.Name] + "-" + tool.Version + "-" + tool.WindowsSHA256[:16]
					i := strings.Index(key, seg)
					if i < pos {
						t.Fatalf("%s: cli-tools key %q missing/in-order %q", f, key, seg)
					}
					pos = i + len(seg)
				}
			case "unity-library":
				for _, want := range []string{
					"${{ env.UNITY_WINDOWS_EDITOR_SHA256 }}",
					"manifest.json", "packages-lock.json", "ProjectSettings", "csc.rsp",
				} {
					if !strings.Contains(key, want) {
						t.Fatalf("%s: unity-library key %q missing %s", f, key, want)
					}
				}
			case "edb":
				if !strings.Contains(key, "-"+stackpin.PostgresVersion+"-") {
					t.Fatalf("%s: edb key %q missing postgres version %s", f, key, stackpin.PostgresVersion)
				}
				if !strings.Contains(key, "${{ env.EDB_ZIP_SHA256 }}") {
					t.Fatalf("%s: edb key %q missing sha pin", f, key)
				}
			case "lfs-objects":
				if !strings.Contains(key, "${{ steps.lfskey.outputs.key }}") {
					t.Fatalf("%s: lfs-objects key %q missing lfskey output", f, key)
				}
			case "unity-android-module":
				if f != ".github/workflows/cache_warm.yml" {
					t.Fatalf("%s: unity-android cache belongs to cache_warm.yml only", f)
				}
				if !strings.Contains(key, "${{ steps.androidkey.outputs.key }}") {
					t.Fatalf("%s: unity-android key %q missing androidkey output", f, key)
				}
			default:
				t.Fatalf("%s: unlisted cache scope in key %q", f, key)
			}
		}
	}
}

func TestRestoreKeysNeverCrossPinOrOs(t *testing.T) {
	for _, f := range workflowFiles {
		for _, s := range cacheSteps(t, f) {
			key := s.with["key"]
			for _, rk := range s.restoreKeys {
				if !strings.Contains(rk, "${{ runner.os }}") {
					t.Fatalf("%s: restore-key %q drops runner.os", f, rk)
				}
				if !strings.HasPrefix(key, rk) {
					t.Fatalf("%s: restore-key %q is not a literal prefix of key %q (crosses pin)", f, rk, key)
				}
			}
		}
	}
}

func TestLibraryCacheExactKeyOnly(t *testing.T) {
	for _, f := range workflowFiles {
		for _, s := range cacheSteps(t, f) {
			if keyScope(s.with["key"]) != "unity-library" {
				continue
			}
			if len(s.restoreKeys) > 0 {
				t.Fatalf("%s: unity-library must restore exact-key only, got restore-keys %v", f, s.restoreKeys)
			}
		}
	}
}

// stepsConditionedOnCache returns step names whose if: references cache-hit.
func stepsConditionedOnCache(t *testing.T, rel string) []string {
	t.Helper()
	var out []string
	for _, s := range parseWorkflow(t, rel) {
		if strings.Contains(s.fields["if"], "cache-hit") {
			out = append(out, s.field("name"))
		}
	}
	return out
}

func TestNoGateSkippedOnCacheHit(t *testing.T) {
	verifyAllowed := map[string]bool{
		"Install pinned CLI tools":        true,
		"Install Unity editor":            true,
		"Provision EDB Postgres binaries": true,
	}
	for _, name := range stepsConditionedOnCache(t, ".github/workflows/verify.yml") {
		if !verifyAllowed[name] {
			t.Fatalf("verify.yml: step %q is skipped on cache-hit (only provision steps may be)", name)
		}
	}
	warmAllowed := map[string]bool{
		"Install pinned CLI tools":        true,
		"Warm go build cache":             true,
		"Install Unity editor":            true,
		"Provision EDB Postgres binaries": true,
		"Install Unity Android modules":   true,
		"Activate Unity licence":          true,
		"Materialize client":              true,
		"Return Unity licence":            true,
	}
	for _, name := range stepsConditionedOnCache(t, ".github/workflows/cache_warm.yml") {
		if !warmAllowed[name] {
			t.Fatalf("cache_warm.yml: step %q is skipped on cache-hit", name)
		}
	}
}

func TestMaterializeCommitStillRequiredOnHit(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range parseWorkflow(t, ".github/workflows/verify.yml") {
		name := s.field("name")
		cond := s.fields["if"]
		switch name {
		case "Materialize client":
			if !strings.Contains(cond, "scope == 'full'") || strings.Contains(cond, "cache-hit") {
				t.Fatalf("materialization must run on every scope=full run (if=%q)", cond)
			}
			seen["materialize"] = true
		case "Drift check (unity-materialized)":
			if !strings.Contains(cond, "always()") || strings.Contains(cond, "cache-hit") {
				t.Fatalf("drift check must run unconditionally (if=%q)", cond)
			}
			seen["drift"] = true
		case "Activate Unity licence":
			if strings.Contains(cond, "cache-hit") {
				t.Fatalf("licence activation must run every attempt (if=%q)", cond)
			}
			seen["licence"] = true
		}
	}
	for _, want := range []string{"drift", "materialize", "licence"} {
		if !seen[want] {
			t.Fatalf("verify.yml missing required step for %s", want)
		}
	}
}

func TestLicenceStateNeverCached(t *testing.T) {
	banned := []string{
		"unity-lic", "unity-cfg", "unity-cache", "unity_lic", ".ulf",
		"unity3d", "programdata\\unity", "localappdata\\unity", "licenses",
	}
	for _, f := range workflowFiles {
		for _, s := range cacheSteps(t, f) {
			blob := strings.ToLower(s.with["key"] + "\n" + s.pathText() + "\n" + strings.Join(s.restoreKeys, "\n"))
			for _, tok := range banned {
				if strings.Contains(blob, tok) {
					t.Fatalf("%s: cache step %q covers licence/credential path token %q", f, s.field("name"), tok)
				}
			}
		}
	}
}

func TestWallTimeFieldsRecorded(t *testing.T) {
	// Every actions/cache step in verify.yml must be followed (later in the
	// same job) by a telemetry emit for its cache-hit output.
	verify := parseWorkflow(t, ".github/workflows/verify.yml")
	for i, s := range verify {
		if !strings.HasPrefix(s.field("uses"), "actions/cache@") {
			continue
		}
		id := s.fields["id"]
		if id == "" {
			t.Fatalf("cache step %q (%s) has no id", s.field("name"), s.job)
		}
		needle := "steps." + id + ".outputs.cache-hit"
		found := false
		for j := i + 1; j < len(verify) && verify[j].job == s.job; j++ {
			body := strings.Join(verify[j].run, "\n")
			if strings.Contains(body, needle) &&
				(strings.Contains(body, "cache_telemetry_emit") || strings.Contains(body, `{"step":`)) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("no telemetry emit for cache step %q (id %s) in job %s", s.field("name"), id, s.job)
		}
	}
	// Emit helpers enforce the {step, result:hit|miss, wall_seconds} schema.
	sh := readRepo(t, ".devin/scripts/cache_telemetry.sh")
	for _, want := range []string{"hit|miss", `"step":"%s","result":"%s","wall_seconds":%s`, "THINHTHAN_CACHE_TELEMETRY"} {
		if !strings.Contains(sh, want) {
			t.Fatalf("cache_telemetry.sh missing %q", want)
		}
	}
	ps := readRepo(t, ".devin/scripts/cache_telemetry.ps1")
	for _, want := range []string{"Write-CacheTelemetry", "'hit', 'miss'", "wall_seconds"} {
		if !strings.Contains(ps, want) {
			t.Fatalf("cache_telemetry.ps1 missing %q", want)
		}
	}
	// verify.ps1 measures its own wall time and folds via cachemerge.
	ps1 := readRepo(t, "scripts/verify.ps1")
	for _, want := range []string{"Stopwatch", "Write-CacheTelemetry", "cachemerge", "THINHTHAN_CACHE_HIT_GO", "verify-pre-unity"} {
		if !strings.Contains(ps1, want) {
			t.Fatalf("verify.ps1 missing %q", want)
		}
	}
}

func TestEvidenceIdentityIndependentOfCache(t *testing.T) {
	// Gate/evidence steps must never carry a cache condition.
	for _, s := range parseWorkflow(t, ".github/workflows/verify.yml") {
		switch s.field("name") {
		case "Pre-Unity verify", "Unity phase verify", "Verify", "Merge evidence", "Fork guard", "Freeze check":
			if strings.Contains(s.fields["if"], "cache-hit") {
				t.Fatalf("gate step %q conditioned on cache-hit", s.field("name"))
			}
		}
	}
	// MergeReports must not carry cache state into the manifest.
	src := readRepo(t, "server/internal/conformance/gates/evidence.go")
	i := strings.Index(src, "func MergeReports")
	j := strings.Index(src[i:], "\nfunc ")
	mergeBody := src[i : i+j]
	if strings.Contains(mergeBody, ".Cache") {
		t.Fatal("MergeReports must not propagate Report.Cache into the manifest")
	}
	// verify.ps1's canonical invocation is unconditional on cache state.
	ps1 := readRepo(t, "scripts/verify.ps1")
	if !strings.Contains(ps1, "& go @args") {
		t.Fatal("verify.ps1 must invoke go run ./cmd/verify")
	}
}

func TestStableProjectionIgnoresRunTimingCacheFields(t *testing.T) {
	doc := readRepo(t, "docs/09_testing/test_and_release_evidence.md")
	for _, want := range []string{"ci_run_id", "run_attempt", "cached_steps", "wall_seconds"} {
		if !strings.Contains(doc, want) {
			t.Fatalf("§2a projection must exclude %s", want)
		}
	}
	src := readRepo(t, "server/internal/conformance/gates/evidence.go")
	i := strings.Index(src, "type Manifest struct")
	j := strings.Index(src[i:], "\n}")
	manifest := src[i : i+j]
	for _, bad := range []string{`"cached_steps"`, `"cache"`, "cached_steps"} {
		if strings.Contains(manifest, bad) {
			t.Fatalf("Manifest must not carry %s (CI-004 projection excludes it)", bad)
		}
	}
}

func TestStableProjectionDetectsGateErrorChange(t *testing.T) {
	doc := readRepo(t, "docs/09_testing/test_and_release_evidence.md")
	if !strings.Contains(doc, "(id,os,owner,result,reason)") {
		t.Fatal("§2a projection must keep the full gate tuple (id,os,owner,result,reason)")
	}
	src := readRepo(t, "server/internal/conformance/gates/report.go")
	for _, want := range []string{
		"Result Result `json:\"result\"`",
		"Reason string `json:\"reason,omitempty\"`",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("GateRow must keep %s", want)
		}
	}
}

func TestCodegenDriftFailsWithWarmCache(t *testing.T) {
	doc := readRepo(t, "docs/09_testing/test_and_release_evidence.md")
	if !strings.Contains(doc, "drift entries all equal `IDENTICAL`") {
		t.Fatal("§2a requires every codegen_drift entry to equal IDENTICAL")
	}
	// No cache may shadow generated-code inputs or the drift check itself.
	for _, f := range workflowFiles {
		for _, s := range cacheSteps(t, f) {
			p := s.pathText()
			for _, bad := range []string{"server/internal/protocol", "client/Assets/Scripts/Protocol", "proto/testdata/golden"} {
				if strings.Contains(p, bad) {
					t.Fatalf("%s: cache path %q covers generated code %s", f, p, bad)
				}
			}
		}
	}
}

func TestCacheWarmMirrorsVerifyCaches(t *testing.T) {
	verify := map[string][]wfStep{}
	for _, s := range cacheSteps(t, ".github/workflows/verify.yml") {
		scope := keyScope(s.with["key"])
		verify[scope] = append(verify[scope], s)
	}
	for _, s := range cacheSteps(t, ".github/workflows/cache_warm.yml") {
		scope := keyScope(s.with["key"])
		refs, ok := verify[scope]
		if !ok {
			if scope != "unity-android-module" {
				t.Fatalf("cache_warm.yml warms scope %q absent from verify.yml", scope)
			}
			continue
		}
		match := false
		for _, ref := range refs {
			if s.with["key"] == ref.with["key"] && s.pathText() == ref.pathText() {
				match = true
				break
			}
		}
		if !match {
			t.Fatalf("cache_warm %s cache step (key %q, path %q) equals no verify.yml step byte-for-byte",
				scope, s.with["key"], s.pathText())
		}
	}
}

func TestLfsObjectCacheKeyAndRestore(t *testing.T) {
	src := readRepo(t, ".github/workflows/verify.yml")
	if !strings.Contains(src, "GIT_LFS_SKIP_SMUDGE: '1'") {
		t.Fatal("verify.yml must set GIT_LFS_SKIP_SMUDGE=1")
	}
	steps := parseWorkflow(t, ".github/workflows/verify.yml")
	var lfskey []wfStep
	var lfsCache *wfStep
	for i := range steps {
		s := steps[i]
		if s.fields["id"] == "lfskey" {
			lfskey = append(lfskey, s)
		}
		if strings.HasPrefix(s.with["key"], "lfs-objects-") {
			cp := s
			lfsCache = &cp
		}
	}
	if len(lfskey) != 1 {
		t.Fatalf("verify.yml must have exactly one lfskey step, got %d", len(lfskey))
	}
	body := strings.Join(lfskey[0].run, "\n")
	if !strings.Contains(body, "git lfs ls-files") || !strings.Contains(body, "sha256sum") {
		t.Fatal("lfskey must hash sorted LFS object OIDs")
	}
	if lfsCache == nil {
		t.Fatal("verify.yml missing lfs-objects cache")
	}
	if lfsCache.with["key"] != "lfs-objects-${{ runner.os }}-${{ steps.lfskey.outputs.key }}" {
		t.Fatalf("lfs-objects key %q", lfsCache.with["key"])
	}
	if lfsCache.pathText() != ".git/lfs/objects" {
		t.Fatalf("lfs-objects path %q", lfsCache.pathText())
	}
	if len(lfsCache.restoreKeys) != 1 || lfsCache.restoreKeys[0] != "lfs-objects-${{ runner.os }}-" {
		t.Fatalf("lfs-objects restore-keys %v", lfsCache.restoreKeys)
	}
	// checkout steps must all disable LFS smudge; git lfs pull belongs to the
	// Unity (Windows) job only.
	pulls := 0
	for _, s := range steps {
		if strings.HasPrefix(s.field("uses"), "actions/checkout@") && s.with["lfs"] != "false" {
			t.Fatalf("checkout in %s missing lfs: false", s.job)
		}
		if strings.Contains(strings.Join(s.run, "\n"), "git lfs pull") {
			pulls++
			if s.job != "unity-windows" {
				t.Fatalf("git lfs pull in job %s (unity-windows only)", s.job)
			}
		}
	}
	if pulls != 1 {
		t.Fatalf("expected exactly one git lfs pull step, got %d", pulls)
	}
}

func TestAndroidModuleCacheKey(t *testing.T) {
	warm := readRepo(t, ".github/workflows/cache_warm.yml")
	steps := parseWorkflow(t, ".github/workflows/cache_warm.yml")
	var android *wfStep
	var keyStep *wfStep
	for i := range steps {
		if strings.HasPrefix(steps[i].with["key"], "unity-android-module-") {
			android = &steps[i]
		}
		if steps[i].fields["id"] == "androidkey" {
			keyStep = &steps[i]
		}
	}
	if android == nil {
		t.Fatal("cache_warm.yml missing unity-android-module cache step")
	}
	if android.pathText() != "${{ runner.temp }}/unity-editor/Editor/Data/PlaybackEngines/AndroidPlayer" {
		t.Fatalf("unity-android path %q", android.pathText())
	}
	if !strings.Contains(android.with["key"], "${{ steps.androidkey.outputs.key }}") {
		t.Fatalf("unity-android key %q must hash module pins via androidkey", android.with["key"])
	}
	if keyStep == nil {
		t.Fatal("cache_warm.yml missing androidkey compute step")
	}
	keyBody := strings.Join(keyStep.run, "\n")
	if !strings.Contains(keyBody, "sha256sum") {
		t.Fatal("androidkey must sha256-hash the module pin tuple")
	}
	// Every stackpin.AndroidModules entry must be pinned as env and hashed
	// into the key.
	envShas := map[string]string{}
	envRe := regexp.MustCompile(`(?m)^  ([A-Z0-9_]+)_SHA256: '([0-9a-f]{64})'$`)
	for _, m := range envRe.FindAllStringSubmatch(warm, -1) {
		envShas[m[1]] = m[2]
	}
	envURL := map[string]string{}
	urlRe := regexp.MustCompile(`(?m)^  ([A-Z0-9_]+)_URL: '(\S+)'$`)
	for _, m := range urlRe.FindAllStringSubmatch(warm, -1) {
		envURL[m[1]] = m[2]
	}
	envName := map[string]string{
		"android-support":       "ANDROID_SUPPORT",
		"openjdk-17.0.18+8":     "OPENJDK",
		"sdk-tools":             "SDK_TOOLS",
		"ndk-r27c":              "NDK",
		"cmake-3.22.1":          "CMAKE",
		"build-tools-36.0.0":    "BUILD_TOOLS",
		"platform-tools-36.0.0": "PLATFORM_TOOLS",
		"platform-34":           "PLATFORM_34",
		"platform-36":           "PLATFORM_36",
		"platform-37.0":         "PLATFORM_37",
		"commandlinetools-16.0": "COMMANDLINETOOLS",
	}
	for mod, pin := range stackpin.AndroidModules {
		env := envName[mod]
		if env == "" {
			t.Fatalf("no env mapping for android module %q", mod)
		}
		if envShas[env] != pin.SHA256 {
			t.Fatalf("env %s_SHA256=%q, want stackpin %q", env, envShas[env], pin.SHA256)
		}
		if envURL[env] != pin.URL {
			t.Fatalf("env %s_URL=%q, want stackpin %q", env, envURL[env], pin.URL)
		}
		if !strings.Contains(keyBody, "$"+env+"_SHA256") {
			t.Fatalf("androidkey must include %s_SHA256 in the hash tuple", env)
		}
	}
	if strings.Contains(readRepo(t, ".github/workflows/verify.yml"), "unity-android-module") {
		t.Fatal("verify.yml must not restore the unity-android cache (Windows player builds only)")
	}
}
