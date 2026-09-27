package files

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sasmaq/incrmit/internal/testutil"
)

// sparseFile creates a file of size bytes without writing them, so a test can
// hold a file at the size cap without spending the disk.
func sparseFile(t *testing.T, path string, size int64) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, size); err != nil {
		t.Fatal(err)
	}
}

// incrmit's own files get the type check a target gets, so a config or a
// journal that is a named pipe is refused rather than opened.
func TestReadOwnFileRejectsFIFO(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "incrmit.toml")
	if err := testutil.Mkfifo(fifo); err != nil {
		t.Skipf("FIFOs unsupported: %v", err)
	}
	testutil.Within(t, "ReadOwnFile on a FIFO", func() {
		if _, err := ReadOwnFile(fifo); !errors.Is(err, ErrNotRegular) {
			t.Errorf("err = %v, want ErrNotRegular", err)
		}
	})
}

// A repository cannot commit a FIFO, but it can commit a link to a device. The
// check follows the link and refuses what it finds.
func TestReadOwnFileRejectsLinkToDevice(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no /dev/zero on Windows")
	}
	if _, err := os.Stat("/dev/zero"); err != nil {
		t.Skipf("no /dev/zero here: %v", err)
	}
	link := filepath.Join(t.TempDir(), "incrmit.toml")
	if err := os.Symlink("/dev/zero", link); err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}
	testutil.Within(t, "ReadOwnFile on a link to /dev/zero", func() {
		if _, err := ReadOwnFile(link); !errors.Is(err, ErrNotRegular) {
			t.Errorf("err = %v, want ErrNotRegular", err)
		}
	})
}

// A link to a regular file is followed, as it is for a target: a symlinked
// config is a legitimate setup.
func TestReadOwnFileFollowsSymlinkToRegularFile(t *testing.T) {
	dir := t.TempDir()
	shared := filepath.Join(dir, "shared.toml")
	if err := os.WriteFile(shared, []byte("ignore = []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "incrmit.toml")
	if err := os.Symlink("shared.toml", link); err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}
	got, err := ReadOwnFile(link)
	if err != nil {
		t.Fatalf("ReadOwnFile: %v", err)
	}
	if string(got) != "ignore = []\n" {
		t.Errorf("content = %q, want the linked file's", got)
	}
}

// The cap is MaxOwnFileBytes whatever --max-file-size says: a file at the cap
// is read whole, and one byte more is refused with the cap as the limit.
func TestReadOwnFileCap(t *testing.T) {
	dir := t.TempDir()
	at := filepath.Join(dir, "at-the-cap")
	over := filepath.Join(dir, "over-the-cap")
	sparseFile(t, at, MaxOwnFileBytes)
	sparseFile(t, over, MaxOwnFileBytes+1)

	if got, err := ReadOwnFile(at); err != nil || len(got) != MaxOwnFileBytes {
		t.Errorf("ReadOwnFile(at the cap) = %d bytes, %v; want the whole %d bytes", len(got), err, MaxOwnFileBytes)
	}

	got, err := ReadOwnFile(over)
	var tooLarge *TooLargeError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("err = %v, want a *TooLargeError", err)
	}
	if got != nil {
		t.Errorf("read %d bytes, want nothing", len(got))
	}
	if tooLarge.Size != MaxOwnFileBytes+1 || tooLarge.Limit != MaxOwnFileBytes {
		t.Errorf("error = %+v, want size %d and limit %d", tooLarge, MaxOwnFileBytes+1, MaxOwnFileBytes)
	}
}
