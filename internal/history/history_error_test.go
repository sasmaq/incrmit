package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A missing journal is deliberately not an error (see TestLoadMissingIsEmpty),
// but every other read failure must surface: silently returning an empty
// history would report "nothing to undo" and strand a recorded bump.
func TestLoadCorruptTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.toml")
	if err := os.WriteFile(path, []byte("entries = [ this is not toml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	h, err := Load(path)
	if err == nil {
		t.Fatalf("Load succeeded on a corrupt journal (entries = %v)", h.Entries)
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("err = %v, want it to name the parse step", err)
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("err = %v, want it to name the journal path", err)
	}
}

func TestLoadUnreadableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions are not enforced")
	}
	path := filepath.Join(t.TempDir(), "state.toml")
	if err := os.WriteFile(path, []byte("entries = []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	if _, err := Load(path); err == nil {
		t.Fatal("Load succeeded on an unreadable journal")
	} else if !strings.Contains(err.Error(), "reading") {
		t.Errorf("err = %v, want it to name the read step", err)
	}
}

func TestSaveToMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-dir", "state.toml")
	if err := Save(path, &History{}); err == nil {
		t.Fatal("Save succeeded into a missing directory")
	}
}

// Push keeps the newest MaxEntries, so a journal at the limit still has the
// most recent bump available to undo after another one is recorded.
func TestPushAtLimitKeepsNewest(t *testing.T) {
	h := &History{}
	for i := 0; i < MaxEntries+5; i++ {
		h.Push(Entry{Config: string(rune('a' + i%26)), Changes: []Change{{New: "1.0." + string(rune('0'+i%10))}}})
	}
	if len(h.Entries) != MaxEntries {
		t.Fatalf("len(Entries) = %d, want %d", len(h.Entries), MaxEntries)
	}
	last, ok := h.Latest()
	if !ok {
		t.Fatal("Latest() reported an empty journal")
	}
	want := MaxEntries + 4
	if got := last.Config; got != string(rune('a'+want%26)) {
		t.Errorf("newest entry = %q, want the last one pushed", got)
	}
}
