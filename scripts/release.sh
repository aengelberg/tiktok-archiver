#!/usr/bin/env bash
set -eo pipefail

git pull --tags
last_version=$(git describe --tags --abbrev=0)
echo "Latest tag: ${last_version}"
version=$(echo $last_version | awk -F. '/[0-9]+\./{$NF++;print}' OFS=.)
echo "Next version: $version"

echo "Publishing release"
cp fyne-cross/dist/windows-amd64/TTDL.exe.zip TTDL-$version-windows-amd64.zip
cp fyne-cross/dist/darwin-amd64/TTDL.zip TTDL-$version-macos-amd64.zip
cp fyne-cross/dist/darwin-arm64/TTDL.zip TTDL-$version-macos-arm64.zip
gh release create $version "TTDL-$version-windows-amd64.zip#Windows (64-bit)" "TTDL-$version-macos-amd64.zip#macOS (Intel)" "TTDL-$version-macos-arm64.zip#macOS (M1)" --title "TTDL $version" --generate-notes
