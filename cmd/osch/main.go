// Package main implements the osch CLI.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/jamessawle/osch/internal/github"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return printVersion(stdout, false)
	}
	switch args[0] {
	case "version":
		jsonOut := false
		for _, a := range args[1:] {
			if a == "--json" {
				jsonOut = true
			} else {
				return fmt.Errorf("unknown argument: %s", a)
			}
		}
		return printVersion(stdout, jsonOut)
	case "add":
		return runAdd(context.Background(), github.NewHTTPClient(), args[1:], stdout)
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

// runAdd validates the repo argument and reports the upstream schemas/ folder.
// Every failure is returned as a friendly, non-stacktrace error so main can
// print it and exit non-zero.
func runAdd(ctx context.Context, client github.Client, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: osch add <user/repo>")
	}
	repo, err := github.ParseRepo(args[0])
	if err != nil {
		return err
	}
	names, err := client.ListSchemas(ctx, repo)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "found %d schema(s) in %s\n", len(names), repo)
	return err
}

func printVersion(w io.Writer, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(w).Encode(map[string]string{
			"version": version,
			"commit":  commit,
			"date":    date,
		})
	}
	_, err := fmt.Fprintf(w, "osch %s (commit %s, built %s)\n", version, commit, date)
	return err
}
