package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jamessawle/osch/internal/openspec"
	"github.com/jamessawle/osch/internal/remove"
	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <schema>",
		Short: "Delete an installed schema folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := remove.ValidateName(name); err != nil {
				return err
			}
			confirmed, err := confirmRemoval(yes, name, cmd.InOrStdin(), cmd.OutOrStdout())
			if err != nil {
				return err
			}
			if !confirmed {
				return nil
			}
			if err := remove.Remove(".", name); err != nil {
				return err
			}
			reset, resetErr := resetActiveSchema(".", name)
			out := cmd.OutOrStdout()
			if resetErr != nil {
				// Folder is already gone; surface the rewrite failure but do
				// not turn the whole command into a failure — the user's
				// primary goal (deleting the folder) is done.
				if _, err := fmt.Fprintf(out, "removed %s\n", name); err != nil {
					return err
				}
				_, err := fmt.Fprintf(out, "warning: failed to reset active schema in openspec/%s: %v\n", openspec.Filename, resetErr)
				return err
			}
			if reset {
				_, err := fmt.Fprintf(out, "removed %s (active schema reset to %s)\n", name, openspec.DefaultSchema)
				return err
			}
			_, err = fmt.Fprintf(out, "removed %s\n", name)
			return err
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	return cmd
}

// resetActiveSchema points openspec/config.yaml at openspec.DefaultSchema when
// its top-level `schema` key matches the just-removed name. Missing or
// unparseable config files are treated as "no active schema" — the caller's
// folder delete has already happened and must not be undone by a follow-up
// edit failure. Returns whether the file was rewritten.
func resetActiveSchema(workingDir, removed string) (bool, error) {
	cfgPath := openspec.Path(workingDir)
	current, exists, err := openspec.ReadSchema(cfgPath)
	if err != nil {
		return false, err
	}
	if !exists || current != removed {
		return false, nil
	}
	if err := openspec.WriteSchema(cfgPath, openspec.DefaultSchema); err != nil {
		return false, err
	}
	return true, nil
}

// confirmRemoval returns whether the caller should proceed with deletion.
// --yes bypasses the prompt; non-TTY stdin without --yes aborts rather than
// silently proceeding (matches the safety contract of `add`'s activation).
func confirmRemoval(yes bool, name string, stdin io.Reader, stdout io.Writer) (bool, error) {
	if yes {
		return true, nil
	}
	if !stdinIsTTY() {
		return false, errors.New("refusing to remove without confirmation: stdin is not a TTY; re-run with --yes")
	}
	if _, err := fmt.Fprintf(stdout, "Remove schema %q? [y/N] ", name); err != nil {
		return false, err
	}
	reader := bufio.NewReader(stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("reading prompt response: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
