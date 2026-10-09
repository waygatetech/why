package decision

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Load parses every *.md file in dir. A missing dir means no decisions.
func Load(dir string) ([]Decision, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", dir, err)
	}
	var ds []Decision
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", p, err)
		}
		d, err := Parse(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		ds = append(ds, d)
	}
	return ds, nil
}

// Active drops decisions that another decision supersedes.
func Active(ds []Decision) []Decision {
	superseded := map[string]bool{}
	for _, d := range ds {
		for _, id := range d.Supersedes {
			superseded[id] = true
		}
	}
	var out []Decision
	for _, d := range ds {
		if !superseded[d.ID] {
			out = append(out, d)
		}
	}
	return out
}

// NextID returns <ticket>-<n>, one past the highest n already used for ticket.
func NextID(ds []Decision, ticket string) string {
	maxN := 0
	for _, d := range ds {
		if s, ok := strings.CutPrefix(d.ID, ticket+"-"); ok {
			if n, err := strconv.Atoi(s); err == nil && n > maxN {
				maxN = n
			}
		}
	}
	return fmt.Sprintf("%s-%d", ticket, maxN+1)
}

// Write stores d as dir/<id>.md, creating dir if needed. It never overwrites
// an existing decision.
func Write(dir string, d Decision) (string, error) {
	if d.ID == "" || strings.ContainsAny(d.ID, `/\`) {
		return "", fmt.Errorf("writing decision: invalid id %q", d.ID)
	}
	data, err := d.Marshal()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}
	path := filepath.Join(dir, d.ID+".md")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", fmt.Errorf("writing decision %s: already exists", d.ID)
		}
		return "", fmt.Errorf("writing decision %s: %w", d.ID, err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("closing %s: %w", path, err)
	}
	return path, nil
}
