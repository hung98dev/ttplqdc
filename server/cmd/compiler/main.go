package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// resolveCatalogDir walks up from cwd for docs/07_content (canonical default
// per the Command Matrix: `go -C server run ./cmd/compiler` resolves against
// the repo checkout).
func resolveCatalogDir(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		cand := filepath.Join(dir, "docs", "07_content")
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("compiler: docs/07_content not found walking up from cwd; pass -catalogs")
		}
		dir = parent
	}
}

func main() {
	catalogs := flag.String("catalogs", "", "catalog directory (default: walk up for docs/07_content)")
	report := flag.String("report", "content_compile_report.json", "compile report output path")
	payload := flag.String("payload", "", "optional canonical payload path ('-' = stdout)")
	coverage := flag.String("coverage", "", "optional coverage report path")
	flag.Parse()

	dir, err := resolveCatalogDir(*catalogs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	code := run(dir, *report, *payload, *coverage)
	os.Exit(code)
}
