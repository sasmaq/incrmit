// Package discovery walks the filesystem, detects version-bearing files, and
// generates an incrmit configuration.
package discovery

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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

// versionRe matches a run of two or more dot-separated integer groups,
// optionally prefixed by a single leading "v" or "V", bounded by non-word
// characters. It deliberately matches more than just MAJOR.MINOR.PATCH: a
// candidate is validated with version.Parse, which accepts only exactly three
// components. Matching the whole dotted run (greedily) is what lets an IPv4
// address such as 192.168.1.1 be seen as a single four-component token and
// rejected, rather than having 192.168.1 (or 168.1.1) pulled out of it. The
// leading \b before the optional [vV] keeps the prefix from being taken out of
// the middle of a word (so "rev1.2.3"/"dev1.2.3" are not treated as versions).
var versionRe = regexp.MustCompile(`\b[vV]?\d+(?:\.\d+)+\b`)

// Discover walks the tree rooted at root and returns every file that contains a
// semantic version token, sorted by path for deterministic output. It scans the
// contents of any text file rather than relying on file names: the first
// MAJOR.MINOR.PATCH token found in a file is recorded as that file's version.
//
// The optional ignore patterns come from the config's `ignore` list and are
// applied in addition to the built-in ignoredDirs: any file or directory whose
// path (relative to root) matches a pattern is skipped, and a matching directory
// prunes its whole subtree. See ignore.go for the matching semantics.
func Discover(root string, ignore ...string) ([]Result, error) {
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

		occ, ok := detect(p)
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
			cfg.Files = append(cfg.Files, config.FileEntry{
				Path:    r.Path,
				Version: v,
			})
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
// and the trimmed text of that line. It is best-effort: unreadable and binary
// files yield ok == false. versionRe also matches dotted-number runs that are
// not versions (two-component numbers like 3.9, four-octet IPv4 addresses like
// 192.168.1.1, ...); those fail version.Parse and are skipped, so only real
// three-component versions are reported.
func detect(path string) ([]Occurrence, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	if isBinary(data) {
		return nil, false
	}
	var occ []Occurrence
	for _, loc := range versionRe.FindAllIndex(data, -1) {
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
