// Package version parses semantic versions of the form MAJOR.MINOR.PATCH and
// applies major, minor, or patch bumps.
package version

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a semantic version made up of major, minor, and patch components.
//
// Prefix records an optional single-character leading "v" or "V" (as written in
// the source token, e.g. "v1.2.3"); it is empty for a bare MAJOR.MINOR.PATCH.
// Carrying the prefix on the value lets it survive a Parse -> Bump -> String
// round trip so a "v1.2.3" token bumps to "v1.2.4" while a bare "1.2.3" stays
// bare. Prefix is the last field so existing keyed and zero-value literals are
// unaffected.
type Version struct {
	Major  int
	Minor  int
	Patch  int
	Prefix string
}

// Parse reads a MAJOR.MINOR.PATCH string into a Version. Surrounding whitespace
// is ignored, as is an optional single leading "v" or "V" (recorded in Prefix).
// Each component must be a non-negative base-10 integer with no sign, and
// exactly three components must be present.
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

	parts := strings.Split(rest, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("version: %q must have 3 components (MAJOR.MINOR.PATCH)", trimmed)
	}

	names := [3]string{"major", "minor", "patch"}
	var nums [3]int
	for i, p := range parts {
		if p == "" {
			return Version{}, fmt.Errorf("version: %s component is empty in %q", names[i], trimmed)
		}
		// strconv.Atoi accepts a leading sign; reject it explicitly so values
		// like "1.-2.3" or "+1.2.3" are not silently accepted.
		if p[0] == '+' || p[0] == '-' {
			return Version{}, fmt.Errorf("version: %s component %q must not be signed", names[i], p)
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return Version{}, fmt.Errorf("version: %s component %q is not a valid integer", names[i], p)
		}
		nums[i] = n
	}

	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2], Prefix: prefix}, nil
}

// BumpMajor increments the major component and resets minor and patch to 0,
// preserving any prefix.
func (v Version) BumpMajor() Version {
	return Version{Major: v.Major + 1, Minor: 0, Patch: 0, Prefix: v.Prefix}
}

// BumpMinor increments the minor component and resets patch to 0, preserving
// any prefix.
func (v Version) BumpMinor() Version {
	return Version{Major: v.Major, Minor: v.Minor + 1, Patch: 0, Prefix: v.Prefix}
}

// BumpPatch increments the patch component, preserving any prefix.
func (v Version) BumpPatch() Version {
	return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1, Prefix: v.Prefix}
}

// String formats the version back to [v]MAJOR.MINOR.PATCH, re-emitting any
// leading "v"/"V" prefix exactly as it was parsed.
func (v Version) String() string {
	return v.Prefix + strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
}
