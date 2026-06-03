// Command brigade is the top-level binary for the agent loop step-1
// Chef extraction. Today it ships one subcommand: `chef <chef-name>`.
package main

import (
	"context"
	"os"

	"github.com/alecthomas/kong"

	chefcmd "github.com/jamessawle/osch/scripts/agent-loop/cmd/brigade/chef"
)

// CLI is the kong root for the brigade binary.
type CLI struct {
	Chef chefcmd.Cmd `cmd:"" help:"Invoke a Chef directly."`
}

func main() {
	var cli CLI
	parser := kong.Must(&cli, kong.Name("brigade"), kong.UsageOnError())
	kctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		parser.FatalIfErrorf(err)
	}
	switch kctx.Command() {
	case "chef <name>":
		if err := cli.Chef.Run(context.Background(), chefcmd.IO{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}); err != nil {
			_, _ = os.Stderr.WriteString("brigade: " + err.Error() + "\n")
			os.Exit(1)
		}
	default:
		parser.FatalIfErrorf(kctx.Error)
	}
}
