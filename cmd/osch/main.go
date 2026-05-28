// Package main is the osch CLI entrypoint. All command logic lives in
// cmd/osch/cmd so this file stays a thin wrapper that wires ldflags-injected
// build metadata into the Cobra command tree.
package main

import "github.com/jamessawle/osch/cmd/osch/cmd"

// Build identity wired by goreleaser via -X main.{version,commit,date}.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.Version = version
	cmd.Commit = commit
	cmd.Date = date
	cmd.Execute()
}
