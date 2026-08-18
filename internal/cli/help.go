package cli

import "io"

// This file is the single source of truth for incrmit's usage and help text.
// Every command (bump, discover, version, and help) references the constants
// below so the `-h` / `--help` output and the `incrmit help` output stay in
// sync and are never duplicated across separate fs.Usage strings.

// bumpFlags and discoverFlags are the single source of truth for each
// command's flag block. They are composed into both the per-command help
// (bumpHelp, discoverHelp) and the top-level overview so the flag text never
// drifts out of sync or gets duplicated across help strings.
const bumpFlags = `  -c, --config string       path to the TOML config file (default "incrmit.toml")
  -f, --file string         bump the version in one file (skips config)
  -M, --major               bump the major version (resets minor and patch)
  -m, --minor               bump the minor version (resets patch)
  -p, --patch               bump the patch version (default)
  -r, --release             promote a prerelease (1.2.3-rc.1 -> 1.2.3)
  -e, --pre id              start or advance a prerelease (1.2.3 -> 1.2.4-id.1)
  -s, --max-file-size size  refuse to read a larger target (default no limit)
  -d, --dry-run             print the new version without writing
`

const discoverFlags = `  -P, --path string         root directory to scan (default ".")
  -o, --output string       path for the generated config (default "incrmit.toml")
  -s, --max-file-size size  skip files larger than this (default 32MiB)
  -d, --dry-run             print discovered files without writing the config
`

const previewFlags = `  -c, --config string       path to the TOML config file (default "incrmit.toml")
  -f, --file string         preview one file (skips config)
  -s, --max-file-size size  refuse to read a larger target (default no limit)
`

const undoFlags = `  -c, --config string  path to the TOML config file (default "incrmit.toml")
  -d, --dry-run        preview the revert (new -> old) without writing
`

// prereleaseNote documents how prereleases and build metadata are handled. It
// is appended to the bump help, where the flags it describes live.
const prereleaseNote = `
A version may carry the semver prerelease and build-metadata sections
(1.2.3-rc.1, 1.2.3+build.7, v2.0.0-beta.1+exp.sha.5114f85); the whole token is
matched and rewritten, never just the numbers inside it.

A --major/--minor/--patch bump drops both sections, so 1.2.3-rc.1 patches to
1.2.4. Use --release to promote a prerelease in place (1.2.3-rc.1 -> 1.2.3);
it is an error (exit 2) on a version that has no prerelease. Use --pre to
start or advance one: 1.2.3 -> 1.2.4-rc.1, then 1.2.4-rc.1 -> 1.2.4-rc.2.
Naming a component alongside --pre opens a new line instead, so --minor --pre
rc gives 1.3.0-rc.1. --release and --pre cannot be combined.
`

// sizeNote explains the --max-file-size value format once, and is appended to
// the help of each command that takes it.
const sizeNote = `
A size is a plain byte count (1048576) or a value with a unit suffix such as
512KB, 32MiB, or 2G. A size of 0 means no limit.
`

const helpFlags = `      --no-banner      hide the ASCII banner above the overview
`

// banner is the incrmit logo printed above the top-level overview. It is
// plain 7-bit ASCII (no Unicode, no color) and 35 columns wide, so it renders
// identically on Linux, macOS, and Windows terminals and stays well inside the
// conventional 80-column width. The middle row is spliced around a backtick,
// which a Go raw string literal cannot contain.
const banner = ` _                           _ _
(_)_ __   ___ _ __ _ __ ___ (_) |_
| | '_ \ / __| '__| '_ ` + "`" + ` _ \| | __|
| | | | | (__| |  | | | | | | | |_
|_|_| |_|\___|_|  |_| |_| |_|_|\__|
`

// overviewHelp is the top-level summary printed by `incrmit help` and by
// top-level `-h` / `--help` (with no subcommand). It lists the commands and
// reuses the centralized flag blocks so the available flags are discoverable
// without drilling into each command's help.
const overviewHelp = `incrmit — increment semantic versions across one or more files

usage:
  incrmit [flags]            bump the version in the configured files (default)
  incrmit discover [flags]   scan the tree for version-bearing files and write a config
  incrmit preview [flags]    show each file's version and its next patch/minor/major
  incrmit undo [flags]       revert the most recent bump
  incrmit version            print the incrmit tool version
  incrmit help [command]     show this overview, or help for a specific command

Bump flags (default command):
` + bumpFlags + `
Discover flags (incrmit discover):
` + discoverFlags + `
Preview flags (incrmit preview):
` + previewFlags + `
Undo flags (incrmit undo):
` + undoFlags + `
Help flags (incrmit help, incrmit -h):
` + helpFlags + `
The version command takes no flags. The --version, -version, and -v flags
print the tool version.

A prerelease or build section is part of the version token: 1.2.3-rc.1 patches
to 1.2.4 (both sections are dropped), --release promotes it to 1.2.3, and
--pre rc starts or advances one (1.2.3 -> 1.2.4-rc.1 -> 1.2.4-rc.2).

A --max-file-size value is a plain byte count (1048576) or a value with a unit
suffix such as 512KB, 32MiB, or 2G. A size of 0 means no limit.

Run "incrmit help <command>" for details, for example "incrmit help discover".
`

// bumpHelp documents the default bump command. It is shown by `incrmit -h`
// (in bump context), `incrmit help bump`, and on a bump usage error.
const bumpHelp = `usage: incrmit [flags]

Bump the semantic version in the configured files.

Flags:
` + bumpFlags + prereleaseNote + sizeNote

// discoverHelp documents the discover command. It is shown by
// `incrmit discover -h`, `incrmit help discover`, and on a discover usage error.
const discoverHelp = `usage: incrmit discover [flags]

Scan a directory tree for version-bearing files and generate a config.

Flags:
` + discoverFlags + sizeNote

// previewHelp documents the preview command. It is shown by
// `incrmit preview -h`, `incrmit help preview`, and on a preview usage error.
const previewHelp = `usage: incrmit preview [flags]

Show, for every file in the config, the version it holds today alongside what a
--patch, --minor, and --major bump would write, as an aligned table:

  PATH        CURRENT  PATCH   MINOR   MAJOR
  README.md   0.1.15   0.1.16  0.2.0   1.0.0

preview is read-only: it writes no file, no config, and no bump history.

A "v" prefix is carried into every projected version (v1.2.3 previews as
v1.2.4 / v1.3.0 / v2.0.0), and a prerelease or build section is dropped by all
three bumps, exactly as a real bump would. A file listed once per version it
contains gets one row per version.

Rows whose version differs from the one most entries hold are marked "*" and
explained under the table, so a file left behind by a partial bump is visible
at a glance. Only semver precedence counts as a difference: v1.2.3 and 1.2.3
name the same release and are not marked.

Flags:
` + previewFlags + sizeNote

// undoHelp documents the undo command. It is shown by `incrmit undo -h`,
// `incrmit help undo`, and on an undo usage error.
const undoHelp = `usage: incrmit undo [flags]

Revert the most recent bump, restoring the previous version in every file it
changed (and the version recorded in incrmit.toml). The bump history is read
from a state file kept next to the config. A file that was edited since the
bump is left untouched and the undo is refused, so your changes are never
clobbered. Repeated undos walk back through successive bumps.

Flags:
` + undoFlags

// versionHelp documents the version command. It is shown by
// `incrmit version -h` and `incrmit help version`.
const versionHelp = `usage: incrmit version

Print the incrmit tool version. The --version, -version, and -v flags are
aliases for this command.
`

// helpHelp documents the help command itself (`incrmit help help`).
const helpHelp = `usage: incrmit help [command] [--no-banner]

Show the incrmit overview, or detailed help for a specific command
(bump, discover, preview, undo, version, or help). The overview is printed
under an ASCII banner; --no-banner leaves it out.

Flags:
` + helpFlags

// overview renders the top-level help. The banner is shown by default and only
// here: per-command help (for example `incrmit help discover`) never carries it,
// so drilling into a command stays terse. showBanner is false when the user
// passed --no-banner.
func overview(showBanner bool) string {
	if !showBanner {
		return overviewHelp
	}
	return banner + "\n" + overviewHelp
}

// parseBannerFlag removes the --no-banner opt-out from args and reports whether
// the banner should still be printed. It is handled here rather than through
// the flag package because the help paths take a bare topic name rather than a
// flag set, and the opt-out has to work in both `incrmit help --no-banner` and
// `incrmit -h --no-banner`.
func parseBannerFlag(args []string) ([]string, bool) {
	rest := make([]string, 0, len(args))
	showBanner := true
	for _, arg := range args {
		if arg == "--no-banner" || arg == "-no-banner" {
			showBanner = false
			continue
		}
		rest = append(rest, arg)
	}
	return rest, showBanner
}

// runHelp implements the `help` subcommand. With no argument it prints the
// top-level overview; with a command name it prints that command's help. An
// unknown command name is an error (exit code ExitUsage) with a hint.
func runHelp(args []string, stdout, stderr io.Writer) int {
	args, showBanner := parseBannerFlag(args)
	if len(args) == 0 {
		fprint(stdout, overview(showBanner))
		return ExitOK
	}
	switch args[0] {
	case "bump":
		fprint(stdout, bumpHelp)
	case "discover":
		fprint(stdout, discoverHelp)
	case "preview":
		fprint(stdout, previewHelp)
	case "undo":
		fprint(stdout, undoHelp)
	case "version":
		fprint(stdout, versionHelp)
	case "help":
		fprint(stdout, helpHelp)
	default:
		fprintf(stderr, "incrmit: unknown command %q\n", args[0])
		fprintln(stderr, "Run 'incrmit help' to see available commands.")
		return ExitUsage
	}
	return ExitOK
}
