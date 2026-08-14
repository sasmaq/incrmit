package version

import "regexp"

// tokenRe matches a candidate version token: a run of two or more dot-separated
// integer groups, optionally prefixed by a single leading "v" or "V" and
// followed by the optional "-prerelease" and "+build" sections of semver,
// bounded by non-word characters.
//
// It deliberately matches more than a valid version so that candidates are
// rejected whole rather than sliced: an IPv4 address (192.168.1.1) is seen as
// one four-component token and rejected by Parse instead of having a
// three-component slice pulled out of it, and two-component numbers are likewise
// rejected. Matching the suffixes is what keeps a prerelease intact — stopping
// at the numeric core would rewrite the "1.2.3" inside "1.2.3-rc.1" and leave
// the "-rc.1" dangling on a version it no longer belongs to.
//
// The leading \b keeps the optional prefix from being taken out of the middle of
// a word (so "rev1.2.3" is not matched), and the trailing \b keeps a suffix from
// running into an adjacent word: "1.2.3-rc_1" matches only "1.2.3", because "_"
// is a word character and no boundary falls before it.
var tokenRe = regexp.MustCompile(`\b[vV]?\d+(?:\.\d+)+(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?\b`)

// coreRe matches the [v]MAJOR.MINOR.PATCH... head of a candidate token, which is
// what remains when a suffix is disowned by FindTokens.
var coreRe = regexp.MustCompile(`^[vV]?\d+(?:\.\d+)+`)

// FindTokens returns the byte ranges of every candidate version token in data,
// as [start, end) pairs in the order they appear — the same shape
// regexp.FindAllIndex returns. It is the single definition of what counts as a
// version token: package files rewrites exactly these ranges and package
// discovery records exactly these ranges, so scanning and rewriting can never
// disagree about where a version begins and ends.
//
// Callers validate each candidate with Parse, which is the authority on whether
// a token is a version at all.
func FindTokens(data []byte) [][]int {
	locs := tokenRe.FindAllIndex(data, -1)
	for _, loc := range locs {
		if !suffixBelongs(data, loc[0]) {
			loc[1] = loc[0] + len(coreRe.Find(data[loc[0]:loc[1]]))
		}
	}
	return locs
}

// suffixBelongs reports whether the token starting at start may keep its
// "-prerelease" section, or whether the token ends at its numeric core.
//
// A hyphen is a legal prerelease identifier character, so semver alone cannot
// tell "1.2.3-rc.1" from the version inside a release filename such as
// "incrmit-1.2.3-linux-amd64.tar.gz" — the latter parses perfectly well as
// 1.2.3 with the prerelease "linux-amd64.tar.gz". Taking that as the version
// would make a bump rewrite the whole filename to "incrmit-1.2.4", silently
// destroying the rest of the line.
//
// The distinguishing signal is not the suffix but what precedes the version: a
// real prerelease token stands on its own (after a quote, an "=", whitespace, or
// the start of a line), while the filename case has the version welded into a
// longer hyphen-joined word. So a token preceded by "-" that is itself preceded
// by a word character keeps only its numeric core, which is exactly what the
// matcher found before prerelease support existed. RE2 has no look-behind, hence
// this check on the surrounding bytes rather than a pattern.
//
// The cost is that a genuine prerelease inside a filename ("app-1.2.3-rc.1.zip")
// is matched as its core alone and bumped to "app-1.2.4-rc.1.zip". That is not
// semver-correct, but it preserves the line rather than eating it.
func suffixBelongs(data []byte, start int) bool {
	if start < 2 || data[start-1] != '-' {
		return true
	}
	return !isWordByte(data[start-2])
}

// isWordByte reports whether b is an ASCII word character, matching what the
// \b assertions in tokenRe treat as part of a word.
func isWordByte(b byte) bool {
	return b == '_' ||
		(b >= '0' && b <= '9') ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z')
}
