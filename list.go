package main

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/waygatetech/why/internal/decision"
)

func newListCmd() *cobra.Command {
	var concepts []string
	var format string
	var all bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Print active (non-superseded) decisions",
		Long:  "Print active decisions, optionally only those tagged with any of the given concepts. --format md suits vet --context.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "md" {
				return fmt.Errorf("unknown --format %q (want text or md)", format)
			}
			dir, err := decisionsDir()
			if err != nil {
				return err
			}
			ds, err := decision.Load(dir)
			if err != nil {
				return err
			}
			if !all {
				ds = decision.Active(ds)
			}
			var out []decision.Decision
			for _, d := range ds {
				if len(concepts) == 0 || slices.ContainsFunc(d.Concepts, func(c string) bool { return slices.Contains(concepts, c) }) {
					out = append(out, d)
				}
			}
			ds = out
			slices.SortFunc(ds, func(a, b decision.Decision) int {
				return cmp.Or(cmp.Compare(a.Date, b.Date), cmp.Compare(a.ID, b.ID))
			})
			if format == "md" {
				return writeMarkdown(cmd.OutOrStdout(), ds)
			}
			return writeText(cmd.OutOrStdout(), ds)
		},
	}
	cmd.Flags().StringSliceVar(&concepts, "concept", nil, "only decisions tagged with this concept (repeatable)")
	cmd.Flags().BoolVar(&all, "all", false, "include superseded decisions")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or md")
	return cmd
}

func writeText(w io.Writer, ds []decision.Decision) error {
	for _, d := range ds {
		ruling, _, _ := strings.Cut(d.Ruling, "\n")
		if _, err := fmt.Fprintf(w, "%s  [%s]  %s\n", d.ID, strings.Join(d.Concepts, ","), ruling); err != nil {
			return fmt.Errorf("writing list: %w", err)
		}
	}
	return nil
}

func writeMarkdown(w io.Writer, ds []decision.Decision) error {
	for _, d := range ds {
		if _, err := fmt.Fprintf(w, "## %s: %s\n\n**Ruling:** %s\n\n**Why:** %s\n\n", d.ID, d.Question, d.Ruling, d.Why); err != nil {
			return fmt.Errorf("writing list: %w", err)
		}
	}
	return nil
}
