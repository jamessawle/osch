// Package chef implements the `brigade chef <chef-name>` subcommand:
// reads a Chit JSON from stdin, dispatches to the named Chef, writes a
// Proof JSON to stdout.
package chef

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	wirechef "github.com/jamessawle/osch/scripts/agent-loop/internal/chef"
	"github.com/jamessawle/osch/scripts/agent-loop/internal/chef/engineer"
)

// Cmd is the kong subcommand binding for `brigade chef <name>`.
type Cmd struct {
	Name string `arg:"" help:"Chef impl to invoke (e.g. engineer)."`
}

// IO bundles the streams the subcommand reads from / writes to.
type IO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Run executes the subcommand against the supplied IO streams.
func (c *Cmd) Run(ctx context.Context, ios IO) error {
	raw, err := readAll(ios.Stdin)
	if err != nil {
		return fmt.Errorf("read chit: %w", err)
	}

	var chit wirechef.Chit
	if err := json.Unmarshal(raw, &chit); err != nil {
		return fmt.Errorf("unmarshal chit: %w", err)
	}

	switch c.Name {
	case "engineer":
		deps := engineer.ProductionDeps(ios.Stderr)
		proof, err := engineer.Run(ctx, chit, deps)
		if err != nil {
			return fmt.Errorf("engineer crash: %w", err)
		}
		return writeProof(ios.Stdout, proof)
	default:
		return errors.New("unknown chef: " + c.Name)
	}
}

func readAll(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, errors.New("nil stdin")
	}
	return io.ReadAll(r)
}

func writeProof(w io.Writer, p wirechef.Proof) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(p)
}
