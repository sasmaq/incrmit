// Package cli implements the incrmit command-line interface: flag parsing,
// command dispatch, and the bump workflow. It is kept separate from package
// main so it can be exercised directly in tests.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/files"
	"github.com/sasmaq/incrmit/internal/version"
)

// fprintf and fprintln write to w, ignoring write errors. Output to stdout or
// stderr is best-effort: there is nothing useful to do if it fails, and these
// wrappers keep that decision in one place rather than scattering _, _ = ...
// across every call site.
func fprintf(w io.Writer, format string, a ...any) {
	_, _ = fmt.Fprintf(w, format, a...)
}

func fprintln(w io.Writer, a ...any) {
	_, _ = fmt.Fprintln(w, a...)
}

// Exit codes, per doc/DEVELOPMENT.md section 10.
const (
	ExitOK         = 0  // success
	ExitError      = 1  // generic runtime error
	ExitUsage      = 2  // invalid arguments or flags
	ExitNoVersion  = 3  // no version found / parse failure
	exitSilentFlag = -1 // flag package already printed help/usage
)

// Main is the entry point used by package main. It parses args, runs the
// selected command, and returns a process exit code. All human-readable output
// goes to stdout; errors go to stderr.
func Main(args []string, stdout, stderr io.Writer) int {
	// The discover subcommand is added in a later milestone; dispatch is
	// structured so it can slot in here.
	if len(args) > 0 && args[0] == "discover" {
		fprintln(stderr, "incrmit: discover is not implemented yet")
		return ExitError
	}
	return runBump(args, stdout, stderr)
}

type bumpOptions struct {
	configPath string
	file       string
	major      bool
	minor      bool
	patch      bool
	dryRun     bool
}

func runBump(args []string, stdout, stderr io.Writer) int {
	opts, code := parseBumpFlags(args, stderr)
	if code != exitSilentFlag && code != ExitOK {
		return code
	}
	if code == exitSilentFlag {
		return ExitOK
	}

	bump, label := resolveBump(opts.major, opts.minor, opts.patch)

	targets, err := resolveTargets(opts)
	if err != nil {
		fprintln(stderr, "incrmit:", err)
		return classify(err)
	}

	// Phase 1: read and plan every target before writing anything, so a
	// failure on one file does not leave others half-updated (fail fast).
	type plan struct {
		display string
		fsPath  string
		data    []byte
		oldVer  version.Version
		newVer  version.Version
	}
	plans := make([]plan, 0, len(targets))
	for _, tgt := range targets {
		data, err := os.ReadFile(tgt.fsPath)
		if err != nil {
			fprintf(stderr, "incrmit: reading %s: %v\n", tgt.display, err)
			return classify(err)
		}
		oldVer, err := files.FindVersion(data)
		if err != nil {
			fprintf(stderr, "incrmit: %s: %v\n", tgt.display, err)
			return classify(err)
		}
		plans = append(plans, plan{
			display: tgt.display,
			fsPath:  tgt.fsPath,
			data:    data,
			oldVer:  oldVer,
			newVer:  bump(oldVer),
		})
	}

	if opts.dryRun {
		fprintf(stdout, "Dry run: would apply a %s bump (no files changed)\n", label)
		for _, p := range plans {
			fprintf(stdout, "  %s: %s -> %s\n", p.display, p.oldVer, p.newVer)
		}
		return ExitOK
	}

	// Phase 2: write each planned change atomically.
	for _, p := range plans {
		updated, err := files.SetVersion(p.data, p.newVer)
		if err != nil {
			fprintf(stderr, "incrmit: %s: %v\n", p.display, err)
			return classify(err)
		}
		if err := files.WriteAtomic(p.fsPath, updated); err != nil {
			fprintf(stderr, "incrmit: %s: %v\n", p.display, err)
			return classify(err)
		}
	}

	fprintf(stdout, "Applied a %s bump to %d file(s):\n", label, len(plans))
	for _, p := range plans {
		fprintf(stdout, "  %s: %s -> %s\n", p.display, p.oldVer, p.newVer)
	}
	return ExitOK
}

// parseBumpFlags registers the bump flags (each with a long and short name)
// and parses args. It returns the populated options and an exit code: ExitOK to
// proceed, ExitUsage on a parse error, or exitSilentFlag when -h/-help was
// handled by the flag package.
func parseBumpFlags(args []string, stderr io.Writer) (bumpOptions, int) {
	var opts bumpOptions
	fs := flag.NewFlagSet("incrmit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fprintln(stderr, "usage: incrmit [flags]")
		fprintln(stderr, "\nBump the semantic version in the configured files.")
		fprintln(stderr, "\nFlags:")
		fprintln(stderr, "  -c, --config string  path to the TOML config file (default \"incrmit.toml\")")
		fprintln(stderr, "  -f, --file string    bump the version in one file (skips config)")
		fprintln(stderr, "  -M, --major          bump the major version (resets minor and patch)")
		fprintln(stderr, "  -m, --minor          bump the minor version (resets patch)")
		fprintln(stderr, "  -p, --patch          bump the patch version (default)")
		fprintln(stderr, "  -d, --dry-run        print the new version without writing")
	}

	fs.StringVar(&opts.configPath, "config", "", "path to the TOML config file")
	fs.StringVar(&opts.configPath, "c", "", "path to the TOML config file (shorthand)")
	fs.StringVar(&opts.file, "file", "", "bump the version in one file (skips config)")
	fs.StringVar(&opts.file, "f", "", "bump the version in one file (shorthand)")
	fs.BoolVar(&opts.major, "major", false, "bump the major version")
	fs.BoolVar(&opts.major, "M", false, "bump the major version (shorthand)")
	fs.BoolVar(&opts.minor, "minor", false, "bump the minor version")
	fs.BoolVar(&opts.minor, "m", false, "bump the minor version (shorthand)")
	fs.BoolVar(&opts.patch, "patch", true, "bump the patch version")
	fs.BoolVar(&opts.patch, "p", true, "bump the patch version (shorthand)")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "print the new version without writing")
	fs.BoolVar(&opts.dryRun, "d", false, "print the new version without writing (shorthand)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return opts, exitSilentFlag
		}
		return opts, ExitUsage
	}
	if fs.NArg() > 0 {
		fprintf(stderr, "incrmit: unexpected argument %q\n", fs.Arg(0))
		fs.Usage()
		return opts, ExitUsage
	}
	return opts, ExitOK
}

// resolveBump picks the bump transform. If more than one of major/minor/patch
// is requested, the highest component wins (major > minor > patch). When none
// is requested, patch is used.
func resolveBump(major, minor, patch bool) (func(version.Version) version.Version, string) {
	switch {
	case major:
		return version.Version.BumpMajor, "major"
	case minor:
		return version.Version.BumpMinor, "minor"
	default:
		_ = patch
		return version.Version.BumpPatch, "patch"
	}
}

type target struct {
	display string // path as the user/config sees it
	fsPath  string // resolved filesystem path
}

// resolveTargets returns the files to bump: either the single --file target or
// every entry in the config. Config-relative paths are resolved against the
// directory containing the config file.
func resolveTargets(opts bumpOptions) ([]target, error) {
	if opts.file != "" {
		return []target{{display: opts.file, fsPath: opts.file}}, nil
	}

	cfgPath := config.ResolvePath(opts.configPath)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}

	baseDir := filepath.Dir(cfgPath)
	targets := make([]target, 0, len(cfg.Files))
	for _, f := range cfg.Files {
		fsPath := f.Path
		if !filepath.IsAbs(fsPath) {
			fsPath = filepath.Join(baseDir, fsPath)
		}
		targets = append(targets, target{display: f.Path, fsPath: fsPath})
	}
	return targets, nil
}

// classify maps an error to the appropriate process exit code.
func classify(err error) int {
	var ambiguous *files.AmbiguousError
	switch {
	case errors.Is(err, files.ErrNoVersion), errors.As(err, &ambiguous):
		return ExitNoVersion
	default:
		return ExitError
	}
}
