#!/usr/bin/env bash
set -euo pipefail

REPO="0xnu/mcp-core"
BINDIR="${BINDIR:-/usr/local/bin}"
VERSION="${VERSION:-latest}"

if [ "$VERSION" = "latest" ]; then
    VERSION=$(curl -sSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
    linux|darwin) ;;
    windows)
        echo "On Windows, use PowerShell instead:"
        echo "  powershell -c \"irm https://raw.githubusercontent.com/$REPO/main/install.ps1 | iex\""
        exit 0
        ;;
    *)
        echo "unsupported OS: $OS" >&2
        exit 1
        ;;
esac

echo "Downloading mcp-core $VERSION for $OS/$ARCH..."

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

URL="https://github.com/$REPO/releases/download/$VERSION/mcp-core-${VERSION}-${OS}-${ARCH}.tar.gz"

curl -fsSL "$URL" | tar -xz -C "$TMPDIR"

install -m 755 "$TMPDIR/mcp-core-${OS}-${ARCH}" "$BINDIR/mcp-core"
install -m 755 "$TMPDIR/corectl-${OS}-${ARCH}" "$BINDIR/corectl"

echo "Installed mcp-core and corectl to $BINDIR"
echo "Run 'mcp-core' to start the daemon"
