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
	"github.com/jamessawle/osch/internal/openspec"
)

const emptyMessage = "No OpenSpec schemas installed"

// List scans workingDir/openspec/schemas/ and writes a NAME/ACTIVE/TRACKED
// table to stdout. Missing or empty schemas directory prints emptyMessage and
// returns nil. Only fs.ErrNotExist on either openspec/schemas/ or the OpenSpec
// config file is treated as a soft failure; every other I/O error is returned.
func List(workingDir string, stdout io.Writer) error {
	schemasDir := filepath.Join(workingDir, "openspec", "schemas")
	entries, err := os.ReadDir(schemasDir)
	if errors.Is(err, fs.ErrNotExist) {
		_, err := fmt.Fprintln(stdout, emptyMessage)
		return err
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", schemasDir, err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		_, err := fmt.Fprintln(stdout, emptyMessage)
		return err
	}
	sort.Strings(names)

	active, _, err := openspec.ReadSchema(openspec.Path(workingDir))
	if err != nil {
		return err
	}

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

// isTracked reports whether schemaDir contains a per-schema manifest. The
// manifest body is not inspected at this layer; downstream commands that need
// the manifest contents will surface any corruption themselves.
func isTracked(schemaDir string) bool {
	_, err := os.Stat(filepath.Join(schemaDir, install.ManifestFile))
	return err == nil
}
