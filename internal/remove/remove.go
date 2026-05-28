// Package remove holds the logic for deleting an installed schema directory
// under openspec/schemas/<name>/. It is kept separate from the Cobra command
// so it can be unit-tested without driving the CLI.
package remove

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateName rejects schema arguments that contain path separators or "..",
// so a malicious or accidental argument cannot escape openspec/schemas/.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("schema name must not be empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid schema name %q", name)
	}
	if strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return fmt.Errorf("invalid schema name %q: must be a plain folder name", name)
	}
	return nil
}

// Dir returns the absolute path to the schema folder under workingDir.
// Callers should ValidateName first.
func Dir(workingDir, name string) string {
	return filepath.Join(workingDir, "openspec", "schemas", name)
}

// Remove deletes workingDir/openspec/schemas/<name>/ recursively. It returns
// an error when the folder does not exist so callers can surface a clear
// non-zero exit; this is intentional rather than treating missing as a no-op.
func Remove(workingDir, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	target := Dir(workingDir, name)
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("schema %q is not installed (%s does not exist)", name, target)
		}
		return fmt.Errorf("checking %s: %w", target, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", target)
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("removing %s: %w", target, err)
	}
	return nil
}
