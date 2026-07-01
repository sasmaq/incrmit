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
var version = "0.1.10"

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

// String returns a human-readable version line, e.g. "incrmit 0.1.0".
func String() string {
	return "incrmit " + Version()
}
