package install

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// CheckLocalFiles reports whether the files on disk under schemaDir match the
// per-file SHA-256 hashes recorded in m.Files (ADR 0006). The manifest file
// itself is excluded from the comparison.
//
// A return of (true, nil) means every tracked file's hash matches and no extra
// files are present. Any of the following collapse to (false, nil): an empty
// or missing m.Files map, a tracked file whose hash differs, a tracked file
// missing locally, or an untracked file (not in m.Files and not the manifest)
// found in the schema folder. An error is returned only for unexpected I/O
// failures while walking or reading.
//
// Exposed as a reusable function so `osch update` can decide whether a refresh
// would clobber local edits.
func CheckLocalFiles(schemaDir string, m Manifest) (bool, error) {
	if len(m.Files) == 0 {
		return false, nil
	}
	seen := make(map[string]bool, len(m.Files))
	modified := false
	err := filepath.WalkDir(schemaDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(schemaDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == ManifestFile {
			return nil
		}
		want, ok := m.Files[rel]
		if !ok {
			modified = true
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		h := sha256.Sum256(data)
		if hex.EncodeToString(h[:]) != want {
			modified = true
		}
		seen[rel] = true
		return nil
	})
	if err != nil {
		return false, err
	}
	if modified {
		return false, nil
	}
	for rel := range m.Files {
		if !seen[rel] {
			return false, nil
		}
	}
	return true, nil
}
