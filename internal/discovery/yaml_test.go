package discovery

import (
	"slices"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/files"
)

// The YAML manifests discover knows get an entry for the project's own
// version alone, and every other candidate is skipped as not the project's. A
// Helm chart has two project keys, the chart's version and the application's
// appVersion, and gets an entry for each it holds.
func TestDiscoverKnownYAMLManifests(t *testing.T) {
	tests := []struct {
		path, body string
		project    []string
		want       []string
		skipped    []string
	}{
		{"Chart.yaml", "version: 0.7.3\nappVersion: \"1.4.2\"\ndependencies:\n  - name: db\n    version: 12.5.6\n",
			[]string{"version", "appVersion"}, []string{"version=0.7.3", "appVersion=1.4.2"}, []string{"dependencies[0].version"}},
		{"lib/Chart.yaml", "type: library\nversion: 0.1.0\n",
			[]string{"version", "appVersion"}, []string{"version=0.1.0"}, nil},
		{"pubspec.yaml", "version: 2.3.1+45\ndependencies:\n  http: 1.2.1\n",
			[]string{"version"}, []string{"version=2.3.1+45"}, []string{"dependencies.http"}},
		{"snap/snapcraft.yaml", "name: x\nversion: '2.3.1'\nparts:\n  x:\n    source-tag: v2.3.1\n",
			[]string{"version"}, []string{"version=2.3.1"}, []string{"parts.x.source-tag"}},
		{"galaxy.yml", "namespace: x\nversion: 1.0.0\ndependencies:\n  community.general: 7.0.0\n",
			[]string{"version"}, []string{"version=1.0.0"}, []string{`dependencies."community.general"`}},
		{"CITATION.cff", "cff-version: 1.2.0\ntitle: x\nversion: 2.0.4\n",
			[]string{"version"}, []string{"version=2.0.4"}, []string{"cff-version"}},
		// A pubspec with no version of its own, such as an app never
		// published, gets no entry rather than one per dependency.
		{"app/pubspec.yaml", "name: app\ndependencies:\n  http: 1.2.1\n", []string{"version"}, nil, []string{"dependencies.http"}},
		// Not a known manifest, and read as YAML all the same.
		{"values.yaml", "image:\n  tag: 1.4.2\nreplicas: 2\n", nil, []string{"image.tag=1.4.2"}, nil},
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
		if r.Parser != files.ParserYAML || r.Fallback != nil {
			t.Errorf("%s: parser %q, fallback %v; want YAML", tt.path, r.Parser, r.Fallback)
		}
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

// A YAML file of more than one document, and one that is not YAML at all, is
// scanned as text, with the reason kept for the dry run and parser = "text"
// written into its entries so a bump reads it the way discover did.
func TestDiscoverYAMLFallsBackToText(t *testing.T) {
	root := writeTree(t, map[string]string{
		"deploy.yaml": "kind: Deployment\nimage: ghcr.io/me/app:1.4.2\n---\nkind: Service\nversion: \"1.4.2\"\n",
		"meta.yaml":   "{% set version = \"1.2.3\" %}\npackage:\n  version: {{ version }}\n",
		"ci.yml":      "steps:\n  - run: go install x@v1.6.0 # v7.0.1\n",
	})
	results, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %+v, want deploy.yaml and meta.yaml (ci.yml holds no whole version)", results)
	}
	for path, want := range map[string]struct {
		err string
		occ []string
	}{
		"deploy.yaml": {"2 YAML documents", []string{"L2 =1.4.2", "L5 =1.4.2"}},
		"meta.yaml":   {"not YAML (found character that cannot start any token)", []string{"L1 =1.2.3"}},
	} {
		r := resultFor(t, results, path)
		if r.Parser != files.ParserText || r.Fallback == nil || r.Fallback.Parser != files.ParserYAML {
			t.Fatalf("%s: parser %q, fallback %v; want text, falling back from YAML", path, r.Parser, r.Fallback)
		}
		if r.Fallback.Error() != want.err {
			t.Errorf("%s: fallback reason %q, want %q", path, r.Fallback, want.err)
		}
		if got := keyed(r); !slices.Equal(got, want.occ) {
			t.Errorf("%s: occurrences %v, want %v", path, got, want.occ)
		}
	}
	data, err := Generate(results)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), `parser = "text"`); got != 2 {
		t.Errorf("config records parser = \"text\" %d times, want twice:\n%s", got, data)
	}
}

// Each occurrence lists the aliases that read its value, which a bump of it
// changes too, so the dry run can show them.
func TestDiscoverYAMLAliases(t *testing.T) {
	root := writeTree(t, map[string]string{
		"values.yaml": "base: &v 1.2.3\npinned: *v\ndefaults: &d\n  tag: 2.0.0\nprod:\n  <<: *d\nplain: 3.0.0\n",
	})
	results, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"base": {"pinned"}, "defaults.tag": {`prod."<<"`}, "plain": nil}
	r := resultFor(t, results, "values.yaml")
	if len(r.Occurrences) != len(want) {
		t.Fatalf("occurrences %v, want %v", keyed(r), want)
	}
	for _, o := range r.Occurrences {
		var got []string
		for _, a := range o.Aliases {
			got = append(got, a.String())
		}
		if w := want[o.Key.String()]; !slices.Equal(got, w) {
			t.Errorf("%s: aliases %v, want %v", o.Key, got, w)
		}
	}
	// The aliases are not entries of their own.
	if got := entries(r); !slices.Equal(got, []string{"base=1.2.3", "defaults.tag=2.0.0", "plain=3.0.0"}) {
		t.Errorf("entries %v", got)
	}
}
