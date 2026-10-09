package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/waygatetech/why/internal/decision"
)

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Print a decision and its supersedes chain in both directions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := decisionsDir()
			if err != nil {
				return err
			}
			ds, err := decision.Load(dir)
			if err != nil {
				return err
			}
			i := slices.IndexFunc(ds, func(d decision.Decision) bool { return d.ID == args[0] })
			if i < 0 {
				return fmt.Errorf("no decision %s", args[0])
			}
			data, err := ds[i].Marshal()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if _, err := w.Write(data); err != nil {
				return fmt.Errorf("writing decision: %w", err)
			}
			older, newer := decision.Chain(ds, args[0])
			if len(older)+len(newer) > 0 {
				fmt.Fprintln(w)
			}
			for _, part := range []struct {
				label string
				ds    []decision.Decision
			}{{"supersedes", older}, {"superseded by", newer}} {
				for _, d := range part.ds {
					ruling, _, _ := strings.Cut(d.Ruling, "\n")
					if _, err := fmt.Fprintf(w, "%s: %s  [%s]  %s\n", part.label, d.ID, d.DecidedBy, ruling); err != nil {
						return fmt.Errorf("writing chain: %w", err)
					}
				}
			}
			return nil
		},
	}
}
