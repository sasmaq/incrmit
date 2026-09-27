package testutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// Within returns as soon as f does, having run it.
func TestWithinRunsF(t *testing.T) {
	ran := false
	Within(t, "a function that returns", func() { ran = true })
	if !ran {
		t.Error("Within returned without running f")
	}
}

// Mkfifo makes a named pipe, or says it cannot so the caller can skip.
func TestMkfifo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pipe")
	if err := Mkfifo(path); err != nil {
		t.Skipf("FIFOs unsupported: %v", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Type() != fs.ModeNamedPipe {
		t.Errorf("mode = %v, want a named pipe", info.Mode())
	}
}
