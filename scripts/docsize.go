// Command docsize reports documentation words for representative routes.
// Run from the repository root with: go run ./scripts/docsize.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var word = regexp.MustCompile(`\S+`)

func count(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return len(word.FindAll(b, -1)), nil
}

func skillCount(path string) (description, body int, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	parts := strings.SplitN(string(b), "---\n", 3)
	if len(parts) != 3 {
		return 0, len(word.FindAll(b, -1)), nil
	}
	for _, line := range strings.Split(parts[1], "\n") {
		if strings.HasPrefix(line, "description:") {
			description = len(word.FindAll([]byte(strings.TrimPrefix(line, "description:")), -1))
		}
	}
	return description, len(word.FindAll([]byte(parts[2]), -1)), nil
}

func main() {
	routes := map[string][]string{
		"data": {"AGENTS.md", "specs/general/roadmap.md", "specs/pages/data/README.md", "specs/pages/data/workspace.md", "specs/pages/data/ingestion.md", "specs/pages/data/datasets.md", "specs/pages/data/api.md", "specs/general/quality/common.md", "specs/general/quality/concurrency.md"},
		"jobs-results": {"AGENTS.md", "specs/general/roadmap.md", "specs/pages/jobs/README.md", "specs/pages/jobs/workspace.md", "specs/pages/jobs/runtime.md", "specs/pages/results/README.md", "specs/pages/results/workspace.md", "specs/pages/results/evaluation-and-backtesting.md", "specs/general/quality/common.md", "specs/general/quality/concurrency.md"},
		"models-inference": {"AGENTS.md", "specs/general/roadmap.md", "specs/pages/models/README.md", "specs/pages/models/workspace.md", "specs/pages/models/estimators.md", "specs/pages/models/artifacts-and-registry.md", "specs/pages/inference/README.md", "specs/pages/inference/workspace.md", "specs/pages/inference/scoring-and-export.md", "specs/general/quality/common.md", "specs/general/quality/concurrency.md"},
	}
	for _, path := range []string{"AGENTS.md"} {
		words, err := count(path)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s\t%d words\n", path, words)
	}
	entries, err := filepath.Glob(".agents/skills/*/SKILL.md")
	if err != nil {
		panic(err)
	}
	for _, path := range entries {
		description, body, err := skillCount(path)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s\tdescription=%d body=%d words\n", path, description, body)
	}
	for name, files := range routes {
		total := 0
		for _, path := range files {
			words, err := count(path)
			if err != nil {
				panic(err)
			}
			total += words
		}
		fmt.Printf("route/%s\t%d words\n", name, total)
	}
}
