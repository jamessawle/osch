package cmd

import (
	"fmt"

	"github.com/jamessawle/osch/internal/github"
	"github.com/jamessawle/osch/internal/install"
	"github.com/jamessawle/osch/internal/source"
	"github.com/spf13/cobra"
)

// clientFactory is the seam tests use to inject a fake source.Client without
// going through the github package. Production code leaves it at its default
// which dispatches by provider.
var clientFactory = clientForProvider

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <owner>/<repo>",
		Short: "Install a schema from an upstream repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := source.ParseRef(args[0])
			if err != nil {
				return err
			}
			client, err := clientFactory(ref.Provider)
			if err != nil {
				return err
			}
			return install.Add(cmd.Context(), client, ref, ".", cmd.OutOrStdout())
		},
	}
}

// clientForProvider returns the concrete source client for a provider. This switch is
// the single place that knows about concrete provider implementations; adding a
// second provider (per ADR 0004) means adding a branch here.
func clientForProvider(provider string) (source.Client, error) {
	switch provider {
	case source.ProviderGitHub:
		return github.NewClient(), nil
	default:
		return nil, fmt.Errorf("unsupported source provider %q", provider)
	}
}
