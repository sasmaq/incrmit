package files

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/version"
)

// The two error types carry text a user reads on stderr, so their wording is
// part of the contract rather than an implementation detail.

func TestAmbiguousErrorMessage(t *testing.T) {
	err := &AmbiguousError{Versions: []string{"1.2.3", "2.0.0"}}
	msg := err.Error()
	if !strings.Contains(msg, "ambiguous") {
		t.Errorf("message %q does not say the version is ambiguous", msg)
	}
	for _, v := range []string{"1.2.3", "2.0.0"} {
		if !strings.Contains(msg, v) {
			t.Errorf("message %q does not name the conflicting version %s", msg, v)
		}
	}
}

func TestTooLargeErrorMessage(t *testing.T) {
	err := &TooLargeError{Size: 2048, Limit: 1024}
	msg := err.Error()
	if !strings.Contains(msg, "2048") || !strings.Contains(msg, "1024") {
		t.Errorf("message %q should report both the size and the limit", msg)
	}
}

// SetVersion locates the current version itself, so it inherits FindVersion's
// failure modes rather than reporting a generic write error.
func TestSetVersionPropagatesFindVersionErrors(t *testing.T) {
	newVer := version.Version{Major: 9, Minor: 9, Patch: 9}

	t.Run("no version", func(t *testing.T) {
		if _, err := SetVersion([]byte("nothing here\n"), newVer); !errors.Is(err, ErrNoVersion) {
			t.Errorf("err = %v, want ErrNoVersion", err)
		}
	})

	t.Run("ambiguous", func(t *testing.T) {
		var ambiguous *AmbiguousError
		_, err := SetVersion([]byte("1.2.3 and 4.5.6\n"), newVer)
		if !errors.As(err, &ambiguous) {
			t.Fatalf("err = %v, want *AmbiguousError", err)
		}
		if len(ambiguous.Versions) != 2 {
			t.Errorf("Versions = %v, want both versions listed", ambiguous.Versions)
		}
	})
}

func TestReadTargetWithLimitMissingFile(t *testing.T) {
	_, err := ReadTargetWithLimit(filepath.Join(t.TempDir(), "absent"), 1024)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want a not-exist error", err)
	}
}

// The size cap is checked before the file is opened, but an unreadable file
// still has to surface as a permission error rather than empty contents.
func TestReadTargetWithLimitUnreadableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions are not enforced")
	}
	path := filepath.Join(t.TempDir(), "VERSION")
	if err := os.WriteFile(path, []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	if _, err := ReadTargetWithLimit(path, 1024); !errors.Is(err, fs.ErrPermission) {
		t.Errorf("err = %v, want a permission error", err)
	}
}

// A file created by WriteAtomic (rather than replaced) has no previous mode to
// preserve, so it must land on the documented 0644 default and never inherit
// the 0600 the temp file holds while it is being written.
func TestWriteAtomicNewFileUsesDefaultMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "created")
	if err := WriteAtomic(path, []byte("1.2.3\n")); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Compare against the umask-adjusted expectation: the mode is set
	// explicitly through the descriptor, so the umask does not apply.
	if got := info.Mode().Perm(); got != 0o644 {
		t.Errorf("mode = %04o, want 0644", got)
	}
}

func TestWriteAtomicMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-dir", "VERSION")
	err := WriteAtomic(path, []byte("1.2.3\n"))
	if err == nil {
		t.Fatal("WriteAtomic succeeded into a missing directory")
	}
	if !strings.Contains(err.Error(), "creating temp file") {
		t.Errorf("err = %v, want it to name the temp-file step", err)
	}
}

func TestWriteAtomicUnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions are not enforced")
	}
	dir := t.TempDir()
	sub := filepath.Join(dir, "ro")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(sub, "VERSION")
	if err := os.WriteFile(target, []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })

	if err := WriteAtomic(target, []byte("9.9.9\n")); err == nil {
		t.Fatal("WriteAtomic succeeded in a read-only directory")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "1.2.3\n" {
		t.Errorf("target changed despite the failure: %q", got)
	}
}

// When the rename cannot happen the deferred cleanup must remove the temp file,
// so a failed write never litters the target's directory. Renaming a file over
// an existing directory is the reliable way to fail at exactly that step, with
// the temp file already created and written.
func TestWriteAtomicRemovesTempFileWhenRenameFails(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "occupied")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := WriteAtomic(target, []byte("1.2.3\n")); err == nil {
		t.Fatal("WriteAtomic succeeded onto a directory path")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".incrmit-") {
			t.Errorf("temp file %q left behind after a failed write", e.Name())
		}
	}
}
