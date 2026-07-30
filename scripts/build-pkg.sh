#!/usr/bin/env bash
# Build a macOS installer package (.pkg) for one architecture with pkgbuild.
# Usage: scripts/build-pkg.sh <version> <arch> [dist]
#   version  release version without a leading "v" (e.g. 0.1.14)
#   arch     Go arch of the darwin binary (amd64 or arm64)
#   dist     output directory (default: dist)
#
# Expects the version-stamped binary dist/incrmit-<version>-darwin-<arch> to
# already exist (see `make darwin-binaries`). Produces:
#   <dist>/incrmit-<version>-darwin-<arch>.pkg
# installing /usr/local/bin/incrmit and /usr/local/share/man/man1/incrmit.1.
set -euo pipefail

version="${1:?version required (e.g. 0.1.14)}"
arch="${2:?arch required (amd64 or arm64)}"
dist="${3:-dist}"

identifier="com.github.sasmaq.incrmit"
binary="${dist}/incrmit-${version}-darwin-${arch}"
man="doc/man/incrmit.1"
out="${dist}/incrmit-${version}-darwin-${arch}.pkg"

command -v pkgbuild >/dev/null 2>&1 || {
	echo "pkgbuild not found; macOS .pkg packages must be built on macOS" >&2
	exit 1
}

if [ ! -f "$binary" ]; then
	echo "binary not found: $binary (run 'make darwin-binaries' or 'make dist' first)" >&2
	exit 1
fi
if [ ! -f "$man" ]; then
	echo "man page not found: $man" >&2
	exit 1
fi

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT

install -d -m 0755 "$root/usr/local/bin"
install -m 0755 "$binary" "$root/usr/local/bin/incrmit"
install -d -m 0755 "$root/usr/local/share/man/man1"
install -m 0644 "$man" "$root/usr/local/share/man/man1/incrmit.1"

# Drop removable extended attributes (for example com.apple.quarantine) from the
# staged tree. Kernel-managed ones such as com.apple.provenance cannot be
# removed and will still be encoded as AppleDouble (._*) entries in the cpio
# payload; `installer` reassembles those into file metadata rather than writing
# literal ._ files, so the installed tree stays clean. Verify with
# `pkgutil --expand-full <pkg>`, which should show no ._ files.
xattr -cr "$root" 2>/dev/null || true

COPYFILE_DISABLE=1 pkgbuild \
	--root "$root" \
	--identifier "$identifier" \
	--version "$version" \
	--install-location / \
	"$out"

echo "pkg written to $out"
