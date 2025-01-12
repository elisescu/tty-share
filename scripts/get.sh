#!/usr/bin/env sh
OS=$(uname -s)
ARCH=$(uname -m)

# Detect OS
case "$OS" in
    Linux*)   OS_NAME="linux";;
    Darwin*)  OS_NAME="darwin";;
    *)        echo "Unsupported OS: $OS"; exit 1;;
esac

# Detect architecture
case "$ARCH" in
    x86_64)   ARCH_NAME="amd64";;
    i386)     ARCH_NAME="386";;
    arm64)    ARCH_NAME="arm64";;
    *)        echo "Unsupported architecture: $ARCH"; exit 1;;
esac

# Construct the binary name (adjust to match your artifact naming)
BINARY="tty-share_${OS_NAME}-${ARCH_NAME}"
DOWNLOAD_URL="https://github.com/elisescu/tty-share/releases/latest/download/${BINARY}"

echo "Downloading ${DOWNLOAD_URL} ..."

# Download the appropriate binary
curl -sL ${DOWNLOAD_URL}  -o tty-share
chmod +x tty-share

echo 'Examples:
    headless mode: ./tty-share -A --public --headless --headless-cols 150 --headless-rows 50 --no-wait --listen :8001
    normal mode: ./tty-share -A --public  --no-wait --listen :8001
'
