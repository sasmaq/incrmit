package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/files"
	"github.com/sasmaq/incrmit/internal/testutil"
)

// Load and LoadIgnore read the config through files.ReadOwnFile, so a config
// that is a named pipe is reported rather than opened. LoadIgnore's leniency
// is for a stale or foreign config, not for this: discover would go on to
// replace the pipe.
func TestLoadRejectsFIFOConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	if err := testutil.Mkfifo(path); err != nil {
		t.Skipf("FIFOs unsupported: %v", err)
	}

	testutil.Within(t, "Load on a FIFO", func() {
		_, err := Load(path)
		if err == nil || !strings.Contains(err.Error(), "not a regular file") || !strings.Contains(err.Error(), path) {
			t.Errorf("Load err = %v, want one naming %s as not a regular file", err, path)
		}
		if IsNotExist(err) {
			t.Errorf("IsNotExist(%v) = true, want false", err)
		}
	})
	testutil.Within(t, "LoadIgnore on a FIFO", func() {
		got, err := LoadIgnore(path)
		if !errors.Is(err, files.ErrNotRegular) {
			t.Errorf("LoadIgnore err = %v, want files.ErrNotRegular", err)
		}
		if got != nil {
			t.Errorf("patterns = %v, want nil alongside the error", got)
		}
	})
}

// A config over files.MaxOwnFileBytes is refused before it is read, by both
// loaders, with an error that names it.
func TestLoadRejectsOversizedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), DefaultPath)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(path, files.MaxOwnFileBytes+1); err != nil {
		t.Fatal(err)
	}

	_, loadErr := Load(path)
	patterns, ignoreErr := LoadIgnore(path)
	for name, err := range map[string]error{"Load": loadErr, "LoadIgnore": ignoreErr} {
		var tooLarge *files.TooLargeError
		if !errors.As(err, &tooLarge) {
			t.Errorf("%s err = %v, want a *files.TooLargeError", name, err)
			continue
		}
		if !strings.Contains(err.Error(), path) {
			t.Errorf("%s err = %v, want it to name %s", name, err, path)
		}
	}
	if patterns != nil {
		t.Errorf("patterns = %v, want nil alongside the error", patterns)
	}
}

// A symlinked config is still read, by both loaders. Its entries resolve
// against the directory holding the link, which here is also the real file's.
func TestLoadFollowsSymlinkedConfig(t *testing.T) {
	dir := t.TempDir()
	body := "ignore = [\"docs/\"]\n\n[[files]]\npath = \"VERSION\"\n"
	for name, content := range map[string]string{"VERSION": "1.0.0\n", "shared.toml": body} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(dir, DefaultPath)
	if err := os.Symlink("shared.toml", link); err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}

	cfg, err := Load(link)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Files) != 1 || cfg.Files[0].Path != "VERSION" {
		t.Errorf("files = %+v, want the one VERSION entry", cfg.Files)
	}
	if got, err := LoadIgnore(link); err != nil || len(got) != 1 || got[0] != "docs/" {
		t.Errorf("LoadIgnore = (%v, %v), want ([docs/], nil)", got, err)
	}
}
