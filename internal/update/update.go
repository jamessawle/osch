// Package update implements `osch update [schema]`: refresh installed
// schemas by replacing their files with the upstream default-branch HEAD
// bytes and rewriting per-schema manifests. With a single name, exactly one
// schema is processed; with no name, every tracked schema under
// openspec/schemas/ is processed in alphabetical order. Update refuses
// (offline, before any network call) to overwrite a schema whose local files
// have drifted from the recorded hashes; passing force=true skips that
// refusal (the always-on snapshot still captures the pre-update state).
package update

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/jamessawle/osch/internal/install"
	"github.com/jamessawle/osch/internal/remove"
	"github.com/jamessawle/osch/internal/source"
)

// ClientFactory returns a source.Client suited to a given Ref. It exists so
// the update command can pick the right provider implementation only after
// reading the per-schema manifest, without internal/update importing concrete
// providers itself.
type ClientFactory func(ref source.Ref) (source.Client, error)

// Status is the outcome of a per-schema update attempt.
type Status int

const (
	// StatusUpToDate means the pinned SHA already matched upstream; nothing
	// was touched on disk.
	StatusUpToDate Status = iota
	// StatusUpdated means upstream bytes were written and the manifest was
	// rewritten to the new SHA. A snapshot was taken.
	StatusUpdated
	// StatusRefused means local modifications were detected and --force was
	// not set. No network call was made and nothing on disk was touched.
	StatusRefused
	// StatusFailed means a genuine error occurred (manifest unreadable,
	// network failure, write failure, etc.). The wrapped error is in
	// Result.Err.
	StatusFailed
)

// Result captures one schema's update outcome. Renderers use Status to pick
// the body text; the other fields are populated only for the relevant
// outcomes.
type Result struct {
	Name         string
	Status       Status
	OldSHA       string   // populated when Status is Updated or UpToDate
	NewSHA       string   // populated when Status is Updated or UpToDate
	SnapshotPath string   // populated only when Status is Updated
	Offenders    []string // populated only when Status is Refused
	Err          error    // populated only when Status is Failed
}

// Update refreshes workingDir/openspec/schemas/<name>/ to the upstream
// default branch HEAD. When the pinned SHA already matches upstream the
// command is a no-op and nothing on disk is touched. The three normal
// outcomes — up-to-date, updated, refused — are encoded in Result.Status
// with a nil error. Genuine failures return a non-nil error and a Result
// with Name set and Status==StatusFailed.
func Update(ctx context.Context, factory ClientFactory, workingDir, name string, force bool) (Result, error) {
	res := Result{Name: name}
	if err := remove.ValidateName(name); err != nil {
		res.Status = StatusFailed
		res.Err = err
		return res, err
	}
	targetDir := remove.Dir(workingDir, name)
	if _, err := os.Stat(targetDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			e := fmt.Errorf("schema %q is not installed (%s does not exist)", name, targetDir)
			res.Status = StatusFailed
			res.Err = e
			return res, e
		}
		e := fmt.Errorf("checking %s: %w", targetDir, err)
		res.Status = StatusFailed
		res.Err = e
		return res, e
	}

	manifest, err := install.ReadManifest(targetDir)
	if err != nil {
		var e error
		if errors.Is(err, fs.ErrNotExist) {
			e = fmt.Errorf("schema %q has no %s manifest; cannot update", name, install.ManifestFile)
		} else {
			e = err
		}
		res.Status = StatusFailed
		res.Err = e
		return res, e
	}

	ref, err := source.ParseRef(manifest.Source)
	if err != nil {
		e := fmt.Errorf("manifest source %q is not a valid repository: %w", manifest.Source, err)
		res.Status = StatusFailed
		res.Err = e
		return res, e
	}

	if !force {
		offenders, err := install.CheckLocalFiles(targetDir, manifest)
		if err != nil {
			e := fmt.Errorf("checking local modifications for %s: %w", name, err)
			res.Status = StatusFailed
			res.Err = e
			return res, e
		}
		if len(offenders) > 0 {
			res.Status = StatusRefused
			res.OldSHA = manifest.SHA
			res.Offenders = offenders
			return res, nil
		}
	}

	client, err := factory(ref)
	if err != nil {
		res.Status = StatusFailed
		res.Err = err
		return res, err
	}

	newSHA, _, err := client.ListSchemas(ctx, ref)
	if err != nil {
		e := fmt.Errorf("resolving upstream HEAD for schema %q (%s): %w", name, ref.String(), err)
		res.Status = StatusFailed
		res.Err = e
		return res, e
	}
	if newSHA == manifest.SHA {
		res.Status = StatusUpToDate
		res.OldSHA = manifest.SHA
		res.NewSHA = manifest.SHA
		return res, nil
	}

	files, err := client.FetchSchemaFiles(ctx, ref, newSHA, name)
	if err != nil {
		res.Status = StatusFailed
		res.Err = err
		return res, err
	}

	snapPath, err := snapshotSchema(targetDir)
	if err != nil {
		e := fmt.Errorf("snapshotting %s before update: %w", name, err)
		res.Status = StatusFailed
		res.Err = e
		return res, e
	}

	hashes, err := install.WriteFiles(targetDir, files)
	if err != nil {
		res.Status = StatusFailed
		res.Err = err
		return res, err
	}
	if err := pruneStale(targetDir, files); err != nil {
		res.Status = StatusFailed
		res.Err = err
		return res, err
	}

	oldSHA := manifest.SHA
	manifest.Source = ref.String()
	manifest.Name = name
	manifest.SHA = newSHA
	manifest.Files = hashes
	if manifest.Schema == "" {
		manifest.Schema = install.ManifestSchemaURL
	}
	if manifest.SchemaVersion == 0 {
		manifest.SchemaVersion = install.ManifestSchemaVersion
	}
	if err := install.WriteManifest(targetDir, manifest); err != nil {
		res.Status = StatusFailed
		res.Err = err
		return res, err
	}

	res.Status = StatusUpdated
	res.OldSHA = oldSHA
	res.NewSHA = newSHA
	res.SnapshotPath = snapPath
	return res, nil
}

// UpdateAll iterates every tracked schema under workingDir/openspec/schemas/
// (every immediate subdirectory containing a readable .osch.json) in
// alphabetical order, running Update against each. A refusal or failure in
// one schema does not stop the others; per-schema failures are folded into
// the returned slice as Result{Status: StatusFailed}. The outer error is
// reserved for "can't even start" cases — schemas directory exists but is
// unreadable for reasons other than not-existing.
//
// A missing or empty schemas directory, or one containing no tracked
// schemas, returns (nil, nil); callers render the "no tracked schemas" hint
// from that.
//
//nolint:revive // UpdateAll is the contract name from the public CLI brief; renaming to All would obscure intent at call sites.
func UpdateAll(ctx context.Context, factory ClientFactory, workingDir string, force bool) ([]Result, error) {
	schemasDir := filepath.Join(workingDir, "openspec", "schemas")
	entries, err := os.ReadDir(schemasDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", schemasDir, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifestPath := filepath.Join(schemasDir, e.Name(), install.ManifestFile)
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}
		names = append(names, e.Name())
	}
	if len(names) == 0 {
		return nil, nil
	}
	sort.Strings(names)
	results := make([]Result, 0, len(names))
	for _, n := range names {
		r, _ := Update(ctx, factory, workingDir, n, force)
		results = append(results, r)
	}
	return results, nil
}

// pruneStale deletes any file under targetDir that is not in keep and is not
// the manifest itself, then removes the directories that are now empty. keep
// is the forward-slash relative-path set of files just written by WriteFiles.
func pruneStale(targetDir string, keep map[string][]byte) error {
	wantedAbs := make(map[string]struct{}, len(keep)+1)
	for rel := range keep {
		wantedAbs[filepath.Join(targetDir, filepath.FromSlash(rel))] = struct{}{}
	}
	wantedAbs[filepath.Join(targetDir, install.ManifestFile)] = struct{}{}

	var dirs []string
	err := filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != targetDir {
				if d.Name() == install.SnapshotDir {
					return filepath.SkipDir
				}
				dirs = append(dirs, path)
			}
			return nil
		}
		if _, ok := wantedAbs[path]; ok {
			return nil
		}
		return os.Remove(path)
	})
	if err != nil {
		return fmt.Errorf("pruning %s: %w", targetDir, err)
	}
	// Remove now-empty directories deepest-first.
	for i := len(dirs) - 1; i >= 0; i-- {
		entries, err := os.ReadDir(dirs[i])
		if err != nil {
			return fmt.Errorf("reading %s: %w", dirs[i], err)
		}
		if len(entries) == 0 {
			if err := os.Remove(dirs[i]); err != nil {
				return fmt.Errorf("removing %s: %w", dirs[i], err)
			}
		}
	}
	return nil
}
