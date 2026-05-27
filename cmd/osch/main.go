package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
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
