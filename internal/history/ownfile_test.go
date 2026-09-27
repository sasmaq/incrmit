package history

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sasmaq/incrmit/internal/files"
	"github.com/sasmaq/incrmit/internal/testutil"
)

// A state file that is a named pipe is an error, not a hang and not an empty
// history: "nothing to undo" would strand the bump it replaced.
func TestLoadRejectsFIFOJournal(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".incrmit.state.toml")
	if err := testutil.Mkfifo(path); err != nil {
		t.Skipf("FIFOs unsupported: %v", err)
	}
	testutil.Within(t, "Load on a FIFO", func() {
		h, err := Load(path)
		if !errors.Is(err, files.ErrNotRegular) {
			t.Errorf("err = %v, want files.ErrNotRegular", err)
		}
		if err != nil && !strings.Contains(err.Error(), path) {
			t.Errorf("err = %v, want it to name %s", err, path)
		}
		if h != nil {
			t.Errorf("history = %+v, want nil alongside the error", h)
		}
	})
}

// A state file over files.MaxOwnFileBytes is refused before it is read.
func TestLoadRejectsOversizedJournal(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".incrmit.state.toml")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, files.MaxOwnFileBytes+1); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	var tooLarge *files.TooLargeError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("err = %v, want a *files.TooLargeError", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("err = %v, want it to name %s", err, path)
	}
}

// journalOf returns a history of n entries of the same encoded size, oldest
// first, each naming its own file so the survivors can be told apart.
func journalOf(n int) *History {
	h := &History{}
	start := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	for i := range n {
		p := fmt.Sprintf("file-%02d", i)
		h.Push(Entry{
			Timestamp: start.Add(time.Duration(i) * time.Minute),
			Changes:   []Change{{Path: p, Old: "1.0.0", New: "1.0.1"}},
		})
	}
	return h
}

// A journal over the cap loses its oldest entries, never its newest, so what
// is written is always something Load reads back. The caller's history is left
// as it was.
func TestSaveDropsOldestEntriesToFitTheCap(t *testing.T) {
	h := journalOf(5)
	fits, err := encode(h.Entries[2:])
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), ".incrmit.state.toml")

	if err := saveWithin(path, h, len(fits)); err != nil {
		t.Fatalf("saveWithin: %v", err)
	}
	if info, err := os.Stat(path); err != nil || info.Size() > int64(len(fits)) {
		t.Fatalf("stat = %v, %v; want a journal of at most %d bytes", info, err, len(fits))
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var paths []string
	for _, e := range got.Entries {
		paths = append(paths, e.Changes[0].Path)
	}
	if want := "file-02 file-03 file-04"; strings.Join(paths, " ") != want {
		t.Errorf("kept %v, want the newest three (%s)", paths, want)
	}
	if len(h.Entries) != 5 {
		t.Errorf("history has %d entries after Save, want its 5 left alone", len(h.Entries))
	}
}

// A newest entry that is over the cap by itself cannot be kept, so Save
// refuses rather than write a journal Load would refuse, and the journal
// already on disk is left as it was.
func TestSaveRefusesAnEntryOverTheCap(t *testing.T) {
	h := journalOf(2)
	alone, err := encode(h.Entries[1:])
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), ".incrmit.state.toml")
	const previous = "# an earlier journal\n"
	if err := os.WriteFile(path, []byte(previous), 0o644); err != nil {
		t.Fatal(err)
	}

	err = saveWithin(path, h, len(alone)-1)
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("err = %v, want one saying the entry is over the limit", err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != previous {
		t.Errorf("journal = %q (%v), want the earlier one left alone", got, err)
	}
}
