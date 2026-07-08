// This file implements matching of user-authored ignore patterns (the config's
// top-level `ignore` list) against paths encountered during the discovery walk.
//
// Matching semantics (all paths are compared in slash form, relative to the
// scan root):
//
//   - A trailing slash marks a pattern as directory-only: "testdata/" prunes a
//     directory named testdata but never matches a file of that name.
//   - A pattern with no slash matches the base name of any file or directory at
//     any depth via path.Match, so "*.lock" ignores every lock file and
//     "node_modules" prunes every such directory wherever it appears.
//   - A pattern containing a slash is matched against the whole relative path,
//     segment by segment. Each segment is a path.Match glob, and "**" matches
//     zero or more segments, so "docs/**" prunes the docs directory and
//     everything under it, while "a/b/*.txt" matches only text files directly in
//     a/b.
//
// Matching is case-sensitive (the same as path.Match). These patterns are
// applied in addition to the built-in ignoredDirs; a path is skipped if either
// the built-in set or any configured pattern matches.
package discovery

import (
	"path"
	"strings"
)

// ignoreMatcher is a compiled set of ignore patterns. The zero value (and a nil
// pointer) matches nothing, so callers can use it unconditionally.
type ignoreMatcher struct {
	patterns []ignorePattern
}

// ignorePattern is a single compiled ignore rule.
type ignorePattern struct {
	// segs holds the slash-separated segments of a pattern that contains a
	// slash; it is nil for a bare (no-slash) pattern.
	segs []string
	// base is the single segment of a bare pattern; it is empty when segs is set.
	base string
	// dirOnly is true when the pattern had a trailing slash and so may only
	// match directories.
	dirOnly bool
}

// newIgnoreMatcher compiles the given patterns (already normalized to slash form
// and trimmed by config loading) into a matcher. Empty patterns are skipped
// defensively even though config validation rejects them.
func newIgnoreMatcher(patterns []string) *ignoreMatcher {
	m := &ignoreMatcher{}
	for _, raw := range patterns {
		p := strings.TrimSpace(raw)
		dirOnly := false
		if strings.HasSuffix(p, "/") {
			dirOnly = true
			p = strings.TrimRight(p, "/")
		}
		if p == "" {
			continue
		}
		pat := ignorePattern{dirOnly: dirOnly}
		if strings.Contains(p, "/") {
			pat.segs = strings.Split(p, "/")
		} else {
			pat.base = p
		}
		m.patterns = append(m.patterns, pat)
	}
	return m
}

// empty reports whether the matcher has no patterns (so the walk can skip the
// per-entry check entirely).
func (m *ignoreMatcher) empty() bool {
	return m == nil || len(m.patterns) == 0
}

// match reports whether the slash-form path rel (relative to the scan root)
// should be ignored. isDir selects whether directory-only patterns apply.
func (m *ignoreMatcher) match(rel string, isDir bool) bool {
	if m == nil {
		return false
	}
	for _, p := range m.patterns {
		if p.dirOnly && !isDir {
			continue
		}
		if p.matches(rel) {
			return true
		}
	}
	return false
}

// matches reports whether a single pattern matches the relative path rel.
func (p ignorePattern) matches(rel string) bool {
	if p.segs == nil {
		base := rel
		if i := strings.LastIndex(rel, "/"); i >= 0 {
			base = rel[i+1:]
		}
		ok, err := path.Match(p.base, base)
		return err == nil && ok
	}
	return matchSegments(p.segs, strings.Split(rel, "/"))
}

// matchSegments matches a slash-split pattern against slash-split path segments.
// A "**" pattern segment matches zero or more path segments; every other
// segment is a path.Match glob matched against exactly one path segment.
func matchSegments(pat, name []string) bool {
	if len(pat) == 0 {
		return len(name) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(name); i++ {
			if matchSegments(pat[1:], name[i:]) {
				return true
			}
		}
		return false
	}
	if len(name) == 0 {
		return false
	}
	ok, err := path.Match(pat[0], name[0])
	if err != nil || !ok {
		return false
	}
	return matchSegments(pat[1:], name[1:])
}
