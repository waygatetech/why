// Command why records and queries decisions: what was ruled, by whom, and
// what it supersedes.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var verbose bool
	cmd := &cobra.Command{
		Use:          "why",
		Short:        "Record and query decisions",
		Version:      version(),
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			level := slog.LevelInfo
			if verbose {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{Level: level})))
		},
	}
	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable debug logging")
	cmd.SetVersionTemplate(fmt.Sprintf("why %s\n", cmd.Version))
	cmd.AddCommand(newRecordCmd(), newListCmd(), newShowCmd(), newCheckCmd())
	return cmd
}

// version reports the module version stamped by `go install pkg@version`,
// or "(devel)" for local builds.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "(devel)"
	}
	return info.Main.Version
}
