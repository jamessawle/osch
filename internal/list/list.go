// Package list implements `osch list`: an inventory of schemas installed
// under openspec/schemas/ in a given working directory.
package list

import (
	"context"
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
	"github.com/jamessawle/osch/internal/source"
)

const emptyMessage = "No OpenSpec schemas installed"

const shortSHALen = 7

// Upstream column values.
const (
	upstreamUpToDate = "up-to-date"
	upstreamBehind   = "behind"
	upstreamUnknown  = "unknown"
)

// Files column values.
const (
	filesClean    = "clean"
	filesModified = "modified"
)

// List scans workingDir/openspec/schemas/ and writes a
// NAME/ACTIVE/TRACKED/SOURCE/SHA/FILES/UPSTREAM table to stdout. Missing or empty
// schemas directory prints emptyMessage and returns nil. Only fs.ErrNotExist
// on either openspec/schemas/ or the OpenSpec config file is treated as a
// soft failure; every other I/O error is returned.
//
// When offline is true, no network lookups are performed and every tracked
// row's UPSTREAM column reads "unknown". Otherwise client is used to resolve
// each distinct tracked source at most once; any client error collapses the
// row's UPSTREAM to "unknown" rather than aborting the command.
func List(ctx context.Context, workingDir string, stdout io.Writer, client source.Client, offline bool) error {
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

	// upstreamCache memoises LatestSHA results by manifest.Source string so
	// schemas from the same upstream share one network call within this
	// invocation. The zero string and error are both meaningful: a non-nil
	// error means we resolved this source to "unknown" and any further row
	// from the same source must read the same value.
	type cached struct {
		sha string
		err error
	}
	upstreamCache := map[string]cached{}
	resolveUpstream := func(manifestSource string) string {
		if offline || client == nil {
			return upstreamUnknown
		}
		if hit, ok := upstreamCache[manifestSource]; ok {
			if hit.err != nil {
				return upstreamUnknown
			}
			return hit.sha
		}
		ref, err := source.ParseRef(manifestSource)
		if err != nil {
			upstreamCache[manifestSource] = cached{err: err}
			return upstreamUnknown
		}
		sha, err := client.LatestSHA(ctx, ref)
		upstreamCache[manifestSource] = cached{sha: sha, err: err}
		if err != nil {
			return upstreamUnknown
		}
		return sha
	}

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "NAME\tACTIVE\tTRACKED\tSOURCE\tSHA\tFILES\tUPSTREAM"); err != nil {
		return err
	}
	for _, name := range names {
		activeCol := ""
		if name == active {
			activeCol = "*"
		}
		tracked := "no"
		source, sha, files, upstream := "", "", "", ""
		schemaDir := filepath.Join(schemasDir, name)
		if m, err := install.ReadManifest(schemaDir); err == nil {
			tracked = "yes"
			source = m.Source
			sha = shortSHA(m.SHA)
			offenders, ferr := install.CheckLocalFiles(schemaDir, m)
			if ferr != nil || len(offenders) > 0 {
				files = filesModified
			} else {
				files = filesClean
			}
			latest := resolveUpstream(m.Source)
			switch latest {
			case upstreamUnknown:
				upstream = upstreamUnknown
			case m.SHA:
				upstream = upstreamUpToDate
			default:
				upstream = upstreamBehind
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			// Manifest exists but is unreadable/unparseable: mark tracked
			// (the file is present) but leave source/sha blank. Without a
			// readable source we cannot resolve upstream, so report unknown.
			tracked = "yes"
			upstream = upstreamUnknown
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", name, activeCol, tracked, source, sha, files, upstream); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func shortSHA(sha string) string {
	if len(sha) <= shortSHALen {
		return sha
	}
	return sha[:shortSHALen]
}
