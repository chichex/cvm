#!/bin/sh
set -e

REPO="chichex/cvm"
INSTALL_DIR="/usr/local/bin"

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Get latest version
VERSION=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"v\(.*\)".*/\1/')
if [ -z "$VERSION" ]; then
  echo "Failed to get latest version"
  exit 1
fi

URL="https://github.com/$REPO/releases/download/v${VERSION}/cvm_${VERSION}_${OS}_${ARCH}.tar.gz"

echo "Installing cvm v${VERSION} (${OS}/${ARCH})..."
TMP=$(mktemp -d)
trap "rm -rf $TMP" EXIT

curl -sL "$URL" -o "$TMP/cvm.tar.gz"
tar -xzf "$TMP/cvm.tar.gz" -C "$TMP"

if [ -w "$INSTALL_DIR" ]; then
  cp "$TMP/cvm" "$INSTALL_DIR/cvm"
else
  echo "Need sudo to install to $INSTALL_DIR"
  sudo cp "$TMP/cvm" "$INSTALL_DIR/cvm"
fi

chmod +x "$INSTALL_DIR/cvm"
echo "Installed cvm v${VERSION} to $INSTALL_DIR/cvm"
