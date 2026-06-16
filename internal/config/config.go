// Package config loads and validates the TOML configuration that lists the
// target files incrmit reads and updates.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// DefaultPath is the config file name resolved when no explicit path is given.
const DefaultPath = "incrmit.toml"

// Config is the in-memory model of an incrmit.toml file.
type Config struct {
	Files []FileEntry `toml:"files"`
}

// FileEntry is a single target file listed in the config. Version is optional
// and is populated by the discover command.
type FileEntry struct {
	Path    string `toml:"path"`
	Version string `toml:"version,omitempty"`
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

	if err := cfg.Validate(filepath.Dir(path)); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate checks that the config is usable: it must list at least one file,
// each entry must have a non-empty path, paths must be unique, and every target
// must exist on disk. Relative target paths are resolved against baseDir (the
// directory containing the config file).
func (c *Config) Validate(baseDir string) error {
	if len(c.Files) == 0 {
		return errors.New("config: no files listed; add at least one [[files]] entry")
	}

	seen := make(map[string]struct{}, len(c.Files))
	for i, f := range c.Files {
		if f.Path == "" {
			return fmt.Errorf("config: files[%d] has an empty path", i)
		}
		if _, dup := seen[f.Path]; dup {
			return fmt.Errorf("config: duplicate path %q", f.Path)
		}
		seen[f.Path] = struct{}{}

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
