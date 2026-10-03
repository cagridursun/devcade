#!/bin/sh
# DevCade installer for macOS and Linux (user-local, no sudo).
#
#   sh install.sh --version 1.0.0 [--base-url URL|DIR] [--install-dir DIR]
#                 [--os darwin|linux] [--arch amd64|arm64] [--force]
#
# Environment equivalents: DEVCADE_VERSION, DEVCADE_BASE_URL,
# DEVCADE_INSTALL_DIR. Command-line options win.
#
# Steps: detect the OS and CPU, download devcade_<version>_<os>_<arch>.tar.gz
# and SHA256SUMS from the base URL, verify the archive's SHA-256 against
# SHA256SUMS, and only then extract it and install the devcade binary into
# the install directory (default: ~/.local/bin). Nothing downloaded is
# executed. An existing file that is not a DevCade binary is never replaced
# unless --force is given.
#
# The base URL defaults to the GitHub release for tag v<version>:
#   https://github.com/cagridursun/devcade/releases/download/v<version>
# It may also be a file:// URL or a local directory containing the release
# files. Plain http:// is refused.

set -eu

prog=devcade-install
marker=github.com/cagridursun/devcade

say() { printf '%s: %s\n' "$prog" "$*"; }
die() {
	printf '%s: error: %s\n' "$prog" "$*" >&2
	exit 1
}

usage() {
	sed -n '2,8p' "$0" 2>/dev/null | sed 's/^# \{0,1\}//'
}

version=${DEVCADE_VERSION:-}
base_url=${DEVCADE_BASE_URL:-}
install_dir=${DEVCADE_INSTALL_DIR:-}
os=
arch=
force=0

need_arg() {
	[ "$2" -ge 2 ] || die "option $1 needs a value"
}

while [ $# -gt 0 ]; do
	case $1 in
	--version) need_arg "$1" $#; version=$2; shift 2 ;;
	--version=*) version=${1#*=}; shift ;;
	--base-url) need_arg "$1" $#; base_url=$2; shift 2 ;;
	--base-url=*) base_url=${1#*=}; shift ;;
	--install-dir) need_arg "$1" $#; install_dir=$2; shift 2 ;;
	--install-dir=*) install_dir=${1#*=}; shift ;;
	--os) need_arg "$1" $#; os=$2; shift 2 ;;
	--os=*) os=${1#*=}; shift ;;
	--arch) need_arg "$1" $#; arch=$2; shift 2 ;;
	--arch=*) arch=${1#*=}; shift ;;
	--force) force=1; shift ;;
	-h | --help) usage; exit 0 ;;
	*) die "unknown option: $1 (see --help)" ;;
	esac
done

# --- version --------------------------------------------------------------

[ -n "$version" ] || die "no version given; use --version X.Y.Z (or DEVCADE_VERSION)"
version=${version#v}
if ! printf '%s\n' "$version" | grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'; then
	die "invalid version '$version' (expected X.Y.Z or X.Y.Z-prerelease)"
fi

# --- platform -------------------------------------------------------------

if [ -z "$os" ]; then
	uname_s=$(uname -s 2>/dev/null || echo unknown)
	case $uname_s in
	Darwin) os=darwin ;;
	Linux) os=linux ;;
	MINGW* | MSYS* | CYGWIN* | Windows*) die "Windows is not supported by install.sh; use install.ps1 or the .zip archive" ;;
	*) die "unsupported operating system: $uname_s (supported: macOS, Linux)" ;;
	esac
fi
case $os in
darwin | linux) ;;
*) die "unsupported operating system: $os (supported: darwin, linux)" ;;
esac

if [ -z "$arch" ]; then
	uname_m=$(uname -m 2>/dev/null || echo unknown)
	case $uname_m in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64 | armv8* | aarch64_be) arch=arm64 ;;
	*) die "unsupported architecture: $uname_m (supported: amd64/x86_64, arm64/aarch64)" ;;
	esac
	# An x86_64 shell under Rosetta 2 on Apple silicon: prefer the native build.
	if [ "$os" = darwin ] && [ "$arch" = amd64 ] &&
		[ "$(sysctl -n sysctl.proc_translated 2>/dev/null || true)" = 1 ]; then
		arch=arm64
	fi
fi
case $arch in
amd64 | arm64) ;;
*) die "unsupported architecture: $arch (supported: amd64, arm64)" ;;
esac

# --- locations ------------------------------------------------------------

[ -n "$base_url" ] || base_url="https://github.com/cagridursun/devcade/releases/download/v$version"
while :; do
	case $base_url in
	*/) base_url=${base_url%/} ;;
	*) break ;;
	esac
done
case $base_url in
https://* | file://*) mode=url ;;
*://*) die "refusing base URL '$base_url': only https:// and file:// URLs or a local directory are allowed" ;;
*)
	mode=dir
	[ -d "$base_url" ] || die "base directory not found: $base_url"
	;;
esac

if [ -z "$install_dir" ]; then
	[ -n "${HOME:-}" ] || die "HOME is not set; pass --install-dir"
	install_dir=$HOME/.local/bin
fi

archive=devcade_${version}_${os}_${arch}.tar.gz
dest=$install_dir/devcade

# --- refuse to clobber unrelated files (checked before downloading) -------

check_dest() {
	if [ -d "$dest" ] && [ ! -L "$dest" ]; then
		die "$dest is a directory; refusing to replace it"
	fi
	if [ -e "$dest" ] || [ -L "$dest" ]; then
		[ "$force" = 1 ] && return 0
		if [ -L "$dest" ]; then
			die "$dest is a symbolic link; refusing to replace it (remove it or use --force)"
		fi
		if ! LC_ALL=C grep -q "$marker" "$dest" 2>/dev/null; then
			die "$dest exists and is not a DevCade binary; refusing to overwrite it (remove it or use --force)"
		fi
	fi
	return 0
}
check_dest

# --- tools ----------------------------------------------------------------

have() { command -v "$1" >/dev/null 2>&1; }

if have sha256sum; then
	sha256_of() { sha256sum "$1" | awk '{print $1}'; }
elif have shasum; then
	sha256_of() { shasum -a 256 "$1" | awk '{print $1}'; }
elif have openssl; then
	sha256_of() { openssl dgst -sha256 -r "$1" | awk '{print $1}'; }
else
	die "no SHA-256 tool found (need sha256sum, shasum or openssl)"
fi
have tar || die "tar is required"
have gzip || die "gzip is required"

tmp=$(mktemp -d 2>/dev/null || mktemp -d -t devcade-install) || die "cannot create a temporary directory"
cleanup() { rm -rf "$tmp"; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

fetch() { # fetch <file name>  -> $tmp/<file name>
	out=$tmp/$1
	if [ "$mode" = dir ]; then
		[ -f "$base_url/$1" ] || die "download failed: $base_url/$1 does not exist"
		cp "$base_url/$1" "$out" || die "download failed: cannot copy $base_url/$1"
		return 0
	fi
	url=$base_url/$1
	if have curl; then
		curl --fail --silent --show-error --location \
			--proto '=https,file' --proto-redir '=https' \
			--retry 2 --connect-timeout 20 -o "$out" "$url" ||
			die "download failed: $url (interrupted, missing, or not public; private repositories cannot be downloaded anonymously)"
	elif have wget; then
		case $url in
		https://*) ;;
		*) die "wget cannot read $url; install curl or use a local directory" ;;
		esac
		wget -q --https-only -O "$out" "$url" ||
			die "download failed: $url (interrupted, missing, or not public; private repositories cannot be downloaded anonymously)"
	else
		die "curl or wget is required to download $url"
	fi
}

# --- download and verify --------------------------------------------------

say "installing DevCade $version for $os/$arch"
fetch SHA256SUMS
fetch "$archive"

expected=$(awk -v f="$archive" '{ n = $2; sub(/^\*/, "", n); if (n == f) print $1 }' "$tmp/SHA256SUMS")
count=$(printf '%s' "$expected" | grep -c . || true)
[ "$count" = 1 ] || die "SHA256SUMS has $count entries for $archive (expected exactly 1)"
expected=$(printf '%s' "$expected" | tr 'ABCDEF' 'abcdef')
printf '%s\n' "$expected" | grep -Eq '^[0-9a-f]{64}$' || die "SHA256SUMS entry for $archive is not a SHA-256 hash"

actual=$(sha256_of "$tmp/$archive") || die "cannot compute the SHA-256 of $archive"
if [ "$actual" != "$expected" ]; then
	die "checksum mismatch for $archive (expected $expected, got $actual); the download is corrupt or was tampered with; nothing was installed"
fi
say "verified SHA-256 $actual"

# --- extract and install (only after verification) ------------------------

mkdir "$tmp/x"
# Relative paths keep GNU tar on Windows shells from reading "C:" as a host.
(cd "$tmp" && tar -xzf "./$archive" -C x devcade) || die "cannot extract devcade from $archive"
bin=$tmp/x/devcade
[ -f "$bin" ] && [ ! -L "$bin" ] || die "$archive does not contain a regular devcade file"

mkdir -p "$install_dir" || die "cannot create $install_dir"
check_dest # again: the destination may have appeared during the download
staged=$install_dir/.devcade.install.$$
if ! cp "$bin" "$staged" || ! chmod 0755 "$staged"; then
	rm -f "$staged"
	die "cannot write to $install_dir"
fi
if ! mv -f "$staged" "$dest"; then
	rm -f "$staged"
	die "cannot install to $dest"
fi

say "installed $dest"
case ":${PATH:-}:" in
*":$install_dir:"*) say "run: devcade" ;;
*) say "$install_dir is not on your PATH; add it, for example: export PATH=\"$install_dir:\$PATH\"" ;;
esac
