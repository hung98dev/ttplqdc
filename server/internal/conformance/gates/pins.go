package gates

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"thinhthan/internal/stackpin"
)

// CheckPins (Q1.pins) verifies every lockfile surface pins exactly the
// versions declared in the stackpin registry: server/go.mod (+ tool.go pin
// via require), client manifest.json, client ProjectVersion.txt, and the
// verify.yml workflow pins (Go/protoc/staticcheck/runner labels).
func CheckPins(root string) []string {
	var errs []string
	errs = append(errs, checkGoModPins(root)...)
	errs = append(errs, checkManifestPins(root)...)
	errs = append(errs, checkProjectVersion(root)...)
	errs = append(errs, checkWorkflowPins(root)...)
	return errs
}

// checkGoModPins: every direct require in server/go.mod must match
// stackpin.GoModulePins exactly; the go directive must match GoVersion.
func checkGoModPins(root string) []string {
	b, err := os.ReadFile(filepath.Join(root, "server", "go.mod"))
	if err != nil {
		return []string{"server/go.mod unreadable: " + err.Error()}
	}
	var errs []string
	var module, gover string
	reqs := map[string]string{}
	var inBlock bool
	for _, line := range strings.Split(string(b), "\n") {
		l := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(l, "module "):
			module = strings.TrimSpace(strings.TrimPrefix(l, "module "))
		case strings.HasPrefix(l, "go ") && gover == "":
			gover = strings.TrimSpace(strings.TrimPrefix(l, "go "))
		case strings.HasPrefix(l, "require ("):
			inBlock = true
		case inBlock && l == ")":
			inBlock = false
		case strings.HasPrefix(l, "require ") && !inBlock:
			addReq(reqs, strings.TrimPrefix(l, "require "))
		case inBlock:
			addReq(reqs, l)
		}
	}
	if module != stackpin.GoModuleName {
		errs = append(errs, fmt.Sprintf("go.mod module %q, want %q", module, stackpin.GoModuleName))
	}
	if gover != stackpin.GoVersion {
		errs = append(errs, fmt.Sprintf("go.mod go directive %q, want %q", gover, stackpin.GoVersion))
	}
	for mod, v := range reqs {
		want, ok := stackpin.GoModulePins[mod]
		if !ok {
			errs = append(errs, fmt.Sprintf("go.mod requires unlisted module %s %s", mod, v))
			continue
		}
		if v != want {
			errs = append(errs, fmt.Sprintf("go.mod %s %s, want %s", mod, v, want))
		}
	}
	sort.Strings(errs)
	return errs
}

func addReq(reqs map[string]string, l string) {
	l = strings.TrimSpace(l)
	if l == "" || strings.HasPrefix(l, "//") {
		return
	}
	if i := strings.Index(l, "//"); i >= 0 {
		l = strings.TrimSpace(l[:i])
	}
	parts := strings.Fields(l)
	if len(parts) >= 2 {
		reqs[parts[0]] = parts[1]
	}
}

// checkManifestPins: client/Packages/manifest.json dependencies must equal the
// pinned package set exactly (no extras, no different versions).
func checkManifestPins(root string) []string {
	b, err := os.ReadFile(filepath.Join(root, "client", "Packages", "manifest.json"))
	if err != nil {
		return []string{"client/Packages/manifest.json unreadable: " + err.Error()}
	}
	var m struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return []string{"manifest.json invalid JSON: " + err.Error()}
	}
	var errs []string
	for pkg, v := range m.Dependencies {
		want, ok := stackpin.UnityPackagePins[pkg]
		if !ok {
			errs = append(errs, fmt.Sprintf("manifest.json has unlisted package %s %s", pkg, v))
			continue
		}
		if v != want {
			errs = append(errs, fmt.Sprintf("manifest.json %s %s, want %s", pkg, v, want))
		}
	}
	for pkg, want := range stackpin.UnityPackagePins {
		if _, ok := m.Dependencies[pkg]; !ok {
			errs = append(errs, fmt.Sprintf("manifest.json missing pinned package %s %s", pkg, want))
		}
	}
	sort.Strings(errs)
	return errs
}

// checkProjectVersion: ProjectVersion.txt must carry the pinned editor and
// changeset exactly.
func checkProjectVersion(root string) []string {
	b, err := os.ReadFile(filepath.Join(root, "client", "ProjectSettings", "ProjectVersion.txt"))
	if err != nil {
		return []string{"client/ProjectSettings/ProjectVersion.txt unreadable: " + err.Error()}
	}
	var errs []string
	s := string(b)
	wantV := "m_EditorVersion: " + stackpin.UnityEditorVersion
	wantC := "m_EditorVersionWithRevision: " + stackpin.UnityEditorVersion + " (" + stackpin.UnityEditorChangeset + ")"
	if !strings.Contains(s, wantV) {
		errs = append(errs, fmt.Sprintf("ProjectVersion.txt missing %q", wantV))
	}
	if !strings.Contains(s, wantC) {
		errs = append(errs, fmt.Sprintf("ProjectVersion.txt missing %q", wantC))
	}
	return errs
}

var (
	usesRe   = regexp.MustCompile(`uses:\s*([a-zA-Z0-9_./-]+)@([^\s#]+)`)
	runsOnRe = regexp.MustCompile(`runs-on:\s*([a-zA-Z0-9._-]+)`)
	shaRe    = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// checkWorkflowPins: every `uses:` must be a full 40-hex SHA in the
// stackpin.Actions allowlist; every runs-on must be a pinned runner label.
func checkWorkflowPins(root string) []string {
	b, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "verify.yml"))
	if err != nil {
		return []string{".github/workflows/verify.yml unreadable: " + err.Error()}
	}
	var errs []string
	for i, line := range strings.Split(string(b), "\n") {
		if m := usesRe.FindStringSubmatch(line); m != nil {
			action, ref := m[1], m[2]
			if !shaRe.MatchString(ref) {
				errs = append(errs, fmt.Sprintf("verify.yml:%d %s not pinned to a 40-hex SHA (%q)", i+1, action, ref))
				continue
			}
			if want, ok := stackpin.Actions[action]; !ok || want != ref {
				errs = append(errs, fmt.Sprintf("verify.yml:%d %s@%s not in pin registry", i+1, action, ref))
			}
		}
		if m := runsOnRe.FindStringSubmatch(line); m != nil {
			label := m[1]
			ok := false
			for _, l := range stackpin.RunnerLabels {
				if l == label {
					ok = true
				}
			}
			if !ok {
				errs = append(errs, fmt.Sprintf("verify.yml:%d runs-on %q not a pinned label", i+1, label))
			}
		}
	}
	return errs
}

// CheckForbiddenDeps (Q1.forbidden_deps): go.mod direct requires are exactly
// the pinned set (checked above); here we check the *full* module closure in
// go.sum — every module in go.sum must be either a GoModulePins member or on
// the TransitiveModuleAllowlist — plus forbidden module/import prefixes.
func CheckForbiddenDeps(root string) []string {
	var errs []string
	b, err := os.ReadFile(filepath.Join(root, "server", "go.sum"))
	if err != nil {
		return []string{"server/go.sum unreadable: " + err.Error()}
	}
	seen := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		mod, ver := f[0], f[1]
		ver = strings.TrimSuffix(ver, "/go.mod")
		if prev, ok := seen[mod]; !ok || ver > prev {
			seen[mod] = ver
		}
	}
	for mod := range seen {
		if _, ok := stackpin.GoModulePins[mod]; ok {
			continue
		}
		if _, ok := stackpin.TransitiveModuleAllowlist[mod]; ok {
			continue
		}
		for _, fmod := range stackpin.ForbiddenModules {
			if mod == fmod || strings.HasPrefix(mod, fmod+"/") {
				errs = append(errs, "forbidden module in go.sum: "+mod)
			}
		}
	}
	// Unlisted-but-benign transitive modules are the go.sum lock exception;
	// only *forbidden* entries are hard failures. Report them sorted.
	sort.Strings(errs)
	return errs
}

// CheckForbiddenImports scans first-party Go files for forbidden import
// prefixes (gorilla, gRPC service imports, etc.). stdlib + thinhthan/* +
// pinned modules only.
func CheckForbiddenImports(root string) []string {
	var errs []string
	// import spec: optional alias/blank/dot then the quoted path.
	impRe := regexp.MustCompile(`^\s*(?:[A-Za-z_.]+\s+)?"([^"]+)"`)
	err := filepath.Walk(filepath.Join(root, "server"), func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || !strings.HasSuffix(fi.Name(), ".go") {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var inBlock bool
		for _, line := range strings.Split(string(b), "\n") {
			l := strings.TrimSpace(line)
			if strings.HasPrefix(l, "import (") {
				inBlock = true
				continue
			}
			if inBlock && l == ")" {
				inBlock = false
				continue
			}
			var imp string
			if m := impRe.FindStringSubmatch(l); m != nil && (inBlock || strings.HasPrefix(l, "import")) {
				imp = m[1]
			} else if strings.HasPrefix(l, "import") && strings.Contains(l, "\"") {
				if m := impRe.FindStringSubmatch(strings.TrimPrefix(l, "import")); m != nil {
					imp = m[1]
				}
			}
			if imp == "" {
				continue
			}
			for _, fp := range stackpin.ForbiddenImportPrefixes {
				match := false
				if strings.HasSuffix(fp, "/") {
					match = strings.HasPrefix(imp, fp)
				} else {
					match = imp == fp || strings.HasPrefix(imp, fp+"/")
				}
				if match {
					errs = append(errs, fmt.Sprintf("%s imports forbidden %q", filepath.ToSlash(rel), imp))
				}
			}
			for _, fx := range stackpin.ForbiddenImportExact {
				if imp == fx {
					errs = append(errs, fmt.Sprintf("%s imports forbidden %q", filepath.ToSlash(rel), imp))
				}
			}
		}
		return nil
	})
	if err != nil {
		errs = append(errs, "walk server/: "+err.Error())
	}
	sort.Strings(errs)
	return errs
}
