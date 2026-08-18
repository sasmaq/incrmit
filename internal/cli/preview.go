package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/sasmaq/incrmit/internal/version"
)

// This file implements the read-only `preview` command: it shows, for every
// configured file, the version it holds today next to what a --patch, --minor,
// and --major bump would write, so all three outcomes are visible at once
// without running a --dry-run per component. It reads target files and the
// config and writes nothing at all — no target, no config, no history.

type previewOptions struct {
	configPath  string
	file        string
	maxFileSize int64 // per-file read cap in bytes; 0 means no limit
}

// previewRow is one line of the preview: a target, the version it holds today,
// and the three versions the component bumps would produce. inSync is false when
// the row's version differs from the one most entries hold (see markDrift).
type previewRow struct {
	path    string
	current version.Version
	patch   version.Version
	minor   version.Version
	major   version.Version
	inSync  bool
}

// driftMarker flags a row whose version differs from the one most entries hold.
const driftMarker = "*"

func runPreview(args []string, stdout, stderr io.Writer) int {
	opts, code := parsePreviewFlags(args, stdout, stderr)
	if code == exitSilentFlag {
		return ExitOK
	}
	if code != ExitOK {
		return code
	}

	targets, _, _, err := resolveTargets(opts.configPath, opts.file)
	if err != nil {
		fprintln(stderr, "incrmit:", err)
		return classify(err)
	}

	// Reuse the bump command's read-and-resolve pass so a preview reports the
	// same current version a bump would start from, including the config-pinned
	// token for a file that holds several.
	groups, code := readGroups(targets, opts.maxFileSize, stderr)
	if code != ExitOK {
		return code
	}

	rows := previewRows(groups)
	common, drifted := markDrift(rows)
	fprint(stdout, renderPreview(rows, common, drifted))
	return ExitOK
}

// previewRows flattens the read file groups into one row per (path, version),
// computing the three projected versions from the version each entry holds. A
// file listed once per distinct version it contains (see Milestone 20) yields
// one row per version; an exact (path, version) repeat is dropped so the same
// line is never printed twice. File and entry order is preserved.
func previewRows(groups []fileGroup) []previewRow {
	rows := make([]previewRow, 0, len(groups))
	seen := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		for _, e := range g.entries {
			key := g.display + "\x00" + e.oldVer.String()
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			// The bump methods carry the "v"/"V" prefix through and drop any
			// prerelease and build section, so every projected version is
			// exactly the token a real bump would write.
			rows = append(rows, previewRow{
				path:    g.display,
				current: e.oldVer,
				patch:   e.oldVer.BumpPatch(),
				minor:   e.oldVer.BumpMinor(),
				major:   e.oldVer.BumpMajor(),
				inSync:  true,
			})
		}
	}
	return rows
}

// markDrift flags every row whose version differs from the one most rows hold,
// so a file left behind by a partial bump stands out in the preview. It returns
// that most-common version and whether any row differs from it; when every row
// agrees, nothing is marked.
//
// Versions are compared by semver precedence, so a difference in the "v" prefix
// or in build metadata alone is not drift: v1.2.3 and 1.2.3 name the same
// release and flagging them would bury the rows that really are behind. The
// most common version wins; ties go to the higher version, which keeps the
// output deterministic and reads the trailing files as the stale ones.
func markDrift(rows []previewRow) (version.Version, bool) {
	if len(rows) < 2 {
		return version.Version{}, false
	}

	counts := make(map[string]int, len(rows))
	first := make(map[string]version.Version, len(rows))
	for _, r := range rows {
		key := precedenceKey(r.current)
		counts[key]++
		if _, ok := first[key]; !ok {
			first[key] = r.current
		}
	}
	if len(counts) < 2 {
		return version.Version{}, false
	}

	// Walk the rows rather than the counts map so the choice does not depend on
	// Go's randomized map iteration order.
	best := precedenceKey(rows[0].current)
	for _, r := range rows[1:] {
		key := precedenceKey(r.current)
		switch {
		case counts[key] > counts[best]:
			best = key
		case counts[key] == counts[best] && version.Compare(r.current, first[best]) > 0:
			best = key
		}
	}

	for i := range rows {
		rows[i].inSync = precedenceKey(rows[i].current) == best
	}
	return first[best], true
}

// precedenceKey renders the part of a version that decides semver precedence:
// the numeric components plus any prerelease, without the "v" prefix or the
// build metadata, which semver says do not affect precedence.
func precedenceKey(v version.Version) string {
	return version.Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch, Prerelease: v.Prerelease}.String()
}

// renderPreview formats the rows as a table with columns padded to their widest
// value, so the output lines up in a plain terminal with no tabs or escapes.
// When some row is out of sync, a marker column and a footnote naming the common
// version are added; otherwise the table stands alone.
func renderPreview(rows []previewRow, common version.Version, drifted bool) string {
	table := make([][]string, 0, len(rows)+1)
	header := []string{"PATH", "CURRENT", "PATCH", "MINOR", "MAJOR"}
	if drifted {
		header = append(header, "")
	}
	table = append(table, header)
	for _, r := range rows {
		cells := []string{r.path, r.current.String(), r.patch.String(), r.minor.String(), r.major.String()}
		if drifted {
			mark := ""
			if !r.inSync {
				mark = driftMarker
			}
			cells = append(cells, mark)
		}
		table = append(table, cells)
	}

	widths := make([]int, len(header))
	for _, cells := range table {
		for i, c := range cells {
			if len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}

	var b strings.Builder
	for _, cells := range table {
		var line strings.Builder
		for i, c := range cells {
			if i > 0 {
				line.WriteString("  ")
			}
			line.WriteString(c)
			line.WriteString(strings.Repeat(" ", widths[i]-len(c)))
		}
		// Padding on the final column (and an empty marker cell) would leave
		// trailing blanks on the line, which diff and copy-paste both dislike.
		b.WriteString(strings.TrimRight(line.String(), " "))
		b.WriteByte('\n')
	}
	if drifted {
		fmt.Fprintf(&b, "\n%s differs from %s, the version most entries hold\n", driftMarker, common)
	}
	return b.String()
}

// parsePreviewFlags registers the preview flags (long and short names) and
// parses args, returning the options and an exit code: ExitOK to proceed,
// ExitUsage on a parse error (help to stderr), or exitSilentFlag when -h/--help
// was requested (help to stdout).
func parsePreviewFlags(args []string, stdout, stderr io.Writer) (previewOptions, int) {
	var opts previewOptions
	fs := flag.NewFlagSet("incrmit preview", flag.ContinueOnError)
	fs.SetOutput(stderr)
	// Help and usage are rendered explicitly below from the centralized text
	// so an explicit -h/--help can go to stdout while errors go to stderr.
	fs.Usage = func() {}

	fs.StringVar(&opts.configPath, "config", "", "path to the TOML config file")
	fs.StringVar(&opts.configPath, "c", "", "path to the TOML config file (shorthand)")
	fs.StringVar(&opts.file, "file", "", "preview one file (skips config)")
	fs.StringVar(&opts.file, "f", "", "preview one file (shorthand)")
	maxFileSizeVar(fs, &opts.maxFileSize, 0, "refuse to read a target larger than this size")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fprint(stdout, previewHelp)
			return opts, exitSilentFlag
		}
		fprint(stderr, previewHelp)
		return opts, ExitUsage
	}
	if fs.NArg() > 0 {
		fprintf(stderr, "incrmit: unexpected argument %q\n", fs.Arg(0))
		fprint(stderr, previewHelp)
		return opts, ExitUsage
	}
	return opts, ExitOK
}
