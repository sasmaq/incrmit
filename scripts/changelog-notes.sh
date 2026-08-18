#!/usr/bin/env bash
# Extract the CHANGELOG.md section for a release version (without the leading "v").
# Usage: scripts/changelog-notes.sh 0.2.1 [CHANGELOG.md]
set -euo pipefail

version="${1:?version required (e.g. 0.2.1)}"
changelog="${2:-CHANGELOG.md}"

if [ ! -f "$changelog" ]; then
	echo "changelog file not found: $changelog" >&2
	exit 1
fi

notes="$(awk -v ver="$version" '
	/^## \[/ {
		if (found) exit
		if ($0 ~ "^## \\[" ver "\\]") found = 1
	}
	found { print }
' "$changelog")"

if [ -z "$notes" ]; then
	echo "no CHANGELOG section found for version ${version}" >&2
	exit 1
fi

printf '%s\n' "$notes"
