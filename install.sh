#!/usr/bin/env bash
set -euo pipefail

fail() { echo "Error: $*" >&2; exit 1; }
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
    linux|darwin) ;;
    mingw*|msys*|cygwin*) fail "On Windows, download saidran-windows-amd64.exe or saidran-windows-arm64.exe from https://github.com/vincentsch/saidran/releases and add its folder to PATH." ;;
    *) fail "Unsupported OS: $os. Supported Unix hosts are Linux and macOS." ;;
esac
arch=$(uname -m)
case "$arch" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) fail "Unsupported architecture: $arch. Use amd64 or arm64." ;;
esac
command -v curl >/dev/null 2>&1 || fail "Install curl, then run this installer again."
if ! command -v git >/dev/null 2>&1; then
    echo "Warning: git is missing. Install git before running saidran demo or checking a repository." >&2
fi
base=${SAIDRAN_DOWNLOAD_BASE:-https://github.com/vincentsch/saidran/releases}
base=${base%/}
if [ -n "${SAIDRAN_VERSION:-}" ]; then
    url="$base/download/$SAIDRAN_VERSION"
else
    url="$base/latest/download"
fi
asset="saidran-$os-$arch"
task_temp=$(mktemp -d)
staging=""
cleanup() {
    rm -rf -- "$task_temp"
    if [ -n "$staging" ]; then rm -f -- "$staging"; fi
}
trap cleanup EXIT
curl -fsSL "$url/$asset" -o "$task_temp/$asset" || fail "Download failed for $asset. Check the release version and download URL."
curl -fsSL "$url/checksums.txt" -o "$task_temp/checksums.txt" || fail "Download failed for checksums.txt."
expected=$(awk -v name="$asset" '$2 == name {print $1}' "$task_temp/checksums.txt")
[[ "$expected" =~ ^[[:xdigit:]]{64}$ ]] || fail "Missing or invalid SHA-256 checksum for $asset."
if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$task_temp/$asset" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
    actual=$(shasum -a 256 "$task_temp/$asset" | awk '{print $1}')
else
    fail "Install sha256sum or shasum to verify the download."
fi
[ "$actual" = "$expected" ] || fail "Checksum mismatch for $asset. The existing installation was kept."
install_dir=${INSTALL_DIR:-$HOME/.local/bin}
mkdir -p -- "$install_dir" || fail "Cannot create $install_dir. Set INSTALL_DIR to a writable folder."
install_dir=$(cd -- "$install_dir" && pwd -P)
staging=$(mktemp "$install_dir/.saidran.XXXXXX")
cp -- "$task_temp/$asset" "$staging"
chmod 755 "$staging"
mv -f -- "$staging" "$install_dir/saidran"
staging=""
printf 'Installed: %s/saidran\n' "$install_dir"
case ":$PATH:" in
    *":$install_dir:"*) ;;
    *) printf 'Add this line to your shell profile:\nexport PATH=%q:"$PATH"\n' "$install_dir" ;;
esac
