// Package discovery walks the filesystem, detects version-bearing files, and
// generates an incrmit configuration.
package discovery

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/BurntSushi/toml"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/version"
)

// Occurrence is a single version token found inside a file: the parsed version,
// the 1-based line it appears on, and the trimmed text of that line (used for
// human-readable dry-run output).
type Occurrence struct {
	Version version.Version
	Line    int
	Text    string
}

// Result is a single discovered target: a path relative to the scan root and
// every version occurrence detected inside it, in the order they appear. A file
// with the same version in several places yields several occurrences; distinct
// versions in one file are all captured here (see Generate for how they map to
// config entries).
type Result struct {
	Path        string
	Occurrences []Occurrence
}

// ignoredDirs are directory names skipped during the walk: version-control
// metadata, dependency caches, and common build outputs.
var ignoredDirs = map[string]struct{}{
	".git":         {},
	".hg":          {},
	".svn":         {},
	"node_modules": {},
	"vendor":       {},
	"dist":         {},
	"build":        {},
	"target":       {},
}

// DefaultMaxScanBytes bounds how much of any one file discovery will read
// unless a caller asks for a different cap. Version tokens live in small text
// files (manifests, VERSION files, source headers), so a larger file is not a
// plausible target and is skipped rather than pulled into memory. This keeps a
// scan bounded when a tree contains very large files.
const DefaultMaxScanBytes = 32 << 20 // 32 MiB

// Candidate version tokens are located with version.FindTokens — the same
// function package files rewrites with, so what discovery records is exactly
// what a bump later replaces. Each candidate is validated with version.Parse,
// which accepts only exactly three numeric components, so an IPv4 address such
// as 192.168.1.1 (matched whole, never sliced) and two-component numbers are
// skipped. See version.FindTokens for the pattern and the filename guard.

// Discover walks the tree rooted at root and returns every file that contains a
// semantic version token, sorted by path for deterministic output. It scans the
// contents of any text file rather than relying on file names: every version
// token found in a file is recorded as an occurrence of that file's version.
//
// The optional ignore patterns come from the config's `ignore` list and are
// applied in addition to the built-in ignoredDirs: any file or directory whose
// path (relative to root) matches a pattern is skipped, and a matching directory
// prunes its whole subtree. See ignore.go for the matching semantics.
//
// Files larger than DefaultMaxScanBytes are skipped; use DiscoverWithLimit to
// choose a different cap.
func Discover(root string, ignore ...string) ([]Result, error) {
	return DiscoverWithLimit(root, DefaultMaxScanBytes, ignore...)
}

// DiscoverWithLimit is Discover with an explicit per-file size cap: a file
// larger than maxBytes is skipped rather than read. A maxBytes of zero (or less)
// removes the cap, so every regular file is scanned no matter its size.
func DiscoverWithLimit(root string, maxBytes int64, ignore ...string) ([]Result, error) {
	matcher := newIgnoreMatcher(ignore)
	var results []Result

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			rel = p
		}
		relSlash := filepath.ToSlash(rel)

		// Never follow a symlink found inside the tree. Following one would let
		// a link read (and, on a later bump, copy in) a file outside the scan
		// root, so links are reported by neither name. Go's WalkDir already
		// declines to descend into symlinked directories; this also covers
		// symlinks to files, devices, and sockets.
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		if d.IsDir() {
			if p != root {
				if _, skip := ignoredDirs[d.Name()]; skip {
					return filepath.SkipDir
				}
				if !matcher.empty() && matcher.match(relSlash, true) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Never treat the config file or the bump-history state file (both
		// tool-maintained) as a discovered target.
		if d.Name() == config.DefaultPath || d.Name() == config.StateFileName {
			return nil
		}

		if !matcher.empty() && matcher.match(relSlash, false) {
			return nil
		}

		occ, ok := detect(p, maxBytes)
		if !ok {
			return nil
		}
		results = append(results, Result{Path: relSlash, Occurrences: occ})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Path < results[j].Path })
	return results, nil
}

// Generate renders the discovered results as the contents of an incrmit.toml
// config file. Each file contributes one [[files]] entry per distinct version
// it contains (in first-seen order): identical repeats collapse to a single
// entry, while a file with several differing versions yields several entries
// that share the same path.
//
// The ignore patterns (typically carried over from an existing config) are
// written back verbatim so regenerating the config never drops user-authored
// ignore entries. A description of the ignore option is always written above
// them; when no patterns are carried over, a commented-out example is emitted in
// their place so the feature is discoverable without reading the docs.
func Generate(results []Result, ignore ...string) ([]byte, error) {
	cfg := config.Config{
		Ignore: ignore,
		Files:  make([]config.FileEntry, 0, len(results)),
	}
	for _, r := range results {
		seen := make(map[string]struct{}, len(r.Occurrences))
		for _, o := range r.Occurrences {
			v := o.Version.String()
			if _, dup := seen[v]; dup {
				continue
			}
			seen[v] = struct{}{}
			cfg.Files = append(cfg.Files, config.EntryFor(r.Path, o.Version))
		}
	}

	var buf bytes.Buffer
	buf.WriteString("# incrmit.toml (generated by `incrmit discover`)\n\n")
	buf.WriteString(config.IgnoreComment(len(cfg.Ignore) > 0))
	if len(cfg.Ignore) == 0 {
		// The encoder emits nothing for an empty ignore list, so separate the
		// commented example from the following [[files]] table ourselves.
		buf.WriteString("\n")
	}
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return nil, fmt.Errorf("discovery: encoding config: %w", err)
	}
	return buf.Bytes(), nil
}

// detect reads the file at path and returns every [v]MAJOR.MINOR.PATCH token it
// contains, in the order they appear, each tagged with its 1-based line number
// and the trimmed text of that line. Files larger than maxBytes are skipped
// (maxBytes <= 0 means no cap). It is best-effort: unreadable, oversized,
// non-regular, and binary files yield ok == false. The matcher also yields
// dotted-number runs that are not versions (two-component numbers like 3.9,
// four-octet IPv4 addresses like 192.168.1.1, ...); those fail version.Parse and
// are skipped, so only real three-component versions are reported.
func detect(path string, maxBytes int64) ([]Occurrence, bool) {
	capped := maxBytes > 0

	// Establish the file type before opening: opening a FIFO blocks until a
	// writer appears, so a pipe in the tree would hang the whole scan. Only
	// regular files within the size cap are read; a character device such as
	// /dev/zero would otherwise stream without end. The walk has already
	// excluded symlinks, so Lstat reports the entry itself.
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || (capped && info.Size() > maxBytes) {
		return nil, false
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()

	// Re-check on the descriptor, so a path swapped for something else between
	// the stat and the open is still rejected.
	if fi, ferr := f.Stat(); ferr != nil || !fi.Mode().IsRegular() || (capped && fi.Size() > maxBytes) {
		return nil, false
	}

	// LimitReader is a second guard: the file may still grow past the cap
	// between the stat above and the read here. With the cap removed there is
	// nothing to guard against, so the file is read whole.
	var r io.Reader = f
	if capped {
		r = io.LimitReader(f, maxBytes)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, false
	}
	if isBinary(data) {
		return nil, false
	}
	var occ []Occurrence
	for _, loc := range version.FindTokens(data) {
		start, end := loc[0], loc[1]
		v, perr := version.Parse(string(data[start:end]))
		if perr != nil {
			continue
		}
		occ = append(occ, Occurrence{
			Version: v,
			Line:    lineNumber(data, start),
			Text:    lineText(data, start),
		})
	}
	if len(occ) == 0 {
		return nil, false
	}
	return occ, true
}

// lineNumber returns the 1-based line number of the byte at offset in data.
func lineNumber(data []byte, offset int) int {
	return 1 + bytes.Count(data[:offset], []byte{'\n'})
}

// lineText returns the trimmed text of the line containing the byte at offset.
func lineText(data []byte, offset int) string {
	start := bytes.LastIndexByte(data[:offset], '\n') + 1
	end := bytes.IndexByte(data[offset:], '\n')
	if end < 0 {
		end = len(data)
	} else {
		end += offset
	}
	return string(bytes.TrimSpace(data[start:end]))
}

// isBinary reports whether data looks like a binary (non-text) file. A NUL byte
// is a reliable signal that a file is not human-readable text, so such files are
// skipped to avoid matching version-like byte sequences in binaries.
func isBinary(data []byte) bool {
	return bytes.IndexByte(data, 0) >= 0
}
