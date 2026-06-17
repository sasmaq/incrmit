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

// runHelp with no argument prints the top-level overview to stdout (exit 0).
func TestRunHelpOverview(t *testing.T) {
	code, stdout, stderr := runHelpCmd()
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}
	if stdout != overviewHelp {
		t.Errorf("stdout = %q, want overviewHelp", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
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
	for _, cmd := range []string{"incrmit [flags]", "incrmit discover", "incrmit version", "incrmit help"} {
		if !strings.Contains(overviewHelp, cmd) {
			t.Errorf("overviewHelp missing %q:\n%s", cmd, overviewHelp)
		}
	}
}

// Every help string should start with usage-oriented text and end with a
// trailing newline so output is well-formed regardless of how it is printed.
func TestHelpTextWellFormed(t *testing.T) {
	texts := map[string]string{
		"overviewHelp": overviewHelp,
		"bumpHelp":     bumpHelp,
		"discoverHelp": discoverHelp,
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
