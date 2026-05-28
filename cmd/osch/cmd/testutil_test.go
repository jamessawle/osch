package cmd

import (
	"os"
	"testing"
)

// chdir switches the process working directory to dir and returns a cleanup
// function. install.Add writes under workingDir = ".", so tests that invoke
// the Cobra tree need to point "." at a temp dir to avoid touching the repo.
func chdir(t *testing.T, dir string) func() {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	return func() {
		if err := os.Chdir(old); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	}
}
