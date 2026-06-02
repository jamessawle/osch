package cmd

import (
	"github.com/jamessawle/osch/internal/list"
	"github.com/jamessawle/osch/internal/source"
	"github.com/spf13/cobra"
)

// listClientFactory is the seam tests use to inject a fake source.Client for
// drift detection without touching the network. Production code resolves the
// GitHub client via clientFactory; this exists separately because the list
// command picks one client up-front (before knowing per-row providers) and
// today only GitHub is supported.
var listClientFactory = func() (source.Client, error) {
	return clientFactory(source.ProviderGitHub)
}

func newListCmd() *cobra.Command {
	var offline bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed schemas",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var client source.Client
			if !offline {
				c, err := listClientFactory()
				if err != nil {
					return err
				}
				client = c
			}
			return list.List(cmd.Context(), ".", cmd.OutOrStdout(), client, offline)
		},
	}
	cmd.Flags().BoolVar(&offline, "offline", false, "Skip upstream lookups; UPSTREAM reads \"unknown\" for every tracked row")
	return cmd
}
