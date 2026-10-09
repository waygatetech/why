package repo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRoot(t *testing.T) {
	tests := []struct {
		name    string
		gitFile bool // .git as a file (worktree) instead of a dir
		noGit   bool
		start   string // relative to the temp root
	}{
		{name: "at root", start: "."},
		{name: "in nested dir", start: "a/b/c"},
		{name: "worktree .git file", gitFile: true, start: "a"},
		{name: "no repo", noGit: true, start: "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			start := filepath.Join(root, tt.start)
			if err := os.MkdirAll(start, 0o755); err != nil {
				t.Fatal(err)
			}
			switch {
			case tt.noGit:
			case tt.gitFile:
				if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: elsewhere\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			default:
				if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			got, err := Root(start)
			if tt.noGit {
				// The temp dir may itself sit under a repo; only assert we didn't stop at root.
				if err == nil && got == root {
					t.Fatalf("Root(%s) = %s, want not found", start, got)
				}
				if err != nil && !errors.Is(err, ErrNotFound) {
					t.Fatalf("Root(%s) error = %v, want ErrNotFound", start, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Root(%s): %v", start, err)
			}
			if got != root {
				t.Errorf("Root(%s) = %s, want %s", start, got, root)
			}
		})
	}
}
