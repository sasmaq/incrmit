// Package cli implements the incrmit command-line interface: flag parsing,
// command dispatch, and the bump workflow. It is kept separate from package
// main so it can be exercised directly in tests.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sasmaq/incrmit/internal/buildinfo"
	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/discovery"
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

func fprint(w io.Writer, s string) {
	_, _ = io.WriteString(w, s)
}

// fsErrorMessage renders a clear, consistent message for a filesystem error,
// calling out permission and missing-file cases explicitly rather than relying
// on the (often path-duplicated) raw OS error text. action is a verb such as
// "reading" or "writing"; display is the path as the user knows it.
func fsErrorMessage(action, display string, err error) string {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return fmt.Sprintf("%s %s: permission denied", action, display)
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Sprintf("%s %s: file does not exist", action, display)
	default:
		return fmt.Sprintf("%s %s: %v", action, display, err)
	}
}

// Exit codes, per doc/DEVELOPMENT.md section 10.
const (
	ExitOK         = 0  // success
	ExitError      = 1  // generic runtime error
	ExitUsage      = 2  // invalid arguments or flags
	ExitNoVersion  = 3  // no version found / parse failure
	exitSilentFlag = -1 // help was printed; exit successfully without running
)

// Main is the entry point used by package main. It parses args, runs the
// selected command, and returns a process exit code. All human-readable output
// goes to stdout; errors go to stderr.
func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "discover":
			return runDiscover(args[1:], stdout, stderr)
		case "version":
			return runVersion(args[1:], stdout, stderr)
		case "--version", "-version", "-v":
			fprintln(stdout, buildinfo.String())
			return ExitOK
		case "help":
			return runHelp(args[1:], stdout, stderr)
		case "-h", "--help", "-help":
			fprint(stdout, overviewHelp)
			return ExitOK
		}
		// A non-flag first argument that is not a known subcommand is an
		// unknown command. (Anything starting with "-" falls through to the
		// default bump command so its flags are parsed there.)
		if !strings.HasPrefix(args[0], "-") {
			fprintf(stderr, "incrmit: unknown command %q\n", args[0])
			fprintln(stderr, "Run 'incrmit help' to see available commands.")
			return ExitUsage
		}
	}
	return runBump(args, stdout, stderr)
}

// runVersion implements the `version` subcommand. It prints the tool version,
// or this command's help when asked with -h / --help.
func runVersion(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "-help":
			fprint(stdout, versionHelp)
			return ExitOK
		default:
			fprintf(stderr, "incrmit: unexpected argument %q\n", args[0])
			fprint(stderr, versionHelp)
			return ExitUsage
		}
	}
	fprintln(stdout, buildinfo.String())
	return ExitOK
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
	opts, code := parseBumpFlags(args, stdout, stderr)
	if code != exitSilentFlag && code != ExitOK {
		return code
	}
	if code == exitSilentFlag {
		return ExitOK
	}

	bump, label := resolveBump(opts.major, opts.minor, opts.patch)

	targets, cfgPath, ignore, err := resolveTargets(opts)
	if err != nil {
		fprintln(stderr, "incrmit:", err)
		return classify(err)
	}

	// Phase 1: read and plan every file before writing anything, so a failure
	// on one file does not leave others half-updated (fail fast). Config entries
	// are grouped by file: a single file may be listed several times (once per
	// distinct version it contains), and all of those versions are bumped in one
	// pass so the writes never clobber each other.
	groups, code := planGroups(targets, bump, stderr)
	if code != ExitOK {
		return code
	}

	if opts.dryRun {
		fprintf(stdout, "Dry run: would apply a %s bump (no files changed)\n", label)
		for _, g := range groups {
			for _, e := range g.entries {
				fprintf(stdout, "  %s: %s -> %s\n", g.display, e.oldVer, e.newVer)
			}
		}
		return ExitOK
	}

	// Phase 2: write each file once, replacing all of its known version tokens
	// in a single pass so overlapping bumps do not cascade.
	for _, g := range groups {
		repl := make(map[string]string, len(g.entries))
		for _, e := range g.entries {
			repl[e.oldVer.String()] = e.newVer.String()
		}
		updated, counts := files.SetKnownVersions(g.data, repl)
		for _, e := range g.entries {
			if counts[e.oldVer.String()] == 0 {
				fprintf(stderr, "incrmit: %s: %v\n", g.display, fmt.Errorf("%w: %s", files.ErrVersionNotFound, e.oldVer))
				return ExitNoVersion
			}
		}
		if err := files.WriteAtomic(g.fsPath, updated); err != nil {
			fprintf(stderr, "incrmit: %s\n", fsErrorMessage("writing", g.display, err))
			return classify(err)
		}
	}

	// Keep the config in sync: record each entry's new version so the next bump
	// reads the correct current version. Only done in config mode (not --file).
	// One entry is written per (path, version) so files with several versions
	// keep all of their entries.
	if cfgPath != "" {
		// Carry the user-authored ignore list through the rewrite so a bump
		// never drops it.
		updated := &config.Config{Ignore: ignore}
		for _, g := range groups {
			for _, e := range g.entries {
				updated.Files = append(updated.Files, config.FileEntry{Path: g.display, Version: e.newVer.String()})
			}
		}
		data, err := config.Marshal(updated)
		if err != nil {
			fprintln(stderr, "incrmit:", err)
			return ExitError
		}
		if err := files.WriteAtomic(cfgPath, data); err != nil {
			fprintf(stderr, "incrmit: %s\n", fsErrorMessage("writing", cfgPath, err))
			return classify(err)
		}
	}

	fprintf(stdout, "Applied a %s bump to %d file(s):\n", label, len(groups))
	for _, g := range groups {
		for _, e := range g.entries {
			fprintf(stdout, "  %s: %s -> %s\n", g.display, e.oldVer, e.newVer)
		}
	}
	return ExitOK
}

// entryPlan is a single version bump within a file: the old token and its
// computed replacement.
type entryPlan struct {
	oldVer version.Version
	newVer version.Version
}

// fileGroup is one target file plus every version bump planned for it. A file
// listed under several config entries (one per distinct version) is bumped as a
// single unit so all of its versions are rewritten in one pass.
type fileGroup struct {
	display string
	fsPath  string
	data    []byte
	entries []entryPlan
}

// planGroups reads every target file once and computes the old/new version for
// each config entry, grouping entries that refer to the same file. File order
// and entry order are preserved for deterministic output. On failure it reports
// the problem to stderr and returns the appropriate exit code; ExitOK indicates
// success.
func planGroups(targets []target, bump func(version.Version) version.Version, stderr io.Writer) ([]fileGroup, int) {
	var groups []fileGroup
	index := make(map[string]int, len(targets))
	for _, tgt := range targets {
		gi, ok := index[tgt.fsPath]
		if !ok {
			data, err := os.ReadFile(tgt.fsPath)
			if err != nil {
				fprintf(stderr, "incrmit: %s\n", fsErrorMessage("reading", tgt.display, err))
				return nil, classify(err)
			}
			groups = append(groups, fileGroup{display: tgt.display, fsPath: tgt.fsPath, data: data})
			gi = len(groups) - 1
			index[tgt.fsPath] = gi
		}

		// Prefer the version recorded in the config: it pins the exact token to
		// bump, so files containing several version-like strings are handled
		// unambiguously. Fall back to scanning the file when none is recorded.
		var oldVer version.Version
		var err error
		if tgt.knownVer != "" {
			oldVer, err = version.Parse(tgt.knownVer)
			if err != nil {
				fprintf(stderr, "incrmit: %s: invalid version %q in config: %v\n", tgt.display, tgt.knownVer, err)
				return nil, ExitNoVersion
			}
		} else {
			oldVer, err = files.FindVersion(groups[gi].data)
			if err != nil {
				fprintf(stderr, "incrmit: %s: %v\n", tgt.display, err)
				return nil, classify(err)
			}
		}
		groups[gi].entries = append(groups[gi].entries, entryPlan{oldVer: oldVer, newVer: bump(oldVer)})
	}
	return groups, ExitOK
}

// parseBumpFlags registers the bump flags (each with a long and short name)
// and parses args. It returns the populated options and an exit code: ExitOK to
// proceed, ExitUsage on a parse error (help printed to stderr), or
// exitSilentFlag when -h/--help was requested (help printed to stdout).
func parseBumpFlags(args []string, stdout, stderr io.Writer) (bumpOptions, int) {
	var opts bumpOptions
	fs := flag.NewFlagSet("incrmit", flag.ContinueOnError)
	fs.SetOutput(stderr)
	// Help and usage are rendered explicitly below from the centralized text
	// so an explicit -h/--help can go to stdout while errors go to stderr.
	fs.Usage = func() {}

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
			fprint(stdout, bumpHelp)
			return opts, exitSilentFlag
		}
		fprint(stderr, bumpHelp)
		return opts, ExitUsage
	}
	if fs.NArg() > 0 {
		fprintf(stderr, "incrmit: unexpected argument %q\n", fs.Arg(0))
		fprint(stderr, bumpHelp)
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
	display  string // path as the user/config sees it
	fsPath   string // resolved filesystem path
	knownVer string // current version recorded in the config ("" if none)
}

// resolveTargets returns the files to bump, the config path they came from
// (empty in --file mode), and the config's ignore list (nil in --file mode, and
// carried through so a config rewrite preserves it). It uses either the single
// --file target or every entry in the config. Config-relative paths are resolved
// against the directory containing the config file. When a config entry records
// a version, it is carried through so the bump can target that exact version and
// avoid re-scanning files that contain several version-like strings.
func resolveTargets(opts bumpOptions) ([]target, string, []string, error) {
	if opts.file != "" {
		return []target{{display: opts.file, fsPath: opts.file}}, "", nil, nil
	}

	cfgPath := config.ResolvePath(opts.configPath)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, "", nil, err
	}

	baseDir := filepath.Dir(cfgPath)
	targets := make([]target, 0, len(cfg.Files))
	for _, f := range cfg.Files {
		fsPath := f.Path
		if !filepath.IsAbs(fsPath) {
			fsPath = filepath.Join(baseDir, fsPath)
		}
		targets = append(targets, target{
			display:  f.Path,
			fsPath:   fsPath,
			knownVer: f.Version,
		})
	}
	return targets, cfgPath, cfg.Ignore, nil
}

type discoverOptions struct {
	path   string
	output string
	dryRun bool
}

func runDiscover(args []string, stdout, stderr io.Writer) int {
	opts, code := parseDiscoverFlags(args, stdout, stderr)
	if code == exitSilentFlag {
		return ExitOK
	}
	if code != ExitOK {
		return code
	}

	// Read any user-authored ignore patterns from an existing config at the
	// --output path so discovery honors them and they survive a regeneration.
	ignore, err := config.LoadIgnore(opts.output)
	if err != nil {
		fprintln(stderr, "incrmit:", err)
		return classify(err)
	}

	results, err := discovery.Discover(opts.path, ignore...)
	if err != nil {
		fprintln(stderr, "incrmit:", err)
		return classify(err)
	}
	results = excludeOutput(results, opts.path, opts.output)
	if len(results) == 0 {
		fprintf(stderr, "incrmit: no version-bearing files found under %s\n", opts.path)
		return ExitNoVersion
	}

	if opts.dryRun {
		fprintf(stdout, "Discovered %d file(s) under %s (no config written):\n", len(results), opts.path)
		if len(ignore) > 0 {
			fprintf(stdout, "  (ignoring: %s)\n", strings.Join(ignore, ", "))
		}
		for _, r := range results {
			fprintf(stdout, "  %s:\n", r.Path)
			for _, o := range r.Occurrences {
				fprintf(stdout, "    L%d: %s\n", o.Line, o.Text)
			}
		}
		return ExitOK
	}

	data, err := discovery.Generate(results, ignore...)
	if err != nil {
		fprintln(stderr, "incrmit:", err)
		return ExitError
	}
	if err := files.WriteAtomic(opts.output, data); err != nil {
		fprintf(stderr, "incrmit: %s\n", fsErrorMessage("writing", opts.output, err))
		return classify(err)
	}

	fprintf(stdout, "Wrote %s with %d file(s):\n", opts.output, len(results))
	for _, r := range results {
		for _, v := range distinctVersions(r.Occurrences) {
			fprintf(stdout, "  %s: %s\n", r.Path, v)
		}
	}
	return ExitOK
}

// distinctVersions returns the version strings in occurrences with duplicates
// removed, in first-seen order, matching how Generate maps a file to config
// entries (one per distinct version).
func distinctVersions(occ []discovery.Occurrence) []string {
	seen := make(map[string]struct{}, len(occ))
	var out []string
	for _, o := range occ {
		v := o.Version.String()
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func parseDiscoverFlags(args []string, stdout, stderr io.Writer) (discoverOptions, int) {
	var opts discoverOptions
	fs := flag.NewFlagSet("incrmit discover", flag.ContinueOnError)
	fs.SetOutput(stderr)
	// Help and usage are rendered explicitly below from the centralized text
	// so an explicit -h/--help can go to stdout while errors go to stderr.
	fs.Usage = func() {}

	fs.StringVar(&opts.path, "path", ".", "root directory to scan")
	fs.StringVar(&opts.path, "P", ".", "root directory to scan (shorthand)")
	fs.StringVar(&opts.output, "output", config.DefaultPath, "path to write the generated config")
	fs.StringVar(&opts.output, "o", config.DefaultPath, "path to write the generated config (shorthand)")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "print discovered files without writing the config")
	fs.BoolVar(&opts.dryRun, "d", false, "print discovered files without writing the config (shorthand)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fprint(stdout, discoverHelp)
			return opts, exitSilentFlag
		}
		fprint(stderr, discoverHelp)
		return opts, ExitUsage
	}
	if fs.NArg() > 0 {
		fprintf(stderr, "incrmit: unexpected argument %q\n", fs.Arg(0))
		fprint(stderr, discoverHelp)
		return opts, ExitUsage
	}
	return opts, ExitOK
}

// excludeOutput drops any discovered result that refers to the config file the
// discover command is about to write, so the generated config never lists
// itself as a target. (Files literally named incrmit.toml are already skipped
// during the walk; this also covers a custom --output path.)
func excludeOutput(results []discovery.Result, root, output string) []discovery.Result {
	outAbs, err := filepath.Abs(output)
	if err != nil {
		return results
	}
	filtered := make([]discovery.Result, 0, len(results))
	for _, r := range results {
		rAbs, err := filepath.Abs(filepath.Join(root, r.Path))
		if err == nil && rAbs == outAbs {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

// classify maps an error to the appropriate process exit code.
func classify(err error) int {
	var ambiguous *files.AmbiguousError
	switch {
	case errors.Is(err, files.ErrNoVersion),
		errors.Is(err, files.ErrVersionNotFound),
		errors.As(err, &ambiguous):
		return ExitNoVersion
	default:
		return ExitError
	}
}
