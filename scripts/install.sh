#!/bin/sh
set -e

REPO="itzzritik/forged"

echo "Installing Forged..."

LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d '"' -f 4)
if [ -z "$LATEST" ]; then
    echo "Error: could not fetch latest release"
    exit 1
fi

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Error: unsupported architecture $ARCH"; exit 1 ;;
esac

URL="https://github.com/$REPO/releases/download/$LATEST/forged-${OS}-${ARCH}.tar.gz"

INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
fi

TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

echo "Downloading forged $LATEST for $OS/$ARCH..."
curl -fsSL "$URL" -o "$TMPDIR/forged.tar.gz"
tar xzf "$TMPDIR/forged.tar.gz" -C "$TMPDIR"

cp "$TMPDIR/forged" "$INSTALL_DIR/forged"
cp "$TMPDIR/forged-sign" "$INSTALL_DIR/forged-sign"
chmod +x "$INSTALL_DIR/forged" "$INSTALL_DIR/forged-sign"

echo "Installed to $INSTALL_DIR"
echo ""
echo "Run 'forged setup' to get started."

if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
    echo ""
    echo "Note: $INSTALL_DIR is not in your PATH. Add it:"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
fi
