package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/waygatetech/why/internal/provenance"
)

func newCheckCmd() *cobra.Command {
	var base string
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Fail if the diff forges or rewrites decision provenance",
		Long: `Compare decisions/ with the merge base of --base and HEAD, including
uncommitted and untracked files. Fails when an existing decision's decided_by
or ruling changed, or a new decision claims human provenance without having
been recorded by why record under the tix approve hook. Meant for the tix
done hook.

Receipts live in .git/why/approved and are never pushed, so run check in the
clone that recorded the decisions. On CI or another clone, every human
decision on the branch is reported as unconfirmed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := decisionsDir()
			if err != nil {
				return err
			}
			violations, err := provenance.Check(cmd.Context(), filepath.Dir(dir), base)
			if err != nil {
				return err
			}
			for _, v := range violations {
				fmt.Fprintln(cmd.OutOrStdout(), v)
			}
			if len(violations) > 0 {
				return fmt.Errorf("%d decision provenance violation(s)", len(violations))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&base, "base", "main", "branch whose merge base the diff starts from")
	return cmd
}
