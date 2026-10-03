// Command guardaction classifies a post-merge guard run: it reads the
// verification jobs' conclusions (JSON array of {name, conclusion} produced
// from the Actions jobs API) plus the ref of the PR the pushed commit merged
// and prints the GuardAction the workflow should take.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"thinhthan/internal/conformance/trusted"
)

func main() {
	jobsPath := flag.String("jobs", "", "JSON file with [{name, conclusion}] for the run's verify jobs")
	mergedRef := flag.String("merged-ref", "", "head ref of the PR the pushed commit merged (empty for direct pushes)")
	flag.Parse()

	data, err := os.ReadFile(*jobsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "guardaction: jobs file: %v\n", err)
		os.Exit(1)
	}
	var raw []struct {
		Name       string `json:"name"`
		Conclusion string `json:"conclusion"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "guardaction: parse jobs: %v\n", err)
		os.Exit(1)
	}
	jobs := make([]trusted.JobResult, 0, len(raw))
	for _, j := range raw {
		jobs = append(jobs, trusted.JobResult{Name: j.Name, Conclusion: j.Conclusion})
	}

	action := trusted.ClassifyGuard(jobs, *mergedRef)
	fmt.Printf("action=%s\n", action)
	if out := os.Getenv("GITHUB_OUTPUT"); out != "" {
		f, err := os.OpenFile(filepath.Clean(out), os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "guardaction: GITHUB_OUTPUT: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		fmt.Fprintf(f, "action=%s\n", action)
	}
}
