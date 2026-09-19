#!/usr/bin/env bash

OWNER="iyear"
REPO="tdl"
LOCATION="/usr/local/bin"

echo_green() {
    echo -e "\033[32m$1\033[0m"
}
echo_red() {
    echo -e "\033[31m$1\033[0m"
}
echo_blue() {
    echo -e "\033[34m$1\033[0m"
}

# Check if script is run as root
if [[ $EUID -ne 0 ]]; then
   echo_red "This script must be run as root"
   exit 1
fi

# Detect available downloader: prefer wget, fall back to curl
if command -v wget >/dev/null 2>&1; then
    DOWNLOADER="wget"
elif command -v curl >/dev/null 2>&1; then
    DOWNLOADER="curl"
else
    echo_red "Neither 'wget' nor 'curl' is installed."
    echo_red "Please install one of them and run this script again."
    exit 1
fi
echo_blue "Using downloader: $DOWNLOADER"

# fetch URL contents to stdout, quietly (used for API calls)
fetch() {
    case $DOWNLOADER in
        wget) wget -qO - "$1" ;;
        curl) curl -fsSL "$1" ;;
    esac
}

# download URL to stdout, with a progress indicator (used for the release tarball)
download() {
    case $DOWNLOADER in
        wget) wget -q --show-progress -O - "$1" ;;
        curl) curl -fL --progress-bar "$1" ;;
    esac
}

PROXY=""
VERSION=""

# flags: --proxy --version
while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        --proxy)
            PROXY="https://mirror.ghproxy.com/"
            echo_blue "Using GitHub proxy: $PROXY"
            shift
            ;;
        --version)
            VERSION="$2"
            shift
            shift
            ;;
        *)
            echo "Unknown flag: $key"
            exit 1
            ;;
    esac
done


# Set OS based on system
case $(uname -s) in
    Linux)
        OS="Linux"
        ;;
    Darwin)
        OS="MacOS"
        ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

# Set download ARCH based on system architecture
case $(uname -m) in
    x86_64)
        ARCH="64bit"
        ;;
    i686)
        ARCH="32bit"
        ;;
    armv5*)
        ARCH="armv5"
        ;;
    armv6*)
        ARCH="armv6"
        ;;
    armv7*)
        ARCH="armv7"
        ;;
    arm64|aarch64*)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# get latest version
if [ -z "$VERSION" ]; then
    VERSION=$(fetch "https://api.github.com/repos/$OWNER/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
fi
echo_blue "Target version: $VERSION"

# build download URL
URL=${PROXY}https://github.com/$OWNER/$REPO/releases/download/$VERSION/${REPO}_${OS}_$ARCH.tar.gz
CHECKSUM_URL=${PROXY}https://github.com/$OWNER/$REPO/releases/download/$VERSION/${REPO}_checksums.txt
echo_blue "Downloading $REPO from $URL"

TMP_DIR=$(mktemp -d) || exit 1
trap 'rm -rf "$TMP_DIR"' EXIT
ARCHIVE="$TMP_DIR/${REPO}_${OS}_$ARCH.tar.gz"

# download archive and checksums
download "$URL" > "$ARCHIVE"
download "$CHECKSUM_URL" > "$TMP_DIR/checksums.txt"

# verify sha256 checksum (macOS may only have shasum)
ARCHIVE_NAME=$(basename "$ARCHIVE")
EXPECTED=$(grep "  $ARCHIVE_NAME\$" "$TMP_DIR/checksums.txt" | awk '{print $1}')
if [ -z "$EXPECTED" ]; then
    echo_red "Checksum for $ARCHIVE_NAME not found in tdl_checksums.txt"
    exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL=$(sha256sum "$ARCHIVE" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
    ACTUAL=$(shasum -a 256 "$ARCHIVE" | awk '{print $1}')
else
    echo_red "Neither 'sha256sum' nor 'shasum' is installed, cannot verify checksum"
    exit 1
fi
if [ "$ACTUAL" != "$EXPECTED" ]; then
    echo_red "Checksum mismatch: expected $EXPECTED, got $ACTUAL"
    exit 1
fi
echo_green "Checksum verified"

# extract and install
tar -xzf "$ARCHIVE" -C "$TMP_DIR" && \
  mv "$TMP_DIR/$REPO" $LOCATION/$REPO && \
  chmod +x $LOCATION/$REPO && \
  echo_green "$REPO installed successfully! Location: $LOCATION/$REPO" && \
  echo_green "Run '$REPO' to get started" && \
  echo_green "To get started with tdl, please visit https://docs.iyear.me/tdl"
