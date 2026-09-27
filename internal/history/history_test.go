package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sasmaq/incrmit/internal/config"
)

func TestResolvePath(t *testing.T) {
	tests := []struct {
		configPath string
		want       string
	}{
		{"", filepath.Join(".", config.StateFileName)},
		{"incrmit.toml", filepath.Join(".", config.StateFileName)},
		{"a/b/incrmit.toml", filepath.Join("a/b", config.StateFileName)},
		{"/abs/dir/custom.toml", filepath.Join("/abs/dir", config.StateFileName)},
	}
	for _, tt := range tests {
		if got := ResolvePath(tt.configPath); got != tt.want {
			t.Errorf("ResolvePath(%q) = %q, want %q", tt.configPath, got, tt.want)
		}
	}
}

// A missing state file loads as an empty history with no error, so a first undo
// has a well-defined "nothing to undo" result.
func TestLoadMissingIsEmpty(t *testing.T) {
	h, err := Load(filepath.Join(t.TempDir(), "absent.toml"))
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if _, ok := h.Latest(); ok {
		t.Errorf("missing file yielded a non-empty history: %+v", h)
	}
}

// labeled returns an entry told apart from others by its one change's path.
func labeled(name string) Entry {
	return Entry{Changes: []Change{{Path: name}}}
}

// label returns the name an entry was labeled with.
func label(e Entry) string {
	return e.Changes[0].Path
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.StateFileName)
	ts := time.Date(2026, 7, 10, 1, 2, 3, 0, time.UTC)
	want := Entry{
		Timestamp: ts,
		Changes: []Change{
			{Path: "VERSION", Old: "1.2.3", New: "1.3.0"},
			{Path: "../shared/notes.md", Old: "2.0.0", New: "2.1.0"},
		},
	}
	h := &History{}
	h.Push(want)
	if err := Save(path, h); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	e, ok := got.Latest()
	if !ok {
		t.Fatal("Latest after round-trip: no entry")
	}
	if !e.Timestamp.Equal(ts) {
		t.Errorf("timestamp = %v, want %v", e.Timestamp, ts)
	}
	if len(e.Changes) != len(want.Changes) {
		t.Fatalf("changes = %d, want %d", len(e.Changes), len(want.Changes))
	}
	for i, c := range e.Changes {
		if c != want.Changes[i] {
			t.Errorf("change[%d] = %+v, want %+v", i, c, want.Changes[i])
		}
	}
}

// Entries written before Milestone 34 also carry the config's absolute path
// (`config`) and each file's (`fs`). They still load, with both keys ignored,
// and the next save leaves them out.
func TestLoadIgnoresRetiredKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), config.StateFileName)
	old := "[[entries]]\n  timestamp = 2026-07-10T01:02:03Z\n  config = \"/old/place/incrmit.toml\"\n\n" +
		"  [[entries.changes]]\n    path = \"VERSION\"\n    fs = \"/old/place/VERSION\"\n    old = \"1.2.3\"\n    new = \"1.3.0\"\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	h, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	e, ok := h.Latest()
	if !ok || len(e.Changes) != 1 {
		t.Fatalf("entries = %+v, want one entry with one change", h.Entries)
	}
	if want := (Change{Path: "VERSION", Old: "1.2.3", New: "1.3.0"}); e.Changes[0] != want {
		t.Errorf("change = %+v, want %+v", e.Changes[0], want)
	}

	if err := Save(path, h); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "/old/place") {
		t.Errorf("the saved journal still records the old location:\n%s", data)
	}
}

// Save writes atomically through the files package, so the written state file
// carries a documenting header and no temp files are left behind.
func TestSaveWritesHeader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, config.StateFileName)
	h := &History{}
	h.Push(Entry{Timestamp: time.Now().UTC(), Changes: []Change{{Path: "V", Old: "1.0.0", New: "1.0.1"}}})
	if err := Save(path, h); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading state: %v", err)
	}
	if got := string(data); !strings.Contains(got, "maintained by incrmit") || !strings.Contains(got, ".gitignore") {
		t.Errorf("state file missing header: %q", got)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != config.StateFileName {
			t.Errorf("unexpected leftover file %q", e.Name())
		}
	}
}

func TestPushTrimsToMaxEntries(t *testing.T) {
	h := &History{}
	for i := 0; i < MaxEntries+5; i++ {
		h.Push(labeled(string(rune('a' + i))))
	}
	if len(h.Entries) != MaxEntries {
		t.Fatalf("entries = %d, want %d", len(h.Entries), MaxEntries)
	}
	// The newest push must be retained; the oldest must have been dropped.
	if last, _ := h.Latest(); label(last) != string(rune('a'+MaxEntries+4)) {
		t.Errorf("newest entry = %q, want %q", label(last), string(rune('a'+MaxEntries+4)))
	}
}

func TestPopReturnsNewestAndShrinks(t *testing.T) {
	h := &History{}
	h.Push(labeled("first"))
	h.Push(labeled("second"))

	e, ok := h.Pop()
	if !ok || label(e) != "second" {
		t.Fatalf("Pop = %+v (ok %v), want second", e, ok)
	}
	if len(h.Entries) != 1 {
		t.Fatalf("entries after pop = %d, want 1", len(h.Entries))
	}
	if e, ok := h.Pop(); !ok || label(e) != "first" {
		t.Fatalf("second Pop = %+v (ok %v), want first", e, ok)
	}
	if _, ok := h.Pop(); ok {
		t.Errorf("Pop on empty history returned ok")
	}
}

func TestLatestEmpty(t *testing.T) {
	h := &History{}
	if _, ok := h.Latest(); ok {
		t.Errorf("Latest on empty history returned ok")
	}
}
