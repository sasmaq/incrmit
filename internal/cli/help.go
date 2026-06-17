package cli

import "io"

// This file is the single source of truth for incrmit's usage and help text.
// Every command (bump, discover, version, and help) references the constants
// below so the `-h` / `--help` output and the `incrmit help` output stay in
// sync and are never duplicated across separate fs.Usage strings.

// overviewHelp is the top-level summary printed by `incrmit help` and by
// top-level `-h` / `--help` (with no subcommand).
const overviewHelp = `incrmit — increment semantic versions across one or more files

usage:
  incrmit [flags]            bump the version in the configured files (default)
  incrmit discover [flags]   scan the tree for version-bearing files and write a config
  incrmit version            print the incrmit tool version
  incrmit help [command]     show this overview, or help for a specific command

Run "incrmit help <command>" for details, for example "incrmit help discover".
`

// bumpHelp documents the default bump command. It is shown by `incrmit -h`
// (in bump context), `incrmit help bump`, and on a bump usage error.
const bumpHelp = `usage: incrmit [flags]

Bump the semantic version in the configured files.

Flags:
  -c, --config string  path to the TOML config file (default "incrmit.toml")
  -f, --file string    bump the version in one file (skips config)
  -M, --major          bump the major version (resets minor and patch)
  -m, --minor          bump the minor version (resets patch)
  -p, --patch          bump the patch version (default)
  -d, --dry-run        print the new version without writing
`

// discoverHelp documents the discover command. It is shown by
// `incrmit discover -h`, `incrmit help discover`, and on a discover usage error.
const discoverHelp = `usage: incrmit discover [flags]

Scan a directory tree for version-bearing files and generate a config.

Flags:
  -P, --path string    root directory to scan (default ".")
  -o, --output string  path to write the generated config (default "incrmit.toml")
  -d, --dry-run        print discovered files without writing the config
`

// versionHelp documents the version command. It is shown by
// `incrmit version -h` and `incrmit help version`.
const versionHelp = `usage: incrmit version

Print the incrmit tool version. The --version, -version, and -v flags are
aliases for this command.
`

// helpHelp documents the help command itself (`incrmit help help`).
const helpHelp = `usage: incrmit help [command]

Show the incrmit overview, or detailed help for a specific command
(bump, discover, version, or help).
`

// runHelp implements the `help` subcommand. With no argument it prints the
// top-level overview; with a command name it prints that command's help. An
// unknown command name is an error (exit code ExitUsage) with a hint.
func runHelp(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fprint(stdout, overviewHelp)
		return ExitOK
	}
	switch args[0] {
	case "bump":
		fprint(stdout, bumpHelp)
	case "discover":
		fprint(stdout, discoverHelp)
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
