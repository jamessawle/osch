package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/jamessawle/osch/internal/source"
	"github.com/jamessawle/osch/internal/update"
	"github.com/spf13/cobra"
)

// errUpdateBatchFailed signals a non-zero exit without an additional stderr
// message — per-schema outcomes have already been rendered to stdout.
var errUpdateBatchFailed = errors.New("")

const shortSHALen = 7

func newUpdateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "update [schema]",
		Short: "Refresh one or all installed schemas to upstream default-branch HEAD",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			ctx := cmd.Context()

			var results []update.Result
			if len(args) == 1 {
				r, _ := update.Update(ctx, updateClientFor, ".", args[0], force)
				results = []update.Result{r}
			} else {
				rs, err := update.UpdateAll(ctx, updateClientFor, ".", force)
				if err != nil {
					return err
				}
				if len(rs) == 0 {
					_, err := fmt.Fprintln(out, "no tracked schemas to update")
					return err
				}
				results = rs
			}

			anyBad := false
			for i, r := range results {
				if i > 0 {
					if _, err := fmt.Fprintln(out); err != nil {
						return err
					}
				}
				if err := renderResult(out, r); err != nil {
					return err
				}
				if r.Status == update.StatusRefused || r.Status == update.StatusFailed {
					anyBad = true
				}
			}
			if anyBad {
				return errUpdateBatchFailed
			}
			return nil
		},
	}
	c.Flags().Bool("force", false, "overwrite locally modified files")
	return c
}

// renderResult prints one schema section: schema name on its own line, body
// indented two spaces. Callers separate sections with a blank line.
func renderResult(out io.Writer, r update.Result) error {
	if _, err := fmt.Fprintln(out, r.Name); err != nil {
		return err
	}
	switch r.Status {
	case update.StatusUpToDate:
		_, err := fmt.Fprintf(out, "  up to date at %s\n", shortSHA(r.OldSHA))
		return err
	case update.StatusUpdated:
		if _, err := fmt.Fprintf(out, "  updated %s → %s\n", shortSHA(r.OldSHA), shortSHA(r.NewSHA)); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, "  snapshot saved to %s\n", r.SnapshotPath)
		return err
	case update.StatusRefused:
		if _, err := fmt.Fprintln(out, "  refused: local modifications"); err != nil {
			return err
		}
		for _, o := range r.Offenders {
			if _, err := fmt.Fprintf(out, "    %s\n", o); err != nil {
				return err
			}
		}
		return nil
	case update.StatusFailed:
		msg := ""
		if r.Err != nil {
			msg = r.Err.Error()
		}
		_, err := fmt.Fprintf(out, "  failed: %s\n", msg)
		return err
	}
	return nil
}

func shortSHA(sha string) string {
	if len(sha) <= shortSHALen {
		return sha
	}
	return sha[:shortSHALen]
}

// updateClientFor wraps the package-level clientFactory so update.Update can
// dispatch by provider without importing internal/github directly.
func updateClientFor(ref source.Ref) (source.Client, error) {
	return clientFactory(ref.Provider)
}
