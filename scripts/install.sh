#!/bin/sh
# Install mtrx without Go. Usage:
#   curl -fsSL https://raw.githubusercontent.com/snhsish/mtrx/main/scripts/install.sh | sh
#   MTRX_VERSION=v0.1.0 sh scripts/install.sh
set -eu

REPO="snhsish/mtrx"
VERSION="${MTRX_VERSION:-latest}"
BINDIR="${MTRX_BINDIR:-}"

if [ "$VERSION" = "latest" ]; then
  if command -v curl >/dev/null 2>&1; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
  elif command -v wget >/dev/null 2>&1; then
    VERSION=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
  else
    echo "error: need curl or wget to resolve latest version (or set MTRX_VERSION=vX.Y.Z)" >&2
    exit 1
  fi
fi
[ -n "$VERSION" ] || { echo "error: could not resolve version" >&2; exit 1; }

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$OS" in
  linux|darwin) ;;
  *) echo "error: unsupported OS '$OS' (Windows: use scripts/install.ps1)" >&2; exit 1 ;;
esac
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "error: unsupported arch '$ARCH'" >&2; exit 1 ;;
esac

if [ -z "$BINDIR" ]; then
  if [ -w /usr/local/bin ] 2>/dev/null; then BINDIR="/usr/local/bin"
  else BINDIR="$HOME/.local/bin"; fi
fi

ARCHIVE="mtrx-${OS}-${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM

echo "Downloading mtrx ${VERSION} for ${OS}/${ARCH}..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$URL" -o "$TMP/$ARCHIVE"
else
  wget -q "$URL" -O "$TMP/$ARCHIVE"
fi
tar -xzf "$TMP/$ARCHIVE" -C "$TMP"

mkdir -p "$BINDIR"
mv "$TMP/mtrx-${OS}-${ARCH}" "$BINDIR/mtrx"
chmod +x "$BINDIR/mtrx"

echo "Installed to $BINDIR/mtrx"
"$BINDIR/mtrx" version
case ":$PATH:" in
  *":$BINDIR:"*) ;;
  *) echo "Note: $BINDIR is not on your PATH." >&2 ;;
esac
