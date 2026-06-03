// Package engineer loads and manages .brigade.yml configuration.
package engineer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var (
	// ErrConfigMissing is returned when .brigade.yml is not found at worktree root.
	ErrConfigMissing = errors.New(".brigade.yml not found at worktree root")
	// ErrConfigMalformed is returned when .brigade.yml cannot be parsed or has invalid structure.
	ErrConfigMalformed = errors.New(".brigade.yml malformed")
)

// Config represents the parsed .brigade.yml configuration.
type Config struct {
	Setup  []string `yaml:"setup"`
	Checks []string `yaml:"checks"`
}

// LoadConfig reads and parses .brigade.yml from the given worktree path.
func LoadConfig(worktreePath string) (Config, error) {
	path := filepath.Join(worktreePath, ".brigade.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, ErrConfigMissing
		}
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	// First parse as generic YAML to validate structure. An empty file is
	// treated as a no-op config (no setup, no checks); the yaml decoder
	// signals empty input with io.EOF, which is otherwise indistinguishable
	// from "malformed" if propagated.
	var rawCfg map[string]interface{}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&rawCfg); err != nil {
		if errors.Is(err, io.EOF) {
			return Config{Setup: []string{}, Checks: []string{}}, nil
		}
		return Config{}, fmt.Errorf("%w: %v", ErrConfigMalformed, err)
	}

	// Validate and extract setup commands
	setup := []string{}
	if setupVal, ok := rawCfg["setup"]; ok {
		setupList, ok := setupVal.([]interface{})
		if !ok {
			return Config{}, ErrConfigMalformed
		}
		for _, cmd := range setupList {
			cmdStr, ok := cmd.(string)
			if !ok {
				return Config{}, ErrConfigMalformed
			}
			setup = append(setup, cmdStr)
		}
	}

	// Validate and extract check commands
	checks := []string{}
	if checksVal, ok := rawCfg["checks"]; ok {
		checksList, ok := checksVal.([]interface{})
		if !ok {
			return Config{}, ErrConfigMalformed
		}
		for _, cmd := range checksList {
			cmdStr, ok := cmd.(string)
			if !ok {
				return Config{}, ErrConfigMalformed
			}
			checks = append(checks, cmdStr)
		}
	}

	return Config{Setup: setup, Checks: checks}, nil
}
