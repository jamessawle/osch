package engineer

import "context"

// ShellRunner runs a `sh -c` command inside a working directory and
// returns the combined stdout/stderr output plus an error if the
// command exited non-zero or could not be started.
type ShellRunner interface {
	Run(ctx context.Context, workdir, command string) (output string, err error)
}
