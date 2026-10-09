// Package plan reads the decisions recorded in a tix plan's frontmatter.
package plan

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Entry is one decision in a plan. tix owns the schema; once the plan is
// approved, Recommend holds the human's ruling.
type Entry struct {
	Q          string   `yaml:"q"`
	Options    []string `yaml:"options"`
	Recommend  string   `yaml:"recommend"`
	Why        string   `yaml:"why"`
	Supersedes []string `yaml:"supersedes"`
}

// Plan is the subset of plan frontmatter why cares about. Other keys are
// ignored.
type Plan struct {
	Ticket      string   `yaml:"ticket"`
	Concepts    []string `yaml:"concepts"`
	NewConcepts []string `yaml:"new_concepts"`
	Decisions   []Entry  `yaml:"decisions"`
}

const delim = "---\n"

// Read parses the frontmatter of the plan file at path.
func Read(path string) (Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Plan{}, fmt.Errorf("reading plan: %w", err)
	}
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(data, []byte(delim)) {
		return Plan{}, fmt.Errorf("parsing plan %s: missing frontmatter", path)
	}
	front, _, ok := bytes.Cut(data[len(delim):], []byte("\n"+delim))
	if !ok {
		return Plan{}, fmt.Errorf("parsing plan %s: unterminated frontmatter", path)
	}
	var p Plan
	if err := yaml.Unmarshal(front, &p); err != nil {
		return Plan{}, fmt.Errorf("parsing plan %s frontmatter: %w", path, err)
	}
	if p.Ticket == "" {
		return Plan{}, fmt.Errorf("parsing plan %s: no ticket", path)
	}
	return p, nil
}
