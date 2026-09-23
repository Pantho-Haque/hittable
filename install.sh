#!/bin/sh
#
# hittable.sh installer.
#
#   curl -fsSL https://raw.githubusercontent.com/Pantho-Haque/hittable/main/install.sh | sh
#
# Downloads the release binary for this machine, verifies its SHA-256 against
# the published checksums, and installs it. Nothing else is fetched and nothing
# is sent anywhere.
#
# Environment:
#   HITTABLE_VERSION      tag to install (default: the latest release)
#   HITTABLE_INSTALL_DIR  where to put the binary (default: ~/.local/bin)
#
# Run it with `sh -x` to see every step.

set -eu

REPO="Pantho-Haque/hittable"
BIN="hittable"

# Colour only when stderr is a terminal: piped into a log, escape codes are noise.
if [ -t 2 ]; then
	C_B='\033[1m'; C_G='\033[32m'; C_Y='\033[33m'; C_R='\033[31m'; C_0='\033[0m'
else
	C_B=''; C_G=''; C_Y=''; C_R=''; C_0=''
fi

say()  { printf '%b\n' "$*" >&2; }
step() { printf '%b\n' "  $*" >&2; }
warn() { printf '%b\n' "${C_Y}warning:${C_0} $*" >&2; }
die()  { printf '%b\n' "${C_R}error:${C_0} $*" >&2; exit 1; }

need() {
	command -v "$1" >/dev/null 2>&1 || die "this installer needs '$1', which is not on your PATH"
}

# ---------- what machine is this ----------

detect_os() {
	case "$(uname -s)" in
	Darwin) echo darwin ;;
	Linux)
		# WSL is Linux and runs the Linux binary; saying so avoids confusion
		# for someone who thinks of their machine as Windows.
		if grep -qi microsoft /proc/version 2>/dev/null; then
			IS_WSL=1
		fi
		echo linux
		;;
	MINGW* | MSYS* | CYGWIN*) echo windows ;;
	*) die "unsupported operating system: $(uname -s)" ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) echo amd64 ;;
	arm64 | aarch64) echo arm64 ;;
	armv7l | armv6l) die "32-bit ARM is not supported; hittable needs a 64-bit system" ;;
	i386 | i686) die "32-bit x86 is not supported; hittable needs a 64-bit system" ;;
	*) die "unsupported architecture: $(uname -m)" ;;
	esac
}

# ---------- downloading ----------

fetch() { # fetch <url> <dest>
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL --retry 3 --retry-delay 1 "$1" -o "$2"
	else
		wget -q -O "$2" "$1"
	fi
}

fetch_stdout() {
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL --retry 3 --retry-delay 1 "$1"
	else
		wget -q -O - "$1"
	fi
}

latest_version() {
	# The redirect on /releases/latest names the tag, which avoids depending on
	# the API's rate limit or on jq being installed.
	if command -v curl >/dev/null 2>&1; then
		curl -fsSLI -o /dev/null -w '%{url_effective}' \
			"https://github.com/$REPO/releases/latest" 2>/dev/null |
			sed 's#.*/tag/##'
	else
		fetch_stdout "https://api.github.com/repos/$REPO/releases/latest" |
			sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1
	fi
}

sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | cut -d' ' -f1
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | cut -d' ' -f1
	else
		echo ""
	fi
}

# ---------- go ----------

IS_WSL=0
OS="$(detect_os)"
ARCH="$(detect_arch)"
EXT="tar.gz"
EXE=""
if [ "$OS" = windows ]; then
	EXT="zip"
	EXE=".exe"
fi

need uname
command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1 ||
	die "this installer needs curl or wget"

VERSION="${HITTABLE_VERSION:-}"
if [ -z "$VERSION" ]; then
	VERSION="$(latest_version || true)"
fi
[ -n "$VERSION" ] || die "could not work out the latest version — set HITTABLE_VERSION=vX.Y.Z and try again"

INSTALL_DIR="${HITTABLE_INSTALL_DIR:-$HOME/.local/bin}"
ASSET="${BIN}_${VERSION}_${OS}_${ARCH}.${EXT}"
BASE="https://github.com/$REPO/releases/download/$VERSION"

say ""
say "${C_B}hittable.sh${C_0} $VERSION  ·  $OS/$ARCH"
[ "$IS_WSL" = 1 ] && step "detected WSL, installing the Linux build"
say ""

TMP="$(mktemp -d 2>/dev/null || mktemp -d -t hittable)"
cleanup() { rm -rf "$TMP"; }
trap cleanup EXIT INT TERM

step "downloading $ASSET"
fetch "$BASE/$ASSET" "$TMP/$ASSET" ||
	die "no build for $OS/$ARCH in release $VERSION
       see https://github.com/$REPO/releases/$VERSION"

# A checksum you fetch alongside the file only proves the two came from the
# same place — but it does catch a truncated or corrupted download, which is
# the failure people actually hit.
if fetch "$BASE/checksums.txt" "$TMP/checksums.txt" 2>/dev/null; then
	want="$(grep " $ASSET\$" "$TMP/checksums.txt" 2>/dev/null | cut -d' ' -f1 || true)"
	got="$(sha256_of "$TMP/$ASSET")"
	if [ -z "$got" ]; then
		warn "no sha256 tool found, skipping checksum verification"
	elif [ -z "$want" ]; then
		warn "$ASSET is not listed in checksums.txt, skipping verification"
	elif [ "$want" != "$got" ]; then
		die "checksum mismatch for $ASSET
       expected $want
       actual   $got
       Refusing to install. Try again, or report this."
	else
		step "checksum ok"
	fi
else
	warn "checksums.txt not published for $VERSION, skipping verification"
fi

step "unpacking"
if [ "$EXT" = "zip" ]; then
	need unzip
	unzip -q "$TMP/$ASSET" -d "$TMP"
else
	need tar
	tar -xzf "$TMP/$ASSET" -C "$TMP"
fi
[ -f "$TMP/$BIN$EXE" ] || die "the archive did not contain $BIN$EXE"

chmod +x "$TMP/$BIN$EXE"

# An arm64 Mach-O with no signature at all is killed by the kernel on Apple
# silicon. Release binaries are cross-compiled and therefore unsigned, so
# they get an ad-hoc signature here — the same thing the project's Makefile
# does for a local build.
if [ "$OS" = darwin ] && command -v codesign >/dev/null 2>&1; then
	xattr -d com.apple.quarantine "$TMP/$BIN$EXE" 2>/dev/null || true
	codesign --force --sign - "$TMP/$BIN$EXE" >/dev/null 2>&1 ||
		warn "could not ad-hoc sign the binary; if macOS refuses to run it, try: codesign --force --sign - $INSTALL_DIR/$BIN"
fi

mkdir -p "$INSTALL_DIR" || die "could not create $INSTALL_DIR"

# Remove before copying rather than writing over: a running process keeps the
# old inode, and on macOS overwriting in place leaves a stale signature that
# the kernel then refuses.
rm -f "$INSTALL_DIR/$BIN$EXE" 2>/dev/null || true
cp "$TMP/$BIN$EXE" "$INSTALL_DIR/$BIN$EXE" ||
	die "could not write to $INSTALL_DIR — set HITTABLE_INSTALL_DIR to somewhere you can write"
chmod +x "$INSTALL_DIR/$BIN$EXE"

step "installed to ${C_B}$INSTALL_DIR/$BIN$EXE${C_0}"

# Prove it actually runs on this machine before claiming success.
if "$INSTALL_DIR/$BIN$EXE" version >/dev/null 2>&1; then
	step "${C_G}$("$INSTALL_DIR/$BIN$EXE" version)${C_0}"
else
	die "installed, but $BIN could not run here.
       On macOS this is usually a signing problem: codesign --force --sign - $INSTALL_DIR/$BIN"
fi

say ""
case ":$PATH:" in
*":$INSTALL_DIR:"*)
	say "Run ${C_B}hittable${C_0} in a project directory to start."
	;;
*)
	say "${C_Y}$INSTALL_DIR is not on your PATH.${C_0} Add it:"
	say ""
	shell_rc="your shell's startup file"
	case "${SHELL:-}" in
	*/zsh) shell_rc="~/.zshrc" ;;
	*/bash) shell_rc="~/.bashrc" ;;
	*/fish) shell_rc="~/.config/fish/config.fish" ;;
	esac
	if [ "${SHELL:-}" = "${SHELL%fish}" ]; then
		say "  echo 'export PATH=\"\$PATH:$INSTALL_DIR\"' >> $shell_rc"
	else
		say "  fish_add_path $INSTALL_DIR"
	fi
	say ""
	say "Then run ${C_B}hittable${C_0} in a project directory."
	;;
esac

if [ "$OS" = windows ]; then
	say ""
	warn "the integrated terminal needs a PTY and is unavailable on native Windows;
         everything else works. WSL gives you the full app."
fi
say ""
