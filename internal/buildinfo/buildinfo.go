// Package buildinfo exposes the incrmit tool's own version and build metadata.
//
// version holds the baked-in release version. It can be overridden at build
// time with a linker flag (for example to stamp a git tag):
//
//	go build -ldflags "-X github.com/sasmaq/incrmit/internal/buildinfo.version=1.2.3"
//
// If the injected value is empty, it falls back to the module version recorded
// by the Go toolchain (e.g. for `go install module@v1.2.3`), then to "dev".
package buildinfo

import (
	"runtime/debug"
)

// version is the current release version. It may be overridden via -ldflags at
// build time; see the package doc.
var version = "0.1.0"

// Version returns the resolved tool version.
func Version() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}

// String returns a human-readable version line, including VCS commit and build
// time when the Go toolchain embedded them in the binary.
func String() string {
	s := "incrmit " + Version()

	revision, buildTime, modified := vcsInfo()
	if revision == "" {
		return s
	}

	short := revision
	if len(short) > 12 {
		short = short[:12]
	}
	if modified {
		short += "-dirty"
	}
	s += " (commit " + short
	if buildTime != "" {
		s += ", built " + buildTime
	}
	s += ")"
	return s
}

// vcsInfo extracts the VCS revision, build time, and dirty flag from the
// embedded build settings, if present.
func vcsInfo() (revision, buildTime string, modified bool) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "", "", false
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			buildTime = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	return revision, buildTime, modified
}
