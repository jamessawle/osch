package engineer

import (
	"bytes"
	"context"
	"io"
	"os/exec"
)

// ShellRunner runs a `sh -c` command inside a working directory and
// returns the combined stdout/stderr output plus an error if the
// command exited non-zero or could not be started.
type ShellRunner interface {
	Run(ctx context.Context, workdir, command string) (output string, err error)
}

type realShell struct{ stderr io.Writer }

func (r realShell) Run(ctx context.Context, workdir, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workdir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = io.MultiWriter(&buf, r.stderr)
	err := cmd.Run()
	return buf.String(), err
}
