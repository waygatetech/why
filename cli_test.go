package main

import (
	"bytes"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waygatetech/why/internal/decision"
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
	if out, err := exec.Command("git", "init", "-q", "-b", "main", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	t.Setenv("TIX_HOOK", "")
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
	// Outside the tix approve hook human provenance is downgraded.
	run(t, "", "record", "--ticket", "why-9", "--ruling", "R.", "--decided-by", "human")
	data, err := os.ReadFile(filepath.Join(root, "decisions", "why-9-1.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "decided_by: agent\n") {
		t.Errorf("record --decided-by human outside the approve hook wrote:\n%s\nwant decided_by: agent", data)
	}
	t.Setenv("TIX_HOOK", "approve")
	run(t, "", "record", "--ticket", "why-10", "--ruling", "R.", "--decided-by", "human")
	t.Setenv("TIX_HOOK", "")
	data, err = os.ReadFile(filepath.Join(root, "decisions", "why-10-1.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "decided_by: human\n") {
		t.Errorf("record --decided-by human under the approve hook wrote:\n%s\nwant decided_by: human", data)
	}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"all active", []string{"list"}, "why-10-1  []  human  R.\nwhy-7-1  [cli,store]  agent  a\nwhy-7-2  [cli,store]  agent  d\nwhy-8-2  [other]  agent  R2.\nwhy-9-1  []  agent  R.\n"},
		{"by concept", []string{"list", "--concept", "store"}, "why-7-1  [cli,store]  agent  a\nwhy-7-2  [cli,store]  agent  d\n"},
		{"markdown", []string{"list", "--concept", "other", "--format", "md"}, "## why-8-2: \n\n**Ruling:** R2.\n\n**Why:** \n\n"},
		{"all", []string{"list", "--all", "--concept", "other"}, "why-8-1  [other]  agent  R.\nwhy-8-2  [other]  agent  R2.\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(t, "", tt.args...); got != tt.want {
				t.Errorf("why %v =\n%s\nwant\n%s", tt.args, got, tt.want)
			}
		})
	}

	run(t, "", "record", "--ticket", "why-8", "--ruling", "R3.", "--supersedes", "why-8-2")
	shows := []struct{ id, want string }{
		{"why-8-1", "\n## Why\n\nW.\n\nsuperseded by: why-8-2  [agent]  R2.\nsuperseded by: why-8-3  [agent]  R3.\n"},
		{"why-8-3", "\n## Ruling\n\nR3.\n\n## Why\n\n\n\nsupersedes: why-8-2  [agent]  R2.\nsupersedes: why-8-1  [agent]  R.\n"},
	}
	for _, tt := range shows {
		t.Run("show "+tt.id, func(t *testing.T) {
			if got := run(t, "", "show", tt.id); !strings.HasSuffix(got, tt.want) {
				t.Errorf("why show %s =\n%s\nwant suffix\n%s", tt.id, got, tt.want)
			}
		})
	}

	cmd := newRootCmd()
	cmd.SetArgs([]string{"record", "--ticket", "why-8", "--ruling", "R.", "--supersedes", "why-8-99"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown decision why-8-99") {
		t.Errorf("record --supersedes unknown id error = %v", err)
	}
}

func TestRecordCollisionWritesNothing(t *testing.T) {
	tests := []struct {
		name  string
		stdin string
		args  []string
	}{
		// decisions/why-7-2.md holds another id, so the plan's second id collides on disk.
		{"from plan", "", []string{"record", "--from-plan", "plan.md"}},
		{"stdin id recorded", "---\nid: other-1\nticket: why-7\n---\n## Ruling\nR.\n", []string{"record"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if out, err := exec.Command("git", "init", "-q", "-b", "main", root).CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}
			t.Setenv("TIX_HOOK", "")
			dir := filepath.Join(root, "decisions")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "plan.md"), []byte(testPlan), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "why-7-2.md"), []byte("---\nid: other-1\nticket: other\n---\n## Ruling\nR.\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)

			cmd := newRootCmd()
			cmd.SetArgs(tt.args)
			cmd.SetIn(strings.NewReader(tt.stdin))
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "already exists") {
				t.Errorf("why %v error = %v, want already exists", tt.args, err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 {
				t.Errorf("decisions/ has %d files, want only the pre-existing one", len(entries))
			}
		})
	}
}

const ruledPlan = `---
ticket: ce-43
concepts: [cli]
decisions:
  - q: "Ruled?"
    recommend: "a"
    ruled_by: brian
  - q: "Unruled?"
    recommend: "b"
---
`

func TestRecordFromPlanProvenance(t *testing.T) {
	tests := []struct {
		name     string
		stdin    string
		args     []string
		want     map[string]string // decision id -> decided_by
		receipts int               // lines in .git/why/approved
		wantErr  string
	}{
		{
			name:     "ci from stdin with ticket override",
			stdin:    ruledPlan,
			args:     []string{"record", "--from-plan", "-", "--ticket", "W-1", "--provenance=ci"},
			want:     map[string]string{"W-1-1": decision.Human, "W-1-2": decision.AgentApproved},
			receipts: 2,
		},
		{
			name: "ticket override without opt-in keeps --decided-by, downgraded",
			args: []string{"record", "--from-plan", "plan.md", "--ticket", "W-1", "--decided-by", "human"},
			want: map[string]string{"W-1-1": decision.Agent, "W-1-2": decision.Agent},
		},
		{
			name: "plan ticket without override",
			args: []string{"record", "--from-plan", "plan.md"},
			want: map[string]string{"ce-43-1": decision.Agent, "ce-43-2": decision.Agent},
		},
		{
			name:    "ci rejects --decided-by on --from-plan",
			args:    []string{"record", "--from-plan", "plan.md", "--provenance=ci", "--decided-by", "agent"},
			wantErr: "--decided-by does not apply",
		},
		{
			name:    "unknown provenance",
			args:    []string{"record", "--ticket", "x-1", "--ruling", "R.", "--provenance=trust-me"},
			wantErr: "invalid --provenance",
		},
		{
			name:    "stdin plan without ticket",
			stdin:   "---\ndecisions:\n  - q: Q?\n    recommend: a\n---\n",
			args:    []string{"record", "--from-plan", "-"},
			wantErr: "no ticket",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if out, err := exec.Command("git", "init", "-q", "-b", "main", root).CombinedOutput(); err != nil {
				t.Fatalf("git init: %v\n%s", err, out)
			}
			t.Setenv("TIX_HOOK", "")
			if err := os.WriteFile(filepath.Join(root, "plan.md"), []byte(ruledPlan), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)

			if tt.wantErr != "" {
				cmd := newRootCmd()
				cmd.SetArgs(tt.args)
				cmd.SetIn(strings.NewReader(tt.stdin))
				cmd.SetOut(&bytes.Buffer{})
				cmd.SetErr(&bytes.Buffer{})
				if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("why %v error = %v, want %q", tt.args, err, tt.wantErr)
				}
				return
			}
			run(t, tt.stdin, tt.args...)
			ds, err := decision.Load(filepath.Join(root, "decisions"))
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]string{}
			for _, d := range ds {
				got[d.ID] = d.DecidedBy
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("recorded %v, want %v", got, tt.want)
			}
			receipts, err := os.ReadFile(filepath.Join(root, ".git", "why", "approved"))
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			if n := strings.Count(string(receipts), "\n"); n != tt.receipts {
				t.Errorf("receipts = %d, want %d", n, tt.receipts)
			}
			if out := run(t, tt.stdin, tt.args...); out != "" {
				t.Errorf("re-recording wrote %q, want nothing", out)
			}
		})
	}
}
