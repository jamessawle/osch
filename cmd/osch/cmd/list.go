package cmd

import (
	"github.com/jamessawle/osch/internal/list"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed schemas",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return list.List(".", cmd.OutOrStdout())
		},
	}
}
