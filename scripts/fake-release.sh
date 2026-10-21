#!/usr/bin/env bash
set -euo pipefail
project_root=$(cd "$(dirname "$0")/.." && pwd -P)
if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    echo "Usage: scripts/fake-release.sh <output-directory> [version]" >&2
    exit 1
fi
output=$1
version=${2:-v0.1.0}
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([+-][a-zA-Z0-9.-]+)?$ ]] || { echo "Invalid release version: $version" >&2; exit 1; }
make -C "$project_root" build-all VERSION="$version"
for layout in latest/download "download/$version"; do
    mkdir -p -- "$output/$layout"
    cp "$project_root"/dist/saidran-* "$project_root/dist/checksums.txt" "$output/$layout/"
done
printf 'Release tree: %s\n' "$output"
