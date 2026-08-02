package cli

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
)

// sizeUnits maps the accepted size suffixes to their multiplier. Both the
// binary units (KiB = 1024) and their decimal counterparts (KB = 1000) are
// understood, along with the bare forms (K, M, G) which are treated as binary
// because that is what "32M" conventionally means for a file size.
var sizeUnits = []struct {
	suffix string
	mult   int64
}{
	{"KIB", 1 << 10},
	{"MIB", 1 << 20},
	{"GIB", 1 << 30},
	{"KB", 1000},
	{"MB", 1000 * 1000},
	{"GB", 1000 * 1000 * 1000},
	{"K", 1 << 10},
	{"M", 1 << 20},
	{"G", 1 << 30},
	{"B", 1},
}

// parseSize converts a human-written size such as "32MiB", "64MB", "512K", or a
// plain byte count such as "1048576" into a number of bytes. The suffix is
// case-insensitive and may be separated from the number by spaces. A value of 0
// means "no limit"; a negative value is rejected.
func parseSize(s string) (int64, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, errors.New("empty size")
	}

	digits := strings.ToUpper(trimmed)
	mult := int64(1)
	for _, u := range sizeUnits {
		if rest, ok := strings.CutSuffix(digits, u.suffix); ok {
			digits, mult = strings.TrimSpace(rest), u.mult
			break
		}
	}

	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a size (want a byte count or a value like 32MiB)", trimmed)
	}
	if n < 0 {
		return 0, fmt.Errorf("%q is negative (use 0 for no limit)", trimmed)
	}
	// Reject a value that would wrap int64 rather than silently turning a huge
	// cap into a small (or negative) one.
	if mult > 1 && n > (1<<63-1)/mult {
		return 0, fmt.Errorf("%q is too large", trimmed)
	}
	return n * mult, nil
}

// formatSize renders a byte count the way parseSize accepts it, using the
// largest binary unit that divides it exactly so a limit reads back as the user
// wrote it (33554432 -> "32MiB"). Sizes that are not whole units are reported
// as a plain byte count.
func formatSize(n int64) string {
	units := []struct {
		suffix string
		mult   int64
	}{
		{"GiB", 1 << 30},
		{"MiB", 1 << 20},
		{"KiB", 1 << 10},
	}
	for _, u := range units {
		if n >= u.mult && n%u.mult == 0 {
			return fmt.Sprintf("%d%s", n/u.mult, u.suffix)
		}
	}
	return fmt.Sprintf("%d bytes", n)
}

// sizeValue adapts parseSize to the flag package so a bad value is reported as
// a usage error at parse time rather than after the command starts working. The
// same value is registered under both the long and short flag names, so either
// spelling writes to the one target.
type sizeValue int64

func (v *sizeValue) String() string {
	if v == nil {
		return ""
	}
	return formatSize(int64(*v))
}

func (v *sizeValue) Set(s string) error {
	n, err := parseSize(s)
	if err != nil {
		return err
	}
	*v = sizeValue(n)
	return nil
}

// maxFileSizeVar registers the --max-file-size flag and its -s shorthand on fs,
// defaulting to def bytes. Both spellings write to target.
func maxFileSizeVar(fs *flag.FlagSet, target *int64, def int64, usage string) {
	*target = def
	v := (*sizeValue)(target)
	fs.Var(v, "max-file-size", usage)
	fs.Var(v, "s", usage+" (shorthand)")
}
