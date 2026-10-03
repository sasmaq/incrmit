package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/testutil"
)

// These tests point discover's --output at a file that already exists. Before
// Milestone 36 nothing checked what was there: `discover -o NOTES.md` replaced
// a Markdown file with a generated config and exited 0, and LoadIgnore read a
// file that did not parse as "no ignore patterns" rather than as a mistake.
// --output may now replace only a missing file, an empty one, or an incrmit
// config; anything else is refused before the scan, with nothing written.

// outputProject is a tree with something for discover to find whichever of its
// files --output names, so a refusal is never just "no version found".
func outputProject(t *testing.T) string {
	t.Helper()
	return project(t, "", map[string]string{
		"VERSION":        "1.2.3\n",
		"NOTES.md":       "# Notes\n\nShip 1.2.3 after the freeze.\n",
		"HEADINGS.md":    "# Release notes\n\n## 1.2.3\n",
		"package.json":   "{\n  \"name\": \"app\",\n  \"version\": \"1.2.3\"\n}\n",
		"pyproject.toml": "[project]\nname = \"app\"\nversion = \"1.2.3\"\n",
		"tool.toml":      "[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\nowner = \"release\"\n",
	})
}

// Every file that is not an incrmit config is refused with exit 1, a message
// naming it and saying what to do, and its bytes unchanged, by a real run and
// by --dry-run alike.
func TestDiscoverRefusesUnrelatedOutput(t *testing.T) {
	for _, tc := range []struct {
		name, output, why string
	}{
		{"markdown", "NOTES.md", "line 3 is not TOML"},
		// Every line is a TOML comment, so it parses, but it sets nothing.
		{"markdown of headings", "HEADINGS.md", "it sets nothing"},
		// excludeOutput drops it from the results just before the write.
		{"a target", "VERSION", "line 1 is not TOML"},
		{"a JSON manifest", "package.json", "line 1 is not TOML"},
		{"another tool's TOML", "pyproject.toml", `"project"`},
		{"a key incrmit does not use", "tool.toml", `"files.owner"`},
	} {
		for _, extra := range [][]string{nil, {"--dry-run"}} {
			t.Run(tc.name+strings.Join(append([]string{""}, extra...), " "), func(t *testing.T) {
				dir := outputProject(t)
				out := filepath.Join(dir, tc.output)
				original, err := os.ReadFile(out)
				if err != nil {
					t.Fatal(err)
				}
				before := snapshotProject(t, dir)

				code, stdout, stderr := runMainWithin(t, append([]string{"discover", "-P", dir, "-o", out}, extra...)...)
				if code != ExitError {
					t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
				}
				if stdout != "" {
					t.Errorf("stdout = %q, want nothing", stdout)
				}
				for _, want := range []string{
					out + " exists and is not an incrmit config",
					tc.why,
					"choose another --output or remove the file",
				} {
					if !strings.Contains(stderr, want) {
						t.Errorf("stderr = %q, want it to contain %q", stderr, want)
					}
				}
				if got, err := os.ReadFile(out); err != nil || string(got) != string(original) {
					t.Errorf("%s = %q (err %v), want it unchanged: %q", tc.output, got, err, original)
				}
				assertSnapshotEqual(t, before, snapshotProject(t, dir))
			})
		}
	}
}

// The check runs before the scan, so a refused --output costs nothing: here
// the scan would have failed on a root that does not exist, and the refusal is
// what is reported instead.
func TestDiscoverRefusesOutputBeforeScanning(t *testing.T) {
	dir := outputProject(t)
	out := filepath.Join(dir, "NOTES.md")
	missing := filepath.Join(dir, "no-such-dir")

	code, _, stderr := runMainWithin(t, "discover", "-P", missing, "-o", out)
	if code != ExitError {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	if !strings.Contains(stderr, "is not an incrmit config") || strings.Contains(stderr, "no-such-dir") {
		t.Errorf("stderr = %q, want the refusal and nothing from the scan", stderr)
	}
}

// A named pipe at --output is refused by the same check, with the same advice.
func TestDiscoverRefusesFIFOOutput(t *testing.T) {
	dir := outputProject(t)
	out := filepath.Join(dir, config.DefaultPath)
	if err := testutil.Mkfifo(out); err != nil {
		t.Skipf("FIFOs unsupported: %v", err)
	}

	code, _, stderr := runMainWithin(t, "discover", "-P", dir, "-o", out)
	if code != ExitError {
		t.Errorf("exit = %d, want %d (stderr %q)", code, ExitError, stderr)
	}
	for _, want := range []string{out + " exists and is not an incrmit config", "not a regular file", "choose another --output"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr = %q, want it to contain %q", stderr, want)
		}
	}
}

// What --output may replace: nothing at all, an empty file, or a config
// incrmit wrote or could have. Each is regenerated as before.
func TestDiscoverReplacesConfigOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		body *string
		keep []string // ignore patterns the regenerated config must carry
	}{
		{name: "no file", body: nil},
		{name: "an empty file", body: ptr("")},
		{name: "a blank file", body: ptr("\n  \n\t\n")},
		{name: "a config discover wrote", body: ptr("# incrmit.toml (maintained by incrmit)\n\n" +
			config.IgnoreComment(true) + "ignore = [\"docs/\"]\n\n[[files]]\n  path = \"VERSION\"\n  version = \"1.0.0\"\n"),
			keep: []string{"docs/"}},
		{name: "a hand-written config", body: ptr("[[files]]\npath = \"VERSION\"\n")},
		{name: "an ignore list alone", body: ptr("ignore = [\"*.md\"]\n"), keep: []string{"*.md"}},
		{name: "a stale config", body: ptr("[[files]]\npath = \"gone.txt\"\nversion = \"0.0.1\"\n")},
		{name: "the split prerelease keys", body: ptr("[[files]]\npath = \"VERSION\"\nversion = \"1.0.0\"\nprerelease = \"rc.1\"\nbuild = \"7\"\n")},
		{name: "an inline prerelease", body: ptr("[[files]]\npath = \"VERSION\"\nversion = \"1.0.0-rc.1\"\n")},
	} {
		for _, extra := range [][]string{nil, {"--dry-run"}} {
			t.Run(tc.name+strings.Join(append([]string{""}, extra...), " "), func(t *testing.T) {
				dir := project(t, "", map[string]string{"VERSION": "1.2.3\n"})
				out := filepath.Join(dir, config.DefaultPath)
				if tc.body != nil {
					if err := os.WriteFile(out, []byte(*tc.body), 0o644); err != nil {
						t.Fatal(err)
					}
				}

				code, _, stderr := runMainWithin(t, append([]string{"discover", "-P", dir, "-o", out}, extra...)...)
				if code != ExitOK {
					t.Fatalf("exit = %d, stderr = %q", code, stderr)
				}
				if len(extra) > 0 {
					return
				}
				cfg, err := config.Load(out)
				if err != nil {
					t.Fatalf("loading the regenerated config: %v", err)
				}
				if len(cfg.Files) != 1 || cfg.Files[0].Path != "VERSION" || cfg.Files[0].Version != "1.2.3" {
					t.Errorf("files = %+v, want VERSION at 1.2.3", cfg.Files)
				}
				if strings.Join(cfg.Ignore, ",") != strings.Join(tc.keep, ",") {
					t.Errorf("ignore = %q, want %q", cfg.Ignore, tc.keep)
				}
			})
		}
	}
}

func ptr(s string) *string { return &s }
