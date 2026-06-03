// Package update implements `osch update <schema>`: refresh a single
// installed schema by replacing its files with the upstream default-branch
// HEAD bytes and rewriting the per-schema manifest. Update refuses (offline,
// before any network call) to overwrite a schema whose local files have
// drifted from the recorded hashes; a `--force` override is left for a
// later slice.
package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jamessawle/osch/internal/install"
	"github.com/jamessawle/osch/internal/remove"
	"github.com/jamessawle/osch/internal/source"
)

// ClientFactory returns a source.Client suited to a given Ref. It exists so
// the update command can pick the right provider implementation only after
// reading the per-schema manifest, without internal/update importing concrete
// providers itself.
type ClientFactory func(ref source.Ref) (source.Client, error)

// Update refreshes workingDir/openspec/schemas/<name>/ to the upstream
// default branch HEAD. When the pinned SHA already matches upstream the
// command is a no-op and nothing on disk is touched.
func Update(ctx context.Context, factory ClientFactory, workingDir, name string, stdout io.Writer) error {
	if err := remove.ValidateName(name); err != nil {
		return err
	}
	targetDir := remove.Dir(workingDir, name)
	if _, err := os.Stat(targetDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("schema %q is not installed (%s does not exist)", name, targetDir)
		}
		return fmt.Errorf("checking %s: %w", targetDir, err)
	}

	manifest, err := install.ReadManifest(targetDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("schema %q has no %s manifest; cannot update", name, install.ManifestFile)
		}
		return err
	}

	ref, err := source.ParseRef(manifest.Source)
	if err != nil {
		return fmt.Errorf("manifest source %q is not a valid repository: %w", manifest.Source, err)
	}

	offenders, err := install.CheckLocalFiles(targetDir, manifest)
	if err != nil {
		return fmt.Errorf("checking local modifications for %s: %w", name, err)
	}
	if len(offenders) > 0 {
		return fmt.Errorf("schema %q has local modifications; refusing to overwrite:\n  %s", name, strings.Join(offenders, "\n  "))
	}

	client, err := factory(ref)
	if err != nil {
		return err
	}

	newSHA, _, err := client.ListSchemas(ctx, ref)
	if err != nil {
		return fmt.Errorf("resolving upstream HEAD for schema %q (%s): %w", name, ref.String(), err)
	}
	if newSHA == manifest.SHA {
		_, err := fmt.Fprintf(stdout, "%s is already up to date at %s\n", name, manifest.SHA)
		return err
	}

	files, err := client.FetchSchemaFiles(ctx, ref, newSHA, name)
	if err != nil {
		return err
	}

	hashes, err := install.WriteFiles(targetDir, files)
	if err != nil {
		return err
	}
	if err := pruneStale(targetDir, files); err != nil {
		return err
	}

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
		return err
	}

	_, err = fmt.Fprintf(stdout, "updated %s to %s\n", name, newSHA)
	return err
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
