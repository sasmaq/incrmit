package discovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sasmaq/incrmit/internal/config"
	"github.com/sasmaq/incrmit/internal/testutil"
	"github.com/sasmaq/incrmit/internal/version"
)

// firstVersion returns the version of a result's first occurrence, a common
// assertion for files that contain a single version.
func firstVersion(r Result) version.Version {
	return r.Occurrences[0].Version
}

// distinct returns the distinct versions in a result, in first-seen order.
func distinct(r Result) []version.Version {
	seen := make(map[string]struct{}, len(r.Occurrences))
	var out []version.Version
	for _, o := range r.Occurrences {
		s := o.Version.String()
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, o.Version)
	}
	return out
}

// fixtureTree writes a tree exercising content-based detection across arbitrary
// file names and types, plus files and directories that must be skipped, and
// returns the root.
func fixtureTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"VERSION":              "1.2.3\n",
		"package.json":         `{"name":"x","version":"2.0.1","dependencies":{"left-pad":"1.0.0"}}`,
		"pyproject.toml":       "[project]\nname = \"x\"\nversion = \"3.4.5\"\n",
		"sub/Cargo.toml":       "[package]\nname = \"x\"\nversion = \"0.1.9\"\n",
		"sub/build_info.go":    "package build\n\nconst Version = \"7.8.9\"\n",
		"notes.txt":            "Released version 5.6.7 yesterday.\n",
		"app.config":           "endpoint=https://x\nversion=10.20.30\n",
		"Dockerfile":           "FROM alpine:3.19\nLABEL version=\"8.0.1\"\n",
		"README.md":            "no version token of interest here\n",
		"empty/marker":         "not a version\n",
		".git/VERSION":         "9.9.9\n",
		"node_modules/VERSION": "9.9.9\n",
		"vendor/pkg/VERSION":   "9.9.9\n",
		"dist/VERSION":         "9.9.9\n",
	}
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A binary file containing a version-like byte sequence must be ignored.
	if err := os.WriteFile(filepath.Join(root, "logo.png"), []byte("\x89PNG\x00\x00 4.5.6"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDiscover(t *testing.T) {
	root := fixtureTree(t)
	got, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	// Detection is content-based and works for any file name. Package.json
	// contains two versions (2.0.1 and the dependency's 1.0.0), so it is
	// reported with both occurrences; the Dockerfile's "3.19" is not a full
	// version, so only 8.0.1 is found. Results are sorted by path.
	want := map[string][]version.Version{
		"Dockerfile":        {{Major: 8, Minor: 0, Patch: 1}},
		"VERSION":           {{Major: 1, Minor: 2, Patch: 3}},
		"app.config":        {{Major: 10, Minor: 20, Patch: 30}},
		"notes.txt":         {{Major: 5, Minor: 6, Patch: 7}},
		"package.json":      {{Major: 2, Minor: 0, Patch: 1}, {Major: 1, Minor: 0, Patch: 0}},
		"pyproject.toml":    {{Major: 3, Minor: 4, Patch: 5}},
		"sub/Cargo.toml":    {{Major: 0, Minor: 1, Patch: 9}},
		"sub/build_info.go": {{Major: 7, Minor: 8, Patch: 9}},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d results, want %d: %+v", len(got), len(want), got)
	}
	for _, r := range got {
		wantVers, ok := want[r.Path]
		if !ok {
			t.Errorf("unexpected result for %q: %+v", r.Path, r)
			continue
		}
		gotVers := distinct(r)
		if len(gotVers) != len(wantVers) {
			t.Errorf("%q: got versions %v, want %v", r.Path, gotVers, wantVers)
			continue
		}
		for i := range wantVers {
			if gotVers[i] != wantVers[i] {
				t.Errorf("%q version[%d] = %v, want %v", r.Path, i, gotVers[i], wantVers[i])
			}
		}
	}
}

// The config file itself must never be reported as a target, even though it is
// a text file containing version tokens.
func TestDiscoverSkipsConfigFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "VERSION"), []byte("1.2.3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, config.DefaultPath), []byte("[[files]]\npath = \"VERSION\"\nversion = \"1.2.3\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got {
		if r.Path == config.DefaultPath {
			t.Fatalf("Discover returned the config file: %+v", got)
		}
	}
	if len(got) != 1 || got[0].Path != "VERSION" {
		t.Errorf("got %+v, want only VERSION", got)
	}
}

func TestDiscoverSkipsIgnoredDirs(t *testing.T) {
	root := fixtureTree(t)
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got {
		for _, ignored := range []string{".git", "node_modules", "vendor", "dist"} {
			if strings.HasPrefix(r.Path, ignored+"/") {
				t.Errorf("result %q is inside ignored dir %q", r.Path, ignored)
			}
		}
		for _, o := range r.Occurrences {
			if o.Version == (version.Version{Major: 9, Minor: 9, Patch: 9}) {
				t.Errorf("picked up a 9.9.9 version from an ignored dir: %+v", r)
			}
		}
	}
}

func TestDiscoverArbitraryFileName(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "anything.weirdext", "the build is 4.5.6 now\n")
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "anything.weirdext" ||
		firstVersion(got[0]) != (version.Version{Major: 4, Minor: 5, Patch: 6}) {
		t.Errorf("got %+v, want one 4.5.6 result for anything.weirdext", got)
	}
}

// Every version token in a file is captured, in the order it appears, each with
// its line number.
func TestDiscoverAllMatches(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "f", "current 1.2.3\nand later 9.9.9\n")
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1: %+v", len(got), got)
	}
	occ := got[0].Occurrences
	if len(occ) != 2 {
		t.Fatalf("got %d occurrences, want 2: %+v", len(occ), occ)
	}
	if occ[0].Version != (version.Version{Major: 1, Minor: 2, Patch: 3}) || occ[0].Line != 1 {
		t.Errorf("occurrence[0] = %+v, want 1.2.3 on line 1", occ[0])
	}
	if occ[1].Version != (version.Version{Major: 9, Minor: 9, Patch: 9}) || occ[1].Line != 2 {
		t.Errorf("occurrence[1] = %+v, want 9.9.9 on line 2", occ[1])
	}
}

// Repeated identical versions collapse to a single distinct version but every
// occurrence is still recorded (with its own line number).
func TestDiscoverIdenticalRepeats(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "f", "v = 1.2.3\nsee 1.2.3 again\nand 1.2.3\n")
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || len(got[0].Occurrences) != 3 {
		t.Fatalf("got %+v, want 1 file with 3 occurrences", got)
	}
	if d := distinct(got[0]); len(d) != 1 || d[0] != (version.Version{Major: 1, Minor: 2, Patch: 3}) {
		t.Errorf("distinct versions = %v, want [1.2.3]", d)
	}
	for i, wantLine := range []int{1, 2, 3} {
		if got[0].Occurrences[i].Line != wantLine {
			t.Errorf("occurrence[%d] line = %d, want %d", i, got[0].Occurrences[i].Line, wantLine)
		}
	}
}

// A file containing several differing versions yields one result with all of
// them; Generate turns those into one entry per distinct version.
func TestDiscoverDifferingVersions(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "notes.md", "app 1.2.3\nlib 2.0.0\napp 1.2.3\n")
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}
	d := distinct(got[0])
	want := []version.Version{{Major: 1, Minor: 2, Patch: 3}, {Major: 2, Minor: 0, Patch: 0}}
	if len(d) != len(want) {
		t.Fatalf("distinct = %v, want %v", d, want)
	}
	for i := range want {
		if d[i] != want[i] {
			t.Errorf("distinct[%d] = %v, want %v", i, d[i], want[i])
		}
	}
}

func TestDiscoverIgnoresTwoComponentNumbers(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "f", "python 3.9 then 1.2.3\n")
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || firstVersion(got[0]) != (version.Version{Major: 1, Minor: 2, Patch: 3}) {
		t.Errorf("got %+v, want 1.2.3 (3.9 is not a full version)", got)
	}
	if len(got[0].Occurrences) != 1 {
		t.Errorf("got %d occurrences, want 1 (3.9 must not count)", len(got[0].Occurrences))
	}
}

func TestDiscoverVPrefix(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "VERSION", "v1.2.3\n")
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	want := version.Version{Major: 1, Minor: 2, Patch: 3, Prefix: "v"}
	if len(got) != 1 || firstVersion(got[0]) != want {
		t.Fatalf("got %+v, want one %q result", got, want)
	}
	if firstVersion(got[0]).String() != "v1.2.3" {
		t.Errorf("Version.String() = %q, want %q", firstVersion(got[0]).String(), "v1.2.3")
	}
}

func TestDiscoverUppercaseVPrefix(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "tag.txt", "current tag is V10.0.42\n")
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	want := version.Version{Major: 10, Minor: 0, Patch: 42, Prefix: "V"}
	if len(got) != 1 || firstVersion(got[0]) != want {
		t.Errorf("got %+v, want one %q result", got, want)
	}
}

// Tokens where "v"/"V" is part of a longer word (rev, dev, Rev) must not be read
// as a prefixed version, and their trailing digits must not be matched either.
func TestDiscoverRejectsNearMissPrefixes(t *testing.T) {
	for _, body := range []string{"rev1.2.3\n", "dev1.2.3\n", "Rev1.2.3 build\n", "abcv1.2.3\n"} {
		root := t.TempDir()
		mustWrite(t, root, "f", body)
		got, err := Discover(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("body %q: got %+v, want 0 results (near-miss prefix)", body, got)
		}
	}
}

// A bare version preceded by other words is still detected; only an immediately
// attached non-boundary "v" is rejected.
func TestDiscoverBareAlongsideNearMiss(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "f", "rev1.0.0 is bogus, real version 2.3.4\n")
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	want := version.Version{Major: 2, Minor: 3, Patch: 4}
	if len(got) != 1 || firstVersion(got[0]) != want {
		t.Errorf("got %+v, want one %q result", got, want)
	}
}

// The discovered prefix survives into the generated config's version string.
func TestGeneratePreservesPrefix(t *testing.T) {
	results := []Result{
		{Path: "VERSION", Occurrences: []Occurrence{{Version: version.Version{Major: 1, Minor: 2, Patch: 3, Prefix: "v"}}}},
		{Path: "bare", Occurrences: []Occurrence{{Version: version.Version{Major: 4, Minor: 5, Patch: 6}}}},
	}
	data, err := Generate(results)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, `version = "v1.2.3"`) {
		t.Errorf("generated config missing v-prefixed version:\n%s", out)
	}
	if !strings.Contains(out, `version = "4.5.6"`) {
		t.Errorf("generated config missing bare version:\n%s", out)
	}
}

// A file with several distinct versions produces one [[files]] entry per
// distinct version (sharing the path); identical repeats collapse to one entry.
func TestGenerateDistinctVersionsPerFile(t *testing.T) {
	results := []Result{
		{Path: "notes.md", Occurrences: []Occurrence{
			{Version: version.Version{Major: 1, Minor: 2, Patch: 3}},
			{Version: version.Version{Major: 2, Minor: 0, Patch: 0}},
			{Version: version.Version{Major: 1, Minor: 2, Patch: 3}},
		}},
	}
	data, err := Generate(results)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	out := string(data)
	if strings.Count(out, `path = "notes.md"`) != 2 {
		t.Errorf("want 2 entries for notes.md (one per distinct version):\n%s", out)
	}
	if !strings.Contains(out, `version = "1.2.3"`) || !strings.Contains(out, `version = "2.0.0"`) {
		t.Errorf("generated config missing a distinct version:\n%s", out)
	}
}

// IPv4 addresses have four octets and must never be read as a three-component
// version, even when every octet is a small, version-like integer.
func TestDiscoverIgnoresIPv4(t *testing.T) {
	ips := []string{
		"127.0.0.1",       // loopback
		"10.0.0.255",      // private class A
		"192.168.1.1",     // private class C
		"172.16.254.1",    // private class B
		"255.255.255.255", // broadcast
		"0.0.0.0",         // unspecified
		"1.2.3.4",         // version-like octets, still four components
	}
	for _, ip := range ips {
		root := t.TempDir()
		mustWrite(t, root, "conf", "listen = "+ip+"\n")
		got, err := Discover(root)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("%q: got %+v, want 0 results (IPv4 is not a version)", ip, got)
		}
	}
}

// A real version on the same line as an IPv4 address is still found, and the IP
// must not have a three-component slice pulled out of it.
func TestDiscoverVersionAlongsideIPv4(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "conf", "server 192.168.1.1 running v2.3.4\n")
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	want := version.Version{Major: 2, Minor: 3, Patch: 4, Prefix: "v"}
	if len(got) != 1 || len(got[0].Occurrences) != 1 || firstVersion(got[0]) != want {
		t.Errorf("got %+v, want one %q result", got, want)
	}
}

// An IPv4 address appearing before a bare version must not shadow it, and no
// inner slice (e.g. 168.1.1) should be discovered from the address.
func TestDiscoverIPv4ThenVersion(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "conf", "host 192.168.1.1\nrelease 3.4.5\n")
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	want := version.Version{Major: 3, Minor: 4, Patch: 5}
	if len(got) != 1 || len(got[0].Occurrences) != 1 || firstVersion(got[0]) != want {
		t.Errorf("got %+v, want one %q result (no slice pulled from the IP)", got, want)
	}
}

func TestDiscoverSkipsBinaryFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bin"), []byte("\x00\x01\x02 1.2.3"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %+v, want 0 results (binary file should be skipped)", got)
	}
}

// A symlink to a file outside the scan root must not be discovered. Following
// one would read a file the scan was never pointed at, expose a matched line in
// the dry-run output, and let a later bump copy that content into the tree.
func TestDiscoverSkipsSymlinkToOutsideFile(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.conf")
	if err := os.WriteFile(secret, []byte("TOKEN=shh\nversion 9.9.9\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	mustWrite(t, root, "real.txt", "ver 1.2.3\n")
	if err := os.Symlink(secret, filepath.Join(root, "link.conf")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "real.txt" {
		t.Fatalf("got %+v, want only real.txt (the symlink must be skipped)", got)
	}
}

// A symlinked directory must not be traversed, so files under it are not
// reported under the link's name.
func TestDiscoverSkipsSymlinkedDirectory(t *testing.T) {
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "sub", "v.txt"), []byte("ver 7.7.7\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	mustWrite(t, root, "real.txt", "ver 1.2.3\n")
	if err := os.Symlink(filepath.Join(outside, "sub"), filepath.Join(root, "linkdir")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "real.txt" {
		t.Fatalf("got %+v, want only real.txt (the linked directory must be skipped)", got)
	}
}

// A FIFO in the tree must be skipped rather than read: reading one blocks until
// a writer appears, which would hang the scan indefinitely. The test fails via
// its own deadline if the skip regresses.
func TestDiscoverSkipsNonRegularFiles(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "real.txt", "ver 1.2.3\n")
	fifo := filepath.Join(root, "pipe")
	if err := testutil.Mkfifo(fifo); err != nil {
		t.Skipf("FIFOs unsupported: %v", err)
	}

	done := make(chan []Result, 1)
	go func() {
		got, err := Discover(root)
		if err != nil {
			t.Errorf("Discover: %v", err)
		}
		done <- got
	}()

	select {
	case got := <-done:
		if len(got) != 1 || got[0].Path != "real.txt" {
			t.Fatalf("got %+v, want only real.txt (the FIFO must be skipped)", got)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Discover blocked on the FIFO instead of skipping it")
	}
}

// A file larger than the scan cap is skipped so one pathological file cannot
// pull an unbounded amount of data into memory.
func TestDiscoverSkipsOversizedFiles(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "small.txt", "ver 1.2.3\n")

	big := filepath.Join(root, "big.txt")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	// Sparse: one byte past the cap makes the file oversized without writing
	// the whole payload.
	if err := f.Truncate(DefaultMaxScanBytes + 1); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte("ver 4.5.6\n"), 0); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "small.txt" {
		t.Fatalf("got %+v, want only small.txt (the oversized file must be skipped)", got)
	}
}

// A file right at the cap is still scanned, so the limit is inclusive and
// ordinary files are never dropped.
func TestDiscoverScansFileAtSizeCap(t *testing.T) {
	root := t.TempDir()
	padding := strings.Repeat("x", DefaultMaxScanBytes-len("\nver 1.2.3\n"))
	mustWrite(t, root, "atcap.txt", padding+"\nver 1.2.3\n")

	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "atcap.txt" {
		t.Fatalf("got %+v, want atcap.txt to be scanned", got)
	}
	if firstVersion(got[0]) != (version.Version{Major: 1, Minor: 2, Patch: 3}) {
		t.Errorf("got %v, want 1.2.3", firstVersion(got[0]))
	}
}

// A caller-supplied cap replaces the default one, so a scan can be tightened
// to skip files the default would have read.
func TestDiscoverWithLimitSkipsAboveCap(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "small.txt", "ver 1.2.3\n")
	mustWrite(t, root, "large.txt", strings.Repeat("x", 4096)+"\nver 4.5.6\n")

	got, err := DiscoverWithLimit(root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "small.txt" {
		t.Fatalf("got %+v, want only small.txt (large.txt is over the 1KiB cap)", got)
	}
}

// A cap of zero removes the limit, so even a file past the default cap is
// scanned.
func TestDiscoverWithLimitZeroScansEverything(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "huge.txt", strings.Repeat("x", DefaultMaxScanBytes+1)+"\nver 1.2.3\n")

	if got, err := Discover(root); err != nil || len(got) != 0 {
		t.Fatalf("Discover = %+v (err %v), want the oversized file skipped by default", got, err)
	}

	got, err := DiscoverWithLimit(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "huge.txt" {
		t.Fatalf("got %+v, want huge.txt to be scanned with the cap disabled", got)
	}
	if firstVersion(got[0]) != (version.Version{Major: 1, Minor: 2, Patch: 3}) {
		t.Errorf("got %v, want 1.2.3", firstVersion(got[0]))
	}
}

func TestDiscoverEmptyTree(t *testing.T) {
	got, err := Discover(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %d results, want 0", len(got))
	}
}

func TestDiscoverFilesWithoutVersionSkipped(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root, "a.txt", "no version here\n")
	mustWrite(t, root, "b.json", "{ totally unrelated }")
	mustWrite(t, root, "c", "year 2024, build 42\n")
	got, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover should not fail: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %+v, want 0 results", got)
	}
}

func TestGenerateRoundTrips(t *testing.T) {
	root := fixtureTree(t)
	results, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}

	data, err := Generate(results)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Write the generated config into the scanned tree and load it back; since
	// every discovered path exists, validation must pass.
	cfgPath := filepath.Join(root, "incrmit.toml")
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("loading generated config: %v", err)
	}

	// The config lists one entry per distinct version across all files.
	wantEntries := 0
	for _, r := range results {
		wantEntries += len(distinct(r))
	}
	if len(cfg.Files) != wantEntries {
		t.Fatalf("config has %d files, want %d", len(cfg.Files), wantEntries)
	}
}

func mustWrite(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
