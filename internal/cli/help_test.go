package cli

import (
	"bytes"
	"strings"
	"testing"
)

// runHelpCmd invokes runHelp with the given args and returns the exit code plus
// captured stdout/stderr.
func runHelpCmd(args ...string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := runHelp(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// runHelp with no argument prints the banner and the top-level overview to
// stdout (exit 0).
func TestRunHelpOverview(t *testing.T) {
	code, stdout, stderr := runHelpCmd()
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	if stdout != overview(true) {
		t.Errorf("stdout = %q, want the banner followed by overviewHelp", stdout)
	}
	if !strings.HasPrefix(stdout, banner) {
		t.Errorf("stdout does not start with the banner:\n%s", stdout)
	}
	if !strings.Contains(stdout, overviewHelp) {
		t.Errorf("stdout does not contain overviewHelp:\n%s", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

// --no-banner drops the banner but leaves the overview itself untouched, on
// both `incrmit help` and (via parseBannerFlag) top-level -h / --help.
func TestRunHelpNoBanner(t *testing.T) {
	for _, flag := range []string{"--no-banner", "-no-banner"} {
		t.Run(flag, func(t *testing.T) {
			code, stdout, stderr := runHelpCmd(flag)
			if code != ExitOK {
				t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitOK, stderr)
			}
			if stdout != overviewHelp {
				t.Errorf("stdout = %q, want overviewHelp with no banner", stdout)
			}
			if strings.Contains(stdout, banner) {
				t.Errorf("stdout still contains the banner:\n%s", stdout)
			}
		})
	}
}

// The banner must be plain ASCII and narrow enough for an 80-column terminal,
// so it renders the same everywhere without Unicode or color support.
func TestBannerIsPlainASCIIAndFits(t *testing.T) {
	const maxWidth = 80
	if !strings.HasSuffix(banner, "\n") {
		t.Errorf("banner does not end with a newline")
	}
	for i, line := range strings.Split(strings.TrimSuffix(banner, "\n"), "\n") {
		if len(line) > maxWidth {
			t.Errorf("banner line %d is %d columns, want <= %d: %q", i+1, len(line), maxWidth, line)
		}
		for _, r := range line {
			if r < 0x20 || r > 0x7e {
				t.Errorf("banner line %d contains non-ASCII rune %q: %q", i+1, r, line)
			}
		}
	}
}

// The banner belongs to the overview only: per-command help stays terse.
func TestPerCommandHelpHasNoBanner(t *testing.T) {
	for _, cmd := range []string{"bump", "discover", "undo", "version", "help"} {
		t.Run(cmd, func(t *testing.T) {
			code, stdout, _ := runHelpCmd(cmd)
			if code != ExitOK {
				t.Fatalf("exit = %d, want %d", code, ExitOK)
			}
			if strings.Contains(stdout, banner) {
				t.Errorf("%s help contains the banner:\n%s", cmd, stdout)
			}
		})
	}
}

// runHelp <command> prints exactly that command's centralized help to stdout.
func TestRunHelpPerCommand(t *testing.T) {
	tests := []struct {
		command string
		want    string
	}{
		{"bump", bumpHelp},
		{"discover", discoverHelp},
		{"undo", undoHelp},
		{"version", versionHelp},
		{"help", helpHelp},
	}
	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			code, stdout, stderr := runHelpCmd(tt.command)
			if code != ExitOK {
				t.Fatalf("exit = %d, want %d", code, ExitOK)
			}
			if stdout != tt.want {
				t.Errorf("stdout = %q, want %q", stdout, tt.want)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
		})
	}
}

// An unknown command name is a usage error: nothing on stdout, a clear message
// plus a hint on stderr, and exit code ExitUsage.
func TestRunHelpUnknownCommand(t *testing.T) {
	code, stdout, stderr := runHelpCmd("bogus")
	if code != ExitUsage {
		t.Fatalf("exit = %d, want %d", code, ExitUsage)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on error", stdout)
	}
	if !strings.Contains(stderr, `unknown command "bogus"`) {
		t.Errorf("stderr = %q, want unknown-command message naming the command", stderr)
	}
	if !strings.Contains(stderr, "incrmit help") {
		t.Errorf("stderr = %q, want hint to run 'incrmit help'", stderr)
	}
}

// Only the first argument selects the help topic; extra arguments are ignored.
func TestRunHelpIgnoresExtraArgs(t *testing.T) {
	code, stdout, stderr := runHelpCmd("discover", "extra", "args")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d (stderr %q)", code, ExitOK, stderr)
	}
	if stdout != discoverHelp {
		t.Errorf("stdout = %q, want discoverHelp", stdout)
	}
}

// The overview must name every command so users can discover them.
func TestOverviewListsAllCommands(t *testing.T) {
	for _, cmd := range []string{"incrmit [flags]", "incrmit discover", "incrmit undo", "incrmit version", "incrmit help"} {
		if !strings.Contains(overviewHelp, cmd) {
			t.Errorf("overviewHelp missing %q:\n%s", cmd, overviewHelp)
		}
	}
}

// The overview must also list the available flags (not just the commands) so
// users can discover them from the top-level help without drilling in.
func TestOverviewListsFlags(t *testing.T) {
	for _, flag := range []string{
		"-c, --config", "-f, --file", "-M, --major", "-m, --minor",
		"-p, --patch", "-d, --dry-run", "-P, --path", "-o, --output",
		"--no-banner",
	} {
		if !strings.Contains(overviewHelp, flag) {
			t.Errorf("overviewHelp missing flag %q:\n%s", flag, overviewHelp)
		}
	}
}

// The overview must reuse the centralized flag blocks verbatim so the flag text
// stays in sync with each command's help (no duplicated flag strings).
func TestOverviewReusesFlagBlocks(t *testing.T) {
	if !strings.Contains(overviewHelp, bumpFlags) {
		t.Errorf("overviewHelp does not embed bumpFlags verbatim:\n%s", overviewHelp)
	}
	if !strings.Contains(overviewHelp, discoverFlags) {
		t.Errorf("overviewHelp does not embed discoverFlags verbatim:\n%s", overviewHelp)
	}
	if !strings.Contains(overviewHelp, undoFlags) {
		t.Errorf("overviewHelp does not embed undoFlags verbatim:\n%s", overviewHelp)
	}
	if !strings.Contains(overviewHelp, helpFlags) {
		t.Errorf("overviewHelp does not embed helpFlags verbatim:\n%s", overviewHelp)
	}
	if !strings.Contains(helpHelp, helpFlags) {
		t.Errorf("helpHelp does not embed helpFlags verbatim:\n%s", helpHelp)
	}
	if !strings.Contains(bumpHelp, bumpFlags) {
		t.Errorf("bumpHelp does not embed bumpFlags verbatim:\n%s", bumpHelp)
	}
	if !strings.Contains(discoverHelp, discoverFlags) {
		t.Errorf("discoverHelp does not embed discoverFlags verbatim:\n%s", discoverHelp)
	}
}

// Every help string should start with usage-oriented text and end with a
// trailing newline so output is well-formed regardless of how it is printed.
func TestHelpTextWellFormed(t *testing.T) {
	texts := map[string]string{
		"overviewHelp": overviewHelp,
		"overview":     overview(true),
		"bumpHelp":     bumpHelp,
		"discoverHelp": discoverHelp,
		"undoHelp":     undoHelp,
		"versionHelp":  versionHelp,
		"helpHelp":     helpHelp,
	}
	for name, text := range texts {
		if text == "" {
			t.Errorf("%s is empty", name)
			continue
		}
		if !strings.HasSuffix(text, "\n") {
			t.Errorf("%s does not end with a newline", name)
		}
		if !strings.Contains(text, "usage:") {
			t.Errorf("%s missing a usage line:\n%s", name, text)
		}
	}
}

// Each command help should document the flags it accepts so help stays a
// complete reference (and catches a flag added without updating help text).
func TestCommandHelpListsFlags(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		flags []string
	}{
		{"bumpHelp", bumpHelp, []string{"--config", "--file", "--major", "--minor", "--patch", "--dry-run"}},
		{"discoverHelp", discoverHelp, []string{"--path", "--output", "--dry-run"}},
		{"undoHelp", undoHelp, []string{"--config", "--dry-run"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, f := range tt.flags {
				if !strings.Contains(tt.text, f) {
					t.Errorf("%s missing flag %q:\n%s", tt.name, f, tt.text)
				}
			}
		})
	}
}
