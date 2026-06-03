package update

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/jamessawle/osch/internal/install"
)

// snapshotClock is the source of timestamps for snapshot folder names. It is
// indirected so tests can pin it deterministically.
var snapshotClock = func() time.Time { return time.Now().UTC() }

// snapshotTimeFormat is UTC ISO 8601 basic format (Zulu, no separators) — the
// parent .osch/ already namespaces the entry, so no "backup-" prefix is added.
const snapshotTimeFormat = "20060102T150405Z"

// snapshotSchema copies every regular file under schemaDir (recursively,
// excluding the .osch/ directory itself) into a fresh
// schemaDir/.osch/<UTC-ISO-datetime>/ folder and returns its absolute path.
// The .osch/.gitignore self-ignoring marker is created on the same operation
// if missing, so git never sees any of this.
//
// Any I/O failure aborts the snapshot with an error and the partial snapshot
// folder is left on disk; callers must treat this as fatal and skip every
// subsequent write to the schema folder.
func snapshotSchema(schemaDir string) (string, error) {
	snapsRoot := filepath.Join(schemaDir, install.SnapshotDir)
	if err := os.MkdirAll(snapsRoot, 0o755); err != nil {
		return "", fmt.Errorf("creating snapshot root %s: %w", snapsRoot, err)
	}
	gitignore := filepath.Join(snapsRoot, ".gitignore")
	if _, err := os.Stat(gitignore); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("checking %s: %w", gitignore, err)
		}
		if err := os.WriteFile(gitignore, []byte("*\n"), 0o644); err != nil {
			return "", fmt.Errorf("writing %s: %w", gitignore, err)
		}
	}

	stamp := snapshotClock().Format(snapshotTimeFormat)
	snapDir := filepath.Join(snapsRoot, stamp)
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		return "", fmt.Errorf("creating snapshot folder %s: %w", snapDir, err)
	}

	err := filepath.WalkDir(schemaDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(schemaDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if filepath.ToSlash(rel) == install.SnapshotDir {
			return filepath.SkipDir
		}
		dst := filepath.Join(snapDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", filepath.Dir(dst), err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", dst, err)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("snapshotting %s: %w", schemaDir, err)
	}
	return snapDir, nil
}
