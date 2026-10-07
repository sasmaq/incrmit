package buildinfo

import (
	"runtime/debug"
	"strings"
	"testing"
)

func TestVersionNonEmpty(t *testing.T) {
	if got := Version(); got == "" {
		t.Error("Version() returned empty string")
	}
}

func TestStringIsToolNameAndVersionOnly(t *testing.T) {
	s := String()
	want := "incrmit " + Version()
	if s != want {
		t.Errorf("String() = %q, want exactly %q", s, want)
	}
	// No build metadata (commit/build date) should be appended.
	if strings.ContainsAny(s, "()") {
		t.Errorf("String() = %q, want no build-metadata suffix", s)
	}
}

func TestVersionPrefersLdflagsValue(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })

	version = "9.9.9"
	if got := Version(); got != "9.9.9" {
		t.Errorf("Version() = %q, want %q", got, "9.9.9")
	}
	if got := String(); got != "incrmit 9.9.9" {
		t.Errorf("String() = %q, want %q", got, "incrmit 9.9.9")
	}
}

func TestVersionFallsBackToDev(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })

	// An empty injected value should fall back (to module version or "dev"),
	// never to the empty string.
	version = ""
	if got := Version(); got == "" {
		t.Error("Version() returned empty string on fallback")
	}
}

// With no injected value, a binary built by `go install module@v1.4.0` reports
// the module version the toolchain recorded. A local build records "(devel)"
// or nothing, and a binary without build info has neither; all of those fall
// through to "dev".
func TestVersionFallsBackToModuleVersion(t *testing.T) {
	origVersion, origRead := version, readBuildInfo
	t.Cleanup(func() { version, readBuildInfo = origVersion, origRead })
	version = ""

	tests := []struct {
		name    string
		mainVer string
		ok      bool
		want    string
	}{
		{"go-install", "v1.4.0", true, "v1.4.0"},
		{"devel", "(devel)", true, "dev"},
		{"empty", "", true, "dev"},
		{"no-build-info", "", false, "dev"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readBuildInfo = func() (*debug.BuildInfo, bool) {
				if !tt.ok {
					return nil, false
				}
				return &debug.BuildInfo{Main: debug.Module{Version: tt.mainVer}}, true
			}
			if got := Version(); got != tt.want {
				t.Errorf("Version() = %q, want %q", got, tt.want)
			}
		})
	}
}
