// Package provenance keeps decided_by honest: human provenance needs an
// interactive confirmation, which leaves a receipt that Check accepts.
//
// This is soft against an agent with shell access that forges a receipt on
// purpose; it stops accidental upgrades.
package provenance

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/waygatetech/why/internal/decision"
)

// Confirm lists ds on tty and asks the human to vouch for their provenance.
func Confirm(tty io.ReadWriter, ds []decision.Decision) (bool, error) {
	for _, d := range ds {
		fmt.Fprintf(tty, "%s [%s]\n  Q: %s\n  R: %s\n", d.ID, d.DecidedBy, d.Question, d.Ruling)
	}
	fmt.Fprint(tty, "Record these with the provenance shown? [y/N] ")
	line, err := bufio.NewReader(tty).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("reading confirmation: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// GitDir returns the absolute git common dir for the repo at root, shared by
// all worktrees and never part of a diff.
func GitDir(ctx context.Context, root string) (string, error) {
	return git(ctx, root, "rev-parse", "--path-format=absolute", "--git-common-dir")
}

func receiptsPath(gitDir string) string {
	return filepath.Join(gitDir, "why", "approved")
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// AddReceipt records that the decision file at path, as it is now, was
// confirmed by a human.
func AddReceipt(gitDir, id, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	p := receiptsPath(gitDir)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(p), err)
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", p, err)
	}
	if _, err := fmt.Fprintf(f, "%s %s\n", id, hash(data)); err != nil {
		f.Close()
		return fmt.Errorf("writing %s: %w", p, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", p, err)
	}
	return nil
}

// receipts returns the set of confirmed file hashes.
func receipts(gitDir string) (map[string]bool, error) {
	data, err := os.ReadFile(receiptsPath(gitDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading receipts: %w", err)
	}
	out := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if f := strings.Fields(line); len(f) == 2 {
			out[f[1]] = true
		}
	}
	return out, nil
}

// Check compares root/decisions/*.md with the merge base of base and HEAD.
// It returns one violation per existing decision whose decided_by or ruling
// changed, per new decision claiming human provenance without a receipt
// for its exact content, and per non-human decision superseding a human one.
func Check(ctx context.Context, root, base string) ([]string, error) {
	mb, err := git(ctx, root, "merge-base", base, "HEAD")
	if err != nil {
		return nil, err
	}
	listed, err := git(ctx, root, "ls-tree", "-r", "--name-only", mb, "--", "decisions/")
	if err != nil {
		return nil, err
	}
	inBase := map[string]bool{}
	for _, p := range strings.Fields(listed) {
		inBase[p] = true
	}
	gitDir, err := GitDir(ctx, root)
	if err != nil {
		return nil, err
	}
	ok, err := receipts(gitDir)
	if err != nil {
		return nil, err
	}

	paths, err := filepath.Glob(filepath.Join(root, "decisions", "*.md"))
	if err != nil {
		return nil, fmt.Errorf("listing decisions: %w", err)
	}
	var violations []string
	parsed := map[string]decision.Decision{}
	for _, p := range paths {
		rel := "decisions/" + filepath.Base(p)
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", p, err)
		}
		cur, err := decision.Parse(bytes.NewReader(data))
		if err != nil {
			violations = append(violations, fmt.Sprintf("%s: %v", rel, err))
			continue
		}
		parsed[cur.ID] = cur
		if !inBase[rel] {
			if decision.IsHuman(cur.DecidedBy) && !ok[hash(data)] {
				violations = append(violations, fmt.Sprintf("%s: new decision claims decided_by %s without a confirmed receipt", rel, cur.DecidedBy))
			}
			continue
		}
		old, err := git(ctx, root, "show", mb+":"+rel)
		if err != nil {
			return nil, err
		}
		prev, err := decision.Parse(strings.NewReader(old))
		if err != nil {
			return nil, fmt.Errorf("%s at %s: %w", rel, base, err)
		}
		if prev.DecidedBy != cur.DecidedBy {
			violations = append(violations, fmt.Sprintf("%s: decided_by changed from %s to %s", rel, prev.DecidedBy, cur.DecidedBy))
		}
		if prev.Ruling != cur.Ruling {
			violations = append(violations, fmt.Sprintf("%s: ruling changed", rel))
		}
	}
	for _, cur := range parsed {
		if decision.IsHuman(cur.DecidedBy) {
			continue
		}
		for _, id := range cur.Supersedes {
			if old, ok := parsed[id]; ok && decision.IsHuman(old.DecidedBy) {
				violations = append(violations, fmt.Sprintf("decisions/%s.md: decided_by %s supersedes %s, a %s ruling", cur.ID, cur.DecidedBy, id, old.DecidedBy))
			}
		}
	}
	slices.Sort(violations)
	return violations, nil
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}
