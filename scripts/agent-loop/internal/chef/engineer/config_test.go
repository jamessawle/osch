package engineer_test

import (
	"path/filepath"
	"testing"

	"github.com/jamessawle/osch/scripts/agent-loop/internal/chef/engineer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, writeBytes(p, []byte(content)))
	return p
}

func TestLoadConfig_ValidRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, ".brigade.yml", `setup:
  - go mod download
checks:
  - go build ./...
  - go test ./...
`)
	cfg, err := engineer.LoadConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{"go mod download"}, cfg.Setup)
	assert.Equal(t, []string{"go build ./...", "go test ./..."}, cfg.Checks)
}

func TestLoadConfig_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := engineer.LoadConfig(t.TempDir())
	require.Error(t, err)
	assert.ErrorIs(t, err, engineer.ErrConfigMissing)
}

func TestLoadConfig_MalformedYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, ".brigade.yml", "setup: [unclosed")
	_, err := engineer.LoadConfig(dir)
	require.Error(t, err)
	assert.ErrorIs(t, err, engineer.ErrConfigMalformed)
}

func TestLoadConfig_SetupNotAList(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, ".brigade.yml", `setup:
  key: value
checks:
  - go test ./...
`)
	_, err := engineer.LoadConfig(dir)
	require.Error(t, err)
	assert.ErrorIs(t, err, engineer.ErrConfigMalformed)
}

func TestLoadConfig_NonStringCommand(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFile(t, dir, ".brigade.yml", `setup:
  - 123
checks:
  - go test ./...
`)
	_, err := engineer.LoadConfig(dir)
	require.Error(t, err)
	assert.ErrorIs(t, err, engineer.ErrConfigMalformed)
}
