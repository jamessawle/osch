// Package cmd assembles the osch Cobra command tree. main.go is a thin
// entrypoint that calls Execute; each subcommand lives in its own file.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewRootCmd returns a fresh root command with every subcommand attached.
// Tests build a new tree per invocation so flag state does not leak between
// cases.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "osch",
		Short:         "Manage OpenSpec schemas across repositories",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCmd())
	root.AddCommand(newAddCmd())
	return root
}

// Execute runs the CLI. It mirrors main()'s old contract: any error is printed
// to stderr and exits non-zero.
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
