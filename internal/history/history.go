// Package history records a journal of bumps so they can be reverted.
//
// After a successful config-driven bump, incrmit appends an entry capturing
// every file it rewrote (the old and new version tokens) and a timestamp to a
// state file kept next to the config (config.StateFileName). The `undo` command
// reads the most recent entry and reverts it, then pops the entry so repeated
// undos walk back through history rather than re-applying the same revert.
//
// The state file is local, machine-specific working state, not something meant
// to be committed: undo restores files in the working copy, so the journal
// belongs alongside them (git-ignore it). Only the most recent MaxEntries bumps
// are retained so the file cannot grow without bound; at minimum the last bump
// is always available to undo.
package history

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/files"
)

// MaxEntries bounds how many bump entries the journal retains. The most recent
// entries are kept and older ones are discarded once the limit is exceeded, so
// the state file stays small while still supporting several successive undos.
const MaxEntries = 20

// Change is a single version-token rewrite within one file during a bump: the
// display path (as listed in the config), the resolved filesystem path used to
// locate the file on undo, and the old and new version tokens.
type Change struct {
	Path string `toml:"path"`
	FS   string `toml:"fs"`
	Old  string `toml:"old"`
	New  string `toml:"new"`
}

// Entry is one recorded bump: when it happened, the resolved path of the config
// that was rewritten (so undo can restore it), and every file change applied.
type Entry struct {
	Timestamp time.Time `toml:"timestamp"`
	Config    string    `toml:"config,omitempty"`
	Changes   []Change  `toml:"changes"`
}

// History is the on-disk journal: a stack of bump entries, oldest first, newest
// last.
type History struct {
	Entries []Entry `toml:"entries"`
}

// ResolvePath returns the state file path for a given config path. The state
// file lives in the same directory as the config so the two are always found
// together; an empty configPath resolves to the current directory.
func ResolvePath(configPath string) string {
	dir := "."
	if configPath != "" {
		dir = filepath.Dir(configPath)
	}
	return filepath.Join(dir, config.StateFileName)
}

// Load reads the journal at path. A missing state file is not an error: it
// yields an empty history so a first `undo` has a well-defined "nothing to
// undo" result.
func Load(path string) (*History, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &History{}, nil
		}
		return nil, fmt.Errorf("history: reading %q: %w", path, err)
	}
	var h History
	if err := toml.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("history: parsing %q: %w", path, err)
	}
	return &h, nil
}

// Save writes the journal to path atomically, reusing the same write path as
// bumped files so a crash never leaves a half-written journal. A short header
// documents that the file is tool-maintained local state.
func Save(path string, h *History) error {
	var buf bytes.Buffer
	buf.WriteString("# incrmit bump history (maintained by incrmit; safe to delete).\n")
	buf.WriteString("# Records recent bumps so `incrmit undo` can revert them. This is local\n")
	buf.WriteString("# working state, not meant to be committed — add it to .gitignore.\n\n")
	if err := toml.NewEncoder(&buf).Encode(h); err != nil {
		return fmt.Errorf("history: encoding: %w", err)
	}
	return files.WriteAtomic(path, buf.Bytes())
}

// Push appends e as the newest entry, trimming the journal to the most recent
// MaxEntries so the state file cannot grow without bound.
func (h *History) Push(e Entry) {
	h.Entries = append(h.Entries, e)
	if len(h.Entries) > MaxEntries {
		h.Entries = h.Entries[len(h.Entries)-MaxEntries:]
	}
}

// Latest returns the most recent entry and true, or a zero Entry and false when
// the journal is empty. It does not modify the journal.
func (h *History) Latest() (Entry, bool) {
	if len(h.Entries) == 0 {
		return Entry{}, false
	}
	return h.Entries[len(h.Entries)-1], true
}

// Pop removes and returns the most recent entry. The second result is false
// when the journal is already empty.
func (h *History) Pop() (Entry, bool) {
	if len(h.Entries) == 0 {
		return Entry{}, false
	}
	last := h.Entries[len(h.Entries)-1]
	h.Entries = h.Entries[:len(h.Entries)-1]
	return last, true
}
