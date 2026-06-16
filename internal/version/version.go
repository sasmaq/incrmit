// Package version parses semantic versions of the form MAJOR.MINOR.PATCH and
// applies major, minor, or patch bumps.
package version

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a semantic version made up of major, minor, and patch components.
type Version struct {
	Major int
	Minor int
	Patch int
}

// Parse reads a MAJOR.MINOR.PATCH string into a Version. Surrounding whitespace
// is ignored. Each component must be a non-negative base-10 integer with no
// sign, and exactly three components must be present.
func Parse(s string) (Version, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return Version{}, fmt.Errorf("version: empty string")
	}

	parts := strings.Split(trimmed, ".")
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

	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2]}, nil
}

// String formats the version back to MAJOR.MINOR.PATCH.
func (v Version) String() string {
	return strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
}
