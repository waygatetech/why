// Package plan reads the decisions recorded in a tix plan's frontmatter.
package plan

import (
	"bytes"
	"fmt"
	"io"
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
	// RuledBy names who ruled, when a human did. tix plans leave it unset.
	RuledBy string `yaml:"ruled_by"`
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
	f, err := os.Open(path)
	if err != nil {
		return Plan{}, fmt.Errorf("reading plan: %w", err)
	}
	defer f.Close()
	return Parse(f, path)
}

// Parse parses plan frontmatter from r; name labels errors. The ticket may
// be empty: callers that can supply one decide whether that is an error.
func Parse(r io.Reader, name string) (Plan, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Plan{}, fmt.Errorf("reading plan %s: %w", name, err)
	}
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(data, []byte(delim)) {
		return Plan{}, fmt.Errorf("parsing plan %s: missing frontmatter", name)
	}
	front, _, ok := bytes.Cut(data[len(delim):], []byte("\n"+delim))
	if !ok {
		return Plan{}, fmt.Errorf("parsing plan %s: unterminated frontmatter", name)
	}
	var p Plan
	if err := yaml.Unmarshal(front, &p); err != nil {
		return Plan{}, fmt.Errorf("parsing plan %s frontmatter: %w", name, err)
	}
	return p, nil
}
