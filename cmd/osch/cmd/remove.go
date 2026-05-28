package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

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
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", name)
			return err
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	return cmd
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
