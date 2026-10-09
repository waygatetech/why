package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/spf13/cobra"
	"github.com/waygatetech/why/internal/decision"
	"github.com/waygatetech/why/internal/plan"
	"github.com/waygatetech/why/internal/provenance"
	"github.com/waygatetech/why/internal/repo"
)

func newRecordCmd() *cobra.Command {
	var fromPlan string
	var d decision.Decision
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record decisions from an approved plan, flags, or a decision file on stdin",
		Long: `Record decisions under <repo>/decisions/.

--from-plan writes one decision per entry in the plan's decisions; the
entry's recommend is the ruling. Entries already recorded for the ticket
(same question) are skipped.

Otherwise one decision is taken from flags, or, when --ruling is not set,
from a decision file on stdin.

decided_by human or agent-proposed-human-approved is written only when run
from the tix approve hook (TIX_HOOK=approve); anywhere else it is recorded as
agent. The human gesture is the Claude Code permission prompt, so configure:
  ask:  Bash(tix approve:*)
  deny: Bash(why record:*--decided-by human*),
        Bash(why record:*--decided-by agent-proposed-human-approved*)`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := decisionsDir()
			if err != nil {
				return err
			}
			existing, err := decision.Load(dir)
			if err != nil {
				return err
			}
			var todo []decision.Decision
			if fromPlan != "" {
				todo, err = fromPlanFile(fromPlan, existing, d.DecidedBy)
			} else {
				todo, err = single(cmd, d)
			}
			if err != nil {
				return err
			}
			today := time.Now().Format(time.DateOnly)
			ids := slices.Clone(existing)
			for i := range todo {
				d := &todo[i]
				if d.Ticket == "" && d.ID == "" {
					return errors.New("recording decision: --ticket is required")
				}
				if d.ID == "" {
					d.ID = decision.NextID(ids, d.Ticket)
				}
				if d.Date == "" {
					d.Date = today
				}
				ids = append(ids, *d)
			}
			for _, d := range todo {
				for _, id := range d.Supersedes {
					if !slices.ContainsFunc(ids, func(e decision.Decision) bool { return e.ID == id }) {
						return fmt.Errorf("recording decision %s: supersedes unknown decision %s", d.ID, id)
					}
				}
			}
			confirmed := approvedByHuman(cmd, todo)
			var gitDir string
			if confirmed {
				if gitDir, err = provenance.GitDir(cmd.Context(), filepath.Dir(dir)); err != nil {
					return err
				}
			}
			for _, d := range todo {
				path, err := decision.Write(dir, d)
				if err != nil {
					return err
				}
				if confirmed && decision.IsHuman(d.DecidedBy) {
					if err := provenance.AddReceipt(gitDir, d.ID, path); err != nil {
						return err
					}
				}
				fmt.Fprintln(cmd.OutOrStdout(), path)
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&fromPlan, "from-plan", "", "tix plan file whose decisions to record")
	f.StringVar(&d.Ticket, "ticket", "", "ticket the decision belongs to")
	f.StringSliceVar(&d.Concepts, "concept", nil, "concept the decision applies to (repeatable)")
	f.StringVar(&d.Question, "question", "", "question that was decided")
	f.StringVar(&d.Ruling, "ruling", "", "the ruling (omit to read a decision file from stdin)")
	f.StringVar(&d.Why, "why", "", "reasoning behind the ruling")
	f.StringSliceVar(&d.Supersedes, "supersedes", nil, "id of a decision this one replaces (repeatable)")
	f.StringVar(&d.DecidedBy, "decided-by", decision.Agent, "who made the ruling: human, agent-proposed-human-approved or agent; the first two are kept only under the tix approve hook, else agent is written")
	cmd.MarkFlagsMutuallyExclusive("from-plan", "ruling")
	return cmd
}

// fromPlanFile maps an approved plan's decisions to new Decisions, skipping
// ones already recorded so re-approving a plan is idempotent.
func fromPlanFile(path string, existing []decision.Decision, decidedBy string) ([]decision.Decision, error) {
	p, err := plan.Read(path)
	if err != nil {
		return nil, err
	}
	recorded := map[string]bool{}
	for _, d := range existing {
		if d.Ticket == p.Ticket {
			recorded[d.Question] = true
		}
	}
	concepts := append(append([]string{}, p.Concepts...), p.NewConcepts...)
	var out []decision.Decision
	for _, e := range p.Decisions {
		if recorded[e.Q] {
			continue
		}
		out = append(out, decision.Decision{
			Ticket: p.Ticket, Concepts: concepts, DecidedBy: decidedBy,
			Question: e.Q, Ruling: e.Recommend, Why: e.Why, Supersedes: e.Supersedes,
		})
	}
	return out, nil
}

// single returns the decision given by flags, or read from stdin when no
// ruling flag was set. Flags fill fields the stdin file leaves empty.
func single(cmd *cobra.Command, flags decision.Decision) ([]decision.Decision, error) {
	if flags.Ruling != "" {
		return []decision.Decision{flags}, nil
	}
	d, err := decision.Parse(cmd.InOrStdin())
	if err != nil {
		return nil, fmt.Errorf("reading decision from stdin: %w", err)
	}
	if d.Ruling == "" {
		return nil, errors.New("recording decision: no ruling (pass --ruling or a decision file on stdin)")
	}
	if d.Ticket == "" {
		d.Ticket = flags.Ticket
	}
	if d.Concepts == nil {
		d.Concepts = flags.Concepts
	}
	if d.DecidedBy == "" {
		d.DecidedBy = flags.DecidedBy
	}
	if d.Supersedes == nil {
		d.Supersedes = flags.Supersedes
	}
	return []decision.Decision{d}, nil
}

// approvedByHuman reports whether why runs inside the tix approve hook, the
// only place human provenance may be written; the human gesture is the
// harness permission prompt on tix approve. Elsewhere every human decision
// is downgraded to agent. An agent can set TIX_HOOK itself; this stops
// accidental upgrades, not determined ones.
func approvedByHuman(cmd *cobra.Command, ds []decision.Decision) bool {
	if os.Getenv("TIX_HOOK") == "approve" {
		return true
	}
	for i := range ds {
		if decision.IsHuman(ds[i].DecidedBy) {
			fmt.Fprintf(cmd.ErrOrStderr(), "why: %s: human provenance is only written from the tix approve hook; recording as %s\n", ds[i].ID, decision.Agent)
			ds[i].DecidedBy = decision.Agent
		}
	}
	return false
}

// decisionsDir is <repo root>/decisions for the working directory.
func decisionsDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	root, err := repo.Root(wd)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "decisions"), nil
}
