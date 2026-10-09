package provenance

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/waygatetech/why/internal/decision"
)

func TestConfirm(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{{"y\n", true}, {"YES\n", true}, {"n\n", false}, {"\n", false}, {"", false}}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			rw := struct {
				io.Reader
				io.Writer
			}{strings.NewReader(tt.in), &bytes.Buffer{}}
			got, err := Confirm(rw, []decision.Decision{{ID: "why-1-1", DecidedBy: decision.Human}})
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("Confirm(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func write(t *testing.T, root string, d decision.Decision) string {
	t.Helper()
	data, err := d.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "decisions", d.ID+".md")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCheck(t *testing.T) {
	committed := decision.Decision{ID: "why-1-1", Ticket: "why-1", Date: "2026-10-08", DecidedBy: decision.AgentApproved, Ruling: "A."}
	tests := []struct {
		name   string
		change func(t *testing.T, root, gitDir string)
		want   string // substring of the only violation; "" for none
	}{
		{"untouched", func(t *testing.T, root, gitDir string) {}, ""},
		{"why edited", func(t *testing.T, root, gitDir string) {
			d := committed
			d.Why = "More reasons."
			write(t, root, d)
		}, ""},
		{"ruling edited", func(t *testing.T, root, gitDir string) {
			d := committed
			d.Ruling = "B."
			write(t, root, d)
		}, "ruling changed"},
		{"decided_by edited", func(t *testing.T, root, gitDir string) {
			d := committed
			d.DecidedBy = decision.Human
			write(t, root, d)
		}, "decided_by changed"},
		{"new agent", func(t *testing.T, root, gitDir string) {
			write(t, root, decision.Decision{ID: "why-2-1", DecidedBy: decision.Agent, Ruling: "C."})
		}, ""},
		{"new human without receipt", func(t *testing.T, root, gitDir string) {
			write(t, root, decision.Decision{ID: "why-2-1", DecidedBy: decision.Human, Ruling: "C."})
		}, "without a confirmed receipt"},
		{"new human with receipt", func(t *testing.T, root, gitDir string) {
			p := write(t, root, decision.Decision{ID: "why-2-1", DecidedBy: decision.Human, Ruling: "C."})
			if err := AddReceipt(gitDir, "why-2-1", p); err != nil {
				t.Fatal(err)
			}
		}, ""},
		{"new human edited after receipt", func(t *testing.T, root, gitDir string) {
			d := decision.Decision{ID: "why-2-1", DecidedBy: decision.Human, Ruling: "C."}
			p := write(t, root, d)
			if err := AddReceipt(gitDir, d.ID, p); err != nil {
				t.Fatal(err)
			}
			d.Ruling = "D."
			write(t, root, d)
		}, "without a confirmed receipt"},
		{"agent supersedes human", func(t *testing.T, root, gitDir string) {
			write(t, root, decision.Decision{ID: "why-2-1", DecidedBy: decision.Agent, Ruling: "C.", Supersedes: []string{"why-1-1"}})
		}, "supersedes why-1-1"},
		{"human supersedes human", func(t *testing.T, root, gitDir string) {
			p := write(t, root, decision.Decision{ID: "why-2-1", DecidedBy: decision.Human, Ruling: "C.", Supersedes: []string{"why-1-1"}})
			if err := AddReceipt(gitDir, "why-2-1", p); err != nil {
				t.Fatal(err)
			}
		}, ""},
		{"agent supersedes agent", func(t *testing.T, root, gitDir string) {
			write(t, root, decision.Decision{ID: "why-2-1", DecidedBy: decision.Agent, Ruling: "C."})
			write(t, root, decision.Decision{ID: "why-2-2", DecidedBy: decision.Agent, Ruling: "D.", Supersedes: []string{"why-2-1"}})
		}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			gitRun(t, root, "init", "-q", "-b", "main")
			if err := os.Mkdir(filepath.Join(root, "decisions"), 0o755); err != nil {
				t.Fatal(err)
			}
			write(t, root, committed)
			gitRun(t, root, "add", ".")
			gitRun(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "base")
			gitRun(t, root, "checkout", "-qb", "work")

			ctx := context.Background()
			gitDir, err := GitDir(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			tt.change(t, root, gitDir)
			got, err := Check(ctx, root, "main")
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case tt.want == "" && len(got) != 0:
				t.Errorf("Check = %q, want no violations", got)
			case tt.want != "" && (len(got) != 1 || !strings.Contains(got[0], tt.want)):
				t.Errorf("Check = %q, want one containing %q", got, tt.want)
			}
		})
	}
}
