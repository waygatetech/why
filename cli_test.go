package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testPlan = `---
ticket: why-7
concepts: [cli]
new_concepts: [store]
paths: [x]
decisions:
  - q: "Ruling source?"
    options: [a, b]
    recommend: "a"
    why: "Simpler."
  - q: "IDs?"
    options: [c, d]
    recommend: "d"
    why: "No collisions."
---
body
`

func run(t *testing.T, stdin string, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	cmd := newRootCmd()
	cmd.SetArgs(args)
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("why %v: %v\n%s", args, err, out.String())
	}
	return out.String()
}

func TestRecordAndList(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plan.md"), []byte(testPlan), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)

	run(t, "", "record", "--from-plan", "plan.md")
	if out := run(t, "", "record", "--from-plan", "plan.md"); out != "" {
		t.Errorf("re-recording plan wrote %q, want nothing", out)
	}
	run(t, "", "record", "--ticket", "why-8", "--concept", "other", "--question", "Q?", "--ruling", "R.", "--why", "W.")
	run(t, "---\nticket: why-8\nconcepts: [other]\nsupersedes: [why-8-1]\n---\n## Ruling\nR2.\n", "record")
	// Tests have no controlling terminal, so human provenance is downgraded.
	run(t, "", "record", "--ticket", "why-9", "--ruling", "R.", "--decided-by", "human")
	data, err := os.ReadFile(filepath.Join(root, "decisions", "why-9-1.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "decided_by: agent\n") {
		t.Errorf("record --decided-by human without a TTY wrote:\n%s\nwant decided_by: agent", data)
	}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"all active", []string{"list"}, "why-7-1  [cli,store]  a\nwhy-7-2  [cli,store]  d\nwhy-8-2  [other]  R2.\nwhy-9-1  []  R.\n"},
		{"by concept", []string{"list", "--concept", "store"}, "why-7-1  [cli,store]  a\nwhy-7-2  [cli,store]  d\n"},
		{"markdown", []string{"list", "--concept", "other", "--format", "md"}, "## why-8-2: \n\n**Ruling:** R2.\n\n**Why:** \n\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, "", tt.args...); got != tt.want {
				t.Errorf("why %v =\n%s\nwant\n%s", tt.args, got, tt.want)
			}
		})
	}
}
