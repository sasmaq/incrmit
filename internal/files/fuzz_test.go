package files

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sasmaq/incrmit/internal/version"
)

// FuzzSetKnownVersions asserts the invariant the whole tool rests on: a bump
// rewrites the version token and nothing else. incrmit edits other people's
// files in place, so a rewriter that eats a byte on either side of a token is
// the worst failure it has; the golden fixtures check that promise against four
// files someone sat down and wrote, and this checks it against input nobody
// imagined.
//
// The checks are deliberately independent of how SetKnownVersions works. In
// particular they do not go through assertOnlyVersionChanged, whose
// strings.ReplaceAll round trip can hide an error when the same token appears
// more than once: reverting every occurrence of the new token repairs a stray
// extra replacement as readily as the intended one. Here the output is
// decomposed into the ranges that changed, and every byte outside them must be
// the input's.
func FuzzSetKnownVersions(f *testing.F) {
	// Seeds pair realistic file bodies with the pins that make them interesting:
	// a plain bump, a repeated token, two versions bumping past each other in one
	// pass, and a prerelease pinned inside a release filename.
	f.Add([]byte("version = \"1.2.3\"\n"), "1.2.3", "1.2.4", "", "")
	f.Add([]byte("1.2.3 ... 1.2.3"), "1.2.3", "2.0.0", "", "")
	f.Add([]byte("app 1.2.3 lib 1.2.4"), "1.2.3", "1.2.4", "1.2.4", "1.2.5")
	f.Add([]byte("incrmit-1.2.3-rc.1.zip"), "1.2.3-rc.1", "1.2.3-rc.2", "", "")
	f.Add([]byte("incrmit-1.2.3-linux-amd64.tar.gz"), "1.2.3", "1.2.4", "", "")
	f.Add([]byte("tag: v1.2.3 (not 1.2.3)"), "v1.2.3", "v1.2.4", "", "")
	f.Add([]byte("192.168.1.1 and 3.9 and 1.2.3"), "1.2.3", "9.9.9", "", "")
	f.Add([]byte("{\"version\":\"1.2.3\"}"), "1.2.3", "1.2.3", "", "")
	f.Add([]byte("1.2.3-rc.10"), "1.2.3-rc.1", "1.2.3-rc.2", "", "")
	f.Add([]byte(""), "1.2.3", "1.2.4", "", "")

	f.Fuzz(func(t *testing.T, data []byte, oldA, newA, oldB, newB string) {
		repls := buildReplacements(oldA, newA, oldB, newB)

		// Nothing to replace means nothing may change: the no-op case is the one
		// a rewriter is most likely to get wrong by rebuilding the buffer.
		if len(repls) == 0 {
			out, counts := SetKnownVersions(data, nil)
			if !bytes.Equal(out, data) {
				t.Fatalf("no replacements changed the data:\n in: %q\nout: %q", data, out)
			}
			if len(counts) != 0 {
				t.Fatalf("no replacements reported counts %v", counts)
			}
			return
		}

		out, counts := SetKnownVersions(data, repls)

		// Every pin must be reported on, present or not, so a caller can tell an
		// out-of-sync config from a successful rewrite.
		if len(counts) != len(repls) {
			t.Fatalf("counts %v does not have one entry per replacement %v", counts, repls)
		}
		for _, r := range repls {
			if _, ok := counts[r.Old.String()]; !ok {
				t.Fatalf("counts %v has no entry for pin %s", counts, r.Old)
			}
		}

		// A token can only be replaced where it occurs, so the tally can never
		// exceed the number of times its text appears in the input. Replacements
		// are disjoint, which is what makes the non-overlapping count the right
		// bound.
		for _, r := range repls {
			tok := r.Old.String()
			if n, occurrences := counts[tok], strings.Count(string(data), tok); n > occurrences {
				t.Errorf("replaced %s %d times but it occurs only %d times in %q", tok, n, occurrences, data)
			}
		}

		// Each replacement moves the output's length by exactly the difference
		// between the two tokens, so the lengths and the counts must agree. This
		// catches a replacement that was made but not counted (and the reverse)
		// without depending on where any of them happened.
		want := len(data)
		for _, r := range repls {
			want += counts[r.Old.String()] * (len(r.New.String()) - len(r.Old.String()))
		}
		if len(out) != want {
			t.Errorf("output is %d bytes, want %d from %d input bytes and counts %v", len(out), want, len(data), counts)
		}

		written := assertOnlyTokensChanged(t, data, out, repls)
		assertNoInventedTokens(t, data, out, repls, written)
	})
}

// buildReplacements turns up to two fuzzed token pairs into replacements,
// dropping the ones that are not versions and any pin repeating an earlier Old.
// A repeated Old is dropped because the counts map is keyed by it: two entries
// would share one tally, which says nothing about either.
func buildReplacements(pairs ...string) []Replacement {
	var repls []Replacement
	seen := map[string]bool{}
	for i := 0; i+1 < len(pairs); i += 2 {
		oldVer, err := version.Parse(pairs[i])
		if err != nil {
			continue
		}
		newVer, err := version.Parse(pairs[i+1])
		if err != nil {
			continue
		}
		if seen[oldVer.String()] {
			continue
		}
		seen[oldVer.String()] = true
		repls = append(repls, Replacement{Old: oldVer, New: newVer})
	}
	return repls
}

// assertOnlyTokensChanged proves that out is in with some set of disjoint token
// ranges rewritten — that is, that every byte outside a replaced range is the
// input's byte, in the input's order.
//
// It does so by searching for any alignment of the two buffers that uses only
// two moves: copy one identical byte, or consume a pin's Old token on the left
// while consuming its New token on the right. If no such alignment reaches the
// end of both buffers, some byte changed that was not part of a token, and the
// search is exhaustive, so the failure is real rather than a decomposition it
// merely failed to guess.
//
// It returns the ranges of out that an alignment attributes to a written new
// token, which is what tells assertNoInventedTokens where the seams are. When
// the check is skipped or fails, the result is nil.
func assertOnlyTokensChanged(t *testing.T, in, out []byte, repls []Replacement) [][2]int {
	t.Helper()
	// The search visits at most len(in) x len(out) states. Fuzz inputs are
	// small; a rare large one is left to the length and count checks above
	// rather than spending a second of the fuzzer's budget on it.
	const maxStates = 1 << 22
	if len(in)*len(out) > maxStates {
		return nil
	}

	type state struct{ i, j int }
	memo := make(map[state]bool)
	var walk func(i, j int) bool
	walk = func(i, j int) bool {
		if i == len(in) && j == len(out) {
			return true
		}
		s := state{i, j}
		if done, ok := memo[s]; ok {
			return done
		}
		// Recorded before recursing so a cycle cannot form; the moves below
		// always advance, so this only ever guards repeated work.
		memo[s] = false

		ok := false
		for _, r := range repls {
			oldTok, newTok := r.Old.String(), r.New.String()
			if bytes.HasPrefix(in[i:], []byte(oldTok)) && bytes.HasPrefix(out[j:], []byte(newTok)) {
				if walk(i+len(oldTok), j+len(newTok)) {
					ok = true
					break
				}
			}
		}
		if !ok && i < len(in) && j < len(out) && in[i] == out[j] {
			ok = walk(i+1, j+1)
		}
		memo[s] = ok
		return ok
	}

	if !walk(0, 0) {
		t.Errorf("output is not the input with version tokens replaced; some other byte changed\n--- input ---\n%q\n--- output ---\n%q\n--- pins ---\n%v", in, out, repls)
		return nil
	}

	// Replay the alignment that succeeded, recording where each new token landed
	// in the output. Every state on a winning path is either memoized true or is
	// the end state, which walk answers before it memoizes anything — so a move
	// is only taken when it leads to one of those, and the replay cannot wander
	// off the path and run past the end of a buffer.
	winning := func(i, j int) bool {
		if i == len(in) && j == len(out) {
			return true
		}
		return memo[state{i, j}]
	}
	var written [][2]int
	for i, j := 0, 0; i < len(in) || j < len(out); {
		moved := false
		for _, r := range repls {
			oldTok, newTok := r.Old.String(), r.New.String()
			if !bytes.HasPrefix(in[i:], []byte(oldTok)) || !bytes.HasPrefix(out[j:], []byte(newTok)) {
				continue
			}
			if winning(i+len(oldTok), j+len(newTok)) {
				written = append(written, [2]int{j, j + len(newTok)})
				i, j = i+len(oldTok), j+len(newTok)
				moved = true
				break
			}
		}
		if moved {
			continue
		}
		if i < len(in) && j < len(out) && in[i] == out[j] && winning(i+1, j+1) {
			i, j = i+1, j+1
			continue
		}
		// walk(0, 0) said a path exists, so this is unreachable; returning
		// rather than looping keeps a future change to walk from hanging the
		// test instead of failing it.
		t.Errorf("internal: alignment replay stuck at in[%d], out[%d] for\n--- input ---\n%q\n--- output ---\n%q", i, j, in, out)
		return nil
	}
	return written
}

// assertNoInventedTokens checks that a rewrite never leaves behind a version
// the file did not have and the pins did not ask for. Splicing a shorter or
// longer token into a line can weld it to the bytes on either side, and a token
// conjured that way would be found by the next scan as if the project had
// really declared it.
//
// One weld is allowed, because the rewriter cannot prevent it: a new token can
// run on into the bytes that followed the token it replaced. The scanner leaves
// such bytes behind when a token is followed by something its grammar cannot
// absorb but a shorter token could — "0.0.0+H+0" is read as the version
// "0.0.0+H" with "+0" left over, so replacing the pin with "0.0.0" produces
// "0.0.0+0". Neither string is a valid version in the first place (semver
// allows one build section, not two), and the file is left reporting a version
// the config does not pin, which the next command refuses with "expected
// version not found" rather than acting on. So this is a wart on input nobody
// writes, not a silent corruption, and the check is stated to match what the
// rewriter can actually promise: a token that begins exactly where a new token
// was written and runs past its end is the documented weld, anything else is a
// version conjured out of nothing.
func assertNoInventedTokens(t *testing.T, in, out []byte, repls []Replacement, written [][2]int) {
	t.Helper()
	allowed := map[string]bool{}
	for _, loc := range version.FindTokens(in) {
		allowed[string(in[loc[0]:loc[1]])] = true
	}
	for _, r := range repls {
		allowed[r.New.String()] = true
		// A pin whose suffix was consumed inside a filename leaves the numeric
		// core of the new version behind, cut back by the same filename guard
		// that found it.
		allowed[r.New.Release().String()] = true
	}
	for _, loc := range version.FindTokens(out) {
		tok := string(out[loc[0]:loc[1]])
		if _, err := version.Parse(tok); err != nil {
			continue
		}
		if allowed[tok] || weldsOntoWrittenToken(loc[0], loc[1], written) {
			continue
		}
		t.Errorf("output contains version %q, which is neither in the input nor a replacement\n--- input ---\n%q\n--- output ---\n%q\n--- pins ---\n%v", tok, in, out, repls)
	}
}

// weldsOntoWrittenToken reports whether the output range [start, end) is a new
// token that ran on into the bytes following the token it replaced: it begins
// where the new token was written and ends past it.
func weldsOntoWrittenToken(start, end int, written [][2]int) bool {
	for _, w := range written {
		if start == w[0] && end > w[1] {
			return true
		}
	}
	return false
}
