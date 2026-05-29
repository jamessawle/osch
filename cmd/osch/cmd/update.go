package cmd

import (
	"github.com/jamessawle/osch/internal/source"
	"github.com/jamessawle/osch/internal/update"
	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <schema>",
		Short: "Refresh an installed schema to the upstream default branch HEAD",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return update.Update(cmd.Context(), updateClientFor, ".", args[0], cmd.OutOrStdout())
		},
	}
}

// updateClientFor wraps the package-level clientFactory so update.Update can
// dispatch by provider without importing internal/github directly.
func updateClientFor(ref source.Ref) (source.Client, error) {
	return clientFactory(ref.Provider)
}
