// Package files reads and writes target files, replacing only the version
// token so surrounding formatting is preserved.
package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/sasmaq/incrmit/internal/version"
)

// ErrNoVersion is returned when a file contains no semantic version token.
var ErrNoVersion = errors.New("files: no semantic version found")

// versionRe matches a bare MAJOR.MINOR.PATCH token bounded by non-word
// characters. It deliberately does not understand any file format; structured
// detection for specific formats lives in the discovery package.
var versionRe = regexp.MustCompile(`\b\d+\.\d+\.\d+\b`)

// AmbiguousError reports that a file contains more than one distinct version
// token, so a generic replacement cannot pick one safely.
type AmbiguousError struct {
	Versions []string
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("files: ambiguous version: found multiple distinct versions %v", e.Versions)
}

// FindVersion locates the semantic version in data. It returns ErrNoVersion if
// no token is present and an *AmbiguousError if multiple distinct versions are
// found. Repeated occurrences of the same version are not ambiguous.
func FindVersion(data []byte) (version.Version, error) {
	matches := versionRe.FindAll(data, -1)
	if len(matches) == 0 {
		return version.Version{}, ErrNoVersion
	}

	distinct := make(map[string]struct{}, len(matches))
	for _, m := range matches {
		distinct[string(m)] = struct{}{}
	}
	if len(distinct) > 1 {
		vs := make([]string, 0, len(distinct))
		for v := range distinct {
			vs = append(vs, v)
		}
		sort.Strings(vs)
		return version.Version{}, &AmbiguousError{Versions: vs}
	}

	// Exactly one distinct token; parse it to validate the numeric components.
	return version.Parse(string(matches[0]))
}

// SetVersion returns a copy of data in which every occurrence of the current
// version token is replaced by newVer. Only the version token is rewritten; all
// surrounding bytes (whitespace, quotes, keys, trailing newline) are preserved.
// It returns the same errors as FindVersion when the current version cannot be
// uniquely identified.
func SetVersion(data []byte, newVer version.Version) ([]byte, error) {
	cur, err := FindVersion(data)
	if err != nil {
		return nil, err
	}
	curToken := cur.String()
	newToken := newVer.String()

	locs := versionRe.FindAllIndex(data, -1)
	var b strings.Builder
	b.Grow(len(data))
	prev := 0
	for _, loc := range locs {
		start, end := loc[0], loc[1]
		if string(data[start:end]) != curToken {
			continue
		}
		b.Write(data[prev:start])
		b.WriteString(newToken)
		prev = end
	}
	b.Write(data[prev:])
	return []byte(b.String()), nil
}

// ReadVersion reads path and returns the semantic version it contains.
func ReadVersion(path string) (version.Version, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return version.Version{}, fmt.Errorf("files: reading %q: %w", path, err)
	}
	v, err := FindVersion(data)
	if err != nil {
		return version.Version{}, fmt.Errorf("files: %q: %w", path, err)
	}
	return v, nil
}

// ApplyBump reads path, transforms its version with bump, and (unless dryRun)
// writes the result back in place atomically. It returns the old and new
// versions. The selection of which component to bump is the caller's concern;
// this function only applies the supplied transform.
func ApplyBump(path string, bump func(version.Version) version.Version, dryRun bool) (oldVer, newVer version.Version, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return version.Version{}, version.Version{}, fmt.Errorf("files: reading %q: %w", path, err)
	}

	oldVer, err = FindVersion(data)
	if err != nil {
		return version.Version{}, version.Version{}, fmt.Errorf("files: %q: %w", path, err)
	}
	newVer = bump(oldVer)

	if dryRun {
		return oldVer, newVer, nil
	}

	updated, err := SetVersion(data, newVer)
	if err != nil {
		return version.Version{}, version.Version{}, fmt.Errorf("files: %q: %w", path, err)
	}
	if err := WriteAtomic(path, updated); err != nil {
		return version.Version{}, version.Version{}, err
	}
	return oldVer, newVer, nil
}

// WriteAtomic writes data to path by writing to a temporary file in the same
// directory and renaming it over the target. The rename is atomic on the same
// filesystem, so a crash mid-write never leaves a partially written file. The
// existing file mode is preserved when the target already exists.
func WriteAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)

	perm := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, ".incrmit-*.tmp")
	if err != nil {
		return fmt.Errorf("files: creating temp file in %q: %w", dir, err)
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if we fail before the rename succeeds.
	defer func() {
		if _, statErr := os.Stat(tmpName); statErr == nil {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("files: writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("files: syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("files: closing temp file: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("files: setting mode on temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("files: renaming temp file to %q: %w", path, err)
	}
	return nil
}
