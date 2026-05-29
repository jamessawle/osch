// Package install holds the provider-agnostic logic for installing a schema
// from an upstream repository: fetching its files, writing them under
// openspec/schemas/<name>/, and emitting the per-schema .osch.json manifest
// described in ADR 0005.
package install

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jamessawle/osch/internal/source"
)

// ManifestFile is the per-schema manifest filename written into each installed
// schema folder (see ADR 0005).
const ManifestFile = ".osch.json"

// ManifestSchemaURL is the value written into the manifest's $schema field. It
// names the manifest format; the URL is stable and intended to host the JSON
// Schema once published.
const ManifestSchemaURL = "https://jamessawle.github.io/osch/schemas/manifest.v1.json"

// ManifestSchemaVersion is the current manifest format version (ADR 0005).
const ManifestSchemaVersion = 1

// Manifest is the on-disk shape of openspec/schemas/<name>/.osch.json.
type Manifest struct {
	Schema        string            `json:"$schema"`
	SchemaVersion int               `json:"schema_version"`
	Source        string            `json:"source"`
	Name          string            `json:"name"`
	SHA           string            `json:"sha"`
	Files         map[string]string `json:"files"`
}

// Add installs a single schema from ref into workingDir/openspec/schemas/<name>/
// and returns the installed schema's name so callers can drive follow-on steps
// (e.g. activation) without re-deriving it. This slice implements the happy
// path for an upstream repo with exactly one schema directory under schemas/;
// multiple schemas or an existing local target produce a friendly error rather
// than a partial install.
func Add(ctx context.Context, client source.Client, ref source.Ref, workingDir string, stdout io.Writer) (string, error) {
	sha, names, err := client.ListSchemas(ctx, ref)
	if err != nil {
		return "", err
	}
	if len(names) != 1 {
		return "", fmt.Errorf("repository %s contains %d schemas; multi-schema installs are not supported yet", ref, len(names))
	}
	name := names[0]
	targetDir := filepath.Join(workingDir, "openspec", "schemas", name)
	if _, err := os.Stat(targetDir); err == nil {
		return "", fmt.Errorf("refusing to overwrite existing schema folder %s (use osch update to refresh)", targetDir)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("checking target folder %s: %w", targetDir, err)
	}

	files, err := client.FetchSchemaFiles(ctx, ref, sha, name)
	if err != nil {
		return "", err
	}

	hashes, err := WriteFiles(targetDir, files)
	if err != nil {
		return "", err
	}

	manifest := Manifest{
		Schema:        ManifestSchemaURL,
		SchemaVersion: ManifestSchemaVersion,
		Source:        ref.String(),
		Name:          name,
		SHA:           sha,
		Files:         hashes,
	}
	if err := WriteManifest(targetDir, manifest); err != nil {
		return "", err
	}

	if _, err := fmt.Fprintf(stdout, "installed %s from %s @ %s\n", name, ref, sha); err != nil {
		return "", err
	}
	return name, nil
}

// WriteFiles writes each fetched file under targetDir and returns the SHA-256
// hash of the bytes written, keyed by the same forward-slash relative path used
// to fetch it. Hashing the bytes after they are written makes the manifest a
// faithful integrity record of what is on disk (ADR 0006).
func WriteFiles(targetDir string, files map[string][]byte) (map[string]string, error) {
	hashes := make(map[string]string, len(files))
	for relPath, data := range files {
		absPath := filepath.Join(targetDir, filepath.FromSlash(relPath))
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return nil, fmt.Errorf("creating %s: %w", filepath.Dir(absPath), err)
		}
		if err := os.WriteFile(absPath, data, 0o644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", absPath, err)
		}
		h := sha256.Sum256(data)
		hashes[relPath] = hex.EncodeToString(h[:])
	}
	return hashes, nil
}

// ReadManifest loads and decodes the per-schema manifest from targetDir.
// fs.ErrNotExist is surfaced unwrapped so callers can distinguish "schema is
// not tracked by osch" from arbitrary I/O failures with errors.Is.
func ReadManifest(targetDir string) (Manifest, error) {
	path := filepath.Join(targetDir, ManifestFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return m, nil
}

// WriteManifest serialises m as pretty-printed JSON (deterministic since
// encoding/json sorts map keys) into targetDir/.osch.json.
func WriteManifest(targetDir string, m Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding manifest: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(targetDir, ManifestFile)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}
