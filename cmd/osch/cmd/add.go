package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/jamessawle/osch/internal/github"
	"github.com/jamessawle/osch/internal/install"
	"github.com/jamessawle/osch/internal/openspec"
	"github.com/jamessawle/osch/internal/source"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// clientFactory is the seam tests use to inject a fake source.Client without
// going through the github package. Production code leaves it at its default
// which dispatches by provider.
var clientFactory = clientForProvider

// stdinIsTTY reports whether the process's standard input is connected to a
// terminal. It is a package-level var so tests can drive the activation flow
// deterministically without needing a real PTY.
var stdinIsTTY = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func newAddCmd() *cobra.Command {
	var activate, noActivate bool
	cmd := &cobra.Command{
		Use:   "add <owner>/<repo> [schema]",
		Short: "Install a schema from an upstream repository",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if activate && noActivate {
				return errors.New("--activate and --no-activate cannot be used together")
			}
			ref, err := source.ParseRef(args[0])
			if err != nil {
				return err
			}
			var selected string
			if len(args) == 2 {
				selected = args[1]
			}
			client, err := clientFactory(ref.Provider)
			if err != nil {
				return err
			}
			name, err := install.Add(cmd.Context(), client, ref, selected, ".", cmd.OutOrStdout())
			if err != nil {
				return err
			}
			mode := activateAuto
			switch {
			case activate:
				mode = activateYes
			case noActivate:
				mode = activateNo
			}
			return runActivation(".", name, mode, cmd.InOrStdin(), cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVar(&activate, "activate", false, "Activate the schema after install without prompting")
	cmd.Flags().BoolVar(&noActivate, "no-activate", false, "Skip the activation prompt and do not activate")
	return cmd
}

// activateMode encodes the user's intent for the post-install activation step.
// activateAuto means "prompt if interactive, otherwise stay silent" — the
// default that matches the non-TTY safety requirement in the brief.
type activateMode int

const (
	activateAuto activateMode = iota
	activateYes
	activateNo
)

func runActivation(workingDir, schemaName string, mode activateMode, stdin io.Reader, stdout io.Writer) error {
	if mode == activateNo {
		return nil
	}
	cfgPath := openspec.Path(workingDir)
	current, exists, err := openspec.ReadSchema(cfgPath)
	if err != nil {
		return err
	}
	if !exists {
		// Brief: skip activation with a clear message when the file is
		// absent. Stay silent in the auto+non-TTY case so unattended runs
		// don't print spurious output, but surface the skip when the user
		// (or interactive prompt) actually asked for activation.
		if mode == activateYes || stdinIsTTY() {
			if _, err := fmt.Fprintf(stdout, "openspec/%s not found; skipping activation\n", openspec.Filename); err != nil {
				return err
			}
		}
		return nil
	}
	if current == schemaName {
		_, err := fmt.Fprintf(stdout, "%s is already the active schema\n", schemaName)
		return err
	}
	if mode == activateAuto {
		if !stdinIsTTY() {
			return nil
		}
		ok, err := promptActivation(stdin, stdout, schemaName, current)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}
	if err := openspec.WriteSchema(cfgPath, schemaName); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			_, ferr := fmt.Fprintf(stdout, "openspec/%s not found; skipping activation\n", openspec.Filename)
			return ferr
		}
		return err
	}
	_, err = fmt.Fprintf(stdout, "activated %s\n", schemaName)
	return err
}

// promptActivation asks the user whether to (re)activate schemaName. The
// reply is treated case-insensitively; anything other than an explicit "y"
// or "yes" is a decline so the default matches the y/N convention.
func promptActivation(stdin io.Reader, stdout io.Writer, schemaName, current string) (bool, error) {
	var promptErr error
	if current == "" {
		_, promptErr = fmt.Fprintf(stdout, "Activate schema %q? [y/N] ", schemaName)
	} else {
		_, promptErr = fmt.Fprintf(stdout, "Replace active schema %q with %q? [y/N] ", current, schemaName)
	}
	if promptErr != nil {
		return false, promptErr
	}
	reader := bufio.NewReader(stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("reading prompt response: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
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
