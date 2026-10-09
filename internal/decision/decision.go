// Package decision defines the on-disk format of a decision file.
//
// A decision file is markdown with YAML frontmatter followed by exactly the
// sections Question, Ruling and Why:
//
//	---
//	id: ...
//	ticket: why-590
//	concepts: [decision-format]
//	date: 2026-10-08
//	decided_by: human
//	supersedes: [...]
//	---
//
//	## Question
//	...
//
//	## Ruling
//	...
//
//	## Why
//	...
package decision

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Decision is one recorded ruling.
type Decision struct {
	ID         string   `yaml:"id"`
	Ticket     string   `yaml:"ticket"`
	Concepts   []string `yaml:"concepts,omitempty"`
	Date       string   `yaml:"date"` // YYYY-MM-DD
	DecidedBy  string   `yaml:"decided_by"`
	Supersedes []string `yaml:"supersedes,omitempty"`

	Question string `yaml:"-"`
	Ruling   string `yaml:"-"`
	Why      string `yaml:"-"`
}

// Provenance values for DecidedBy.
const (
	Human         = "human"
	AgentApproved = "agent-proposed-human-approved"
	Agent         = "agent"
)

// IsHuman reports whether decidedBy claims a human ruled.
func IsHuman(decidedBy string) bool {
	return decidedBy == Human || decidedBy == AgentApproved
}

func checkDecidedBy(decidedBy string) error {
	if decidedBy != Agent && !IsHuman(decidedBy) {
		return fmt.Errorf("invalid decided_by %q (want %s, %s or %s)", decidedBy, Human, AgentApproved, Agent)
	}
	return nil
}

// sections lists the body headings in file order.
var sections = []string{"Question", "Ruling", "Why"}

const delim = "---\n"

// Parse reads a decision file. Unknown frontmatter keys, an unknown
// decided_by, unknown or duplicate sections, and text outside a section are
// errors. An empty decided_by is allowed so callers can fill it in.
func Parse(r io.Reader) (Decision, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Decision{}, fmt.Errorf("reading decision: %w", err)
	}
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(data, []byte(delim)) {
		return Decision{}, errors.New("parsing decision: missing frontmatter")
	}
	front, body, ok := bytes.Cut(data[len(delim):], []byte("\n"+delim))
	if !ok {
		return Decision{}, errors.New("parsing decision: unterminated frontmatter")
	}

	var d Decision
	dec := yaml.NewDecoder(bytes.NewReader(front))
	dec.KnownFields(true)
	if err := dec.Decode(&d); err != nil && !errors.Is(err, io.EOF) {
		return Decision{}, fmt.Errorf("parsing decision frontmatter: %w", err)
	}
	if d.DecidedBy != "" {
		if err := checkDecidedBy(d.DecidedBy); err != nil {
			return Decision{}, fmt.Errorf("parsing decision: %w", err)
		}
	}

	text := map[string]*strings.Builder{}
	var cur *strings.Builder
	sc := bufio.NewScanner(bytes.NewReader(body))
	for sc.Scan() {
		line := sc.Text()
		if heading, ok := strings.CutPrefix(line, "## "); ok {
			heading = strings.TrimSpace(heading)
			if !slices.Contains(sections, heading) {
				return Decision{}, fmt.Errorf("parsing decision: unknown section %q", heading)
			}
			if text[heading] != nil {
				return Decision{}, fmt.Errorf("parsing decision: duplicate section %q", heading)
			}
			cur = &strings.Builder{}
			text[heading] = cur
			continue
		}
		if cur == nil {
			if strings.TrimSpace(line) != "" {
				return Decision{}, fmt.Errorf("parsing decision: text outside a section: %q", line)
			}
			continue
		}
		cur.WriteString(line)
		cur.WriteByte('\n')
	}
	if err := sc.Err(); err != nil {
		return Decision{}, fmt.Errorf("parsing decision body: %w", err)
	}

	get := func(s string) string {
		if b := text[s]; b != nil {
			return strings.TrimSpace(b.String())
		}
		return ""
	}
	d.Question, d.Ruling, d.Why = get("Question"), get("Ruling"), get("Why")
	return d, nil
}

// Marshal renders d in the decision file format.
func (d Decision) Marshal() ([]byte, error) {
	if err := checkDecidedBy(d.DecidedBy); err != nil {
		return nil, fmt.Errorf("marshaling decision %s: %w", d.ID, err)
	}
	front, err := yaml.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("marshaling decision %s: %w", d.ID, err)
	}
	var b bytes.Buffer
	b.WriteString(delim)
	b.Write(front)
	b.WriteString(delim)
	for i, text := range []string{d.Question, d.Ruling, d.Why} {
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", sections[i], strings.TrimSpace(text))
	}
	return b.Bytes(), nil
}
