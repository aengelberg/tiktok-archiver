SHELL := /bin/bash

build-macos:
	fyne-cross darwin -app-id "com.aengelberg.tiktok-archiver" -icon resources/icon1024.png -name "TikTok Archiver" -arch='*'
	codesign -f -s "Alex Engelberg" fyne-cross/dist/darwin-amd64/TikTok\ Archiver.app/ --deep
	codesign -f -s "Alex Engelberg" fyne-cross/dist/darwin-arm64/TikTok\ Archiver.app/ --deep
	find fyne-cross/dist -name "TikTok Archiver.app" -type d -execdir sh -c 'zip -r "TikTok Archiver.zip" "{}"' \;

build-windows:
	fyne-cross windows -app-id "com.aengelberg.tiktok-archiver" -icon resources/icon1024.png -name "TikTok Archiver.exe" -arch='amd64'

release:
	scripts/release.sh

build-and-release: build-macos build-windows release

.PHONY: build-macos build-windows release build-and-release
