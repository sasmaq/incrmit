// Package config loads and validates the TOML configuration that lists the
// target files incrmit reads and updates.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// DefaultPath is the config file name resolved when no explicit path is given.
const DefaultPath = "incrmit.toml"

// StateFileName is the name of the bump-history state file written next to the
// config after a successful bump so `incrmit undo` can revert it. It is local,
// tool-maintained state (see package history) and is not a discovery target.
const StateFileName = ".incrmit.state.toml"

// Config is the in-memory model of an incrmit.toml file.
//
// Ignore is declared before Files so it is encoded as a top-level array ahead of
// the `[[files]]` array-of-tables: in TOML a bare key written after a table
// would be parsed as belonging to that table, so the field order here is what
// keeps the generated config valid.
type Config struct {
	Ignore []string    `toml:"ignore,omitempty"`
	Files  []FileEntry `toml:"files"`
}

// FileEntry is a single target file listed in the config. Version is optional
// and is populated by the discover command.
type FileEntry struct {
	Path    string `toml:"path"`
	Version string `toml:"version,omitempty"`
}

// Marshal renders the config as TOML bytes, with a header noting that the file
// is tool-maintained. The ignore option is documented above the entries; when
// the config has no patterns yet, a commented-out example is written in their
// place so the feature is discoverable even after a bump rewrites the file.
func Marshal(c *Config) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("# incrmit.toml (maintained by incrmit)\n\n")
	buf.WriteString(IgnoreComment(len(c.Ignore) > 0))
	if len(c.Ignore) == 0 {
		// The encoder emits nothing for an empty ignore list, so separate the
		// commented example from the following [[files]] table ourselves.
		buf.WriteString("\n")
	}
	if err := toml.NewEncoder(&buf).Encode(c); err != nil {
		return nil, fmt.Errorf("config: encoding: %w", err)
	}
	return buf.Bytes(), nil
}

// IgnoreComment returns the comment block that documents the top-level `ignore`
// option. It always describes the matching rules; when hasIgnore is false it
// also appends a commented-out example so a user can enable the feature by
// uncommenting a line. The block has no trailing blank line: when hasIgnore is
// true the real `ignore = [...]` line follows immediately below its description.
// It is shared by the discover config generation and the bump-time rewrite so
// both files carry the same guidance.
func IgnoreComment(hasIgnore bool) string {
	var b bytes.Buffer
	b.WriteString("# ignore: folders and files for `incrmit discover` to skip, on top of the\n")
	b.WriteString("# built-in ignores (.git, node_modules, vendor, and build outputs).\n")
	if !hasIgnore {
		b.WriteString("# ignore = [\"testdata/\", \"*.lock\", \"docs/**\"]\n")
	}
	return b.String()
}

// ResolvePath returns path when it is non-empty, otherwise DefaultPath. It lets
// callers pass a user-provided --config value straight through and fall back to
// the conventional location.
func ResolvePath(path string) string {
	if path == "" {
		return DefaultPath
	}
	return path
}

// Load reads and parses the config at path. A missing file is reported with a
// dedicated, actionable error (see IsNotExist) so callers can suggest running
// `incrmit discover`. The parsed config is validated before being returned.
func Load(path string) (*Config, error) {
	path = ResolvePath(path)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, &NotExistError{Path: path}
		}
		return nil, fmt.Errorf("config: reading %q: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parsing %q: %w", path, err)
	}
	cfg.normalizeIgnore()

	if err := cfg.Validate(filepath.Dir(path)); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadIgnore reads only the ignore list from the config at path, without
// validating the file targets. It is used by `discover`, which needs the
// user-authored ignore patterns from an existing config even when the config's
// listed files are stale or the config is about to be regenerated. Because the
// --output path is a file discover overwrites (and may not currently be a valid
// config at all), this is deliberately lenient: a missing or unparseable file
// yields a nil list and no error, so discovery simply falls back to the built-in
// ignores. Only an unexpected read error (e.g. a permission problem) is
// surfaced.
func LoadIgnore(path string) ([]string, error) {
	path = ResolvePath(path)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("config: reading %q: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, nil
	}
	cfg.normalizeIgnore()
	return cfg.Ignore, nil
}

// normalizeIgnore trims surrounding whitespace and converts backslash path
// separators to forward slashes in every ignore pattern, so matching is
// platform-independent (a config authored on Windows works on Unix and vice
// versa) and patterns compare consistently. Backslashes are replaced
// unconditionally rather than via filepath.ToSlash, which is a no-op on
// Unix. Order is preserved; empty entries are left in place for Validate to
// reject with a clear message.
func (c *Config) normalizeIgnore() {
	for i, pat := range c.Ignore {
		c.Ignore[i] = strings.TrimSpace(strings.ReplaceAll(pat, "\\", "/"))
	}
}

// Validate checks that the config is usable: it must list at least one file,
// each entry must have a non-empty path, and every target must exist on disk as
// an ordinary file (not a directory, named pipe, device, or socket).
// A path may be listed in more than one entry as long as each such entry pins a
// distinct, non-empty version (so a file containing several differing versions
// can be tracked); exact (path, version) duplicates are rejected, as is a
// repeated path where any entry omits the version (which would be ambiguous).
// Relative target paths are resolved against baseDir (the directory containing
// the config file).
func (c *Config) Validate(baseDir string) error {
	for i, pat := range c.Ignore {
		if strings.TrimSpace(pat) == "" {
			return fmt.Errorf("config: ignore[%d] is empty; remove it or give a folder/file pattern", i)
		}
	}

	if len(c.Files) == 0 {
		return errors.New("config: no files listed; add at least one [[files]] entry")
	}

	seen := make(map[string]struct{}, len(c.Files))
	pathsSeen := make(map[string]struct{}, len(c.Files))
	for i, f := range c.Files {
		if f.Path == "" {
			return fmt.Errorf("config: files[%d] has an empty path", i)
		}
		key := f.Path + "\x00" + f.Version
		if _, dup := seen[key]; dup {
			if f.Version == "" {
				return fmt.Errorf("config: duplicate path %q", f.Path)
			}
			return fmt.Errorf("config: duplicate path %q with version %q", f.Path, f.Version)
		}
		// A repeated path is only allowed when every entry for it pins a
		// distinct version; a bare (version-less) repeat is ambiguous.
		if _, repeat := pathsSeen[f.Path]; repeat && f.Version == "" {
			return fmt.Errorf("config: duplicate path %q", f.Path)
		}
		if _, repeat := pathsSeen[f.Path]; repeat {
			if _, bare := seen[f.Path+"\x00"]; bare {
				return fmt.Errorf("config: duplicate path %q", f.Path)
			}
		}
		seen[key] = struct{}{}
		pathsSeen[f.Path] = struct{}{}

		resolved := f.Path
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(baseDir, resolved)
		}
		info, err := os.Stat(resolved)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("config: target %q does not exist", f.Path)
			}
			return fmt.Errorf("config: stat %q: %w", f.Path, err)
		}
		if info.IsDir() {
			return fmt.Errorf("config: target %q is a directory, not a file", f.Path)
		}
		// Named pipes, devices, and sockets cannot hold a version token, and
		// opening one would block or stream without end. Rejecting them here
		// names the config entry at fault instead of surfacing the problem
		// later as a read failure.
		if !info.Mode().IsRegular() {
			return fmt.Errorf("config: target %q is not a regular file", f.Path)
		}
	}

	return nil
}

// NotExistError is returned by Load when the config file is missing. It carries
// the resolved path and is detectable with errors.As or IsNotExist.
type NotExistError struct {
	Path string
}

func (e *NotExistError) Error() string {
	return fmt.Sprintf("config: %q not found; run `incrmit discover` to generate one", e.Path)
}

// IsNotExist reports whether err indicates a missing config file.
func IsNotExist(err error) bool {
	var nee *NotExistError
	return errors.As(err, &nee)
}
