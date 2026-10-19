// Package discovery walks the filesystem, detects version-bearing files, and
// generates an incrmit configuration.
package discovery

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf8"

	"github.com/BurntSushi/toml"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/files"
	"github.com/sasmaq/incrmit/internal/version"
)

// Occurrence is a single version candidate found inside a file: the parsed
// version, the 1-based line it appears on, the trimmed text of that line (used
// for human-readable dry-run output), and the key it sits under when the
// file's parser reads its format (nil for text). A long line is clipped to the
// stretch around the token, with "..." marking each cut (see contextText).
type Occurrence struct {
	Version version.Version
	Line    int
	Text    string
	Key     files.KeyPath
}

// Result is a single discovered target: its path as a config names it, relative
// to the config's directory in slash form (see DiscoverWithLimit), the parser
// that read it, and every version occurrence detected inside it, in the order
// they appear. A file with the same version in several places yields several
// occurrences; distinct versions in one file are all captured here (see
// Generate for how they map to config entries).
//
// A .json or .toml file its parser cannot read is scanned as text instead:
// Parser is then files.ParserText and Fallback says why. Skipped lists the
// values the parser passed over although they decode to a version (an escape
// such as \u002e), so the dry run can say so.
type Result struct {
	Path        string
	Parser      files.Parser
	Fallback    *files.ParseError
	Occurrences []Occurrence
	Skipped     []files.Skipped
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

// Candidates are located with the parser each file's name selects
// (files.ParserFor) — the same parser a bump of the generated config reads the
// file with, so what discovery records is exactly what a bump later replaces.
// The text parser takes every token version.FindTokens finds and version.Parse
// accepts, so an IPv4 address such as 192.168.1.1 (matched whole, never
// sliced) and two-component numbers are skipped; the JSON and TOML parsers
// take only those that are a whole string value. See version.FindTokens for
// the pattern and the filename guard.

// Discover walks the tree rooted at root and returns every file that contains a
// semantic version token, sorted by path for deterministic output. It scans the
// contents of any text file rather than relying on file names: every version
// candidate found in a file is recorded as an occurrence of that file's
// version. A file's name decides only how it is read: a .json or .toml file by
// its format, falling back to text when it is not that format, and anything
// else as text.
//
// The optional ignore patterns come from the config's `ignore` list and are
// applied in addition to the built-in ignoredDirs: any file or directory whose
// path matches a pattern is skipped, and a matching directory prunes its whole
// subtree. See ignore.go for the matching semantics.
//
// Discover is for a config written in root itself, so paths are relative to
// root; DiscoverWithLimit takes the config's directory separately. Files larger
// than DefaultMaxScanBytes are skipped; DiscoverWithLimit also chooses the cap.
func Discover(root string, ignore ...string) ([]Result, error) {
	return DiscoverWithLimit(root, root, DefaultMaxScanBytes, ignore...)
}

// DiscoverWithLimit is Discover for a config written in configDir, with an
// explicit per-file size cap.
//
// Every command resolves a config's paths against the directory holding it, so
// each result's Path is relative to configDir, not to root: scanning sub for a
// config in . records sub/VERSION, and a root outside configDir gives "../"
// paths. The two directories are compared with symbolic links resolved (see
// resolveDir). The ignore patterns are matched against those same paths, so a
// config's patterns mean the same thing whichever root it is regenerated from.
// The root itself is always walked, even when a pattern or a built-in name
// matches it; the patterns apply to what is inside it.
//
// A file larger than maxBytes is skipped rather than read. A maxBytes of zero
// (or less) removes the cap, so every regular file is scanned no matter its
// size.
func DiscoverWithLimit(root, configDir string, maxBytes int64, ignore ...string) ([]Result, error) {
	rootAbs, err := resolveDir(root)
	if err != nil {
		return nil, fmt.Errorf("discovery: resolving %q: %w", root, err)
	}
	baseAbs, err := resolveDir(configDir)
	if err != nil {
		return nil, fmt.Errorf("discovery: resolving %q: %w", configDir, err)
	}
	matcher := newIgnoreMatcher(ignore)
	var results []Result

	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// The walk keeps root as given, so its errors name the path the way
		// the user wrote it; only the recorded path is resolved. Every p is
		// root joined with the names below it, so Rel cannot fail here.
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		relSlash := relativeTo(baseAbs, filepath.Join(rootAbs, rel))

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

		// Never treat incrmit's own files as discovered targets: the config,
		// the bump-history state file, and the project lock are all
		// tool-maintained.
		if d.Name() == config.DefaultPath || d.Name() == config.StateFileName || d.Name() == config.LockFileName {
			return nil
		}

		// Skip a WriteAtomic temp file. One is a copy of a target with a new
		// version already written into it, so a scan that caught a concurrent
		// run mid-write — or that found the residue of a crashed one — would
		// otherwise record a `.incrmit-*.tmp` path as a real target, and the
		// generated config would list a file that vanishes at the next rename.
		if files.IsTempName(d.Name()) {
			return nil
		}

		if !matcher.empty() && matcher.match(relSlash, false) {
			return nil
		}

		r, ok := detect(p, maxBytes)
		if !ok {
			return nil
		}
		r.Path = relSlash
		results = append(results, r)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Path < results[j].Path })
	return results, nil
}

// SameDir reports whether a and b are one directory once resolved the way
// DiscoverWithLimit resolves the scan root and the config's directory, which
// is when the paths it returns are also relative to the scan root.
func SameDir(a, b string) bool {
	aDir, errA := resolveDir(a)
	bDir, errB := resolveDir(b)
	return errA == nil && errB == nil && aDir == bDir
}

// resolveDir returns dir as an absolute path with symbolic links resolved, so
// two spellings of one directory give clean paths relative to each other: on
// macOS /tmp is /private/tmp, and os.Getwd may report either, so a relative
// --output and an absolute --path could otherwise produce a path that climbs
// to / and back down. Only the longest prefix that exists is resolved, and the
// rest is kept as written, since a --dry-run may name an --output directory
// that is not there yet.
func resolveDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	rest := ""
	for p := abs; ; {
		if resolved, err := filepath.EvalSymlinks(p); err == nil {
			return filepath.Join(resolved, rest), nil
		}
		parent := filepath.Dir(p)
		if parent == p {
			return abs, nil
		}
		rest = filepath.Join(filepath.Base(p), rest)
		p = parent
	}
}

// relativeTo returns target relative to the directory base, both resolved by
// resolveDir, in slash form: the path a config in base lists target under. On
// Windows a target on another volume than base has no relative form, so its
// absolute path is returned; a config accepts an absolute path.
func relativeTo(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(rel)
}

// Generate renders the discovered results as the contents of an incrmit.toml
// config file. Each file contributes one [[files]] entry per distinct version
// it contains (in first-seen order): identical repeats collapse to a single
// entry, while a file with several differing versions yields several entries
// that share the same path. An entry records its `parser` only when the file
// was read with another parser than its name selects, which is a .json or
// .toml file scanned as text, so a bump reads it the way discover did.
//
// The ignore patterns (typically carried over from an existing config) are
// written back verbatim so regenerating the config never drops user-authored
// ignore entries. A description of the ignore option is always written above
// them; when no patterns are carried over, a commented-out example is emitted in
// their place so the feature is discoverable without reading the docs.
//
// A path that is not valid UTF-8 is an error: a TOML string cannot hold it, and
// writing it anyway would produce a config that nothing can load. Callers drop
// such results first (the discover command warns about each one).
func Generate(results []Result, ignore ...string) ([]byte, error) {
	cfg := config.Config{
		Ignore: ignore,
		Files:  make([]config.FileEntry, 0, len(results)),
	}
	for _, r := range results {
		if !utf8.ValidString(r.Path) {
			return nil, fmt.Errorf("discovery: %q cannot be listed in a config: its name is not valid UTF-8", r.Path)
		}
		seen := make(map[string]struct{}, len(r.Occurrences))
		for _, o := range r.Occurrences {
			v := o.Version.String()
			if _, dup := seen[v]; dup {
				continue
			}
			seen[v] = struct{}{}
			e := config.EntryFor(r.Path, o.Version)
			if r.Parser != files.ParserFor(r.Path) {
				e.Parser = string(r.Parser)
			}
			cfg.Files = append(cfg.Files, e)
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

// detect reads the file at path and returns every version candidate its parser
// finds, in the order they appear, each tagged with its 1-based line number,
// the text of that line, and its key. Files larger than maxBytes are skipped
// (maxBytes <= 0 means no cap). It is best-effort: unreadable, oversized,
// non-regular, and binary files yield ok == false, and so does a file with no
// candidate and nothing skipped. A .json or .toml file its parser cannot read
// is scanned as text, with the parser's error recorded as the Result's
// Fallback.
func detect(path string, maxBytes int64) (Result, bool) {
	capped := maxBytes > 0

	// Establish the file type before opening: opening a FIFO blocks until a
	// writer appears, so a pipe in the tree would hang the whole scan. Only
	// regular files within the size cap are read; a character device such as
	// /dev/zero would otherwise stream without end. The walk has already
	// excluded symlinks, so Lstat reports the entry itself.
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || (capped && info.Size() > maxBytes) {
		return Result{}, false
	}

	f, err := os.Open(path)
	if err != nil {
		return Result{}, false
	}
	defer func() { _ = f.Close() }()

	// Re-check on the descriptor, so a path swapped for something else between
	// the stat and the open is still rejected.
	if fi, ferr := f.Stat(); ferr != nil || !fi.Mode().IsRegular() || (capped && fi.Size() > maxBytes) {
		return Result{}, false
	}

	data, ok := readAtMost(f, maxBytes)
	if !ok || isBinary(data) {
		return Result{}, false
	}
	r := Result{Parser: files.ParserFor(path)}
	loc, err := r.Parser.Locate(data)
	if err != nil {
		if !errors.As(err, &r.Fallback) {
			return Result{}, false
		}
		r.Parser = files.ParserText
		if loc, err = r.Parser.Locate(data); err != nil {
			return Result{}, false
		}
	}
	r.Occurrences = scan(data, loc.Candidates)
	r.Skipped = loc.Skipped
	return r, len(r.Occurrences) > 0 || len(r.Skipped) > 0
}

// readAtMost reads r to the end, refusing it (ok == false) when it holds more
// than maxBytes; maxBytes <= 0 reads everything. It is the second guard behind
// the size check in detect, for a file that grows past the cap between the stat
// and the read. Such a file is refused whole rather than scanned truncated: a
// cut that falls mid-token turns "1.2.34" into "1.2.3", and discovery would
// record a version the file does not contain.
func readAtMost(r io.Reader, maxBytes int64) ([]byte, bool) {
	if maxBytes <= 0 {
		data, err := io.ReadAll(r)
		return data, err == nil
	}
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil || int64(len(data)) > maxBytes {
		return nil, false
	}
	return data, true
}

// scan returns every candidate a parser found in data as an occurrence, in the
// order they appear, each tagged with its 1-based line number and the text
// around it. cands must be in byte order, as Parser.Locate returns them.
func scan(data []byte, cands []files.Candidate) []Occurrence {
	occ := make([]Occurrence, 0, len(cands))
	lines := newLineCursor(data)
	for _, c := range cands {
		lines.seek(c.Start)
		occ = append(occ, Occurrence{
			Version: c.Version,
			Line:    lines.num,
			Text:    contextText(data, lines.start, lines.end, c.Start, c.End),
			Key:     c.Key,
		})
	}
	return occ
}

// lineCursor tracks the line holding each token as scan walks the tokens in
// order. A line ends at "\n", "\r\n", or a lone "\r" — the three conventions an
// editor recognizes — so a file with classic Mac line endings is numbered line
// by line, rather than reported as one line with carriage returns embedded in
// its text, which a terminal would act on when the dry run printed it.
//
// The cursor only moves forward, so a file costs one pass over its bytes however
// many tokens share a line. Locating each token's line from the start of the
// file instead made a minified file quadratic: twenty thousand occurrences on
// one 620 KB line took seconds and allocated gigabytes.
type lineCursor struct {
	data       []byte
	num        int // 1-based number of the current line
	start, end int // the current line's bytes, excluding its terminator
}

func newLineCursor(data []byte) *lineCursor {
	return &lineCursor{data: data, num: 1, end: lineEnd(data, 0)}
}

// seek moves the cursor to the line containing offset, which must not lie
// before the current line.
func (c *lineCursor) seek(offset int) {
	for offset > c.end && c.end < len(c.data) {
		next := c.end + 1
		if c.data[c.end] == '\r' && next < len(c.data) && c.data[next] == '\n' {
			next++ // "\r\n" is one line break, not two
		}
		c.num++
		c.start = next
		c.end = lineEnd(c.data, next)
	}
}

// lineEnd returns the index of the first line terminator at or after from, or
// len(data) when the last line has none.
func lineEnd(data []byte, from int) int {
	if i := bytes.IndexAny(data[from:], "\r\n"); i >= 0 {
		return from + i
	}
	return len(data)
}

// maxContext bounds how much of a token's line is kept on each side of it as
// dry-run context. Hand-written lines are shorter than that and are shown whole,
// trimmed; a minified file is one enormous line, and copying all of it into every
// occurrence made memory grow with occurrences times line length, and made the
// dry run print the whole file once per occurrence.
const maxContext = 80

// utf8BOM is the byte-order mark some Windows tools write at the start of a
// UTF-8 file.
var utf8BOM = []byte("\xEF\xBB\xBF")

// contextText returns the trimmed text of the line [start, end) holding the
// token [tokStart, tokEnd), keeping at most maxContext bytes on either side of
// the token and marking a cut with "...". A cut never splits a UTF-8 sequence.
// A byte-order mark opening the file is an encoding marker rather than text, so
// it is left out of the first line's context.
func contextText(data []byte, start, end, tokStart, tokEnd int) string {
	if start == 0 && bytes.HasPrefix(data, utf8BOM) {
		start = len(utf8BOM)
	}
	head, tail := "", ""
	if tokStart-start > maxContext {
		start = runeStart(data, tokStart-maxContext, tokStart)
		head = "..."
	}
	if end-tokEnd > maxContext {
		end = runeStart(data, tokEnd+maxContext, end)
		tail = "..."
	}
	return head + string(bytes.TrimSpace(data[start:end])) + tail
}

// runeStart returns the first index from i (and no further than limit) where a
// UTF-8 sequence begins, so cutting there never splits a character. Bytes that
// are not UTF-8 at all, such as Latin-1, end the search after utf8.UTFMax steps.
func runeStart(data []byte, i, limit int) int {
	for n := 0; n < utf8.UTFMax && i < limit && !utf8.RuneStart(data[i]); n++ {
		i++
	}
	return i
}

// isBinary reports whether data looks like a binary (non-text) file. A NUL byte
// is a reliable signal that a file is not human-readable text, so such files are
// skipped to avoid matching version-like byte sequences in binaries.
func isBinary(data []byte) bool {
	return bytes.IndexByte(data, 0) >= 0
}
