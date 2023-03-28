#!/usr/bin/env bash
set -eo pipefail

last_version=$(git describe --tags --abbrev=0)
echo "Latest tag: ${last_version}"
version=$(echo $last_version | awk -F. '/[0-9]+\./{$NF++;print}' OFS=.)
echo "Next version: $version"

echo "Publishing release"
gh release create $version "fyne-cross/dist/windows-amd64/TTDL.exe.zip#Windows (64-bit)" "fyne-cross/dist/darwin-amd64/TTDL.zip#macOS (Intel)" "fyne-cross/dist/darwin-arm64/TTDL.zip#macOS (M1)" --title "TTDL $version" --notes "Release for version $version"
