// Package style implements the C# source conventions of
// engineering_conventions.md §2.7/§1.1 as checkable text rules: Allman
// braces, 4-space indentation, LF endings/no BOM/final newline, _camelCase
// private fields, one type per file named after the file, and a
// block-scoped namespace derived from the owning asmdef directory.
package style

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// CheckCSharpFile runs every CODE-002 text rule over one .cs file's bytes.
func CheckCSharpFile(repoRoot, path string, src []byte) []string {
	var errs []string
	errs = append(errs, CheckLineEndingsBOM(src)...)
	errs = append(errs, CheckIndentWhitespace(src)...)
	errs = append(errs, CheckBraceLines(src)...)
	errs = append(errs, CheckPrivateFieldNaming(src)...)
	errs = append(errs, CheckOneTypePerFile(path, src)...)
	errs = append(errs, CheckNamespaceDecl(repoRoot, path, src)...)
	errs = append(errs, CheckLintFileIgnore(src)...)
	return errs
}

// CheckCSharpTree walks client/Assets/**/*.cs and checks each file.
func CheckCSharpTree(repoRoot string) []string {
	var errs []string
	root := filepath.Join(repoRoot, "client", "Assets")
	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || !strings.HasSuffix(fi.Name(), ".cs") {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(repoRoot, p)
		for _, e := range CheckCSharpFile(repoRoot, filepath.ToSlash(rel), b) {
			errs = append(errs, fmt.Sprintf("%s: %s", filepath.ToSlash(rel), e))
		}
		return nil
	})
	sort.Strings(errs)
	return errs
}

// CheckLineEndingsBOM: LF only, no BOM, file ends with a newline.
func CheckLineEndingsBOM(src []byte) []string {
	var errs []string
	if len(src) == 0 {
		return []string{"empty file"}
	}
	if src[0] == 0xEF {
		errs = append(errs, "UTF-8 BOM present")
	}
	for i, b := range src {
		if b == '\r' {
			errs = append(errs, "CR byte present (LF required)")
			break
		}
		_ = i
	}
	if src[len(src)-1] != '\n' {
		errs = append(errs, "missing final newline")
	}
	return errs
}

// CheckIndentWhitespace: 4-space indent, no tabs, no trailing whitespace.
func CheckIndentWhitespace(src []byte) []string {
	var errs []string
	for i, l := range strings.Split(string(src), "\n") {
		if strings.HasSuffix(l, " ") || strings.HasSuffix(l, "\t") {
			errs = append(errs, line(i, "trailing whitespace"))
		}
		lead := len(l) - len(strings.TrimLeft(l, " "))
		if strings.ContainsRune(l[:len(l)-len(strings.TrimLeft(l, " \t"))], '\t') {
			errs = append(errs, line(i, "tab indentation"))
		}
		if lead%4 != 0 && strings.TrimSpace(l) != "" {
			errs = append(errs, line(i, "indent not a multiple of 4"))
		}
	}
	return dedupe(errs)
}

func line(i int, msg string) string { return fmt.Sprintf("line %d: %s", i+1, msg) }

func dedupe(in []string) []string {
	if len(in) <= 1 {
		return in
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// stripCommentsStrings removes `//` comments, string and char literals so
// brace checks don't fire inside them. Handles verbatim strings loosely.
func stripCommentsStrings(line string) string {
	var b strings.Builder
	inStr, inChr, esc := false, false, false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case esc:
			esc = false
		case inStr:
			if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
		case inChr:
			if c == '\\' {
				esc = true
			} else if c == '\'' {
				inChr = false
			}
		case c == '/' && i+1 < len(line) && line[i+1] == '/':
			return b.String()
		case c == '"':
			inStr = true
		case c == '\'':
			inChr = true
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// exprBracePrefix reports whether the content preceding the first '{' marks
// it as an expression brace (object/collection initializer, lambda, switch
// expression) rather than a block brace: Allman rules do not apply to it.
func exprBracePrefix(prefix string) bool {
	p := strings.TrimSpace(prefix)
	if p == "" {
		return false
	}
	for _, m := range []string{"=>", "new ", "new[", "return "} {
		if strings.Contains(p, m) {
			return true
		}
	}
	// A bare '=' marks initializer context (`var x = new List<int> {`) but
	// '==' '!=' '<=' '>=' are comparisons, not initializers.
	masked := strings.NewReplacer("==", "", "!=", "", "<=", "", ">=", "").Replace(p)
	if strings.Contains(masked, "=") {
		return true
	}
	// An unclosed '(' puts the brace inside an argument list — always
	// expression context (`Split(new[] { " and " }, opts)`).
	if strings.Count(p, "(") > strings.Count(p, ")") {
		return true
	}
	switch p[len(p)-1] {
	case '(', '[', ',', '{', '}':
		return true
	}
	return false
}

// CheckBraceLines enforces Allman braces: block '{' and '}' must each be the
// first non-whitespace character of their line; '}' may only be followed by
// expression terminators (`,`, `;`, `)`) — never `else`/`while`/`catch`/`finally`
// (csharp_new_line_before_open_brace=all, new_line_before_else=true).
func CheckBraceLines(src []byte) []string {
	var errs []string
	for i, raw := range strings.Split(string(src), "\n") {
		code := stripCommentsStrings(raw)
		trim := strings.TrimSpace(code)
		if strings.Contains(code, "{") && !strings.HasPrefix(trim, "{") && !strings.HasPrefix(trim, "}") {
			if !exprBracePrefix(code[:strings.Index(code, "{")]) {
				errs = append(errs, line(i, "'{' not on its own line"))
			}
		}
		if strings.Contains(code, "}") && !strings.HasPrefix(trim, "}") && !strings.HasPrefix(trim, "{") {
			if open := strings.Index(code, "{"); open < 0 || open > strings.Index(code, "}") || !exprBracePrefix(code[:strings.Index(code, "{")]) {
				errs = append(errs, line(i, "'}' not on its own line"))
			}
		}
		// '}' leading the line but followed by a block keyword is a violation.
		if strings.HasPrefix(trim, "}") {
			rest := strings.TrimSpace(strings.TrimLeft(trim, "}"))
			rest = strings.TrimRight(rest, ",;)")
			if rest != "" && rest != "}" {
				errs = append(errs, line(i, "'}' followed by "+rest+"' on same line"))
			}
		}
	}
	return errs
}

var privateFieldRe = regexp.MustCompile(
	`^\s*private\s+(?:readonly\s+)?(?:[\w<>\[\].?]+)\s+([A-Za-z_]\w*)\s*(?:=|;)`)

// CheckPrivateFieldNaming: private (non-static, non-const) instance fields
// must be _camelCase.
func CheckPrivateFieldNaming(src []byte) []string {
	var errs []string
	for i, raw := range strings.Split(string(src), "\n") {
		code := stripCommentsStrings(raw)
		if strings.Contains(code, "static") || strings.Contains(code, "const") {
			continue
		}
		if m := privateFieldRe.FindStringSubmatch(code); m != nil {
			name := m[1]
			if !strings.HasPrefix(name, "_") {
				errs = append(errs, line(i, fmt.Sprintf("private field %q must be _camelCase", name)))
			}
		}
	}
	return errs
}

var typeDeclRe = regexp.MustCompile(`\b(?:class|struct|interface|enum|record)\s+([A-Za-z_]\w*)`)

// CheckOneTypePerFile: exactly one top-level type declaration per file,
// named after the file. Nested types (depth >= 2) are permitted.
func CheckOneTypePerFile(path string, src []byte) []string {
	var names []string
	depth := 0
	for _, raw := range strings.Split(string(src), "\n") {
		code := stripCommentsStrings(raw)
		if m := typeDeclRe.FindStringSubmatch(code); m != nil && depth <= 1 {
			names = append(names, m[1])
		}
		depth += strings.Count(code, "{") - strings.Count(code, "}")
		if depth < 0 {
			depth = 0
		}
	}
	var errs []string
	base := strings.TrimSuffix(filepath.Base(path), ".cs")
	switch {
	case len(names) == 0:
		errs = append(errs, "no type declaration")
	case len(names) > 1:
		errs = append(errs, fmt.Sprintf("%d type declarations %v (one type per file)", len(names), names))
	case names[0] != base:
		errs = append(errs, fmt.Sprintf("type %q != file name %q", names[0], base))
	}
	return errs
}

var nsDeclRe = regexp.MustCompile(`^\s*namespace\s+([A-Za-z_][\w.]*)`)

// CheckNamespaceDecl: file must declare a block-scoped namespace equal to
// the owning asmdef name plus any subdirectories beneath it.
func CheckNamespaceDecl(repoRoot, path string, src []byte) []string {
	var ns string
	for _, raw := range strings.Split(string(src), "\n") {
		if m := nsDeclRe.FindStringSubmatch(stripCommentsStrings(raw)); m != nil {
			ns = m[1]
			break
		}
	}
	if ns == "" {
		return []string{"no namespace declaration"}
	}
	want := expectedNamespace(repoRoot, path)
	if want != "" && ns != want {
		return []string{fmt.Sprintf("namespace %q, want %q (asmdef dir + subpath)", ns, want)}
	}
	return nil
}

// expectedNamespace resolves the namespace from the nearest ancestor .asmdef
// plus the folder path beneath it.
func expectedNamespace(repoRoot, relPath string) string {
	dir := filepath.Dir(filepath.Join(repoRoot, filepath.FromSlash(relPath)))
	var asmName, asmDir string
	for {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return ""
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".asmdef") {
				asmName = strings.TrimSuffix(e.Name(), ".asmdef")
				asmDir = dir
			}
		}
		if asmName != "" {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir || !strings.HasPrefix(dir, filepath.Join(repoRoot, "client")) {
			return ""
		}
		dir = parent
	}
	sub, err := filepath.Rel(asmDir, filepath.Dir(filepath.Join(repoRoot, filepath.FromSlash(relPath))))
	if err != nil || sub == "." {
		return asmName
	}
	return asmName + "." + strings.ReplaceAll(filepath.ToSlash(sub), "/", ".")
}

// CheckLintFileIgnore rejects //lint:file-ignore (CODE-003).
func CheckLintFileIgnore(src []byte) []string {
	if strings.Contains(string(src), "//lint:file-ignore") || strings.Contains(string(src), "// lint:file-ignore") {
		return []string{"forbidden //lint:file-ignore"}
	}
	return nil
}
