#!/usr/bin/env bash
set -eo pipefail

git pull --tags
last_version=$(git tag --sort=committerdate | tail -1)
echo "Latest tag: ${last_version}"
version=$(echo $last_version | awk -F. '/[0-9]+\./{$NF++;print}' OFS=.)
echo "Next version: $version"

echo "Publishing release"
mkdir -p release/
cp "fyne-cross/dist/windows-amd64/TikTok Archiver.exe.zip" "release/TikTok Archiver-$version-windows-amd64.zip"
cp "fyne-cross/dist/darwin-amd64/TikTok Archiver.zip" "release/TikTok Archiver-$version-macos-amd64.zip"
cp "fyne-cross/dist/darwin-arm64/TikTok Archiver.zip" "release/TikTok Archiver-$version-macos-arm64.zip"
gh release create $version "release/TikTok Archiver-$version-windows-amd64.zip#Windows (64-bit)" "release/TikTok Archiver-$version-macos-amd64.zip#macOS (Intel)" "release/TikTok Archiver-$version-macos-arm64.zip#macOS (M1)" --title "TikTok Archiver $version" --generate-notes
