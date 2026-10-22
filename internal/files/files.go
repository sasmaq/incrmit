// Package files reads and writes target files, replacing only the version
// token so surrounding formatting is preserved, and locates the versions in a
// file with a parser chosen for its format (parser.go).
package files

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/sasmaq/incrmit/internal/version"
)

// ErrNoVersion is returned when a file contains no semantic version token.
var ErrNoVersion = errors.New("files: no semantic version found")

// ErrVersionNotFound is returned by SetKnownVersion when the expected current
// version is not present in the file (e.g. the config is out of sync).
var ErrVersionNotFound = errors.New("files: expected version not found")

// Every function here that finds or rewrites a version takes its candidates
// from a Parser (parser.go): the text parser's are the tokens
// version.FindTokens finds, which is the single definition of where a version
// begins and ends (see its documentation for the pattern and the filename
// guard), and the JSON and TOML parsers' are the subset of those that are a
// whole string value. Nothing here calls FindTokens itself, so a file is read
// the same way whichever command reads it. A token version.Parse rejects (an
// IPv4 address, a two-component number) is never a candidate, and a
// "v"-prefixed token is distinct from its bare form so the exact written token
// is rewritten.

// AmbiguousError reports that a file contains more than one distinct version
// token, so a generic replacement cannot pick one safely.
type AmbiguousError struct {
	Versions []string
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("files: ambiguous version: found multiple distinct versions %v", e.Versions)
}

// FindVersion returns the one version among cands, the candidates a parser
// found in a file. It returns ErrNoVersion if there is none and an
// *AmbiguousError if they hold several distinct versions. Repeated occurrences
// of the same version are not ambiguous.
//
// Distinct means a distinct token, so v1.2.3 and 1.2.3 are two versions; a
// candidate's token is exactly its Version's String(), which version.Parse
// guarantees by refusing any spelling it would not write back.
func FindVersion(cands []Candidate) (version.Version, error) {
	distinct := make(map[string]struct{}, len(cands))
	var sole version.Version
	for _, c := range cands {
		s := c.Version.String()
		if _, seen := distinct[s]; !seen {
			distinct[s] = struct{}{}
			sole = c.Version
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
	return sole, nil
}

// SetVersion returns a copy of data in which every candidate holding the
// current version is replaced by newVer, where cands are the candidates a
// parser found in data. Only the version token is rewritten; all surrounding
// bytes (whitespace, quotes, keys, trailing newline) are preserved. It returns
// the same errors as FindVersion when the current version cannot be uniquely
// identified.
func SetVersion(data []byte, cands []Candidate, newVer version.Version) ([]byte, error) {
	cur, err := FindVersion(cands)
	if err != nil {
		return nil, err
	}
	out, _ := SetKnownVersions(data, cands, []Replacement{{Old: cur, New: newVer}})
	return out, nil
}

// SetKnownVersion replaces every candidate holding oldVer's token with newVer's
// token, leaving any other version-like strings in the file untouched. Unlike
// SetVersion it does not require the file to contain a single unambiguous
// version, so it is used when the current version is known from configuration.
// It returns ErrVersionNotFound if no candidate holds oldVer's token.
func SetKnownVersion(data []byte, cands []Candidate, oldVer, newVer version.Version) ([]byte, error) {
	out, counts := SetKnownVersions(data, cands, []Replacement{{Old: oldVer, New: newVer}})
	if counts[oldVer.String()] == 0 {
		return nil, fmt.Errorf("%w: %s", ErrVersionNotFound, oldVer)
	}
	return out, nil
}

// Replacement is one pinned version and the version to write in its place. Old
// is the version as recorded in the config (or found in the file), New the
// bumped result.
//
// A replacement without a key rewrites every candidate holding Old. One whose
// entry has a key is Keyed, and rewrites only the candidates the key claims
// (Located.Claim), whose start offsets At holds in byte order: in a
// package.json that pins a dependency at the project's version, the entry
// keyed to `version` leaves the dependency alone.
type Replacement struct {
	Old   version.Version
	New   version.Version
	Keyed bool
	At    []int
}

// SetKnownVersions applies several replacements in a single pass over data,
// whose candidates a parser found as cands (in byte order). Every candidate
// holding a Replacement's Old version is rewritten to its New version, a keyed
// Replacement's only where it claims one; all other bytes (including
// version-like tokens that are not candidates, or that no Replacement lists)
// are left untouched.
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
// consumed and the whole "1.2.3-rc.1" is rewritten, leaving ".zip" alone. A
// JSON or TOML candidate is a whole string value, so no suffix follows one.
func SetKnownVersions(data []byte, cands []Candidate, repls []Replacement) ([]byte, map[string]int) {
	out, perRepl, _ := setKnownVersions(data, cands, repls)
	counts := make(map[string]int, len(repls))
	for i, r := range repls {
		counts[r.Old.String()] += perRepl[i]
	}
	return out, counts
}

// Matches returns how many candidates each of repls would rewrite, in repls'
// order, matching exactly as SetKnownVersions does. Unlike its counts, two
// replacements of one version are counted apart, which is what tells two keyed
// entries for one version apart.
func Matches(data []byte, cands []Candidate, repls []Replacement) []int {
	_, counts, _ := setKnownVersions(data, cands, repls)
	return counts
}

// setKnownVersions is SetKnownVersions, counting the rewrites of each
// replacement apart, in repls' order, and also returning each rewrite it
// made, for the check Rewrite makes afterwards.
func setKnownVersions(data []byte, cands []Candidate, repls []Replacement) ([]byte, []int, []rewrite) {
	counts := make([]int, len(repls))

	// Try the most specific pin first: at one location a bare 1.2.3 also matches
	// an occurrence carrying the recorded prerelease, and the pin that consumes
	// the suffix is the one that means it.
	ordered := make([]int, len(repls))
	for i := range ordered {
		ordered[i] = i
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return len(suffixOf(repls[ordered[i]].Old)) > len(suffixOf(repls[ordered[j]].Old))
	})

	var b strings.Builder
	b.Grow(len(data))
	var done []rewrite
	prev := 0
	for _, c := range cands {
		start, end := c.Start, c.End
		// A previous replacement may have consumed a recorded suffix that
		// happens to contain a candidate token of its own; skip anything that
		// falls inside what was already written.
		if start < prev {
			continue
		}
		for _, i := range ordered {
			r := repls[i]
			if r.Keyed {
				if _, claimed := slices.BinarySearch(r.At, start); !claimed {
					continue
				}
			}
			matchEnd, ok := matchAt(data, start, end, r.Old)
			if !ok {
				continue
			}
			b.Write(data[prev:start])
			b.WriteString(r.New.String())
			prev = matchEnd
			counts[i]++
			done = append(done, rewrite{key: c.Key, old: string(data[start:matchEnd]), new: r.New.String()})
			break
		}
	}
	b.Write(data[prev:])
	return []byte(b.String()), counts, done
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
// This is the one function that decides a file is safe to open: every target
// is read through it, and so are incrmit's own files, through ReadOwnFile.
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

// MaxOwnFileBytes caps how much of one of incrmit's own files ReadOwnFile
// reads. Both are small: the config holds one short entry per target, and the
// journal holds at most history.MaxEntries bumps, which history.Save keeps
// under this cap. Parsing TOML allocates about twenty times the file's size, so
// the cap is what bounds the memory a planted file can cost. It is fixed rather
// than tied to --max-file-size, which is about targets.
const MaxOwnFileBytes = 16 << 20

// ReadOwnFile reads a file incrmit keeps for itself rather than one it bumps:
// the config (including the --output discover reads its ignore list from) and
// the bump journal. It makes the same type check as a target read, since a
// repository can commit `incrmit.toml -> /dev/zero` as easily as a target, and
// refuses a file over MaxOwnFileBytes with a *TooLargeError.
func ReadOwnFile(path string) ([]byte, error) {
	return ReadTargetWithLimit(path, MaxOwnFileBytes)
}

// ReadVersion reads path and returns the semantic version it contains, read
// with the parser its name selects (ParserFor). A .json or .toml file that is
// not its format is an error, a *ParseError, rather than scanned as text.
func ReadVersion(path string) (version.Version, error) {
	data, err := ReadTarget(path)
	if err != nil {
		return version.Version{}, fmt.Errorf("files: reading %q: %w", path, err)
	}
	loc, err := ParserFor(path).Locate(data)
	if err != nil {
		return version.Version{}, fmt.Errorf("files: %q: %w", path, err)
	}
	v, err := FindVersion(loc.Candidates)
	if err != nil {
		return version.Version{}, fmt.Errorf("files: %q: %w", path, err)
	}
	return v, nil
}

// ApplyBump reads path, transforms its version with bump, and (unless dryRun)
// writes the result back in place atomically. It returns the old and new
// versions. The file is read with the parser its name selects, as ReadVersion
// reads it. The selection of which component to bump is the caller's concern;
// this function only applies the supplied transform, and a transform that fails
// (a *version.OverflowError from a component already at math.MaxInt) fails the
// call before anything is written.
func ApplyBump(path string, bump func(version.Version) (version.Version, error), dryRun bool) (oldVer, newVer version.Version, err error) {
	data, err := ReadTarget(path)
	if err != nil {
		return version.Version{}, version.Version{}, fmt.Errorf("files: reading %q: %w", path, err)
	}

	p := ParserFor(path)
	loc, err := p.Locate(data)
	if err != nil {
		return version.Version{}, version.Version{}, fmt.Errorf("files: %q: %w", path, err)
	}
	oldVer, err = FindVersion(loc.Candidates)
	if err != nil {
		return version.Version{}, version.Version{}, fmt.Errorf("files: %q: %w", path, err)
	}
	newVer, err = bump(oldVer)
	if err != nil {
		return version.Version{}, version.Version{}, fmt.Errorf("files: %q: %w", path, err)
	}

	if dryRun {
		return oldVer, newVer, nil
	}

	updated, _, err := Rewrite(p, data, loc.Candidates, []Replacement{{Old: oldVer, New: newVer}})
	if err != nil {
		return version.Version{}, version.Version{}, fmt.Errorf("files: %q: %w", path, err)
	}
	if err := WriteAtomic(path, updated); err != nil {
		return version.Version{}, version.Version{}, err
	}
	return oldVer, newVer, nil
}

// tempPrefix and tempSuffix bracket the name WriteAtomic gives its in-flight
// temporary file. They are exported through IsTempName and SweepTemps so the
// discovery walk can recognize one of incrmit's own temp files and skip it
// rather than scanning it as a target, and so a run holding the project lock
// can clear the ones a crashed run left behind.
const (
	tempPrefix = ".incrmit-"
	tempSuffix = ".tmp"
)

// IsTempName reports whether name is the base name of an in-flight (or
// leftover) WriteAtomic temporary file. The prefix ends in "-" and the suffix
// begins with ".", so the two can never overlap into a false match on a short
// name: anything carrying both is one of ours.
func IsTempName(name string) bool {
	return strings.HasPrefix(name, tempPrefix) && strings.HasSuffix(name, tempSuffix)
}

// SweepTemps removes the WriteAtomic temporary files directly inside dir and
// returns how many it removed. Subdirectories are not descended into: a temp
// file only ever appears beside the target being written, so the caller sweeps
// exactly the directories it is about to write in.
//
// This is only safe while the project lock is held, which is the one moment no
// other incrmit run can have a write in flight — every temp file present then
// is the residue of a run that died before its rename. Removing one that is
// still being written would cost another run its bump, so callers must not
// sweep when the lock was unavailable.
//
// Individual removals that fail are skipped rather than reported: a leftover
// temp file is clutter, not a reason to fail the command the user asked for.
func SweepTemps(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("files: sweeping temp files in %q: %w", dir, err)
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !IsTempName(e.Name()) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
			removed++
		}
	}
	return removed, nil
}

// WriteAtomic writes data to path by writing to a temporary file in the same
// directory and renaming it over the target. The rename is atomic on the same
// filesystem, so a crash mid-write never leaves a partially written file. The
// existing permission bits are preserved when the target already exists.
//
// The temp file is created in the target's own directory (never a shared temp
// directory) under an unpredictable name with O_EXCL, and holds mode 0600 while
// the data is written, so its contents are never readable at a wider mode than
// the target ends up with. Nothing here widens permissions: the final
// permission bits are exactly the target's previous ones, or 0644 for a file
// being created.
//
// Because the rename replaces the name itself, the file at path afterwards is a
// new file, not the old one rewritten. Everything the old file carried beyond
// its contents and permission bits follows from that, and each consequence is
// deliberate:
//
//   - A symlink ends up a regular file: incrmit never writes *through* a link to
//     whatever it points at, which keeps a link in the tree from redirecting a
//     write elsewhere.
//   - A read-only file (0444) is still rewritten, and stays 0444: nothing opens
//     the target for writing, so its own mode does not stand in the way. Write
//     protection comes from the directory.
//   - The setuid, setgid, and sticky bits are dropped, because only Perm() is
//     carried across. Re-applying setuid would mint a setuid file owned by
//     whoever ran incrmit, which is why the kernel clears those bits on an
//     unprivileged in-place write too.
//   - A hard link is broken: the other names keep the old file, and with it the
//     old contents. Keeping the link would mean truncating and rewriting the
//     shared file in place, which is the partial write the rename rules out.
//   - Ownership, extended attributes, and ACLs are those of a new file created
//     by the running user, not the old file's.
func WriteAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)

	perm := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(dir, tempPrefix+"*"+tempSuffix)
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
