package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// Build-time identity. main wires these from its own ldflags-injected vars so
// the -X main.{version,commit,date} pattern in .goreleaser.yaml keeps working.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	var jsonOut bool
	c := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return printVersion(cmd.OutOrStdout(), jsonOut)
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, "output JSON")
	return c
}

func printVersion(w io.Writer, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(w).Encode(map[string]string{
			"version": Version,
			"commit":  Commit,
			"date":    Date,
		})
	}
	_, err := fmt.Fprintf(w, "osch %s (commit %s, built %s)\n", Version, Commit, Date)
	return err
}
