package main

import "fmt"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	fmt.Printf("osch %s (commit %s, built %s)\n", version, commit, date)
}
