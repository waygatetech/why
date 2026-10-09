// Package repo locates the repository that decisions belong to.
package repo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNotFound means no ancestor of the start directory contains .git.
var ErrNotFound = errors.New("not inside a git repository")

// Root returns the nearest ancestor of start (inclusive) that contains .git.
// .git may be a directory or, in worktrees and submodules, a file.
func Root(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", start, err)
	}
	for {
		_, err := os.Stat(filepath.Join(dir, ".git"))
		if err == nil {
			return dir, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("checking %s for .git: %w", dir, err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("searching from %s: %w", start, ErrNotFound)
		}
		dir = parent
	}
}
