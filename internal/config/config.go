// Package config reads and writes the OpenSpec project config file
// (`openspec/config.yaml`). Only the top-level `schema` key is meaningful to
// osch; other keys are preserved on write but otherwise opaque. osch never
// creates the file — that remains OpenSpec's responsibility.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Filename is the basename of the OpenSpec project config inside `openspec/`.
const Filename = "config.yaml"

// Path returns the conventional config path for workingDir.
func Path(workingDir string) string {
	return filepath.Join(workingDir, "openspec", Filename)
}

// ReadSchema returns the value of the top-level `schema` key. `exists` reports
// whether the file is present; a malformed YAML file is treated as present
// with no active schema, mirroring `osch list`'s tolerant behaviour.
func ReadSchema(path string) (name string, exists bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("reading %s: %w", path, err)
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", true, nil
	}
	if v, ok := cfg["schema"].(string); ok {
		return v, true, nil
	}
	return "", true, nil
}

// WriteSchema sets the top-level `schema` key to schemaName, preserving other
// top-level keys' values. The file must already exist; ErrNotExist is
// returned otherwise. A read-modify-write round-trip through a generic map
// loses comments, blank lines, and key order — this is an accepted trade-off
// (see issue #29 brief).
func WriteSchema(path, schemaName string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	cfg := make(map[string]any)
	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("parsing %s: %w", path, err)
		}
		if cfg == nil {
			cfg = make(map[string]any)
		}
	}
	cfg["schema"] = schemaName
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
