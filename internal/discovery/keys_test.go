package discovery

import (
	"slices"
	"strings"
	"testing"
)

// entries renders the entries a result gives as "key=version", the key empty
// for an entry without one.
func entries(r Result) []string {
	var out []string
	for _, e := range r.Entries() {
		out = append(out, e.KeyText()+"="+e.Token())
	}
	return out
}

// In a manifest discover knows, the one entry is for the project's own
// version, at its key, and every other candidate is skipped as not the
// project's. A known manifest with no version at any of its project keys, such
// as a private monorepo root, gets no entry rather than one per dependency.
// The name must match exactly, as each tool requires.
func TestDiscoverKnownManifests(t *testing.T) {
	deps := `"dependencies": {"left-pad": "1.0.0"}`
	tests := []struct {
		path, body string
		project    []string
		want       []string
		skipped    []string
	}{
		{"package.json", `{"version": "1.2.3", ` + deps + `}`, []string{"version"}, []string{"version=1.2.3"}, []string{"dependencies.left-pad"}},
		{"composer.json", `{"version": "1.2.3", "require": {"php": "8.3.0"}}`, []string{"version"}, []string{"version=1.2.3"}, []string{"require.php"}},
		{"deno.json", `{"version": "v1.2.3", "imports": {"std": "0.224.0"}}`, []string{"version"}, []string{"version=v1.2.3"}, []string{"imports.std"}},
		{"jsr.json", `{"name": "@x/y", "version": "1.2.3-rc.1"}`, []string{"version"}, []string{"version=1.2.3-rc.1"}, nil},
		{"Cargo.toml", "[package]\nversion = \"0.4.0\"\n[dependencies]\nserde = \"1.0.200\"\n",
			[]string{"package.version", "workspace.package.version"}, []string{"package.version=0.4.0"}, []string{"dependencies.serde"}},
		{"ws/Cargo.toml", "[workspace.package]\nversion = \"0.4.0\"\n[workspace.dependencies]\nserde = \"1.0.200\"\n",
			[]string{"package.version", "workspace.package.version"}, []string{"workspace.package.version=0.4.0"}, []string{"workspace.dependencies.serde"}},
		{"pyproject.toml", "[project]\nversion = \"2.1.0\"\n[tool.x]\nv = \"2.1.0\"\n",
			[]string{"project.version", "tool.poetry.version"}, []string{"project.version=2.1.0"}, []string{"tool.x.v"}},
		{"poetry/pyproject.toml", "[tool.poetry]\nversion = \"2.1.0\"\n[tool.poetry.dependencies]\nclick = \"8.1.7\"\n",
			[]string{"project.version", "tool.poetry.version"}, []string{"tool.poetry.version=2.1.0"}, []string{"tool.poetry.dependencies.click"}},
		// Both of Poetry 2's spellings are the project's version.
		{"both/pyproject.toml", "[project]\nversion = \"2.1.0\"\n[tool.poetry]\nversion = \"2.1.0\"\n",
			[]string{"project.version", "tool.poetry.version"}, []string{"project.version=2.1.0", "tool.poetry.version=2.1.0"}, nil},
		{"private/package.json", `{"private": true, ` + deps + `}`, []string{"version"}, nil, []string{"dependencies.left-pad"}},
		// Cargo's workspace inheritance leaves no version at package.version.
		{"member/Cargo.toml", "[package]\nversion.workspace = true\n[dependencies]\nserde = \"1.0.200\"\n",
			[]string{"package.version", "workspace.package.version"}, nil, []string{"dependencies.serde"}},
		// Not a known manifest: every candidate gets an entry of its own. (In
		// a directory of its own, since macOS file names ignore case.)
		{"other/Package.json", `{"version": "1.2.3", ` + deps + `}`, nil, []string{"version=1.2.3", "dependencies.left-pad=1.0.0"}, nil},
	}
	tree := map[string]string{}
	for _, tt := range tests {
		tree[tt.path] = tt.body
	}
	results, err := Discover(writeTree(t, tree))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		r := resultFor(t, results, tt.path)
		if !slices.Equal(r.ProjectKeys, tt.project) {
			t.Errorf("%s: project keys %v, want %v", tt.path, r.ProjectKeys, tt.project)
		}
		if got := entries(r); !slices.Equal(got, tt.want) {
			t.Errorf("%s: entries %v, want %v", tt.path, got, tt.want)
		}
		var skipped []string
		for _, o := range r.Occurrences {
			if o.Skip == NotProject {
				skipped = append(skipped, o.Key.String())
			}
		}
		if !slices.Equal(skipped, tt.skipped) {
			t.Errorf("%s: skipped %v, want %v", tt.path, skipped, tt.skipped)
		}
	}
}

// A file discover does not know gets one entry per candidate, each keyed to
// its own value, so each can be deleted on its own. A candidate whose key
// names more than one value, as a JSON key written twice does, gets none,
// since no key could name it alone. A text file keeps one entry per version,
// with no key, and so does a .json file scanned as text.
func TestGenerateKeysEachCandidate(t *testing.T) {
	root := writeTree(t, map[string]string{
		"appsettings.json": `{"App": {"Version": "1.2.3", "Api": {"Version": "1.2.3"}}, "Tools": ["4.5.6"], "dup": "7.7.7", "dup": "7.7.7", "ok.v": "8.8.8"}`,
		"tsconfig.json":    "{\n  // c\n  \"version\": \"1.2.3\", \"x\": \"1.2.3\"\n}\n",
		"VERSION.txt":      "1.2.3 and 1.2.3\n",
		"root.json":        `"9.9.9"`,
	})
	results, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"appsettings.json": {"App.Version=1.2.3", "App.Api.Version=1.2.3", "Tools[0]=4.5.6", `"ok.v"=8.8.8`},
		"tsconfig.json":    {"=1.2.3"},
		"VERSION.txt":      {"=1.2.3"},
		"root.json":        {"=9.9.9"},
	}
	for path, w := range want {
		if got := entries(resultFor(t, results, path)); !slices.Equal(got, w) {
			t.Errorf("%s: entries %v, want %v", path, got, w)
		}
	}
	var repeated []string
	for _, o := range resultFor(t, results, "appsettings.json").Occurrences {
		if o.Skip == RepeatedKey {
			repeated = append(repeated, o.Key.String())
		}
	}
	if !slices.Equal(repeated, []string{"dup", "dup"}) {
		t.Errorf("repeated keys %v, want both of dup's values", repeated)
	}

	data, err := Generate(results)
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(data)
	for _, want := range []string{
		"  path = \"appsettings.json\"\n  version = \"1.2.3\"\n  key = \"App.Api.Version\"\n",
		"  path = \"appsettings.json\"\n  version = \"8.8.8\"\n  key = \"\\\"ok.v\\\"\"\n",
		"  path = \"tsconfig.json\"\n  version = \"1.2.3\"\n  parser = \"text\"\n",
		"  path = \"root.json\"\n  version = \"9.9.9\"\n\n",
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("generated config lacks\n%s\nin\n%s", want, cfg)
		}
	}
}
