#!/bin/sh
set -e

REPO="luuuc/council"
BINARY_NAME="council"

# Determine install directory (prefer user-writable locations)
if [ -n "$INSTALL_DIR" ]; then
  # User specified
  :
elif [ -w "/usr/local/bin" ]; then
  INSTALL_DIR="/usr/local/bin"
elif [ -d "$HOME/.local/bin" ] || mkdir -p "$HOME/.local/bin" 2>/dev/null; then
  INSTALL_DIR="$HOME/.local/bin"
else
  INSTALL_DIR="/usr/local/bin"
fi

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin) OS="darwin" ;;
  linux) OS="linux" ;;
  mingw*|msys*|cygwin*) OS="windows" ;;
  *)
    echo "Unsupported OS: $OS"
    exit 1
    ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1
    ;;
esac

# Get latest version
echo "Fetching latest release..."
VERSION=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)

if [ -z "$VERSION" ]; then
  echo "Failed to fetch latest version"
  exit 1
fi

echo "Latest version: $VERSION"

# Check if already installed with same version
CURRENT_VERSION=""
if command -v council >/dev/null 2>&1; then
  CURRENT_VERSION=$(council --version 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' || echo "")
fi

if [ "$CURRENT_VERSION" = "$VERSION" ]; then
  echo "council $VERSION is already installed and up to date."
  echo ""
  echo "Run 'council --help' to see available commands."
  exit 0
fi

if [ -n "$CURRENT_VERSION" ]; then
  echo "Upgrading from $CURRENT_VERSION to $VERSION"
fi

# Build download URL (GoReleaser format)
VERSION_NUM="${VERSION#v}"  # Strip leading 'v'
if [ "$OS" = "windows" ]; then
  ARCHIVE="council_${VERSION_NUM}_${OS}_${ARCH}.zip"
else
  ARCHIVE="council_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
fi

URL="https://github.com/$REPO/releases/download/$VERSION/$ARCHIVE"

echo "Downloading from: $URL"

# Download and extract
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

if ! curl -fsSL "$URL" -o "$TMP_DIR/$ARCHIVE"; then
  echo "Download failed"
  exit 1
fi

cd "$TMP_DIR"
if [ "$OS" = "windows" ]; then
  unzip -q "$ARCHIVE"
else
  tar -xzf "$ARCHIVE"
fi

# Install
chmod +x "$BINARY_NAME"
mkdir -p "$INSTALL_DIR"
mv "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"

echo ""
echo "council $VERSION installed to $INSTALL_DIR/$BINARY_NAME"

# Check if install dir is in PATH
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo ""
    echo "Add to your PATH:"
    echo "  export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac

echo ""
echo "Get started:"
echo "  council init           Create .council/ and install the council skills"
echo "  /council-assemble      In your AI tool: pick and build your council"
echo "  /council <topic>       In your AI tool: convene it"
