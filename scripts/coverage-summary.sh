#!/usr/bin/env bash
# Render a Go coverage profile as a Markdown summary: the total against the
# threshold, then the statement coverage of each package.
# Usage: scripts/coverage-summary.sh THRESHOLD [coverage.out]
set -euo pipefail

threshold="${1:?threshold required (e.g. 90)}"
profile="${2:-coverage.out}"

if [ ! -f "$profile" ]; then
	echo "coverage profile not found: $profile" >&2
	exit 1
fi

module="$(awk '/^module / { print $2; exit }' go.mod)"
# The total comes from the same command `make cover` gates on, so the two
# never disagree about the number.
total="$(go tool cover -func="$profile" | awk '/^total:/ { print $3 }' | tr -d '%')"

echo "### Coverage: ${total}% of statements (threshold ${threshold}%)"
echo
if ! awk "BEGIN { exit !(${total} >= ${threshold}) }"; then
	echo "**Below the ${threshold}% threshold.**"
	echo
fi
echo "| Package | Coverage |"
echo "| :-- | --: |"

# Each profile line is "file:start,end statements count". A package's coverage
# is its covered statements over all of its statements; a block listed more
# than once (profiles merged across runs) counts once, as covered if any run
# covered it.
awk -v module="$module" '
	NR > 1 {
		block = $1
		pkg = block
		sub(/:.*/, "", pkg)
		sub(/\/[^\/]*$/, "", pkg)
		stmts[block] = $2
		pkgOf[block] = pkg
		if ($3 > 0) hit[block] = 1
	}
	END {
		for (b in stmts) {
			all[pkgOf[b]] += stmts[b]
			if (b in hit) covered[pkgOf[b]] += stmts[b]
		}
		for (p in all) {
			name = p
			if (name == module) name = "."
			else sub("^" module "/", "", name)
			printf "| `%s` | %.1f%% |\n", name, 100 * covered[p] / all[p]
		}
	}
' "$profile" | sort
