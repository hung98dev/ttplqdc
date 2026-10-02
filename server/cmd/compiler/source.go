package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"thinhthan/internal/config"
)

// File is one loaded catalog source: UTF-8 (BOM rejected), line endings
// normalized to LF, with a lexed section tree.
type File struct {
	Name  string // catalog filename, e.g. monster_catalog.md
	Path  string
	Lines []string
	Root  *Section
	diags *config.Diagnostics
}

// LoadFile reads and lexes one catalog file.
func LoadFile(path string, diags *config.Diagnostics) (*File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		diags.Addf(config.DiagCatalogFileNotFound, path, 0, "%v", err)
		return nil, err
	}
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		diags.Addf(config.DiagTableSyntaxError, path, 1, "UTF-8 BOM not permitted in catalog source")
		return nil, fmt.Errorf("%s: BOM", path)
	}
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	f := &File{
		Name:  filepath.Base(path),
		Path:  path,
		Lines: strings.Split(text, "\n"),
		diags: diags,
	}
	f.Root = lex(f)
	return f, nil
}

// FindSections returns descendant sections whose title matches exactly
// (after trimming). Locators use exact heading ancestry per §1.
func (s *Section) FindSections(title string) []*Section {
	var out []*Section
	var walk func(sec *Section)
	walk = func(sec *Section) {
		if sec != s && sec.Title == title {
			out = append(out, sec)
		}
		for _, c := range sec.Children {
			walk(c)
		}
	}
	walk(s)
	return out
}

// SectionAt resolves a heading ancestry path like
// "Compiler Source Schema" or "Launch Roster — 58 > Act I — Kiến Vàng".
// '>' separates ancestry levels. Each step must match a descendant's exact
// title; deeper matches under already-matched ancestors win.
func (s *Section) SectionAt(path string) *Section {
	parts := strings.Split(path, ">")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	cur := s
	for _, p := range parts {
		var next *Section
		var find func(sec *Section)
		find = func(sec *Section) {
			if next != nil {
				return
			}
			for _, c := range sec.Children {
				if c.Title == p {
					next = c
					return
				}
				find(c)
			}
		}
		find(cur)
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}
