#!/usr/bin/env bash

# waPC CLI location
: ${WAPC_INSTALL_DIR:="/usr/local/bin"}

# sudo is required to copy binary to WAPC_INSTALL_DIR for linux
: ${USE_SUDO:="false"}

# Http request CLI
WAPC_HTTP_REQUEST_CLI=curl

# GitHub Organization and repo name to download release
GITHUB_ORG=wapc
GITHUB_REPO=cli

# waPC CLI filename
WAPC_CLI_FILENAME=wapc

WAPC_CLI_FILE="${WAPC_INSTALL_DIR}/${WAPC_CLI_FILENAME}"

getSystemInfo() {
    ARCH=$(uname -m)
    case $ARCH in
        armv7*) ARCH="arm";;
        aarch64) ARCH="arm64";;
        x86_64) ARCH="amd64";;
    esac

    OS=$(echo `uname`|tr '[:upper:]' '[:lower:]')

    # Most linux distro needs root permission to copy the file to /usr/local/bin
    if [[ "$OS" == "linux" || "$OS" == "darwin" ]] && [ "$WAPC_INSTALL_DIR" == "/usr/local/bin" ]; then
        USE_SUDO="true"
    fi
}

verifySupported() {
    local supported=(darwin-amd64 linux-amd64 linux-arm linux-arm64)
    local current_osarch="${OS}-${ARCH}"

    for osarch in "${supported[@]}"; do
        if [ "$osarch" == "$current_osarch" ]; then
            echo "Your system is ${OS}_${ARCH}"
            return
        fi
    done

    if [ "$current_osarch" == "darwin-arm64" ]; then
        echo "The darwin_arm64 arch has no native binary, however you can use the amd64 version so long as you have rosetta installed"
        echo "Use 'softwareupdate --install-rosetta' to install rosetta if you don't already have it"
        ARCH="amd64"
        return
    fi


    echo "No prebuilt binary for ${current_osarch}"
    exit 1
}

runAsRoot() {
    local CMD="$*"

    if [ $EUID -ne 0 -a $USE_SUDO = "true" ]; then
        CMD="sudo $CMD"
    fi

    $CMD
}

checkHttpRequestCLI() {
    if type "curl" > /dev/null; then
        WAPC_HTTP_REQUEST_CLI=curl
    elif type "wget" > /dev/null; then
        WAPC_HTTP_REQUEST_CLI=wget
    else
        echo "Either curl or wget is required"
        exit 1
    fi
}

checkExistingwaPC() {
    if [ -f "$WAPC_CLI_FILE" ]; then
        echo -e "\nwaPC CLI is detected:"
        $WAPC_CLI_FILE version
        echo -e "Reinstalling waPC CLI - ${WAPC_CLI_FILE}...\n"
    else
        echo -e "Installing waPC CLI...\n"
    fi
}

getLatestRelease() {
    local wapcReleaseUrl="https://api.github.com/repos/${GITHUB_ORG}/${GITHUB_REPO}/releases"
    local latest_release=""

    if [ "$WAPC_HTTP_REQUEST_CLI" == "curl" ]; then
        latest_release=$(curl -s $wapcReleaseUrl | grep \"tag_name\" | grep -v rc | awk 'NR==1{print $2}' |  sed -n 's/\"\(.*\)\",/\1/p')
    else
        latest_release=$(wget -q --header="Accept: application/json" -O - $wapcReleaseUrl | grep \"tag_name\" | grep -v rc | awk 'NR==1{print $2}' |  sed -n 's/\"\(.*\)\",/\1/p')
    fi

    ret_val=$latest_release
}

downloadFile() {
    LATEST_RELEASE_TAG=$1

    WAPC_CLI_ARTIFACT="${WAPC_CLI_FILENAME}_${OS}_${ARCH}.tar.gz"
    DOWNLOAD_BASE="https://github.com/${GITHUB_ORG}/${GITHUB_REPO}/releases/download"
    DOWNLOAD_URL="${DOWNLOAD_BASE}/${LATEST_RELEASE_TAG}/${WAPC_CLI_ARTIFACT}"

    # Create the temp directory
    WAPC_TMP_ROOT=$(mktemp -dt wapc-install-XXXXXX)
    ARTIFACT_TMP_FILE="$WAPC_TMP_ROOT/$WAPC_CLI_ARTIFACT"

    echo "Downloading $DOWNLOAD_URL ..."
    if [ "$WAPC_HTTP_REQUEST_CLI" == "curl" ]; then
        curl -SsL "$DOWNLOAD_URL" -o "$ARTIFACT_TMP_FILE"
    else
        wget -q -O "$ARTIFACT_TMP_FILE" "$DOWNLOAD_URL"
    fi

    if [ ! -f "$ARTIFACT_TMP_FILE" ]; then
        echo "failed to download $DOWNLOAD_URL ..."
        exit 1
    fi
}

installFile() {
    tar xf "$ARTIFACT_TMP_FILE" -C "$WAPC_TMP_ROOT"
    local tmp_root_wapc_cli="$WAPC_TMP_ROOT/${WAPC_CLI_FILENAME}_${OS}_${ARCH}/$WAPC_CLI_FILENAME"

    if [ ! -f "$tmp_root_wapc_cli" ]; then
        echo "Failed to unpack waPC CLI executable."
        exit 1
    fi

    chmod o+x $tmp_root_wapc_cli
    runAsRoot cp "$tmp_root_wapc_cli" "$WAPC_INSTALL_DIR"

    if [ -f "$WAPC_CLI_FILE" ]; then
        echo "$WAPC_CLI_FILENAME installed into $WAPC_INSTALL_DIR successfully."

        $WAPC_CLI_FILE version
    else 
        echo "Failed to install $WAPC_CLI_FILENAME"
        exit 1
    fi
}

fail_trap() {
    result=$?
    if [ "$result" != "0" ]; then
        echo "Failed to install waPC CLI"
        echo "For support, go to https://wapc.io"
    fi
    cleanup
    exit $result
}

cleanup() {
    if [[ -d "${WAPC_TMP_ROOT:-}" ]]; then
        rm -rf "$WAPC_TMP_ROOT"
    fi
}

installCompleted() {
    echo -e "\nwaPC CLI is installed successfully."
}

# -----------------------------------------------------------------------------
# main
# -----------------------------------------------------------------------------
trap "fail_trap" EXIT

getSystemInfo
verifySupported
checkExistingwaPC
checkHttpRequestCLI


if [ -z "$1" ]; then
    echo "Getting the latest waPC CLI..."
    getLatestRelease
else
    ret_val=v$1
fi

echo "Installing $ret_val waPC CLI..."

downloadFile $ret_val
installFile
cleanup

installCompleted
