#!/usr/bin/env bash
set -eo pipefail

last_version=$(git describe --tags --abbrev=0)
echo "Latest tag: ${last_version}"

version=$(echo $last_version | awk -F. '/[0-9]+\./{$NF++;print}' OFS=.)
echo "Next version: $version"

echo "Building Windows"
GOOS=windows GOARCH=amd64 go build -o ttdl.exe -ldflags -H=windowsgui main.go
zip -r ttdl-${version}-windows-amd64.zip ttdl.exe

echo "Building macOS amd64"
GOOS=darwin GOARCH=amd64 go build -o ttdl main.go
go run macapp.go -assets resources/ -bin ttdl -icon resources/ttdl1024x1024.png -identifier com.aengelberg.ttdl -name TTDL -o .
zip -r ttdl-${version}-macos-amd64.zip TTDL.app

echo "Building macOS arm64"
GOOS=darwin GOARCH=arm64 go build -o ttdl main.go
go run macapp.go -assets resources/ -bin ttdl -icon resources/ttdl1024x1024.png -identifier com.aengelberg.ttdl -name TTDL -o .
zip -r ttdl-${version}-macos-arm64.zip TTDL.app

echo "Publishing release"
echo $GH_TOKEN | gh auth login --with-token
gh release create $version "ttdl-${version}-windows-amd64.zip#Windows (64-bit)" "ttdl-${version}-macos-amd64.zip#macOS (Intel)" "ttdl-${version}-macos-arm64.zip#macOS (M1)" --title "TTDL $version" --notes "Release for version $version"