package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jamessawle/osch/internal/openspec"
	"github.com/jamessawle/osch/internal/remove"
	"github.com/spf13/cobra"
)

// promptRetries caps the number of times the post-removal selection prompt
// will re-ask after invalid input before falling back to the default schema.
const promptRetries = 3

func newRemoveCmd() *cobra.Command {
	var yes, noActivate bool
	var activate string
	cmd := &cobra.Command{
		Use:   "remove <schema>",
		Short: "Delete an installed schema folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if activate != "" && noActivate {
				return errors.New("--activate and --no-activate cannot be used together")
			}
			name := args[0]
			if err := remove.ValidateName(name); err != nil {
				return err
			}
			// Validate --activate before deleting so a typo cannot cost the
			// user their schema folder (per the brief's recommendation).
			if activate != "" {
				if activate == name {
					return fmt.Errorf("cannot --activate %q: it is the schema being removed", activate)
				}
				if activate != openspec.DefaultSchema {
					installed, err := installedSchemas(".")
					if err != nil {
						return err
					}
					if !containsString(installed, activate) {
						return fmt.Errorf("schema %q is not installed; cannot activate", activate)
					}
				}
			}
			in := bufio.NewReader(cmd.InOrStdin())
			confirmed, err := confirmRemoval(yes, name, in, cmd.OutOrStdout())
			if err != nil {
				return err
			}
			if !confirmed {
				return nil
			}
			if err := remove.Remove(".", name); err != nil {
				return err
			}
			chosen, wrote, resetErr := resolveActiveSchema(".", name, activate, noActivate, in, cmd.OutOrStdout())
			out := cmd.OutOrStdout()
			if resetErr != nil {
				if _, err := fmt.Fprintf(out, "removed %s\n", name); err != nil {
					return err
				}
				_, err := fmt.Fprintf(out, "warning: failed to reset active schema in openspec/%s: %v\n", openspec.Filename, resetErr)
				return err
			}
			if wrote {
				if chosen == openspec.DefaultSchema {
					_, err := fmt.Fprintf(out, "removed %s (active schema reset to %s)\n", name, chosen)
					return err
				}
				_, err := fmt.Fprintf(out, "removed %s (active schema set to %s)\n", name, chosen)
				return err
			}
			_, err = fmt.Fprintf(out, "removed %s\n", name)
			return err
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip the confirmation prompt")
	cmd.Flags().StringVar(&activate, "activate", "", "After removal, activate this schema without prompting")
	cmd.Flags().BoolVar(&noActivate, "no-activate", false, "After removal, fall back to spec-driven without prompting")
	return cmd
}

// resolveActiveSchema rewrites openspec/config.yaml's `schema` key when the
// just-removed folder was the active one. The chosen replacement comes from
// (in order): --activate, --no-activate or non-TTY (both → spec-driven),
// otherwise an interactive numbered prompt over the remaining installed
// schemas plus spec-driven. Returns the name written and whether the file
// was rewritten.
func resolveActiveSchema(workingDir, removed, activateFlag string, noActivate bool, in *bufio.Reader, stdout io.Writer) (string, bool, error) {
	cfgPath := openspec.Path(workingDir)
	current, exists, err := openspec.ReadSchema(cfgPath)
	if err != nil {
		return "", false, err
	}
	if !exists || current != removed {
		return "", false, nil
	}
	var target string
	switch {
	case activateFlag != "":
		target = activateFlag
	case noActivate, !stdinIsTTY():
		target = openspec.DefaultSchema
	default:
		installed, err := installedSchemas(workingDir)
		if err != nil {
			return "", false, err
		}
		candidates := buildCandidates(installed)
		if len(candidates) <= 1 {
			// Only spec-driven would be on the menu — skip the prompt and
			// fall back silently to preserve #39's behaviour.
			target = openspec.DefaultSchema
		} else {
			chosen, err := promptForActiveSchema(in, stdout, candidates)
			if err != nil {
				return "", false, err
			}
			target = chosen
		}
	}
	if err := openspec.WriteSchema(cfgPath, target); err != nil {
		return "", false, err
	}
	return target, true, nil
}

// confirmRemoval returns whether the caller should proceed with deletion.
// --yes bypasses the prompt; non-TTY stdin without --yes aborts rather than
// silently proceeding (matches the safety contract of `add`'s activation).
func confirmRemoval(yes bool, name string, in *bufio.Reader, stdout io.Writer) (bool, error) {
	if yes {
		return true, nil
	}
	if !stdinIsTTY() {
		return false, errors.New("refusing to remove without confirmation: stdin is not a TTY; re-run with --yes")
	}
	if _, err := fmt.Fprintf(stdout, "Remove schema %q? [y/N] ", name); err != nil {
		return false, err
	}
	line, err := in.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("reading prompt response: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// promptForActiveSchema shows a numbered menu of candidates and reads a
// selection (either a 1-based index or the literal schema name). Empty input
// or an EOF accepts the default (spec-driven). Invalid input re-prompts up
// to promptRetries times before falling back rather than looping forever.
func promptForActiveSchema(in *bufio.Reader, stdout io.Writer, candidates []string) (string, error) {
	for attempt := 0; attempt < promptRetries; attempt++ {
		if _, err := fmt.Fprintln(stdout, "Choose the new active schema:"); err != nil {
			return "", err
		}
		for i, c := range candidates {
			if _, err := fmt.Fprintf(stdout, "  %d) %s\n", i+1, c); err != nil {
				return "", err
			}
		}
		if _, err := fmt.Fprintf(stdout, "Selection [%s]: ", openspec.DefaultSchema); err != nil {
			return "", err
		}
		line, err := in.ReadString('\n')
		eof := errors.Is(err, io.EOF)
		if err != nil && !eof {
			return openspec.DefaultSchema, nil
		}
		answer := strings.TrimSpace(line)
		if answer == "" {
			return openspec.DefaultSchema, nil
		}
		if idx, perr := strconv.Atoi(answer); perr == nil {
			if idx >= 1 && idx <= len(candidates) {
				return candidates[idx-1], nil
			}
		} else {
			for _, c := range candidates {
				if c == answer {
					return c, nil
				}
			}
		}
		if _, err := fmt.Fprintf(stdout, "invalid selection %q\n", answer); err != nil {
			return "", err
		}
		if eof {
			return openspec.DefaultSchema, nil
		}
	}
	return openspec.DefaultSchema, nil
}

// installedSchemas returns the sorted list of directories under
// workingDir/openspec/schemas/. A missing directory is treated as an empty
// list so the caller can branch on len() without a special case.
func installedSchemas(workingDir string) ([]string, error) {
	dir := filepath.Join(workingDir, "openspec", "schemas")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// buildCandidates appends spec-driven to the installed list unless it is
// already present, preserving the installed list's order so the numbered
// menu is stable.
func buildCandidates(installed []string) []string {
	out := make([]string, 0, len(installed)+1)
	seen := make(map[string]bool)
	for _, n := range installed {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	if !seen[openspec.DefaultSchema] {
		out = append(out, openspec.DefaultSchema)
	}
	return out
}

func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
