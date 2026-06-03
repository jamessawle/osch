package install

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// CheckLocalFiles compares the files on disk under schemaDir against the
// per-file SHA-256 hashes recorded in m.Files (ADR 0006) and returns the
// forward-slash relative paths of any offenders. The manifest file itself is
// excluded from the comparison.
//
// An empty slice means the schema is clean: every tracked file's hash matches
// and no extra files are present. A non-empty slice (sorted) collects: tracked
// files whose hash differs, tracked files missing locally, and untracked files
// (not in m.Files and not the manifest) found in the schema folder. An error
// is returned only for unexpected I/O failures while walking or reading.
//
// Exposed as a reusable helper so `osch list` can label drift and `osch
// update` can refuse to clobber local edits.
func CheckLocalFiles(schemaDir string, m Manifest) ([]string, error) {
	if len(m.Files) == 0 {
		// Pre-hash-era manifest: we have no recorded hashes to verify against,
		// so we cannot prove the schema is clean. Return a synthetic offender
		// so callers (list, update) treat the schema as dirty.
		return []string{"(manifest records no file hashes)"}, nil
	}
	seen := make(map[string]bool, len(m.Files))
	var offenders []string
	err := filepath.WalkDir(schemaDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(schemaDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if rel == SnapshotDir {
				return filepath.SkipDir
			}
			return nil
		}
		if rel == ManifestFile {
			return nil
		}
		want, ok := m.Files[rel]
		if !ok {
			offenders = append(offenders, rel)
			return nil
		}
		seen[rel] = true
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		h := sha256.Sum256(data)
		if hex.EncodeToString(h[:]) != want {
			offenders = append(offenders, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for rel := range m.Files {
		if !seen[rel] {
			offenders = append(offenders, rel)
		}
	}
	sort.Strings(offenders)
	return offenders, nil
}
