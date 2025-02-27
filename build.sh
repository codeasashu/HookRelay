#!/bin/bash

NAME_FILE_BIN='hookrelay'

# Path info to export the app
PATH_DIR_SCRIPT="$(cd "$(dirname "${BASH_SOURCE:-$0}")" && pwd)"
PATH_DIR_BIN="${PATH_DIR_SCRIPT}/bin"
PATH_FILE_BIN="${PATH_DIR_BIN}/${NAME_FILE_BIN}"

# Default values
GOOS_DEFAULT='linux'
GOARCH_DEFAULT='amd64'
GOARM_DEFAULT=''

# Status
SUCCESS=0
FAILURE=1

# -----------------------------------------------------------------------------
#  Functions
# -----------------------------------------------------------------------------

echoHelp() {
    cat <<'HEREDOC'
About:
  This script builds the binary of the app under ./bin directory.

Usage:
  ./build.sh <GOOS>
  ./build.sh <GOOS> [<GOARCH>]
  ./build.sh <GOOS> [<GOARCH> <GOARM>]
  ./build.sh [-l] [--list] [-h] [--help]

GOOS:
  The fisrt argument is the OS platform. Such as:

    "linux", "darwin", "windows", etc.

  For supported platforms specify '--list' option.

GOARCH:
  The 2nd argument is the architecture (CPU type). Such as:

    "amd64"(64bit, Intel/AMD/x86_64), "arm", "arm64", "386", etc. (Default: "amd64")

  For supported architectures specify '--list' option.

GOARM:
  The 3rd argument is the ARM variant/version. Such as:

    "5", "6", "7".(Default: empty)

  For supported combination see: https://github.com/golang/go/wiki/GoArm

Options:
  -l --list ... Displays available platforms and architectures to build.
  -h --help ... Displays this help.

Sample usage:

  # Display available arcitectures
  ./build.sh --list
  ./build.sh -l

  # Build Linux (Intel) binary
  ./build.sh linux

  # Build macOS binary
  ./build.sh darwin        #Equivalent to: ./build.sh darwin amd64
  ./build.sh darwin arm64

  # Build Windows10 binary
  ./build.sh windows

  # Build Raspberry Pi 3 binary
  ./build.sh linux arm 7

  # Build QNAP ARM5 binary
  ./build.sh linux arm 5

HEREDOC

    exit $SUCCESS
}

indentSTDIN() {
    indent='    '
    while read -r line; do
        echo "${indent}${line}"
    done
    echo
}

listPlatforms() {
    list=$(go tool dist list) || {
        echo >&2 'Failed to get supported platforms.'
        echo "$list" | indentSTDIN 1>&2
        exit $FAILURE
    }
    echo 'List of available platforms to build. (GOOS/GOARCH)'
    echo "$list" | indentSTDIN
    exit $SUCCESS
}

# -----------------------------------------------------------------------------
#  Main
# -----------------------------------------------------------------------------

if [ "$#" -eq 0 ]; then
    echoHelp
fi

case "$1" in
"--help") echoHelp ;;
"-h") echoHelp ;;
"--list") listPlatforms ;;
"-l") listPlatforms ;;
esac

GOOS="${1:-"$GOOS_DEFAULT"}"
GOARCH="${2:-"$GOARCH_DEFAULT"}"
GOARM="${3:-"$GOARM_DEFAULT"}"
GOARM_SUFFIX="${GOARM:+"-${GOARM}"}"

# Get version from Git tags or VERSION file
if VERSION_APP="$(git describe --tags --always --dirty 2>/dev/null)"; then
    echo "Using Git tag for version: ${VERSION_APP}"
else
    VERSION_APP="$(cat VERSION 2>/dev/null || echo 'v0.0.0-unknown')"
    echo "Using VERSION file for version: ${VERSION_APP}"
fi

# Build as static linked binary
echo '- Building binary to ...'
echo "  ${PATH_FILE_BIN}"
echo "  Ver: ${VERSION_APP}"

if CGO_ENABLED=1 GOOS="$GOOS" \
    GOARCH="$GOARCH" \
    GOARM="$GOARM" \
    go build \
    -installsuffix "$NAME_FILE_BIN" \
    -ldflags="-s -w -X 'main.Version=${VERSION_APP}'" \
    -o="$PATH_FILE_BIN" \
    main.go; then
    exit $SUCCESS
fi
echo 'Failed to build binary.'
exit $FAILURE
