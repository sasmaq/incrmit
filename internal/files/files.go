// Package files reads and writes target files, replacing only the version
// token so surrounding formatting is preserved.
package files

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sasmaq/incrmit/internal/version"
)

// ErrNoVersion is returned when a file contains no semantic version token.
var ErrNoVersion = errors.New("files: no semantic version found")

// ErrVersionNotFound is returned by SetKnownVersion when the expected current
// version is not present in the file (e.g. the config is out of sync).
var ErrVersionNotFound = errors.New("files: expected version not found")

// Candidate version tokens are located with version.FindTokens, which is the
// single definition of where a version begins and ends (see its documentation
// for the pattern and the filename guard). This package deliberately does not
// understand any file format; structured detection for specific formats lives
// in the discovery package. A candidate that version.Parse rejects — an IPv4
// address, a two-component number — is skipped, and a "v"-prefixed token is
// distinct from its bare form so the exact written token is rewritten.

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
//
// The matcher also yields dotted-number runs that are not versions
// (two-component numbers, four-octet IPv4 addresses, ...); those fail
// version.Parse and are ignored, so neither ErrNoVersion nor ambiguity is
// affected by them.
func FindVersion(data []byte) (version.Version, error) {
	locs := version.FindTokens(data)

	distinct := make(map[string]struct{}, len(locs))
	var sole string
	for _, loc := range locs {
		s := string(data[loc[0]:loc[1]])
		if _, err := version.Parse(s); err != nil {
			continue
		}
		if _, seen := distinct[s]; !seen {
			distinct[s] = struct{}{}
			sole = s
		}
	}
	if len(distinct) == 0 {
		return version.Version{}, ErrNoVersion
	}
	if len(distinct) > 1 {
		vs := make([]string, 0, len(distinct))
		for v := range distinct {
			vs = append(vs, v)
		}
		sort.Strings(vs)
		return version.Version{}, &AmbiguousError{Versions: vs}
	}

	return version.Parse(sole)
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
	out, _ := SetKnownVersions(data, []Replacement{{Old: cur, New: newVer}})
	return out, nil
}

// SetKnownVersion replaces every occurrence of oldVer's token with newVer's
// token, leaving any other version-like strings in the file untouched. Unlike
// SetVersion it does not require the file to contain a single unambiguous
// version, so it is used when the current version is known from configuration.
// It returns ErrVersionNotFound if oldVer's token does not appear in data.
func SetKnownVersion(data []byte, oldVer, newVer version.Version) ([]byte, error) {
	out, counts := SetKnownVersions(data, []Replacement{{Old: oldVer, New: newVer}})
	if counts[oldVer.String()] == 0 {
		return nil, fmt.Errorf("%w: %s", ErrVersionNotFound, oldVer)
	}
	return out, nil
}

// Replacement is one pinned version and the version to write in its place. Old
// is the version as recorded in the config (or found in the file), New the
// bumped result.
type Replacement struct {
	Old version.Version
	New version.Version
}

// SetKnownVersions applies several replacements in a single pass over data.
// Every occurrence of a Replacement's Old version is rewritten to its New
// version; all other bytes (including version-like tokens not listed) are left
// untouched.
//
// Doing this in one pass over the original data is what makes overlapping bumps
// safe: replacing 1.2.3 -> 1.2.4 alongside 1.2.4 -> 1.2.5 does not cascade,
// because matches are taken from the original bytes and never re-scanned. The
// returned map reports how many times each old token was replaced (0 for tokens
// that never appeared), keyed by Old.String(), so callers can detect an
// out-of-sync config.
//
// Matching is by version rather than by raw text, which is what lets a pinned
// prerelease be found inside a release filename. version.FindTokens cuts
// "incrmit-1.2.3-rc.1.zip" back to its numeric core, because it cannot tell a
// prerelease from a hyphenated filename part on grammar alone; a Replacement
// pinning 1.2.3-rc.1 says which suffix is real, so exactly that suffix is
// consumed and the whole "1.2.3-rc.1" is rewritten, leaving ".zip" alone.
func SetKnownVersions(data []byte, repls []Replacement) ([]byte, map[string]int) {
	counts := make(map[string]int, len(repls))
	for _, r := range repls {
		counts[r.Old.String()] = 0
	}

	// Try the most specific pin first: at one location a bare 1.2.3 also matches
	// an occurrence carrying the recorded prerelease, and the pin that consumes
	// the suffix is the one that means it.
	ordered := make([]Replacement, len(repls))
	copy(ordered, repls)
	sort.SliceStable(ordered, func(i, j int) bool {
		return len(suffixOf(ordered[i].Old)) > len(suffixOf(ordered[j].Old))
	})

	var b strings.Builder
	b.Grow(len(data))
	prev := 0
	for _, loc := range version.FindTokens(data) {
		start, end := loc[0], loc[1]
		// A previous replacement may have consumed a recorded suffix that
		// happens to contain a candidate token of its own; skip anything that
		// falls inside what was already written.
		if start < prev {
			continue
		}
		for _, r := range ordered {
			matchEnd, ok := matchAt(data, start, end, r.Old)
			if !ok {
				continue
			}
			b.Write(data[prev:start])
			b.WriteString(r.New.String())
			prev = matchEnd
			counts[r.Old.String()]++
			break
		}
	}
	b.Write(data[prev:])
	return []byte(b.String()), counts
}

// matchAt reports where an occurrence of pin ends at the candidate token
// [start, end), and whether it occurs there at all.
//
// The straightforward case is an exact token match. The second case is a token
// the filename guard cut back to its numeric core: the pin names the suffix that
// really belongs to the version, so if the bytes after the core continue with
// exactly that suffix, the match extends over it. The byte after the suffix must
// not be alphanumeric, which keeps a pinned "-rc.1" from matching the start of
// "-rc.10".
func matchAt(data []byte, start, end int, pin version.Version) (int, bool) {
	tok := string(data[start:end])
	if tok == pin.String() {
		return end, true
	}

	suffix := suffixOf(pin)
	if suffix == "" || tok != pin.Release().String() {
		return 0, false
	}
	if !bytes.HasPrefix(data[end:], []byte(suffix)) {
		return 0, false
	}
	after := end + len(suffix)
	if after < len(data) && isAlnum(data[after]) {
		return 0, false
	}
	return after, true
}

// suffixOf returns the "-prerelease+build" tail of v as written in a token, or
// "" when v has neither section.
func suffixOf(v version.Version) string {
	s := ""
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

func isAlnum(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// ErrNotRegular reports that a target is not an ordinary file. Callers can
// detect it with errors.Is to phrase their own message.
var ErrNotRegular = errors.New("not a regular file")

// TooLargeError reports that a target is bigger than the caller's size cap, so
// it was not read. Size is the file's size in bytes and Limit the cap that
// rejected it.
type TooLargeError struct {
	Size  int64
	Limit int64
}

func (e *TooLargeError) Error() string {
	return fmt.Sprintf("files: file is %d bytes, over the %d byte limit", e.Size, e.Limit)
}

// ReadTarget reads a file incrmit is about to inspect or bump, with no size
// cap. See ReadTargetWithLimit for the details of what it refuses to read.
func ReadTarget(path string) ([]byte, error) {
	return ReadTargetWithLimit(path, 0)
}

// ReadTargetWithLimit reads a file incrmit is about to inspect or bump, refusing
// one larger than maxBytes with a *TooLargeError (maxBytes <= 0 means no cap).
// It establishes that the target is an ordinary file before opening it, because
// opening a named pipe blocks until a writer appears and a device such as
// /dev/zero never ends: either would leave incrmit hanging with no output
// instead of reporting a problem. The check follows symlinks, so a link to a
// real file is still read (a write later replaces the link rather than
// following it).
//
// A file that grows past the cap between the size check and the read is
// reported as too large rather than returned truncated: the caller writes the
// bumped contents back over the file, so short data would silently discard the
// rest of it.
func ReadTargetWithLimit(path string, maxBytes int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrNotRegular
	}
	if maxBytes <= 0 {
		return os.ReadFile(path)
	}
	if info.Size() > maxBytes {
		return nil, &TooLargeError{Size: info.Size(), Limit: maxBytes}
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		size := int64(len(data))
		if fi, ferr := f.Stat(); ferr == nil {
			size = fi.Size()
		}
		return nil, &TooLargeError{Size: size, Limit: maxBytes}
	}
	return data, nil
}

// ReadVersion reads path and returns the semantic version it contains.
func ReadVersion(path string) (version.Version, error) {
	data, err := ReadTarget(path)
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
	data, err := ReadTarget(path)
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
//
// The temp file is created in the target's own directory (never a shared temp
// directory) under an unpredictable name with O_EXCL, and holds mode 0600 while
// the data is written, so its contents are never readable at a wider mode than
// the target ends up with. Nothing here widens permissions: the final mode is
// exactly the target's previous one, or 0644 for a file being created.
//
// Because the rename replaces the name itself, a path that is a symlink ends up
// a regular file: incrmit never writes *through* a link to whatever it points
// at, which keeps a link in the tree from redirecting a write elsewhere.
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
	// Set the mode through the open descriptor rather than the temp file's name,
	// so the mode lands on the file just written even if something replaces that
	// name in the meantime.
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("files: setting mode on temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("files: closing temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("files: renaming temp file to %q: %w", path, err)
	}
	return nil
}
