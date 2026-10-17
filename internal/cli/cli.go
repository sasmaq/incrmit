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
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sasmaq/incrmit/internal/buildinfo"
	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/discovery"
	"github.com/sasmaq/incrmit/internal/files"
	"github.com/sasmaq/incrmit/internal/history"
	"github.com/sasmaq/incrmit/internal/lock"
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
// "reading" or "writing"; display is the path as the user knows it, rendered
// with displayName.
func fsErrorMessage(action, display string, err error) string {
	display = displayName(display)
	// An *fs.PathError, wrapped or not, repeats the path raw ahead of the reason
	// ("stat <path>: file name too long"). The name has already been given,
	// rendered safely, so keep only the reason: which file and why is the
	// message, and which step of reading or writing it failed at is not.
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		err = pathErr.Err
	}
	var tooLarge *files.TooLargeError
	switch {
	case errors.Is(err, fs.ErrPermission):
		return fmt.Sprintf("%s %s: permission denied", action, display)
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Sprintf("%s %s: file does not exist", action, display)
	case errors.Is(err, files.ErrNotRegular):
		return fmt.Sprintf("%s %s: not a regular file", action, display)
	case errors.As(err, &tooLarge):
		return fmt.Sprintf("%s %s: file is %s, over the --max-file-size limit of %s",
			action, display, formatSize(tooLarge.Size), formatSize(tooLarge.Limit))
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
	// Everything printed from here on may carry bytes from the files and names
	// incrmit was pointed at; see display.go.
	stdout, stderr = terminalWriter{stdout}, terminalWriter{stderr}
	if len(args) > 0 {
		switch args[0] {
		case "discover":
			return runDiscover(args[1:], stdout, stderr)
		case "preview":
			return runPreview(args[1:], stdout, stderr)
		case "undo":
			return runUndo(args[1:], stdout, stderr)
		case "version":
			return runVersion(args[1:], stdout, stderr)
		case "--version", "-version", "-v":
			fprintln(stdout, buildinfo.String())
			return ExitOK
		case "help":
			return runHelp(args[1:], stdout, stderr)
		case "-h", "--help", "-help":
			_, showBanner := parseBannerFlag(args[1:])
			fprint(stdout, overview(showBanner))
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
	configPath  string
	file        string
	major       bool
	minor       bool
	patch       bool
	release     bool
	pre         string
	dryRun      bool
	wait        bool  // queue behind another run instead of failing on a held lock
	maxFileSize int64 // per-file read cap in bytes; 0 means no limit

	// preSet and componentSet record whether the user named those flags
	// explicitly. --patch defaults to true and --pre defaults to the empty
	// string, so the values alone cannot tell "asked for" from "left at the
	// default", which is what decides whether --pre bumps a component before
	// starting a prerelease.
	preSet       bool
	componentSet bool
}

func runBump(args []string, stdout, stderr io.Writer) int {
	opts, code := parseBumpFlags(args, stdout, stderr)
	if code != exitSilentFlag && code != ExitOK {
		return code
	}
	if code == exitSilentFlag {
		return ExitOK
	}

	bump, label, err := resolveBump(opts)
	if err != nil {
		fprintln(stderr, "incrmit:", err)
		fprint(stderr, bumpHelp)
		return ExitUsage
	}

	// Take the project lock before reading anything, because everything from
	// here to the last write is one read-modify-write: the config names the
	// targets, the targets give the versions to bump, and all of it plus the
	// journal is written back at the end. A second run that read between our
	// read and our write would compute from the same starting version and its
	// save would erase ours (or ours its), leaving the tree bumped twice but
	// recorded once. A dry run writes nothing, so it stays lock-free.
	var lk *lock.Lock
	if !opts.dryRun {
		var code int
		if lk, code = acquireProject(projectDir(opts.configPath, opts.file), opts.wait, stderr); code != ExitOK {
			return code
		}
		defer func() { _ = lk.Release() }()
	}

	targets, cfgPath, ignore, err := resolveTargets(opts.configPath, opts.file)
	if err != nil {
		fprintln(stderr, "incrmit:", err)
		return classify(err)
	}

	// Now that the project is locked and the targets are known, clear any temp
	// file a crashed run left in a directory this bump writes to.
	sweepPaths := []string{cfgPath}
	for _, tgt := range targets {
		sweepPaths = append(sweepPaths, tgt.fsPath)
	}
	sweepTemps(lk, dirsOf(sweepPaths...)...)

	// The journal is read before anything is written, so a state file that
	// cannot be read fails the bump with the tree untouched; a dry run reads
	// it as well, so it predicts that failure. Only config mode keeps a
	// journal: an --file bump has no config-anchored location to keep (or
	// find) the state file.
	var journal *history.History
	statePath := ""
	if cfgPath != "" {
		statePath = history.ResolvePath(cfgPath)
		if journal, err = history.Load(statePath); err != nil {
			fprintln(stderr, "incrmit:", err)
			return classify(err)
		}
	}

	// Phase 1: read and plan every file, down to the bytes each one will hold,
	// before writing anything, so a problem with one file fails the bump with
	// none of them changed (fail fast). Config entries are grouped by file: a
	// single file may be listed several times (once per distinct version it
	// contains), and all of those versions are bumped in one pass so the
	// writes never clobber each other.
	groups, code := planGroups(targets, bump, opts.maxFileSize, stderr)
	if code != ExitOK {
		// A pending entry on top is a run that stopped partway, which is the
		// likeliest reason the config and the files no longer agree.
		if journal != nil {
			if latest, ok := journal.Latest(); ok && latest.Pending() {
				fprintf(stderr, "incrmit: the last bump or undo in this project stopped partway; run `%s` to finish putting it back\n",
					undoCommand(opts.configPath))
			}
		}
		return code
	}

	writes := make([]fileWrite, 0, len(groups)+1)
	for _, g := range groups {
		writes = append(writes, fileWrite{display: g.display, fsPath: g.fsPath, before: g.data, after: g.updated})
	}
	if cfgPath != "" {
		data, err := bumpedConfig(groups, ignore)
		if err != nil {
			fprintln(stderr, "incrmit:", err)
			return ExitError
		}
		writes = append(writes, fileWrite{display: cfgPath, fsPath: cfgPath, after: data})
	}

	if opts.dryRun {
		fprintf(stdout, "Dry run: would apply a %s (no files changed)\n", label)
		for _, g := range groups {
			for _, e := range g.entries {
				fprintf(stdout, "  %s: %s -> %s\n", displayName(g.display), e.oldVer, e.newVer)
			}
		}
		return ExitOK
	}

	// Phase 2, in an order chosen so the run can stop at any point and leave
	// something undo can finish (doc/DEVELOPMENT.md section 8.7). The journal
	// entry goes first, marked pending, so it exists before any file changes.
	// The targets follow, then the config: it records the new versions, so a
	// config at the new versions means every target was written. The entry is
	// marked complete last.
	var restore func() error
	if cfgPath != "" {
		if restore, code = startEntry(statePath, journal, groups, stderr); code != ExitOK {
			return code
		}
	}
	run := writeRun{what: "bump", journal: statePath, undo: undoCommand(opts.configPath), then: "revert what it wrote"}
	if code := commitWrites(writes, restore, run, stderr); code != ExitOK {
		return code
	}
	if cfgPath != "" {
		journal.MarkLatest(history.StatusComplete)
		if err := saveJournal(statePath, journal); err != nil {
			// Every file holds the new version, and a pending entry undoes
			// exactly as a complete one would, so the bump stands.
			fprintf(stderr, "incrmit: warning: %s; the bump is applied, but its journal entry stays marked unfinished, which `%s` still reverts\n",
				fsErrorMessage("writing", statePath, err), run.undo)
		}
	}

	fprintf(stdout, "Applied a %s to %d file(s):\n", label, len(groups))
	for _, g := range groups {
		for _, e := range g.entries {
			fprintf(stdout, "  %s: %s -> %s\n", displayName(g.display), e.oldVer, e.newVer)
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
	data    []byte // contents as read, put back if a later write fails
	updated []byte // contents with every entry bumped, set by planGroups
	entries []entryPlan
}

// planGroups reads every target file once and computes the old/new version for
// each config entry, grouping entries that refer to the same file, and then
// the bytes each file will hold: all of its known version tokens replaced in a
// single pass, so overlapping bumps do not cascade. A target larger than
// maxBytes is reported rather than read (maxBytes of 0 means no limit), and so
// is a pinned version the file no longer contains. File order and entry order
// are preserved for deterministic output. On failure it reports the problem to
// stderr and returns the appropriate exit code; ExitOK indicates success.
//
// The pinned-version check used to run as each file was written, so a config
// out of sync with its third file left the first two bumped.
func planGroups(targets []target, bump bumpFunc, maxBytes int64, stderr io.Writer) ([]fileGroup, int) {
	groups, code := readGroups(targets, maxBytes, stderr)
	if code != ExitOK {
		return nil, code
	}
	for i := range groups {
		for j := range groups[i].entries {
			e := &groups[i].entries[j]
			newVer, err := bump(e.oldVer)
			if err != nil {
				fprintf(stderr, "incrmit: %s: %v\n", displayName(groups[i].display), err)
				return nil, classify(err)
			}
			e.newVer = newVer
		}
	}
	for i := range groups {
		g := &groups[i]
		repl := make([]files.Replacement, 0, len(g.entries))
		for _, e := range g.entries {
			repl = append(repl, files.Replacement{Old: e.oldVer, New: e.newVer})
		}
		updated, counts := files.SetKnownVersions(g.data, repl)
		for _, e := range g.entries {
			if counts[e.oldVer.String()] == 0 {
				fprintf(stderr, "incrmit: %s: %v\n", displayName(g.display), fmt.Errorf("%w: %s", files.ErrVersionNotFound, e.oldVer))
				return nil, ExitNoVersion
			}
		}
		g.updated = updated
	}
	return groups, ExitOK
}

// bumpedConfig renders the config a bump writes back: each entry at its new
// version, so the next bump reads the correct current version. One entry is
// written per (path, version), so files with several versions keep all of
// their entries, and the user-authored ignore list is carried through so a
// bump never drops it.
func bumpedConfig(groups []fileGroup, ignore []string) ([]byte, error) {
	updated := &config.Config{Ignore: ignore}
	for _, g := range groups {
		for _, e := range g.entries {
			updated.Files = append(updated.Files, config.EntryFor(g.display, e.newVer))
		}
	}
	return config.Marshal(updated)
}

// readGroups reads every target file once and records the version each entry
// starts from, grouping entries that refer to the same file. It is the half of
// the planning work that does not depend on which bump was asked for, so the
// read-only `preview` command shares it with the bump path; entries come back
// with oldVer set and newVer left zero for the caller to fill in.
func readGroups(targets []target, maxBytes int64, stderr io.Writer) ([]fileGroup, int) {
	var groups []fileGroup
	index := make(map[string]int, len(targets))
	for _, tgt := range targets {
		gi, ok := index[tgt.fsPath]
		if !ok {
			data, err := files.ReadTargetWithLimit(tgt.fsPath, maxBytes)
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
				fprintf(stderr, "incrmit: %s: invalid version %q in config: %v\n", displayName(tgt.display), tgt.knownVer, err)
				return nil, ExitNoVersion
			}
		} else {
			oldVer, err = files.FindVersion(groups[gi].data)
			if err != nil {
				fprintf(stderr, "incrmit: %s: %v\n", displayName(tgt.display), err)
				return nil, classify(err)
			}
		}
		groups[gi].entries = append(groups[gi].entries, entryPlan{oldVer: oldVer})
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
	fs.BoolVar(&opts.release, "release", false, "promote a prerelease to its release")
	fs.BoolVar(&opts.release, "r", false, "promote a prerelease to its release (shorthand)")
	fs.StringVar(&opts.pre, "pre", "", "start or advance a prerelease with this identifier")
	fs.StringVar(&opts.pre, "e", "", "start or advance a prerelease with this identifier (shorthand)")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "print the new version without writing")
	fs.BoolVar(&opts.dryRun, "d", false, "print the new version without writing (shorthand)")
	fs.BoolVar(&opts.wait, "wait", false, "wait for another incrmit run to finish instead of failing")
	fs.BoolVar(&opts.wait, "w", false, "wait for another incrmit run to finish (shorthand)")
	maxFileSizeVar(fs, &opts.maxFileSize, 0, "refuse to read a target larger than this size")

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

	// Record which of the overlapping flags were named on the command line:
	// --patch is on by default and --pre defaults to the empty string, so their
	// values cannot distinguish an explicit choice from the default.
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "pre", "e":
			opts.preSet = true
		case "major", "M", "minor", "m", "patch", "p":
			opts.componentSet = true
		}
	})
	return opts, ExitOK
}

// bumpFunc computes the new version for one target from the version it holds
// today. It returns an error when the requested transform is meaningless for
// that particular version — promoting a version that has no prerelease, say —
// which is a usage error even though it can only be detected once the file (or
// the config entry) has been read. It also returns a *version.OverflowError
// when the number it would increment is already math.MaxInt, which classify
// maps to ExitNoVersion. Either way planGroups fails before anything is
// written.
type bumpFunc func(version.Version) (version.Version, error)

// usageError marks a failure caused by the flags the user chose rather than by
// the state of a file, so classify maps it to ExitUsage.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func usagef(format string, a ...any) error {
	return &usageError{msg: fmt.Sprintf(format, a...)}
}

// resolveBump picks the bump transform and the label used in the summary lines.
//
// Plain component bumps are unchanged: if more than one of major/minor/patch is
// requested the highest component wins (major > minor > patch), and patch is the
// default. --release and --pre select the two prerelease transforms instead; see
// prereleaseBump for how --pre combines with a component flag. Flag combinations
// that cannot mean anything are rejected here with a usage error.
func resolveBump(opts bumpOptions) (bumpFunc, string, error) {
	component, label := componentBump(opts)

	switch {
	case opts.release && opts.preSet:
		return nil, "", usagef("--release and --pre are mutually exclusive: one drops a prerelease, the other starts or advances one")

	case opts.release:
		if opts.componentSet {
			return nil, "", usagef("--release promotes a prerelease in place and changes no numbers; drop --major/--minor/--patch")
		}
		return func(v version.Version) (version.Version, error) {
			if !v.IsPrerelease() {
				return version.Version{}, usagef("%s is not a prerelease, so there is nothing to promote with --release", v)
			}
			return v.Release(), nil
		}, "release promotion", nil

	case opts.preSet:
		if err := version.ValidPrereleaseID(opts.pre); err != nil {
			return nil, "", usagef("invalid --pre value %q: %v", opts.pre, err)
		}
		return prereleaseBump(opts, component), fmt.Sprintf("%s prerelease bump", opts.pre), nil

	default:
		return component, label, nil
	}
}

// componentBump returns the plain numeric transform selected by the
// major/minor/patch flags, along with its summary label. The transform fails
// with a *version.OverflowError when the component is already math.MaxInt.
func componentBump(opts bumpOptions) (bumpFunc, string) {
	switch {
	case opts.major:
		return version.Version.BumpMajor, "major bump"
	case opts.minor:
		return version.Version.BumpMinor, "minor bump"
	default:
		return version.Version.BumpPatch, "patch bump"
	}
}

// prereleaseBump implements --pre. Naming a component explicitly always opens a
// new release line, so `--minor --pre rc` on 1.2.3 gives 1.3.0-rc.1 whatever the
// current version looks like. With no component flag the behavior follows the
// version at hand: a release starts the next patch's prerelease
// (1.2.3 -> 1.2.4-rc.1), and a version already in a prerelease iterates on the
// spot (1.2.4-rc.1 -> 1.2.4-rc.2, or 1.2.4-beta.2 -> 1.2.4-rc.1 for a different
// series) rather than skipping a patch number for every preview.
func prereleaseBump(opts bumpOptions, component bumpFunc) bumpFunc {
	return func(v version.Version) (version.Version, error) {
		var line bumpFunc
		switch {
		case opts.componentSet:
			line = component
		case !v.IsPrerelease():
			line = version.Version.BumpPatch
		default:
			return v.BumpPrerelease(opts.pre)
		}
		next, err := line(v)
		if err != nil {
			return version.Version{}, err
		}
		return next.StartPrerelease(opts.pre), nil
	}
}

type target struct {
	display  string // path as the user/config sees it
	fsPath   string // resolved filesystem path
	knownVer string // current version recorded in the config ("" if none)
}

// resolveTargets returns the files a command should act on, the config path they
// came from (empty in --file mode), and the config's ignore list (nil in --file
// mode, and carried through so a config rewrite preserves it). It uses either
// the single file target or every entry in the config at configPath.
// Config-relative paths are resolved against the directory containing the config
// file. When a config entry records a version, it is carried through so the
// caller can target that exact version and avoid re-scanning files that contain
// several version-like strings. It is shared by `bump` and `preview` so both
// resolve targets identically.
func resolveTargets(configPath, file string) ([]target, string, []string, error) {
	if file != "" {
		return []target{{display: file, fsPath: file}}, "", nil, nil
	}

	cfgPath := config.ResolvePath(configPath)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, "", nil, err
	}

	baseDir := filepath.Dir(cfgPath)
	targets := make([]target, 0, len(cfg.Files))
	for _, f := range cfg.Files {
		targets = append(targets, target{
			display:  f.Path,
			fsPath:   config.TargetPath(baseDir, f.Path),
			knownVer: f.Token(),
		})
	}
	return targets, cfgPath, cfg.Ignore, nil
}

type discoverOptions struct {
	path        string
	output      string
	dryRun      bool
	wait        bool  // queue behind another run instead of failing on a held lock
	maxFileSize int64 // per-file scan cap in bytes; 0 means no limit
}

func runDiscover(args []string, stdout, stderr io.Writer) int {
	opts, code := parseDiscoverFlags(args, stdout, stderr)
	if code == exitSilentFlag {
		return ExitOK
	}
	if code != ExitOK {
		return code
	}

	// discover rewrites the config it reads its ignore list from, so the same
	// one-writer rule applies: lock before that read, and hold it until the
	// config has been written. A --dry-run writes nothing and stays lock-free.
	if !opts.dryRun {
		lk, code := acquireProject(filepath.Dir(opts.output), opts.wait, stderr)
		if code != ExitOK {
			return code
		}
		defer func() { _ = lk.Release() }()
		sweepTemps(lk, dirsOf(opts.output)...)
	}

	// Read any user-authored ignore patterns from an existing config at the
	// --output path so discovery honors them and they survive a regeneration.
	// This is also where a file discover must not replace is refused, before
	// the scan, so a mistyped -o costs nothing; --dry-run refuses it too.
	ignore, err := config.LoadIgnore(opts.output)
	if err != nil {
		var notConfig *config.NotConfigError
		if errors.As(err, &notConfig) {
			fprintf(stderr, "incrmit: %s exists and is not an incrmit config (%s); choose another --output or remove the file\n",
				displayName(opts.output), notConfig.Reason)
			return ExitError
		}
		fprintln(stderr, "incrmit:", err)
		return classify(err)
	}

	// Every path is recorded relative to the config's directory, which is
	// where a bump resolves it, and the dry run and the summary print it so.
	cfgDir := filepath.Dir(opts.output)
	results, err := discovery.DiscoverWithLimit(opts.path, cfgDir, opts.maxFileSize, ignore...)
	if err != nil {
		fprintln(stderr, "incrmit:", err)
		return classify(err)
	}
	results = excludeOutput(results, opts.output)
	results = excludeUnlistable(results, stderr)
	if len(results) == 0 {
		fprintf(stderr, "incrmit: no version-bearing files found under %s\n", displayName(opts.path))
		return ExitNoVersion
	}

	if opts.dryRun {
		fprintf(stdout, "Discovered %d file(s) under %s (no config written):\n", len(results), displayName(opts.path))
		if !discovery.SameDir(opts.path, cfgDir) {
			fprintf(stdout, "  (paths relative to %s)\n", displayName(opts.output))
		}
		if len(ignore) > 0 {
			fprintf(stdout, "  (ignoring: %s)\n", strings.Join(displayNames(ignore), ", "))
		}
		for _, r := range results {
			fprintf(stdout, "  %s:\n", displayName(r.Path))
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

	fprintf(stdout, "Wrote %s with %d file(s):\n", displayName(opts.output), len(results))
	for _, r := range results {
		for _, v := range distinctVersions(r.Occurrences) {
			fprintf(stdout, "  %s: %s\n", displayName(r.Path), v)
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
	fs.BoolVar(&opts.wait, "wait", false, "wait for another incrmit run to finish instead of failing")
	fs.BoolVar(&opts.wait, "w", false, "wait for another incrmit run to finish (shorthand)")
	maxFileSizeVar(fs, &opts.maxFileSize, discovery.DefaultMaxScanBytes, "skip files larger than this size")

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

// excludeUnlistable drops, with a warning, any discovered result whose path is
// not valid UTF-8. Such names exist on Linux, where a file name is any bytes,
// but a TOML string must be UTF-8 and has no escape for a raw byte: writing one
// into the config produces a file that every later command refuses to load.
// Skipping it, and saying so, keeps the rest of the tree discoverable; the dry
// run skips it too, so it shows what the config would hold.
func excludeUnlistable(results []discovery.Result, stderr io.Writer) []discovery.Result {
	kept := results[:0]
	for _, r := range results {
		if !utf8.ValidString(r.Path) {
			fprintf(stderr, "incrmit: warning: skipping %s: its name is not valid UTF-8, which a TOML config cannot hold\n", displayName(r.Path))
			continue
		}
		kept = append(kept, r)
	}
	return kept
}

// excludeOutput drops any discovered result that refers to the config file the
// discover command is about to write, so the generated config never lists
// itself as a target. (Files literally named incrmit.toml are already skipped
// during the walk; this also covers a custom --output path.) A result's path is
// relative to the output's directory, so it names the output exactly when it
// resolves there the way a bump will resolve it.
func excludeOutput(results []discovery.Result, output string) []discovery.Result {
	dir, out := filepath.Dir(output), filepath.Clean(output)
	filtered := make([]discovery.Result, 0, len(results))
	for _, r := range results {
		if config.TargetPath(dir, r.Path) == out {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

// startEntry pushes the journal entry for a config-mode bump onto h, the
// journal read from statePath during planning, marked pending, and saves it
// there before the bump writes anything, so `incrmit undo` can revert the bump
// however far it gets. Each change records the path as the config lists it,
// the same value the config rewrite writes, and undo resolves it against the
// config it is given. Nothing absolute is stored, so the journal follows the
// project when it is copied or moved.
//
// It returns the function that puts the journal back as it was, for a bump
// that is rolled back: the earlier entries, or no state file at all if there
// was none. On failure it reports to stderr and returns a non-OK exit code,
// with nothing written.
func startEntry(statePath string, h *history.History, groups []fileGroup, stderr io.Writer) (func() error, int) {
	entry := history.Entry{Timestamp: time.Now().UTC(), Status: history.StatusPending}
	for _, g := range groups {
		for _, e := range g.entries {
			entry.Changes = append(entry.Changes, history.Change{
				Path: g.display,
				Old:  e.oldVer.String(),
				New:  e.newVer.String(),
			})
		}
	}

	_, statErr := os.Lstat(statePath)
	existed := !errors.Is(statErr, fs.ErrNotExist)
	saved := slices.Clone(h.Entries)
	h.Push(entry)
	if err := saveJournal(statePath, h); err != nil {
		fprintf(stderr, "incrmit: %s\n", fsErrorMessage("writing", statePath, err))
		return nil, classify(err)
	}
	return func() error {
		h.Entries = saved
		if !existed {
			return os.Remove(statePath)
		}
		return saveJournal(statePath, h)
	}, ExitOK
}

type undoOptions struct {
	configPath string
	dryRun     bool
	wait       bool // queue behind another run instead of failing on a held lock
}

// runUndo reverts the most recent recorded bump: it restores the previous
// version token in every file that bump wrote and rewrites the config's
// recorded versions to match, then pops the journal entry so a repeated undo
// walks further back rather than re-applying the same revert.
func runUndo(args []string, stdout, stderr io.Writer) int {
	opts, code := parseUndoFlags(args, stdout, stderr)
	if code == exitSilentFlag {
		return ExitOK
	}
	if code != ExitOK {
		return code
	}

	// Every path undo touches derives from this one: the lock and the journal
	// sit beside it, it is the config rewritten, and each recorded file is
	// resolved against its directory. Nothing the journal records can name
	// another location.
	cfgPath := config.ResolvePath(opts.configPath)

	// undo is the same read-modify-write as bump, in reverse: it reads the
	// journal, the config, and every recorded file, then writes all three back.
	// It takes the same lock, so an undo and a bump can never interleave.
	var lk *lock.Lock
	if !opts.dryRun {
		var code int
		if lk, code = acquireProject(filepath.Dir(cfgPath), opts.wait, stderr); code != ExitOK {
			return code
		}
		defer func() { _ = lk.Release() }()
	}

	statePath := history.ResolvePath(cfgPath)
	h, err := history.Load(statePath)
	if err != nil {
		fprintln(stderr, "incrmit:", err)
		return classify(err)
	}
	entry, ok := h.Latest()
	if !ok {
		fprintln(stdout, "Nothing to undo: no bump history recorded.")
		return ExitOK
	}

	// Load the config up front (before writing anything) so a config problem
	// aborts the undo before any file is touched, mirroring bump's fail-fast.
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fprintln(stderr, "incrmit:", err)
		return classify(err)
	}
	listed := make(map[listing]bool, len(cfg.Files))
	for _, f := range cfg.Files {
		listed[listing{f.Path, f.Token()}] = true
	}

	// The config is trusted input and the journal is not: a repository can
	// commit a state file. So undo reverts only what a bump of this config
	// could have written, each recorded file listed at the version the bump
	// left it at, and refuses the whole entry otherwise. A path the config
	// lists as absolute or with ../ is allowed exactly because it lists it.
	//
	// A pending entry may also find the config at the versions the bump
	// started from, since the config is written after every target: a bump
	// that stopped before it, or an undo that got past it. The config is
	// written whole, so it is entirely at one side or the other.
	pending := entry.Pending()
	atNew, atOld := true, true
	for _, c := range entry.Changes {
		isNew, isOld := listed[listing{c.Path, c.New}], listed[listing{c.Path, c.Old}]
		atNew, atOld = atNew && isNew, atOld && isOld
		switch {
		case isNew, pending && isOld:
		case pending:
			fprintf(stderr, "incrmit: %s: not listed in %s at %s or %s, the versions the unfinished bump recorded (refusing to undo)\n",
				displayName(c.Path), displayName(cfgPath), c.New, c.Old)
			return ExitError
		default:
			fprintf(stderr, "incrmit: %s: not listed in %s at %s, the version the last bump recorded (refusing to undo)\n",
				displayName(c.Path), displayName(cfgPath), c.New)
			return ExitError
		}
	}
	if !atNew && !atOld {
		fprintf(stderr, "incrmit: %s lists some of the unfinished bump's files at the versions it wrote and others at the ones it started from (refusing to undo)\n",
			displayName(cfgPath))
		return ExitError
	}

	// Phase 1: read every recorded file once, group changes by file, and build
	// the reverse (new -> old) replacement, verifying the recorded "new" token
	// is still present so a file edited since the bump is caught before any
	// write happens.
	groups, code := planReverts(entry, filepath.Dir(cfgPath), stderr)
	if code != ExitOK {
		return code
	}

	// Restore the config's recorded versions to their pre-bump values so the
	// config stays in sync with the reverted files (mirroring the bump's
	// self-update in reverse). It is the config undo was given, whatever
	// location an older journal recorded. A pending entry whose bump never
	// reached the config leaves it alone.
	var writes []fileWrite
	for _, g := range groups {
		if !g.unwritten {
			writes = append(writes, fileWrite{display: g.display, fsPath: g.fsPath, before: g.data, after: g.updated})
		}
	}
	if atNew {
		revert := make(map[listing]string, len(entry.Changes))
		for _, c := range entry.Changes {
			revert[listing{c.Path, c.New}] = c.Old
		}
		for i := range cfg.Files {
			f := &cfg.Files[i]
			if old, ok := revert[listing{f.Path, f.Token()}]; ok {
				f.SetToken(old)
			}
		}
		data, err := config.Marshal(cfg)
		if err != nil {
			fprintln(stderr, "incrmit:", err)
			return ExitError
		}
		writes = append(writes, fileWrite{display: cfgPath, fsPath: cfgPath, after: data})
	}

	which := "most recent"
	if pending {
		which = "unfinished"
	}
	if opts.dryRun {
		fprintf(stdout, "Dry run: would undo the %s bump (no files changed)\n", which)
		printReverts(stdout, groups)
		return ExitOK
	}

	// Clear any temp file a crashed run left in a directory this undo writes
	// to, now that the project is locked and the files are known.
	sweepPaths := []string{cfgPath}
	for _, g := range groups {
		sweepPaths = append(sweepPaths, g.fsPath)
	}
	sweepTemps(lk, dirsOf(sweepPaths...)...)

	// Phase 2, in the bump's order: the entry is marked pending before the
	// first write, so an undo that stops partway can be run again and finish;
	// then the files, then the config; then the entry is popped.
	var restore func() error
	if !pending {
		h.MarkLatest(history.StatusPending)
		if err := saveJournal(statePath, h); err != nil {
			fprintf(stderr, "incrmit: %s\n", fsErrorMessage("writing", statePath, err))
			return classify(err)
		}
		restore = func() error {
			h.MarkLatest(history.StatusComplete)
			return saveJournal(statePath, h)
		}
	}
	run := writeRun{what: "undo", journal: statePath, undo: undoCommand(opts.configPath), then: "finish the undo"}
	if code := commitWrites(writes, restore, run, stderr); code != ExitOK {
		return code
	}

	// Pop the reverted entry so a repeated undo does not re-apply it.
	h.Pop()
	if err := saveJournal(statePath, h); err != nil {
		// The files and the config are back at the old versions, which is
		// what an unfinished entry's undo leaves alone, so the next undo
		// clears the entry without writing anything.
		fprintf(stderr, "incrmit: warning: %s; the undo is applied, but its journal entry stays, marked unfinished, until `%s` clears it\n",
			fsErrorMessage("writing", statePath, err), run.undo)
	}

	reverted := 0
	for _, g := range groups {
		if !g.unwritten {
			reverted++
		}
	}
	fprintf(stdout, "Undid the %s bump in %d file(s):\n", which, reverted)
	printReverts(stdout, groups)
	return ExitOK
}

// printReverts lists what an undo reverts, each change as new -> old, or
// "already at" the old version for a file the unfinished bump never wrote.
func printReverts(w io.Writer, groups []revertGroup) {
	for _, g := range groups {
		for _, c := range g.changes {
			if g.unwritten {
				fprintf(w, "  %s: already at %s\n", displayName(c.Path), c.Old)
				continue
			}
			fprintf(w, "  %s: %s -> %s\n", displayName(c.Path), c.New, c.Old)
		}
	}
}

// listing is a config entry as undo matches it against the journal: the path
// as written and the full version token.
type listing struct{ path, version string }

// revertGroup is one file plus every recorded change to undo in it, along with
// the reverted bytes computed during planning (written verbatim in phase 2).
type revertGroup struct {
	display string
	fsPath  string
	data    []byte // contents as read, put back if a later write fails
	updated []byte // contents with every change reverted
	changes []history.Change
	// unwritten marks a file of a pending entry that holds the versions the
	// bump started from: the run never wrote it, or put it back, so there is
	// nothing to revert.
	unwritten bool
}

// planReverts reads each recorded file once, groups the entry's changes by
// file, and computes the reverse (new -> old) rewrite in a single pass so
// overlapping reverts do not cascade (matching the bump write path). Recorded
// paths are resolved against baseDir, the directory of the config undo was
// given, exactly as resolveTargets resolves them for a bump. It fails fast if a
// file no longer contains the "new" token the bump wrote, which means the file
// was edited since the bump and undoing would clobber that change.
//
// A pending entry's run may have stopped before writing a file, so there a
// file holding every "old" token and not every "new" one is marked unwritten
// and left alone, rather than refused as edited. A file holding all of both,
// or all of neither, is refused: the first cannot be told apart from a file
// that already held the new version somewhere, and the second was edited.
func planReverts(entry history.Entry, baseDir string, stderr io.Writer) ([]revertGroup, int) {
	var groups []revertGroup
	index := make(map[string]int, len(entry.Changes))
	for _, c := range entry.Changes {
		fsPath := config.TargetPath(baseDir, c.Path)
		gi, ok := index[fsPath]
		if !ok {
			data, err := files.ReadTarget(fsPath)
			if err != nil {
				fprintf(stderr, "incrmit: %s\n", fsErrorMessage("reading", c.Path, err))
				return nil, classify(err)
			}
			groups = append(groups, revertGroup{display: c.Path, fsPath: fsPath, data: data})
			gi = len(groups) - 1
			index[fsPath] = gi
		}
		groups[gi].changes = append(groups[gi].changes, c)
	}

	for i := range groups {
		g := &groups[i]
		back := make([]files.Replacement, 0, len(g.changes))
		for _, c := range g.changes {
			// The journal stores whole tokens; parse them back so the revert
			// matches the same way the bump did, prerelease suffix included.
			oldVer, oldErr := version.Parse(c.Old)
			newVer, newErr := version.Parse(c.New)
			if oldErr != nil || newErr != nil {
				fprintf(stderr, "incrmit: %s: recorded versions %q -> %q are not valid (refusing to undo)\n", displayName(g.display), c.Old, c.New)
				return nil, ExitNoVersion
			}
			back = append(back, files.Replacement{Old: newVer, New: oldVer})
		}
		updated, newCounts := files.SetKnownVersions(g.data, back)
		g.updated = updated
		missingNew := firstMissing(back, newCounts)

		if !entry.Pending() {
			if missingNew != nil {
				fprintf(stderr, "incrmit: %s: expected version %s from the last bump is no longer present; the file was changed since (refusing to undo)\n", displayName(g.display), missingNew.Old)
				return nil, ExitError
			}
			continue
		}

		forward := make([]files.Replacement, len(back))
		for j, r := range back {
			forward[j] = files.Replacement{Old: r.New, New: r.Old}
		}
		_, oldCounts := files.SetKnownVersions(g.data, forward)
		missingOld := firstMissing(forward, oldCounts)
		switch {
		case missingNew == nil && missingOld != nil:
			// Written: revert it as for a finished bump.
		case missingNew != nil && missingOld == nil:
			g.unwritten = true
		case missingNew == nil:
			fprintf(stderr, "incrmit: %s: holds both %s, which the unfinished bump wrote, and %s, which it started from, so whether the bump reached it cannot be told (refusing to undo)\n",
				displayName(g.display), g.changes[0].New, g.changes[0].Old)
			return nil, ExitError
		default:
			fprintf(stderr, "incrmit: %s: holds neither %s, which the unfinished bump wrote, nor %s, which it started from; the file was changed since (refusing to undo)\n",
				displayName(g.display), missingNew.Old, missingOld.Old)
			return nil, ExitError
		}
	}
	return groups, ExitOK
}

// firstMissing returns the first replacement in repls whose Old version
// SetKnownVersions counted no occurrence of, or nil when every one occurs.
func firstMissing(repls []files.Replacement, counts map[string]int) *files.Replacement {
	for i := range repls {
		if counts[repls[i].Old.String()] == 0 {
			return &repls[i]
		}
	}
	return nil
}

// parseUndoFlags registers the undo flags (long and short names) and parses
// args, returning the options and an exit code: ExitOK to proceed, ExitUsage on
// a parse error (help to stderr), or exitSilentFlag when -h/--help was
// requested (help to stdout).
func parseUndoFlags(args []string, stdout, stderr io.Writer) (undoOptions, int) {
	var opts undoOptions
	fs := flag.NewFlagSet("incrmit undo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	// Help and usage are rendered explicitly below from the centralized text
	// so an explicit -h/--help can go to stdout while errors go to stderr.
	fs.Usage = func() {}

	fs.StringVar(&opts.configPath, "config", "", "path to the TOML config file")
	fs.StringVar(&opts.configPath, "c", "", "path to the TOML config file (shorthand)")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "preview the revert without writing")
	fs.BoolVar(&opts.dryRun, "d", false, "preview the revert without writing (shorthand)")
	fs.BoolVar(&opts.wait, "wait", false, "wait for another incrmit run to finish instead of failing")
	fs.BoolVar(&opts.wait, "w", false, "wait for another incrmit run to finish (shorthand)")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fprint(stdout, undoHelp)
			return opts, exitSilentFlag
		}
		fprint(stderr, undoHelp)
		return opts, ExitUsage
	}
	if fs.NArg() > 0 {
		fprintf(stderr, "incrmit: unexpected argument %q\n", fs.Arg(0))
		fprint(stderr, undoHelp)
		return opts, ExitUsage
	}
	return opts, ExitOK
}

// classify maps an error to the appropriate process exit code.
func classify(err error) int {
	var ambiguous *files.AmbiguousError
	var overflow *version.OverflowError
	var usage *usageError
	switch {
	case errors.As(err, &usage):
		return ExitUsage
	case errors.Is(err, files.ErrNoVersion),
		errors.Is(err, files.ErrVersionNotFound),
		errors.As(err, &ambiguous),
		errors.As(err, &overflow):
		return ExitNoVersion
	default:
		return ExitError
	}
}
