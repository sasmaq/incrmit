// Package version parses semantic versions of the form
// MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD] and applies major, minor, patch,
// release, and prerelease bumps.
package version

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a semantic version made up of major, minor, and patch components,
// plus the optional prerelease and build-metadata sections of semver 2.0.0.
//
// Prefix records an optional single-character leading "v" or "V" (as written in
// the source token, e.g. "v1.2.3"); it is empty for a bare MAJOR.MINOR.PATCH.
// Carrying the prefix on the value lets it survive a Parse -> Bump -> String
// round trip so a "v1.2.3" token bumps to "v1.2.4" while a bare "1.2.3" stays
// bare.
//
// Prerelease and Build hold the sections after "-" and "+" respectively, stored
// without their leading punctuation ("rc.1", "build.7"). They are carried on the
// value so a token round-trips through Parse -> String unchanged; every bump
// drops them (see BumpPatch). Prefix, Prerelease, and Build are last so existing
// keyed and positional literals of the numeric components are unaffected.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prefix     string
	Prerelease string
	Build      string
}

// Parse reads a [v]MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD] string into a Version.
// Surrounding whitespace is ignored, as is an optional single leading "v" or "V"
// (recorded in Prefix). Each numeric component must be a non-negative base-10
// integer with no sign, and exactly three must be present.
//
// The prerelease and build sections follow the semver 2.0.0 grammar: a
// dot-separated list of non-empty identifiers made of ASCII alphanumerics and
// hyphens. A numeric prerelease identifier must not carry a leading zero
// ("1.2.3-rc.01" is invalid); build identifiers have no such restriction.
func Parse(s string) (Version, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return Version{}, fmt.Errorf("version: empty string")
	}

	prefix := ""
	rest := trimmed
	if rest[0] == 'v' || rest[0] == 'V' {
		prefix = rest[:1]
		rest = rest[1:]
		if rest == "" {
			return Version{}, fmt.Errorf("version: %q has a %q prefix but no MAJOR.MINOR.PATCH", trimmed, prefix)
		}
	}

	// Split the build metadata off first: "+" is not a legal character anywhere
	// else in a version, so the first one always opens the build section, while
	// "-" may appear inside a build identifier ("1.2.3+exp-1"). Splitting in the
	// other order would tear such a token apart.
	build := ""
	if i := strings.IndexByte(rest, '+'); i >= 0 {
		build = rest[i+1:]
		rest = rest[:i]
		if err := checkIdentifiers(build, "build metadata", trimmed, false); err != nil {
			return Version{}, err
		}
	}

	prerelease := ""
	if i := strings.IndexByte(rest, '-'); i >= 0 {
		prerelease = rest[i+1:]
		rest = rest[:i]
		if err := checkIdentifiers(prerelease, "prerelease", trimmed, true); err != nil {
			return Version{}, err
		}
	}

	parts := strings.Split(rest, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("version: %q must have 3 components (MAJOR.MINOR.PATCH)", trimmed)
	}

	// Both "+" and "-" have been consumed by the splits above, so a signed
	// component can no longer reach here: "+1.2.3" and "1.-2.3" fail as an empty
	// or short numeric core instead.
	names := [3]string{"major", "minor", "patch"}
	var nums [3]int
	for i, p := range parts {
		if p == "" {
			return Version{}, fmt.Errorf("version: %s component is empty in %q", names[i], trimmed)
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return Version{}, fmt.Errorf("version: %s component %q is not a valid integer", names[i], p)
		}
		nums[i] = n
	}

	return Version{
		Major:      nums[0],
		Minor:      nums[1],
		Patch:      nums[2],
		Prefix:     prefix,
		Prerelease: prerelease,
		Build:      build,
	}, nil
}

// checkIdentifiers validates one dot-separated identifier list (a prerelease or
// build section, given without its leading "-" or "+"). kind names the section
// and token the full version for the error message. When numericRules is set,
// an all-digit identifier must not carry a leading zero, as semver requires of
// prerelease identifiers but not of build metadata.
func checkIdentifiers(s, kind, token string, numericRules bool) error {
	if s == "" {
		return fmt.Errorf("version: %q has an empty %s section", token, kind)
	}
	for _, id := range strings.Split(s, ".") {
		if id == "" {
			return fmt.Errorf("version: %q has an empty %s identifier", token, kind)
		}
		for _, r := range id {
			if !isIdentChar(r) {
				return fmt.Errorf("version: %s identifier %q in %q may only contain letters, digits, and hyphens", kind, id, token)
			}
		}
		if numericRules && isNumericID(id) && len(id) > 1 && id[0] == '0' {
			return fmt.Errorf("version: numeric %s identifier %q in %q must not have a leading zero", kind, id, token)
		}
	}
	return nil
}

// isIdentChar reports whether r may appear in a prerelease or build identifier.
func isIdentChar(r rune) bool {
	return r == '-' ||
		(r >= '0' && r <= '9') ||
		(r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z')
}

// isNumericID reports whether id consists only of digits, which is what makes an
// identifier numeric for both the leading-zero rule and precedence ordering.
func isNumericID(id string) bool {
	if id == "" {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] < '0' || id[i] > '9' {
			return false
		}
	}
	return true
}

// ValidPrereleaseID checks a prerelease identifier supplied on the command line
// (the `--pre` value, e.g. "rc" or "alpha.1"). It accepts exactly what may
// follow the "-" in a version token.
func ValidPrereleaseID(id string) error {
	return checkIdentifiers(id, "prerelease", "-"+id, true)
}

// BumpMajor increments the major component and resets minor and patch to 0.
// See BumpPatch for what happens to the prerelease and build sections.
func (v Version) BumpMajor() Version {
	return Version{Major: v.Major + 1, Minor: 0, Patch: 0, Prefix: v.Prefix}
}

// BumpMinor increments the minor component and resets patch to 0. See BumpPatch
// for what happens to the prerelease and build sections.
func (v Version) BumpMinor() Version {
	return Version{Major: v.Major, Minor: v.Minor + 1, Patch: 0, Prefix: v.Prefix}
}

// BumpPatch increments the patch component.
//
// Like BumpMajor and BumpMinor it preserves any prefix and drops both the
// prerelease and the build metadata: 1.2.3-rc.1 bumps to 1.2.4, not to
// 1.2.4-rc.1. Carrying a prerelease forward would claim the new version is
// still a preview of a release it no longer names, and build metadata describes
// one specific build, so it is never inherited. Use Release to promote a
// prerelease in place (1.2.3-rc.1 -> 1.2.3) and BumpPrerelease to iterate on one.
func (v Version) BumpPatch() Version {
	return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1, Prefix: v.Prefix}
}

// Release promotes a prerelease to the release it precedes, dropping both the
// prerelease and the build metadata without touching the numeric components:
// 1.2.3-rc.1 becomes 1.2.3.
func (v Version) Release() Version {
	return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch, Prefix: v.Prefix}
}

// IsPrerelease reports whether v carries a prerelease section.
func (v Version) IsPrerelease() bool {
	return v.Prerelease != ""
}

// PrereleaseID returns v's prerelease identifiers with a trailing numeric one
// removed, which is the part that names the prerelease series rather than
// counting within it: "rc.1" and "rc" both yield "rc". It is empty when v has no
// prerelease, and when the prerelease is a bare number ("1.2.3-1").
func (v Version) PrereleaseID() string {
	if v.Prerelease == "" {
		return ""
	}
	ids := strings.Split(v.Prerelease, ".")
	if isNumericID(ids[len(ids)-1]) {
		ids = ids[:len(ids)-1]
	}
	return strings.Join(ids, ".")
}

// StartPrerelease returns v marked as the first prerelease of the given series
// ("rc" yields 1.2.3-rc.1), leaving the numeric components alone and dropping
// any existing prerelease or build metadata.
func (v Version) StartPrerelease(id string) Version {
	return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch, Prefix: v.Prefix, Prerelease: id + ".1"}
}

// AdvancePrerelease returns the next prerelease in v's current series:
// 1.2.3-rc.1 -> 1.2.3-rc.2. A prerelease with no trailing number gains one
// (1.2.3-rc -> 1.2.3-rc.1). Build metadata is dropped. A version with no
// prerelease at all is returned unchanged; callers decide what to start.
func (v Version) AdvancePrerelease() Version {
	if v.Prerelease == "" {
		return v
	}
	next := v
	next.Build = ""
	ids := strings.Split(v.Prerelease, ".")
	last := ids[len(ids)-1]
	if n, err := strconv.Atoi(last); err == nil && isNumericID(last) {
		ids[len(ids)-1] = strconv.Itoa(n + 1)
	} else {
		ids = append(ids, "1")
	}
	next.Prerelease = strings.Join(ids, ".")
	return next
}

// BumpPrerelease starts or advances a prerelease of the same numeric version:
// it advances the counter when v is already in the id series
// (1.2.4-rc.1 -> 1.2.4-rc.2) and otherwise starts that series at 1
// (1.2.4 -> 1.2.4-rc.1, 1.2.4-beta.2 -> 1.2.4-rc.1). It never changes the
// numeric components; callers that want a new release line bump first.
func (v Version) BumpPrerelease(id string) Version {
	if v.IsPrerelease() && v.PrereleaseID() == id {
		return v.AdvancePrerelease()
	}
	return v.StartPrerelease(id)
}

// String formats the version back to [v]MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD],
// re-emitting any leading "v"/"V" prefix and both optional sections exactly as
// they were parsed, so a token round-trips unchanged.
func (v Version) String() string {
	var b strings.Builder
	b.WriteString(v.Prefix)
	b.WriteString(strconv.Itoa(v.Major))
	b.WriteByte('.')
	b.WriteString(strconv.Itoa(v.Minor))
	b.WriteByte('.')
	b.WriteString(strconv.Itoa(v.Patch))
	if v.Prerelease != "" {
		b.WriteByte('-')
		b.WriteString(v.Prerelease)
	}
	if v.Build != "" {
		b.WriteByte('+')
		b.WriteString(v.Build)
	}
	return b.String()
}

// Compare orders two versions by semver 2.0.0 precedence, returning -1 if a
// precedes b, 0 if they have equal precedence, and +1 if a follows b.
//
// Numeric components are compared first, then the prerelease: a version with a
// prerelease precedes the release it names (1.2.3-rc.1 < 1.2.3), and two
// prereleases are compared identifier by identifier, numerically when both are
// numeric, otherwise as ASCII text, with a numeric identifier ranking below an
// alphanumeric one and a shorter run of otherwise-equal identifiers ranking
// below a longer one. Neither the "v" prefix nor build metadata affects
// precedence, so v1.2.3, 1.2.3, and 1.2.3+build.7 all compare equal.
func Compare(a, b Version) int {
	if c := cmpInt(a.Major, b.Major); c != 0 {
		return c
	}
	if c := cmpInt(a.Minor, b.Minor); c != 0 {
		return c
	}
	if c := cmpInt(a.Patch, b.Patch); c != 0 {
		return c
	}
	return comparePrerelease(a.Prerelease, b.Prerelease)
}

// comparePrerelease applies the semver precedence rules for the prerelease
// section. An empty section (a release) ranks above any prerelease.
func comparePrerelease(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}

	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if c := compareIdentifier(as[i], bs[i]); c != 0 {
			return c
		}
	}
	return cmpInt(len(as), len(bs))
}

// compareIdentifier compares one prerelease identifier against another.
func compareIdentifier(a, b string) int {
	aNum, bNum := isNumericID(a), isNumericID(b)
	switch {
	case aNum && bNum:
		// Parsed rather than compared as text so 2 < 10. Leading zeros are
		// rejected at parse time, so equal-length digit runs are unambiguous;
		// an identifier too large for an int falls back to a length-then-text
		// comparison, which is the same ordering for zero-padding-free digits.
		an, aErr := strconv.Atoi(a)
		bn, bErr := strconv.Atoi(b)
		if aErr == nil && bErr == nil {
			return cmpInt(an, bn)
		}
		if c := cmpInt(len(a), len(b)); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	case aNum:
		return -1
	case bNum:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

// cmpInt returns -1, 0, or +1 for a < b, a == b, and a > b.
func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
