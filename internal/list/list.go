// Package list implements `osch list`: an inventory of schemas installed
// under openspec/schemas/ in a given working directory.
package list

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/jamessawle/osch/internal/install"
	"gopkg.in/yaml.v3"
)

// configFile is the OpenSpec-owned project config; the top-level `schema` key
// names the currently-active schema.
const configFile = "config.yml"

// List scans workingDir/openspec/schemas/ and writes a NAME/ACTIVE/TRACKED
// table to stdout. A missing schemas directory is not an error.
func List(workingDir string, stdout io.Writer) error {
	schemasDir := filepath.Join(workingDir, "openspec", "schemas")
	entries, err := os.ReadDir(schemasDir)
	if errors.Is(err, fs.ErrNotExist) {
		_, err := fmt.Fprintln(stdout, "no schemas installed")
		return err
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", schemasDir, err)
	}

	active := readActiveSchema(filepath.Join(workingDir, "openspec", configFile))

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "NAME\tACTIVE\tTRACKED"); err != nil {
		return err
	}
	for _, name := range names {
		activeCol := ""
		if name == active {
			activeCol = "*"
		}
		tracked := "no"
		if isTracked(filepath.Join(schemasDir, name)) {
			tracked = "yes"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", name, activeCol, tracked); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// readActiveSchema reads the top-level `schema` key from path. A missing file
// or unparseable contents return an empty string: no schema is marked active,
// which the acceptance criteria treat as a non-error.
func readActiveSchema(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var cfg struct {
		Schema string `yaml:"schema"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return cfg.Schema
}

// isTracked reports whether schemaDir contains a readable per-schema manifest.
// Presence + readability is the signal; the manifest body is not validated in
// this slice.
func isTracked(schemaDir string) bool {
	f, err := os.Open(filepath.Join(schemaDir, install.ManifestFile))
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
