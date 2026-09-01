package discovery

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/version"
)

// paths returns the (sorted) result paths for convenient assertions.
func paths(results []Result) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Path
	}
	sort.Strings(out)
	return out
}

// mustWriteNested writes a file at a (possibly nested) path, creating parent
// directories as needed.
func mustWriteNested(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// equalStrings reports whether two string slices have identical contents.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestIgnoreMatcher(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		rel      string
		isDir    bool
		want     bool
	}{
		{"bare dir name at root", []string{"testdata"}, "testdata", true, true},
		{"bare dir name nested", []string{"testdata"}, "a/b/testdata", true, true},
		{"bare name matches file too", []string{"testdata"}, "testdata", false, true},
		{"dir-only does not match file", []string{"testdata/"}, "testdata", false, false},
		{"dir-only matches dir", []string{"testdata/"}, "testdata", true, true},
		{"file glob any depth", []string{"*.lock"}, "sub/pkg.lock", false, true},
		{"file glob no match", []string{"*.lock"}, "sub/pkg.json", false, false},
		{"double star prunes dir", []string{"docs/**"}, "docs", true, true},
		{"double star matches nested", []string{"docs/**"}, "docs/api/x.md", false, true},
		{"double star no match outside", []string{"docs/**"}, "src/x.md", false, false},
		{"slash path exact", []string{"a/b"}, "a/b", true, true},
		{"slash path no partial", []string{"a/b"}, "a/bc", true, false},
		{"glob segment in path", []string{"a/*/z"}, "a/mid/z", false, true},
		{"no patterns", nil, "anything", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newIgnoreMatcher(tt.patterns)
			if got := m.match(tt.rel, tt.isDir); got != tt.want {
				t.Errorf("match(%q, dir=%v) with %v = %v, want %v", tt.rel, tt.isDir, tt.patterns, got, tt.want)
			}
		})
	}
}

// Patterns that reach the matcher after config loading has trimmed them, and
// paths that run out before a pattern does. None of these may match, and none
// may panic: the walk calls match on every entry it sees.
func TestIgnoreMatcherEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		rel      string
		isDir    bool
		want     bool
	}{
		// A pattern of only slashes trims to nothing and is dropped, rather
		// than becoming a rule that matches every path.
		{"slash-only pattern is dropped", []string{"/"}, "anything", true, false},
		{"whitespace-only pattern is dropped", []string{"  "}, "anything", false, false},
		// The path ends before the pattern does.
		{"pattern longer than path", []string{"a/b/c"}, "a/b", true, false},
		{"glob segment past the end", []string{"a/*"}, "a", true, false},
		// "**" matches zero or more segments, but the segments after it still
		// have to match something.
		{"double star tail unmatched", []string{"docs/**/api"}, "docs/x/y", false, false},
		{"double star matches zero segments", []string{"docs/**/api"}, "docs/api", true, true},
		{"leading double star any depth", []string{"**/vendor"}, "a/b/vendor", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newIgnoreMatcher(tt.patterns)
			if got := m.match(tt.rel, tt.isDir); got != tt.want {
				t.Errorf("match(%q, dir=%v) with %v = %v, want %v", tt.rel, tt.isDir, tt.patterns, got, tt.want)
			}
		})
	}
}

// A config with no ignore list leaves the matcher nil, and the walk calls it
// unconditionally, so nil has to behave as "matches nothing".
func TestIgnoreMatcherNil(t *testing.T) {
	var m *ignoreMatcher
	if !m.empty() {
		t.Error("empty() = false for a nil matcher, want true")
	}
	if m.match("anything", true) {
		t.Error("match() = true for a nil matcher, want false")
	}
}

// A directory named by an ignore pattern is pruned along with everything under
// it, while files elsewhere are still discovered.
func TestDiscoverIgnoresDirectory(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "VERSION", "1.2.3\n")
	mustWriteNested(t, root, "testdata/fixture.txt", "9.9.9\n")
	mustWriteNested(t, root, "testdata/nested/deep.txt", "8.8.8\n")

	got, err := Discover(root, "testdata/")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"VERSION"}; !equalStrings(paths(got), want) {
		t.Errorf("paths = %v, want %v", paths(got), want)
	}
}

// A file glob skips matching files anywhere in the tree.
func TestDiscoverIgnoresFileGlob(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "VERSION", "1.2.3\n")
	mustWrite(t, root, "deps.lock", "2.0.0\n")
	mustWriteNested(t, root, "sub/other.lock", "3.0.0\n")

	got, err := Discover(root, "*.lock")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"VERSION"}; !equalStrings(paths(got), want) {
		t.Errorf("paths = %v, want %v", paths(got), want)
	}
}

// A "dir/**" pattern prunes the directory and its whole subtree.
func TestDiscoverIgnoresDoubleStar(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "VERSION", "1.2.3\n")
	mustWriteNested(t, root, "docs/guide.md", "4.5.6\n")
	mustWriteNested(t, root, "docs/api/ref.md", "7.8.9\n")

	got, err := Discover(root, "docs/**")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"VERSION"}; !equalStrings(paths(got), want) {
		t.Errorf("paths = %v, want %v", paths(got), want)
	}
}

// Non-matching files are still discovered when ignore patterns are present.
func TestDiscoverIgnoreKeepsNonMatching(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "VERSION", "1.2.3\n")
	mustWrite(t, root, "keep.txt", "2.0.0\n")
	mustWriteNested(t, root, "skip/inside.txt", "9.9.9\n")

	got, err := Discover(root, "skip")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"VERSION", "keep.txt"}; !equalStrings(paths(got), want) {
		t.Errorf("paths = %v, want %v", paths(got), want)
	}
}

// Configured ignores combine with the built-in ignored dirs rather than
// replacing them.
func TestDiscoverIgnoreCombinesWithBuiltIns(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "VERSION", "1.2.3\n")
	mustWriteNested(t, root, "node_modules/pkg/VERSION", "9.9.9\n")
	mustWriteNested(t, root, "custom/ver.txt", "8.8.8\n")

	got, err := Discover(root, "custom")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"VERSION"}; !equalStrings(paths(got), want) {
		t.Errorf("paths = %v, want %v", paths(got), want)
	}
}

// Generate writes the ignore list back so it survives regeneration, and the
// result is a valid, loadable config.
func TestGeneratePreservesIgnore(t *testing.T) {
	results := []Result{
		{Path: "VERSION", Occurrences: []Occurrence{{Version: version.Version{Major: 1, Minor: 2, Patch: 3}}}},
	}
	data, err := Generate(results, "docs/**", "*.lock")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, `ignore = ["docs/**", "*.lock"]`) {
		t.Errorf("generated config missing ignore list:\n%s", out)
	}
	// The ignore array must come before the [[files]] table to be valid TOML.
	if strings.Index(out, "ignore =") > strings.Index(out, "[[files]]") {
		t.Errorf("ignore must be encoded before [[files]]:\n%s", out)
	}
}

// With no ignore patterns, the config carries a described, commented-out
// example rather than an active ignore key, and still loads cleanly.
func TestGenerateWithoutIgnoreWritesCommentedExample(t *testing.T) {
	results := []Result{
		{Path: "VERSION", Occurrences: []Occurrence{{Version: version.Version{Major: 1, Minor: 2, Patch: 3}}}},
	}
	data, err := Generate(results)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "# ignore:") {
		t.Errorf("generated config missing the ignore description comment:\n%s", out)
	}
	if !strings.Contains(out, `# ignore = ["testdata/", "*.lock", "docs/**"]`) {
		t.Errorf("generated config missing the commented ignore example:\n%s", out)
	}
	// No active (uncommented) ignore key should be present.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "ignore =") {
			t.Errorf("empty ignore list must not emit an active key: %q", line)
		}
	}
}

// A round trip: discover with an ignore, write the config, and reload it.
func TestDiscoverIgnoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "VERSION", "1.2.3\n")
	mustWriteNested(t, root, "docs/x.md", "2.0.0\n")

	results, err := Discover(root, "docs/")
	if err != nil {
		t.Fatal(err)
	}
	data, err := Generate(results, "docs/")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "incrmit.toml"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	ignore, err := config.LoadIgnore(filepath.Join(root, "incrmit.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"docs/"}; !equalStrings(ignore, want) {
		t.Errorf("reloaded ignore = %v, want %v", ignore, want)
	}
}
